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
		model = "qwen3.6:35b-q4"
	}

	predictTimeout := 30 * time.Second
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
		client:         &http.Client{Timeout: 65 * time.Second},
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

// extractJSON 辅助提取字符串中的第一个闭合的 JSON 结构
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return s
	}

	// 匹配大括号深度以获得首个闭合的完整 JSON
	depth := 0
	for i := start; i < len(s); i++ {
		if s[i] == '{' {
			depth++
		} else if s[i] == '}' {
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}

	// 降级保护：直接截取到第一个出现的 }
	end := strings.Index(s[start:], "}")
	if end != -1 {
		return s[start : start+end+1]
	}
	return s
}

// rewriteFuzzyQuery 拦截并重写模糊的天气和场馆检索 Query
func (s *OllamaService) rewriteFuzzyQuery(toolCallJSON string, match models.Match) string {
	var call struct {
		Tool  string `json:"tool"`
		Query string `json:"query"`
	}
	if err := json.Unmarshal([]byte(toolCallJSON), &call); err != nil {
		return toolCallJSON
	}

	if call.Tool == "web_search" {
		qLower := strings.ToLower(call.Query)
		// 拦截各种模糊比赛地、当前天气等 Query 并注入物理 Venue 以确保检索精确性
		if strings.Contains(qLower, "目前这场比赛") ||
			strings.Contains(qLower, "当前比赛") ||
			strings.Contains(qLower, "比赛地") ||
			strings.Contains(qLower, "这场比赛的天气") ||
			strings.Contains(qLower, "今天的天气") ||
			strings.Contains(qLower, "现在的天气") ||
			qLower == "目前这场比赛的比赛地天气如何？" ||
			qLower == "目前这场比赛的比赛地天气如何" {
			call.Query = fmt.Sprintf("%s weather forecast", match.Venue)
			newJSON, err := json.Marshal(call)
			if err == nil {
				return string(newJSON)
			}
		}
	}
	return toolCallJSON
}

// cleanBoringPrefix 强力清洗诊断、分类及调度说明废话，保证直切主题
func cleanBoringPrefix(reply string) string {
	lines := strings.Split(reply, "\n")
	var keptLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 排除大模型自我解析的冗余废话
		if strings.Contains(trimmed, "类型B") ||
			strings.Contains(trimmed, "类型A") ||
			strings.Contains(trimmed, "常规咨询与理论分析") ||
			strings.Contains(trimmed, "不需要基于【已勾选比赛列表】进行") ||
			strings.Contains(trimmed, "不需要基于已勾选比赛") ||
			strings.Contains(trimmed, "根据我的首轮判定") ||
			strings.Contains(trimmed, "常规咨询") ||
			strings.Contains(trimmed, "首轮调度") ||
			strings.Contains(trimmed, "意图判定") ||
			strings.Contains(trimmed, "属于“常规咨询”") ||
			strings.Contains(trimmed, "分类为") {
			continue
		}
		keptLines = append(keptLines, line)
	}
	return strings.TrimSpace(strings.Join(keptLines, "\n"))
}

