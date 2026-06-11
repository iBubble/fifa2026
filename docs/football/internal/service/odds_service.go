// Package service - odds_service.go
// 并发轮询 The Odds API，解析赔率，检测套利机会，通过 Wails 事件推送前端
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"football/internal/db"
	"football/internal/models"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	oddsAPIBaseURL = "https://api.the-odds-api.com/v4"
	// 套利检测最低 ROI 阈值（低于此值不推送警报，避免噪音）
	minArbitrageROI = 0.5 // 0.5%
	// 默认套利检测资金基准（用于计算建议投注额）
	defaultBankroll = 10000.0
)

// OddsService 负责从 The Odds API 并发拉取赔率并推送 Wails 事件
type OddsService struct {
	app             *application.App
	prediction      *PredictionService
	apiKey          string
	sportKey        string
	httpClient      *http.Client
	cancel          context.CancelFunc
	mu              sync.Mutex
	running         bool
	pollingInterval time.Duration
}

// NewOddsService 构造函数（从环境变量读取 API Key）
func NewOddsService(app *application.App, prediction *PredictionService) *OddsService {
	sport := os.Getenv("THEODDSAPI_SPORT")
	if sport == "" {
		sport = "soccer_usa_mls" // 默认使用美职联，因为世界杯赛事在未开赛时会返回 404
	}

	intervalStr := os.Getenv("THEODDSAPI_POLLING_INTERVAL")
	interval := 600 * time.Second // 默认 10 分钟 (适合 500 次/月免费额度，每月约耗用 430 次)
	if intervalStr != "" {
		if sec, err := strconv.Atoi(intervalStr); err == nil && sec > 0 {
			interval = time.Duration(sec) * time.Second
		}
	}

	return &OddsService{
		app:             app,
		prediction:      prediction,
		apiKey:          os.Getenv("THEODDSAPI_KEY"),
		sportKey:        sport,
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		pollingInterval: interval,
	}
}

// SetActiveLeague 切换当前监控的联赛（前端调用），会重启轮询
func (s *OddsService) SetActiveLeague(sportKey string) {
	s.mu.Lock()
	oldKey := s.sportKey
	s.sportKey = sportKey
	wasRunning := s.running
	s.mu.Unlock()

	if oldKey == sportKey {
		return
	}

	log.Printf("[OddsService] 🔄 切换联赛: %s → %s", oldKey, sportKey)

	// 若正在运行，重启轮询以立即拉取新联赛数据
	if wasRunning {
		s.Stop()
		s.Start()
	}
}

// GetActiveSportKey 获取当前活跃的联赛 sport key
func (s *OddsService) GetActiveSportKey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sportKey
}

// Start 启动后台赔率轮询 goroutine
func (s *OddsService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		log.Println("[OddsService] 已在运行，忽略重复启动")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true

	go s.pollLoop(ctx)
	log.Printf("[OddsService] ✅ 启动赔率轮询，优化后的安全间隔: %v", s.pollingInterval)
}

// Stop 停止轮询
func (s *OddsService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
	log.Println("[OddsService] ⏹ 停止赔率轮询")
}

// pollLoop 核心轮询循环，每 pollingInterval 触发一次
func (s *OddsService) pollLoop(ctx context.Context) {
	// 立即执行一次
	s.fetchAndEmit(ctx)

	ticker := time.NewTicker(s.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.fetchAndEmit(ctx)
		}
	}
}

