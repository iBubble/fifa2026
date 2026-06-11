// Package service - match_service.go
// 并发轮询 API-Football (api-sports.io)，获取实时比分、首发阵容、统计数据和 xG 动能数据
// 严格控制请求频率：免费版每日 100 次请求限额
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
	"sync/atomic"
	"time"

	"football/internal/db"
	"football/internal/models"

	"github.com/wailsapp/wails/v3/pkg/application"
)

const (
	apiFootballBaseURL = "https://v3.football.api-sports.io"

	// ── 请求频率控制 ──
	// 免费版：100 次/天
	// 默认轮询间隔 15 分钟 = 96 次/天（安全范围内）
	// 有走地比赛时可加速到 5 分钟轮询
	defaultMatchPollingInterval = 15 * time.Minute
	liveMatchPollingInterval    = 5 * time.Minute
	dailyRequestLimit           = 100

	// 每次请求之间的最小间隔（防止突发大量请求）
	minRequestGap = 3 * time.Second
)

// MatchService 负责从 API-Football 并发拉取比赛数据
type MatchService struct {
	app        *application.App
	apiKey     string
	httpClient *http.Client
	cancel     context.CancelFunc
	mu         sync.Mutex
	running    bool

	// 当前活跃联赛
	activeLeagueID int    // API-Football league ID
	activeSeason   int    // 赛季年份
	activeLeague   string // 联赛名称（用于日志）

	// ── 请求频率控制 ──
	dailyRequests  atomic.Int64  // 今日已使用请求次数
	dailyResetDate string        // 上次重置日期 (YYYY-MM-DD)
	lastRequestAt  time.Time     // 上次请求时间
	requestMu      sync.Mutex    // 请求互斥锁
}

// NewMatchService 构造函数
func NewMatchService(app *application.App) *MatchService {
	ms := &MatchService{
		app:        app,
		apiKey:     os.Getenv("APIFOOTBALL_KEY"),
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}

	// 从环境变量或联赛注册表设定初始联赛
	sportKey := os.Getenv("THEODDSAPI_SPORT")
	if sportKey == "" {
		sportKey = "soccer_epl"
	}
	if league, ok := GetLeagueByKey(sportKey); ok {
		ms.activeLeagueID = league.APIFootballID
		ms.activeSeason = league.Season
		ms.activeLeague = league.Name
	} else {
		ms.activeLeagueID = 39 // 默认英超
		ms.activeSeason = 2025
		ms.activeLeague = "英超"
	}

	return ms
}

// SetActiveLeague 切换当前监控的联赛（前端调用），会重启轮询
func (s *MatchService) SetActiveLeague(sportKey string) {
	league, ok := GetLeagueByKey(sportKey)
	if !ok {
		s.emitLog(models.LogWarning, fmt.Sprintf("未知联赛 key: %s, 忽略切换", sportKey), "MatchService")
		return
	}

	s.mu.Lock()
	oldName := s.activeLeague
	s.activeLeagueID = league.APIFootballID
	s.activeSeason = league.Season
	s.activeLeague = league.Name
	wasRunning := s.running
	s.mu.Unlock()

	log.Printf("[MatchService] 🔄 切换联赛: %s → %s (leagueID=%d, season=%d)",
		oldName, league.Name, league.APIFootballID, league.Season)

	// 若正在运行，重启轮询以立即拉取新联赛数据
	if wasRunning {
		s.Stop()
		s.Start()
	}
}

// Start 启动比赛数据轮询
func (s *MatchService) Start() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.running = true

	go s.pollLoop(ctx)
	s.emitLog(models.LogSuccess,
		fmt.Sprintf("启动 [%s] 比赛数据轮询，默认间隔: %v (走地加速: %v)，每日限额: %d 次",
			s.activeLeague, defaultMatchPollingInterval, liveMatchPollingInterval, dailyRequestLimit),
		"MatchService")
}

// Stop 停止轮询
func (s *MatchService) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
	log.Println("[MatchService] ⏹ 停止比赛数据轮询")
}

// pollLoop 轮询主循环，根据是否有走地比赛动态调整间隔
func (s *MatchService) pollLoop(ctx context.Context) {
	hasLive := s.fetchAndEmit(ctx)

	for {
		// 动态计算下次轮询间隔
		interval := defaultMatchPollingInterval
		if hasLive {
			interval = liveMatchPollingInterval
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			hasLive = s.fetchAndEmit(ctx)
		}
	}
}

