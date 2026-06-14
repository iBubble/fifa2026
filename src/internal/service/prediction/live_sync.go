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

type RolledBackMatch struct {
	TargetHomeScore int
	TargetAwayScore int
	DetectedTime    time.Time
}

type LiveSyncService struct {
	dcService         *DixonColesService
	backtestService   *BacktestService
	ollamaService     *ai.OllamaService
	mu                sync.Mutex
	listeners         []chan string
	listenersMu       sync.Mutex
	rolledBackMatches map[string]RolledBackMatch
	rbMu              sync.Mutex
}

type RealtimeMatch struct {
	HomeScore int
	AwayScore int
	Status    string
}

func NewLiveSyncService(dc *DixonColesService, backtest *BacktestService, ollama *ai.OllamaService) *LiveSyncService {
	return &LiveSyncService{
		dcService:         dc,
		backtestService:   backtest,
		ollamaService:     ollama,
		listeners:         make([]chan string, 0),
		rolledBackMatches: make(map[string]RolledBackMatch),
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
		// 修正比分锁死 Bug：即使状态为 FT，如果开赛在 24 小时内，仍然允许请求外部源并更新/纠正比分
		if m.Status == "FT" && now.Sub(m.ScheduledAt) > 24*time.Hour {
			continue
		}

		key := NormalizeTeam(m.HomeTeam) + "_" + NormalizeTeam(m.AwayTeam)
		
		var finalMatch RealtimeMatch
		hasResult := false

		// 共识与优先级机制：
		// 1. CCTV 拥有最高优先级，若存在 CCTV 结果则直接采用
		// 2. 若 CCTV 缺失，则合并百度与 LiveScore 的比分及状态（比分取二者最大值，状态优先级 FT > Live > NS）
		if r, exists := cctvData[key]; exists {
			finalMatch = r
			hasResult = true
		} else {
			var subs []RealtimeMatch
			if r, exists := liveScoreData[key]; exists {
				subs = append(subs, r)
			}
			if r, exists := baiduData[key]; exists {
				subs = append(subs, r)
			}

			if len(subs) > 0 {
				maxHome := 0
				maxAway := 0
				finalStatus := "NS"
				for _, sub := range subs {
					if sub.HomeScore > maxHome {
						maxHome = sub.HomeScore
					}
					if sub.AwayScore > maxAway {
						maxAway = sub.AwayScore
					}
					if sub.Status == "FT" {
						finalStatus = "FT"
					} else if sub.Status == "Live" && finalStatus != "FT" {
						finalStatus = "Live"
					}
				}
				finalMatch = RealtimeMatch{
					HomeScore: maxHome,
					AwayScore: maxAway,
					Status:    finalStatus,
				}
				hasResult = true
			}
		}

		if hasResult {
			maxHome := finalMatch.HomeScore
			maxAway := finalMatch.AwayScore
			finalStatus := finalMatch.Status

			// 兜底状态修正：有比分产生时，状态绝不可能是未开赛 (NS)
			if finalStatus == "NS" && (maxHome > 0 || maxAway > 0) {
				finalStatus = "Live"
			}

			// 完赛时间兜底修正：如果已经开始且开赛超过 105 分钟，状态必须升级为已完赛 (FT)
			if finalStatus == "Live" && time.Now().After(m.ScheduledAt.Add(105*time.Minute)) {
				finalStatus = "FT"
			}

			// 若合并后的比分或状态发生变更，执行更新并广播
			// 判断是否发生比分倒流
			isRollback := maxHome < m.HomeScore || maxAway < m.AwayScore
			allowUpdate := true

			if isRollback {
				s.rbMu.Lock()
				rbKey := m.ID
				if rbKey == "" {
					rbKey = key
				}

				rb, exists := s.rolledBackMatches[rbKey]
				if !exists || rb.TargetHomeScore != maxHome || rb.TargetAwayScore != maxAway {
					// 第一次检测到该比分倒流，登记到待确认队列，本次拦截更新
					s.rolledBackMatches[rbKey] = RolledBackMatch{
						TargetHomeScore: maxHome,
						TargetAwayScore: maxAway,
						DetectedTime:    time.Now(),
					}
					allowUpdate = false
					log.Printf("[LiveSync] ⚠️ 检测到比赛 %s vs %s 比分疑似倒流 (从 %d:%d 到 %d:%d)，加入2分钟VAR防抖待确认队列", 
						m.HomeTeam, m.AwayTeam, m.HomeScore, m.AwayScore, maxHome, maxAway)
				} else {
					// 已经登记过，检查是否已持续超过 2 分钟
					if time.Since(rb.DetectedTime) >= 2*time.Minute {
						// 确认为 VAR 真实回滚，允许执行更新
						log.Printf("[LiveSync] 🚨 比赛 %s vs %s 比分持续倒流超2分钟，确认触发VAR进球无效回滚，允许数据落地", 
							m.HomeTeam, m.AwayTeam)
						delete(s.rolledBackMatches, rbKey)
					} else {
						// 未满 2 分钟，继续拦截更新
						allowUpdate = false
					}
				}
				s.rbMu.Unlock()
			} else {
				// 如果比分正常（未倒流），若当前存在倒流登记，则予以清除
				s.rbMu.Lock()
				rbKey := m.ID
				if rbKey == "" {
					rbKey = key
				}
				delete(s.rolledBackMatches, rbKey)
				s.rbMu.Unlock()
			}

			// 若合并后的比分或状态发生变更，且被允许更新时，执行更新并广播
			if allowUpdate && (m.HomeScore != maxHome || m.AwayScore != maxAway || m.Status != finalStatus) {
				m.HomeScore = maxHome
				m.AwayScore = maxAway
				m.Status = finalStatus
				_ = db.SaveMatch(m)
				s.broadcast("match_update")
				log.Printf("[LiveSync] ⚽ 优先级共识比分变更 %s vs %s: (%d:%d) [%s] (CCTV优先)", 
					m.HomeTeam, m.AwayTeam, m.HomeScore, m.AwayScore, m.Status)
			}
		} else {
			// 若全部数据源均拉取失败/无该比赛，则按照时间进行本地降级流转（比分保持旧值，防幻觉）
			if now.After(m.ScheduledAt) {
				elapsed := now.Sub(m.ScheduledAt)
				if elapsed < 105*time.Minute {
					if m.Status != "Live" {
						m.Status = "Live"
						_ = db.SaveMatch(m)
						s.broadcast("match_update")
						log.Printf("[LiveSync] ⚽ 比赛 %s vs %s 状态自动晋级为 Live", m.HomeTeam, m.AwayTeam)
					}
				} else {
					if m.Status != "FT" {
						m.Status = "FT"
						_ = db.SaveMatch(m)
						s.broadcast("match_update")
						log.Printf("[LiveSync] ⚽ 比赛 %s vs %s 已到时间且无外部比分，状态转为 FT", m.HomeTeam, m.AwayTeam)
					}
				}
			}
		}
	}
}

