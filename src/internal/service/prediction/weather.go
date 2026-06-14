package prediction

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// WeatherInfo 比赛日天气数据
type WeatherInfo struct {
	Venue       string  `json:"venue"`
	City        string  `json:"city"`
	Temperature float64 `json:"temperature"` // 最高温 ℃
	TempMin     float64 `json:"tempMin"`     // 最低温 ℃
	PrecipProb  float64 `json:"precipProb"`  // 降水概率 %
	WindSpeed   float64 `json:"windSpeed"`   // 最大风速 km/h
}

// WeatherService 天气数据服务（使用 Open-Meteo 免费 API）
type WeatherService struct {
	client *http.Client
	cache  map[string]weatherCacheEntry
	mu     sync.RWMutex
}

type weatherCacheEntry struct {
	info      WeatherInfo
	fetchedAt time.Time
}

// 场馆 → 经纬度 + 城市名
type venueGeo struct {
	Lat  float64
	Lon  float64
	City string
	TZ   string // IANA 时区
}

// 17 座世界杯场馆精确经纬度
var venueCoordinates = map[string]venueGeo{
	"MetLife Stadium":                {40.8128, -74.0742, "纽约/东卢瑟福", "America/New_York"},
	"SoFi Stadium":                   {33.9534, -118.3390, "洛杉矶", "America/Los_Angeles"},
	"AT&T Stadium":                   {32.7473, -97.0945, "达拉斯", "America/Chicago"},
	"GEHA Field at Arrowhead Stadium": {39.0489, -94.4839, "堪萨斯城", "America/Chicago"},
	"NRG Stadium":                    {29.6847, -95.4107, "休斯顿", "America/Chicago"},
	"Mercedes-Benz Stadium":          {33.7554, -84.4010, "亚特兰大", "America/New_York"},
	"Lincoln Financial Field":        {39.9012, -75.1674, "费城", "America/New_York"},
	"Hard Rock Stadium":              {25.9580, -80.2389, "迈阿密", "America/New_York"},
	"Lumen Field":                    {47.5952, -122.3316, "西雅图", "America/Los_Angeles"},
	"Levi's Stadium":                 {37.4033, -121.9694, "旧金山", "America/Los_Angeles"},
	"Gillette Stadium":               {42.0909, -71.2643, "波士顿/福克斯堡", "America/New_York"},
	"Estadio Azteca":                 {19.3029, -99.1505, "墨西哥城", "America/Mexico_City"},
	"Estadio Akron":                  {20.6818, -103.4626, "瓜达拉哈拉", "America/Mexico_City"},
	"Estadio BBVA":                   {25.6723, -100.2458, "蒙特雷", "America/Monterrey"},
	"BC Place":                       {49.2768, -123.1118, "温哥华", "America/Vancouver"},
	"BMO Field":                      {43.6332, -79.4186, "多伦多", "America/Toronto"},
	"Seoul World Cup Stadium":        {37.5683, 126.8972, "首尔", "Asia/Seoul"},
}

// 48 队母国 6 月平均温度（℃）
var teamHomeClimate = map[string]float64{
	"Mexico": 25, "South Africa": 12, "South Korea": 24, "Czech Republic": 18,
	"Canada": 18, "Bosnia and Herzegovina": 20, "Qatar": 42, "Switzerland": 17,
	"Brazil": 25, "Morocco": 26, "Haiti": 30, "Scotland": 13,
	"United States": 25, "Paraguay": 20, "Australia": 14, "Turkey": 24,
	"Germany": 18, "Curaçao": 29, "Ivory Coast": 27, "Ecuador": 14,
	"Netherlands": 16, "Japan": 22, "Sweden": 15, "Tunisia": 28,
	"Belgium": 16, "Egypt": 33, "Iran": 30, "New Zealand": 10,
	"Spain": 26, "Cape Verde": 25, "Saudi Arabia": 42, "Uruguay": 13,
	"France": 19, "Senegal": 30, "Iraq": 38, "Norway": 13,
	"Argentina": 12, "Algeria": 30, "Austria": 18, "Jordan": 32,
	"Portugal": 22, "Democratic Republic of the Congo": 24,
	"Uzbekistan": 30, "Colombia": 14,
	"England": 15, "Croatia": 22, "Ghana": 27, "Panama": 28,
}

// 场馆海拔（米）
var venueAltitude = map[string]int{
	"Estadio Azteca": 2200,
	"Estadio Akron":  1566,
	"Estadio BBVA":   540,
}

// 高原适应国家
var highAltitudeTeams = map[string]bool{
	"Mexico": true, "Ecuador": true, "Colombia": true,
}

func NewWeatherService() *WeatherService {
	return &WeatherService{
		client: &http.Client{Timeout: 5 * time.Second},
		cache:  make(map[string]weatherCacheEntry),
	}
}

