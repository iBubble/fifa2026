package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/ai"
	"log"
	"sync"
	"time"
)

type LiveSyncService struct {
	dcService       *DixonColesService
	backtestService *BacktestService
	ollamaService   *ai.OllamaService
	mu              sync.Mutex
}

func NewLiveSyncService(dc *DixonColesService, backtest *BacktestService, ollama *ai.OllamaService) *LiveSyncService {
	return &LiveSyncService{
		dcService:       dc,
		backtestService: backtest,
		ollamaService:   ollama,
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
				// 严格遵守诚实与防幻觉设计：在未接入真实实时数据源前，
				// 进行中 Live 状态比赛的即时比分应保持为初始 0:0，杜绝在运行中伪造假进球。
				m.HomeScore = 0
				m.AwayScore = 0
				_ = db.SaveMatch(m)
			} else {
				// 比赛完赛 FT
				// 严格禁止任何基于 Dixon-Coles 或大模型的虚假/幻觉完赛比分模拟
				// 完赛状态下比分应忠实保留为实际赛果（若未通过其他渠道录入则为 0:0）
				// 并且坚决不产生或写入任何虚假的复盘精度报告
				m.Status = "FT"
				_ = db.SaveMatch(m)
				log.Printf("[LiveSync] ⚽ 比赛 %s vs %s 已到完赛时间，已转为 FT 状态（不产生模拟比分及假复盘数据）", m.HomeTeam, m.AwayTeam)
			}
		}
	}
}
