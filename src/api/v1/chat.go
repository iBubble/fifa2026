package v1

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/ai"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ChatAgent 智能对话接口，支持大模型多轮意图调度与外网/本地工具观察融合
func (ctrl *APIController) ChatAgent(c *gin.Context) {
	var req struct {
		MatchID         string               `json:"matchId"`
		Message         string               `json:"message"`
		Predictions     interface{}          `json:"predictions"`
		CheckedMatchIDs []string             `json:"checkedMatchIds"`
		History         []models.ChatMessage `json:"history"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	match, err := db.GetMatch(req.MatchID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "未找到指定比赛"})
		return
	}

	predictionsBytes, _ := json.Marshal(req.Predictions)
	predictionsStr := string(predictionsBytes)

	var sbChecked strings.Builder
	if len(req.CheckedMatchIDs) > 0 {
		sbChecked.WriteString("已勾选比赛列表:\n")
		for _, cmID := range req.CheckedMatchIDs {
			cm, errCm := db.GetMatch(cmID)
			if errCm == nil {
				cOdds := ctrl.SportteryService.GetMatchOdds(cm.HomeTeam, cm.AwayTeam, cm.ScheduledAt)
				predStr := "暂无预测数据"
				if rep, errRep := db.GetPredictionReport(cmID); errRep == nil && len(rep.ScoreMatrix) > 0 {
					var homeP, drawP, awayP float64
					for _, cell := range rep.ScoreMatrix {
						if cell.HomeScore > cell.AwayScore {
							homeP += cell.Prob
						} else if cell.HomeScore == cell.AwayScore {
							drawP += cell.Prob
						} else {
							awayP += cell.Prob
						}
					}
					sumP := homeP + drawP + awayP
					if sumP > 0 {
						predStr = fmt.Sprintf("主胜概率:%.1f%%, 平局概率:%.1f%%, 客胜概率:%.1f%%",
							homeP/sumP*100, drawP/sumP*100, awayP/sumP*100)
					}
				}
				sbChecked.WriteString(fmt.Sprintf("- 比赛: %s vs %s, 状态: %s, 竞彩赔率: 主胜%.2f/平局%.2f/客胜%.2f, 泊松预测: %s\n",
					cm.HomeTeam, cm.AwayTeam, cm.Status, cOdds.HomeOdds, cOdds.DrawOdds, cOdds.AwayOdds, predStr))
			}
		}
	} else {
		sbChecked.WriteString("暂无勾选比赛")
	}
	checkedMatchesCtx := sbChecked.String()

	reply, toolCallJSON, err := ctrl.OllamaService.ChatAgentDispatcher(c.Request.Context(), match, req.Message, predictionsStr, checkedMatchesCtx, req.History)
	if err != nil {
		log.Printf("[Chat] ⚠️ AI 智能体首轮调度失败: %v", err)
		offlineReply := buildOfflineFallbackReply(match, req.Message)
		c.JSON(http.StatusOK, gin.H{"reply": offlineReply})
		return
	}

	if toolCallJSON != "" {
		var call struct {
			Tool  string `json:"tool"`
			Query string `json:"query"`
		}
		if errJson := json.Unmarshal([]byte(toolCallJSON), &call); errJson == nil {
			observation := ""
			log.Printf("[ChatAgent] 🚀 命中工具调用: Tool=%s, Query=%s", call.Tool, call.Query)

			isWebSearch := false
			if call.Tool == "web_search" {
				isWebSearch = true
			}

			switch call.Tool {
			case "web_search":
				obs, errSearch := ai.WebSearch(call.Query)
				if errSearch == nil {
					observation = obs
				} else {
					observation = "全网搜索失败: " + errSearch.Error()
				}
			case "local_search":
				isStandingsQuery := false
				qLower := strings.ToLower(call.Query)
				if strings.Contains(qLower, "积分") ||
					strings.Contains(qLower, "排名") ||
					strings.Contains(qLower, "小组") ||
					strings.Contains(qLower, "出线") ||
					strings.Contains(qLower, "夺冠") ||
					strings.Contains(qLower, "standing") ||
					strings.Contains(qLower, "group") ||
					strings.Contains(qLower, "rank") {
					isStandingsQuery = true
				}

				var obsList []string
				if isStandingsQuery {
					standingsObs := buildGroupStandingsObservation()
					obsList = append(obsList, standingsObs)

					mcObs := buildMonteCarloObservation(call.Query)
					obsList = append(obsList, mcObs)
				}

				dbObsList, errLocal := db.FuzzySearchLocalData(call.Query)
				if errLocal == nil {
					obsList = append(obsList, dbObsList...)
				} else {
					obsList = append(obsList, "本地模糊搜索失败: "+errLocal.Error())
				}
				observation = strings.Join(obsList, "\n")
			default:
				observation = "未识别的工具名称"
			}

			finalReply, errObs := ctrl.OllamaService.ChatWithObservation(c.Request.Context(), match, req.Message, predictionsStr, checkedMatchesCtx, call.Tool, observation, req.History)
			if errObs != nil {
				log.Printf("[Chat] ⚠️ AI 智能体二次生成失败: %v", errObs)
				finalReply = fmt.Sprintf("> 🧠 **FIFA 2026 离线智能决策引擎已激活**：由于深度分析阶段超时，已自动切换至本地排版展示检索事实。\n\n**【最新事实检索数据】**：\n%s\n\n*(当前大模型负载较高，以上为已为您自动对齐事实并呈现的 Observation 数据，供您参考决策。)*", observation)
			} else {
				if isWebSearch {
					finalReply = fmt.Sprintf("> 🧠 **FIFA 2026 智能决策引擎思考中**...\n> 🔍 **全网事实检索**：已针对 “%s” 进行全网实时搜索以校准事实数据。\n\n%s", call.Query, finalReply)
				} else if call.Tool == "local_search" {
					finalReply = fmt.Sprintf("> 🧠 **FIFA 2026 智能决策引擎思考中**...\n> 📂 **本地数据检索**：已针对 “%s” 检索本地历史数据库以校准偏置事实。\n\n%s", call.Query, finalReply)
				}
			}
			reply = finalReply
		} else {
			log.Printf("[Chat] ⚠️ 无法解析大模型工具JSON: %s", toolCallJSON)
		}
	}

	c.JSON(http.StatusOK, gin.H{"reply": reply})
}
