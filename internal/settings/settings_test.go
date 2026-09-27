package settings

import (
	"os"
	"path/filepath"
	"testing"

	"deploy-app/internal/appconf"
)

func withTempSettingsDir(t *testing.T) {
	t.Helper()
	old := settingsBaseDir
	settingsBaseDir = t.TempDir()
	t.Cleanup(func() { settingsBaseDir = old })
}

func TestSanitizeProfileName(t *testing.T) {
	if _, err := SanitizeProfileName("  管理后台 v1 "); err != nil {
		t.Fatalf("expected valid name, got %v", err)
	}
	for _, name := range []string{"", "a/b", "a\\b", "a:b", "CON", "..", "a..b", "x."} {
		if _, err := SanitizeProfileName(name); err == nil {
			t.Fatalf("expected error for %q", name)
		}
	}
}

func TestLastConfigRoundtrip(t *testing.T) {
	withTempSettingsDir(t)

	// 文件不存在时返回零值，不报错
	empty, err := LoadLastConfig()
	if err != nil {
		t.Fatal(err)
	}
	if empty.AppName != "" {
		t.Fatalf("expected zero config, got %+v", empty)
	}

	config := appconf.BuildConfig{AppName: "MyApp", WindowWidth: 800, ProxyRules: []appconf.ProxyRule{{Path: "/api/", Target: "http://x", Enabled: true}}}
	if err := SaveLastConfig(config); err != nil {
		t.Fatal(err)
	}
	got, err := LoadLastConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.AppName != "MyApp" || got.WindowWidth != 800 || len(got.ProxyRules) != 1 {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestProfileRoundtrip(t *testing.T) {
	withTempSettingsDir(t)

	config := appconf.BuildConfig{AppName: "MyApp", Version: "1.2.3"}
	clean, err := SaveProfile("管理后台 v1", config)
	if err != nil {
		t.Fatal(err)
	}
	if clean != "管理后台 v1" {
		t.Fatalf("unexpected clean name: %q", clean)
	}

	names, err := ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 || names[0] != "管理后台 v1" {
		t.Fatalf("unexpected profiles: %v", names)
	}

	got, err := LoadProfile("管理后台 v1")
	if err != nil {
		t.Fatal(err)
	}
	if got.AppName != "MyApp" || got.Version != "1.2.3" {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}

	if err := DeleteProfile("管理后台 v1"); err != nil {
		t.Fatal(err)
	}
	names, _ = ListProfiles()
	if len(names) != 0 {
		t.Fatalf("expected empty profiles, got %v", names)
	}
	// 删除不存在的方案视为成功
	if err := DeleteProfile("管理后台 v1"); err != nil {
		t.Fatal(err)
	}
}

func TestProfileFilePlacement(t *testing.T) {
	withTempSettingsDir(t)
	if _, err := SaveProfile("demo", appconf.BuildConfig{AppName: "demo"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(settingsBaseDir, "profiles", "demo.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
