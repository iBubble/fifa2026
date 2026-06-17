package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// WorldCup26SyncService 从 worldcup26.ir 同步实时比分
type WorldCup26SyncService struct {
	client *http.Client
	cache  []WC26Game
	mu     sync.RWMutex
}

// WC26Response worldcup26.ir API 的外层包装对象
type WC26Response struct {
	Games []WC26Game `json:"games"`
}

// WC26Game worldcup26.ir API 返回的单场比赛数据
type WC26Game struct {
	HomeTeam    string `json:"home_team_name_en"`
	AwayTeam    string `json:"away_team_name_en"`
	HomeScore   int    `json:"home_score,string"`
	AwayScore   int    `json:"away_score,string"`
	Finished    string `json:"finished"`    // "TRUE" / "FALSE"
	HomeScorers string `json:"home_scorers"`
	AwayScorers string `json:"away_scorers"`
	Group       string `json:"group"`
}

func NewWorldCup26SyncService() *WorldCup26SyncService {
	return &WorldCup26SyncService{
		client: &http.Client{Timeout: 35 * time.Second},
	}
}

// FetchGames 从 worldcup26.ir 获取所有比赛数据
func (s *WorldCup26SyncService) FetchGames() ([]WC26Game, error) {
	req, err := http.NewRequest("GET", "https://worldcup26.ir/get/games", nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Close = true
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worldcup26.ir 请求失败: %w", err)
	}
	defer resp.Body.Close()

	var wrapper WC26Response
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, fmt.Errorf("worldcup26.ir JSON 解析失败: %w", err)
	}

	s.mu.Lock()
	s.cache = wrapper.Games
	s.mu.Unlock()

	return wrapper.Games, nil
}

// SyncFinishedMatches 将已完赛比赛的比分同步到本地 DB
// 严格保护赛程：仅更新比分和状态，绝不修改时间/主客队/场馆
func (s *WorldCup26SyncService) SyncFinishedMatches() (int, error) {
	games, err := s.FetchGames()
	if err != nil {
		return 0, err
	}

	synced := 0
	for _, g := range games {
		if !strings.EqualFold(g.Finished, "TRUE") {
			continue
		}

		// 规范化队名映射
		homeTeam := normalizeWC26Team(g.HomeTeam)
		awayTeam := normalizeWC26Team(g.AwayTeam)

		if homeTeam == "" || awayTeam == "" {
			continue
		}

		// 查找本地对应比赛并更新比分
		matches, err := db.GetMatchesByTeam("fifa_2026", homeTeam)
		if err != nil {
			continue
		}

		for _, m := range matches {
			if m.HomeTeam == homeTeam && m.AwayTeam == awayTeam && m.Status != "FT" {
				// 仅更新比分和状态
				err := db.UpdateMatchScore(m.ID, g.HomeScore, g.AwayScore, "FT")
				if err == nil {
					synced++
					log.Printf("[WC26Sync] ✅ 同步: %s %d-%d %s",
						homeTeam, g.HomeScore, g.AwayScore, awayTeam)
				}
				break
			}
		}
	}

	return synced, nil
}

// normalizeWC26Team 将 worldcup26.ir 的队名映射到我们系统的标准英文名
func normalizeWC26Team(name string) string {
	nameMap := map[string]string{
		"USA":              "United States",
		"Korea Republic":   "South Korea",
		"IR Iran":          "Iran",
		"Türkiye":          "Turkey",
		"Czechia":          "Czech Republic",
		"DR Congo":         "Democratic Republic of the Congo",
		"Côte d'Ivoire":    "Ivory Coast",
		"Bosnia & Herzegovina": "Bosnia and Herzegovina",
		"Cabo Verde":       "Cape Verde",
	}
	if mapped, ok := nameMap[name]; ok {
		return mapped
	}
	return name
}
