package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"deploy-app/internal/appconf"
	"deploy-app/internal/buildkit"
	"deploy-app/internal/selfupdate"
	"deploy-app/internal/settings"

	"golang.org/x/sys/windows"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// swShowNormal 是 Win32 的 SW_SHOWNORMAL；swHide 是 SW_HIDE
const (
	swShowNormal = 1
	swHide       = 0
)

// 解压安全上限：防止 zip 炸弹写满磁盘
var (
	maxZipFiles     = 10000          // 单个 ZIP 允许的文件数
	maxZipTotalSize = int64(2) << 30 // 解压后总大小上限（2 GiB）
)

// App 应用主结构体：Wails 绑定的薄适配层，具体逻辑在 internal 各包。
type App struct {
	ctx            context.Context
	mu             sync.Mutex
	building       bool
	lastProgress   int
	tempDirs       map[string]struct{}
	lastImportRoot string
	buildCancel    context.CancelFunc
}

func NewApp() *App {
	return &App{
		tempDirs: make(map[string]struct{}),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	// 拖拽文件/文件夹到窗口：把路径转交给前端统一处理
	wailsRuntime.OnFileDrop(ctx, func(x, y int, paths []string) {
		wailsRuntime.EventsEmit(ctx, "app:file-drop", paths)
	})
}

func (a *App) shutdown(context.Context) {
	// 若构建仍在进行，先取消并短暂等待，避免删除正在使用的工作目录
	a.mu.Lock()
	cancel := a.buildCancel
	a.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	for i := 0; i < 100; i++ {
		a.mu.Lock()
		building := a.building
		a.mu.Unlock()
		if !building {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	a.cleanupTempDirs()
}

func (a *App) confirmClose(ctx context.Context) bool {
	result, err := wailsRuntime.MessageDialog(ctx, wailsRuntime.MessageDialogOptions{
		Type:    wailsRuntime.QuestionDialog,
		Title:   "确认关闭",
		Message: "确定要关闭应用吗？",
	})
	if err != nil {
		// 对话框失败时允许关闭，避免卡死
		return false
	}
	return result != "Yes" && result != "Ok" && result != "OK"
}

// ---------- 构建 ----------

// beginBuild 标记构建开始，并创建可用于取消的上下文。
func (a *App) beginBuild() (context.Context, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.building {
		return nil, false
	}
	a.building = true
	a.lastProgress = 0
	ctx, cancel := context.WithCancel(context.Background())
	a.buildCancel = cancel
	return ctx, true
}

func (a *App) endBuild() {
	a.mu.Lock()
	if a.buildCancel != nil {
		a.buildCancel() // 释放 context 资源
		a.buildCancel = nil
	}
	a.building = false
	a.mu.Unlock()
}

// CancelBuild 取消当前正在进行的构建
func (a *App) CancelBuild() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.building || a.buildCancel == nil {
		return fmt.Errorf("当前没有进行中的构建")
	}
	a.buildCancel()
	return nil
}

// BuildApp 执行完整的应用构建流程
func (a *App) BuildApp(config appconf.BuildConfig) error {
	buildCtx, ok := a.beginBuild()
	if !ok {
		return fmt.Errorf("已有构建任务正在进行")
	}
	defer a.endBuild()

	opts, err := buildOptions()
	if err != nil {
		return fmt.Errorf("读取运行壳资产失败: %w", err)
	}
	var tempDir string
	opts.OnTempDir = func(dir string) {
		tempDir = dir
		a.trackTempDir(dir)
	}

	outputPath, signScript, err := buildkit.Build(buildCtx, config, opts, a.emitProgress)
	if tempDir != "" {
		// 管线返回时工作目录已随 defer 删除，这里解除登记
		a.untrackTempDir(tempDir)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("构建已取消")
		}
		return err
	}
	if a.ctx != nil {
		wailsRuntime.EventsEmit(a.ctx, "build:complete", map[string]interface{}{
			"outputPath":     outputPath,
			"signScriptPath": signScript,
		})
	}
	return nil
}

// emitProgress 把管线进度转成单调递增的前端事件
func (a *App) emitProgress(step string, percent int) {
	a.mu.Lock()
	if percent < a.lastProgress {
		percent = a.lastProgress
	}
	a.lastProgress = percent
	a.mu.Unlock()
	if a.ctx == nil {
		return
	}
	wailsRuntime.EventsEmit(a.ctx, "build:progress", map[string]interface{}{
		"step":     step,
		"progress": percent,
	})
}

// ---------- 导入构建产物 ----------

// OpenDistFolder 选择前端构建产物目录
func (a *App) OpenDistFolder() (string, error) {
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择前端构建产物文件夹",
	})
	if err != nil || path == "" {
		return path, err
	}
	return a.importDistDir(path)
}

