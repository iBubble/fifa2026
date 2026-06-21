package main

import (
	"fifa2026/src/api/v1"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/ai"
	"fifa2026/src/internal/service/news"
	"fifa2026/src/internal/service/prediction"
	"fifa2026/src/utils"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. 初始化 SQLite 数据库并创建隔离表
	dataDir := "./data/db"
	if err := db.Init(dataDir); err != nil {
		log.Fatalf("[Server] 数据库初始化失败: %v", err)
	}
	defer db.Close()

	// 初始化核心量化与AI算法服务
	eloService, err := prediction.NewEloService("./data/seasons/history_features.json")
	if err != nil {
		log.Fatalf("[Server] 初始化Elo服务失败: %v", err)
	}

	apiSportsKey := os.Getenv("APISPORTS_KEY")
	if apiSportsKey == "" {
		apiSportsKey = "7eea26f9d015bc60899c2c322937b237,80a8043f046c4a926d609e11ae94438e"
	}
	apiSportsService := prediction.NewAPISportsService(apiSportsKey)
	weatherService := prediction.NewWeatherService()

	dcService := prediction.NewDixonColesService(eloService, apiSportsService, weatherService)
	mcSimulator := prediction.NewMonteCarloSimulator(dcService, eloService)
	ollamaService := ai.NewOllamaService(os.Getenv("OLLAMA_URL"), os.Getenv("OLLAMA_MODEL"))
	shinService := prediction.NewShinService()
	kellyService := prediction.NewMultiKellyService()
	decayService := prediction.NewTimeDecayService(dcService)
	arbService := prediction.NewArbitrageService()

	sportteryService := prediction.NewSportteryService()
	sportteryService.StartBackgroundRefresh()
	backtestService := prediction.NewBacktestService(eloService, ollamaService, dcService)
	lotteryService := prediction.NewLotteryService(dcService, sportteryService)
	parlayService := prediction.NewParlayService(dcService, sportteryService, eloService, shinService)

	newsService := news.NewNewsService("")
	oddsTrackerService := prediction.NewOddsTrackerService()
	liveSyncService := prediction.NewLiveSyncService(dcService, backtestService, ollamaService)
	liveSyncService.StartSyncLoop()

	// 2. 导入 2026 世界杯冷启动初始数据
	seasonsJSON := "./data/seasons/fifa_2026.json"
	featuresJSON := "./data/seasons/history_features.json"
	if err := db.ImportInitialData(seasonsJSON, featuresJSON); err != nil {
		log.Printf("[Server] ⚠️ 数据冷启动填充警报: %v", err)
	} else {
		log.Println("[Server] ✅ 世界杯冷启动赛事数据导入成功")
	}

	// 3. 启动 Gin 引擎
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// 配置跨域中间件 (限制跨域来源域名，防范CORS注入)
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost", "http://127.0.0.1", "https://fifa2026.liukun.com"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))
	// 4. 强制禁用静态资源与主页面的浏览器缓存，确保前后端代码热更新实时对齐
	r.Use(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/static/") || path == "/" {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
		}
		c.Next()
	})

	// 5. 初始化 API 控制器依赖，并注册全部 30 个量化预测路由
	ctrl := &v1.APIController{
		MCSimulator:        mcSimulator,
		DCService:          dcService,
		EloService:         eloService,
		ShinService:        shinService,
		SportteryService:   sportteryService,
		ParlayService:      parlayService,
		LiveSyncService:    liveSyncService,
		APISportsService:   apiSportsService,
		NewsService:        newsService,
		WeatherService:     weatherService,
		BacktestService:    backtestService,
		OllamaService:      ollamaService,
		KellyService:       kellyService,
		DecayService:       decayService,
		ArbService:         arbService,
		LotteryService:     lotteryService,
		OddsTrackerService: oddsTrackerService,
	}
	v1.RegisterRoutes(r, ctrl)

	// 6. 挂载前端静态网页资源托管
	r.Static("/static", "./src/frontend")
	r.StaticFile("/", "./src/frontend/index.html")

	// 7. 启动后台定时常驻抓取任务 (使用 SafeGo 保护防止协程崩溃)
	utils.SafeGo(func() {
		time.Sleep(3 * time.Second)
		log.Println("[Background Job] ⏳ 自动启动，立即执行体彩赔率和外围情报预热拉取...")
		sportteryService.FetchAllOdds()
		if _, err := newsService.FetchAndCacheRealNews(); err != nil {
			log.Printf("[Background Job] ⚠️ 首次外围情报拉取异常: %v", err)
		} else {
			log.Println("[Background Job] ✅ 首次外围情报拉取完成，缓存已建立")
		}

		prewarmH2HForIncomingMatches(apiSportsService)

		wcSync := prediction.NewWorldCup26SyncService()
		if synced, err := wcSync.SyncFinishedMatches(); err != nil {
			log.Printf("[Background Job] ⚠️ 首次完赛比分同步异常: %v", err)
		} else {
			log.Printf("[Background Job] ✅ 首次完赛比分同步成功，已同步 %d 场比赛", synced)
		}

		tickerNews := time.NewTicker(10 * time.Minute)
		tickerSporttery := time.NewTicker(30 * time.Minute)
		tickerOptimize := time.NewTicker(5 * time.Minute)
		tickerWorldCupSync := time.NewTicker(5 * time.Minute)
		defer tickerNews.Stop()
		defer tickerSporttery.Stop()
		defer tickerOptimize.Stop()
		defer tickerWorldCupSync.Stop()

		for {
			select {
			case <-tickerNews.C:
				log.Println("[Background Job] ⏳ 正在自动更新外围情报新闻...")
				if _, err := newsService.FetchAndCacheRealNews(); err != nil {
					log.Printf("[Background Job] ⚠️ 外围情报自动更新失败: %v", err)
				}
			case <-tickerSporttery.C:
				log.Println("[Background Job] ⏳ 正在自动更新体彩官方赔率数据...")
				sportteryService.FetchAllOdds()
				prewarmH2HForIncomingMatches(apiSportsService)
			case <-tickerOptimize.C:
				v1.CheckAndRunDailyOptimization(dcService)
			case <-tickerWorldCupSync.C:
				wcSync := prediction.NewWorldCup26SyncService()
				if synced, err := wcSync.SyncFinishedMatches(); err != nil {
					log.Printf("[Background Job] ⚠️ 自动同步完赛比分异常: %v", err)
				} else {
					log.Printf("[Background Job] ✅ 自动同步完赛比分成功，已同步 %d 场比赛", synced)
				}
			}
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "20260"
	}

	ollamaService.WarmUp()

	log.Printf("[Server] 🚀 FIFA2026 网页量化预测系统已启动，监听端口: %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[Server] 启动失败: %v", err)
	}
}

