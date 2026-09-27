// Package appconf 定义构建配置的数据结构、校验规则，以及写入生成应用的
// 运行时配置（proxy_config.json / app_config.json）的生成逻辑。
// 除 ReadConfigFile 与校验中的文件存在性检查外，不触碰文件系统，可独立测试。
package appconf

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"deploy-app/internal/nginxproxy"
)

// ProxyRule 反向代理规则（nginx location + proxy_pass 语义）
type ProxyRule struct {
	Path    string `json:"path"`    // location 前缀，例如 "/api/"
	Target  string `json:"target"`  // proxy_pass 目标，例如 "http://localhost:8080/"
	Rewrite string `json:"rewrite"` // 可选，覆盖 target 的 URI 替换前缀
	Enabled bool   `json:"enabled"`
}

// BuildConfig 构建配置
type BuildConfig struct {
	AppName          string      `json:"appName"`
	IconPath         string      `json:"iconPath"`
	DistPath         string      `json:"distPath"`
	OutputPath       string      `json:"outputPath"` // 构建前选择的保存位置
	TempPath         string      `json:"tempPath"`
	ProxyRules       []ProxyRule `json:"proxyRules"`
	WindowWidth      int         `json:"windowWidth"`
	WindowHeight     int         `json:"windowHeight"`
	WindowFullscreen bool        `json:"windowFullscreen"`
	WindowMaximized  bool        `json:"windowMaximized"`
	ConfirmClose     bool        `json:"confirmClose"`
	Version          string      `json:"version"`
	Description      string      `json:"description"`
	Company          string      `json:"company"`

	WindowTitle    string `json:"windowTitle"`    // 窗口标题，留空使用应用名
	SingleInstance bool   `json:"singleInstance"` // 生成应用单实例锁
	RememberWindow bool   `json:"rememberWindow"` // 生成应用记住窗口位置/大小
	TargetPlatform string `json:"targetPlatform"` // 发布平台，当前仅支持 "windows"，留空视为 windows

	SignPfxPath   string `json:"signPfxPath"`   // 代码签名证书，留空则不生成签名脚本
	SignTimestamp string `json:"signTimestamp"` // RFC3161 时间戳服务器，留空则不加时间戳
}

// ProxyConfig 写入 proxy_config.json
type ProxyConfig struct {
	Rules []ProxyRule `json:"rules"`
}

// AppRuntimeConfig 写入 app_config.json。
// 版本/描述/公司只写入 PE 版本资源，不在这里重复。
type AppRuntimeConfig struct {
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	Fullscreen     bool   `json:"fullscreen"`
	Maximized      bool   `json:"maximized"`
	ConfirmClose   bool   `json:"confirmClose"`
	Title          string `json:"title"`          // 窗口标题，空串表示使用 exe 名
	SingleInstance bool   `json:"singleInstance"` // 单实例锁
	RememberWindow bool   `json:"rememberWindow"` // 记住窗口位置/大小
}

// DistInfo 构建产物信息
type DistInfo struct {
	Path          string `json:"path"`
	FileCount     int    `json:"fileCount"`
	TotalSize     int64  `json:"totalSize"`
	Valid         bool   `json:"valid"`
	SuggestedName string `json:"suggestedName"` // 从同级 package.json 推断的默认应用名
}

var (
	invalidAppNameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)
	windowsReserved     = map[string]struct{}{
		"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
		"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {}, "COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
		"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {}, "LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
	}
	semverLike = regexp.MustCompile(`^\d+(\.\d+){0,3}$`)
)

// IsWindowsReservedName 报告 name 是否命中 Windows 保留设备名
// （含 "CON.txt" 这类带扩展名前缀的形式），供文件命名类校验复用。
func IsWindowsReservedName(name string) bool {
	upper := strings.ToUpper(name)
	if _, ok := windowsReserved[upper]; ok {
		return true
	}
	if dot := strings.IndexByte(upper, '.'); dot > 0 {
		if _, ok := windowsReserved[upper[:dot]]; ok {
			return true
		}
	}
	return false
}

// SanitizeAppName validates and normalizes a Windows-safe application name.
func SanitizeAppName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("应用名称不能为空")
	}
	if utf8.RuneCountInString(name) > 50 {
		return "", fmt.Errorf("应用名称长度不能超过 50 个字符")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("应用名称不能包含 ..")
	}
	if invalidAppNameChars.MatchString(name) {
		return "", fmt.Errorf("应用名称包含非法字符（不能包含 <>:\"/\\|?* 和控制字符）")
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", fmt.Errorf("应用名称不能以空格或点结尾")
	}
	if IsWindowsReservedName(name) {
		return "", fmt.Errorf("应用名称不能使用 Windows 保留名: %s", name)
	}
	return name, nil
}

// NormalizeTargetPlatform 规范化发布平台标识；空值视为 windows。
func NormalizeTargetPlatform(p string) string {
	p = strings.ToLower(strings.TrimSpace(p))
	if p == "" {
		return "windows"
	}
	return p
}

// ValidateProxyRule validates a single proxy rule using nginx-compatible expectations.
func ValidateProxyRule(rule ProxyRule, index int) error {
	prefix := fmt.Sprintf("代理规则 #%d", index+1)
	if !rule.Enabled {
		return nil
	}
	path := nginxproxy.NormalizeLocation(rule.Path)
	if path == "" {
		return fmt.Errorf("%s: 路径前缀不能为空", prefix)
	}
	if strings.Contains(path, "..") {
		return fmt.Errorf("%s: 路径前缀非法", prefix)
	}
	target := strings.TrimSpace(rule.Target)
	if target == "" {
		return fmt.Errorf("%s: 目标地址不能为空", prefix)
	}
	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("%s: 目标地址无效，需形如 http://host:port/ 或 https://host", prefix)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("%s: 目标地址仅支持 http/https", prefix)
	}
	if rw := strings.TrimSpace(rule.Rewrite); rw != "" {
		if !strings.HasPrefix(rw, "/") {
			return fmt.Errorf("%s: 重写路径应以 / 开头", prefix)
		}
		if strings.Contains(rw, "..") {
			return fmt.Errorf("%s: 重写路径非法", prefix)
		}
	}
	return nil
}

