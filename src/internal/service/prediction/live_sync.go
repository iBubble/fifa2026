package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/ai"
	"log"
	"math/rand"
	"sync"
	"time"
)

type LiveSyncService struct {
	dcService       *DixonColesService
	backtestService *BacktestService
	ollamaService   *ai.OllamaService
	matchEvents     map[string]*MatchEventTimeline
	mu              sync.Mutex
}

type MatchEventTimeline struct {
	HomeGoals []int
	AwayGoals []int
	FinalHome int
	FinalAway int
}

func NewLiveSyncService(dc *DixonColesService, backtest *BacktestService, ollama *ai.OllamaService) *LiveSyncService {
	return &LiveSyncService{
		dcService:       dc,
		backtestService: backtest,
		ollamaService:   ollama,
		matchEvents:     make(map[string]*MatchEventTimeline),
	}
}

// StartSyncLoop 开启常驻后台轮询协程 (每10秒扫描一次比赛状态)
func (s *LiveSyncService) StartSyncLoop() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			s.SyncMatches()
		}
	}()
}

// SyncMatches 根据比赛时间自动演化进行中 Live 比分或 FT 完赛结果并触发复盘优化
func (s *LiveSyncService) SyncMatches() {
	s.mu.Lock()
	defer s.mu.Unlock()

	matches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		return
	}

	now := time.Now()
	for _, m := range matches {
		if m.Status == "FT" {
			continue
		}

		// 若当前时间超过比赛设定的 ScheduledAt，则说明比赛开始
		if now.After(m.ScheduledAt) {
			elapsed := now.Sub(m.ScheduledAt)
			if elapsed < 105*time.Minute {
				// 比赛进行中 Live
				m.Status = "Live"
				timeline, ok := s.matchEvents[m.ID]
				if !ok {
					timeline = s.createTimeline(m)
					s.matchEvents[m.ID] = timeline
				}
				minutes := int(elapsed.Minutes())
				if minutes > 90 {
					minutes = 90
				}
				m.HomeScore = countGoals(timeline.HomeGoals, minutes)
				m.AwayScore = countGoals(timeline.AwayGoals, minutes)
				_ = db.SaveMatch(m)
			} else {
				// 比赛完赛 FT
				m.Status = "FT"
				timeline, ok := s.matchEvents[m.ID]
				if ok {
					m.HomeScore = timeline.FinalHome
					m.AwayScore = timeline.FinalAway
					delete(s.matchEvents, m.ID)
				} else {
					t := s.createTimeline(m)
					m.HomeScore = t.FinalHome
					m.AwayScore = t.FinalAway
				}
				_ = db.SaveMatch(m)
				log.Printf("[LiveSync] ⚽ 完赛状态变更: %s vs %s (%d:%d)，立即触发自动复盘与权重优化...", m.HomeTeam, m.AwayTeam, m.HomeScore, m.AwayScore)

				go func(match models.Match) {
					params := s.dcService.CalculateParams(match.HomeTeam, match.AwayTeam)
					matrix, over25, under25 := s.dcService.GenerateProbabilityMatrix(params)
					rep := models.PredictionReport{
						MatchID:        match.ID,
						OriginalParams: params,
						RefinedParams:  params,
						ScoreMatrix:    matrix,
						Over2_5Prob:    over25,
						Under2_5Prob:   under25,
					}
					_, errReview := s.backtestService.ReviewMatch(match, &rep)
					if errReview != nil {
						log.Printf("[LiveSync] ⚠️ 自动复盘权重更新失败: %v", errReview)
					} else {
						log.Printf("[LiveSync] ✅ 自动复盘成功，已在线纠偏两队 Elo 实力特征")
					}
				}(m)
			}
		}
	}
}

func countGoals(goals []int, currentMinute int) int {
	count := 0
	for _, g := range goals {
		if g <= currentMinute {
			count++
		}
	}
	return count
}

func (s *LiveSyncService) createTimeline(m models.Match) *MatchEventTimeline {
	params := s.dcService.CalculateParams(m.HomeTeam, m.AwayTeam)
	matrix, _, _ := s.dcService.GenerateProbabilityMatrix(params)

	rVal := rand.New(rand.NewSource(time.Now().UnixNano())).Float64()
	var cumulative float64
	finalHome, finalAway := 1, 0
	for _, cell := range matrix {
		cumulative += cell.Prob
		if rVal <= cumulative {
			finalHome = cell.HomeScore
			finalAway = cell.AwayScore
			break
		}
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	homeGoals := make([]int, finalHome)
	for i := 0; i < finalHome; i++ {
		homeGoals[i] = r.Intn(90) + 1
	}
	awayGoals := make([]int, finalAway)
	for i := 0; i < finalAway; i++ {
		awayGoals[i] = r.Intn(90) + 1
	}

	return &MatchEventTimeline{
		HomeGoals: homeGoals,
		AwayGoals: awayGoals,
		FinalHome: finalHome,
		FinalAway: finalAway,
	}
}
