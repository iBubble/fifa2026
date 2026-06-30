package v1

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/service/ai"
	"fmt"
	"math"
	"strings"
)

// GetTeamCnName 根据英文名翻译中文名
func (ctrl *APIController) GetTeamCnName(enName string) string {
	t, err := db.GetTeamTranslation(enName)
	if err == nil && t.CnName != "" {
		return t.CnName
	}
	return enName
}

// fillSchemeProb 为生成的方案回填基于 Dixon-Coles 预测模型的精确中奖概率
func (ctrl *APIController) fillSchemeProb(scheme []ai.BetAdviceItem, matchNameToInput map[string]ai.BetAdviceMatchInput) []ai.BetAdviceItem {
	for idx, item := range scheme {
		parts := strings.Split(item.MatchName, "&")
		sels := strings.Split(item.Selection, "&")
		totalP := 1.0
		hasValidMatch := false
		for i, part := range parts {
			if i >= len(sels) {
				break
			}
			mName := strings.ToLower(strings.TrimSpace(part))
			sel := strings.TrimSpace(sels[i])
			mInput, ok := matchNameToInput[mName]
			if !ok {
				for k, v := range matchNameToInput {
					if strings.Contains(mName, k) || strings.Contains(k, mName) {
						mInput = v
						ok = true
						break
					}
				}
			}
			if ok {
				m, errM := db.GetMatch(mInput.MatchID)
				if errM == nil {
					rep, _ := db.GetPredictionReport(m.ID)
					advices := ctrl.LotteryService.GenerateFivePlaysAdvice(m, &rep, mInput.IsSingleHad)
					singleP := 0.0
					found := false
					for _, adv := range advices {
						for _, opt := range adv.Safe {
							if opt.Option == sel {
								singleP = opt.Prob
								found = true
								break
							}
						}
						if found {
							break
						}
						for _, opt := range adv.Aggressive {
							if opt.Option == sel {
								singleP = opt.Prob
								found = true
								break
							}
						}
						if found {
							break
						}
					}
					if !found {
						if sel == "主胜" {
							singleP = mInput.HomeProb
						} else if sel == "平局" {
							singleP = mInput.DrawProb
						} else if sel == "客胜" {
							singleP = mInput.AwayProb
						} else {
							singleP = 0.33
						}
					}
					totalP *= singleP
					hasValidMatch = true
				}
			}
		}
		if hasValidMatch {
			scheme[idx].Prob = totalP
		}
	}
	return scheme
}

