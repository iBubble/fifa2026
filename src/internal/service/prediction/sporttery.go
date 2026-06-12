package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type OfficialOdds struct {
	HomeOdds     float64            `json:"homeOdds"`
	DrawOdds     float64            `json:"drawOdds"`
	AwayOdds     float64            `json:"awayOdds"`
	GoalLine     int                `json:"goalLine"`
	HhadHomeOdds float64            `json:"hhadHomeOdds"`
	HhadDrawOdds float64            `json:"hhadDrawOdds"`
	HhadAwayOdds float64            `json:"hhadAwayOdds"`
	CrsOdds      map[string]float64 `json:"crsOdds"`
	TtgOdds      map[string]float64 `json:"ttgOdds"`
	HafuOdds     map[string]float64 `json:"hafuOdds"`
	IsAvailable  bool               `json:"isAvailable"`
	IsSimulation bool               `json:"isSimulation"`
}


type SportteryService struct {
	cachedOdds    map[string]OfficialOdds
	lastFetchTime time.Time
	mu            sync.Mutex
	isFetching    bool
}

func NewSportteryService() *SportteryService {
	return &SportteryService{
		cachedOdds: make(map[string]OfficialOdds),
	}
}

func isJCOpen(t time.Time) bool {
	hour := t.Hour()
	w := t.Weekday()
	if hour < 11 {
		return false
	}
	if w == time.Saturday || w == time.Sunday {
		return hour < 23
	}
	return hour < 22
}

// StartBackgroundRefresh 启动后台定时刷新（在开售时间段内每10分钟主动刷新一次）
func (s *SportteryService) StartBackgroundRefresh() {
	go func() {
		// 启动时如果处于开售时段，先强刷一次赔率，保障页面数据新鲜度
		if isJCOpen(time.Now()) {
			s.FetchAllOddsForced()
		}
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			if isJCOpen(time.Now()) {
				s.FetchAllOddsForced()
			}
		}
	}()
}

// FetchAllOddsForced 强行刷新赔率，绕过缓存间隔限制
func (s *SportteryService) FetchAllOddsForced() {
	s.mu.Lock()
	if s.isFetching {
		s.mu.Unlock()
		return
	}
	s.isFetching = true
	s.mu.Unlock()

	s.executeFetch()
}

// FetchAllOdds 拉取体彩官网全部实时赔率并存入内存缓存（开售期缓存10分钟，非开售期缓存30分钟）
func (s *SportteryService) FetchAllOdds() {
	s.mu.Lock()
	interval := 30 * time.Minute
	if isJCOpen(time.Now()) {
		interval = 10 * time.Minute
	}
	if s.isFetching || (time.Since(s.lastFetchTime) < interval && len(s.cachedOdds) > 0) {
		s.mu.Unlock()
		return
	}
	s.isFetching = true
	s.mu.Unlock()

	s.executeFetch()
}

