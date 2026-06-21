package v1

import (
	"fifa2026/src/internal/db"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetHealth 检查系统及其依赖的数据库是否健康
func (ctrl *APIController) GetHealth(c *gin.Context) {
	if db.DB == nil {
		log.Println("[HealthCheck] ❌ 数据库连接尚未初始化")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "DOWN",
			"error":  "数据库未初始化",
		})
		return
	}

	// 探测 SQLite 数据库连接及活跃状态
	var one int
	err := db.DB.QueryRow("SELECT 1;").Scan(&one)
	if err != nil {
		log.Printf("[HealthCheck] ❌ 数据库探测失败 (可能发生死锁或连接断开): %v", err)
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"status": "DOWN",
			"error":  err.Error(),
		})
		return
	}

	// 返回 UP 状态
	c.JSON(http.StatusOK, gin.H{
		"status": "UP",
	})
}
