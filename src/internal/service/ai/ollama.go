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
		apiURL = "http://127.0.0.1:11434/api/chat"
	} else {
		if !strings.Contains(apiURL, "/v1/") && !strings.Contains(apiURL, "/api/") {
			apiURL = strings.TrimSuffix(apiURL, "/")
			apiURL = apiURL + "/api/chat"
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
		client:         &http.Client{Timeout: 180 * time.Second},
		apiURL:         apiURL,
		model:          model,
		predictTimeout: predictTimeout,
		reviewTimeout:  reviewTimeout,
	}
}

// RefineParams 交叉推理：接收定量泊松参数与定性因子，请求本地大模型输出修正偏置 JSON
func (s *OllamaService) RefineParams(match models.Match, eloDiff float64, p models.DixonColesParams, info string) (models.LLMRefineOffsets, error) {
	prompt := fmt.Sprintf(`足球赔率精算专家，执行三阶段CoT辩论(立论->反驳->仲裁)，输出Dixon-Coles参数修正。

重要：本届世界杯东道主仅为USA/Canada/Mexico三国！除这三队外，所有队伍都无任何主场优势！"主队""客队"仅为赛程排位，不代表物理主客场！

规则:
- 仅当USA/Canada/Mexico本土作战时: lambdaOffset可+0.08~+0.15
- 核心伤停/被高估: lambdaOffset必须-0.15~-0.05
- 防守平局/冷门: rhoOffset必须-0.08~-0.04
- 严禁全零调整
- 非东道主三国的队伍禁止给予任何主场优势偏置

数据: 赛事:%s 场地:%s
%s(排位主)VS %s(排位客) Elo差:%.2f
主队L=%.3f 客队L=%.3f rho=%.3f

情报:%s

严格输出JSON无markdown:
{"lambdaHomeOffset":0.0,"lambdaAwayOffset":0.0,"rhoOffset":0.0,"tacticsAnalysis":"60字中文","posterPrompt":"english poster prompt","proponentOpinion":"60字中文","critiqueAnalysis":"60字中文","consensusReason":"60字中文"}
/no_think
`, match.TournamentID, match.Venue, match.HomeTeam, match.AwayTeam, eloDiff, p.LambdaHome, p.LambdaAway, p.Rho, info)

	payload := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
		"think":  false,
		"options": map[string]interface{}{
			"temperature": 0.1,
			"num_ctx":     2048,
			"num_predict": 400,
		},
		"keep_alive": -1,
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
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawRes); err != nil {
		return models.LLMRefineOffsets{}, err
	}

	if rawRes.Message.Content == "" {
		return models.LLMRefineOffsets{}, fmt.Errorf("Ollama 返回了空内容")
	}

	var offsets models.LLMRefineOffsets
	cleanedJSON := extractJSON(rawRes.Message.Content)
	if err := json.Unmarshal([]byte(cleanedJSON), &offsets); err != nil {
		return models.LLMRefineOffsets{}, fmt.Errorf("解析偏置JSON失败(原始:%s): %w", rawRes.Message.Content, err)
	}

	return offsets, nil
}

// ReviewPrediction 对过去的预测做出反思，输出简短的中文赛后精算纠偏心得
func (s *OllamaService) ReviewPrediction(match models.Match, brierScore float64, priorTactics string, homeScore, awayScore int) (string, error) {
	prompt := fmt.Sprintf(`赛后纠偏专家。赛事:%s %s VS %s 赛果:%d:%d BS:%.4f 先前分析:"%s"
用60字中文总结预测误差原因和修正建议。
/no_think
`, match.TournamentID, match.HomeTeam, match.AwayTeam, homeScore, awayScore, brierScore, priorTactics)

	payload := map[string]interface{}{
		"model": s.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
		"think":  false,
		"options": map[string]interface{}{
			"temperature": 0,
			"num_predict": 128,
		},
		"keep_alive": -1,
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
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawRes); err != nil {
		return "", err
	}

	if rawRes.Message.Content == "" {
		return "未生成反思心得", nil
	}

	return rawRes.Message.Content, nil
}

// extractJSON 辅助提取字符串中的第一个 { 到最后一个 }
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start == -1 || end == -1 || start >= end {
		return s
	}
	return s[start : end+1]
}
