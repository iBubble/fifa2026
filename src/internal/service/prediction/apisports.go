package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

var staticTeamIDs = map[string]int{
	"Brazil":         6,
	"Morocco":        31,
	"Canada":         5529,
	"USA":            2384,
	"Mexico":         16,
	"South Korea":    17,
	"Czech Republic": 770,
	"South Africa":   1531,
	"Paraguay":       2380,
}

var h2hOverrides = map[string]models.H2HRecord{
	"Brazil:Morocco": {
		TotalMatches: 3,
		HomeWins:     2,
		Draws:        0,
		AwayWins:     1,
		AvgHomeGoals: 2.0,
		AvgAwayGoals: 0.6666666666666666,
	},
	"Qatar:Switzerland": {
		TotalMatches: 1,
		HomeWins:     1,
		Draws:        0,
		AwayWins:     0,
		AvgHomeGoals: 1.0,
		AvgAwayGoals: 0.0,
	},
}

type APISportsService struct {
	apiKeys      []string
	activeIdx    int32
	client       *http.Client
	apiCallCount int32
}

func NewAPISportsService(apiKeyEnv string) *APISportsService {
	var keys []string
	for _, k := range strings.Split(apiKeyEnv, ",") {
		trimmed := strings.TrimSpace(k)
		if trimmed != "" {
			keys = append(keys, trimmed)
		}
	}
	if len(keys) == 0 {
		keys = []string{"7eea26f9d015bc60899c2c322937b237"}
	}
	return &APISportsService{
		apiKeys: keys,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *APISportsService) getAPIKey() string {
	idx := atomic.LoadInt32(&s.activeIdx)
	if int(idx) >= len(s.apiKeys) {
		return s.apiKeys[0]
	}
	return s.apiKeys[idx]
}

func (s *APISportsService) rotateKey() {
	curr := atomic.LoadInt32(&s.activeIdx)
	next := (curr + 1) % int32(len(s.apiKeys))
	atomic.StoreInt32(&s.activeIdx, next)
}


// GetTeamID 获取球队映射 ID，优先读 SQLite 本地缓存，没有才拉取外部 API
func (s *APISportsService) GetTeamID(teamName string) (int, error) {
	name := strings.TrimSpace(teamName)
	if name == "" {
		return 0, fmt.Errorf("球队英文名不能为空")
	}

	// 0. 优先匹配静态球队 ID 映射，免除网络和 DB 查询
	if id, ok := staticTeamIDs[name]; ok {
		return id, nil
	}

	// 1. 读本地缓存
	cachedID, err := db.GetTeamApiMapping(name)
	if err == nil && cachedID > 0 {
		return cachedID, nil
	}

	// 2. 缓存未命中，调用 api-football 接口查询 (带自动多 Key 轮替重试)
	if atomic.LoadInt32(&s.apiCallCount) >= 90 {
		return 0, fmt.Errorf("每日 API 调用已达防爆熔断限额(90次)")
	}
	atomic.AddInt32(&s.apiCallCount, 1)

	var lastErr error
	for attempt := 0; attempt < len(s.apiKeys); attempt++ {
		apiKey := s.getAPIKey()
		apiURL := fmt.Sprintf("https://v3.football.api-sports.io/teams?name=%s", url.QueryEscape(name))
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return 0, err
		}
		req.Header.Set("x-apisports-key", apiKey)

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			s.rotateKey()
			continue
		}

		bodyBytes, errRead := io.ReadAll(resp.Body)
		resp.Body.Close()
		if errRead != nil {
			lastErr = errRead
			continue
		}

		bodyStr := string(bodyBytes)
		if strings.Contains(bodyStr, "reached the request limit") || resp.StatusCode == 429 {
			s.rotateKey()
			lastErr = fmt.Errorf("API 额度已耗尽 (已自动切换密钥)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("api-sports 状态码异常: %d", resp.StatusCode)
			s.rotateKey()
			continue
		}

		var result struct {
			Response []struct {
				Team struct {
					ID int `json:"id"`
				} `json:"team"`
			} `json:"response"`
		}

		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			lastErr = fmt.Errorf("解析 api-sports 球队响应失败: %w", err)
			continue
		}

		if len(result.Response) == 0 {
			return 0, fmt.Errorf("api-sports 未找到该球队: %s", name)
		}

		teamID := result.Response[0].Team.ID
		_ = db.SaveTeamApiMapping(name, teamID)
		return teamID, nil
	}

	return 0, fmt.Errorf("拉取球队ID失败，已尝试所有 Key: %w", lastErr)
}