// GetMatchWeather 获取指定场馆在指定日期的天气预报
func (s *WeatherService) GetMatchWeather(venue string, matchDate time.Time) (WeatherInfo, error) {
	geo, ok := venueCoordinates[venue]
	if !ok {
		return WeatherInfo{}, fmt.Errorf("未知场馆: %s", venue)
	}

	cacheKey := fmt.Sprintf("%s_%s", venue, matchDate.Format("2006-01-02"))
	s.mu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Since(entry.fetchedAt) < 30*time.Minute {
		s.mu.RUnlock()
		return entry.info, nil
	}
	s.mu.RUnlock()

	dateStr := matchDate.Format("2006-01-02")
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&daily=temperature_2m_max,temperature_2m_min,precipitation_probability_max,wind_speed_10m_max"+
			"&timezone=%s&start_date=%s&end_date=%s",
		geo.Lat, geo.Lon, geo.TZ, dateStr, dateStr,
	)

	resp, err := s.client.Get(url)
	if err != nil {
		return WeatherInfo{}, fmt.Errorf("天气API请求失败: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Daily struct {
			TempMax   []float64 `json:"temperature_2m_max"`
			TempMin   []float64 `json:"temperature_2m_min"`
			PrecipMax []float64 `json:"precipitation_probability_max"`
			WindMax   []float64 `json:"wind_speed_10m_max"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return WeatherInfo{}, fmt.Errorf("天气数据解析失败: %w", err)
	}

	info := WeatherInfo{Venue: venue, City: geo.City}
	if len(result.Daily.TempMax) > 0 {
		info.Temperature = result.Daily.TempMax[0]
	}
	if len(result.Daily.TempMin) > 0 {
		info.TempMin = result.Daily.TempMin[0]
	}
	if len(result.Daily.PrecipMax) > 0 {
		info.PrecipProb = result.Daily.PrecipMax[0]
	}
	if len(result.Daily.WindMax) > 0 {
		info.WindSpeed = result.Daily.WindMax[0]
	}

	s.mu.Lock()
	s.cache[cacheKey] = weatherCacheEntry{info: info, fetchedAt: time.Now()}
	s.mu.Unlock()

	return info, nil
}

// GetClimateAdaptation 计算气候适应 λ 偏移量
func (s *WeatherService) GetClimateAdaptation(team, venue string, matchDate time.Time) float64 {
	weather, err := s.GetMatchWeather(venue, matchDate)
	if err != nil {
		return 0.0
	}

	homeTemp, ok := teamHomeClimate[team]
	if !ok {
		return 0.0
	}

	offset := 0.0
	tempDiff := math.Abs(weather.Temperature - homeTemp)

	// 温差偏移
	if tempDiff > 15 {
		offset -= 0.08
	} else if tempDiff > 10 {
		offset -= 0.04
	}

	// 高温惩罚（北欧/高纬度球队）
	if weather.Temperature > 32 && homeTemp < 18 {
		offset -= 0.03
	}

	// 降水影响（传控型球队不利）
	if weather.PrecipProb > 60 {
		offset -= 0.02
	}

	return offset
}

// GetAltitudeOffset 计算海拔因子 λ 偏移量
func GetAltitudeOffset(team, venue string) float64 {
	alt, ok := venueAltitude[venue]
	if !ok || alt < 1500 {
		return 0.0
	}
	if highAltitudeTeams[team] {
		return 0.0 // 高原国家不受影响
	}
	return -0.05 // 非高原队在高海拔场馆的体能惩罚
}

// BuildWeatherSummary 生成天气摘要文本（用于 LLM Prompt）
func (s *WeatherService) BuildWeatherSummary(homeTeam, awayTeam, venue string, matchDate time.Time) string {
	weather, err := s.GetMatchWeather(venue, matchDate)
	if err != nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("赛场: %s, %s\n", weather.Venue, weather.City))
	sb.WriteString(fmt.Sprintf("比赛日天气: %.0f~%.0f℃, 降水概率%.0f%%, 风速%.0fkm/h\n",
		weather.TempMin, weather.Temperature, weather.PrecipProb, weather.WindSpeed))

	if ht, ok := teamHomeClimate[homeTeam]; ok {
		diff := math.Abs(weather.Temperature - ht)
		sb.WriteString(fmt.Sprintf("主队%s母国6月均温%.0f℃ (温差%.0f℃)\n", homeTeam, ht, diff))
	}
	if at, ok := teamHomeClimate[awayTeam]; ok {
		diff := math.Abs(weather.Temperature - at)
		sb.WriteString(fmt.Sprintf("客队%s母国6月均温%.0f℃ (温差%.0f℃)\n", awayTeam, at, diff))
	}

	// 海拔提示
	if alt, ok := venueAltitude[venue]; ok && alt > 500 {
		sb.WriteString(fmt.Sprintf("⚠️ 赛场海拔%dm", alt))
		if alt > 1500 {
			sb.WriteString("（高海拔，非高原球队体能消耗增加）")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}
