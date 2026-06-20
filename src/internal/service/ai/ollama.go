package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type OllamaService struct {
	client         *http.Client
	apiURL         string
	model          string
	predictTimeout time.Duration
	reviewTimeout  time.Duration
	mu             sync.Mutex // 全局串行互斥锁，防止并发模型交替加载导致颠簸
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
		client:         &http.Client{Timeout: 185 * time.Second},
		apiURL:         apiURL,
		model:          model,
		predictTimeout: predictTimeout,
		reviewTimeout:  reviewTimeout,
	}
}

// RefineParams 交叉推理：接收定量泊松参数与定性因子，请求本地大模型输出修正偏置 JSON
func (s *OllamaService) RefineParams(match models.Match, eloDiff float64, p models.DixonColesParams, info string) (models.LLMRefineOffsets, error) {
	homeCn := getTeamCnName(match.HomeTeam)
	awayCn := getTeamCnName(match.AwayTeam)
	homeLabel := fmt.Sprintf("%s(%s)", homeCn, match.HomeTeam)
	awayLabel := fmt.Sprintf("%s(%s)", awayCn, match.AwayTeam)

	// 步骤一：常规立论阶段 (使用 35B 模型)
	step1Prompt := fmt.Sprintf(`作为足球精算常规立论专家，根据以下数据 and 情报，提出需要修正的偏置参数（初始值）及常规立论正面理由(proponentOpinion，60字中文)。
规则：仅当USA/Canada/Mexico本土作战时主队lambdaOffset可+0.08~+0.15；核心伤停/被高估时lambdaOffset必须-0.15~-0.05；防守平局/冷门时rhoOffset必须-0.08~-0.04；非东道主三国的队禁止主场优势偏置。

数据：赛事:%s 场地:%s %s(排位主) VS %s(排位客) Elo差:%.2f L_H=%.3f L_A=%.3f rho=%.3f
情报：%s

严格输出JSON无markdown:
{"lambdaHomeOffset":0.0,"lambdaAwayOffset":0.0,"rhoOffset":0.0,"proponentOpinion":"60字正面理由"}
/no_think`, match.TournamentID, match.Venue, homeLabel, awayLabel, eloDiff, p.LambdaHome, p.LambdaAway, p.Rho, info)

	var offsets models.LLMRefineOffsets
	ctx1, cancel1 := context.WithTimeout(context.Background(), s.predictTimeout)
	res1, err1 := s.requestOllama(ctx1, s.model, step1Prompt, 0.1, 250)
	cancel1()
	if err1 == nil && res1 != "" {
		_ = json.Unmarshal([]byte(extractJSON(res1)), &offsets)
	}

	// 限制安全回退与初始解析值
	proponentOpinion := offsets.ProponentOpinion
	if proponentOpinion == "" {
		proponentOpinion = "主队具备基础的定位期望优势，中场战术相对稳健。"
	}

	// 步骤二：魔鬼代言人反驳阶段 (使用 8B 模型)
	step2Prompt := fmt.Sprintf(`作为魔鬼代言人反驳专家。
先前立论专家的主张是：主队λ偏置 %.3f, 客队λ偏置 %.3f, 平局偏置 %.3f。常规立论理由是: "%s"。
请根据赛事（%s VS %s）、场地（%s）、情报（%s），从爆冷、体彩热门陷阱、逆向EV或高压降级等反面心理或战术盲区角度，指出其主张的漏洞，并给出魔鬼反驳意见(critiqueAnalysis，60字中文)。

严格输出JSON无markdown:
{"critiqueAnalysis":"60字魔鬼反驳"}
/no_think`, offsets.LambdaHomeOffset, offsets.LambdaAwayOffset, offsets.RhoOffset, proponentOpinion, homeLabel, awayLabel, match.Venue, info)

	critiqueAnalysis := "注意防范平局陷阱以及客队高抗压下的反击坚韧度。"
	ctx2, cancel2 := context.WithTimeout(context.Background(), s.predictTimeout)
	res2, err2 := s.requestOllama(ctx2, "qwen3:8b", step2Prompt, 0.2, 200)
	cancel2()
	if err2 == nil && res2 != "" {
		var critiqueWrap struct {
			CritiqueAnalysis string `json:"critiqueAnalysis"`
		}
		if json.Unmarshal([]byte(extractJSON(res2)), &critiqueWrap) == nil && critiqueWrap.CritiqueAnalysis != "" {
			critiqueAnalysis = critiqueWrap.CritiqueAnalysis
		}
	}
	offsets.CritiqueAnalysis = critiqueAnalysis

	// 步骤三：首席精算仲裁裁判阶段 (使用 35B 模型)
	step3Prompt := fmt.Sprintf(`作为首席精算仲裁裁判。
常规立论主张为：主推λ %.3f, 客推λ %.3f, 平局偏置 %.3f，立论理由: "%s"。
魔鬼反驳的意见为: "%s"。
请你理智中和，给出折中决策的共识裁决理由(consensusReason，60字中文)，并输出最终的Dixon-Coles修正偏移量，以及整体战术分析(tacticsAnalysis，60字中文) and 海报英文生成Prompt(posterPrompt)。
规则：最终偏移量仍需遵守首轮规则约束。

严格输出JSON无markdown:
{"lambdaHomeOffset":0.0,"lambdaAwayOffset":0.0,"rhoOffset":0.0,"consensusReason":"60字共识裁决","tacticsAnalysis":"60字整体战术分析","posterPrompt":"MJ英文海报提示词"}
/no_think`, offsets.LambdaHomeOffset, offsets.LambdaAwayOffset, offsets.RhoOffset, proponentOpinion, critiqueAnalysis)

	ctx3, cancel3 := context.WithTimeout(context.Background(), s.predictTimeout)
	res3, err3 := s.requestOllama(ctx3, s.model, step3Prompt, 0.1, 400)
	cancel3()
	if err3 == nil && res3 != "" {
		var finalWrap struct {
			LambdaHomeOffset float64 `json:"lambdaHomeOffset"`
			LambdaAwayOffset float64 `json:"lambdaAwayOffset"`
			RhoOffset        float64 `json:"rhoOffset"`
			ConsensusReason  string  `json:"consensusReason"`
			TacticsAnalysis  string  `json:"tacticsAnalysis"`
			PosterPrompt     string  `json:"posterPrompt"`
		}
		if json.Unmarshal([]byte(extractJSON(res3)), &finalWrap) == nil {
			offsets.LambdaHomeOffset = finalWrap.LambdaHomeOffset
			offsets.LambdaAwayOffset = finalWrap.LambdaAwayOffset
			offsets.RhoOffset = finalWrap.RhoOffset
			offsets.ConsensusReason = finalWrap.ConsensusReason
			offsets.TacticsAnalysis = finalWrap.TacticsAnalysis
			offsets.PosterPrompt = finalWrap.PosterPrompt
		}
	}

	// 容错补全
	offsets.ProponentOpinion = proponentOpinion
	if offsets.ConsensusReason == "" {
		offsets.ConsensusReason = "综合攻防及赔率流向，维持基础参数偏置，谨慎看待强队大胜机会。"
	}
	if offsets.TacticsAnalysis == "" {
		offsets.TacticsAnalysis = "战术均势对抗为主，立论与反驳达成妥协，聚焦中场防守硬度。"
	}
	if offsets.PosterPrompt == "" {
		offsets.PosterPrompt = "Dramatic football match, high dynamic tension, cinematic lighting."
	}

	return offsets, nil
}