// getDailyRemaining 获取今日剩余可用请求数（自动按日重置计数器）
func (s *MatchService) getDailyRemaining() int64 {
	today := time.Now().Format("2006-01-02")
	s.requestMu.Lock()
	if s.dailyResetDate != today {
		s.dailyRequests.Store(0)
		s.dailyResetDate = today
		log.Printf("[MatchService] 📅 新一天，API 请求计数器已重置 (日期: %s)", today)
	}
	s.requestMu.Unlock()

	return int64(dailyRequestLimit) - s.dailyRequests.Load()
}

// consumeRequest 消耗一次请求配额，返回 false 表示已达限额
func (s *MatchService) consumeRequest() bool {
	if s.getDailyRemaining() <= 0 {
		return false
	}

	// 确保两次请求之间有最小间隔
	s.requestMu.Lock()
	elapsed := time.Since(s.lastRequestAt)
	if elapsed < minRequestGap {
		time.Sleep(minRequestGap - elapsed)
	}
	s.lastRequestAt = time.Now()
	s.requestMu.Unlock()

	s.dailyRequests.Add(1)
	return true
}

// fetchAndEmit 拉取今日比赛数据，返回是否有走地中的比赛
func (s *MatchService) fetchAndEmit(ctx context.Context) bool {
	if s.apiKey == "" || s.apiKey == "YOUR_API_FOOTBALL_KEY_HERE" {
		s.emitLog(models.LogWarning,
			fmt.Sprintf("APIFOOTBALL_KEY 未设置，[%s] 使用本地缓存数据", s.activeLeague),
			"MatchService")
		s.emitCachedMatches()
		return false
	}

	// 1. 自动检测并同步状态滞留的历史比赛（如已结束的欧冠决赛等）
	s.checkAndSyncPastMatches(ctx)

	remaining := s.getDailyRemaining()
	if remaining <= 5 {
		s.emitLog(models.LogWarning,
			fmt.Sprintf("⚠️ API-Football 今日请求配额即将耗尽 (剩余: %d/%d)，暂停 API 请求，使用本地缓存",
				remaining, dailyRequestLimit),
			"MatchService")
		s.emitCachedMatches()
		return false
	}

	// 消耗 1 次配额拉取赛程
	if !s.consumeRequest() {
		s.emitLog(models.LogError, "今日 API 请求配额已耗尽", "MatchService")
		return false
	}

	s.mu.Lock()
	leagueID := s.activeLeagueID
	season := s.activeSeason
	leagueName := s.activeLeague
	s.mu.Unlock()

	// 拉取今日比赛列表（使用 UTC 时间对齐 API Calendar 日期，防止本地时区跨子夜导致的数据时差）
	today := time.Now().UTC().Format("2006-01-02")
	fixtures, err := s.fetchFixtures(ctx, today, leagueID, season)
	if err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("[%s] 获取比赛列表失败: %v", leagueName, err), "MatchService")
		return false
	}

	hasLive := false

	// 处理每场比赛（串行化以节省配额，不再并发请求额外的统计接口）
	for _, fixture := range fixtures {
		s.processFixture(ctx, fixture)
		if status := fixture.Fixture.Status.Short; status == "1H" || status == "2H" || status == "HT" {
			hasLive = true
		}
	}

	used := s.dailyRequests.Load()
	s.emitLog(models.LogInfo,
		fmt.Sprintf("[%s] 比赛数据更新完成，共 %d 场 | API 配额: %d/%d (剩余 %d)",
			leagueName, len(fixtures), used, dailyRequestLimit, int64(dailyRequestLimit)-used),
		"MatchService")

	if hasLive {
		s.emitLog(models.LogSuccess,
			fmt.Sprintf("🔴 检测到 [%s] 走地中的比赛，轮询间隔自动加速为 %v", leagueName, liveMatchPollingInterval),
			"MatchService")
	}

	return hasLive
}

