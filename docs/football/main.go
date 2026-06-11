// main.go - Wails v3 应用程序入口
// 创建应用窗口并绑定 App 服务，启动 Wails 事件循环
package main

import (
	"bufio"
	"embed"
	"log"
	"os"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// 使用 Go embed 将编译好的前端文件嵌入到二进制中
// 注意：embed 必须在 package main 所在目录声明
//
//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 加载本地环境变量配置文件 .env
	loadEnvFile(".env")

	myApp := NewApp()

	// 创建 Wails v3 应用实例
	wailsApp := application.New(application.Options{
		Name:        "Football Quant Terminal",
		Description: "足球量化分析桌面终端 - 多联赛赔率监控与套利分析",
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
		Title:     "足球量化分析终端",
		Width:     1440,
		Height:    900,
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

// loadEnvFile 加载指定的环境变量文件
func loadEnvFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		return // 若文件不存在，则不作处理（默认使用系统环境变量）
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// 忽略空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// 按第一个等号拆分
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		// 移除值两端的单引号或双引号
		if (strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) ||
			(strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) {
			val = val[1 : len(val)-1]
		}

		// 每次加载都直接设置环境变量，以允许通过直接修改 .env 文件来覆盖本地配置
		os.Setenv(key, val)
	}
}

