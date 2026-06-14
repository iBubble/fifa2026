package db

import "log"

func initTeamTranslationsPartC() {
	teams := []TeamTranslation{
		// ── Group I ──
		{EnName: "France", CnName: "法国", FlagCode: "fr", InitialElo: 1850, AvgGoalsScored: 1.95, AvgGoalsConceded: 0.90, CleanSheetRate: 0.40, ApiTeamID: 2, Aliases: []string{"FRA", "法国队", "高卢雄鸡"}, FifaRanking: 2},
		{EnName: "Senegal", CnName: "塞内加尔", FlagCode: "sn", InitialElo: 1660, AvgGoalsScored: 1.20, AvgGoalsConceded: 1.10, CleanSheetRate: 0.30, ApiTeamID: 42, Aliases: []string{"SEN", "塞内加尔队"}, FifaRanking: 20},
		{EnName: "Iraq", CnName: "伊拉克", FlagCode: "iq", InitialElo: 1570, AvgGoalsScored: 1.05, AvgGoalsConceded: 1.30, CleanSheetRate: 0.22, ApiTeamID: 5581, Aliases: []string{"IRQ", "伊拉克队"}, FifaRanking: 62},
		{EnName: "Norway", CnName: "挪威", FlagCode: "no", InitialElo: 1680, AvgGoalsScored: 1.30, AvgGoalsConceded: 1.15, CleanSheetRate: 0.28, ApiTeamID: 1106, Aliases: []string{"NOR", "挪威队"}, FifaRanking: 27},
		// ── Group J ──
		{EnName: "Argentina", CnName: "阿根廷", FlagCode: "ar", InitialElo: 1865, AvgGoalsScored: 2.05, AvgGoalsConceded: 1.05, CleanSheetRate: 0.38, ApiTeamID: 26, Aliases: []string{"ARG", "阿根廷队"}, FifaRanking: 1},
		{EnName: "Algeria", CnName: "阿尔及利亚", FlagCode: "dz", InitialElo: 1620, AvgGoalsScored: 1.15, AvgGoalsConceded: 1.15, CleanSheetRate: 0.30, ApiTeamID: 1530, Aliases: []string{"ALG", "阿尔及利亚队"}, FifaRanking: 46},
		{EnName: "Austria", CnName: "奥地利", FlagCode: "at", InitialElo: 1690, AvgGoalsScored: 1.35, AvgGoalsConceded: 1.15, CleanSheetRate: 0.30, ApiTeamID: 775, Aliases: []string{"AUT", "奥地利队"}, FifaRanking: 25},
		{EnName: "Jordan", CnName: "约旦", FlagCode: "jo", InitialElo: 1560, AvgGoalsScored: 1.00, AvgGoalsConceded: 1.30, CleanSheetRate: 0.22, ApiTeamID: 5579, Aliases: []string{"JOR", "约旦队"}, FifaRanking: 75},
		// ── Group K ──
		{EnName: "Portugal", CnName: "葡萄牙", FlagCode: "pt", InitialElo: 1820, AvgGoalsScored: 1.80, AvgGoalsConceded: 1.05, CleanSheetRate: 0.34, ApiTeamID: 27, Aliases: []string{"POR", "葡萄牙队"}, FifaRanking: 7},
		{EnName: "Democratic Republic of the Congo", CnName: "民主刚果", FlagCode: "cd", InitialElo: 1540, AvgGoalsScored: 1.00, AvgGoalsConceded: 1.40, CleanSheetRate: 0.20, ApiTeamID: 5575, Aliases: []string{"COD", "DR Congo", "刚果民主共和国", "刚果（金）", "民主刚果队"}, FifaRanking: 72},
		{EnName: "Uzbekistan", CnName: "乌兹别克斯坦", FlagCode: "uz", InitialElo: 1580, AvgGoalsScored: 1.10, AvgGoalsConceded: 1.20, CleanSheetRate: 0.25, ApiTeamID: 5583, Aliases: []string{"UZB", "乌兹别克斯坦队"}, FifaRanking: 58},
		{EnName: "Colombia", CnName: "哥伦比亚", FlagCode: "co", InitialElo: 1740, AvgGoalsScored: 1.50, AvgGoalsConceded: 1.10, CleanSheetRate: 0.33, ApiTeamID: 2383, Aliases: []string{"COL", "哥伦比亚队"}, FifaRanking: 12},
		// ── Group L ──
		{EnName: "England", CnName: "英格兰", FlagCode: "gb-eng", InitialElo: 1825, AvgGoalsScored: 1.85, AvgGoalsConceded: 0.95, CleanSheetRate: 0.39, ApiTeamID: 10, Aliases: []string{"ENG", "英格兰队", "三狮军团"}, FifaRanking: 4},
		{EnName: "Croatia", CnName: "克罗地亚", FlagCode: "hr", InitialElo: 1790, AvgGoalsScored: 1.55, AvgGoalsConceded: 1.00, CleanSheetRate: 0.33, ApiTeamID: 3, Aliases: []string{"CRO", "克罗地亚队", "格子军团"}, FifaRanking: 22},
		{EnName: "Ghana", CnName: "加纳", FlagCode: "gh", InitialElo: 1610, AvgGoalsScored: 1.15, AvgGoalsConceded: 1.25, CleanSheetRate: 0.24, ApiTeamID: 5575, Aliases: []string{"GHA", "加纳队"}, FifaRanking: 80},
		{EnName: "Panama", CnName: "巴拿马", FlagCode: "pa", InitialElo: 1560, AvgGoalsScored: 1.00, AvgGoalsConceded: 1.35, CleanSheetRate: 0.22, ApiTeamID: 1536, Aliases: []string{"PAN", "巴拿马队"}, FifaRanking: 43},
		// ── 额外冗余映射：意大利（history_features.json 中存在但未参赛，保留 Elo 兜底） ──
		{EnName: "Italy", CnName: "意大利", FlagCode: "it", InitialElo: 1800, AvgGoalsScored: 1.70, AvgGoalsConceded: 0.80, CleanSheetRate: 0.48, ApiTeamID: 768, Aliases: []string{"ITA", "意大利队", "蓝衣军团"}, FifaRanking: 10},
	}
	for _, t := range teams {
		if err := SaveTeamTranslation(t); err != nil {
			log.Printf("[TeamTranslations] ⚠️ 保存 %s 失败: %v", t.EnName, err)
		}
	}
}
