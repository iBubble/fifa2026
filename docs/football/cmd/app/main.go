// main.go - Wails v3 应用程序入口
// 创建应用窗口并绑定 App 服务，启动 Wails 事件循环
package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// 使用 Go embed 将编译好的前端文件嵌入到二进制中
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	myApp := NewApp()

	// 创建 Wails v3 应用实例
	wailsApp := application.New(application.Options{
		Name:        "WC2026 Quant Terminal",
		Description: "2026 FIFA World Cup Quantitative Trading Desktop",
		// 将 App 结构体的所有公开方法绑定为前端可调用的服务
		Services: []application.Service{
			application.NewService(myApp),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		// macOS 特定配置
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// 将 wailsApp 引用传回给 App（用于 Event.Emit）
	myApp.wailsApp = wailsApp

	// 创建主窗口（Wails v3 正确 API: app.Window.NewWithOptions）
	wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "WC2026 Quant Terminal",
		Width:  1440,
		Height: 900,
		MinWidth:  1280,
		MinHeight: 720,
		// 无边框磨砂玻璃效果（macOS）
		Mac: application.MacWindow{
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
			InvisibleTitleBarHeight: 50,
		},
		BackgroundColour: application.NewRGB(13, 17, 23), // #0D1117
		URL:              "/",
	})

	// 启动事件循环（阻塞直到应用退出）
	if err := wailsApp.Run(); err != nil {
		log.Fatalf("应用启动失败: %v", err)
	}
}
