package main

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/news"
	"fifa2026/src/internal/service/prediction"
	"fmt"
	"log"
	"os"
)

func main() {
	log.Println("====== 🚀 启动全量数据同步与抓取校验 ======")

	// 1. 初始化数据库连接
	dbPath := "./data"
	if err := db.Init(dbPath); err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer db.Close()

	// 2. 统计当前数据库的基础数据量
	printTableCounts("同步前")

	// 3. 执行新闻智能抓取
	log.Println("\n[News] 🔍 开始抓取全网最新突发新闻与战术资讯...")
	newsService := news.NewNewsService("")
	articles, err := newsService.FetchAndCacheRealNews()
	if err != nil {
		log.Printf("[News] ⚠️ 新闻同步出错 (这可能是因为外部接口限流): %v", err)
	} else {
		log.Printf("[News] ✅ 成功抓取并写入数据库 %d 条实时新闻！", len(articles))
	}

	// 4. 执行竞彩官方赔率拉取
	log.Println("\n[Sporttery] 🔍 开始拉取最新足球竞彩赔率与盘口...")
	sportteryService := prediction.NewSportteryService()
	// 临时注入环境变量以支持本地代理
	os.Setenv("HTTP_PROXY", "http://127.0.0.1:7897")
	os.Setenv("HTTPS_PROXY", "http://127.0.0.1:7897")
	
	sportteryService.FetchAllOdds()
	log.Println("[Sporttery] ✅ 竞彩赔率拉取完成并写入 odds_history 表。")

	// 5. 执行 worldcup26.ir 比分与进球人同步
	log.Println("\n[WorldCupSync] 🔍 开始同步世界级比赛即时完赛数据...")
	wcSync := prediction.NewWorldCup26SyncService()
	syncedMatches, errWc := wcSync.SyncFinishedMatches()
	if errWc != nil {
		log.Printf("[WorldCupSync] ⚠️ 完赛比分同步异常: %v", errWc)
	} else {
		log.Printf("[WorldCupSync] ✅ 成功同步 %d 场比赛的进球与首发数据！", syncedMatches)
	}

	// 6. 统计同步后的数据库数据量，验证抓取结果
	fmt.Println()
	printTableCounts("同步后")
	log.Println("====== ✅ 全量数据同步与抓取校验完成 ======")
}

func printTableCounts(stage string) {
	fmt.Printf("--- 本地 SQLite 数据库 %s 状态统计 ---\n", stage)
	tables := []string{"matches", "news_articles", "odds_history", "backtest_reports", "prediction_reports", "lottery_plans"}
	for _, t := range tables {
		var count int
		err := db.DB.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", t)).Scan(&count)
		if err != nil {
			fmt.Printf("表 %s 查询失败: %v\n", t, err)
		} else {
			fmt.Printf("📁 表 %s 当前数据量: %d 条\n", t, count)
		}
	}
}