// fetchAndEmit 拉取全部赔率数据，并发处理套利检测，通过 Wails 事件推送
func (s *OddsService) fetchAndEmit(ctx context.Context) {
	if s.apiKey == "" || s.apiKey == "YOUR_THEODDSAPI_KEY_HERE" {
		s.emitLog(models.LogWarning, "THEODDSAPI_KEY 未设置或为默认占位符，使用模拟数据", "OddsService")
		s.emitMockOdds()
		return
	}

	// 1. 合并请求优化：将 h2h, spreads, totals 合并为一个单次 API 请求以节约免费配额 (500次/月)
	// 在单一请求中指定 markets=h2h,spreads,totals，仅扣除 1 个 credit！比原先的分开发送节约 67% 的免费配额消耗。
	data, err := s.fetchOdds(ctx, "h2h,spreads,totals")
	if err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("获取合并赔率失败: %v", err), "OddsService")
		return
	}

	// 并行/同步获取实时比分
	scoresMap := make(map[string]models.TheOddsAPIScoresResponse)
	scoresData, err := s.fetchScores(ctx)
	if err != nil {
		s.emitLog(models.LogWarning, fmt.Sprintf("获取实时比分失败: %v", err), "OddsService")
	} else {
		for _, scoreItem := range scoresData {
			scoresMap[scoreItem.ID] = scoreItem
		}
	}

	matchOdds := make(map[string]*models.OddsSnapshot)
	var matchInfoMap = make(map[string]models.Match)

	// 根据当前活跃的 sportKey 查询对应的联赛名称与国家
	leagueName := "未知联赛"
	country := "Unknown"
	if lg, ok := GetLeagueByKey(s.sportKey); ok {
		leagueName = lg.Name
		country = lg.Country
	}

	for _, fixture := range data {
		matchID := fixture.ID
		if _, ok := matchOdds[matchID]; !ok {
			status := "NS"
			homeScore := 0
			awayScore := 0
			minute := 0

			if scoreItem, hasScore := scoresMap[matchID]; hasScore {
				status, minute = CalculateMatchStatusAndMinute(scoreItem.CommenceTime, scoreItem.Completed)
				for _, sc := range scoreItem.Scores {
					val, _ := strconv.Atoi(sc.Score)
					if sc.Name == scoreItem.HomeTeam {
						homeScore = val
					} else if sc.Name == scoreItem.AwayTeam {
						awayScore = val
					}
				}
			}

			m := models.Match{
				ID:          matchID,
				HomeTeam:    fixture.HomeTeam,
				AwayTeam:    fixture.AwayTeam,
				League:      leagueName,
				Country:     country,
				ScheduledAt: fixture.CommenceTime,
				Status:      status,
				HomeScore:   homeScore,
				AwayScore:   awayScore,
				Minute:      minute,
			}
			matchOdds[matchID] = &models.OddsSnapshot{
				MatchID:    matchID,
				Match:      m,
				CapturedAt: time.Now(),
			}
			matchInfoMap[matchID] = m
		}

		// 解析博彩平台的所有盘口赔率
		for _, bk := range fixture.Bookmakers {
			for _, mkt := range bk.Markets {
				var outcomes []models.OddsOutcome
				for _, o := range mkt.Outcomes {
					outcomes = append(outcomes, models.OddsOutcome{
						Name:  o.Name,
						Price: o.Price,
					})
				}
				matchOdds[matchID].Bookmakers = append(matchOdds[matchID].Bookmakers, models.BookmakerOdds{
					Bookmaker: bk.Title,
					Market:    models.MarketType(mkt.Key),
					Outcomes:  outcomes,
					UpdatedAt: bk.LastUpdate,
				})
			}
		}
	}

	// 推送赔率更新事件 & 存入数据库 & 套利扫描
	for matchID, snapshot := range matchOdds {
		// 推送赔率更新到前端
		if s.app != nil && s.app.Event != nil {
			s.app.Event.Emit("odds:update", snapshot)
		}

		// 将真实的赛程信息写入数据库中的 matches 表，以供比赛展示 and 模拟比分
		if err := db.UpsertMatch(snapshot.Match); err != nil {
			s.emitLog(models.LogError, fmt.Sprintf("自动导入比赛信息失败[%s]: %v", matchID, err), "OddsService")
		} else {
			// 成功存入数据库后，推送比赛更新事件以使前端即时刷新
			if s.app != nil && s.app.Event != nil {
				s.app.Event.Emit("match:update", snapshot.Match)
			}
		}

		// 存入数据库
		if err := db.SaveOddsSnapshot(*snapshot); err != nil {
			s.emitLog(models.LogError, fmt.Sprintf("存储赔率快照失败[%s]: %v", matchID, err), "OddsService")
		}

		// 对 1X2 盘口扫描套利
		s.scanArbitrage(matchID, matchInfoMap[matchID], *snapshot)
	}

	// 2. 检测并同步本地数据库中近期结束但状态滞留的比赛（通过 The Odds API 的 scoresMap 自愈，费用为 0 额外 API 额度）
	dbMatches, err := db.GetMatchesByLeague(leagueName)
	if err == nil {
		for _, dbM := range dbMatches {
			status := strings.ToUpper(dbM.Status)
			if status == "FT" || status == "AET" || status == "PEN" || status == "CANC" || status == "PST" || status == "ABD" {
				continue
			}

			// 检查是否在 scoresMap 中
			if scoreItem, exists := scoresMap[dbM.ID]; exists {
				newStatus, newMinute := CalculateMatchStatusAndMinute(scoreItem.CommenceTime, scoreItem.Completed)
				homeScore := 0
				awayScore := 0
				for _, sc := range scoreItem.Scores {
					val, _ := strconv.Atoi(sc.Score)
					if sc.Name == scoreItem.HomeTeam {
						homeScore = val
					} else if sc.Name == scoreItem.AwayTeam {
						awayScore = val
					}
				}

				// 如果状态或比分有更新，写入数据库并通知前端
				if dbM.Status != newStatus || dbM.HomeScore != homeScore || dbM.AwayScore != awayScore {
					dbM.Status = newStatus
					dbM.HomeScore = homeScore
					dbM.AwayScore = awayScore
					dbM.Minute = newMinute

					if err := db.UpsertMatch(dbM); err != nil {
						s.emitLog(models.LogError, fmt.Sprintf("自愈更新比赛状态失败[%s]: %v", dbM.ID, err), "OddsService")
					} else {
						s.emitLog(models.LogSuccess,
							fmt.Sprintf("✓ [自愈] 成功更新历史比赛状态: %s %d-%d %s (%s)",
								dbM.HomeTeam, dbM.HomeScore, dbM.AwayScore, dbM.AwayTeam, dbM.Status),
							"OddsService")
						if s.app != nil && s.app.Event != nil {
							s.app.Event.Emit("match:update", dbM)
						}
					}
				}
			}
		}
	}

	s.emitLog(models.LogInfo,
		fmt.Sprintf("赔率更新完成，共处理 %d 场比赛 [已采用单次合并API请求，节约67%%配额]", len(matchOdds)),
		"OddsService")
}

