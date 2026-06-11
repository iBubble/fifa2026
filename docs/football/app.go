// app.go - Wails v3 应用主结构体
// 负责初始化所有 Service，绑定所有暴露给前端的方法，以及日志重定向
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"football/internal/db"
	"football/internal/models"
	"football/internal/service"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// App Wails 应用主结构体（所有公开方法均可被前端直接调用）
type App struct {
	wailsApp   *application.App
	prediction *service.PredictionService
	odds       *service.OddsService
	matches    *service.MatchService
	ai         *service.AIService
}

// NewApp 构造函数（在 main.go 中调用）
func NewApp() *App {
	return &App{}
}

// ServiceStartup Wails v3 生命周期：应用启动时执行
// 签名必须为 ServiceStartup(ctx context.Context, options application.ServiceOptions) error
func (a *App) ServiceStartup(ctx context.Context, options application.ServiceOptions) error {
	// 1. 初始化 SQLite 数据库
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".football")
	if err := db.Init(dataDir); err != nil {
		return fmt.Errorf("数据库初始化失败: %w", err)
	}

	// 2. 设置日志重定向到前端 Terminal 页面
	logWriter := &wailsLogWriter{app: a.wailsApp}
	log.SetOutput(io.MultiWriter(os.Stdout, logWriter))
	log.SetFlags(log.Ltime | log.Lmicroseconds)

	// 3. 初始化 Services
	a.prediction = service.NewPredictionService()
	a.odds = service.NewOddsService(a.wailsApp, a.prediction)
	a.matches = service.NewMatchService(a.wailsApp)
	a.ai = service.NewAIService()

	// 4. 启动后台并发轮询
	a.odds.Start()
	a.matches.Start()

	log.Println("[App] ✅ 足球量化分析终端已启动")
	return nil
}

// ServiceShutdown Wails v3 生命周期：应用退出时执行（清理资源）
func (a *App) ServiceShutdown() error {
	log.Println("[App] 正在关闭...")
	if a.odds != nil {
		a.odds.Stop()
	}
	if a.matches != nil {
		a.matches.Stop()
	}
	db.Close()
	log.Println("[App] ✅ 资源已释放")
	return nil
}

// ─────────────────────────────────────────────────────────────
// 暴露给前端的方法（通过 wails3 generate bindings 生成 TS 类型）
// ─────────────────────────────────────────────────────────────

// GetMatches 获取所有比赛列表（前端启动时调用，获取初始数据）
func (a *App) GetMatches() ([]models.Match, error) {
	return db.GetMatches()
}

// GetOddsHistory 获取某场比赛的历史赔率（最近100条）
func (a *App) GetOddsHistory(matchID string) ([]models.OddsSnapshot, error) {
	return db.GetOddsHistory(matchID, 100)
}

// CalculateKelly 凯利公式计算（前端 AI 沙盒调用）
func (a *App) CalculateKelly(params models.KellyParams) (models.KellyResult, error) {
	result, err := a.prediction.CalculateKelly(params)
	if err != nil {
		return models.KellyResult{}, fmt.Errorf("凯利公式计算失败: %w", err)
	}
	return result, nil
}

// GetBets 获取所有投注记录
func (a *App) GetBets() ([]models.Bet, error) {
	return db.GetBets()
}

// AddBet 新增一条投注记录
func (a *App) AddBet(bet models.Bet) (int64, error) {
	id, err := db.AddBet(bet)
	if err != nil {
		return 0, fmt.Errorf("添加投注失败: %w", err)
	}
	log.Printf("[App] 新增投注: %s vs %s | %s | 赔率%.2f | 金额%.2f",
		bet.HomeTeam, bet.AwayTeam, bet.Outcome, bet.Odds, bet.Stake)
	return id, nil
}

// UpdateBetResult 结算投注
func (a *App) UpdateBetResult(betID int64, result string, pnl float64) error {
	return db.UpdateBetResult(betID, models.BetResult(result), pnl)
}

// GetBetSummary 获取账本汇总统计
func (a *App) GetBetSummary() (models.BetSummary, error) {
	return db.GetBetSummary()
}

// ─────────────────────────────────────────────────────────────
// 多联赛管理（前端联赛选择器调用）
// ─────────────────────────────────────────────────────────────

// GetLeagues 获取所有支持的联赛列表（前端初始化联赛选择器时调用）
func (a *App) GetLeagues() []models.League {
	return service.GetAllLeagues()
}