// ValidateBuildConfig validates the full build configuration before packaging.
// 应用名由调用方先行 sanitize，这里不再重复校验。
func ValidateBuildConfig(config BuildConfig) error {
	dist := strings.TrimSpace(config.DistPath)
	if dist == "" {
		return fmt.Errorf("请先导入前端构建产物")
	}
	info, err := os.Stat(dist)
	if err != nil {
		return fmt.Errorf("构建产物目录无效: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("构建产物路径不是目录")
	}
	if _, err := os.Stat(filepath.Join(dist, "index.html")); err != nil {
		return fmt.Errorf("构建产物目录中未找到 index.html")
	}

	out := strings.TrimSpace(config.OutputPath)
	if out == "" {
		return fmt.Errorf("请先选择保存位置")
	}
	if !strings.EqualFold(filepath.Ext(out), ".exe") {
		return fmt.Errorf("输出文件名需以 .exe 结尾")
	}

	if NormalizeTargetPlatform(config.TargetPlatform) != "windows" {
		return fmt.Errorf("发布平台 %s 暂不支持，当前仅支持 Windows（macOS/Linux 即将支持）", config.TargetPlatform)
	}

	if config.IconPath != "" {
		if _, err := os.Stat(config.IconPath); err != nil {
			return fmt.Errorf("图标文件无效: %w", err)
		}
		ext := strings.ToLower(filepath.Ext(config.IconPath))
		if ext != ".ico" && ext != ".png" {
			return fmt.Errorf("图标仅支持 .ico 或 .png")
		}
	}

	if config.TempPath != "" {
		if err := os.MkdirAll(config.TempPath, 0755); err != nil {
			return fmt.Errorf("临时目录不可用: %w", err)
		}
	}

	if config.WindowWidth != 0 && (config.WindowWidth < 400 || config.WindowWidth > 3840) {
		return fmt.Errorf("窗口宽度需在 400-3840 之间")
	}
	if config.WindowHeight != 0 && (config.WindowHeight < 300 || config.WindowHeight > 2160) {
		return fmt.Errorf("窗口高度需在 300-2160 之间")
	}

	if pfx := strings.TrimSpace(config.SignPfxPath); pfx != "" {
		info, err := os.Stat(pfx)
		if err != nil {
			return fmt.Errorf("签名证书文件无效: %w", err)
		}
		if info.IsDir() || !strings.EqualFold(filepath.Ext(pfx), ".pfx") {
			return fmt.Errorf("签名证书需为 .pfx 文件")
		}
	}
	if ts := strings.TrimSpace(config.SignTimestamp); ts != "" {
		u, err := url.Parse(ts)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			return fmt.Errorf("时间戳服务器需为 http/https 地址，示例: http://timestamp.digicert.com")
		}
	}

	for i, rule := range config.ProxyRules {
		if err := ValidateProxyRule(rule, i); err != nil {
			return err
		}
	}

	if v := strings.TrimSpace(config.Version); v != "" {
		if !semverLike.MatchString(v) {
			return fmt.Errorf("版本号格式无效，示例: 1.0.0")
		}
		// PE 版本资源每段是 uint16，超限会被静默截断，必须在入口拒绝
		for _, part := range strings.Split(v, ".") {
			if n, err := strconv.Atoi(part); err != nil || n > 65535 {
				return fmt.Errorf("版本号每段需在 0-65535 之间: %s", v)
			}
		}
	}
	return nil
}

// BuildProxyConfigJSON 生成写入生成应用的 proxy_config.json。
// 唯一的规范化入口：nginx 语义要求路径前缀以 / 开头，此处幂等处理，
// 调用方无需（也不应）预先 normalize。
func BuildProxyConfigJSON(rules []ProxyRule) ([]byte, error) {
	if rules == nil {
		rules = []ProxyRule{}
	}
	normalized := make([]ProxyRule, 0, len(rules))
	for _, r := range rules {
		r.Path = nginxproxy.NormalizeLocation(r.Path)
		r.Target = strings.TrimSpace(r.Target)
		r.Rewrite = strings.TrimSpace(r.Rewrite)
		normalized = append(normalized, r)
	}
	cfg := ProxyConfig{Rules: normalized}
	return json.MarshalIndent(cfg, "", "  ")
}

// BuildAppConfigJSON 生成写入生成应用的 app_config.json。
func BuildAppConfigJSON(config BuildConfig) ([]byte, error) {
	width := config.WindowWidth
	if width < 400 || width > 3840 {
		width = 1024
	}
	height := config.WindowHeight
	if height < 300 || height > 2160 {
		height = 768
	}
	cfg := AppRuntimeConfig{
		Width:          width,
		Height:         height,
		Fullscreen:     config.WindowFullscreen,
		Maximized:      config.WindowMaximized,
		ConfirmClose:   config.ConfirmClose,
		Title:          strings.TrimSpace(config.WindowTitle),
		SingleInstance: config.SingleInstance,
		RememberWindow: config.RememberWindow,
	}
	return json.MarshalIndent(cfg, "", "  ")
}

// ReadConfigFile 读取一个 BuildConfig JSON 文件；文件不存在返回零值与 nil。
func ReadConfigFile(path string) (BuildConfig, error) {
	var config BuildConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("配置文件格式无效: %w", err)
	}
	return config, nil
}