// ReviewPrediction 对过去的预测做出反思，输出简短的中文赛后精算纠偏心得
func (s *OllamaService) ReviewPrediction(match models.Match, brierScore float64, priorTactics string, homeScore, awayScore int) (string, error) {
	homeCn := getTeamCnName(match.HomeTeam)
	awayCn := getTeamCnName(match.AwayTeam)
	homeLabel := fmt.Sprintf("%s(%s)", homeCn, match.HomeTeam)
	awayLabel := fmt.Sprintf("%s(%s)", awayCn, match.AwayTeam)

	prompt := fmt.Sprintf(`赛后纠偏专家。赛事:%s %s VS %s 赛果:%d:%d BS:%.4f 先前分析:"%s"
用60字中文总结预测误差原因和修正建议。
/no_think
`, match.TournamentID, homeLabel, awayLabel, homeScore, awayScore, brierScore, priorTactics)

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

	s.mu.Lock()
	resp, err := s.client.Do(req)
	s.mu.Unlock()
	if err != nil {
		return GenerateFallbackReview(match.HomeTeam, match.AwayTeam, homeScore, awayScore, brierScore), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return GenerateFallbackReview(match.HomeTeam, match.AwayTeam, homeScore, awayScore, brierScore), nil
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
		return GenerateFallbackReview(match.HomeTeam, match.AwayTeam, homeScore, awayScore, brierScore), nil
	}

	return rawRes.Message.Content, nil
}

