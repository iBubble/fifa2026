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

	for _, route := range r.Routes() {
		log.Printf("[Route Debug] %s %s -> %s", route.Method, route.Path, route.Handler)
	}

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

		// 引入自适应时间档位定义
		type CronMode int
		const (
			ModeLive CronMode = iota // 有正在进行的比赛
			ModeNear                 // 距离下场比赛 < 2小时
			ModeMid                  // 距离下场比赛 2-12小时
			ModeFar                  // 距离下场比赛 >= 12小时（或无比赛）
		)

		// 判定当前调度模式
		evaluateCronMode := func() CronMode {
			matches, err := db.GetMatchesByTournament("fifa_2026")
			if err != nil || len(matches) == 0 {
				return ModeFar
			}

			now := time.Now()
			hasLive := false
			var nextStart time.Time
			hasNext := false

			for _, m := range matches {
				// 判定进行中状态
				if m.Status == "1H" || m.Status == "2H" || m.Status == "HT" || m.Status == "Live" {
					hasLive = true
				}
				if m.Status == "NS" {
					if !hasNext || m.ScheduledAt.Before(nextStart) {
						nextStart = m.ScheduledAt
						hasNext = true
					}
				}
			}

			if hasLive {
				return ModeLive
			}
			if !hasNext {
				return ModeFar
			}

			diff := nextStart.Sub(now)
			if diff < 0 {
				return ModeLive // 开赛时间已过但未结算，按 Live 级别防卫
			}
			if diff < 2*time.Hour {
				return ModeNear
			}
			if diff < 12*time.Hour {
				return ModeMid
			}
			return ModeFar
		}

		// 记录各个子任务的上一次执行时间
		var lastNewsTime, lastSportteryTime, lastSyncTime time.Time

		log.Println("[Background Job] ⚙️ 自适应调频后台守护进程已启动")

		for {
			mode := evaluateCronMode()
			now := time.Now()

			// 根据档位动态确定触发阈值与休眠步长
			var newsInterval, sportteryInterval, syncInterval time.Duration
			var modeLabel string

			switch mode {
			case ModeLive:
				modeLabel = "进行中 (Live)"
				newsInterval = 1 * time.Hour
				sportteryInterval = 15 * time.Minute
				syncInterval = 3 * time.Minute
			case ModeNear:
				modeLabel = "临赛 (Near)"
				newsInterval = 2 * time.Hour
				sportteryInterval = 30 * time.Minute
				syncInterval = 10 * time.Minute
			case ModeMid:
				modeLabel = "常态 (Mid)"
				newsInterval = 6 * time.Hour
				sportteryInterval = 2 * time.Hour
				syncInterval = 30 * time.Minute
			case ModeFar:
				modeLabel = "闲置 (Far)"
				newsInterval = 12 * time.Hour
				sportteryInterval = 6 * time.Hour
				syncInterval = 4 * time.Hour
			}

			// 1. 自动同步完赛比分与赛后回归参数优化 (事件驱动触发)
			if now.Sub(lastSyncTime) >= syncInterval {
				lastSyncTime = now
				log.Printf("[Background Job] [%s] ⏳ 开始同步完赛比分...", modeLabel)
				
				wcSyncLocal := prediction.NewWorldCup26SyncService()
				if synced, errSync := wcSyncLocal.SyncFinishedMatches(); errSync != nil {
					log.Printf("[Background Job] ⚠️ 比分同步异常: %v", errSync)
				} else {
					log.Printf("[Background Job] ✅ 比分同步成功，已同步 %d 场比赛", synced)
					// 判定当天所有比赛是否全部完赛，并自动运行回归调参优化与反思 (取代原定时 5 分钟的 Ticker)
					v1.CheckAndRunDailyOptimization(dcService)
				}
			}

			// 2. 自动拉取体彩赔率和 H2H 交锋数据预热
			if now.Sub(lastSportteryTime) >= sportteryInterval {
				lastSportteryTime = now
				log.Printf("[Background Job] [%s] ⏳ 开始更新体彩官方赔率与 H2H 历史交锋...", modeLabel)
				sportteryService.FetchAllOdds()
				prewarmH2HForIncomingMatches(apiSportsService)
			}

			// 3. 自动更新舆情新闻
			if now.Sub(lastNewsTime) >= newsInterval {
				lastNewsTime = now
				log.Printf("[Background Job] [%s] ⏳ 开始拉取最新球队外围情报新闻...", modeLabel)
				if _, errNews := newsService.FetchAndCacheRealNews(); errNews != nil {
					log.Printf("[Background Job] ⚠️ 外围情报自动更新失败: %v", errNews)
				} else {
					log.Println("[Background Job] ✅ 外围情报新闻更新并缓存成功")
				}
			}

			// 自适应调频自检休眠 30 秒，确保轻量低耗
			time.Sleep(30 * time.Second)
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