// ChatAgentDispatcher 执行首轮 AI 意图判定。
// 返回 (最终答复文本, 工具调用JSON, 错误)
func (s *OllamaService) ChatAgentDispatcher(ctx context.Context, match models.Match, userMessage string, predictionsJSON string, checkedMatchesCtx string, history []models.ChatMessage) (string, string, error) {
	currentTime := time.Now().Format("2006-01-02 15:04:05 MST")
	prompt := fmt.Sprintf(`你是全能智能决策与通用精算助手。
【当前系统本地时间】：%s

【最高优先级：场景隔离法则】：
你必须首先识别用户的提问是否完全与当前比赛、世界杯及足球/体育博彩无关（如：常规科学常识、全球天气、技术编程、普通闲聊或数学计算等）：
1. 若为完全无关的非足球常规问题：
   - 你必须立刻自适应地扮演专业的“通用智能 AI 助理”进行答复。
   - 此时，你必须完全忽略并抛弃下文提供的“当前比赛”、“量化预测数据”、“已勾选比赛列表”等所有足球/世界杯相关的上下文，不得受其干扰。
   - 你的回答或工具调用中绝对禁止包含任何足球、比赛对阵、博彩赔率、期望值 EV、泊松模型等词汇，防止强套足球概念。
   - 对于通用的数学计算、简单闲聊或内置常识/编程代码（如“写一个快速排序的 Python 函数”），必须直接在首轮内置解答，禁止调用任何搜索工具。对于需要外部实时事实的常规问题（如“明天东京天气”），生成 web_search 工具调用，检索词只含核心检索词（如 {"tool": "web_search", "query": "Tokyo weather"}），禁止带有足球废话。
2. 若为足球、体育或博彩相关问题：
   - 扮演量化精算专家，结合比赛和预测数据进行解答。
   - 仅当明确咨询过关投注实单推荐时，才必须基于【已勾选比赛列表】进行精算和推荐。若未勾选，礼貌提示。
   - 2026世界杯本届赛程/完赛比分等查询，必须且只能优先调用 'local_search' 查询本地，防范日期时区污染。

当前咨询的比赛（仅在未勾选任何比赛时作为兜底或用于常规基本面咨询）：%s %s VS %s，举办场馆 venue：%s。
该比赛当前的量化预测数据如下：
%s

【已勾选比赛列表】（如有数据且用户询问具体购彩实单推荐时，请以此列表为核心设计投注组合）：
%s

可用工具列表：
1. {"tool": "web_search", "query": "搜索引擎检索关键词"} ：用于查询外部最新的实时新闻、突发事件、常规实时气象 facts。
2. {"tool": "local_search", "query": "本地匹配词"} ：用于模糊查询本地历史比赛记录、赛程与比分。

任务：
根据用户提出的问题，判断是否需要调用上述工具以获取确定数据：
- 只要本地数据不足以回答，且该问题不是基础常识、基础自我介绍、编程代码、简单闲聊或数学逻辑计算，必须立刻调用工具，绝对禁止硬答或拒绝回答。
- 你每次只允许输出一个大括号包裹的工具 JSON 调用，绝对禁止输出多个大括号或并行多个工具！
- 如果不需要工具（即你拥有十足的把握直接用理论或内置常识直接作答），则直接以专业、严谨的中文给出深入解答（不超过150字，以 Markdown 格式呈现）。

用户提出的问题是：
"%s"
/no_think
`, currentTime, match.TournamentID, match.HomeTeam, match.AwayTeam, match.Venue, predictionsJSON, checkedMatchesCtx, userMessage)

	var messages []map[string]string
	for _, msg := range history {
		role := msg.Role
		if role == "ai" || role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		messages = append(messages, map[string]string{
			"role":    role,
			"content": msg.Content,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": prompt,
	})

	payload := map[string]interface{}{
		"model":    s.model,
		"messages": messages,
		"stream":   false,
		"think":    false,
		"options": map[string]interface{}{
			"temperature": 0.1,
			"num_predict": 300,
		},
		"keep_alive": -1,
	}

	bytesPayload, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	subCtx, cancel := context.WithTimeout(ctx, s.predictTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(subCtx, "POST", s.apiURL, bytes.NewBuffer(bytesPayload))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("Ollama 首轮调度连接超时: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Ollama 状态码异常: %d", resp.StatusCode)
	}

	var rawRes struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawRes); err != nil {
		return "", "", err
	}

	content := strings.TrimSpace(rawRes.Message.Content)
	if content == "" {
		return "", "", fmt.Errorf("Ollama 返回了空内容")
	}

	// 检查是否输出了符合规范的 JSON 工具调用
	if strings.Contains(content, `"tool":`) {
		cleaned := extractJSON(content)
		cleaned = s.rewriteFuzzyQuery(cleaned, match)
		return "", cleaned, nil
	}

	// 兼容处理：如果模型漏掉了大括号或者直接输出了裸的 local_search / web_search 文本
	if strings.Contains(content, "local_search") || strings.Contains(content, "web_search") {
		toolName := "local_search"
		if strings.Contains(content, "web_search") {
			toolName = "web_search"
		}
		// 自动从用户原始提问中过滤掉标点等无用词作为 Query
		query := userMessage
		cleaned := fmt.Sprintf(`{"tool": "%s", "query": "%s"}`, toolName, query)
		cleaned = s.rewriteFuzzyQuery(cleaned, match)
		return "", cleaned, nil
	}

	return cleanBoringPrefix(content), "", nil
}