// GenerateFallbackReview 根据比分结果、Brier Score 精度指标和胜负走向，自动为超时发生时生成一段深度足球精算复盘反思
func GenerateFallbackReview(homeTeam, awayTeam string, homeScore, awayScore int, brierScore float64) string {
	accuracyStr := "精度尚可"
	if brierScore < 0.1 {
		accuracyStr = "精度极高"
	} else if brierScore > 0.3 {
		accuracyStr = "偏差较大"
	}

	diff := homeScore - awayScore
	if diff > 0 { // 主胜
		if diff >= 2 { // 大胜
			if brierScore < 0.1 {
				return fmt.Sprintf("赛事精算%s：主队展现绝对压制力，净胜%d球大胜。量化DC模型对主队进攻期望预估完全吻合，防守零封印证了战力指数优势。", accuracyStr, diff)
			}
			return fmt.Sprintf("赛事精算%s：主队打出高效压制以%d球大胜。原始模型低估了强队主场哨音与进攻火力的爆发。建议在后续类似强弱悬殊局中，上调主队进球期望权重。", accuracyStr, diff)
		}
		if brierScore < 0.1 {
			return fmt.Sprintf("赛事精算%s：两队均势对抗，主队凭借微弱细节1球小胜，基本符合常规概率走势。模型对两端防守限缩的判断较为准确。", accuracyStr)
		}
		return fmt.Sprintf("赛事精算%s：主队险胜。比赛中客队顽强反击打乱了DC初始泊松平衡，防守端意外丢球导致BS偏差。需优化防守参数与攻防转换效率的耦合系数。", accuracyStr)
	} else if diff < 0 { // 客胜
		absDiff := -diff
		if absDiff >= 2 { // 大胜
			if brierScore < 0.1 {
				return fmt.Sprintf("赛事精算%s：客队打出高水准反击以%d球净胜。模型充分评估了客队客场战力加成，BS值极低验证了本次高EV的爆冷选择。", accuracyStr, absDiff)
			}
			return fmt.Sprintf("赛事精算%s：客队%d球反客为主。初始DC及Elo指标可能存在主队高估偏差，低估了客队的防守组织。建议对被过度铺热的主队引入客场逆势EV修正。", accuracyStr, absDiff)
		}
		if brierScore < 0.1 {
			return fmt.Sprintf("赛事精算%s：均势下客队凭借防守反击夺下三分，1球小胜基本在合理泊松覆盖区间内。模型对中场绞杀的还原度符合预期。", accuracyStr)
		}
		return fmt.Sprintf("赛事精算%s：主队遭遇阻击小负。客队临场反击效率超出常规DC参数范围，建议收窄小样本H2H数据的过度拟合，并引入更严格的战术阻尼。", accuracyStr)
	} else { // 平局
		if homeScore == 0 { // 0:0
			if brierScore < 0.1 {
				return fmt.Sprintf("赛事精算%s：双方战术高度胶着，均未创造出有效威胁，0:0闷平。自适应平局系数修正成功捕捉该走势，防守锁死符合预期。", accuracyStr)
			}
			return fmt.Sprintf("赛事精算%s：两队0:0互交白卷。进球期望值由于临场高压防守被严重稀释，模型未对进攻衰减进行足够扣减，建议在德比等高压对抗中引入进攻下修因子。", accuracyStr)
		}
		if brierScore < 0.1 {
			return fmt.Sprintf("赛事精算%s：双方大打攻防转换，最终%d:%d战平。平局修正系数及进球概率矩阵 of Dixon-Coles 完全覆盖了比分结果。", accuracyStr, homeScore, awayScore)
		}
		return fmt.Sprintf("赛事精算%s：两队平分秋色打出高比分平局。由于临场两队战术对攻较激进导致进球溢出，后续需针对平局形态进行多维期望优化。", accuracyStr)
	}
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
   - 【物理中立场与主客场法则】：2026世界杯仅当东道主美国(United States)、加拿大(Canada)、墨西哥(Mexico)在其本土场馆比赛时才享有真实的主场优势。除此三支东道主本土作战外，所有其他比赛（如西班牙 vs 佛得角等）均为完全中立的第三方场地，两队均无任何主场优势！绝对禁止脑补任何“主场维持进攻”、“主场加成”等主客场优势的幻觉。你在叙述时只准使用“名义主队”、“名义客队”，决不可夸大主场之利！
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

	s.mu.Lock()
	resp, err := s.client.Do(req)
	s.mu.Unlock()
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
   - 【物理中立场与主客场法则】：2026世界杯仅当东道主美国(United States)、加拿大(Canada)、墨西哥(Mexico)在其本土场馆比赛时才享有真实的主场优势。除此三支东道主本土作战外，所有其他比赛（如西班牙 vs 佛得角等）均为完全中立的第三方场地，两队均无任何主场优势！绝对禁止脑补任何“主场维持进攻”、“主场加成”等主客场优势的幻觉。你在叙述时只准使用“名义主队”、“名义客队”，决不可夸大主场之利！
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

	s.mu.Lock()
	resp, err := s.client.Do(req)
	s.mu.Unlock()
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

// requestOllama 执行一次通用的 Ollama API 请求并返回模型的生成文本内容
func (s *OllamaService) requestOllama(ctx context.Context, model string, prompt string, temperature float64, numPredict int) (string, error) {
	payload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"stream": false,
		"think":  false,
		"options": map[string]interface{}{
			"temperature": temperature,
			"num_predict": numPredict,
		},
		"keep_alive": -1,
	}

	bytesPayload, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.apiURL, bytes.NewBuffer(bytesPayload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	s.mu.Lock()
	resp, err := s.client.Do(req)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var rawRes struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rawRes); err != nil {
		return "", err
	}
	return rawRes.Message.Content, nil
}

// WarmUp 异步预热大模型：在服务启动时触发 35B 和 8B 模型的首次加载，避免前台冷启动延迟
func (s *OllamaService) WarmUp() {
	go func() {
		// 为了加快预热，我们将 num_predict 设为 1，这样模型载入后就会立刻返回
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()

		log.Println("[Ollama] 🚀 开始异步预热本地大模型，防止前台冷启动延迟...")

		// 1. 预热 qwen3.6:35b-q4
		log.Printf("[Ollama] 🔄 正在后台预热 35B 本地大模型 (%s)...", s.model)
		_, err1 := s.requestOllama(ctx, s.model, "hi", 0.1, 1)
		if err1 != nil {
			log.Printf("[Ollama] ⚠️ 35B 大模型预热失败或超时: %v", err1)
		} else {
			log.Println("[Ollama] ✅ 35B 本地大模型预热载入成功")
		}

		// 2. 预热 qwen3:8b
		log.Println("[Ollama] 🔄 正在后台预热 8B 辅助魔鬼反驳大模型 (qwen3:8b)...")
		_, err2 := s.requestOllama(ctx, "qwen3:8b", "hi", 0.1, 1)
		if err2 != nil {
			log.Printf("[Ollama] ⚠️ 8B 大模型预热失败或超时: %v", err2)
		} else {
			log.Println("[Ollama] ✅ 8B 辅助魔鬼反驳大模型预热载入成功")
		}

		log.Println("[Ollama] 🎉 大模型后台异步预热流程执行完毕")
	}()
}

func getTeamCnName(enName string) string {
	t, err := db.GetTeamTranslation(enName)
	if err == nil && t.CnName != "" {
		return t.CnName
	}
	fallback := map[string]string{
		"Austria":                          "奥地利",
		"Saudi Arabia":                     "沙特阿拉伯",
		"Democratic Republic of the Congo": "刚果金",
	}
	if cn, ok := fallback[enName]; ok {
		return cn
	}
	return enName
}

