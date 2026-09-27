// Package settings 负责打包工具自身的持久化：上次构建配置与命名构建方案。
// 存储位置为 %AppData%\deploy-app（os.UserConfigDir）。
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"deploy-app/internal/appconf"
)

// settingsBaseDir 配置持久化根目录，测试时可替换为临时目录。
var settingsBaseDir = defaultSettingsBaseDir()

func defaultSettingsBaseDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return "."
	}
	return filepath.Join(base, "deploy-app")
}

func settingsDir() string { return settingsBaseDir }

// ProfilesDir 返回方案存储目录，供导出等需要直接定位方案文件的场景使用。
func ProfilesDir() string { return filepath.Join(settingsBaseDir, "profiles") }

// ProfilePath 返回指定方案的存储路径（名称经校验）。
func ProfilePath(name string) (string, error) {
	clean, err := SanitizeProfileName(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(ProfilesDir(), clean+".json"), nil
}

var invalidProfileNameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

// SanitizeProfileName 校验方案名称，并返回可直接用作文件名的干净名称（不含扩展名）。
func SanitizeProfileName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("方案名称不能为空")
	}
	if utf8.RuneCountInString(name) > 60 {
		return "", fmt.Errorf("方案名称长度不能超过 60 个字符")
	}
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("方案名称不能包含 ..")
	}
	if invalidProfileNameChars.MatchString(name) {
		return "", fmt.Errorf("方案名称包含非法字符（不能包含 <>:\"/\\|?* 和控制字符）")
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", fmt.Errorf("方案名称不能以空格或点结尾")
	}
	if appconf.IsWindowsReservedName(name) {
		return "", fmt.Errorf("方案名称不能使用 Windows 保留名: %s", name)
	}
	return name, nil
}

func saveJSONFile(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// SaveLastConfig 保存上次构建配置，用于下次启动恢复。
func SaveLastConfig(config appconf.BuildConfig) error {
	return saveJSONFile(filepath.Join(settingsDir(), "last_config.json"), config)
}

// LoadLastConfig 读取上次构建配置；文件不存在时返回零值与 nil。
func LoadLastConfig() (appconf.BuildConfig, error) {
	return appconf.ReadConfigFile(filepath.Join(settingsDir(), "last_config.json"))
}

// SaveProfile 保存命名方案，返回清洗后的方案名。
func SaveProfile(name string, config appconf.BuildConfig) (string, error) {
	clean, err := SanitizeProfileName(name)
	if err != nil {
		return "", err
	}
	if err := saveJSONFile(filepath.Join(ProfilesDir(), clean+".json"), config); err != nil {
		return "", err
	}
	return clean, nil
}

// ListProfiles 返回全部方案名（按名称排序）。
func ListProfiles() ([]string, error) {
	entries, err := os.ReadDir(ProfilesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if !strings.EqualFold(ext, ".json") {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ext))
	}
	sort.Strings(names)
	return names, nil
}

// LoadProfile 读取指定方案。
func LoadProfile(name string) (appconf.BuildConfig, error) {
	path, err := ProfilePath(name)
	if err != nil {
		return appconf.BuildConfig{}, err
	}
	return appconf.ReadConfigFile(path)
}

// DeleteProfile 删除指定方案；方案不存在视为成功。
func DeleteProfile(name string) error {
	path, err := ProfilePath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
