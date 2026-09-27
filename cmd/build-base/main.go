// build-base 开发工具
// 用于预编译基础 Wails 应用程序为 exe 文件
// 生成的 exe 会被嵌入到 deploy-app 中，这样最终用户不需要 Go 环境就能构建应用
//
// 使用方法：go run ./cmd/build-base
//
// 输出文件：
//   - templates/base/base.exe
//   - templates/base/base_version.txt（源文件 SHA256 清单，供打包工具检测漂移）
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"text/template"

	"deploy-app/internal/shellinfo"
)

// templateData 模板数据结构体
// 用于渲染生成应用的源代码模板
type templateData struct {
	AppName    string // 应用名称
	Width      int    // 窗口宽度
	Height     int    // 窗口高度
	Fullscreen bool   // 是否全屏
	Maximized  bool   // 是否最大化
}

// main 主函数
// 执行以下步骤：
// 1. 创建临时工作目录
// 2. 渲染 Go 源代码模板，并复制与主工程共享的源文件
// 3. 创建占位配置文件
// 4. 运行 go mod tidy
// 5. 编译生成 base.exe
// 6. 写入源文件哈希清单 base_version.txt
func main() {
	// 获取当前工作目录（项目根目录）
	projectRoot, err := os.Getwd()
	if err != nil {
		fatal("获取工作目录失败: %v", err)
	}

	// 定义目录路径
	templateDir := filepath.Join(projectRoot, "templates", "generated-app") // 模板目录
	outputDir := filepath.Join(projectRoot, "templates", "base")            // 输出目录
	outputExe := filepath.Join(outputDir, "base.exe")                       // 输出文件路径

	// 创建输出目录
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fatal("创建输出目录失败: %v", err)
	}

	// 创建临时工作目录
	tempDir, err := os.MkdirTemp("", "build-base-*")
	if err != nil {
		fatal("创建工作目录失败: %v", err)
	}
	defer os.RemoveAll(tempDir) // 程序结束时清理临时目录

	fmt.Println("工作目录:", tempDir)

	// 模板数据（使用占位值）
	data := templateData{
		AppName:    "BaseApp", // 应用名称
		Width:      1024,      // 默认宽度
		Height:     768,       // 默认高度
		Fullscreen: false,     // 默认不全屏
		Maximized:  false,     // 默认不最大化
	}

	// 渲染 Go 源代码模板
	templates := []string{"main.go", "app.go", "loader.go", "proxy.go", "error_windows.go", "go.mod"}
	for _, name := range templates {
		tmplPath := filepath.Join(templateDir, name+".tmpl")
		outputPath := filepath.Join(tempDir, name)

		// 渲染模板并写入文件
		if err := renderTemplate(tmplPath, outputPath, data); err != nil {
			fatal("渲染模板 %s 失败: %v", name, err)
		}
		fmt.Printf("  ✓ 渲染 %s\n", name)
	}

	// 复制与主工程共享的源文件（替换 package 声明为 main）
	for _, s := range shellinfo.SharedSources {
		srcPath := filepath.Join(projectRoot, filepath.FromSlash(s.Src))
		content, err := os.ReadFile(srcPath)
		if err != nil {
			fatal("读取共享源文件 %s 失败: %v", s.Src, err)
		}
		if !bytes.Contains(content, []byte(s.Pkg)) {
			fatal("共享源文件 %s 缺少 %q 声明", s.Src, s.Pkg)
		}
		content = bytes.Replace(content, []byte(s.Pkg), []byte("package main"), 1)
		if err := os.WriteFile(filepath.Join(tempDir, s.Dst), content, 0644); err != nil {
			fatal("写入 %s 失败: %v", s.Dst, err)
		}
		fmt.Printf("  ✓ 复制 %s -> %s\n", s.Src, s.Dst)
	}

	// 创建空的 proxy_config.json（基础应用没有代理规则）
	proxyConfig := `{"rules": []}`
	if err := os.WriteFile(filepath.Join(tempDir, "proxy_config.json"), []byte(proxyConfig), 0644); err != nil {
		fatal("写入 proxy_config.json 失败: %v", err)
	}

	// 创建空的 dist 目录（基础应用没有前端资源）
	distDir := filepath.Join(tempDir, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		fatal("创建 dist 目录失败: %v", err)
	}

	// 写入占位的 index.html，确保 embed 指令有内容可嵌入
	placeholder := `<!DOCTYPE html><html><body><h1>Resource not found</h1></body></html>`
	if err := os.WriteFile(filepath.Join(distDir, "index.html"), []byte(placeholder), 0644); err != nil {
		fatal("写入 placeholder 失败: %v", err)
	}

	// 运行 go mod tidy，下载依赖
	fmt.Println("运行 go mod tidy...")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = tempDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("go mod tidy 失败: %v", err)
	}

	// go build 拒绝覆盖“已存在但不是合法 object 文件”的输出（例如占位 base.exe）
	if _, err := os.Stat(outputExe); err == nil {
		fmt.Println("移除旧的 base.exe...")
		if err := os.Remove(outputExe); err != nil {
			fatal("删除旧 base.exe 失败: %v", err)
		}
	}

	// 编译生成 base.exe
	fmt.Println("编译 base.exe...")
	cmd = exec.Command("go", "build",
		"-tags", "production", // 生产环境标签
		"-ldflags", "-w -s -H windowsgui", // 去掉调试信息，隐藏控制台窗口
		"-o", outputExe, // 输出文件路径
		".", // 当前目录
	)
	cmd.Dir = tempDir
	cmd.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64") // 交叉编译为 Windows 64位
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fatal("go build 失败: %v", err)
	}

	// 写入源文件哈希清单（encoding/json 会对 map 键排序，输出确定性内容）
	if err := writeBaseShellVersion(projectRoot); err != nil {
		fatal("写入 base_version.txt 失败: %v", err)
	}

	// 获取输出文件大小
	stat, err := os.Stat(outputExe)
	if err != nil {
		fatal("获取输出文件信息失败: %v", err)
	}

	// 输出结果
	fmt.Printf("\n✓ base.exe 已生成: %s (%.1f MB)\n", outputExe, float64(stat.Size())/(1024*1024))
	fmt.Printf("✓ base_version.txt 已更新（%d 个源文件哈希，Go %s）\n", len(shellinfo.SourceFiles), runtime.Version())
}

// writeBaseShellVersion 计算参与运行壳编译的源文件哈希并写入 base_version.txt。
// 打包工具构建时会用嵌入的这份清单与当前源文件比对，检测 base.exe 是否过期。
func writeBaseShellVersion(projectRoot string) error {
	files := make(map[string]string, len(shellinfo.SourceFiles))
	for _, rel := range shellinfo.SourceFiles {
		data, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(rel)))
		if err != nil {
			return fmt.Errorf("读取 %s 失败: %w", rel, err)
		}
		sum := sha256.Sum256(data)
		files[rel] = hex.EncodeToString(sum[:])
	}
	data, err := json.MarshalIndent(shellinfo.Manifest{Files: files}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(projectRoot, "templates", "base", "base_version.txt"), data, 0644)
}

// renderTemplate 渲染模板文件并写入输出文件
func renderTemplate(tmplPath, outputPath string, data interface{}) error {
	content, err := os.ReadFile(tmplPath)
	if err != nil {
		return err
	}

	tmpl, err := template.New(filepath.Base(tmplPath)).Parse(string(content))
	if err != nil {
		return err
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, data)
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "错误: "+format+"\n", args...)
	os.Exit(1)
}
