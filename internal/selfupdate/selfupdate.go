// Package selfupdate 实现打包工具自身的检查更新与在线更新。
// 更新源为 GitHub Releases（latest）；更新通过生成的脚本完成：
// 应用退出后由脚本替换 exe 并自动重启。
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// releasesAPI 最新版本查询接口
const releasesAPI = "https://api.github.com/repos/ddz12123/desktop-packager/releases/latest"

// releasesDownloadPrefix 官方发布产物的下载地址前缀，Apply 用它做第一道校验
const releasesDownloadPrefix = "https://github.com/ddz12123/desktop-packager/releases/download/"

// Info 检查更新的结果
type Info struct {
	HasUpdate      bool   `json:"hasUpdate"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	NotesURL       string `json:"notesUrl"`
	DownloadURL    string `json:"downloadUrl"`
}

// CompareVersions 比较两个点分版本号（忽略前导 v）：a>b 返回 1，相等返回 0，a<b 返回 -1。
func CompareVersions(a, b string) int {
	pa, pb := splitVersion(a), splitVersion(b)
	for i := 0; i < 4; i++ {
		if pa[i] > pb[i] {
			return 1
		}
		if pa[i] < pb[i] {
			return -1
		}
	}
	return 0
}

func splitVersion(v string) [4]int {
	var out [4]int
	v = strings.TrimPrefix(strings.TrimSpace(strings.ToLower(v)), "v")
	parts := strings.Split(v, ".")
	for i := 0; i < len(parts) && i < 4; i++ {
		if n, err := strconv.Atoi(strings.TrimSpace(parts[i])); err == nil && n > 0 {
			out[i] = n
		}
	}
	return out
}

// Check 查询最新 Release 并与当前版本比较。
func Check(currentVersion string) (*Info, error) {
	info, err := fetchLatest()
	if err != nil {
		return nil, err
	}
	info.CurrentVersion = currentVersion
	info.HasUpdate = CompareVersions(info.LatestVersion, currentVersion) > 0
	return info, nil
}

func fetchLatest() (*Info, error) {
	req, err := http.NewRequest(http.MethodGet, releasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "deploy-app")
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接更新服务器: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("仓库还没有发布过版本")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("检查更新失败（HTTP %d）", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name               string `json:"name"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("解析更新信息失败: %w", err)
	}

	info := &Info{
		LatestVersion: strings.TrimPrefix(release.TagName, "v"),
		NotesURL:      release.HTMLURL,
	}
	for _, a := range release.Assets {
		name := strings.ToLower(a.Name)
		if strings.HasPrefix(name, "deploy-app-v") && strings.HasSuffix(name, ".exe") {
			info.DownloadURL = a.BrowserDownloadURL
			break
		}
	}
	if info.DownloadURL == "" {
		return nil, fmt.Errorf("最新版本 %s 没有可下载的 Windows 安装包", release.TagName)
	}
	return info, nil
}

// Apply 下载新版本到 exe 同目录的临时文件，生成更新脚本并返回脚本路径。
// 下载地址先校验前缀、再与最新 Release 的资源精确比对，防止从任意地址下载。
func Apply(ctx context.Context, downloadURL, exePath string) (string, error) {
	if !strings.HasPrefix(downloadURL, releasesDownloadPrefix) {
		return "", fmt.Errorf("下载地址无效，请重新检查更新")
	}
	info, err := fetchLatest()
	if err != nil {
		return "", err
	}
	if downloadURL != info.DownloadURL {
		return "", fmt.Errorf("下载地址无效，请重新检查更新")
	}

	tmpPath := exePath + ".update"
	if err := download(ctx, downloadURL, tmpPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	scriptPath, err := writeUpdateScript(exePath, tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}
	return scriptPath, nil
}

func download(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "deploy-app")
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败（HTTP %d）", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	n, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return fmt.Errorf("下载失败: %w", copyErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if n < 1<<20 {
		return fmt.Errorf("下载的文件不完整，请重试")
	}

	// 基本完整性校验：必须是 Windows 可执行文件
	f, err := os.Open(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	header := make([]byte, 2)
	if _, err := io.ReadFull(f, header); err != nil || string(header) != "MZ" {
		return fmt.Errorf("下载的文件不是有效的 Windows 程序")
	}
	return nil
}

// writeUpdateScript 生成 <exe名>-update.cmd：等待应用退出 → 替换 exe → 重启 → 自删。
func writeUpdateScript(exePath, tmpPath string) (string, error) {
	scriptPath := strings.TrimSuffix(exePath, ".exe") + "-update.cmd"
	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	sb.WriteString("rem Deploy App update script, auto-deleted after applying.\r\n")
	sb.WriteString("timeout /t 2 /nobreak >nul\r\n")
	sb.WriteString(fmt.Sprintf("move /y \"%s\" \"%s\"\r\n", tmpPath, exePath))
	sb.WriteString(fmt.Sprintf("start \"\" \"%s\"\r\n", exePath))
	sb.WriteString("del \"%~f0\"\r\n")

	// cmd 按 ANSI 代码页（中文系统为 GBK）解码批处理，路径含中文时必须 GBK 编码
	content := sb.String()
	if encoded, err := simplifiedchinese.GBK.NewEncoder().String(content); err == nil {
		content = encoded
	}
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入更新脚本失败: %w", err)
	}
	return scriptPath, nil
}

// ScriptDir 返回更新脚本所在目录（即 exe 目录），供日志提示使用。
func ScriptDir(exePath string) string {
	return filepath.Dir(exePath)
}