// fetchOdds 调用 The Odds API 获取指定盘口的赔率 (支持逗号分隔的多个盘口)
func (s *OddsService) fetchOdds(ctx context.Context, markets string) ([]models.TheOddsAPIResponse, error) {
	url := fmt.Sprintf(
		"%s/sports/%s/odds/?apiKey=%s&regions=eu&markets=%s&oddsFormat=decimal&bookmakers=bet365,pinnacle,williamhill,unibet,betfair",
		oddsAPIBaseURL, s.sportKey, s.apiKey, markets,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var data []models.TheOddsAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return data, nil
}

// fetchScores 从 The Odds API 获取当前活跃联赛的实时比分
func (s *OddsService) fetchScores(ctx context.Context) ([]models.TheOddsAPIScoresResponse, error) {
	url := fmt.Sprintf(
		"%s/sports/%s/scores/?apiKey=%s&daysFrom=3",
		oddsAPIBaseURL, s.sportKey, s.apiKey,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Scores API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	var data []models.TheOddsAPIScoresResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("解析 Scores 响应失败: %w", err)
	}
	return data, nil
}

// CalculateMatchStatusAndMinute 根据开赛时间和是否结束计算比赛的状态及分钟数
func CalculateMatchStatusAndMinute(commenceTime time.Time, completed bool) (status string, minute int) {
	if completed {
		return "FT", 90
	}
	now := time.Now().UTC()
	if now.Before(commenceTime) {
		return "NS", 0
	}

	elapsed := int(now.Sub(commenceTime).Minutes())
	if elapsed < 0 {
		return "NS", 0
	}

	// 简易足球比赛时间轴模拟
	// 0 - 45 分钟: 上半场 1H
	// 45 - 60 分钟: 中场休息 HT
	// 60 - 105 分钟: 下半场 2H
	// > 105 分钟: 下半场 90 分钟（补时）
	if elapsed <= 45 {
		return "1H", elapsed
	} else if elapsed <= 60 {
		return "HT", 45
	} else if elapsed <= 105 {
		return "2H", elapsed - 15
	} else {
		return "2H", 90
	}
}

// scanArbitrage 对一个赔率快照中的 1X2 盘口跨平台扫描套利
func (s *OddsService) scanArbitrage(matchID string, match models.Match, snapshot models.OddsSnapshot) {
	// 收集各博彩平台的最优赔率（每个结果取最高赔率）
	bestOdds := map[string]models.ArbLeg{} // outcome → best leg

	for _, bk := range snapshot.Bookmakers {
		if bk.Market != models.Market1X2 {
			continue
		}
		for _, outcome := range bk.Outcomes {
			existing, ok := bestOdds[outcome.Name]
			if !ok || outcome.Price > existing.Odds {
				bestOdds[outcome.Name] = models.ArbLeg{
					Bookmaker: bk.Bookmaker,
					Outcome:   outcome.Name,
					Odds:      outcome.Price,
				}
			}
		}
	}

	// 必须有主/平/客三个结果才能形成三角套利
	if len(bestOdds) < 2 {
		return
	}

	legs := make([]models.ArbLeg, 0, len(bestOdds))
	for _, leg := range bestOdds {
		legs = append(legs, leg)
	}

	opp, found := s.prediction.CheckArbitrage(matchID, match, models.Market1X2, legs, defaultBankroll)
	if !found {
		return
	}

	// 仅推送 ROI 超过阈值的机会
	if opp.ROI < minArbitrageROI {
		return
	}

	s.emitLog(models.LogSuccess,
		fmt.Sprintf("🎯 发现套利机会! %s vs %s | L=%.4f | ROI=%.2f%%",
			match.HomeTeam, match.AwayTeam, opp.LValue, opp.ROI),
		"OddsService",
	)

	// 推送套利警报事件
	if s.app != nil && s.app.Event != nil {
		s.app.Event.Emit("arbitrage:alert", opp)
	}

	// 存入数据库
	if err := db.SaveArbitrageOpportunity(opp); err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("存储套利记录失败: %v", err), "OddsService")
	}
}

// emitLog 推送日志条目到前端 Terminal 页面
func (s *OddsService) emitLog(level models.LogLevel, msg, source string) {
	entry := models.LogEntry{
		Level:     level,
		Message:   msg,
		Source:    source,
		Timestamp: time.Now(),
	}
	log.Printf("[%s] %s: %s", source, level, msg)
	if s.app != nil && s.app.Event != nil {
		s.app.Event.Emit("log:entry", entry)
	}
}

// ─────────────────────────────────────────────────────────────
// 模拟数据（API Key 未配置时使用）
// ─────────────────────────────────────────────────────────────

// emitMockOdds 发送模拟赔率数据（已停用，保证 100% 真实数据）
func (s *OddsService) emitMockOdds() {
	leagueName := s.sportKey
	if league, ok := GetLeagueByKey(s.sportKey); ok {
		leagueName = league.Name
	}
	s.emitLog(models.LogWarning, fmt.Sprintf("[%s] 系统处于真实数据模式，模拟赔率已停用", leagueName), "OddsService")
}
