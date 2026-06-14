package ai

import (
	"context"
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestAgentQuestions(t *testing.T) {
	// 1. 初始化本地数据库，确保模糊搜索有数据支持
	dbPath := "/Users/gemini/Projects/Own/FIFA2026/data"
	err := db.Init(dbPath)
	if err != nil {
		t.Fatalf("初始化数据库失败: %v", err)
	}
	defer db.Close()

	// 导入测试冷启动数据以防测试用空库
	seasonsJSON := "/Users/gemini/Projects/Own/FIFA2026/data/seasons/fifa_2026.json"
	featuresJSON := "/Users/gemini/Projects/Own/FIFA2026/data/seasons/history_features.json"
	_ = db.ImportInitialData(seasonsJSON, featuresJSON)

	// 2. 创建 Ollama 智能决策助手
	// 本地开发机上直接使用 11434 端口
	s := NewOllamaService("http://127.0.0.1:11434", "qwen3.6:35b-q4")

	// 3. 构建当前比赛上下文 (Mock德国对其他国家的比赛，作为兜底上下文)
	match := models.Match{
		ID:           "test_match_eval",
		TournamentID: "fifa_2026",
		HomeTeam:     "Germany",
		AwayTeam:     "Costa Rica",
		Group:        "E",
		ScheduledAt:  time.Now().Add(24 * time.Hour),
		Status:       "NS",
		Venue:        "Lumen Field Seattle",
	}

	predictionsJSON := `{"lambdaHome": 2.15, "lambdaAway": 0.85, "rho": -0.05}`
	checkedMatchesCtx := "暂无勾选比赛"

	// 4. 用户需要测试的 5 个问题 + 自定义设计的问题
	questions := []string{
		"于让3球的这种购彩规则，足以从各个维度证明两队实力差距的情况下，为什么连德国进球5以上都不敢猜",
		"基于本届世界杯的对战情况，什么情况下才能有分差4球以上的情况出现",
		"目前这场比赛的比赛地天气如何？",
		"昆明 and 巴黎目前的天气情况如何？",
		"对明天西班牙的比赛做个大胆的预测",
		"本届世界杯如果要在美国 Lumen Field 举办比赛，该场馆的海拔和草坪材质是什么？",
		"效力于皇家马德里的超级球星基利安·姆巴佩在本次对话中，能否正确代表法国国家队，而不会被错写为西班牙队？",
		"请用大白话解释一下什么是量子纠缠？用最通俗易懂的语言解释。",
		"请用 Python 编写一个二分查找的完整函数实现，并加上简单的注释。",
		"明天纽约的天气情况如何？",
	}

	fmt.Println("\n====== AI 智能决策助手问答测评开始 ======")
	for i, q := range questions {
		fmt.Printf("\n--- 问题 %d: %s ---\n", i+1, q)

		// 运行 Dispatcher 第一阶段
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		reply, toolCallJSON, err := s.ChatAgentDispatcher(ctx, match, q, predictionsJSON, checkedMatchesCtx, nil)
		cancel()

		if err != nil {
			fmt.Printf("[Dispatcher Error]: %v\n", err)
			continue
		}

		if toolCallJSON != "" {
			fmt.Printf("[Dispatcher 判定命中工具]: %s\n", toolCallJSON)

			// 解析工具调用
			var call struct {
				Tool  string `json:"tool"`
				Query string `json:"query"`
			}
			if errJson := json.Unmarshal([]byte(toolCallJSON), &call); errJson == nil {
				observation := ""
				switch call.Tool {
				case "web_search":
					fmt.Printf("[执行全网搜索]: Query = %s\n", call.Query)
					obs, errSearch := WebSearch(call.Query)
					if errSearch == nil {
						observation = obs
						fmt.Printf("[搜索结果缩略]: %s\n", limitString(obs, 200))
					} else {
						observation = "全网搜索失败: " + errSearch.Error()
						fmt.Printf("[搜索失败]: %v\n", errSearch)
					}
				case "local_search":
					fmt.Printf("[执行本地搜索]: Query = %s\n", call.Query)
					obsList, errLocal := db.FuzzySearchLocalData(call.Query)
					if errLocal == nil {
						observation = strings.Join(obsList, "\n")
						fmt.Printf("[本地结果缩略]: %s\n", limitString(observation, 200))
					} else {
						observation = "本地搜索失败: " + errLocal.Error()
						fmt.Printf("[本地搜索失败]: %v\n", errLocal)
					}
				default:
					observation = "未识别的工具名称"
				}

				// 第二阶段：Observation 喂回
				ctxObs, cancelObs := context.WithTimeout(context.Background(), 80*time.Second)
				finalReply, errObs := s.ChatWithObservation(ctxObs, match, q, predictionsJSON, checkedMatchesCtx, call.Tool, observation, nil)
				cancelObs()

				if errObs != nil {
					fmt.Printf("[二次生成失败]: %v\n", errObs)
				} else {
					fmt.Printf("[最终 AI 回答]:\n%s\n", finalReply)
				}

			} else {
				fmt.Printf("[工具JSON解析失败]: %s\n", toolCallJSON)
			}
		} else {
			fmt.Printf("[Dispatcher 直接回答]:\n%s\n", reply)
		}
	}
	fmt.Println("\n====== AI 智能决策助手问答测评结束 ======")
}

func limitString(s string, limit int) string {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit]) + "..."
	}
	return s
}