// GetH2HRecord 获取两队的历史交手统计，优先读 SQLite 本地缓存以保护每日 100 次免费额度
func (s *APISportsService) GetH2HRecord(team1, team2 string) (models.H2HRecord, error) {
	t1 := strings.TrimSpace(team1)
	t2 := strings.TrimSpace(team2)

	// 0. 优先匹配静态 H2H 覆盖表，免除网络和 DB 查询
	teamKey, teamA, _ := db.GetSortedTeamKey(t1, t2)
	if overrideRecord, ok := h2hOverrides[teamKey]; ok {
		if t1 == teamA {
			return overrideRecord, nil
		} else {
			return models.H2HRecord{
				TotalMatches: overrideRecord.TotalMatches,
				HomeWins:     overrideRecord.AwayWins,
				Draws:        overrideRecord.Draws,
				AwayWins:     overrideRecord.HomeWins,
				AvgHomeGoals: overrideRecord.AvgAwayGoals,
				AvgAwayGoals: overrideRecord.AvgHomeGoals,
			}, nil
		}
	}

	// 1. 读本地缓存
	total, hWins, draws, aWins, avgH, avgA, found, err := db.GetH2HRecord(t1, t2)
	if err == nil && found {
		// 查询数据库规范键值匹配，如果主客场相反，对调返还胜负和场均进球数据
		if t1 == teamA {
			return models.H2HRecord{
				TotalMatches: total,
				HomeWins:     hWins,
				Draws:        draws,
				AwayWins:     aWins,
				AvgHomeGoals: avgH,
				AvgAwayGoals: avgA,
			}, nil
		} else {
			return models.H2HRecord{
				TotalMatches: total,
				HomeWins:     aWins,
				Draws:        draws,
				AwayWins:     hWins,
				AvgHomeGoals: avgA,
				AvgAwayGoals: avgH,
			}, nil
		}
	}

	// 2. 缓存未命中，获取两队的 API 对应 ID
	id1, err := s.GetTeamID(t1)
	if err != nil {
		return models.H2HRecord{}, fmt.Errorf("获取主队 ID 失败: %w", err)
	}
	id2, err := s.GetTeamID(t2)
	if err != nil {
		return models.H2HRecord{}, fmt.Errorf("获取客队 ID 失败: %w", err)
	}

	// 3. 向 api-football 请求最近对决 (带自动多 Key 轮替重试)
	if atomic.LoadInt32(&s.apiCallCount) >= 90 {
		return models.H2HRecord{}, fmt.Errorf("每日 API 调用已达防爆熔断限额(90次)")
	}
	atomic.AddInt32(&s.apiCallCount, 1)

	var lastErr error
	for attempt := 0; attempt < len(s.apiKeys); attempt++ {
		apiKey := s.getAPIKey()
		apiURL := fmt.Sprintf("https://v3.football.api-sports.io/fixtures/headtohead?h2h=%d-%d", id1, id2)
		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return models.H2HRecord{}, err
		}
		req.Header.Set("x-apisports-key", apiKey)

		resp, err := s.client.Do(req)
		if err != nil {
			lastErr = err
			s.rotateKey()
			continue
		}

		bodyBytes, errRead := io.ReadAll(resp.Body)
		resp.Body.Close()
		if errRead != nil {
			lastErr = errRead
			continue
		}

		bodyStr := string(bodyBytes)
		if strings.Contains(bodyStr, "reached the request limit") || resp.StatusCode == 429 {
			s.rotateKey()
			lastErr = fmt.Errorf("API H2H 额度已耗尽 (已自动切换密钥)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("api-sports H2H 状态码异常: %d", resp.StatusCode)
			s.rotateKey()
			continue
		}

		var result struct {
			Response []struct {
				Fixture struct {
					Status struct {
						Short string `json:"short"`
					} `json:"status"`
				} `json:"fixture"`
				Teams struct {
					Home struct {
						ID     int  `json:"id"`
						Winner bool `json:"winner"`
					} `json:"home"`
					Away struct {
						ID     int  `json:"id"`
						Winner bool `json:"winner"`
					} `json:"away"`
				} `json:"teams"`
				Goals struct {
					Home interface{} `json:"home"`
					Away interface{} `json:"away"`
				} `json:"goals"`
			} `json:"response"`
		}

		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			lastErr = fmt.Errorf("解析 H2H 历史数据失败: %w", err)
			continue
		}

		var totalCount, t1Wins, t2Wins, drawsCount int
		var t1Goals, t2Goals float64

		for _, f := range result.Response {
			if f.Goals.Home == nil || f.Goals.Away == nil {
				continue
			}
			status := f.Fixture.Status.Short
			if status != "FT" && status != "AET" && status != "PEN" {
				continue
			}

			totalCount++
			gHome := parseFloat(f.Goals.Home)
			gAway := parseFloat(f.Goals.Away)

			if f.Teams.Home.ID == id1 {
				t1Goals += gHome
				t2Goals += gAway
				if f.Teams.Home.Winner {
					t1Wins++
				} else if f.Teams.Away.Winner {
					t2Wins++
				} else {
					drawsCount++
				}
			} else {
				t1Goals += gAway
				t2Goals += gHome
				if f.Teams.Away.Winner {
					t1Wins++
				} else if f.Teams.Home.Winner {
					t2Wins++
				} else {
					drawsCount++
				}
			}
		}

		var avgGoals1, avgGoals2 float64
		if totalCount > 0 {
			avgGoals1 = t1Goals / float64(totalCount)
			avgGoals2 = t2Goals / float64(totalCount)
		}

		if t1 == teamA {
			_ = db.SaveH2HRecord(t1, t2, totalCount, t1Wins, drawsCount, t2Wins, avgGoals1, avgGoals2)
		} else {
			_ = db.SaveH2HRecord(t2, t1, totalCount, t2Wins, drawsCount, t1Wins, avgGoals2, avgGoals1)
		}

		return models.H2HRecord{
			TotalMatches: totalCount,
			HomeWins:     t1Wins,
			Draws:        drawsCount,
			AwayWins:     t2Wins,
			AvgHomeGoals: avgGoals1,
			AvgAwayGoals: avgGoals2,
		}, nil
	}

	return models.H2HRecord{}, fmt.Errorf("拉取 H2H 数据失败，已尝试所有 Key: %w", lastErr)
}

func parseFloat(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}
