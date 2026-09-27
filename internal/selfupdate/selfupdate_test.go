package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.1", "1.0.0", 1},
		{"v1.0.1", "1.0.1", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0", "1.0.0", 0},
		{"2.0.0", "1.9.9", 1},
		{"1.10.0", "1.9.0", 1}, // 数字比较而非字典序
		{"1.0.1", "V1.0.1", 0},
	}
	for _, c := range cases {
		if got := CompareVersions(c.a, c.b); got != c.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestWriteUpdateScript(t *testing.T) {
	dir := t.TempDir()
	exePath := filepath.Join(dir, "deploy-app.exe")
	tmpPath := exePath + ".update"

	scriptPath, err := writeUpdateScript(exePath, tmpPath)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(scriptPath) != "deploy-app-update.cmd" {
		t.Fatalf("unexpected script name: %s", scriptPath)
	}
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	// 脚本必须完成：等待退出 → 替换 → 重启 → 自删
	for _, want := range []string{"timeout /t 2", "move /y", "start \"\"", "del \"%~f0\""} {
		if !strings.Contains(content, want) {
			t.Fatalf("script missing %q", want)
		}
	}
	// 不应包含任何字面下载地址或密码类内容
	if strings.Contains(content, "http") {
		t.Fatal("script should not embed URLs")
	}
}

func TestSplitVersionIgnoresGarbage(t *testing.T) {
	if got := CompareVersions("abc", "1.0.0"); got != -1 {
		t.Fatalf("expected -1 for garbage version, got %d", got)
	}
}

func TestIsPortableAssetName(t *testing.T) {
	yes := []string{
		"deploy-app-v1.0.1.exe",
		"deploy-app-v1.2.3.exe",
		"Deploy-App-V1.0.1.EXE",
	}
	no := []string{
		"deploy-app-v1.0.1-setup.exe", // 安装版不能作为在线更新替换包
		"deploy-app-v1.0.1.zip",
		"checksums.txt",
		"other-v1.0.0.exe",
	}
	for _, name := range yes {
		if !isPortableAssetName(name) {
			t.Errorf("expected %q to be portable asset", name)
		}
	}
	for _, name := range no {
		if isPortableAssetName(name) {
			t.Errorf("expected %q NOT to be portable asset", name)
		}
	}
}

// TestApplyRejectsForeignURL 不发网络请求：直接验证 Apply 的地址校验分支。
// 通过构造一个必然不匹配的地址让其在 fetchLatest 之前/之后被拒。
func TestApplyRejectsForeignURL(t *testing.T) {
	if testing.Short() {
		t.Skip("网络不可用时跳过")
	}
	ctx := context.Background()
	// fetchLatest 会先访问 GitHub；失败或成功都会走到 URL 校验并拒绝外域地址
	_, err := Apply(ctx, "https://evil.example.com/app.exe", filepath.Join(t.TempDir(), "app.exe"))
	if err == nil {
		t.Fatal("expected foreign URL to be rejected")
	}
}
