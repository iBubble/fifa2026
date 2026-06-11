// Package service - league_registry.go
// 内置联赛注册表，定义所有支持的足球联赛及其 API 映射关系
package service

import "football/internal/models"

// allLeagues 内置联赛注册表（按推荐显示顺序排列）
var allLeagues = []models.League{
	{
		SportKey:      "soccer_epl",
		Name:          "英超",
		FullName:      "英格兰足球超级联赛",
		Country:       "England",
		Emoji:         "🏴󠁧󠁢󠁥󠁮󠁧󠁿",
		APIFootballID: 39,
		Season:        2025,
		Type:          models.LeagueDomestic,
	},
	{
		SportKey:      "soccer_spain_la_liga",
		Name:          "西甲",
		FullName:      "西班牙足球甲级联赛",
		Country:       "Spain",
		Emoji:         "🇪🇸",
		APIFootballID: 140,
		Season:        2025,
		Type:          models.LeagueDomestic,
	},
	{
		SportKey:      "soccer_italy_serie_a",
		Name:          "意甲",
		FullName:      "意大利足球甲级联赛",
		Country:       "Italy",
		Emoji:         "🇮🇹",
		APIFootballID: 135,
		Season:        2025,
		Type:          models.LeagueDomestic,
	},
	{
		SportKey:      "soccer_germany_bundesliga",
		Name:          "德甲",
		FullName:      "德国足球甲级联赛",
		Country:       "Germany",
		Emoji:         "🇩🇪",
		APIFootballID: 78,
		Season:        2025,
		Type:          models.LeagueDomestic,
	},
	{
		SportKey:      "soccer_france_ligue_one",
		Name:          "法甲",
		FullName:      "法国足球甲级联赛",
		Country:       "France",
		Emoji:         "🇫🇷",
		APIFootballID: 61,
		Season:        2025,
		Type:          models.LeagueDomestic,
	},
	{
		SportKey:      "soccer_uefa_champs_league",
		Name:          "欧冠",
		FullName:      "欧洲冠军联赛",
		Country:       "Europe",
		Emoji:         "🏆",
		APIFootballID: 2,
		Season:        2025,
		Type:          models.LeagueCup,
	},
	{
		SportKey:      "soccer_uefa_europa_league",
		Name:          "欧联",
		FullName:      "欧洲联赛",
		Country:       "Europe",
		Emoji:         "🥈",
		APIFootballID: 3,
		Season:        2025,
		Type:          models.LeagueCup,
	},
	{
		SportKey:      "soccer_fifa_world_cup",
		Name:          "世界杯",
		FullName:      "2026 FIFA 美加墨世界杯",
		Country:       "World",
		Emoji:         "🌍",
		APIFootballID: 1,
		Season:        2026,
		Type:          models.LeagueCup,
	},
	{
		SportKey:      "soccer_usa_mls",
		Name:          "美职联",
		FullName:      "美国职业足球大联盟",
		Country:       "USA",
		Emoji:         "🇺🇸",
		APIFootballID: 253,
		Season:        2026,
		Type:          models.LeagueDomestic,
	},
}

// leagueIndex 按 sportKey 索引的联赛查找表
var leagueIndex map[string]models.League

func init() {
	leagueIndex = make(map[string]models.League, len(allLeagues))
	for _, l := range allLeagues {
		leagueIndex[l.SportKey] = l
	}
}

// GetAllLeagues 获取所有支持的联赛列表
func GetAllLeagues() []models.League {
	result := make([]models.League, len(allLeagues))
	copy(result, allLeagues)
	return result
}

// GetLeagueByKey 根据 sportKey 查找联赛元数据
func GetLeagueByKey(sportKey string) (models.League, bool) {
	l, ok := leagueIndex[sportKey]
	return l, ok
}
