// Package buildkit 实现把前端构建产物打包为 Windows 桌面应用的完整管线：
// 复制预编译运行壳 → 修补图标/版本 → 追加资源 zip 与 footer → 输出 → 签名脚本。
// 本包不依赖 Wails：进度通过 ProgressFunc 回调上报，取消通过 ctx 传递，
// GUI 与 CLI 共用同一套实现。
package buildkit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"deploy-app/internal/appconf"
	"deploy-app/internal/shellinfo"

	"golang.org/x/sys/windows"
)

// Options 打包所需的运行壳资产与校验数据。
// go:embed 与所在包路径绑定，因此嵌入内容由调用方（根包）持有并在这里注入。
type Options struct {
	BaseExe       []byte // 预编译运行壳 base.exe
	ShellManifest []byte // base_version.txt 内容（源文件 SHA256 清单）
	ShellHashes   map[string]string
	// OnTempDir 在创建构建工作目录后回调，供调用方登记以便异常退出时清理；可为 nil。
	OnTempDir func(dir string)
}

// ProgressFunc 在每个构建阶段被调用，percent 取值 0-100。
type ProgressFunc func(step string, percent int)

// Build 执行完整打包流程，返回输出 exe 路径与（配置了签名时的）签名脚本路径。
// 校验失败、运行壳过期、文件占用等错误信息均已面向最终用户措辞。
func Build(ctx context.Context, config appconf.BuildConfig, opts Options, progress ProgressFunc) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	progress = clampProgress(progress)

	appName, err := appconf.SanitizeAppName(config.AppName)
	if err != nil {
		return "", "", err
	}
	config.AppName = appName

	if err := appconf.ValidateBuildConfig(config); err != nil {
		return "", "", err
	}

	progress("准备工作目录", 5)
	tempBase, err := ResolveTempBase(config.TempPath, filepath.Dir(config.DistPath))
	if err != nil {
		return "", "", err
	}
	tempDir, err := os.MkdirTemp(tempBase, "deploy-build-*")
	if err != nil {
		return "", "", fmt.Errorf("创建工作目录失败: %w", err)
	}
	if opts.OnTempDir != nil {
		opts.OnTempDir(tempDir)
	}
	defer os.RemoveAll(tempDir)

	progress("校验运行壳", 15)
	if err := CheckShellFresh(opts.ShellManifest, opts.ShellHashes); err != nil {
		return "", "", err
	}

	progress("复制基础程序", 20)
	if len(opts.BaseExe) < 1024 || opts.BaseExe[0] != 'M' || opts.BaseExe[1] != 'Z' {
		return "", "", fmt.Errorf("内置 base.exe 无效或仍是占位文件，请先执行: go run ./cmd/build-base")
	}
	exePath := filepath.Join(tempDir, config.AppName+".exe")
	if err := os.WriteFile(exePath, opts.BaseExe, 0755); err != nil {
		return "", "", fmt.Errorf("复制基础程序失败: %w", err)
	}

	// 图标/版本修补必须在追加资源之前（PE 资源段重写会破坏尾部追加的数据）。
	progress("修补应用图标与版本信息", 35)
	if err := patchIcon(exePath, config); err != nil {
		return "", "", fmt.Errorf("修补图标失败: %w", err)
	}

	progress("打包前端资源", 55)
	if err := appendResources(ctx, exePath, config, progress); err != nil {
		return "", "", fmt.Errorf("打包资源失败: %w", err)
	}

	progress("保存文件", 85)
	finalPath, err := saveOutput(ctx, exePath, config.OutputPath)
	if err != nil {
		return "", "", fmt.Errorf("输出文件失败: %w", err)
	}

	signScript := ""
	if strings.TrimSpace(config.SignPfxPath) != "" {
		progress("生成签名脚本", 90)
		signScript, err = writeSignScript(config, finalPath)
		if err != nil {
			return "", "", err
		}
	}

	progress("构建完成!", 100)
	return finalPath, signScript, nil
}

// CheckShellFresh 比较运行壳构建时记录的源文件哈希清单与当前源文件哈希，
// 防止模板或共享算法修改后仍使用过期的运行壳。
func CheckShellFresh(manifest []byte, current map[string]string) error {
	var recorded shellinfo.Manifest
	if err := json.Unmarshal(manifest, &recorded); err != nil || len(recorded.Files) == 0 {
		return fmt.Errorf("base.exe 版本信息缺失或损坏，请执行: go run ./cmd/build-base")
	}
	var changed []string
	for name, hash := range current {
		if recorded.Files[name] != hash {
			changed = append(changed, name)
		}
	}
	for name := range recorded.Files {
		if _, ok := current[name]; !ok {
			changed = append(changed, name)
		}
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		return fmt.Errorf("以下源文件在 base.exe 生成后已修改，请重新执行: go run ./cmd/build-base（%s）", strings.Join(changed, ", "))
	}
	return nil
}

// ResolveTempBase 决定临时目录的父目录。
// 用户显式指定的目录失败时直接报错；隐式默认目录（如 ZIP 所在目录，
// 可能位于只读介质）失败时回退到系统临时目录。
func ResolveTempBase(explicitPath, implicitPath string) (string, error) {
	explicit := strings.TrimSpace(explicitPath)
	if explicit != "" {
		if err := os.MkdirAll(explicit, 0755); err != nil {
			return "", fmt.Errorf("创建临时目录失败: %w", err)
		}
		return explicit, nil
	}
	base := implicitPath
	if err := os.MkdirAll(base, 0755); err != nil {
		base = os.TempDir()
		if err := os.MkdirAll(base, 0755); err != nil {
			return "", fmt.Errorf("创建临时目录失败: %w", err)
		}
	}
	return base, nil
}

func clampProgress(progress ProgressFunc) ProgressFunc {
	if progress == nil {
		return func(string, int) {}
	}
	return func(step string, percent int) {
		if percent < 0 {
			percent = 0
		}
		if percent > 100 {
			percent = 100
		}
		progress(step, percent)
	}
}

func saveOutput(ctx context.Context, srcPath, outputPath string) (string, error) {
	outputPath = strings.TrimSpace(outputPath)
	if outputPath == "" {
		return "", fmt.Errorf("请先选择保存位置")
	}
	// 先写同目录临时文件、成功后原子改名：取消或失败不会在目标路径留下截断的 exe。
	tmpPath := outputPath + ".tmp"
	if err := copyFile(ctx, srcPath, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		if isFileBusy(err) {
			return "", fmt.Errorf("输出文件可能正在运行或被其他程序占用，请先关闭它后重试")
		}
		return "", err
	}
	if err := os.Rename(tmpPath, outputPath); err != nil {
		_ = os.Remove(tmpPath)
		if isFileBusy(err) {
			return "", fmt.Errorf("输出文件可能正在运行或被其他程序占用，请先关闭它后重试")
		}
		return "", err
	}
	return outputPath, nil
}

// isFileBusy 判断是否为 Windows 文件占用（分享冲突）类错误，常见于覆盖一个正在运行的 exe。
func isFileBusy(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == windows.ERROR_SHARING_VIOLATION || errno == windows.ERROR_LOCK_VIOLATION
	}
	return false
}

// ctxReader 在构建被取消时中断读取
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (c *ctxReader) Read(p []byte) (int, error) {
	if err := c.ctx.Err(); err != nil {
		return 0, err
	}
	return c.r.Read(p)
}

func copyFile(ctx context.Context, src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), os.ModePerm); err != nil {
		return err
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, &ctxReader{ctx: ctx, r: in})
	return err
}