// normalizeDistDir 校验并规范化 dist 目录：
// 目录含 index.html 直接返回；只有一个子目录且其中包含 index.html 时自动下钻
// （常见于选中项目根目录或 ZIP 内的 dist 包一层）。
func normalizeDistDir(dir string) (string, error) {
	dir = filepath.Clean(dir)
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
		return dir, nil
	}
	if entries, err := os.ReadDir(dir); err == nil && len(entries) == 1 && entries[0].IsDir() {
		sub := filepath.Join(dir, entries[0].Name())
		if _, err := os.Stat(filepath.Join(sub, "index.html")); err == nil {
			return sub, nil
		}
	}
	return "", fmt.Errorf("所选目录中未找到 index.html")
}

// importDistDir 校验并规范化 dist 目录。
func (a *App) importDistDir(dir string) (string, error) {
	return normalizeDistDir(dir)
}

// UploadDistZip 选择并解压 ZIP 构建产物
func (a *App) UploadDistZip(tempPath string) (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择 ZIP 压缩包",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "ZIP 压缩包", Pattern: "*.zip"},
		},
	})
	if err != nil || path == "" {
		return path, err
	}
	return a.importZipFile(path, tempPath)
}

// ImportFromPaths 处理拖拽导入：一次一个 dist 目录或 ZIP 包
func (a *App) ImportFromPaths(tempPath string, paths []string) (string, error) {
	if len(paths) != 1 {
		return "", fmt.Errorf("一次只能拖入一个 dist 目录或 ZIP 压缩包")
	}
	p := paths[0]
	info, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("拖入的路径无效: %w", err)
	}
	if info.IsDir() {
		return a.importDistDir(p)
	}
	if strings.EqualFold(filepath.Ext(p), ".zip") {
		return a.importZipFile(p, tempPath)
	}
	return "", fmt.Errorf("仅支持拖入 dist 目录或 ZIP 压缩包")
}

// importZipFile 解压 ZIP 到会话临时目录并返回 dist 目录。
func (a *App) importZipFile(zipPath, tempPath string) (string, error) {
	tempBase, err := buildkit.ResolveTempBase(tempPath, filepath.Dir(zipPath))
	if err != nil {
		return "", err
	}
	tempRoot, err := os.MkdirTemp(tempBase, "deploy-dist-*")
	if err != nil {
		return "", fmt.Errorf("创建临时目录失败: %w", err)
	}
	a.trackTempDir(tempRoot)

	if err := extractZip(zipPath, tempRoot); err != nil {
		_ = os.RemoveAll(tempRoot)
		a.untrackTempDir(tempRoot)
		return "", fmt.Errorf("解压失败: %w", err)
	}

	distDir, err := normalizeDistDir(tempRoot)
	if err != nil {
		_ = os.RemoveAll(tempRoot)
		a.untrackTempDir(tempRoot)
		return "", err
	}

	// 导入成功后清理上一次导入的临时目录，避免反复导入时堆积。
	a.replaceLastImportRoot(tempRoot)
	return distDir, nil
}