// Global team normalized dictionary mapping various names to standard db team names (48 nations in 2026 World Cup)
var teamDictionary = map[string]string{
	// Group A
	"墨西哥": "Mexico", "Mexico": "Mexico",
	"南非": "South Africa", "South Africa": "South Africa",
	"韩国": "South Korea", "South Korea": "South Korea",
	"捷克": "Czech Republic", "Czech Republic": "Czech Republic",
	
	// Group B
	"加拿大": "Canada", "Canada": "Canada",
	"波黑": "Bosnia and Herzegovina", "Bosnia and Herzegovina": "Bosnia and Herzegovina",
	"卡塔尔": "Qatar", "Qatar": "Qatar",
	"瑞士": "Switzerland", "Switzerland": "Switzerland",

	// Group C
	"巴西": "Brazil", "Brazil": "Brazil",
	"摩洛哥": "Morocco", "Morocco": "Morocco",
	"海地": "Haiti", "Haiti": "Haiti",
	"苏格兰": "Scotland", "Scotland": "Scotland",

	// Group D
	"美国": "United States", "USA": "United States", "United States": "United States",
	"巴拉圭": "Paraguay", "Paraguay": "Paraguay",
	"澳大利亚": "Australia", "Australia": "Australia",
	"土耳其": "Turkey", "Turkey": "Turkey",

	// Group E
	"德国": "Germany", "Germany": "Germany",
	"库拉索": "Curaçao", "Curacao": "Curaçao", "Curaçao": "Curaçao",
	"科特迪瓦": "Ivory Coast", "Ivory Coast": "Ivory Coast",
	"厄瓜多尔": "Ecuador", "Ecuador": "Ecuador",

	// Group F
	"荷兰": "Netherlands", "Netherlands": "Netherlands",
	"日本": "Japan", "Japan": "Japan",
	"瑞典": "Sweden", "Sweden": "Sweden",
	"突尼斯": "Tunisia", "Tunisia": "Tunisia",

	// Group G
	"比利时": "Belgium", "Belgium": "Belgium",
	"埃及": "Egypt", "Egypt": "Egypt",
	"伊朗": "Iran", "Iran": "Iran",
	"新西兰": "New Zealand", "New Zealand": "New Zealand",

	// Group H
	"西班牙": "Spain", "Spain": "Spain",
	"佛得角": "Cape Verde", "Cape Verde": "Cape Verde",
	"沙特阿拉伯": "Saudi Arabia", "Saudi Arabia": "Saudi Arabia", "沙特": "Saudi Arabia",
	"乌拉圭": "Uruguay", "Uruguay": "Uruguay",

	// Group I
	"法国": "France", "France": "France",
	"塞内加尔": "Senegal", "Senegal": "Senegal",
	"伊拉克": "Iraq", "Iraq": "Iraq",
	"挪威": "Norway", "Norway": "Norway",

	// Group J
	"阿根廷": "Argentina", "Argentina": "Argentina",
	"阿尔及利亚": "Algeria", "Algeria": "Algeria",
	"奥地利": "Austria", "Austria": "Austria",
	"约旦": "Jordan", "Jordan": "Jordan",

	// Group K
	"葡萄牙": "Portugal", "Portugal": "Portugal",
	"民主刚果": "Democratic Republic of the Congo", "DR Congo": "Democratic Republic of the Congo", "Democratic Republic of the Congo": "Democratic Republic of the Congo", "刚果民主共和国": "Democratic Republic of the Congo", "刚果（金）": "Democratic Republic of the Congo",
	"乌兹别克斯坦": "Uzbekistan", "Uzbekistan": "Uzbekistan",
	"哥伦比亚": "Colombia", "Colombia": "Colombia",

	// Group L
	"英格兰": "England", "England": "England",
	"克罗地亚": "Croatia", "Croatia": "Croatia",
	"加纳": "Ghana", "Ghana": "Ghana",
	"巴拿马": "Panama", "Panama": "Panama",
}

// NormalizeTeam 将抓取到的各种简称、中英文别名映射归一化为数据库标准全称
func NormalizeTeam(name string) string {
	name = strings.TrimSpace(name)
	if std, ok := teamDictionary[name]; ok {
		return std
	}
	return name
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

	for _, tab := range resData.Data.TabsList {
		for _, day := range tab.All.Data {
			for _, item := range day.List {
				homeEn := NormalizeTeam(item.LeftLogo.Name)
				awayEn := NormalizeTeam(item.RightLogo.Name)
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
	dates := []string{
		now.AddDate(0, 0, -1).Format("20060102"),
		now.Format("20060102"),
		now.AddDate(0, 0, 1).Format("20060102"),
	}

	client := &http.Client{Timeout: 4 * time.Second}

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
					homeNm := NormalizeTeam(event.T1[0].Nm)
					awayNm := NormalizeTeam(event.T2[0].Nm)

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

	for _, m := range data.Data.List {
		homeEn := NormalizeTeam(m.HomeName)
		awayEn := NormalizeTeam(m.AwayName)
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


