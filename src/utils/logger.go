package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

// RecordH2HError 将获取两队交锋记录时的错误持久化追加写入本地日志中，便于事后定位复盘
func RecordH2HError(team1, team2 string, err error) {
	if err == nil {
		return
	}
	logMsg := fmt.Sprintf("[%s] ❌ H2H Error (%s vs %s): %v\n",
		time.Now().Format("2006-01-02 15:04:05"), team1, team2, err)

	// 控制台输出
	log.Print(logMsg)

	// 持久化记录到 data/logs/h2h_error.log
	logPath := "./data/logs/h2h_error.log"
	if mkdirErr := os.MkdirAll(filepath.Dir(logPath), 0755); mkdirErr != nil {
		log.Printf("[Logger] ⚠️ 无法创建日志目录: %v", mkdirErr)
		return
	}

	f, openErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if openErr != nil {
		log.Printf("[Logger] ⚠️ 无法打开日志文件: %v", openErr)
		return
	}
	defer f.Close()

	if _, writeErr := f.WriteString(logMsg); writeErr != nil {
		log.Printf("[Logger] ⚠️ 写入日志文件失败: %v", writeErr)
	}
}