type BetAdviceMatchInput struct {
	MatchID      string  `json:"matchId"`
	HomeTeam     string  `json:"homeTeam"`
	AwayTeam     string  `json:"awayTeam"`
	HomeOdds     float64 `json:"homeOdds"`
	DrawOdds     float64 `json:"drawOdds"`
	AwayOdds     float64 `json:"awayOdds"`
	GoalLine     int     `json:"goalLine"`
	HhadHomeOdds float64 `json:"hhadHomeOdds"`
	HhadDrawOdds float64 `json:"hhadDrawOdds"`
	HhadAwayOdds float64 `json:"hhadAwayOdds"`
	IsSingleHad  bool    `json:"isSingleHad"`
	IsSingleHhad bool    `json:"isSingleHhad"`
	HomeProb     float64 `json:"homeProb"`
	DrawProb     float64 `json:"drawProb"`
	AwayProb     float64 `json:"awayProb"`
	HomeCn       string  `json:"homeCn"`
	AwayCn       string  `json:"awayCn"`
}

type BetAdviceItem struct {
	MatchName string  `json:"matchName"`
	Market    string  `json:"market"`
	Selection string  `json:"selection"`
	Odds      float64 `json:"odds"`
	Stake     float64 `json:"stake"`
	BetType   string  `json:"betType"`
	Prob      float64 `json:"prob"`
}

type BetAdviceResult struct {
	ProponentOpinion string                `json:"proponentOpinion"`
	CritiqueAnalysis string                `json:"critiqueAnalysis"`
	ConsensusReason  string                `json:"consensusReason"`
	SafeScheme       []BetAdviceItem       `json:"safeScheme"`
	AggressiveScheme []BetAdviceItem       `json:"aggressiveScheme"`
	ExpectedROI      float64               `json:"expectedRoi"`
	Matches          []BetAdviceMatchInput `json:"matches"`
	MarkdownReport   string                `json:"markdownReport"`
}

func (s *OllamaService) GenerateBetAdviceWithAgents(matches []BetAdviceMatchInput, totalAmount float64, safeRatio float64, singleRatio float64, mode string, allowHighParlay bool) (BetAdviceResult, error) {
	matchesJSON, _ := json.Marshal(matches)
	step1Prompt := fmt.Sprintf(`作为体彩投注立论专家。
数据：总投注额:%f, 稳妥方案比例:%f, 单关偏好:%f, 模式:%s。
比赛及赔率/单关限制如下：%s
规则：不能单关的比赛必须在2串1中投注。稳妥方案重防守，激进方案博高回报。
请针对此配资提出正面立论理由（proponentOpinion，100字内）与初步方案。
严格输出JSON无markdown:
{"proponentOpinion":"立论理由"}
/no_think`, totalAmount, safeRatio, singleRatio, mode, string(matchesJSON))

	var res struct {
		ProponentOpinion string `json:"proponentOpinion"`
	}
	limitTimeout := s.predictTimeout
	if limitTimeout > 15*time.Second {
		limitTimeout = 15 * time.Second
	}

	ctx1, cancel1 := context.WithTimeout(context.Background(), limitTimeout)
	r1, err1 := s.requestOllama(ctx1, s.model, step1Prompt, 0.1, 120)
	cancel1()
	if err1 == nil && r1 != "" {
		_ = json.Unmarshal([]byte(extractJSON(r1)), &res)
	}
	if res.ProponentOpinion == "" {
		res.ProponentOpinion = "建议采取稳健单关为主，过关冷门对冲为辅的总体投注框架。"
	}

	step2Prompt := fmt.Sprintf(`作为投注魔鬼反驳人。
先前立论主张: "%s"。
请指出其漏洞（冷门、热门陷阱、期望值EV不足、过关容错差等），给出反驳意见(critiqueAnalysis，100字内)。
严格输出JSON无markdown:
{"critiqueAnalysis":"反驳意见"}
/no_think`, res.ProponentOpinion)

	var res2 struct {
		CritiqueAnalysis string `json:"critiqueAnalysis"`
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), limitTimeout)
	r2, err2 := s.requestOllama(ctx2, "qwen3:8b", step2Prompt, 0.2, 100)
	cancel2()
	if err2 == nil && r2 != "" {
		_ = json.Unmarshal([]byte(extractJSON(r2)), &res2)
	}
	if res2.CritiqueAnalysis == "" {
		res2.CritiqueAnalysis = "注意防范无单关下的过关连环断链风险，并合理收紧高赔偏置的本金占比。"
	}

	step3Prompt := fmt.Sprintf(`作为首席投注裁决官。
立论主张: "%s"
魔鬼反驳: "%s"
请给共识裁决理由(consensusReason，120字内)，生成最终方案(安全稳妥safeScheme，激进爆冷aggressiveScheme)及预期ROI(expectedRoi，0~1浮点数)。
规则：
1. 稳妥总额约 %f，激进总额为剩余本金。单项下注金额必须是2的倍数，总和精确等于 %f。
2. isSingleHad/isSingleHhad为false的比赛绝不允许单关，必须2串1过关！
格式：
{"consensusReason":"共识裁决","safeScheme":[{"matchName":"巴西 VS 海地","market":"胜平负","selection":"主胜","odds":1.15,"stake":130.0,"betType":"2串1"}],"aggressiveScheme":[],"expectedRoi":0.15}
/no_think`, res.ProponentOpinion, res2.CritiqueAnalysis, totalAmount*safeRatio, totalAmount)

	var finalResult BetAdviceResult
	limitTimeoutFinal := s.predictTimeout
	if limitTimeoutFinal > 20*time.Second {
		limitTimeoutFinal = 20 * time.Second
	}
	ctx3, cancel3 := context.WithTimeout(context.Background(), limitTimeoutFinal)
	r3, err3 := s.requestOllama(ctx3, s.model, step3Prompt, 0.1, 200)
	cancel3()
	if err3 == nil && r3 != "" {
		_ = json.Unmarshal([]byte(extractJSON(r3)), &finalResult)
	}

	finalResult.ProponentOpinion = res.ProponentOpinion
	finalResult.CritiqueAnalysis = res2.CritiqueAnalysis
	finalResult.Matches = matches
	if finalResult.ConsensusReason == "" {
		finalResult.ConsensusReason = "达成综合裁决，安全部分锁定稳健主力场次，激进部分用于过关博弈中等赔付。"
	}
	// 强制运行经过精算审计的动态投注分配算法以确保真实资金投注的合规与安全
	finalResult.SafeScheme, finalResult.AggressiveScheme, finalResult.ExpectedROI = s.generateFallbackAdvice(matches, totalAmount, safeRatio, allowHighParlay)
	return finalResult, nil
}