// SetActiveLeague 切换当前监控的联赛（前端联赛选择器调用）
// 会同时切换赔率服务和比赛服务的目标联赛
func (a *App) SetActiveLeague(sportKey string) error {
	_, ok := service.GetLeagueByKey(sportKey)
	if !ok {
		return fmt.Errorf("不支持的联赛: %s", sportKey)
	}

	if a.odds != nil {
		a.odds.SetActiveLeague(sportKey)
	}
	if a.matches != nil {
		a.matches.SetActiveLeague(sportKey)
	}

	log.Printf("[App] 🔄 已切换活跃联赛: %s", sportKey)
	return nil
}

// GetActiveLeague 获取当前活跃联赛的 sportKey
func (a *App) GetActiveLeague() string {
	if a.odds != nil {
		return a.odds.GetActiveSportKey()
	}
	return ""
}

// GetMatchesByLeague 按联赛名称过滤获取比赛
func (a *App) GetMatchesByLeague(league string) ([]models.Match, error) {
	return db.GetMatchesByLeague(league)
}

// GetLLMAnalysis 获取指定比赛的大模型深度量化预测报告
func (a *App) GetLLMAnalysis(matchID string, oddsHome, oddsDraw, oddsAway float64) (string, error) {
	// 1. 从 DB 读取 match
	matches, err := db.GetMatches()
	if err != nil {
		return "", fmt.Errorf("读取比赛失败: %w", err)
	}

	var match models.Match
	found := false
	for _, m := range matches {
		if m.ID == matchID {
			match = m
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("比赛ID %s 不存在", matchID)
	}

	// 2. 调用 AIService
	return a.ai.GetLLMAnalysis(context.Background(), match, oddsHome, oddsDraw, oddsAway)
}

// APIQuotaStatus API 配额状态（前端展示用）
type APIQuotaStatus struct {
	OddsUsed     string `json:"oddsUsed"`     // The Odds API 使用描述
	MatchUsed    int64  `json:"matchUsed"`    // API-Football 今日已用次数
	MatchLimit   int    `json:"matchLimit"`   // API-Football 每日限额
	MatchRemain  int64  `json:"matchRemain"` // API-Football 今日剩余
}

// GetAPIQuotaStatus 获取 API 配额使用状态
func (a *App) GetAPIQuotaStatus() APIQuotaStatus {
	var status APIQuotaStatus
	if a.matches != nil {
		status.MatchUsed, status.MatchLimit, status.MatchRemain = a.matches.GetDailyRequestStats()
	}
	status.OddsUsed = "The Odds API: 合并请求模式"
	return status
}

// StartPolling 手动启动轮询（前端控制）
func (a *App) StartPolling() {
	if a.odds != nil {
		a.odds.Start()
	}
	if a.matches != nil {
		a.matches.Start()
	}
	log.Println("[App] 手动启动轮询")
}

// StopPolling 手动停止轮询（前端控制）
func (a *App) StopPolling() {
	if a.odds != nil {
		a.odds.Stop()
	}
	if a.matches != nil {
		a.matches.Stop()
	}
	log.Println("[App] 手动停止轮询")
}

// ─────────────────────────────────────────────────────────────
// 日志重定向：将 Go log 实时推送到前端 Terminal 页面
// ─────────────────────────────────────────────────────────────

// wailsLogWriter 实现 io.Writer 接口，将标准日志条目转发为 Wails 事件
type wailsLogWriter struct {
	app *application.App
}

// Write 捕获每次 log.Printf 的输出，推送 "log:entry" 事件
func (w *wailsLogWriter) Write(p []byte) (n int, err error) {
	if w.app == nil {
		return len(p), nil
	}
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	// 根据内容关键字自动判断日志级别
	level := models.LogInfo
	switch {
	case containsAny(msg, "ERROR", "FATAL", "错误", "失败"):
		level = models.LogError
	case containsAny(msg, "WARN", "WARNING", "警告"):
		level = models.LogWarning
	case containsAny(msg, "✅", "完成", "启动"):
		level = models.LogSuccess
	case containsAny(msg, "🎯", "套利"):
		level = models.LogSuccess
	}

	entry := models.LogEntry{
		Level:     level,
		Message:   msg,
		Source:    "System",
		Timestamp: time.Now(),
	}
	// Wails v3 API: app.Event.Emit
	w.app.Event.Emit("log:entry", entry)
	return len(p), nil
}

// containsAny 检查字符串是否包含任意关键字
func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		for i := 0; i <= len(s)-len(kw); i++ {
			if s[i:i+len(kw)] == kw {
				return true
			}
		}
	}
	return false
}