// ChatWithObservation 接收工具执行的 Observation 观测结果，请求大模型生成对用户的最终问答。
func (s *OllamaService) ChatWithObservation(ctx context.Context, match models.Match, userMessage string, predictionsJSON string, checkedMatchesCtx string, toolName string, observation string, history []models.ChatMessage) (string, error) {
	currentTime := time.Now().Format("2006-01-02 15:04:05 MST")
	prompt := fmt.Sprintf(`你是全能决策与足球量化智能助手。
【当前系统本地时间】：%s

【最高优先级：场景隔离法则】：
你必须首先识别用户的提问是否完全与当前比赛、世界杯及足球/体育博彩无关（如：常规科学常识、全球天气、技术编程、普通闲聊或数学计算等）：
1. 若为完全无关的非足球常规问题：
   - 你必须立刻自适应地扮演专业的“通用智能 AI 助理”进行答复。
   - 此时，你必须完全忽略并抛弃下文提供的“当前比赛”、“量化预测数据”、“已勾选比赛列表”等所有足球/世界杯相关的上下文，不得受其干扰。
   - 你的回答绝对禁止包含任何足球、比赛对阵、博彩赔率、期望值 EV、泊松模型等词汇，防止强套足球概念。必须直接、正面、仅针对检索到的 Observation 事实或内置常识给出解答。
2. 若为足球、体育或博彩相关问题：
   - 结合量化精算专家角色，利用 Observation 中的事实和预测数据进行解答。
   - 仅当明确咨询具体购彩实单推荐时，才必须基于【已勾选比赛列表】进行精算和推荐。
   - 在涉及球星国籍与国家队大名单时必须保持绝对的专业风控，确保球员国籍一致，绝不乌龙。
   - 根据本地系统时间精准换算“明天/今天/后天”的开赛日程。

当前咨询的比赛（仅在未勾选任何比赛时作为兜底或用于常规基本面咨询）：%s %s VS %s。
该比赛当前的量化预测数据如下：
%s

【已勾选比赛列表】：
%s

我们通过执行 '%s' 工具，在全网/本地检索到的 Observation（观测事实数据）是：
%s

用户提出的原始问题是：
"%s"

要求：
- 请基于上述检索到的 Observation 事实数据，给出深入、客观的中文回答。
- 字数保持在200字以内。
- 对于常规非投注/非体育问题（如天气、球员名单、科学常识等），必须直接给出准确的、贴合 Observation 事实的直接解答，切勿生搬硬套投注/赔率/当前比赛分析，更严禁以任何理由拒绝回答！
- 禁止编造任何虚假的数据，严格基于工具返回的结果进行提炼。
/no_think
`, currentTime, match.TournamentID, match.HomeTeam, match.AwayTeam, predictionsJSON, checkedMatchesCtx, toolName, observation, userMessage)

	var messages []map[string]string
	for _, msg := range history {
		role := msg.Role
		if role == "ai" || role == "assistant" {
			role = "assistant"
		} else {
			role = "user"
		}
		messages = append(messages, map[string]string{
			"role":    role,
			"content": msg.Content,
		})
	}
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": prompt,
	})

	payload := map[string]interface{}{
		"model":    s.model,
		"messages": messages,
		"stream":   false,
		"think":    false,
		"options": map[string]interface{}{
			"temperature": 0.3,
			"num_predict": 400,
		},
		"keep_alive": -1,
	}

	bytesPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	subCtx, cancel := context.WithTimeout(ctx, s.reviewTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(subCtx, "POST", s.apiURL, bytes.NewBuffer(bytesPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("Ollama 二次生成连接超时: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama 状态码异常: %d", resp.StatusCode)
	}

	var rawRes struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawRes); err != nil {
		return "", err
	}

	return cleanBoringPrefix(strings.TrimSpace(rawRes.Message.Content)), nil
}
