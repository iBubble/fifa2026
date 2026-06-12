package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/ai"
	"fmt"
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
	listeners       []chan string
	listenersMu     sync.Mutex
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
		listeners:       make([]chan string, 0),
	}
}

func (s *LiveSyncService) RegisterListener() chan string {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	ch := make(chan string, 10)
	s.listeners = append(s.listeners, ch)
	return ch
}

func (s *LiveSyncService) RemoveListener(ch chan string) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	for i, l := range s.listeners {
		if l == ch {
			s.listeners = append(s.listeners[:i], s.listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

func (s *LiveSyncService) broadcast(msg string) {
	s.listenersMu.Lock()
	defer s.listenersMu.Unlock()
	for _, ch := range s.listeners {
		select {
		case ch <- msg:
		default:
		}
	}
}

// StartSyncLoop 开启常驻后台轮询协程 (依据是否有 Live 比赛动态调节周期: 60秒 / 10分钟，冷启动为空时 5秒)
func (s *LiveSyncService) StartSyncLoop() {
	go func() {
		for {
			s.SyncMatches()

			hasLive := false
			matches, err := db.GetMatchesByTournament("fifa_2026")
			
			var delay time.Duration
			if err != nil || len(matches) == 0 {
				delay = 5 * time.Second
			} else {
				for _, m := range matches {
					if m.Status == "Live" {
						hasLive = true
						break
					}
				}
				if hasLive {
					delay = 60 * time.Second
				} else {
					delay = 10 * time.Minute
				}
			}
			log.Printf("[LiveSync] ⏳ 下一次比分同步休眠延时: %v (hasLive: %t, matches: %d)", delay, hasLive, len(matches))
			time.Sleep(delay)
		}
	}()
}

// SyncMatches 依据百度、LiveScore 和 CCTV 三源并发拉取，并采用共识算法合并状态和比分
func (s *LiveSyncService) SyncMatches() {
	s.mu.Lock()
	defer s.mu.Unlock()

	matches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		return
	}

	// 并发拉取三大源数据
	var baiduData, liveScoreData, cctvData map[string]RealtimeMatch
	var wg sync.WaitGroup
	wg.Add(3)
	log.Println("[LiveSync] ⏳ 开始并发拉取三大数据源...")
	go func() {
		defer wg.Done()
		baiduData = fetchBaiduMatchResults()
		log.Println("[LiveSync] ✅ 百度数据拉取完成")
	}()
	go func() {
		defer wg.Done()
		liveScoreData = fetchLiveScoreMatchResults()
		log.Println("[LiveSync] ✅ LiveScore 数据拉取完成")
	}()
	go func() {
		defer wg.Done()
		cctvData = fetchCCTVMatchResults()
		log.Println("[LiveSync] ✅ CCTV 数据拉取完成")
	}()
	wg.Wait()
	log.Println("[LiveSync] ✅ 三大数据源全部并发拉取完成")

	now := time.Now()
	for _, m := range matches {
		if m.Status == "FT" {
			continue
		}

		key := m.HomeTeam + "_" + m.AwayTeam
		
		// 收集各个源中当前比赛的数据
		var candidates []RealtimeMatch
		if r, exists := baiduData[key]; exists {
			candidates = append(candidates, r)
		}
		if r, exists := liveScoreData[key]; exists {
			candidates = append(candidates, r)
		}
		if r, exists := cctvData[key]; exists {
			candidates = append(candidates, r)
		}

		if len(candidates) > 0 {
			// 共识机制：
			// 1. 比分取所有源中最大值（防止数据更新滞后漏掉进球）
			// 2. 状态优先级：FT > Live > NS
			maxHome := 0
			maxAway := 0
			finalStatus := "NS"
			
			for _, cand := range candidates {
				if cand.HomeScore > maxHome {
					maxHome = cand.HomeScore
				}
				if cand.AwayScore > maxAway {
					maxAway = cand.AwayScore
				}
				if cand.Status == "FT" {
					finalStatus = "FT"
				} else if cand.Status == "Live" && finalStatus != "FT" {
					finalStatus = "Live"
				}
			}

			// 兜底状态修正：有比分产生时，状态绝不可能是未开赛 (NS)
			if finalStatus == "NS" && (maxHome > 0 || maxAway > 0) {
				finalStatus = "Live"
			}

			// 完赛时间兜底修正：如果已经开始且开赛超过 105 分钟，状态必须升级为已完赛 (FT)
			if finalStatus == "Live" && time.Now().After(m.ScheduledAt.Add(105*time.Minute)) {
				finalStatus = "FT"
			}

			// 若合并后的比分或状态发生变更，执行更新并广播
			if m.HomeScore != maxHome || m.AwayScore != maxAway || m.Status != finalStatus {
				m.HomeScore = maxHome
				m.AwayScore = maxAway
				m.Status = finalStatus
				_ = db.SaveMatch(m)
				s.broadcast("match_update")
				log.Printf("[LiveSync] ⚽ 多源共识比分变更 %s vs %s: (%d:%d) [%s] (合并了 %d 个源数据)", 
					m.HomeTeam, m.AwayTeam, m.HomeScore, m.AwayScore, m.Status, len(candidates))
			}
		} else {
			// 若全部数据源均拉取失败/无该比赛，则按照时间进行本地降级流转（比分保持 0:0，防幻觉）
			if now.After(m.ScheduledAt) {
				elapsed := now.Sub(m.ScheduledAt)
				if elapsed < 105*time.Minute {
					if m.Status != "Live" {
						m.Status = "Live"
						m.HomeScore = 0
						m.AwayScore = 0
						_ = db.SaveMatch(m)
						s.broadcast("match_update")
						log.Printf("[LiveSync] ⚽ 比赛 %s vs %s 状态自动晋级为 Live (0:0)", m.HomeTeam, m.AwayTeam)
					}
				} else {
					if m.Status != "FT" {
						m.Status = "FT"
						_ = db.SaveMatch(m)
						s.broadcast("match_update")
						log.Printf("[LiveSync] ⚽ 比赛 %s vs %s 已到时间且无外部比分，状态转为 FT (0:0)", m.HomeTeam, m.AwayTeam)
					}
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
				statusText := strings.TrimSpace(item.MatchStatusText)
				if statusText == "已完赛" || statusText == "已结束" || statusText == "完赛" || statusText == "结束" || statusText == "FT" {
					status = "FT"
				} else if statusText == "未开赛" || statusText == "VS" || statusText == "" {
					status = "NS"
				} else {
					status = "Live"
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

// fetchLiveScoreMatchResults 从 LiveScore 官方 APP CDN 接口并发获取昨日、今日、明日比分
func fetchLiveScoreMatchResults() map[string]RealtimeMatch {
	results := make(map[string]RealtimeMatch)
	now := time.Now()
	// 查询昨日、今日、明日的数据，以应对时区和比赛日的重合性
	dates := []string{
		now.AddDate(0, 0, -1).Format("20060102"),
		now.Format("20060102"),
		now.AddDate(0, 0, 1).Format("20060102"),
	}

	client := &http.Client{Timeout: 4 * time.Second}
	teamEnMap := map[string]string{
		"United States": "USA",
		"USA":           "USA",
		"South Korea":    "South Korea",
		"South Africa":   "South Africa",
		"Czech Republic": "Czech Republic",
	}

	type LiveScoreResponse struct {
		Stages []struct {
			Events []struct {
				Eps string `json:"Eps"` // "FT", "NS", "HT", "AP", "AET" 或 分钟数如 "75"
				Tr1 string `json:"Tr1"` // 主队比分
				Tr2 string `json:"Tr2"` // 客队比分
				T1  []struct {
					Nm string `json:"Nm"` // 主队英文名
				} `json:"T1"`
				T2 []struct {
					Nm string `json:"Nm"` // 客队英文名
				} `json:"T2"`
			} `json:"Events"`
		} `json:"Stages"`
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	for _, date := range dates {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			url := fmt.Sprintf("https://prod-cdn-public-api.livescore.com/v1/api/app/date/soccer/%s/8", d)
			req, err := http.NewRequest("GET", url, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return
			}

			var res LiveScoreResponse
			if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
				return
			}

			mu.Lock()
			defer mu.Unlock()

			for _, stage := range res.Stages {
				for _, event := range stage.Events {
					if len(event.T1) == 0 || len(event.T2) == 0 {
						continue
					}
					homeNm := event.T1[0].Nm
					awayNm := event.T2[0].Nm

					if mapHome, ok := teamEnMap[homeNm]; ok {
						homeNm = mapHome
					}
					if mapAway, ok := teamEnMap[awayNm]; ok {
						awayNm = mapAway
					}

					hScore := 0
					aScore := 0
					if event.Tr1 != "" {
						hScore, _ = strconv.Atoi(event.Tr1)
					}
					if event.Tr2 != "" {
						aScore, _ = strconv.Atoi(event.Tr2)
					}

					status := "NS"
					eps := event.Eps
					if eps == "FT" || eps == "AP" || eps == "AET" {
						status = "FT"
					} else if eps != "NS" && eps != "" {
						status = "Live"
					}

					key := homeNm + "_" + awayNm
					results[key] = RealtimeMatch{
						HomeScore: hScore,
						AwayScore: aScore,
						Status:    status,
					}
				}
			}
		}(date)
	}

	wg.Wait()
	return results
}

// fetchCCTVMatchResults 尝试抓取 CCTV 世界杯比分数据
func fetchCCTVMatchResults() map[string]RealtimeMatch {
	results := make(map[string]RealtimeMatch)
	now := time.Now()
	startTime := now.Format("2006-01-02") + "%2000:00:00"
	endTime := now.Format("2006-01-02") + "%2023:59:59"
	url := fmt.Sprintf("https://cbs.sports.cctv.com/game/date_game_list?startTime=%s&endTime=%s&leagueId=3400&client=pc", startTime, endTime)

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return results
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Referer", "https://cbs.sports.cctv.com/worldcup2026_schedule_tabs.html")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[LiveSync] CCTV 同步网络连接失败: %v", err)
		return results
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[LiveSync] ⚠️ CCTV 比分采集受安全策略拦截或错误 (HTTP %d)，优雅跳过 CCTV 同步", resp.StatusCode)
		return results
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
	bodyStr := string(bodyBytes)

	if strings.Contains(bodyStr, "HOY_TR") || strings.Contains(bodyStr, "HBB_HC") {
		log.Printf("[LiveSync] ⚠️ CCTV 比分采集受云盾安全策略挑战拦截，优雅跳过 CCTV 同步")
		return results
	}

	var data struct {
		Data struct {
			List []struct {
				HomeName    string `json:"homeName"`
				AwayName    string `json:"awayName"`
				HomeScore   string `json:"homeScore"`
				AwayScore   string `json:"awayScore"`
				MatchStatus string `json:"matchStatus"`
			} `json:"list"`
		} `json:"data"`
	}

	if err := json.Unmarshal(bodyBytes, &data); err != nil {
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

	for _, m := range data.Data.List {
		homeEn := teamCnToEn[m.HomeName]
		awayEn := teamCnToEn[m.AwayName]
		if homeEn == "" || awayEn == "" {
			continue
		}

		hScore, _ := strconv.Atoi(m.HomeScore)
		aScore, _ := strconv.Atoi(m.AwayScore)

		status := "NS"
		if m.MatchStatus == "1" {
			status = "Live"
		} else if m.MatchStatus == "2" {
			status = "FT"
		}

		key := homeEn + "_" + awayEn
		results[key] = RealtimeMatch{
			HomeScore: hScore,
			AwayScore: aScore,
			Status:    status,
		}
	}

	return results
}