// GetDistInfo 扫描构建产物目录
func (a *App) GetDistInfo(path string) (*appconf.DistInfo, error) {
	info := &appconf.DistInfo{Path: path}
	indexFile := filepath.Join(path, "index.html")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) {
		return info, fmt.Errorf("目录中未找到 index.html")
	}

	var fileCount int
	var totalSize int64
	err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileCount++
			fi, err := d.Info()
			if err == nil {
				totalSize += fi.Size()
			}
		}
		return nil
	})
	if err != nil {
		return info, err
	}
	info.FileCount = fileCount
	info.TotalSize = totalSize
	info.Valid = true
	info.SuggestedName = suggestAppName(path)
	return info, nil
}

// suggestAppName 从 dist 同级或自身的 package.json 推断默认应用名。
func suggestAppName(distPath string) string {
	for _, dir := range []string{filepath.Dir(distPath), distPath} {
		data, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err != nil {
			continue
		}
		var pkg struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(data, &pkg) != nil || pkg.Name == "" {
			continue
		}
		if name, err := appconf.SanitizeAppName(pkg.Name); err == nil {
			return name
		}
	}
	return ""
}

// ---------- 代理 ----------

// TestProxyTarget 测试代理目标地址连通性，返回 HTTP 状态文本
func (a *App) TestProxyTarget(target string) (string, error) {
	target = strings.TrimSpace(target)
	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("目标地址无效，需形如 http://host:port/")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("仅支持 http/https")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	return resp.Status, nil
}

// ---------- 文件选择与其他绑定 ----------

// OpenTempFolder 选择临时目录
func (a *App) OpenTempFolder() (string, error) {
	path, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择临时文件目录",
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// OpenIconFile 选择图标文件
func (a *App) OpenIconFile() (string, error) {
	path, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择应用图标",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "图标文件", Pattern: "*.ico;*.png"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// ValidateIcon 验证图标并返回 base64 预览
func (a *App) ValidateIcon(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".ico" && ext != ".png" {
		return "", fmt.Errorf("不支持的图标格式: %s（请使用 .ico 或 .png）", ext)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取图标文件失败: %w", err)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("图标文件为空")
	}

	if ext == ".png" {
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return "", fmt.Errorf("无效的 PNG 文件: %w", err)
		}
		bounds := img.Bounds()
		if bounds.Dx() != bounds.Dy() {
			return "", fmt.Errorf("图标必须为正方形，当前尺寸: %dx%d", bounds.Dx(), bounds.Dy())
		}
		if bounds.Dx() < 64 {
			return "", fmt.Errorf("图标尺寸太小: %dx%d（建议至少 256x256）", bounds.Dx(), bounds.Dy())
		}
	} else {
		// Basic ICO header validation: reserved(0), type(1), count>0
		if len(data) < 6 {
			return "", fmt.Errorf("无效的 ICO 文件")
		}
		if data[0] != 0 || data[1] != 0 || data[2] != 1 || data[3] != 0 {
			return "", fmt.Errorf("无效的 ICO 文件头")
		}
		count := int(data[4]) | int(data[5])<<8
		if count <= 0 {
			return "", fmt.Errorf("ICO 文件不包含图标图像")
		}
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	mime := "image/png"
	if ext == ".ico" {
		mime = "image/x-icon"
	}
	return "data:" + mime + ";base64," + b64, nil
}

// OpenSignPfxFile 选择签名证书文件
func (a *App) OpenSignPfxFile() (string, error) {
	return wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "选择签名证书 (PFX)",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "PFX 证书", Pattern: "*.pfx"},
		},
	})
}

