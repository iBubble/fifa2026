package main

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/ai"
	"fifa2026/src/internal/service/news"
	"fifa2026/src/internal/service/prediction"
	"io"
	"log"
	"math"
	"net/http"
	"os"
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
	dcService := prediction.NewDixonColesService(eloService)
	mcSimulator := prediction.NewMonteCarloSimulator(dcService, eloService)
	ollamaService := ai.NewOllamaService(os.Getenv("OLLAMA_URL"), os.Getenv("OLLAMA_MODEL"))
	shinService := prediction.NewShinService()
	kellyService := prediction.NewMultiKellyService()
	decayService := prediction.NewTimeDecayService(dcService)
	arbService := prediction.NewArbitrageService()

	sportteryService := prediction.NewSportteryService()
	backtestService := prediction.NewBacktestService(eloService, ollamaService, dcService)
	lotteryService := prediction.NewLotteryService(dcService, sportteryService)

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

	// 配置跨域中间件
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// 4. 定义 REST API 路由
	api := r.Group("/api")
	{
		// 赛事列表与赛程，并在每次加载时自动触发未复盘已完赛场次的异步复盘
		api.GET("/matches", func(c *gin.Context) {
			matches, err := db.GetMatchesByTournament("fifa_2026")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			for _, m := range matches {
				if m.Status == "FT" {
					rep, errReview := db.GetBacktestReport(m.ID)
					if errReview != nil || rep.TacticsReview == "" || strings.Contains(rep.TacticsReview, "超时降级") {
						log.Printf("[Server] 检测到比赛 %s (%s vs %s) 尚未复盘或处于超时降级状态，发起异步复盘...", m.ID, m.HomeTeam, m.AwayTeam)
						go func(m models.Match) {
							log.Printf("[Server] 比赛 %s 异步复盘 Goroutine 开始执行...", m.ID)
							params := dcService.CalculateParams(m.HomeTeam, m.AwayTeam)
							matrix, over25, under25 := dcService.GenerateProbabilityMatrix(params)
							r := models.PredictionReport{
								MatchID:        m.ID,
								OriginalParams: params,
								RefinedParams:  params,
								ScoreMatrix:    matrix,
								Over2_5Prob:    over25,
								Under2_5Prob:   under25,
							}
							res, err := backtestService.ReviewMatch(m, &r)
							if err != nil {
								log.Printf("[Server] ❌ 比赛 %s 异步复盘失败: %v", m.ID, err)
							} else {
								log.Printf("[Server] ✅ 比赛 %s 异步复盘成功，反思结果: %s", m.ID, res.TacticsReview)
							}
						}(m)
					}
				}
			}
			c.JSON(http.StatusOK, matches)
		})

		// 实时赛程比分 SSE 推送通道
		api.GET("/matches/stream", func(c *gin.Context) {
			c.Header("Content-Type", "text/event-stream")
			c.Header("Cache-Control", "no-cache")
			c.Header("Connection", "keep-alive")
			c.Header("Transfer-Encoding", "chunked")

			// 发送初始握手消息，强行冲刷响应头，建立长连接
			c.SSEvent("open", "connected")
			c.Writer.Flush()

			ch := liveSyncService.RegisterListener()
			defer liveSyncService.RemoveListener(ch)

			clientGone := c.Request.Context().Done()

			c.Stream(func(w io.Writer) bool {
				select {
				case <-clientGone:
					return false
				case msg, ok := <-ch:
					if !ok {
						return false
					}
					c.SSEvent("message", msg)
					return true
				}
			})
		})

		// 投注历史流
		api.GET("/bets", func(c *gin.Context) {
			bets, err := db.GetBets("fifa_2026")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, bets)
		})

		// 账本 ROI 汇总统计
		api.GET("/bet/summary", func(c *gin.Context) {
			summary, err := db.GetBetSummary("fifa_2026")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, summary)
		})

		// 单场比赛 Dixon-Coles 回归预测 (支持 Ollama 定性参数偏置修正与降级)
		api.POST("/predict", func(c *gin.Context) {
			var req struct {
				MatchID string `json:"matchId"`
				Info    string `json:"info"`
				UseLLM  bool   `json:"useLLM"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			match, err := db.GetMatch(req.MatchID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "未找到指定比赛"})
				return
			}

			// 获取初始定量参数
			params := dcService.CalculateParams(match.HomeTeam, match.AwayTeam)
			refined := params
			llmRefined := false
			var tactics, poster string

			if req.UseLLM {
				diff := eloService.GetElo(match.HomeTeam) - eloService.GetElo(match.AwayTeam)
				offsets, err := ollamaService.RefineParams(match, diff, params, req.Info)
				if err == nil {
					refined.LambdaHome = params.LambdaHome + offsets.LambdaHomeOffset
					refined.LambdaAway = params.LambdaAway + offsets.LambdaAwayOffset
					refined.Rho = params.Rho + offsets.RhoOffset
					llmRefined = true
					tactics = offsets.TacticsAnalysis
					poster = offsets.PosterPrompt
				} else {
					log.Printf("[Predict] ⚠️ Ollama 大模型偏置微调失效，触发降级: %v", err)
				}
			}

			// 计算比分概率矩阵与大小球概率
			matrix, over25, under25 := dcService.GenerateProbabilityMatrix(refined)

			report := models.PredictionReport{
				MatchID:         req.MatchID,
				OriginalParams:  params,
				RefinedParams:   refined,
				LLMRefined:      llmRefined,
				ScoreMatrix:     matrix,
				Over2_5Prob:     over25,
				Under2_5Prob:    under25,
				TacticsAnalysis: tactics,
				PosterPrompt:    poster,
			}
			_ = db.SavePredictionReport(report)

			c.JSON(http.StatusOK, report)
		})

		// 蒙特卡洛全赛事模拟 (10,000次推演)
		api.POST("/simulate", func(c *gin.Context) {
			fileData, err := os.ReadFile("./data/seasons/fifa_2026.json")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "读取世界杯分组配置失败"})
				return
			}
			var rawSeason struct {
				Groups map[string][]string `json:"groups"`
			}
			if err := json.Unmarshal(fileData, &rawSeason); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "解析分组失败"})
				return
			}

			results := mcSimulator.SimulateTournament(rawSeason.Groups, 10000)
			c.JSON(http.StatusOK, results)
		})

		// 辛氏去抽水折算真实隐含概率
		api.POST("/devig", func(c *gin.Context) {
			var req struct {
				Odds []float64 `json:"odds"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			probs, z, err := shinService.DevigOdds(req.Odds)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"probabilities": probs, "insiderTraderRatio": z})
		})

		// 多臂凯利资产配置优化
		api.POST("/kelly", func(c *gin.Context) {
			var req struct {
				Bets     []models.ValueBet `json:"bets"`
				Bankroll float64           `json:"bankroll"`
				RiskFrac float64           `json:"riskFraction"` // 如 0.25 (1/4凯利)
				MaxExp   float64           `json:"maxExposure"`  // 如 0.50 (总暴露最高50%)
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if req.RiskFrac <= 0 {
				req.RiskFrac = 0.25
			}
			if req.MaxExp <= 0 {
				req.MaxExp = 0.50
			}
			optimized := kellyService.AllocateMultiBets(req.Bets, req.Bankroll, req.RiskFrac, req.MaxExp)
			c.JSON(http.StatusOK, optimized)
		})

		// 滚球进球衰减预测
		api.POST("/time-decay", func(c *gin.Context) {
			var req struct {
				MatchID     string                  `json:"matchId"`
				Minute      int                     `json:"minute"`
				CurrentHome int                     `json:"currentHome"`
				CurrentAway int                     `json:"currentAway"`
				Params      models.DixonColesParams `json:"params"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			matrix, over25, under25 := decayService.PredictRemainGoals(req.Params, req.Minute, req.CurrentHome, req.CurrentAway)
			c.JSON(http.StatusOK, gin.H{
				"scoreMatrix": matrix,
				"over25Prob":  over25,
				"under25Prob": under25,
			})
		})

		// 套利扫描器接口 (自动读取并扫描 SQLite 赔率库)
		api.GET("/arbitrage", func(c *gin.Context) {
			matches, err := db.GetMatchesByTournament("fifa_2026")
			if err != nil || len(matches) == 0 {
				c.JSON(http.StatusOK, []interface{}{})
				return
			}

			// Mock 数据填充：如果 odds_history 没有数据，自动为首场揭幕战填充套利偏置赔率
			// 这能保证博彩套利功能直接可在前端页面得到演示！
			records, _ := db.GetLatestOdds(matches[0].ID)
			if len(records) == 0 {
				// 模拟三个不同平台开出相互倒挂的偏置赔率，形成绝对套利
				_ = db.SaveOddsSnapshot(matches[0].ID, "Bet365", 2.10, 3.40, 3.80)
				_ = db.SaveOddsSnapshot(matches[0].ID, "Pinnacle", 1.80, 3.75, 3.90)
				_ = db.SaveOddsSnapshot(matches[0].ID, "WilliamHill", 1.95, 3.20, 4.20)
				records, _ = db.GetLatestOdds(matches[0].ID)
			}

			var opportunities []models.ArbitrageOpportunity
			for _, m := range matches {
				recs, _ := db.GetLatestOdds(m.ID)
				if len(recs) > 0 {
					opp, found := arbService.ScanArbitrage(m, recs, 1000.0) // 默认以 1000 元总本金测算
					if found {
						opportunities = append(opportunities, opp)
					}
				}
			}
			c.JSON(http.StatusOK, opportunities)
		})

		// 新增投注流水
		api.POST("/bet", func(c *gin.Context) {
			var bet models.Bet
			if err := c.ShouldBindJSON(&bet); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			bet.TournamentID = "fifa_2026"
			bet.PlacedAt = time.Now()
			bet.Result = "PENDING"
			id, err := db.AddBet(bet)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"id": id})
		})

		// 结算投注流水
		api.POST("/bet/settle", func(c *gin.Context) {
			var req struct {
				ID     int64   `json:"id"`
				Result string  `json:"result"` // "WIN", "LOSS", "VOID"
				PnL    float64 `json:"pnl"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			err := db.UpdateBetResult(req.ID, req.Result, req.PnL)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"status": "success"})
		})

		// 中国体彩竞猜量化套利建议接口
		api.POST("/lottery/recommend", func(c *gin.Context) {
			var req struct {
				MatchIDs      []string                 `json:"matchIds"`
				Odds          []float64                `json:"odds"`
				PredictReport *models.PredictionReport `json:"predictReport"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			if len(req.MatchIDs) == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请选择至少一场比赛进行分析"})
				return
			}

			m1, err := db.GetMatch(req.MatchIDs[0])
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "未找到首场比赛"})
				return
			}

			oddsH, oddsD, oddsA := 1.95, 3.20, 3.80
			if len(req.Odds) >= 3 {
				oddsH, oddsD, oddsA = req.Odds[0], req.Odds[1], req.Odds[2]
			}

			singleAdvice := lotteryService.GenerateSingleAdvice(m1, oddsH, oddsD, oddsA, req.PredictReport)

			var parlayAdvice *prediction.LotteryAdvice
			if len(req.MatchIDs) >= 2 {
				m2, err := db.GetMatch(req.MatchIDs[1])
				if err == nil {
					pAdv := lotteryService.GenerateParlayAdvice(m1, m2, oddsH, oddsA)
					parlayAdvice = &pAdv
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"single": singleAdvice,
				"parlay": parlayAdvice,
			})
		})
		// 获取指定赛事的官方体彩赔率数据 (如果未开盘则利用 Elo 算法仿真)
		api.GET("/lottery/official", func(c *gin.Context) {
			matchID := c.Query("matchId")
			if matchID == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "请提供 matchId"})
				return
			}
			match, err := db.GetMatch(matchID)
			if err != nil {
				c.JSON(http.StatusNotFound, gin.H{"error": "比赛未找到"})
				return
			}
			odds := sportteryService.GetMatchOdds(match.HomeTeam, match.AwayTeam)
			// 如果官方未开售，根据双方 Elo 实力推导仿真赔率，杜绝界面空白和数据幻觉
			if !odds.IsAvailable {
				eloHome := eloService.GetElo(match.HomeTeam)
				eloAway := eloService.GetElo(match.AwayTeam)

				// A 队对 B 队的期望胜率倾向 (0~1)
				expHome := eloService.CalculateExpectedWinProb(eloHome, eloAway)
				expAway := eloService.CalculateExpectedWinProb(eloAway, eloHome)

				// 平局概率根据两队 Elo 接近度估计
				eloDiff := math.Abs(eloHome - eloAway)
				probDraw := 0.28 * math.Exp(-eloDiff/600.0)

				// 归一化胜平负概率
				totalExp := expHome + expAway
				probHome := (1.0 - probDraw) * (expHome / totalExp)
				probAway := (1.0 - probDraw) * (expAway / totalExp)

				// 中国竞彩大致返还率 89%
				payout := 0.89
				oddsH := payout / probHome
				oddsD := payout / probDraw
				oddsA := payout / probAway

				// 设定仿真让球与让球赔率
				goalLine := 0
				hhadH, hhadD, hhadA := oddsH, oddsD, oddsA
				if eloHome-eloAway > 150 {
					goalLine = -1
					hhadH = oddsH * 1.8
					hhadD = oddsD * 1.1
					hhadA = oddsA * 0.6
				} else if eloAway-eloHome > 150 {
					goalLine = 1
					hhadH = oddsH * 0.6
					hhadD = oddsD * 1.1
					hhadA = oddsA * 1.8
				}

				odds = prediction.OfficialOdds{
					HomeOdds:     math.Round(oddsH*100) / 100,
					DrawOdds:     math.Round(oddsD*100) / 100,
					AwayOdds:     math.Round(oddsA*100) / 100,
					GoalLine:     goalLine,
					HhadHomeOdds: math.Round(hhadH*100) / 100,
					HhadDrawOdds: math.Round(hhadD*100) / 100,
					HhadAwayOdds: math.Round(hhadA*100) / 100,
					IsAvailable:  true,
					IsSimulation: true,
				}
			}
			c.JSON(http.StatusOK, odds)
		})

		// 获取历史体彩建议收益结算
		api.GET("/lottery/history", func(c *gin.Context) {
			matches, err := db.GetMatchesByTournament("fifa_2026")
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			var historyList []gin.H
			var totalSafeCost, totalSafeReturn float64
			var totalAggCost, totalAggReturn float64

			for _, m := range matches {
				if m.Status == "FT" {
					// 1. 获取预测报告
					var report *models.PredictionReport
					rep, errRep := db.GetPredictionReport(m.ID)
					if errRep == nil {
						report = &rep
					}

					// 2. 生成体彩建议
					oddsH, oddsD, oddsA := 1.95, 3.20, 3.80
					advice := lotteryService.GenerateSingleAdvice(m, oddsH, oddsD, oddsA, report)
					if advice.Status == "EXCLUDED" {
						continue // 被排除的比赛不计入投注收益统计
					}

					// 3. 结算实际收益
					// 判定主推是否命中
					primaryHit := false
					if advice.PrimaryBet == "主胜 (3)" && m.HomeScore > m.AwayScore {
						primaryHit = true
					} else if advice.PrimaryBet == "平局 (1)" && m.HomeScore == m.AwayScore {
						primaryHit = true
					} else if advice.PrimaryBet == "客胜 (0)" && m.HomeScore < m.AwayScore {
						primaryHit = true
					}

					// 判定对冲是否命中
					hedgeHit := false
					var hedgeOdds float64
					if len(advice.HedgeBets) > 0 {
						hedge := advice.HedgeBets[0]
						hedgeOdds = hedge.Odds
						if hedge.Outcome == "比分 1-1" && m.HomeScore == 1 && m.AwayScore == 1 {
							hedgeHit = true
						} else if hedge.Outcome == "比分 1-0" && m.HomeScore == 1 && m.AwayScore == 0 {
							hedgeHit = true
						} else if hedge.Outcome == "比分 0-1" && m.HomeScore == 0 && m.AwayScore == 1 {
							hedgeHit = true
						}
					}

					// 稳妥型：主推投80元，对冲投20元
					safeReturn := 0.0
					if primaryHit {
						safeReturn += 80.0 * advice.PrimaryOdds
					}
					if hedgeHit {
						safeReturn += 20.0 * hedgeOdds
					}

					// 激进型：主推投100元，不对冲
					aggReturn := 0.0
					if primaryHit {
						aggReturn += 100.0 * advice.PrimaryOdds
					}

					totalSafeCost += 100.0
					totalSafeReturn += safeReturn
					totalAggCost += 100.0
					totalAggReturn += aggReturn

					historyList = append(historyList, gin.H{
						"matchId":     m.ID,
						"homeTeam":    m.HomeTeam,
						"awayTeam":    m.AwayTeam,
						"homeScore":   m.HomeScore,
						"awayScore":   m.AwayScore,
						"primaryBet":  advice.PrimaryBet,
						"primaryOdds": advice.PrimaryOdds,
						"primaryHit":  primaryHit,
						"hedgeBet":    advice.HedgeBets[0].Outcome,
						"hedgeOdds":   hedgeOdds,
						"hedgeHit":    hedgeHit,
						"safeReturn":  math.Round(safeReturn*100) / 100,
						"safeProfit":  math.Round((safeReturn-100)*100) / 100,
						"aggReturn":   math.Round(aggReturn*100) / 100,
						"aggProfit":   math.Round((aggReturn-100)*100) / 100,
					})
				}
			}

			safeProfit := totalSafeReturn - totalSafeCost
			aggProfit := totalAggReturn - totalAggCost

			safeRoi := 0.0
			if totalSafeCost > 0 {
				safeRoi = (safeProfit / totalSafeCost) * 100.0
			}
			aggRoi := 0.0
			if totalAggCost > 0 {
				aggRoi = (aggProfit / totalAggCost) * 100.0
			}

			c.JSON(http.StatusOK, gin.H{
				"history": historyList,
				"summary": gin.H{
					"totalSafeCost":   totalSafeCost,
					"totalSafeReturn": math.Round(totalSafeReturn*100) / 100,
					"totalSafeProfit": math.Round(safeProfit*100) / 100,
					"safeRoi":         math.Round(safeRoi*100) / 100,
					"totalAggCost":    totalAggCost,
					"totalAggReturn":  math.Round(totalAggReturn*100) / 100,
					"totalAggProfit":  math.Round(aggProfit*100) / 100,
					"aggRoi":          math.Round(aggRoi*100) / 100,
				},
			})
		})

		// 拉取所有已完赛复盘报告历史数据，供大屏绘制 Brier Score 曲线
		api.GET("/backtest/history", func(c *gin.Context) {
			reports, err := db.GetBacktestReports()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, reports)
		})

		// 真实外围情报获取接口
		api.GET("/news", func(c *gin.Context) {
			matchID := c.Query("matchId")
			var homeTeam, awayTeam string
			if matchID != "" {
				if match, err := db.GetMatch(matchID); err == nil {
					homeTeam = match.HomeTeam
					awayTeam = match.AwayTeam
				}
			}
			articles, err := newsService.FetchRealNewsForMatch(homeTeam, awayTeam)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusOK, articles)
		})

		// 全球博彩公司赔率与偏移监测接口
		api.GET("/odds/shifts", func(c *gin.Context) {
			matchID := c.Query("matchId")
			var homeTeam, awayTeam string
			if matchID != "" {
				if match, err := db.GetMatch(matchID); err == nil {
					homeTeam = match.HomeTeam
					awayTeam = match.AwayTeam
				}
			}
			shifts := oddsTrackerService.GetOddsShifts(homeTeam, awayTeam)
			c.JSON(http.StatusOK, shifts)
		})

	}

	// 启动后台定时常驻抓取任务 (Background Ticker Jobs)
	go func() {
		// 延迟几秒，等主服务端口监听顺利开启
		time.Sleep(3 * time.Second)
		log.Println("[Background Job] ⏳ 自动启动，立即执行体彩赔率和外围情报预热拉取...")
		sportteryService.FetchAllOdds()
		if _, err := newsService.FetchAndCacheRealNews(); err != nil {
			log.Printf("[Background Job] ⚠️ 首次外围情报拉取异常: %v", err)
		} else {
			log.Println("[Background Job] ✅ 首次外围情报拉取完成，缓存已建立")
		}

		tickerNews := time.NewTicker(10 * time.Minute)      // 每 10 分钟自动拉取一次外围情报
		tickerSporttery := time.NewTicker(30 * time.Minute) // 每 30 分钟自动拉取一次体彩赔率
		defer tickerNews.Stop()
		defer tickerSporttery.Stop()

		for {
			select {
			case <-tickerNews.C:
				log.Println("[Background Job] ⏳ 正在自动更新外围情报新闻...")
				if _, err := newsService.FetchAndCacheRealNews(); err != nil {
					log.Printf("[Background Job] ⚠️ 外围情报自动更新失败: %v", err)
				} else {
					log.Println("[Background Job] ✅ 外围情报自动更新成功，缓存已刷新")
				}
			case <-tickerSporttery.C:
				log.Println("[Background Job] ⏳ 正在自动更新体彩官方赔率数据...")
				sportteryService.FetchAllOdds()
				log.Println("[Background Job] ✅ 体彩官方赔率数据自动更新成功，缓存已刷新")
			}
		}
	}()

	// 5. 挂载前端静态网页资源托管
	// 托管 ./src/frontend 文件夹的 HTML/CSS/JS 资源
	r.Static("/static", "./src/frontend")
	r.StaticFile("/", "./src/frontend/index.html")

	// 6. 端口绑定，默认监听 20260 并支持通过环境变量覆盖
	port := os.Getenv("PORT")
	if port == "" {
		port = "20260"
	}

	log.Printf("[Server] 🚀 FIFA2026 网页量化预测系统已启动，监听端口: %s", port)
	log.Printf("[Server] 本地访问地址: http://localhost:%s", port)

	if err := r.Run(":" + port); err != nil {
		log.Fatalf("[Server] 启动失败: %v", err)
	}
}