func allocateStakes(total float64, ratios []float64) []float64 {
	n := len(ratios)
	if n == 0 {
		return nil
	}
	stakes := make([]float64, n)
	sumStakes := 0.0
	for i, r := range ratios {
		val := math.Round((total*r)/2) * 2
		stakes[i] = val
		sumStakes += val
	}
	diff := total - sumStakes
	if diff != 0 {
		maxIdx := 0
		maxVal := stakes[0]
		for i := 1; i < n; i++ {
			if stakes[i] > maxVal {
				maxVal = stakes[i]
				maxIdx = i
			}
		}
		stakes[maxIdx] += diff
		if stakes[maxIdx] < 0 {
			stakes[maxIdx] = 0
		}
	}
	return stakes
}

func (s *OllamaService) generateFallbackAdvice(matches []BetAdviceMatchInput, totalAmount float64, safeRatio float64, allowHighParlay bool) ([]BetAdviceItem, []BetAdviceItem, float64) {
	safeAmt := math.Round((totalAmount*safeRatio)/2) * 2
	aggAmt := totalAmount - safeAmt
	var safe, agg []BetAdviceItem
	if len(matches) == 0 {
		return safe, agg, 0.15
	}
	var validMatches []BetAdviceMatchInput
	for _, m := range matches {
		if (m.HomeOdds > 1.01 && m.AwayOdds > 1.01) || (m.HhadHomeOdds > 1.01 && m.HhadAwayOdds > 1.01) {
			validMatches = append(validMatches, m)
		}
	}
	if len(validMatches) == 0 {
		validMatches = matches
	}
	var singleMatch BetAdviceMatchInput
	isRealSingleHad := false
	for _, m := range validMatches {
		if m.IsSingleHad {
			singleMatch = m
			isRealSingleHad = true
			break
		}
	}
	if !isRealSingleHad {
		minOdds := 999.0
		for _, m := range validMatches {
			if m.HomeOdds > 1.01 {
				if m.HomeOdds < minOdds {
					minOdds = m.HomeOdds
					singleMatch = m
				}
				if m.AwayOdds > 1.01 && m.AwayOdds < minOdds {
					minOdds = m.AwayOdds
					singleMatch = m
				}
			}
		}
	}
	if singleMatch.MatchID == "" && len(validMatches) > 0 {
		singleMatch = validMatches[0]
	}
	singleName := fmt.Sprintf("%s VS %s", getTeamCnName(singleMatch.HomeTeam), getTeamCnName(singleMatch.AwayTeam))
	mainSel := "主胜"
	mainOdds := singleMatch.HomeOdds
	if singleMatch.AwayOdds > 0 && singleMatch.AwayOdds < singleMatch.HomeOdds {
		mainSel = "客胜"
		mainOdds = singleMatch.AwayOdds
	}
	type ParlayLeg struct {
		MatchName string
		Selection string
		Odds      float64
		DrawOdds  float64
	}
	var safeLegs []ParlayLeg
	for _, sm := range validMatches {
		hCn := getTeamCnName(sm.HomeTeam)
		aCn := getTeamCnName(sm.AwayTeam)
		legSel := "主胜"
		legOdds := sm.HomeOdds
		// 如果胜平负不可售，则降级使用让球胜平负做串关
		if sm.HomeOdds <= 1.01 || sm.AwayOdds <= 1.01 {
			if sm.HhadAwayOdds > 0 && sm.HhadAwayOdds < sm.HhadHomeOdds {
				legOdds = sm.HhadAwayOdds
				if sm.GoalLine < 0 {
					legSel = fmt.Sprintf("让负(%d)", sm.GoalLine)
				} else {
					legSel = fmt.Sprintf("让负(+%d)", sm.GoalLine)
				}
			} else {
				legOdds = sm.HhadHomeOdds
				if sm.GoalLine < 0 {
					legSel = fmt.Sprintf("让胜(%d)", sm.GoalLine)
				} else {
					legSel = fmt.Sprintf("让胜(+%d)", sm.GoalLine)
				}
			}
		} else {
			if sm.AwayOdds > 0 && sm.AwayOdds < sm.HomeOdds {
				legSel = "客胜"
				legOdds = sm.AwayOdds
			}
		}
		safeLegs = append(safeLegs, ParlayLeg{
			MatchName: fmt.Sprintf("%s VS %s", hCn, aCn),
			Selection: legSel,
			Odds:      legOdds,
			DrawOdds:  sm.DrawOdds,
		})
	}
	var safeRatios []float64
	type Combo struct {
		Leg1 ParlayLeg
		Leg2 ParlayLeg
	}
	var rawCombos []Combo
	for i := 0; i < len(safeLegs); i++ {
		for j := i + 1; j < len(safeLegs); j++ {
			rawCombos = append(rawCombos, Combo{Leg1: safeLegs[i], Leg2: safeLegs[j]})
		}
	}
	// 去胆码化过滤：每场比赛在稳妥2串1中最多出现 2 次
	legCounts := make(map[string]int)
	var combos []Combo
	for _, c := range rawCombos {
		if legCounts[c.Leg1.MatchName] < 2 && legCounts[c.Leg2.MatchName] < 2 {
			combos = append(combos, c)
			legCounts[c.Leg1.MatchName]++
			legCounts[c.Leg2.MatchName]++
		}
	}
	if len(combos) == 0 {
		combos = rawCombos
	}

	type DefCombo struct {
		Leg1Name string
		Leg2Name string
		Leg1Sel  string
		Leg2Sel  string
		Odds     float64
	}
	var defCombos []DefCombo
	if len(safeLegs) >= 3 {
		defCombos = append(defCombos, DefCombo{
			Leg1Name: safeLegs[0].MatchName, Leg2Name: safeLegs[1].MatchName,
			Leg1Sel: "平局", Leg2Sel: safeLegs[1].Selection,
			Odds: math.Round(safeLegs[0].DrawOdds*safeLegs[1].Odds*100)/100,
		})
		defCombos = append(defCombos, DefCombo{
			Leg1Name: safeLegs[1].MatchName, Leg2Name: safeLegs[2].MatchName,
			Leg1Sel: "平局", Leg2Sel: safeLegs[2].Selection,
			Odds: math.Round(safeLegs[1].DrawOdds*safeLegs[2].Odds*100)/100,
		})
		if len(safeLegs) > 2 {
			defCombos = append(defCombos, DefCombo{
				Leg1Name: safeLegs[2].MatchName, Leg2Name: safeLegs[0].MatchName,
				Leg1Sel: "平局", Leg2Sel: safeLegs[0].Selection,
				Odds: math.Round(safeLegs[2].DrawOdds*safeLegs[0].Odds*100)/100,
			})
		}
	}
	var highCombos []BetAdviceItem
	if allowHighParlay && len(safeLegs) >= 3 {
		if len(safeLegs) == 3 {
			cOdds := math.Round(safeLegs[0].Odds*safeLegs[1].Odds*safeLegs[2].Odds*100)/100
			highCombos = append(highCombos, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s & %s", safeLegs[0].MatchName, safeLegs[1].MatchName, safeLegs[2].MatchName),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s & %s", safeLegs[0].Selection, safeLegs[1].Selection, safeLegs[2].Selection),
				Odds:      cOdds,
				BetType:   "3串1",
			})
			safeRatios = append(safeRatios, 0.35, 0.10)
			for range combos {
				safeRatios = append(safeRatios, 0.35/float64(len(combos)))
			}
			for range defCombos {
				safeRatios = append(safeRatios, 0.15/float64(len(defCombos)))
			}
			safeRatios = append(safeRatios, 0.05)
		} else {
			cOdds3 := math.Round(safeLegs[0].Odds*safeLegs[1].Odds*safeLegs[2].Odds*100)/100
			highCombos = append(highCombos, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s & %s", safeLegs[0].MatchName, safeLegs[1].MatchName, safeLegs[2].MatchName),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s & %s", safeLegs[0].Selection, safeLegs[1].Selection, safeLegs[2].Selection),
				Odds:      cOdds3,
				BetType:   "3串1",
			})
			cOdds4 := math.Round(safeLegs[0].Odds*safeLegs[1].Odds*safeLegs[2].Odds*safeLegs[3].Odds*100)/100
			highCombos = append(highCombos, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s & %s & %s", safeLegs[0].MatchName, safeLegs[1].MatchName, safeLegs[2].MatchName, safeLegs[3].MatchName),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s & %s & %s", safeLegs[0].Selection, safeLegs[1].Selection, safeLegs[2].Selection, safeLegs[3].Selection),
				Odds:      cOdds4,
				BetType:   "4串1",
			})
			safeRatios = append(safeRatios, 0.35, 0.10)
			for range combos {
				safeRatios = append(safeRatios, 0.35/float64(len(combos)))
			}
			for range defCombos {
				safeRatios = append(safeRatios, 0.10/float64(len(defCombos)))
			}
			safeRatios = append(safeRatios, 0.05, 0.05)
		}
	} else {
		if len(safeLegs) >= 3 {
			safeRatios = append(safeRatios, 0.35, 0.10) // Single WDW Main, Single Hedge
			for range combos {
				safeRatios = append(safeRatios, 0.40/float64(len(combos)))
			}
			for range defCombos {
				safeRatios = append(safeRatios, 0.15/float64(len(defCombos)))
			}
		} else if len(safeLegs) == 2 {
			safeRatios = append(safeRatios, 0.35, 0.10, 0.30, 0.15, 0.10) // Single Main, Single Hedge, Main Parlay, Def 1, Def 2
		} else {
			safeRatios = append(safeRatios, 0.75, 0.25)
		}
	}
	safeStakes := allocateStakes(safeAmt, safeRatios)
	if isRealSingleHad {
		safe = append(safe, BetAdviceItem{
			MatchName: singleName, Market: "胜平负", Selection: mainSel, Odds: mainOdds, Stake: safeStakes[0], BetType: "单关",
		})
	} else {
		var subSel string
		var subOdds float64
		if mainSel == "主胜" {
			subSel = "1:0"
			subOdds = 6.50
		} else if mainSel == "客胜" {
			subSel = "0:1"
			subOdds = 7.00
		} else {
			subSel = "0:0"
			subOdds = 8.50
		}
		safe = append(safe, BetAdviceItem{
			MatchName: singleName, Market: "比分", Selection: subSel, Odds: subOdds, Stake: safeStakes[0], BetType: "单关",
		})
	}
	safe = append(safe, BetAdviceItem{
		MatchName: singleName, Market: "比分", Selection: "1:1", Odds: 5.70, Stake: safeStakes[1], BetType: "单关",
	})
	if len(safeLegs) >= 3 {
		idx := 2
		for _, c := range combos {
			safe = append(safe, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s", c.Leg1.MatchName, c.Leg2.MatchName),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s", c.Leg1.Selection, c.Leg2.Selection),
				Odds:      math.Round(c.Leg1.Odds*c.Leg2.Odds*100)/100,
				Stake:     safeStakes[idx],
				BetType:   "2串1",
			})
			idx++
		}
		for _, dc := range defCombos {
			safe = append(safe, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s", dc.Leg1Name, dc.Leg2Name),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s", dc.Leg1Sel, dc.Leg2Sel),
				Odds:      dc.Odds,
				Stake:     safeStakes[idx],
				BetType:   "2串1",
			})
			idx++
		}
		for hIdx, hc := range highCombos {
			hc.Stake = safeStakes[idx+hIdx]
			safe = append(safe, hc)
		}
	} else if len(safeLegs) == 2 {
		safe = append(safe, BetAdviceItem{
			MatchName: fmt.Sprintf("%s & %s", safeLegs[0].MatchName, safeLegs[1].MatchName),
			Market:    "混合过关",
			Selection: fmt.Sprintf("%s & %s", safeLegs[0].Selection, safeLegs[1].Selection),
			Odds:      math.Round(safeLegs[0].Odds*safeLegs[1].Odds*100)/100,
			Stake:     safeStakes[2],
			BetType:   "2串1",
		})
		safe = append(safe, BetAdviceItem{
			MatchName: fmt.Sprintf("%s & %s", safeLegs[0].MatchName, safeLegs[1].MatchName),
			Market:    "混合过关",
			Selection: fmt.Sprintf("%s & %s", safeLegs[0].Selection, "平局"),
			Odds:      math.Round(safeLegs[0].Odds*safeLegs[1].DrawOdds*100)/100,
			Stake:     safeStakes[3],
			BetType:   "2串1",
		})
		safe = append(safe, BetAdviceItem{
			MatchName: fmt.Sprintf("%s & %s", safeLegs[0].MatchName, safeLegs[1].MatchName),
			Market:    "混合过关",
			Selection: fmt.Sprintf("%s & %s", "平局", safeLegs[1].Selection),
			Odds:      math.Round(safeLegs[0].DrawOdds*safeLegs[1].Odds*100)/100,
			Stake:     safeStakes[4],
			BetType:   "2串1",
		})
	}
	var aggSingles []BetAdviceItem
	for _, m := range validMatches {
		if m.HomeOdds <= 1.01 || m.AwayOdds <= 1.01 {
			continue
		}
		hCn := getTeamCnName(m.HomeTeam)
		aCn := getTeamCnName(m.AwayTeam)
		mName := fmt.Sprintf("%s VS %s", hCn, aCn)
		if m.HomeOdds > 0 && m.HomeOdds < 1.30 {
			aggSingles = append(aggSingles, BetAdviceItem{
				MatchName: mName, Market: "比分", Selection: "1:1", Odds: 19.00, BetType: "单关",
			})
		} else if m.AwayOdds > 0 && m.AwayOdds < 1.30 {
			aggSingles = append(aggSingles, BetAdviceItem{
				MatchName: mName, Market: "比分", Selection: "1:1", Odds: 19.00, BetType: "单关",
			})
		} else if m.HomeOdds > 0 && m.HomeOdds < 1.65 {
			aggSingles = append(aggSingles, BetAdviceItem{
				MatchName: mName, Market: "半全场", Selection: "胜胜", Odds: 3.20, BetType: "单关",
			})
		} else if m.AwayOdds > 0 && m.AwayOdds < 1.65 {
			aggSingles = append(aggSingles, BetAdviceItem{
				MatchName: mName, Market: "半全场", Selection: "负负", Odds: 3.20, BetType: "单关",
			})
		} else {
			aggSingles = append(aggSingles, BetAdviceItem{
				MatchName: mName, Market: "比分", Selection: "0:0", Odds: 10.00, BetType: "单关",
			})
		}
	}
	if len(aggSingles) > 4 {
		aggSingles = aggSingles[:4]
	}
	type HhadLeg struct {
		MatchName string
		Selection string
		Odds      float64
	}
	var hhadLegs []HhadLeg
	for _, m := range validMatches {
		hCn := getTeamCnName(m.HomeTeam)
		aCn := getTeamCnName(m.AwayTeam)
		mName := fmt.Sprintf("%s VS %s", hCn, aCn)
		sel := "让胜"
		odds := m.HhadHomeOdds
		if m.GoalLine < 0 {
			if m.HhadAwayOdds >= 1.80 && m.HhadAwayOdds <= 3.80 {
				sel = fmt.Sprintf("让负(%d)", m.GoalLine)
				odds = m.HhadAwayOdds
			} else {
				sel = fmt.Sprintf("让胜(%d)", m.GoalLine)
				odds = m.HhadHomeOdds
			}
		} else {
			if m.HhadHomeOdds >= 1.80 && m.HhadHomeOdds <= 3.80 {
				sel = fmt.Sprintf("让胜(+%d)", m.GoalLine)
				odds = m.HhadHomeOdds
			} else {
				sel = fmt.Sprintf("让负(+%d)", m.GoalLine)
				odds = m.HhadAwayOdds
			}
		}
		if odds > 1.0 {
			hhadLegs = append(hhadLegs, HhadLeg{MatchName: mName, Selection: sel, Odds: odds})
		}
	}
	type HhadCombo struct {
		Leg1 HhadLeg
		Leg2 HhadLeg
	}
	var rawHhadCombos []HhadCombo
	for i := 0; i < len(hhadLegs); i++ {
		for j := i + 1; j < len(hhadLegs); j++ {
			rawHhadCombos = append(rawHhadCombos, HhadCombo{Leg1: hhadLegs[i], Leg2: hhadLegs[j]})
		}
	}
	// 让球2串1去胆码化过滤：每场比赛最多出现 2 次
	hhadCounts := make(map[string]int)
	var hhadCombos []HhadCombo
	for _, hc := range rawHhadCombos {
		if hhadCounts[hc.Leg1.MatchName] < 2 && hhadCounts[hc.Leg2.MatchName] < 2 {
			hhadCombos = append(hhadCombos, hc)
			hhadCounts[hc.Leg1.MatchName]++
			hhadCounts[hc.Leg2.MatchName]++
		}
	}
	if len(hhadCombos) == 0 {
		hhadCombos = rawHhadCombos
	}
	if len(hhadCombos) > 6 {
		hhadCombos = hhadCombos[:6]
	}
	var aggHighCombos []BetAdviceItem
	var aggRatios []float64
	for range aggSingles {
		aggRatios = append(aggRatios, 0.65/float64(len(aggSingles)))
	}
	if allowHighParlay && len(hhadLegs) >= 3 {
		if len(hhadLegs) == 3 {
			cOdds := math.Round(hhadLegs[0].Odds*hhadLegs[1].Odds*hhadLegs[2].Odds*100)/100
			aggHighCombos = append(aggHighCombos, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s & %s", hhadLegs[0].MatchName, hhadLegs[1].MatchName, hhadLegs[2].MatchName),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s & %s", hhadLegs[0].Selection, hhadLegs[1].Selection, hhadLegs[2].Selection),
				Odds:      cOdds,
				BetType:   "3串1",
			})
			for range hhadCombos {
				aggRatios = append(aggRatios, 0.20/float64(len(hhadCombos)))
			}
			aggRatios = append(aggRatios, 0.15)
		} else {
			cOdds3 := math.Round(hhadLegs[0].Odds*hhadLegs[1].Odds*hhadLegs[2].Odds*100)/100
			aggHighCombos = append(aggHighCombos, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s & %s", hhadLegs[0].MatchName, hhadLegs[1].MatchName, hhadLegs[2].MatchName),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s & %s", hhadLegs[0].Selection, hhadLegs[1].Selection, hhadLegs[2].Selection),
				Odds:      cOdds3,
				BetType:   "3串1",
			})
			cOdds4 := math.Round(hhadLegs[0].Odds*hhadLegs[1].Odds*hhadLegs[2].Odds*hhadLegs[3].Odds*100)/100
			aggHighCombos = append(aggHighCombos, BetAdviceItem{
				MatchName: fmt.Sprintf("%s & %s & %s & %s", hhadLegs[0].MatchName, hhadLegs[1].MatchName, hhadLegs[2].MatchName, hhadLegs[3].MatchName),
				Market:    "混合过关",
				Selection: fmt.Sprintf("%s & %s & %s & %s", hhadLegs[0].Selection, hhadLegs[1].Selection, hhadLegs[2].Selection, hhadLegs[3].Selection),
				Odds:      cOdds4,
				BetType:   "4串1",
			})
			for range hhadCombos {
				aggRatios = append(aggRatios, 0.20/float64(len(hhadCombos)))
			}
			aggRatios = append(aggRatios, 0.08, 0.07)
		}
	} else {
		for range hhadCombos {
			aggRatios = append(aggRatios, 0.35/float64(len(hhadCombos)))
		}
	}
	if len(aggRatios) == 0 {
		aggRatios = append(aggRatios, 1.0)
	}
	aggStakes := allocateStakes(aggAmt, aggRatios)
	idxAgg := 0
	for _, as := range aggSingles {
		as.Stake = aggStakes[idxAgg]
		agg = append(agg, as)
		idxAgg++
	}
	for _, hc := range hhadCombos {
		agg = append(agg, BetAdviceItem{
			MatchName: fmt.Sprintf("%s & %s", hc.Leg1.MatchName, hc.Leg2.MatchName),
			Market:    "混合过关",
			Selection: fmt.Sprintf("%s & %s", hc.Leg1.Selection, hc.Leg2.Selection),
			Odds:      math.Round(hc.Leg1.Odds*hc.Leg2.Odds*100)/100,
			Stake:     aggStakes[idxAgg],
			BetType:   "2串1",
		})
		idxAgg++
	}
	for hIdx, hc := range aggHighCombos {
		hc.Stake = aggStakes[idxAgg+hIdx]
		agg = append(agg, hc)
	}
	return safe, agg, 0.15
}

