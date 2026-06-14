package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/models"
	"log"
	"math"
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
	MatchTime    time.Time          `json:"matchTime"`
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
			if !strings.Contains(m.LeagueAllName, "世界杯") {
				continue
			}
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

			var matchTime time.Time
			loc, errLoc := time.LoadLocation("Asia/Shanghai")
			if errLoc == nil {
				matchTime, _ = time.ParseInLocation("2006-01-02 15:04:05", m.MatchDate+" "+m.MatchTime, loc)
			} else {
				matchTime, _ = time.Parse("2006-01-02 15:04:05", m.MatchDate+" "+m.MatchTime)
			}

			// 获取缓存中的旧赔率以进行突变率校验
			var oldH, oldD, oldA float64
			if oldOdds, exists := s.cachedOdds[key]; exists {
				oldH = oldOdds.HomeOdds
				oldD = oldOdds.DrawOdds
				oldA = oldOdds.AwayOdds
			}

			// 执行熔断器校验
			isCircuitBroken := checkOddsCircuitBreaker(h, d, a, oldH, oldD, oldA)

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
				IsAvailable:  (h > 0.0 || hh > 0.0 || len(crsOdds) > 0) && !isCircuitBroken,
				MatchTime:    matchTime,
			}

			if isCircuitBroken && h > 0.0 {
				log.Printf("[CircuitBreaker] ⚠️ 比赛 %s 赔率异常 (H:%.2f, D:%.2f, A:%.2f)，触发熔断，挂起凯利仓位推荐", key, h, d, a)
			}
		}
	}

	s.lastFetchTime = time.Now()
	s.mu.Unlock()
}

// GetMatchOdds 获取指定比赛的体彩官网最新赔率，并采用双重核实匹配机制
func (s *SportteryService) GetMatchOdds(homeTeam, awayTeam string, scheduledAt time.Time) OfficialOdds {
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
			// 第二重校验：开赛时间差在 24 小时以内
			timeDiff := odds.MatchTime.Sub(scheduledAt)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			if timeDiff <= 24*time.Hour {
				return odds
			}
		}
	}
	return OfficialOdds{IsAvailable: false}
}

// matchTeam 严格英译名称匹配，调用包内 NormalizeTeam 归一化后比对
func matchTeam(localEnName, officialCnName string) bool {
	localStd := NormalizeTeam(localEnName)
	officialStd := NormalizeTeam(officialCnName)
	return localStd != "" && officialStd != "" && localStd == officialStd
}

