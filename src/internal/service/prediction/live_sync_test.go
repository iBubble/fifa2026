package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/ai"
	"os"
	"testing"
	"time"
)

func TestLiveSyncService(t *testing.T) {
	// 初始化测试数据库前，先尝试清理残留
	_ = os.Remove("./test_sync.db")
	_ = os.Remove("./fifa2026.db")
	_ = os.Remove("./fifa2026.db-shm")
	_ = os.Remove("./fifa2026.db-wal")

	err := db.Init("./")
	if err != nil {
		t.Fatalf("初始化测试数据库失败: %v", err)
	}
	defer func() {
		db.Close()
		_ = os.Remove("./fifa2026.db")
		_ = os.Remove("./fifa2026.db-shm")
		_ = os.Remove("./fifa2026.db-wal")
		_ = os.Remove("./test_sync.db")
	}()

	// 插入一个测试赛季和比赛
	_ = db.SaveTournament(models.Tournament{ID: "fifa_2026", Name: "World Cup 2026", Year: 2026, Status: "PENDING"})
	
	m := models.Match{
		ID:           "test_m1",
		TournamentID: "fifa_2026",
		HomeTeam:     "TestHome",
		AwayTeam:     "TestAway",
		Group:        "A",
		Status:       "NS",
		ScheduledAt:  time.Now().Add(-10 * time.Minute),
		Venue:        "Estadio Azteca",
	}
	_ = db.SaveMatch(m)

	elo, _ := NewEloService("../../../../data/seasons/history_features.json")
	dc := NewDixonColesService(elo, nil)
	ollama := ai.NewOllamaService("", "")
	backtest := NewBacktestService(elo, ollama, dc)

	syncService := NewLiveSyncService(dc, backtest, ollama)
	
	// 执行一次同步
	syncService.SyncMatches()

	// 此时比赛应当变为了 Live
	updated, _ := db.GetMatch("test_m1")
	if updated.Status != "Live" {
		t.Errorf("预期比赛状态为 'Live'，实际为 '%s'", updated.Status)
	}

	// 将时间改到完赛（2小时前）
	m.ScheduledAt = time.Now().Add(-2 * time.Hour)
	_ = db.SaveMatch(m)

	syncService.SyncMatches()

	updated2, _ := db.GetMatch("test_m1")
	if updated2.Status != "FT" {
		t.Errorf("预期比赛状态为 'FT'，实际为 '%s'", updated2.Status)
	}
}
