package db

import "log"

func initTeamTranslationsPartB() {
	teams := []TeamTranslation{
		// ── Group E ──
		{EnName: "Germany", CnName: "德国", FlagCode: "de", InitialElo: 1810, AvgGoalsScored: 2.10, AvgGoalsConceded: 1.15, CleanSheetRate: 0.35, ApiTeamID: 25, Aliases: []string{"GER", "德国队", "德意志战车"}, FifaRanking: 13},
		{EnName: "Curaçao", CnName: "库拉索", FlagCode: "cw", InitialElo: 1480, AvgGoalsScored: 0.85, AvgGoalsConceded: 1.80, CleanSheetRate: 0.12, ApiTeamID: 458, Aliases: []string{"CUW", "Curacao", "库拉索队"}, FifaRanking: 90},
		{EnName: "Ivory Coast", CnName: "科特迪瓦", FlagCode: "ci", InitialElo: 1640, AvgGoalsScored: 1.25, AvgGoalsConceded: 1.20, CleanSheetRate: 0.28, ApiTeamID: 5041, Aliases: []string{"CIV", "科特迪瓦队", "Côte d'Ivoire"}, FifaRanking: 38},
		{EnName: "Ecuador", CnName: "厄瓜多尔", FlagCode: "ec", InitialElo: 1690, AvgGoalsScored: 1.25, AvgGoalsConceded: 1.15, CleanSheetRate: 0.29, ApiTeamID: 2382, Aliases: []string{"ECU", "厄瓜多尔队"}, FifaRanking: 30},
		// ── Group F ──
		{EnName: "Netherlands", CnName: "荷兰", FlagCode: "nl", InitialElo: 1815, AvgGoalsScored: 1.90, AvgGoalsConceded: 1.00, CleanSheetRate: 0.36, ApiTeamID: 1118, Aliases: []string{"NED", "荷兰队", "橙衣军团"}, FifaRanking: 8},
		{EnName: "Japan", CnName: "日本", FlagCode: "jp", InitialElo: 1720, AvgGoalsScored: 1.40, AvgGoalsConceded: 1.20, CleanSheetRate: 0.28, ApiTeamID: 12, Aliases: []string{"JPN", "日本队", "蓝武士"}, FifaRanking: 18},
		{EnName: "Sweden", CnName: "瑞典", FlagCode: "se", InitialElo: 1700, AvgGoalsScored: 1.35, AvgGoalsConceded: 1.15, CleanSheetRate: 0.30, ApiTeamID: 1104, Aliases: []string{"SWE", "瑞典队"}, FifaRanking: 26},
		{EnName: "Tunisia", CnName: "突尼斯", FlagCode: "tn", InitialElo: 1620, AvgGoalsScored: 1.10, AvgGoalsConceded: 1.15, CleanSheetRate: 0.30, ApiTeamID: 28, Aliases: []string{"TUN", "突尼斯队"}, FifaRanking: 41},
		// ── Group G ──
		{EnName: "Belgium", CnName: "比利时", FlagCode: "be", InitialElo: 1790, AvgGoalsScored: 1.75, AvgGoalsConceded: 1.05, CleanSheetRate: 0.35, ApiTeamID: 1, Aliases: []string{"BEL", "比利时队", "红魔"}, FifaRanking: 6},
		{EnName: "Egypt", CnName: "埃及", FlagCode: "eg", InitialElo: 1630, AvgGoalsScored: 1.15, AvgGoalsConceded: 1.10, CleanSheetRate: 0.32, ApiTeamID: 3588, Aliases: []string{"EGY", "埃及队"}, FifaRanking: 36},
		{EnName: "Iran", CnName: "伊朗", FlagCode: "ir", InitialElo: 1660, AvgGoalsScored: 1.20, AvgGoalsConceded: 1.10, CleanSheetRate: 0.33, ApiTeamID: 22, Aliases: []string{"IRN", "伊朗队"}, FifaRanking: 21},
		{EnName: "New Zealand", CnName: "新西兰", FlagCode: "nz", InitialElo: 1520, AvgGoalsScored: 0.95, AvgGoalsConceded: 1.55, CleanSheetRate: 0.18, ApiTeamID: 1530, Aliases: []string{"NZL", "新西兰队"}, FifaRanking: 95},
		// ── Group H ──
		{EnName: "Spain", CnName: "西班牙", FlagCode: "es", InitialElo: 1830, AvgGoalsScored: 1.80, AvgGoalsConceded: 0.85, CleanSheetRate: 0.45, ApiTeamID: 9, Aliases: []string{"ESP", "西班牙队", "斗牛士"}, FifaRanking: 3},
		{EnName: "Cape Verde", CnName: "佛得角", FlagCode: "cv", InitialElo: 1500, AvgGoalsScored: 0.90, AvgGoalsConceded: 1.40, CleanSheetRate: 0.18, ApiTeamID: 5575, Aliases: []string{"CPV", "佛得角队", "Cape Verde Islands"}, FifaRanking: 70},
		{EnName: "Saudi Arabia", CnName: "沙特阿拉伯", FlagCode: "sa", InitialElo: 1620, AvgGoalsScored: 1.15, AvgGoalsConceded: 1.20, CleanSheetRate: 0.28, ApiTeamID: 23, Aliases: []string{"KSA", "沙特", "沙特队", "沙特阿拉伯队"}, FifaRanking: 65},
		{EnName: "Uruguay", CnName: "乌拉圭", FlagCode: "uy", InitialElo: 1760, AvgGoalsScored: 1.55, AvgGoalsConceded: 1.00, CleanSheetRate: 0.36, ApiTeamID: 7, Aliases: []string{"URU", "乌拉圭队"}, FifaRanking: 11},
	}
	for _, t := range teams {
		if err := SaveTeamTranslation(t); err != nil {
			log.Printf("[TeamTranslations] ⚠️ 保存 %s 失败: %v", t.EnName, err)
		}
	}
}