// prewarmH2HForIncomingMatches 预热最临近 4 场未完赛/进行中比赛的 H2H 历史交锋数据
func prewarmH2HForIncomingMatches(apiService *prediction.APISportsService) {
	if apiService == nil {
		return
	}
	log.Println("[Background Job] ⏳ 开始对最临近的 4 场未完赛赛事进行 H2H 预热...")

	matches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		log.Printf("[Background Job] ⚠️ 预热拉取比赛列表失败: %v", err)
		return
	}

	var incoming []models.Match
	for _, m := range matches {
		if m.Status == "NS" || m.Status == "1H" || m.Status == "2H" || m.Status == "HT" {
			incoming = append(incoming, m)
		}
	}

	sort.Slice(incoming, func(i, j int) bool {
		return incoming[i].ScheduledAt.Before(incoming[j].ScheduledAt)
	})

	limit := 4
	if len(incoming) < limit {
		limit = len(incoming)
	}

	prewarmedCount := 0
	for i := 0; i < limit; i++ {
		m := incoming[i]

		_, _, _, _, _, _, _, found, err := db.GetH2HRecord(m.HomeTeam, m.AwayTeam)
		if err == nil && found {
			continue
		}

		log.Printf("[Background Job] 检测到未缓存赛事，正在后台预热: %s vs %s 的 H2H 数据...", m.HomeTeam, m.AwayTeam)

		utils.SafeGo(func() {
			_, errFetch := apiService.GetH2HRecord(m.HomeTeam, m.AwayTeam)
			if errFetch != nil {
				log.Printf("[Background Job] ❌ 预热失败 (%s vs %s): %v", m.HomeTeam, m.AwayTeam, errFetch)
			} else {
				log.Printf("[Background Job] ✅ 预热成功 (%s vs %s) 并已落库缓存", m.HomeTeam, m.AwayTeam)
			}
		})

		prewarmedCount++
	}

	log.Printf("[Background Job] ✅ H2H 预热自检完毕，本次激活了 %d 场比赛的异步抓取任务", prewarmedCount)
}
