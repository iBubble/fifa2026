package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fifa2026/src/internal/models"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type OllamaService struct {
	client         *http.Client
	apiURL         string
	model          string
	predictTimeout time.Duration
	reviewTimeout  time.Duration
}

func NewOllamaService(apiURL, model string) *OllamaService {
	if apiURL == "" {
		apiURL = "http://127.0.0.1:11434/v1/chat/completions"
	} else {
		if !strings.Contains(apiURL, "/v1/") && !strings.Contains(apiURL, "/api/") {
			apiURL = strings.TrimSuffix(apiURL, "/")
			apiURL = apiURL + "/v1/chat/completions"
		}
	}
	if model == "" {
		model = "qwen2.5"
	}

	predictTimeout := 15 * time.Second
	if envPredict := os.Getenv("OLLAMA_PREDICT_TIMEOUT"); envPredict != "" {
		if d, err := strconv.Atoi(envPredict); err == nil && d > 0 {
			predictTimeout = time.Duration(d) * time.Second
		}
	}

	reviewTimeout := 60 * time.Second
	if envReview := os.Getenv("OLLAMA_REVIEW_TIMEOUT"); envReview != "" {
		if d, err := strconv.Atoi(envReview); err == nil && d > 0 {
			reviewTimeout = time.Duration(d) * time.Second
		}
	}

	return &OllamaService{
		client:         &http.Client{Timeout: 90 * time.Second}, // 全局安全兜底超时，具体超时由 Context 控制
		apiURL:         apiURL,
		model:          model,
		predictTimeout: predictTimeout,
		reviewTimeout:  reviewTimeout,
	}
}

// RefineParams 交叉推理：接收定量泊松参数与定性因子，请求本地大模型输出修正偏置 JSON
func (s *OllamaService) RefineParams(match models.Match, eloDiff float64, p models.DixonColesParams, info string) (models.LLMRefineOffsets, error) {
	prompt := fmt.Sprintf(`
你是一位顶尖的足球量化模型分析与大模型策略专家。请分析本场比赛的基本面，并对定量 Dixon-Coles 泊松模型的进球期望值（lambda）及平局系数（rho）进行定性修正偏置输出。

【比赛基本面】
- 赛事: %s
- 对决: %s (主队) VS %s (客队)
- 历史实力 Elo 差值: %.2f (主队 - 客队)
- 定量 Dixon-Coles 参数: 主队λ=%.3f, 客队λ=%.3f, 平局系数ρ=%.3f

【定性情报 (伤停/战意/天气)】
%s

请分析上述输入（如核心伤停导致攻击力下降，天气影响传控等），并微调 lambda 和 rho 参数。你必须严格输出如下 JSON 格式的修正偏置量，禁止带有任何其他 markdown 包装或解释：
{
  "lambdaHomeOffset": [主队λ偏移量, 范围-0.5到0.5, 浮点数],
  "lambdaAwayOffset": [客队λ偏移量, 范围-0.5到0.5, 浮点数],
  "rhoOffset": [平局因子偏移量, 范围-0.05到0.05, 浮点数],
  "tacticsAnalysis": "[简明扼要的中文战术定性分析报告，限80字内]",
  "posterPrompt": "[为本场对决生成的 Midjourney 海报英文提示词，如: A cinematic football poster of Mexico vs Ecuador, neon violet and green glows, dynamic action, 8k]"
}
`, match.TournamentID, match.HomeTeam, match.AwayTeam, eloDiff, p.LambdaHome, p.LambdaAway, p.Rho, info)

	payload := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]string{"type": "json_object"}, // 强约束 JSON 格式
		"temperature":     0.2,
	}

	bytesPayload, err := json.Marshal(payload)
	if err != nil {
		return models.LLMRefineOffsets{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.predictTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewBuffer(bytesPayload))
	if err != nil {
		return models.LLMRefineOffsets{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return models.LLMRefineOffsets{}, fmt.Errorf("Ollama 连接超时或挂起: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.LLMRefineOffsets{}, fmt.Errorf("API 响应错误状态码: %d", resp.StatusCode)
	}

	var rawRes struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawRes); err != nil {
		return models.LLMRefineOffsets{}, err
	}

	if len(rawRes.Choices) == 0 {
		return models.LLMRefineOffsets{}, fmt.Errorf("Ollama 返回了空选择")
	}

	var offsets models.LLMRefineOffsets
	if err := json.Unmarshal([]byte(rawRes.Choices[0].Message.Content), &offsets); err != nil {
		return models.LLMRefineOffsets{}, fmt.Errorf("解析 Ollama 偏置 JSON 失败: %w", err)
	}

	return offsets, nil
}

// ReviewPrediction 对过去的预测做出反思，输出简短的中文赛后精算纠偏心得
func (s *OllamaService) ReviewPrediction(match models.Match, brierScore float64, priorTactics string, homeScore, awayScore int) (string, error) {
	prompt := fmt.Sprintf(`
你是一位足球赔率精算与量化大模型专家。本场比赛已经结束，请对比我们先前的预测和实际赛果，总结预测误差的定性原因，为以后的参数微调提供修正建议。

【比赛数据】
- 赛事: %s
- 对阵: %s VS %s
- 实际赛果: %d : %d
- 先前预测的 Brier Score (胜平负联合概率布莱尔得分，越接近0越精确): %.4f
- 先前的大模型战术分析预估: "%s"

请用 80 字以内的中文，简明扼要地生成一份赛后纠偏心得（例如分析是否高估了某队攻击力或低估了防守战意），对之后的 Dixon-Coles 建模或 LLM 修正提供实质性指引。
`, match.TournamentID, match.HomeTeam, match.AwayTeam, homeScore, awayScore, brierScore, priorTactics)

	payload := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
	}

	bytesPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.reviewTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewBuffer(bytesPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "Ollama 超时降级: 无法获取赛后反思文本", nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "Ollama 服务响应异常，生成失败", nil
	}

	var rawRes struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawRes); err != nil {
		return "", err
	}

	if len(rawRes.Choices) == 0 {
		return "未生成反思心得", nil
	}

	return rawRes.Choices[0].Message.Content, nil
}