// SelectOutputPath 在构建前选择输出文件位置
func (a *App) SelectOutputPath(appName string) (string, error) {
	defaultName := "app.exe"
	if name, err := appconf.SanitizeAppName(strings.TrimSpace(appName)); err == nil && name != "" {
		defaultName = name + ".exe"
	}
	path, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "选择保存位置",
		DefaultFilename: defaultName,
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "可执行文件", Pattern: "*.exe"},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// RunOutput 试运行生成的 exe
func (a *App) RunOutput(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("输出路径为空")
	}
	if !strings.EqualFold(filepath.Ext(path), ".exe") {
		return fmt.Errorf("仅支持运行 .exe 文件")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("文件不存在: %w", err)
	}
	filePtr, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return err
	}
	// ShellExecute 是 Windows 打开文件的标准方式，等价于在资源管理器中双击
	return windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), filePtr, nil, nil, swShowNormal)
}

// OpenOutputFolder 在资源管理器中打开输出目录
func (a *App) OpenOutputFolder(path string) error {
	if path == "" {
		return fmt.Errorf("输出路径为空")
	}
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("输出目录不存在: %w", err)
	}
	cmd := exec.Command("explorer", dir)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("打开目录失败: %w", err)
	}
	return nil
}

// ---------- 检查更新与在线更新 ----------

// AppVersion 返回打包工具自身版本号
func (a *App) AppVersion() string {
	return appVersion
}

// CheckUpdate 检查是否有新版本
func (a *App) CheckUpdate() (*selfupdate.Info, error) {
	return selfupdate.Check(appVersion)
}

// ApplyUpdate 下载新版本并生成更新脚本，启动脚本后应用自动退出，
// 由脚本完成替换并重启新版。
func (a *App) ApplyUpdate(downloadURL string) (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(exePath)
	if err != nil {
		return "", err
	}
	scriptPath, err := selfupdate.Apply(context.Background(), downloadURL, abs)
	if err != nil {
		return "", err
	}
	scriptPtr, err := windows.UTF16PtrFromString(scriptPath)
	if err != nil {
		return "", err
	}
	// 隐藏窗口运行更新脚本；脚本会等应用退出后替换并重启
	if err := windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), scriptPtr, nil, nil, swHide); err != nil {
		return "", fmt.Errorf("启动更新程序失败: %w", err)
	}
	go func() {
		time.Sleep(800 * time.Millisecond)
		wailsRuntime.Quit(a.ctx)
	}()
	return scriptPath, nil
}

// ---------- 配置持久化与方案（internal/settings 的绑定适配） ----------

// SaveSettings 保存上次构建配置（前端在配置变化时调用）
func (a *App) SaveSettings(config appconf.BuildConfig) error {
	return settings.SaveLastConfig(config)
}

// LoadSettings 读取上次构建配置；首次使用返回零值
func (a *App) LoadSettings() (appconf.BuildConfig, error) {
	return settings.LoadLastConfig()
}

// SaveProfile 保存当前配置为命名方案，返回清洗后的方案名
func (a *App) SaveProfile(name string, config appconf.BuildConfig) (string, error) {
	return settings.SaveProfile(name, config)
}

// ListProfiles 列出全部已保存方案
func (a *App) ListProfiles() ([]string, error) {
	return settings.ListProfiles()
}

// LoadProfile 载入指定方案
func (a *App) LoadProfile(name string) (appconf.BuildConfig, error) {
	return settings.LoadProfile(name)
}

// DeleteProfile 删除指定方案
func (a *App) DeleteProfile(name string) error {
	return settings.DeleteProfile(name)
}

