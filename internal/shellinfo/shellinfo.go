// Package shellinfo 定义运行壳（base.exe）的源文件清单，
// 由 cmd/build-base 与打包工具（builder.go）共用，
// 避免两处各自维护同一份列表导致相互脱节。
package shellinfo

// Manifest 是 templates/base/base_version.txt 的结构：
// 参与运行壳编译的源文件（相对仓库根） -> SHA256。
type Manifest struct {
	Files map[string]string `json:"files"`
}

// SourceFiles 列出参与运行壳编译的全部源文件（相对仓库根目录）。
// build-base 计算这些文件的 SHA256 写入 base_version.txt；
// 打包工具构建时用同一份清单逐项比对，检测「模板已改但 base.exe 过期」。
var SourceFiles = []string{
	"templates/generated-app/main.go.tmpl",
	"templates/generated-app/app.go.tmpl",
	"templates/generated-app/loader.go.tmpl",
	"templates/generated-app/proxy.go.tmpl",
	"templates/generated-app/error_windows.go.tmpl",
	"templates/generated-app/go.mod.tmpl",
	"internal/nginxproxy/path.go",
	"internal/resourcefs/resourcefs.go",
}

// SharedSource 描述与主工程共用实现、需复制进生成壳的源文件。
type SharedSource struct {
	Src string // 仓库内路径
	Dst string // 生成目录内文件名
	Pkg string // 原 package 声明（复制时替换为 package main）
}

// SharedSources 复制进生成壳的共享源文件。
var SharedSources = []SharedSource{
	{Src: "internal/nginxproxy/path.go", Dst: "path.go", Pkg: "package nginxproxy"},
	{Src: "internal/resourcefs/resourcefs.go", Dst: "resourcefs.go", Pkg: "package resourcefs"},
}
