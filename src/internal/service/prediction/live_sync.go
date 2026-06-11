package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/ai"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type LiveSyncService struct {
	dcService       *DixonColesService
	backtestService *BacktestService
	ollamaService   *ai.OllamaService
	mu              sync.Mutex
}

type RealtimeMatch struct {
	HomeScore int
	AwayScore int
	Status    string
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

// SyncMatches 根据百度体育或比赛时间自动更新进行中 Live 比分或 FT 完赛结果
func (s *LiveSyncService) SyncMatches() {
	s.mu.Lock()
	defer s.mu.Unlock()

	matches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		return
	}

	// 从百度体育拉取最新的真实比分与状态
	realtimeData := fetchBaiduMatchResults()

	now := time.Now()
	for _, m := range matches {
		if m.Status == "FT" {
			continue
		}

		key := m.HomeTeam + "_" + m.AwayTeam
		real, exists := realtimeData[key]

		if exists {
			// 若百度体育中存在该比赛，则以真实的实时比分和状态进行覆盖更新
			m.HomeScore = real.HomeScore
			m.AwayScore = real.AwayScore
			m.Status = real.Status
			_ = db.SaveMatch(m)
			if real.Status == "FT" {
				log.Printf("[LiveSync] ⚽ 比赛 %s vs %s 已完赛，真实赛果为 (%d:%d)", m.HomeTeam, m.AwayTeam, m.HomeScore, m.AwayScore)
			}
		} else {
			// 若拉取失败，则按照时间进行本地的状态降级流转（比分保持 0:0，防幻觉）
			if now.After(m.ScheduledAt) {
				elapsed := now.Sub(m.ScheduledAt)
				if elapsed < 105*time.Minute {
					m.Status = "Live"
					m.HomeScore = 0
					m.AwayScore = 0
					_ = db.SaveMatch(m)
				} else {
					m.Status = "FT"
					_ = db.SaveMatch(m)
					log.Printf("[LiveSync] ⚽ 比赛 %s vs %s 已到时间且无外部比分，转为 FT 状态（0:0）", m.HomeTeam, m.AwayTeam)
				}
			}
		}
	}
}

// fetchBaiduMatchResults 从百度体育世界杯赛程页面异步拉取明文渲染 JSON，解析实时比分和状态
func fetchBaiduMatchResults() map[string]RealtimeMatch {
	results := make(map[string]RealtimeMatch)
	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("GET", "https://tiyu.baidu.com/al/match?match=%E4%B8%96%E7%95%8C%E6%9D%AF&tab=%E8%B5%9B%E7%A8%8B", nil)
	if err != nil {
		return results
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return results
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return results
	}

	// 将 HTML 限制在 2MB 以内防止内存撑爆
	var buf strings.Builder
	_, _ = io.Copy(&buf, io.LimitReader(resp.Body, 2*1024*1024))
	html := buf.String()

	// 截取 s-data JSON
	startIdx := strings.Index(html, "<!--s-data:")
	if startIdx == -1 {
		return results
	}
	startIdx += len("<!--s-data:")

	endIdx := strings.Index(html[startIdx:], "-->")
	if endIdx == -1 {
		return results
	}
	endIdx += startIdx

	jsonStr := html[startIdx:endIdx]

	var resData struct {
		Data struct {
			TabsList []struct {
				All struct {
					Data []struct {
						List []struct {
							MatchStatusText string `json:"matchStatusText"`
							LeftLogo        struct {
								Name  string `json:"name"`
								Score string `json:"score"`
							} `json:"leftLogo"`
							RightLogo struct {
								Name  string `json:"name"`
								Score string `json:"score"`
							} `json:"rightLogo"`
						} `json:"list"`
					} `json:"data"`
				} `json:"all"`
			} `json:"tabsList"`
		} `json:"data"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &resData); err != nil {
		return results
	}

	teamCnToEn := map[string]string{
		"墨西哥":   "Mexico",
		"南非":     "South Africa",
		"韩国":     "South Korea",
		"捷克":     "Czech Republic",
		"加拿大":   "Canada",
		"波黑":     "Bosnia and Herzegovina",
		"美国":     "USA",
		"巴拉圭":   "Paraguay",
		"卡塔尔":   "Qatar",
		"瑞士":     "Switzerland",
		"巴西":     "Brazil",
		"摩洛哥":   "Morocco",
		"海地":     "Haiti",
		"苏格兰":   "Scotland",
		"澳大利亚": "Australia",
		"土耳其":   "Turkey",
	}

	for _, tab := range resData.Data.TabsList {
		for _, day := range tab.All.Data {
			for _, item := range day.List {
				homeEn := teamCnToEn[item.LeftLogo.Name]
				awayEn := teamCnToEn[item.RightLogo.Name]
				if homeEn == "" || awayEn == "" {
					continue
				}

				hScore, _ := strconv.Atoi(item.LeftLogo.Score)
				aScore, _ := strconv.Atoi(item.RightLogo.Score)

				var status string
				switch item.MatchStatusText {
				case "进行中":
					status = "Live"
				case "已完赛":
					status = "FT"
				default:
					status = "NS"
				}

				key := homeEn + "_" + awayEn
				results[key] = RealtimeMatch{
					HomeScore: hScore,
					AwayScore: aScore,
					Status:    status,
				}
			}
		}
	}

	return results
}