// ExportProfile 将方案导出为 JSON 文件（弹出保存对话框），返回导出路径
func (a *App) ExportProfile(name string) (string, error) {
	clean, err := settings.SanitizeProfileName(name)
	if err != nil {
		return "", err
	}
	src := filepath.Join(settings.ProfilesDir(), clean+".json")
	if _, err := os.Stat(src); err != nil {
		return "", fmt.Errorf("方案不存在: %s", name)
	}
	dest, err := wailsRuntime.SaveFileDialog(a.ctx, wailsRuntime.SaveDialogOptions{
		Title:           "导出构建方案",
		DefaultFilename: clean + ".json",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "JSON 文件", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if dest == "" {
		return "", nil
	}
	if err := copyFileSync(src, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// ImportProfile 从 JSON 文件导入方案（弹出选择对话框），返回方案名
func (a *App) ImportProfile() (string, error) {
	src, err := wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "导入构建方案",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "JSON 文件", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", err
	}
	if src == "" {
		return "", nil
	}
	config, err := appconf.ReadConfigFile(src)
	if err != nil {
		return "", fmt.Errorf("导入失败: %w", err)
	}
	name := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	clean, err := settings.SaveProfile(name, config)
	if err != nil {
		// 文件名不合法时回退到默认名称
		clean, err = settings.SaveProfile("导入的方案", config)
		if err != nil {
			return "", err
		}
	}
	return clean, nil
}

// ---------- 会话临时目录管理 ----------

func (a *App) trackTempDir(dir string) {
	if dir == "" {
		return
	}
	a.mu.Lock()
	a.tempDirs[dir] = struct{}{}
	a.mu.Unlock()
}

func (a *App) untrackTempDir(dir string) {
	a.mu.Lock()
	delete(a.tempDirs, dir)
	a.mu.Unlock()
}

func (a *App) cleanupTempDirs() {
	a.mu.Lock()
	dirs := make([]string, 0, len(a.tempDirs))
	for d := range a.tempDirs {
		dirs = append(dirs, d)
	}
	a.tempDirs = make(map[string]struct{})
	a.lastImportRoot = ""
	a.mu.Unlock()
	for _, d := range dirs {
		_ = os.RemoveAll(d)
	}
}

// replaceLastImportRoot 记录新的 ZIP 导入临时根目录，并删除上一个
// （构建进行中时跳过删除，避免影响正在读取的 dist）。
func (a *App) replaceLastImportRoot(root string) {
	a.mu.Lock()
	old := a.lastImportRoot
	a.lastImportRoot = root
	building := a.building
	a.mu.Unlock()
	if old != "" && old != root && !building {
		_ = os.RemoveAll(old)
		a.untrackTempDir(old)
	}
}

// copyFileSync 同步复制文件（对话框导出场景使用，无需取消支持）
func copyFileSync(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}

// ---------- ZIP 解压 ----------

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	if len(r.File) > maxZipFiles {
		return fmt.Errorf("压缩包文件数超过上限: %d > %d", len(r.File), maxZipFiles)
	}
	var declared uint64
	for _, f := range r.File {
		declared += f.UncompressedSize64
	}
	if declared > uint64(maxZipTotalSize) {
		return fmt.Errorf("压缩包解压后大小超过上限: %s > %s", formatBytes(declared), formatBytes(uint64(maxZipTotalSize)))
	}

	var extracted int64
	for _, f := range r.File {
		// Reject absolute paths and path traversal (Zip Slip).
		name := filepath.ToSlash(f.Name)
		name = strings.TrimPrefix(name, "./")
		if name == "" || !filepath.IsLocal(filepath.FromSlash(name)) {
			return fmt.Errorf("非法的 zip 路径: %s", f.Name)
		}

		fpath := filepath.Join(dest, filepath.FromSlash(name))
		rel, err := filepath.Rel(dest, fpath)
		if err != nil || !filepath.IsLocal(rel) {
			return fmt.Errorf("非法的 zip 路径: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(fpath, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(fpath), 0755); err != nil {
			return err
		}
		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}
		n, copyErr := io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
		if copyErr != nil {
			return copyErr
		}
		// 实际解压字节数校验，防止中央目录声明的 size 不可信。
		extracted += n
		if extracted > maxZipTotalSize {
			return fmt.Errorf("压缩包解压后大小超过上限")
		}
	}
	return nil
}

func formatBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