// processFixture 处理单场比赛：更新DB + 推送事件 + 提取xG
func (s *MatchService) processFixture(ctx context.Context, f models.APIFootballFixture) {
	homeScore := 0
	if f.Goals.Home != nil {
		homeScore = *f.Goals.Home
	}
	awayScore := 0
	if f.Goals.Away != nil {
		awayScore = *f.Goals.Away
	}

	scheduledAt, _ := time.Parse(time.RFC3339, f.Fixture.Date)
	match := models.Match{
		ID:          strconv.Itoa(f.Fixture.ID),
		HomeTeam:    f.Teams.Home.Name,
		AwayTeam:    f.Teams.Away.Name,
		League:      f.League.Name,
		Country:     f.League.Country,
		ScheduledAt: scheduledAt,
		Status:      f.Fixture.Status.Short,
		HomeScore:   homeScore,
		AwayScore:   awayScore,
		Minute:      f.Fixture.Status.Elapsed,
	}

	// 存入数据库
	if err := db.UpsertMatch(match); err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("存储比赛数据失败[%s]: %v", match.ID, err), "MatchService")
		return
	}

	// 推送比赛更新事件
	s.app.Event.Emit("match:update", match)

	// 若比赛进行中，提取 xG 数据
	if match.Status == "1H" || match.Status == "2H" || match.Status == "HT" {
		s.extractAndEmitXG(ctx, match, f)
	}
}

// extractAndEmitXG 从统计数据中提取 xG 并构建时间序列推送
func (s *MatchService) extractAndEmitXG(ctx context.Context, match models.Match, f models.APIFootballFixture) {
	var homeXG, awayXG float64

	for _, teamStat := range f.Statistics {
		for _, stat := range teamStat.Stats {
			if stat.Type == "expected_goals" {
				var xg float64
				switch v := stat.Value.(type) {
				case float64:
					xg = v
				case string:
					xg, _ = strconv.ParseFloat(v, 64)
				}
				if teamStat.Team.Name == match.HomeTeam {
					homeXG = xg
				} else {
					awayXG = xg
				}
			}
		}
	}

	// 构造当前时刻的 xG 数据点
	point := models.XGDataPoint{
		Minute:    match.Minute,
		HomeXG:    homeXG,
		AwayXG:    awayXG,
		HomeCumXG: homeXG,
		AwayCumXG: awayXG,
	}

	xgUpdate := models.XGUpdate{
		MatchID: match.ID,
		Points:  []models.XGDataPoint{point},
	}

	s.app.Event.Emit("xg:update", xgUpdate)
}

