package prediction

import (
	"encoding/json"
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

// FetchAllOdds 拉取体彩官网全部实时赔率并存入内存缓存，限制半小时内最多请求一次
func (s *SportteryService) FetchAllOdds() {
	s.mu.Lock()
	// 如果已经在抓取，或者缓存还没过期，直接无阻塞返回
	if s.isFetching || (time.Since(s.lastFetchTime) < 30*time.Minute && len(s.cachedOdds) > 0) {
		s.mu.Unlock()
		return
	}
	s.isFetching = true
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.isFetching = false
		s.mu.Unlock()
	}()

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest("GET", "https://webapi.sporttery.cn/gateway/jc/football/getMatchCalculatorV1.qry?poolCode=had,hhad,crs&channel=c", nil)
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

	// 允许 200 和云盾的 567 挑战状态码，只要内容是 JSON 即可
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
					Crs map[string]interface{} `json:"crs"`
				} `json:"subMatchList"`
			} `json:"matchInfoList"`
		} `json:"value"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil || !res.Success {
		return
	}

	// 内存写入时重新加锁，采用增量覆盖方式，保留已下架赛事的历史真实赔率
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

			s.cachedOdds[key] = OfficialOdds{
				HomeOdds:     h,
				DrawOdds:     d,
				AwayOdds:     a,
				GoalLine:     gl,
				HhadHomeOdds: hh,
				HhadDrawOdds: hd,
				HhadAwayOdds: ha,
				CrsOdds:      crsOdds,
				IsAvailable:  h > 0.0,
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
