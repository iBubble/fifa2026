package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// SafeGo 启动一个受保护的后台协程，防止其发生 Panic 导致整个服务崩溃
func SafeGo(fn func()) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				// 获取格式化的崩溃堆栈
				stackTrace := debug.Stack()
				errMessage := fmt.Sprintf("[%s] ⚠️ Background Goroutine Panic Detected:\nErr: %v\nStack:\n%s\n\n",
					time.Now().Format("2006-01-02 15:04:05.000 (MST)"), r, string(stackTrace))

				// 输出到标准日志控制台
				log.Printf(errMessage)

				// 持久化记录到 data/logs/panic.log，方便复盘
				logPath := "./data/logs/panic.log"
				if err := os.MkdirAll(filepath.Dir(logPath), 0755); err != nil {
					log.Printf("[SafeGo] ⚠️ 无法创建日志目录: %v", err)
					return
				}

				f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
				if err != nil {
					log.Printf("[SafeGo] ⚠️ 无法打开日志文件: %v", err)
					return
				}
				defer f.Close()

				if _, err := f.WriteString(errMessage); err != nil {
					log.Printf("[SafeGo] ⚠️ 写入日志文件失败: %v", err)
				}
			}
		}()
		fn()
	}()
}
