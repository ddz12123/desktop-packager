package appconf

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeAppName(t *testing.T) {
	ok, err := SanitizeAppName("MyApp")
	if err != nil || ok != "MyApp" {
		t.Fatalf("expected MyApp, got %q err=%v", ok, err)
	}
	if _, err := SanitizeAppName("../evil"); err == nil {
		t.Fatal("expected error for path traversal")
	}
	if _, err := SanitizeAppName("CON"); err == nil {
		t.Fatal("expected error for reserved name")
	}
	if _, err := SanitizeAppName("CON.txt"); err == nil {
		t.Fatal("expected error for reserved name with extension")
	}
	if _, err := SanitizeAppName("bad:name"); err == nil {
		t.Fatal("expected error for invalid char")
	}
	if _, err := SanitizeAppName(""); err == nil {
		t.Fatal("expected error for empty")
	}
}

func TestIsWindowsReservedName(t *testing.T) {
	for _, name := range []string{"CON", "con", "NUL.txt", "com1"} {
		if !IsWindowsReservedName(name) {
			t.Fatalf("expected %q to be reserved", name)
		}
	}
	for _, name := range []string{"MyApp", "Console", "CONX"} {
		if IsWindowsReservedName(name) {
			t.Fatalf("expected %q not to be reserved", name)
		}
	}
}

func TestValidateProxyRule(t *testing.T) {
	err := ValidateProxyRule(ProxyRule{Path: "/api/", Target: "http://localhost:8080/", Enabled: true}, 0)
	if err != nil {
		t.Fatal(err)
	}
	err = ValidateProxyRule(ProxyRule{Path: "/api/", Target: "ftp://x", Enabled: true}, 0)
	if err == nil {
		t.Fatal("expected scheme error")
	}
	err = ValidateProxyRule(ProxyRule{Path: "/api/", Target: "http://localhost:8080/", Rewrite: "v2", Enabled: true}, 0)
	if err == nil {
		t.Fatal("expected rewrite must start with /")
	}
}

func TestValidateBuildConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	base := BuildConfig{
		AppName:    "MyApp",
		DistPath:   dir,
		OutputPath: filepath.Join(t.TempDir(), "MyApp.exe"),
	}
	if err := ValidateBuildConfig(base); err != nil {
		t.Fatal(err)
	}

	noOut := base
	noOut.OutputPath = ""
	if err := ValidateBuildConfig(noOut); err == nil {
		t.Fatal("expected missing output path error")
	}

	badExt := base
	badExt.OutputPath = filepath.Join(t.TempDir(), "MyApp.zip")
	if err := ValidateBuildConfig(badExt); err == nil {
		t.Fatal("expected exe extension error")
	}

	noDist := base
	noDist.DistPath = ""
	if err := ValidateBuildConfig(noDist); err == nil {
		t.Fatal("expected missing dist error")
	}
}

func TestBuildAppConfigJSON(t *testing.T) {
	data, err := BuildAppConfigJSON(BuildConfig{
		WindowWidth:  800,
		WindowHeight: 600,
		ConfirmClose: true,
		Version:      "9.9.9",
		Description:  "should not appear",
		Company:      "should not appear",
	})
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg["width"] != float64(800) || cfg["height"] != float64(600) {
		t.Fatalf("unexpected window size: %v", cfg)
	}
	for _, key := range []string{"version", "description", "company"} {
		if _, ok := cfg[key]; ok {
			t.Fatalf("%s should not be written to app_config.json", key)
		}
	}
}

func TestBuildProxyConfigJSON_Empty(t *testing.T) {
	data, err := BuildProxyConfigJSON(nil)
	if err != nil {
		t.Fatal(err)
	}
	var cfg ProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Rules == nil || len(cfg.Rules) != 0 {
		t.Fatalf("expected empty rules, got %v", cfg.Rules)
	}
}

func TestBuildProxyConfigJSON_Normalizes(t *testing.T) {
	// 规范化以本函数为唯一入口：无前导 / 的路径自动补全
	data, err := BuildProxyConfigJSON([]ProxyRule{{Path: "api/", Target: " http://x ", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	var cfg ProxyConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Rules[0].Path != "/api/" || cfg.Rules[0].Target != "http://x" {
		t.Fatalf("normalize mismatch: %+v", cfg.Rules[0])
	}
}
