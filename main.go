package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"strings"

	"deploy-app/internal/appconf"
	"deploy-app/internal/buildkit"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// appVersion 是打包工具自身的版本号，发布新版本时需与 git 标签保持一致
// （release 工作流会校验两者一致）。
const appVersion = "1.0.1"

func main() {
	// CLI 模式：deploy-app build --config profile.json [--out out.exe]
	if len(os.Args) > 1 && os.Args[1] == "build" {
		os.Exit(runCLIBuild(os.Args[2:]))
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "桌面应用生成器",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop:     true,
			DisableWebViewDrop: true, // 避免拖入文件被 webview 当作导航打开
		},
		OnStartup:     app.startup,
		OnShutdown:    app.shutdown,
		OnBeforeClose: app.confirmClose,
		Bind: []interface{}{
			app,
		},
	})
	if err != nil {
		println("Error:", err.Error())
	}
}

// runCLIBuild 无界面构建，读取方案 JSON 直接打包，供 CI / 脚本调用。
func runCLIBuild(args []string) int {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	configPath := fs.String("config", "", "构建方案 JSON 路径（必填，可由界面“导出方案”生成）")
	outPath := fs.String("out", "", "输出 exe 路径（覆盖方案中的 outputPath）")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if strings.TrimSpace(*configPath) == "" {
		fmt.Fprintln(os.Stderr, "错误: 缺少 --config 参数")
		fmt.Fprintln(os.Stderr, "用法: deploy-app build --config profile.json [--out out.exe]")
		return 1
	}
	if _, err := os.Stat(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 配置文件不存在: %s\n", *configPath)
		return 1
	}

	config, err := appconf.ReadConfigFile(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: 读取配置失败: %v\n", err)
		return 1
	}
	if strings.TrimSpace(*outPath) != "" {
		config.OutputPath = *outPath
	}

	opts, err := buildOptions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return 1
	}
	outputPath, signScript, err := buildkit.Build(context.Background(), config, opts, func(step string, percent int) {
		fmt.Printf("[%3d%%] %s\n", percent, step)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
		return 1
	}
	fmt.Printf("构建完成: %s\n", outputPath)
	if signScript != "" {
		fmt.Printf("签名脚本: %s（运行并输入证书密码完成签名）\n", signScript)
	}
	return 0
}