// fetchFixtures 调用 API-Football 获取指定日期/或全赛季+联赛的比赛列表
func (s *MatchService) fetchFixtures(ctx context.Context, date string, leagueID, season int) ([]models.APIFootballFixture, error) {
	var url string
	if date != "" {
		url = fmt.Sprintf("%s/fixtures?date=%s&league=%d&season=%d",
			apiFootballBaseURL, date, leagueID, season)
	} else {
		url = fmt.Sprintf("%s/fixtures?league=%d&season=%d",
			apiFootballBaseURL, leagueID, season)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apisports-key", s.apiKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 返回错误 %d: %s", resp.StatusCode, string(body))
	}

	// API-Football 响应外层包裹 {"response": [...]}
	var wrapper struct {
		Response []models.APIFootballFixture `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}
	return wrapper.Response, nil
}

// emitLog 推送日志到前端
func (s *MatchService) emitLog(level models.LogLevel, msg, source string) {
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

// emitCachedMatches 从数据库加载已缓存的比赛数据并推送给前端（API 不可用时的兜底逻辑）
func (s *MatchService) emitCachedMatches() {
	// 清除历史遗留的模拟比赛
	if err := db.DeleteMockMatches(); err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("清除模拟比赛失败: %v", err), "MatchService")
	}

	allMatches, err := db.GetMatches()
	if err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("获取本地比赛数据失败: %v", err), "MatchService")
		return
	}

	var realMatches []models.Match
	for _, m := range allMatches {
		if !strings.HasPrefix(m.ID, "wc2026-") {
			realMatches = append(realMatches, m)
		}
	}

	for _, m := range realMatches {
		s.app.Event.Emit("match:update", m)
	}

	s.emitLog(models.LogInfo,
		fmt.Sprintf("[%s] 使用本地缓存数据，共 %d 场比赛", s.activeLeague, len(realMatches)),
		"MatchService")
}

// GetDailyRequestStats 获取每日请求统计（前端展示用）
func (s *MatchService) GetDailyRequestStats() (used int64, limit int, remaining int64) {
	used = s.dailyRequests.Load()
	limit = dailyRequestLimit
	remaining = s.getDailyRemaining()
	return
}

// checkAndSyncPastMatches 检查是否有未结束的历史比赛，并拉取全赛季赛程进行更新
func (s *MatchService) checkAndSyncPastMatches(ctx context.Context) {
	s.mu.Lock()
	leagueID := s.activeLeagueID
	season := s.activeSeason
	leagueName := s.activeLeague
	s.mu.Unlock()

	// 从本地数据库读取当前联赛的所有比赛
	allMatches, err := db.GetMatchesByLeague(leagueName)
	if err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("检查历史比赛失败: %v", err), "MatchService")
		return
	}

	now := time.Now()
	var stuckMatches []models.Match
	for _, m := range allMatches {
		// 忽略已结束的比赛状态
		status := strings.ToUpper(m.Status)
		if status == "FT" || status == "AET" || status == "PEN" || status == "CANC" || status == "PST" || status == "ABD" {
			continue
		}
		// 开赛时间超过 2 小时，但状态仍未标记为结束，判定为需要更新的“滞留比赛”
		if m.ScheduledAt.Add(2 * time.Hour).Before(now) {
			stuckMatches = append(stuckMatches, m)
		}
	}

	if len(stuckMatches) == 0 {
		return
	}

	s.emitLog(models.LogInfo,
		fmt.Sprintf("⚠️ 检测到 [%s] 有 %d 场历史比赛状态未结束，正在发起全赛季同步...", leagueName, len(stuckMatches)),
		"MatchService")

	// 消耗一次 API 额度
	if !s.consumeRequest() {
		s.emitLog(models.LogError, "无法执行历史比赛更新：今日 API 请求配额已耗尽", "MatchService")
		return
	}

	// 拉取该联赛该赛季的所有比赛
	fixtures, err := s.fetchFixtures(ctx, "", leagueID, season)
	if err != nil {
		s.emitLog(models.LogError, fmt.Sprintf("同步全赛季比赛数据失败: %v", err), "MatchService")
		return
	}

	updatedCount := 0
	for _, stuck := range stuckMatches {
		matched := false
		for _, f := range fixtures {
			// 1. 优先通过 API-Football ID 匹配
			if stuck.ID == strconv.Itoa(f.Fixture.ID) {
				matched = true
			} else {
				// 2. 队名 + 日期模糊匹配（容忍24小时时区误差）
				scheduledAt, parseErr := time.Parse(time.RFC3339, f.Fixture.Date)
				if parseErr != nil {
					continue
				}
				dateDiff := scheduledAt.Sub(stuck.ScheduledAt)
				if dateDiff < 0 {
					dateDiff = -dateDiff
				}
				if dateDiff < 24*time.Hour && teamNamesMatch(stuck.HomeTeam, f.Teams.Home.Name) && teamNamesMatch(stuck.AwayTeam, f.Teams.Away.Name) {
					matched = true
				}
			}

			if matched {
				homeScore := 0
				if f.Goals.Home != nil {
					homeScore = *f.Goals.Home
				}
				awayScore := 0
				if f.Goals.Away != nil {
					awayScore = *f.Goals.Away
				}

				stuck.Status = f.Fixture.Status.Short
				stuck.HomeScore = homeScore
				stuck.AwayScore = awayScore
				stuck.Minute = f.Fixture.Status.Elapsed

				if err := db.UpsertMatch(stuck); err != nil {
					s.emitLog(models.LogError, fmt.Sprintf("更新历史比赛失败[%s]: %v", stuck.ID, err), "MatchService")
				} else {
					updatedCount++
					s.emitLog(models.LogSuccess,
						fmt.Sprintf("✓ 成功更新历史比赛: %s %d-%d %s (%s)",
							stuck.HomeTeam, stuck.HomeScore, stuck.AwayScore, stuck.AwayTeam, stuck.Status),
						"MatchService")
					if s.app != nil && s.app.Event != nil {
						s.app.Event.Emit("match:update", stuck)
					}
				}
				break
			}
		}
	}

	s.emitLog(models.LogSuccess,
		fmt.Sprintf("历史比赛同步完成，成功更新 %d/%d 场比赛", updatedCount, len(stuckMatches)),
		"MatchService")
}

func normalizeTeamName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "fc", "")
	name = strings.ReplaceAll(name, "cf", "")
	name = strings.ReplaceAll(name, "united", "")
	name = strings.ReplaceAll(name, "utd", "")
	return name
}

func teamNamesMatch(a, b string) bool {
	na := normalizeTeamName(a)
	nb := normalizeTeamName(b)
	return na == nb || strings.Contains(na, nb) || strings.Contains(nb, na)
}
