package v1

import (
	"fifa2026/src/internal/service/ai"
	"fifa2026/src/internal/service/news"
	"fifa2026/src/internal/service/prediction"

	"github.com/gin-gonic/gin"
)

// APIController 存储所有的核心服务依赖，用于 API 控制器方法挂载
type APIController struct {
	MCSimulator        *prediction.MonteCarloSimulator
	DCService          *prediction.DixonColesService
	EloService         *prediction.EloService
	ShinService        *prediction.ShinService
	SportteryService   *prediction.SportteryService
	ParlayService      *prediction.ParlayService
	LiveSyncService    *prediction.LiveSyncService
	APISportsService   *prediction.APISportsService
	NewsService        *news.NewsService
	WeatherService     *prediction.WeatherService
	BacktestService    *prediction.BacktestService
	OllamaService      *ai.OllamaService
	KellyService       *prediction.MultiKellyService
	DecayService       *prediction.TimeDecayService
	ArbService         *prediction.ArbitrageService
	LotteryService     *prediction.LotteryService
	OddsTrackerService *prediction.OddsTrackerService
}

// RegisterRoutes 注册系统全部路由接口并映射到具体的 API 控制器方法中
func RegisterRoutes(r *gin.Engine, ctrl *APIController) {
	api := r.Group("/api")
	{
		// 0. 系统健康检测与监控
		api.GET("/health", ctrl.GetHealth)

		// 1. 赛事列表与比分流
		api.GET("/matches", ctrl.GetMatches)
		api.GET("/matches/stream", ctrl.StreamMatches)
		api.GET("/bet/matches", ctrl.GetBetMatches)

		// 2. 投注决策与配资生成
		api.POST("/predict", ctrl.PredictMatch)
		api.POST("/parlay/recommend", ctrl.ParlayRecommend)
		api.POST("/bet/generate", ctrl.BetGenerate)

		// 3. 大模型智能问答
		api.POST("/chat", ctrl.ChatAgent)

		// 4. 数据全量同步入口
		api.POST("/sync/all", ctrl.SyncAll)

		// 5. 账本与投注统计
		api.GET("/bets", ctrl.GetBets)
		api.GET("/bet/summary", ctrl.GetBetSummary)
		api.POST("/bet/save-advice", ctrl.SaveBetAdvice)
		api.POST("/bet", ctrl.CreateBet)
		api.POST("/bet/settle", ctrl.SettleBet)

		// 6. 体彩彩票方案
		api.POST("/lottery/recommend", ctrl.RecommendLottery)
		api.POST("/lottery/save-single", ctrl.SaveSingleLottery)
		api.POST("/lottery/save-parlay", ctrl.SaveParlayLottery)
		api.GET("/lottery/official", ctrl.GetOfficialLottery)
		api.GET("/lottery/history", ctrl.GetLotteryHistory)
		api.GET("/lottery/saved", ctrl.GetSavedLotteryPlans)
		api.POST("/lottery/delete", ctrl.DeleteLotteryPlans)
		api.POST("/lottery/settle", ctrl.SettleLottery)

		// 7. 量化预测算法
		api.POST("/simulate", ctrl.SimulateMonteCarlo)
		api.POST("/devig", ctrl.DevigOdds)
		api.POST("/kelly", ctrl.KellyAllocate)
		api.POST("/time-decay", ctrl.TimeDecayLive)
		api.GET("/arbitrage", ctrl.ScanArbitrage)

		// 8. 回测与新闻
		api.POST("/backtest/optimize", ctrl.OptimizeBacktest)
		api.GET("/backtest/history", ctrl.GetBacktestHistory)
		api.GET("/news", ctrl.GetNews)
		api.GET("/odds/shifts", ctrl.GetOddsShifts)
	}
}