// buildBetAdviceMarkdown 智能组装投注方案 Markdown 报告文本
func (ctrl *APIController) buildBetAdviceMarkdown(reqDate string, targetInputs []ai.BetAdviceMatchInput, agentAmount float64, safeRatio float64, result *ai.BetAdviceResult, mixedScheme []ai.BetAdviceItem, betTypeStr string, mixedProb float64, mixedOdds float64) string {
	md := fmt.Sprintf("# FIFA 2026 投注方案推荐报告 (%s)\n\n", reqDate)
	md += fmt.Sprintf("本项目基于 Dixon-Coles 泊松定量模型与实时赔率偏差（EV）进行多维度智能计算，结合小组赛阶段限额拦截风控规范，专为 %s 的比赛提供量化策略。\n\n---\n\n", reqDate)
	md += "## 一、 核心预测数据概览（竞彩官方实际赔率/在售状态校准）\n\n经过对竞彩官网（[sporttery.cn](https://www.sporttery.cn/jc/jsq/zqhhgg/)）实际在售状态核对：\n"
	for i, m := range targetInputs {
		statusStr := "仅限【过关】"
		if m.IsSingleHad {
			statusStr = "常规胜平负【单关】"
		}
		md += fmt.Sprintf("%d. **%s vs %s**：%s。\n", i+1, m.HomeCn, m.AwayCn, statusStr)
	}
	md += "\n| 赛事 ID | 比赛对阵 | 官方实际赔率 (胜/平/负) | 让球赔率 (让胜/让平/让负) | 模型预测概率 (胜/平/负) | 单关可用状态 |\n| :--- | :--- | :--- | :--- | :--- | :--- |\n"
	for _, m := range targetInputs {
		oddsStr := fmt.Sprintf("%.2f / %.2f / %.2f", m.HomeOdds, m.DrawOdds, m.AwayOdds)
		hhadOddsStr := fmt.Sprintf("%.2f / %.2f / %.2f *(让球%d)*", m.HhadHomeOdds, m.HhadDrawOdds, m.HhadAwayOdds, m.GoalLine)
		if m.GoalLine > 0 {
			hhadOddsStr = fmt.Sprintf("%.2f / %.2f / %.2f *(让球+%d)*", m.HhadHomeOdds, m.HhadDrawOdds, m.HhadAwayOdds, m.GoalLine)
		}
		probStr := fmt.Sprintf("%.1f%% / %.1f%% / %.1f%%", m.HomeProb*100, m.DrawProb*100, m.AwayProb*100)
		statusStr := "仅限【过关】"
		if m.IsSingleHad {
			statusStr = "常规胜平负【单关】"
		}
		md += fmt.Sprintf("| %s | %s vs %s | %s | %s | %s | %s |\n", m.MatchID, m.HomeCn, m.AwayCn, oddsStr, hhadOddsStr, probStr, statusStr)
	}
	md += "\n---\n\n"
	md += fmt.Sprintf("## 二、 方案一：稳妥型方案（总投入：%.0f 元）\n\n针对单关限制限制进行结构优化：仅对单关比赛购买常规单关或对冲，其余比赛的常规投注以 2 串 1 比例分仓。\n\n### 1. 单场单关投注\n", math.Round(agentAmount*safeRatio))
	hasSafeSingle := false
	for _, item := range result.SafeScheme {
		if item.BetType == "单关" {
			hasSafeSingle = true
			md += fmt.Sprintf("- **%s** (%s)：选择「%s」@%.2f *(概率: %.1f%%)* ── 推荐投注 %.0f 元。若中返奖 **%.2f 元**。\n", item.MatchName, item.Market, item.Selection, item.Odds, item.Prob*100, item.Stake, item.Stake*item.Odds)
		}
	}
	if !hasSafeSingle {
		md += "- 无\n"
	}
	md += "\n### 2. 混合过关 (2 串 1) 方案\n"
	hasSafeParlay := false
	for _, item := range result.SafeScheme {
		if item.BetType == "2串1" {
			hasSafeParlay = true
			md += fmt.Sprintf("- **%s** (%s) ── 组合赔率 **%.2f** *(中奖率: %.1f%%)*，推荐投注 %.0f 元。若中返奖 **%.2f 元**。\n", item.MatchName, item.Selection, item.Odds, item.Prob*100, item.Stake, item.Stake*item.Odds)
		}
	}
	if !hasSafeParlay {
		md += "- 无\n"
	}
	md += "\n### 3. 高串过关方案\n"
	hasSafeHigh := false
	for _, item := range result.SafeScheme {
		if item.BetType != "单关" && item.BetType != "2串1" && item.BetType != "1串1" && strings.Contains(item.BetType, "串1") {
			hasSafeHigh = true
			md += fmt.Sprintf("- **%s** (%s) ── 组合赔率 **%.2f** *(中奖率: %.1f%%)*，推荐投注 %.0f 元。若中返奖 **%.2f 元**。\n", item.MatchName, item.Selection, item.Odds, item.Prob*100, item.Stake, item.Stake*item.Odds)
		}
	}
	if !hasSafeHigh {
		md += "- 无\n"
	}
	md += "\n---\n\n"
	md += fmt.Sprintf("## 三、 方案二：激进爆冷型方案（总投入：%.0f 元）\n\n注重高赔率冷门或半全场高收益博弈，多场比赛让球玩法以 2 串 1 为核心，高置信度高赔率做单关。\n\n### 1. 单场激进单关/半全场/比分投注\n", agentAmount-math.Round(agentAmount*safeRatio))
	hasAggSingle := false
	for _, item := range result.AggressiveScheme {
		if item.BetType == "单关" {
			hasAggSingle = true
			md += fmt.Sprintf("- **%s** (%s)：选择「%s」@%.2f *(概率: %.1f%%)* ── 推荐投注 %.0f 元。若中返奖 **%.2f 元**。\n", item.MatchName, item.Market, item.Selection, item.Odds, item.Prob*100, item.Stake, item.Stake*item.Odds)
		}
	}
	if !hasAggSingle {
		md += "- 无\n"
	}
	md += "\n### 2. 让球混合过关 (2 串 1) 方案\n"
	hasAggParlay := false
	for _, item := range result.AggressiveScheme {
		if item.BetType == "2串1" {
			hasAggParlay = true
			md += fmt.Sprintf("- **%s** (%s) ── 组合赔率 **%.2f** *(中奖率: %.1f%%)*，推荐投注 %.0f 元。若中返奖 **%.2f 元**。\n", item.MatchName, item.Selection, item.Odds, item.Prob*100, item.Stake, item.Stake*item.Odds)
		}
	}
	if !hasAggParlay {
		md += "- 无\n"
	}
	md += "\n---\n\n## 四、 方案三：综合混合过关方案 (总投入：10 元)\n\n自动在 5 种玩法中选出模型预测概率最高的且可售的黄金选项，将前 4 场进行串关联投注。\n\n### 1. 投注详情\n"
	if len(mixedScheme) > 0 {
		md += fmt.Sprintf("- **过关方式**：%s\n- **综合概率**：%.2f%%\n- **组合总赔率**：%.2f\n- **推荐投注**：10 元（5倍）\n- **最高收益 (全中)**：**%.2f 元**\n- **最低收益**：**0.00 元**\n", betTypeStr, mixedProb*100, mixedOdds, 10.0*mixedOdds)
	} else {
		md += "- 无在售可用比赛，无法组合\n"
	}
	md += "\n### 2. 串关明细\n"
	if len(mixedScheme) > 0 {
		md += "| 赛事对阵 | 推荐玩法 | 推荐投注选项 | 选项赔率 | 单场预测中奖率 |\n| :--- | :--- | :--- | :--- | :--- |\n"
		for _, item := range mixedScheme {
			singleProb := 0.0
			for _, c := range result.SafeScheme {
				if c.MatchName == item.MatchName && c.Selection == item.Selection {
					singleProb = c.Prob
					break
				}
			}
			if singleProb == 0.0 {
				for _, c := range result.AggressiveScheme {
					if c.MatchName == item.MatchName && c.Selection == item.Selection {
						singleProb = c.Prob
						break
					}
				}
			}
			md += fmt.Sprintf("| %s | %s | %s | %.2f | %.1f%% |\n", item.MatchName, item.Market, item.Selection, item.Odds, singleProb*100)
		}
	} else {
		md += "- 无\n"
	}
	md += "\n---\n\n## 🤖 多 Agent 智能决策研判\n\n"
	md += fmt.Sprintf("- **常规立论 (35B)**：%s\n\n- **魔鬼反驳 (8B)**：%s\n\n- **裁决共识 (35B)**：%s\n\n预期综合回报率 (ROI)：**%.1f%%**\n", result.ProponentOpinion, result.CritiqueAnalysis, result.ConsensusReason, result.ExpectedROI*100)
	return md
}
