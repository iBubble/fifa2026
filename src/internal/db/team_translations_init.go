package db

import "log"

// InitTeamTranslations 初始化 48 支 2026 世界杯参赛队的权威中英文映射数据
// api_team_id 来源于 api-football (v3.football.api-sports.io)
func InitTeamTranslations() {
	teams := []TeamTranslation{
		// ── Group A ──
		{EnName: "Mexico", CnName: "墨西哥", FlagCode: "mx", InitialElo: 1710, AvgGoalsScored: 1.35, AvgGoalsConceded: 1.25, CleanSheetRate: 0.27, ApiTeamID: 16, Aliases: []string{"MEX", "墨西哥队"}, FifaRanking: 17},
		{EnName: "South Africa", CnName: "南非", FlagCode: "za", InitialElo: 1610, AvgGoalsScored: 1.10, AvgGoalsConceded: 1.25, CleanSheetRate: 0.22, ApiTeamID: 1531, Aliases: []string{"RSA", "南非队"}, FifaRanking: 66},
		{EnName: "South Korea", CnName: "韩国", FlagCode: "kr", InitialElo: 1690, AvgGoalsScored: 1.35, AvgGoalsConceded: 1.20, CleanSheetRate: 0.28, ApiTeamID: 17, Aliases: []string{"KOR", "韩国队", "太极虎"}, FifaRanking: 23},
		{EnName: "Czech Republic", CnName: "捷克", FlagCode: "cz", InitialElo: 1680, AvgGoalsScored: 1.30, AvgGoalsConceded: 1.15, CleanSheetRate: 0.30, ApiTeamID: 770, Aliases: []string{"CZE", "捷克队"}, FifaRanking: 32},
		// ── Group B ──
		{EnName: "Canada", CnName: "加拿大", FlagCode: "ca", InitialElo: 1640, AvgGoalsScored: 1.20, AvgGoalsConceded: 1.30, CleanSheetRate: 0.24, ApiTeamID: 5529, Aliases: []string{"CAN", "加拿大队"}, FifaRanking: 48},
		{EnName: "Bosnia and Herzegovina", CnName: "波黑", FlagCode: "ba", InitialElo: 1620, AvgGoalsScored: 1.15, AvgGoalsConceded: 1.25, CleanSheetRate: 0.25, ApiTeamID: 773, Aliases: []string{"BIH", "波黑队", "波斯尼亚和黑塞哥维那"}, FifaRanking: 60},
		{EnName: "Qatar", CnName: "卡塔尔", FlagCode: "qa", InitialElo: 1600, AvgGoalsScored: 1.10, AvgGoalsConceded: 1.35, CleanSheetRate: 0.20, ApiTeamID: 1569, Aliases: []string{"QAT", "卡塔尔队"}, FifaRanking: 44},
		{EnName: "Switzerland", CnName: "瑞士", FlagCode: "ch", InitialElo: 1730, AvgGoalsScored: 1.45, AvgGoalsConceded: 1.10, CleanSheetRate: 0.35, ApiTeamID: 15, Aliases: []string{"SUI", "瑞士队"}, FifaRanking: 15},
		// ── Group C ──
		{EnName: "Brazil", CnName: "巴西", FlagCode: "br", InitialElo: 1880, AvgGoalsScored: 2.25, AvgGoalsConceded: 0.95, CleanSheetRate: 0.42, ApiTeamID: 6, Aliases: []string{"BRA", "巴西队", "桑巴军团"}, FifaRanking: 5},
		{EnName: "Morocco", CnName: "摩洛哥", FlagCode: "ma", InitialElo: 1760, AvgGoalsScored: 1.50, AvgGoalsConceded: 0.90, CleanSheetRate: 0.40, ApiTeamID: 31, Aliases: []string{"MAR", "摩洛哥队"}, FifaRanking: 14},
		{EnName: "Haiti", CnName: "海地", FlagCode: "ht", InitialElo: 1510, AvgGoalsScored: 0.90, AvgGoalsConceded: 1.70, CleanSheetRate: 0.15, ApiTeamID: 1535, Aliases: []string{"HAI", "海地队"}, FifaRanking: 86},
		{EnName: "Scotland", CnName: "苏格兰", FlagCode: "gb-sct", InitialElo: 1660, AvgGoalsScored: 1.20, AvgGoalsConceded: 1.15, CleanSheetRate: 0.30, ApiTeamID: 1108, Aliases: []string{"SCO", "苏格兰队"}, FifaRanking: 50},
		// ── Group D ──
		{EnName: "United States", CnName: "美国", FlagCode: "us", InitialElo: 1700, AvgGoalsScored: 1.30, AvgGoalsConceded: 1.30, CleanSheetRate: 0.25, ApiTeamID: 2384, Aliases: []string{"USA", "美国队", "美利坚"}, FifaRanking: 16},
		{EnName: "Paraguay", CnName: "巴拉圭", FlagCode: "py", InitialElo: 1650, AvgGoalsScored: 1.10, AvgGoalsConceded: 1.05, CleanSheetRate: 0.35, ApiTeamID: 2380, Aliases: []string{"PAR", "巴拉圭队"}, FifaRanking: 35},
		{EnName: "Australia", CnName: "澳大利亚", FlagCode: "au", InitialElo: 1650, AvgGoalsScored: 1.25, AvgGoalsConceded: 1.20, CleanSheetRate: 0.27, ApiTeamID: 20, Aliases: []string{"AUS", "澳大利亚队", "袋鼠军团"}, FifaRanking: 24},
		{EnName: "Turkey", CnName: "土耳其", FlagCode: "tr", InitialElo: 1710, AvgGoalsScored: 1.40, AvgGoalsConceded: 1.25, CleanSheetRate: 0.28, ApiTeamID: 777, Aliases: []string{"TUR", "土耳其队", "星月军团"}, FifaRanking: 28},
	}
	for _, t := range teams {
		if err := SaveTeamTranslation(t); err != nil {
			log.Printf("[TeamTranslations] ⚠️ 保存 %s 失败: %v", t.EnName, err)
		}
	}
	initTeamTranslationsPartB()
	initTeamTranslationsPartC()
	log.Println("[TeamTranslations] ✅ 48 支参赛队中英文映射初始化完成")
}
