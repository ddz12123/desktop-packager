package main

import (
	"testing"

	"deploy-app/internal/buildkit"
)

// TestShellAssetsMatchManifest 校验随仓库分发的 base_version.txt 与当前嵌入的
// 源文件一致：改了模板或共享源码但没重新生成 base.exe 时，go test 即可发现。
func TestShellAssetsMatchManifest(t *testing.T) {
	opts, err := buildOptions()
	if err != nil {
		t.Fatalf("计算运行壳源文件哈希失败: %v", err)
	}
	if err := buildkit.CheckShellFresh(opts.ShellManifest, opts.ShellHashes); err != nil {
		t.Fatalf("base.exe 已过期: %v", err)
	}
}