func (s *SportteryService) executeFetch() {
	defer func() {
		s.mu.Lock()
		s.isFetching = false
		s.mu.Unlock()
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "https://webapi.sporttery.cn/gateway/jc/football/getMatchCalculatorV1.qry?poolCode=had,hhad,crs,ttg,hafu&channel=c", nil)
	if err != nil {
		return
	}
	req.Header.Set("Referer", "https://www.lottery.gov.cn/")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != 567 {
		return
	}

	var res struct {
		Success bool `json:"success"`
		Value   struct {
			MatchInfoList []struct {
				SubMatchList []struct {
					HomeTeamAbbName string `json:"homeTeamAbbName"`
					AwayTeamAbbName string `json:"awayTeamAbbName"`
					MatchId         int    `json:"matchId"`
					MatchDate       string `json:"matchDate"`
					MatchTime       string `json:"matchTime"`
					LeagueAllName   string `json:"leagueAllName"`
					HomeTeamAbbEnName string `json:"homeTeamAbbEnName"`
					AwayTeamAbbEnName string `json:"awayTeamAbbEnName"`
					Had             struct {
						H string `json:"h"`
						D string `json:"d"`
						A string `json:"a"`
					} `json:"had"`
					Hhad struct {
						H        string `json:"h"`
						D        string `json:"d"`
						A        string `json:"a"`
						GoalLine string `json:"goalLine"`
					} `json:"hhad"`
					Crs  map[string]interface{} `json:"crs"`
					Ttg  map[string]interface{} `json:"ttg"`
					Hafu map[string]interface{} `json:"hafu"`
				} `json:"subMatchList"`
			} `json:"matchInfoList"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || !res.Success {
		return
	}

	s.mu.Lock()
	if s.cachedOdds == nil {
		s.cachedOdds = make(map[string]OfficialOdds)
	}
	for _, day := range res.Value.MatchInfoList {
		for _, m := range day.SubMatchList {
			key := m.HomeTeamAbbName + "_" + m.AwayTeamAbbName
			h, _ := strconv.ParseFloat(m.Had.H, 64)
			d, _ := strconv.ParseFloat(m.Had.D, 64)
			a, _ := strconv.ParseFloat(m.Had.A, 64)
			hh, _ := strconv.ParseFloat(m.Hhad.H, 64)
			hd, _ := strconv.ParseFloat(m.Hhad.D, 64)
			ha, _ := strconv.ParseFloat(m.Hhad.A, 64)
			gl, _ := strconv.Atoi(m.Hhad.GoalLine)

			crsOdds := make(map[string]float64)
			for k, v := range m.Crs {
				if strVal, ok := v.(string); ok && strVal != "" {
					if val, err := strconv.ParseFloat(strVal, 64); err == nil {
						crsOdds[k] = val
					}
				}
			}

			ttgOdds := make(map[string]float64)
			for k, v := range m.Ttg {
				if strVal, ok := v.(string); ok && strVal != "" {
					if val, err := strconv.ParseFloat(strVal, 64); err == nil {
						ttgOdds[k] = val
					}
				}
			}

			hafuOdds := make(map[string]float64)
			for k, v := range m.Hafu {
				if strVal, ok := v.(string); ok && strVal != "" {
					if val, err := strconv.ParseFloat(strVal, 64); err == nil {
						hafuOdds[k] = val
					}
				}
			}

			s.cachedOdds[key] = OfficialOdds{
				HomeOdds:     h,
				DrawOdds:     d,
				AwayOdds:     a,
				GoalLine:     gl,
				HhadHomeOdds: hh,
				HhadDrawOdds: hd,
				HhadAwayOdds: ha,
				CrsOdds:      crsOdds,
				TtgOdds:      ttgOdds,
				HafuOdds:     hafuOdds,
				IsAvailable:  h > 0.0,
			}

			// 同步当前竞彩在售比赛到本地 SQLite matches 表中
			if m.MatchId > 0 {
				matchID := fmt.Sprintf("sporttery_%d", m.MatchId)
				scheduledStr := m.MatchDate + " " + m.MatchTime
				var scheduledAt time.Time
				loc, errLoc := time.LoadLocation("Asia/Shanghai")
				if errLoc == nil {
					scheduledAt, _ = time.ParseInLocation("2006-01-02 15:04:05", scheduledStr, loc)
				} else {
					scheduledAt, _ = time.Parse("2006-01-02 15:04:05", scheduledStr)
				}

				homeEn := getNormalizedTeamEnName(m.HomeTeamAbbName, m.HomeTeamAbbEnName)
				awayEn := getNormalizedTeamEnName(m.AwayTeamAbbName, m.AwayTeamAbbEnName)

				matchModel := models.Match{
					ID:           matchID,
					TournamentID: "fifa_2026",
					HomeTeam:     homeEn,
					AwayTeam:     awayEn,
					Group:        "Lottery",
					ScheduledAt:  scheduledAt,
					Status:       "NS",
					HomeScore:    0,
					AwayScore:    0,
					Venue:        m.LeagueAllName,
				}

				// 检查数据库中是否已存在该比赛
				if existing, errExist := db.GetMatch(matchID); errExist == nil {
					// 已存在比赛，继承已完赛的状态及比分，或者由 live_sync 服务已经推进的实时状态
					matchModel.Status = existing.Status
					matchModel.HomeScore = existing.HomeScore
					matchModel.AwayScore = existing.AwayScore
				}

				_ = db.SaveMatch(matchModel)
			}
		}
	}

	s.lastFetchTime = time.Now()
	s.mu.Unlock()
}

// GetMatchOdds 获取指定比赛的体彩官网最新赔率
func (s *SportteryService) GetMatchOdds(homeTeam, awayTeam string) OfficialOdds {
	s.FetchAllOdds() // 尝试更新缓存

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, odds := range s.cachedOdds {
		parts := strings.Split(key, "_")
		if len(parts) != 2 {
			continue
		}
		officialHome, officialAway := parts[0], parts[1]

		if matchTeam(homeTeam, officialHome) && matchTeam(awayTeam, officialAway) {
			return odds
		}
	}
	return OfficialOdds{IsAvailable: false}
}

// matchTeam 模糊队名匹配
func matchTeam(localEnName, officialCnName string) bool {
	dict := map[string]string{
		"Brazil": "巴西", "Argentina": "阿根廷", "France": "法国", "Germany": "德国",
		"Spain": "西班牙", "England": "英格兰", "Italy": "意大利", "Netherlands": "荷兰",
		"Portugal": "葡萄牙", "Croatia": "克罗地亚", "Japan": "日本", "USA": "美国",
		"Mexico": "墨西哥", "Ecuador": "厄瓜多尔", "South Africa": "南非",
		"Venezuela": "委内瑞拉", "Jamaica": "牙买加", "Iran": "伊朗", "Wales": "威尔士",
		"Saudi Arabia": "沙特阿拉伯", "Poland": "波兰", "Australia": "澳大利亚",
		"Denmark": "丹麦", "Tunisia": "突尼斯", "Costa Rica": "哥斯达黎加",
		"Belgium": "比利时", "Canada": "加拿大", "Morocco": "摩洛哥",
		"Serbia": "塞尔维亚", "Switzerland": "瑞士", "Cameroon": "喀麦隆",
		"Ghana": "加纳", "Uruguay": "乌拉圭", "South Korea": "韩国",
		"Colombia": "哥伦比亚", "Algeria": "阿尔及利亚", "Chile": "智利",
		"Nigeria": "尼日利亚", "Scotland": "苏格兰", "Hungary": "匈牙利",
		"Panama": "巴拿马", "Bolivia": "玻利维亚", "Peru": "秘鲁",
		"Czech Republic": "捷克", "Bosnia and Herzegovina": "波黑",
		"Paraguay": "巴拉圭", "Qatar": "卡塔尔", "Haiti": "海地", "Turkey": "土耳其",
	}
	localCn := dict[localEnName]
	if localCn == "" {
		localCn = localEnName
	}
	cleanLocal := strings.ReplaceAll(strings.ReplaceAll(localCn, "队", ""), " ", "")
	cleanOfficial := strings.ReplaceAll(strings.ReplaceAll(officialCnName, "队", ""), " ", "")
	return cleanLocal == cleanOfficial || strings.Contains(cleanOfficial, cleanLocal) || strings.Contains(cleanLocal, cleanOfficial)
}

var cnToEnDict = map[string]string{
	"巴西": "Brazil", "阿根廷": "Argentina", "法国": "France", "德国": "Germany",
	"西班牙": "Spain", "英格兰": "England", "意大利": "Italy", "荷兰": "Netherlands",
	"葡萄牙": "Portugal", "克罗地亚": "Croatia", "日本": "Japan", "美国": "USA",
	"墨西哥": "Mexico", "厄瓜多尔": "Ecuador", "南非": "South Africa",
	"委内瑞拉": "Venezuela", "牙买加": "Jamaica", "伊朗": "Iran", "威尔士": "Wales",
	"沙特": "Saudi Arabia", "沙特阿拉伯": "Saudi Arabia", "波兰": "Poland", "澳大利亚": "Australia",
	"丹麦": "Denmark", "突尼斯": "Tunisia", "哥斯达黎加": "Costa Rica",
	"比利时": "Belgium", "加拿大": "Canada", "摩洛哥": "Morocco",
	"塞尔维亚": "Serbia", "瑞士": "Switzerland", "喀麦隆": "Cameroon",
	"加纳": "Ghana", "Uruguay": "乌拉圭", "韩国": "South Korea",
	"哥伦比亚": "Colombia", "阿尔及利亚": "Algeria", "智利": "Chile",
	"尼日利亚": "Nigeria", "苏格兰": "Scotland", "匈牙利": "Hungary",
	"巴拿马": "Panama", "玻利维亚": "Bolivia", "秘鲁": "Peru",
	"捷克": "Czech Republic", "波黑": "Bosnia and Herzegovina",
	"巴拉圭": "Paraguay", "卡塔尔": "Qatar", "海地": "Haiti", "土耳其": "Turkey",
}

func getNormalizedTeamEnName(cnName, enAbbName string) string {
	cleanCn := strings.ReplaceAll(strings.ReplaceAll(cnName, "队", ""), " ", "")
	for cn, en := range cnToEnDict {
		cleanDictCn := strings.ReplaceAll(strings.ReplaceAll(cn, "队", ""), " ", "")
		if cleanCn == cleanDictCn || strings.Contains(cleanCn, cleanDictCn) || strings.Contains(cleanDictCn, cleanCn) {
			return en
		}
	}
	if enAbbName != "" {
		return enAbbName
	}
	return cnName
}
