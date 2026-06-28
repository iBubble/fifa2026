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
	client        *http.Client
	cache         []WC26Game
	lastFetchTime time.Time
	mu            sync.RWMutex
}

// WC26Response worldcup26.ir API 的外层包装对象
type WC26Response struct {
	Games []WC26Game `json:"games"`
}

// WC26Game worldcup26.ir API 返回的单场比赛数据
type WC26Game struct {
	ID          string `json:"id"`
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
	s.mu.RLock()
	// 若缓存不为空且距离上一次拉取成功少于1分钟，直接读缓存，防止高频防抖API被封禁
	if len(s.cache) > 0 && time.Since(s.lastFetchTime) < 1*time.Minute {
		defer s.mu.RUnlock()
		return s.cache, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 双重锁检查
	if len(s.cache) > 0 && time.Since(s.lastFetchTime) < 1*time.Minute {
		return s.cache, nil
	}

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

	s.cache = wrapper.Games
	s.lastFetchTime = time.Now()

	return wrapper.Games, nil
}

// SyncFinishedMatches 将已完赛比赛的比分同步到本地 DB，并对淘汰赛进行队局自动更新
func (s *WorldCup26SyncService) SyncFinishedMatches() (int, error) {
	games, err := s.FetchGames()
	if err != nil {
		return 0, err
	}

	synced := 0
	for _, g := range games {
		homeTeam := normalizeWC26Team(g.HomeTeam)
		awayTeam := normalizeWC26Team(g.AwayTeam)

		// 判定是否是淘汰赛阶段的比赛
		isKnockout := g.Group == "R32" || g.Group == "R16" || g.Group == "QF" || g.Group == "SF" || g.Group == "3RD" || g.Group == "FINAL"

		if isKnockout {
			if g.ID == "" {
				continue
			}
			localID := "wc2026_m" + g.ID
			m, errGet := db.GetMatch(localID)
			if errGet == nil {
				// 只要 API 给出了有效的参赛球队，且本地还是 "0" 或者队名不同，则自动同步队名
				if homeTeam != "" && awayTeam != "" && homeTeam != "0" && awayTeam != "0" {
					if m.HomeTeam != homeTeam || m.AwayTeam != awayTeam {
						errUpdate := db.UpdateMatchTeams(m.ID, homeTeam, awayTeam)
						if errUpdate == nil {
							synced++
							log.Printf("[WC26Sync] 🏆 淘汰赛对阵更新: %s (id: %s) -> %s vs %s", m.ID, g.ID, homeTeam, awayTeam)
							m.HomeTeam = homeTeam
							m.AwayTeam = awayTeam
						}
					}
				}

				// 如果淘汰赛已完赛，同步状态和比分
				if strings.EqualFold(g.Finished, "TRUE") {
					if m.Status != "FT" || m.HomeScore != g.HomeScore || m.AwayScore != g.AwayScore {
						errScore := db.UpdateMatchScore(m.ID, g.HomeScore, g.AwayScore, "FT")
						if errScore == nil {
							synced++
							log.Printf("[WC26Sync] ✅ 淘汰赛完赛比分同步: %s %d-%d %s", homeTeam, g.HomeScore, g.AwayScore, awayTeam)
						}
					}
				}
			}
		} else {
			// 小组赛依旧保留原先安全匹配的逻辑
			if !strings.EqualFold(g.Finished, "TRUE") {
				continue
			}
			if homeTeam == "" || awayTeam == "" {
				continue
			}
			matches, errGet := db.GetMatchesByTeam("fifa_2026", homeTeam)
			if errGet != nil {
				continue
			}
			for _, m := range matches {
				if m.HomeTeam == homeTeam && m.AwayTeam == awayTeam && (m.Status != "FT" || m.HomeScore != g.HomeScore || m.AwayScore != g.AwayScore) {
					errScore := db.UpdateMatchScore(m.ID, g.HomeScore, g.AwayScore, "FT")
					if errScore == nil {
						synced++
						log.Printf("[WC26Sync] ✅ 小组赛同步: %s %d-%d %s", homeTeam, g.HomeScore, g.AwayScore, awayTeam)
					}
					break
				}
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