var cnToEnDict = map[string]string{
	// 2026 世界杯 48 支参赛队中文→英文标准名映射（权威来源：team_translations 表）
	"巴西": "Brazil", "阿根廷": "Argentina", "法国": "France", "德国": "Germany",
	"西班牙": "Spain", "英格兰": "England", "意大利": "Italy", "荷兰": "Netherlands",
	"葡萄牙": "Portugal", "克罗地亚": "Croatia", "日本": "Japan",
	"美国": "United States", "墨西哥": "Mexico", "厄瓜多尔": "Ecuador", "南非": "South Africa",
	"伊朗": "Iran", "沙特": "Saudi Arabia", "沙特阿拉伯": "Saudi Arabia",
	"澳大利亚": "Australia", "突尼斯": "Tunisia",
	"比利时": "Belgium", "加拿大": "Canada", "摩洛哥": "Morocco",
	"瑞士": "Switzerland", "加纳": "Ghana", "乌拉圭": "Uruguay", "韩国": "South Korea",
	"哥伦比亚": "Colombia", "阿尔及利亚": "Algeria", "苏格兰": "Scotland",
	"巴拿马": "Panama", "捷克": "Czech Republic", "波黑": "Bosnia and Herzegovina",
	"巴拉圭": "Paraguay", "卡塔尔": "Qatar", "海地": "Haiti", "土耳其": "Turkey",
	"库拉索": "Curaçao", "科特迪瓦": "Ivory Coast", "瑞典": "Sweden",
	"埃及": "Egypt", "新西兰": "New Zealand", "佛得角": "Cape Verde",
	"塞内加尔": "Senegal", "伊拉克": "Iraq", "挪威": "Norway",
	"奥地利": "Austria", "约旦": "Jordan",
	"民主刚果": "Democratic Republic of the Congo", "刚果（金）": "Democratic Republic of the Congo",
	"刚果民主共和国": "Democratic Republic of the Congo",
	"乌兹别克斯坦": "Uzbekistan",
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

// applyOddsShiftsToProbs 根据博彩巨头的赔率偏移，微调胜平负的基础几率，使推荐更灵敏地反映巨头大单资金偏向
func applyOddsShiftsToProbs(homeTeam, awayTeam string, pHome, pDraw, pAway float64) (float64, float64, float64) {
	tracker := &OddsTrackerService{}
	shifts := tracker.GetOddsShifts(homeTeam, awayTeam)
	var shiftH, shiftD, shiftA float64
	for _, s := range shifts {
		if s.Bookmaker == "Bet365" && s.Outcome == "主胜" {
			shiftH = s.ShiftPct
		} else if s.Bookmaker == "Pinnacle (平博)" && s.Outcome == "平局" {
			shiftD = s.ShiftPct
		} else if s.Bookmaker == "William Hill" && s.Outcome == "客胜" {
			shiftA = s.ShiftPct
		}
	}

	// 降水（负值）代表资金看好、几率微升；升水（正值）代表资金撤出，几率微降
	// 影响因子设为 0.005 (0.5%) 以保持合理的偏置修正，防过度倾斜
	pH := pHome * (1.0 - 0.005*shiftH)
	pD := pDraw * (1.0 - 0.005*shiftD)
	pA := pAway * (1.0 - 0.005*shiftA)

	// 归一化
	sum := pH + pD + pA
	if sum > 0 {
		pH = pH / sum
		pD = pD / sum
		pA = pA / sum
	}
	return pH, pD, pA
}

// checkOddsCircuitBreaker 熔断器：校验赔率合法性、抽水率 Margin 和赔率突变偏离度
func checkOddsCircuitBreaker(newH, newD, newA float64, oldH, oldD, oldA float64) bool {
	// 1. 基本合法性校验：体彩非售或异常时赔率可能为 0 或小于 1.0 等
	if newH <= 1.01 || newD <= 1.01 || newA <= 1.01 {
		return true // 熔断
	}

	// 2. 抽水率 Margin 校验
	// Margin = 1.0 - 1.0 / (1.0/H + 1.0/D + 1.0/A)
	invSum := 1.0/newH + 1.0/newD + 1.0/newA
	margin := 1.0 - 1.0/invSum
	if margin < -0.02 || margin > 0.30 {
		// 抽水率异常（小于 -2% 或大于 30%），触发熔断。体彩正常抽水一般在 5% 到 15% 之间
		return true
	}

	// 3. 赔率突变校验 (单次变动大于 50%)
	if oldH > 0.0 {
		diffH := math.Abs(newH-oldH) / oldH
		diffD := math.Abs(newD-oldD) / oldD
		diffA := math.Abs(newA-oldA) / oldA
		if diffH > 0.50 || diffD > 0.50 || diffA > 0.50 {
			// 赔率变动超过 50%，可能是数据录入错误或接口异常，熔断保护
			return true
		}
	}

	return false // 未触发熔断
}

// applyShiftsToMatrix 将比分概率矩阵依据赔率偏移进行调整
func applyShiftsToMatrix(homeTeam, awayTeam string, matrix []models.ScoreProbability) []models.ScoreProbability {
	pHomeOrig, pDrawOrig, pAwayOrig := 0.0, 0.0, 0.0
	for _, cell := range matrix {
		if cell.HomeScore > cell.AwayScore {
			pHomeOrig += cell.Prob
		} else if cell.HomeScore == cell.AwayScore {
			pDrawOrig += cell.Prob
		} else {
			pAwayOrig += cell.Prob
		}
	}

	pH, pD, pA := applyOddsShiftsToProbs(homeTeam, awayTeam, pHomeOrig, pDrawOrig, pAwayOrig)

	for i := range matrix {
		cell := &matrix[i]
		if cell.HomeScore > cell.AwayScore && pHomeOrig > 0 {
			cell.Prob = cell.Prob * (pH / pHomeOrig)
		} else if cell.HomeScore == cell.AwayScore && pDrawOrig > 0 {
			cell.Prob = cell.Prob * (pD / pDrawOrig)
		} else if cell.HomeScore < cell.AwayScore && pAwayOrig > 0 {
			cell.Prob = cell.Prob * (pA / pAwayOrig)
		}
	}
	return matrix
}
