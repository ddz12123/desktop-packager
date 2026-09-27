package buildkit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"deploy-app/internal/appconf"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// locateSigntool 查找 signtool.exe：优先 PATH，其次 Windows SDK 常见安装位置（取版本号最新的）。
// 仅做路径查找，不执行任何程序。
func locateSigntool() (string, error) {
	if p, err := exec.LookPath("signtool.exe"); err == nil {
		return p, nil
	}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		root := os.Getenv(env)
		if root == "" {
			continue
		}
		pattern := filepath.Join(root, "Windows Kits", "10", "bin", "*", "x64", "signtool.exe")
		matches, _ := filepath.Glob(pattern)
		if len(matches) == 0 {
			continue
		}
		sort.Strings(matches)
		return matches[len(matches)-1], nil
	}
	return "", fmt.Errorf("未找到 signtool.exe，请安装 Windows SDK 或将其加入 PATH 后重试")
}

// writeSignScript 在输出 exe 旁生成 <输出 exe 名>-sign.cmd 签名脚本。
// 出于安全考虑，工具不在进程内直接调用 signtool（证书密码等敏感参数不经过子进程），
// 而是生成脚本由用户手动运行：密码在运行时输入，不落盘。
// 找不到 signtool 时不视为构建失败，脚本退化为运行时依赖 PATH 查找。
func writeSignScript(config appconf.BuildConfig, exePath string) (string, error) {
	signtoolPath, err := locateSigntool()
	if err != nil {
		signtoolPath = "signtool.exe"
	}
	scriptPath := strings.TrimSuffix(exePath, ".exe") + "-sign.cmd"

	ts := strings.TrimSpace(config.SignTimestamp)
	var sb strings.Builder
	sb.WriteString("@echo off\r\n")
	sb.WriteString("setlocal EnableDelayedExpansion\r\n")
	sb.WriteString("rem 本脚本由 Deploy App 生成，用于对 exe 做 Authenticode 签名，签名完成后可删除本文件。\r\n")
	sb.WriteString("rem 密码在运行时输入、不落盘；不支持包含英文感叹号(!)的密码，此类密码请手动执行 signtool。\r\n")
	sb.WriteString(fmt.Sprintf("set \"SIGNTOOL=%s\"\r\n", signtoolPath))
	sb.WriteString(fmt.Sprintf("set \"PFX=%s\"\r\n", config.SignPfxPath))
	sb.WriteString(fmt.Sprintf("set \"TARGET=%s\"\r\n", exePath))
	sb.WriteString(fmt.Sprintf("set \"TSURL=%s\"\r\n", ts))
	sb.WriteString("set \"TSARGS=\"\r\n")
	sb.WriteString("if defined TSURL set \"TSARGS=/tr \"!TSURL!\" /td SHA256\"\r\n")
	sb.WriteString("set /p \"PXP=请输入证书密码（无密码直接回车）: \"\r\n")
	sb.WriteString("set \"PWDARGS=/p !PXP!\"\r\n")
	sb.WriteString("if not defined PXP set \"PWDARGS=\"\r\n")
	sb.WriteString("\"!SIGNTOOL!\" sign /fd SHA256 /f \"!PFX!\" !PWDARGS! !TSARGS! \"!TARGET!\"\r\n")
	sb.WriteString("endlocal\r\n")
	sb.WriteString("pause\r\n")

	// cmd 按 ANSI 代码页（中文系统为 GBK）逐行解码批处理，UTF-8 中文会被错误配对，
	// 甚至吞掉换行符导致两行合并；因此优先以 GBK 编码落盘。
	// 路径含 GBK 无法表示的字符时，退化为 UTF-8 并让 cmd 切换到 UTF-8 代码页解析。
	content := sb.String()
	if encoded, err := simplifiedchinese.GBK.NewEncoder().String(content); err == nil {
		content = encoded
	} else {
		content = strings.Replace(content, "@echo off\r\n", "@echo off\r\nchcp 65001 >nul\r\n", 1)
	}
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("写入签名脚本失败: %w", err)
	}
	return scriptPath, nil
}
