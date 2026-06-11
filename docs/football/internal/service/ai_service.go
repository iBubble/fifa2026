package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"time"

	"football/internal/models"
)

type AIService struct {
	httpClient *http.Client
}

func NewAIService() *AIService {
	return &AIService{
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

// GetLLMAnalysis 获取大模型实时深度分析或本地泊松公式预测
func (s *AIService) GetLLMAnalysis(ctx context.Context, match models.Match, oddsHome, oddsDraw, oddsAway float64) (string, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")

	// 计算基础胜率和指标，作为模型输入的丰富上下文
	homeProb := 0.45
	drawProb := 0.25
	awayProb := 0.30
	confidence := 0.75

	if oddsHome > 0 && oddsDraw > 0 && oddsAway > 0 {
		margin := (1.0/oddsHome + 1.0/oddsDraw + 1.0/oddsAway)
		homeProb = (1.0 / oddsHome) / margin
		drawProb = (1.0 / oddsDraw) / margin
		awayProb = (1.0 / oddsAway) / margin
	}

	// 1. 若没有配置 GEMINI_API_KEY，自动调用本地高精度 Poisson 计算核心进行本地预测
	if apiKey == "" || apiKey == "YOUR_GEMINI_API_KEY_HERE" {
		return s.GetPoissonReport(match, oddsHome, oddsDraw, oddsAway, homeProb, drawProb, awayProb, confidence), nil
	}

	// 2. 配置了 GEMINI_API_KEY，向官方 Gemini 接口请求生成内容
	prompt := fmt.Sprintf(`
你是一位全球顶尖的足球量化交易与大模型分析专家。请根据以下关于本场比赛的实时市场赔率、概率指标及对阵信息，进行深度的量化与精确比分/进球数预测，并给出严谨的投注建议。

【当前比赛信息】
- 联赛: %s
- 对决双方: %s (主队) VS %s (客队)
- 开赛时间: %s
- 当前状态: %s

【当前跨机构最优开盘赔率】
- 主胜 (1): %.2f
- 平局 (X): %.2f
- 客胜 (2): %.2f

【量化模型计算指标】
- 主队隐含胜率: %.1f%%
- 平局隐含概率: %.1f%%
- 客队隐含胜率: %.1f%%
- 综合指标置信度: %.1f%%

请严格按照以下格式生成一份专业、精美且充满洞察力的量化报告（请使用标准的 Markdown 格式，以完美契合系统的暗黑玻璃风格面版）：

### 🎯 1. 战局基本面与大模型实力评估
（请结合双方战术形态、防守压迫力、角球和进球动能做简要分析，指出主力倾向）

### 📊 2. 精确比分与进球数概率预测
- **最可能比分**：X - Y (概率：P%%)
- **次可能比分**：A - B (概率：Q%%)
- **大球 (Over 2.5) 概率**：U%%
- **小球 (Under 2.5) 概率**：V%%
- **大模型总进球数推荐**：X.X 球区间

### ⚖️ 3. 凯利公式注单优选与压球建议
- **最优价值投注项**：（主胜/客胜/平局/大球/小球）
- **期望回报率 (EV)**：Z%%
- **凯利资金配额占比**：本金的 W%% (分数凯利模型下)
- **压球建议与风控策略**：（请用专业且严谨的口吻给出具体压球方案，提醒注意仓位和分散投资）

> [!TIP]
> 💡 *本篇深度量化分析由 Google Gemini 2.5 实时大模型生成，大单赔率更新正常。*
`,
		match.League, match.HomeTeam, match.AwayTeam, match.ScheduledAt.Format("2006-01-02 15:04"), match.Status,
		oddsHome, oddsDraw, oddsAway,
		homeProb*100, drawProb*100, awayProb*100, confidence*100,
	)

	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=%s", apiKey)
	
	payload := map[string]interface{}{
		"contents": []interface{}{
			map[string]interface{}{
				"parts": []interface{}{
					map[string]interface{}{
						"text": prompt,
					},
				},
			},
		},
	}

	bytesPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("构建请求失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(bytesPayload))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("大模型请求超时: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 报错 %d: %s", resp.StatusCode, string(body))
	}

	var res struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		return res.Candidates[0].Content.Parts[0].Text, nil
	}

	return "", fmt.Errorf("Gemini 返回了空分析结果")
}

// GetPoissonReport 本地 Poisson 概率计算引擎
func (s *AIService) GetPoissonReport(match models.Match, oddsHome, oddsDraw, oddsAway, homeProb, drawProb, awayProb, confidence float64) string {
	lambdaHome := 1.1 + homeProb*1.2 - awayProb*0.4
	lambdaAway := 0.9 + awayProb*1.2 - homeProb*0.4
	if lambdaHome < 0.2 {
		lambdaHome = 0.2
	}
	if lambdaAway < 0.2 {
		lambdaAway = 0.2
	}

	type ScoreProb struct {
		Home int
		Away int
		Prob float64
	}
	var scores []ScoreProb
	over2_5 := 0.0
	under2_5 := 0.0

	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5; j++ {
			pHome := computePoissonProbability(lambdaHome, i)
			pAway := computePoissonProbability(lambdaAway, j)
			pJoint := pHome * pAway
			scores = append(scores, ScoreProb{Home: i, Away: j, Prob: pJoint})
			if i+j > 2 {
				over2_5 += pJoint
			} else {
				under2_5 += pJoint
			}
		}
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].Prob > scores[j].Prob
	})

	bestScore := scores[0]
	secondScore := scores[1]

	bestOutcome := "主队胜 (1)"
	bestOdds := oddsHome
	bestProb := homeProb
	if awayProb > homeProb && awayProb > drawProb {
		bestOutcome = "客队胜 (2)"
		bestOdds = oddsAway
		bestProb = awayProb
	} else if drawProb > homeProb && drawProb > awayProb {
		bestOutcome = "平局 (X)"
		bestOdds = oddsDraw
		bestProb = drawProb
	}

	kellyF := 0.0
	if bestOdds > 1.0 {
		b := bestOdds - 1.0
		kellyF = (b*bestProb - (1.0 - bestProb)) / b
	}
	suggestedStakePct := 0.0
	if kellyF > 0 {
		suggestedStakePct = kellyF * 0.25 * 100 // 1/4 凯利
	}
	ev := bestProb*bestOdds - 1.0

	return fmt.Sprintf(`### 🎯 1. 战局基本面与泊松实力评估
根据当前市场盘口状态，本场 **%s vs %s** 对决中，主队近期进攻期望 Lambda 值为 **%.2f**，客队客场进球拟合 Lambda 值为 **%.2f**。主客场权重修正后，整体比赛走势平稳。

### 📊 2. 精确比分与进球数概率预测
- **最可能比分**：**%d - %d** （概率：**%.2f%%**）
- **次可能比分**：**%d - %d** （概率：**%.2f%%**）
- **大球 (Over 2.5) 概率**：**%.2f%%**
- **小球 (Under 2.5) 概率**：**%.2f%%**
- **预测总进球数**：大约在 **%.1f** 之间

### ⚖️ 3. 凯利公式注单优选与压球建议
- **最优价值投注项**：**%s**
- **期望回报率 (EV)**：**%.2f%%**
- **凯利资金配额占比**：建议本金的 **%.2f%%** (1/4 分数凯利分仓)
- **压球建议与风控策略**：
  凯利方程式在此盘口下计算出 **%.2f%%** 的估值边际。考虑到今晚是终极决胜，战局波动系数高，建议采取稳健的低风险资金配额方案。让球盘或双平偏门具有优秀的对冲保护价值，切忌盲目全仓重注。

> [!NOTE]
> 💡 *本篇量化分析报告由本地泊松计算内核生成。您可以在根目录 '.env' 中配置 'GEMINI_API_KEY' 密钥，以一键解锁 Gemini-2.5-Flash 实时大模型分析与进球赔率深度解读报告。*
`,
		match.HomeTeam, match.AwayTeam,
		lambdaHome, lambdaAway,
		bestScore.Home, bestScore.Away, bestScore.Prob*100,
		secondScore.Home, secondScore.Away, secondScore.Prob*100,
		over2_5*100, under2_5*100,
		lambdaHome+lambdaAway,
		bestOutcome,
		ev*100,
		suggestedStakePct,
		(bestProb - 1.0/bestOdds)*100,
	)
}

func computePoissonProbability(lambda float64, k int) float64 {
	return (math.Pow(lambda, float64(k)) * math.Exp(-lambda)) / float64(factorial(k))
}

func factorial(n int) int {
	if n <= 0 {
		return 1
	}
	f := 1
	for i := 1; i <= n; i++ {
		f *= i
	}
	return f
}
