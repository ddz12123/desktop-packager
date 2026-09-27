package buildkit

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"deploy-app/internal/appconf"
	"deploy-app/internal/shellinfo"
)

func TestCheckShellFresh(t *testing.T) {
	hashes := map[string]string{
		"templates/generated-app/main.go.tmpl": "aaa",
		"internal/nginxproxy/path.go":          "bbb",
	}
	good, err := json.Marshal(shellinfo.Manifest{Files: hashes})
	if err != nil {
		t.Fatal(err)
	}

	// 清单与当前哈希一致 → 通过
	if err := CheckShellFresh(good, hashes); err != nil {
		t.Fatalf("expected fresh shell to pass, got %v", err)
	}

	// 任一源文件哈希不一致 → 报漂移错误
	drifted := map[string]string{"templates/generated-app/main.go.tmpl": "aaa", "internal/nginxproxy/path.go": "zzz"}
	if err := CheckShellFresh(good, drifted); err == nil || !strings.Contains(err.Error(), "path.go") {
		t.Fatalf("expected drift error mentioning path.go, got %v", err)
	}

	// 清单中多出的文件也视为漂移
	extra, _ := json.Marshal(shellinfo.Manifest{Files: map[string]string{"a": "1", "b": "2"}})
	if err := CheckShellFresh(extra, map[string]string{"a": "1"}); err == nil {
		t.Fatal("expected drift error for extra manifest entry")
	}

	// 清单损坏 → 报可操作的错误
	if err := CheckShellFresh([]byte("not json"), hashes); err == nil || !strings.Contains(err.Error(), "go run ./cmd/build-base") {
		t.Fatalf("expected actionable corrupt-record error, got %v", err)
	}
}

func TestWriteSignScript(t *testing.T) {
	if _, err := locateSigntool(); err != nil {
		t.Skip("signtool 不可用，跳过签名脚本测试")
	}
	dir := t.TempDir()
	exePath := filepath.Join(dir, "MyApp.exe")
	config := appconf.BuildConfig{
		SignPfxPath:   `C:\certs\my.pfx`,
		SignTimestamp: "http://timestamp.digicert.com",
	}
	scriptPath, err := writeSignScript(config, exePath)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(scriptPath) != "MyApp-sign.cmd" {
		t.Fatalf("unexpected script name: %s", scriptPath)
	}
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	for _, want := range []string{"my.pfx", "MyApp.exe", "/fd SHA256", "timestamp.digicert.com", "set /p"} {
		if !strings.Contains(content, want) {
			t.Fatalf("script missing %q", want)
		}
	}
	// 密码必须为运行时输入，脚本中不应出现字面密码
	if !strings.Contains(content, "set /p \"PXP=") {
		t.Fatal("script should prompt for the password at runtime")
	}
}

func TestBuildCanceledBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := Build(ctx, appconf.BuildConfig{}, Options{}, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
