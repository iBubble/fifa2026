package v1

import (
	"fifa2026/src/internal/service/prediction"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// SyncAll 全量同步各项数据接口 (主动触发数据补充)
func (ctrl *APIController) SyncAll(c *gin.Context) {
	log.Println("[SyncAll] 🚀 主动触发全量数据同步流程...")

	// 1. worldcup26.ir 比分与进球人同步
	wcSync := prediction.NewWorldCup26SyncService()
	syncedMatches, errWc := wcSync.SyncFinishedMatches()
	if errWc != nil {
		log.Printf("[SyncAll] ⚠️ worldcup26.ir 同步失败: %v", errWc)
	}

	// 2. 新闻智能抓取
	articles, errNews := ctrl.NewsService.FetchAndCacheRealNews()
	if errNews != nil {
		log.Printf("[SyncAll] ⚠️ 新闻抓取失败: %v", errNews)
	}

	// 3. 赔率自动更新
	go ctrl.SportteryService.FetchAllOdds()

	// 4. 并发拉取比分
	ctrl.LiveSyncService.SyncMatches()

	c.JSON(http.StatusOK, gin.H{
		"status":                    "success",
		"message":                   "全量数据同步任务已触发/执行完成",
		"worldcup26_synced_matches": syncedMatches,
		"news_articles_fetched":     len(articles),
		"timestamp":                 time.Now().Format(time.RFC3339),
	})
}
