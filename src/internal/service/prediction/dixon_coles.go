package prediction

import (
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fmt"
	"math"
	"strings"
	"time"
)

type DixonColesService struct {
	elo              *EloService
	apiSports        *APISportsService
	rhoOffset        float64
	lambdaHomeOffset float64
	lambdaAwayOffset float64

	// 可配置优化预测参数
	NormDivulator    float64
	DiffMultiplier   float64
	H2hMultiplier    float64
	InitialRho       float64
}

func NewDixonColesService(elo *EloService, apiSports *APISportsService) *DixonColesService {
	s := &DixonColesService{
		elo:              elo,
		apiSports:        apiSports,
		rhoOffset:        0.0,
		lambdaHomeOffset: 0.0,
		lambdaAwayOffset: 0.0,
		NormDivulator:    1.10,
		DiffMultiplier:   0.20,
		H2hMultiplier:    0.05,
		InitialRho:       -0.08,
	}
	s.LoadParametersFromDB()
	s.RecalculateRhoOffset()
	s.RecalculateLambdaOffset()
	return s
}

// LoadParametersFromDB 从数据库载入系统调优参数，若存在则覆盖默认硬编码
func (s *DixonColesService) LoadParametersFromDB() {
	if val, found, err := db.GetSystemParam("NormDivulator"); err == nil && found {
		s.NormDivulator = val
	}
	if val, found, err := db.GetSystemParam("DiffMultiplier"); err == nil && found {
		s.DiffMultiplier = val
	}
	if val, found, err := db.GetSystemParam("H2hMultiplier"); err == nil && found {
		s.H2hMultiplier = val
	}
	if val, found, err := db.GetSystemParam("InitialRho"); err == nil && found {
		s.InitialRho = val
	}
}

// CalculateParams 根据两队当前 Elo 计算 Dixon-Coles 初始期望参数（包含自适应偏移）
func (s *DixonColesService) CalculateParams(homeTeam, awayTeam string) models.DixonColesParams {
	p := s.CalculateParamsWithoutOffset(homeTeam, awayTeam)
	
	// 应用全局累计自适应 Lambda 偏移量
	p.LambdaHome += s.lambdaHomeOffset
	p.LambdaAway += s.lambdaAwayOffset

	// 限制 Lambda 期望在 [0.2, 2.8] 安全边界内，防止数学计算溢出
	if p.LambdaHome > 2.8 {
		p.LambdaHome = 2.8
	}
	if p.LambdaHome < 0.2 {
		p.LambdaHome = 0.2
	}
	if p.LambdaAway > 2.8 {
		p.LambdaAway = 2.8
	}
	if p.LambdaAway < 0.2 {
		p.LambdaAway = 0.2
	}

	p.Rho += s.rhoOffset
	// 强约束 rho 的范围在 [-0.15, -0.01] 之间
	if p.Rho > -0.01 {
		p.Rho = -0.01
	}
	if p.Rho < -0.15 {
		p.Rho = -0.15
	}
	return p
}

func (s *DixonColesService) CalculateParamsWithoutOffset(homeTeam, awayTeam string) models.DixonColesParams {
	eloH := s.elo.GetElo(homeTeam)
	eloA := s.elo.GetElo(awayTeam)
	diff := (eloH - eloA) / 400.0

	// 整合历史场均进失球特征 (交战攻防底蕴)
	featH := s.elo.GetFeature(homeTeam)
	featA := s.elo.GetFeature(awayTeam)

	// 计算基础进球概率倾向 (几何平均数以平衡两队攻防，使用 NormDivulator 归一化以适度释放进球率，回归大盘平均期望)
	baseH := math.Sqrt(featH.AvgGoalsScored * featA.AvgGoalsConceded) / s.NormDivulator
	baseA := math.Sqrt(featA.AvgGoalsScored * featH.AvgGoalsConceded) / s.NormDivulator

	// 融入 api-football 历史直接交锋记录 (H2H) 场均进球数并与模型大盘进球率进行自适应加权混合
	var h2hDiff float64
	var hasH2H bool
	var drawRate float64
	var totalH2HMatches int
	if s.apiSports != nil {
		h2h, err := s.apiSports.GetH2HRecord(homeTeam, awayTeam)
		if err == nil && h2h.TotalMatches > 0 {
			hasH2H = true
			totalH2HMatches = h2h.TotalMatches
			// 自适应 H2H 权重：交手次数越多，克制权重越高，上限为 30%
			h2hWeight := math.Min(0.30, float64(h2h.TotalMatches)*0.08)
			if h2h.TotalMatches < 3 {
				// H2H 小样本（< 3 场）克制权重强行衰减 80% 平滑
				h2hWeight = h2hWeight * 0.20
			}

			avgHomeGoals := h2h.AvgHomeGoals
			avgAwayGoals := h2h.AvgAwayGoals
			h2hDiff = (float64(h2h.HomeWins) - float64(h2h.AwayWins)) / float64(h2h.TotalMatches)
			drawRate = float64(h2h.Draws) / float64(h2h.TotalMatches)

			if len(h2h.Matches) > 0 {
				var weightSum float64
				var weightedHomeGoals float64
				var weightedAwayGoals float64
				var weightedDiff float64
				var weightedDraws float64

				phi := 0.15 // 时间加权衰减常数 (以年为单位)
				now := time.Now()

				for _, m := range h2h.Matches {
					// 解析开赛时间
					tMatch, errTime := time.Parse(time.RFC3339, m.MatchTime)
					if errTime != nil {
						tMatch, errTime = time.Parse("2006-01-02", m.MatchTime)
					}

					daysDiff := 0.0
					if errTime == nil {
						daysDiff = now.Sub(tMatch).Hours() / 24.0
						if daysDiff < 0 {
							daysDiff = 0
						}
					}

					// 计算权重: e^(-phi * t_years)
					w := math.Exp(-phi * (daysDiff / 365.0))
					weightSum += w

					weightedHomeGoals += w * m.HomeGoals
					weightedAwayGoals += w * m.AwayGoals

					winSign := 0.0
					if m.HomeGoals > m.AwayGoals {
						winSign = 1.0
					} else if m.HomeGoals < m.AwayGoals {
						winSign = -1.0
					}
					weightedDiff += w * winSign

					if m.HomeGoals == m.AwayGoals {
						weightedDraws += w
					}
				}

				if weightSum > 0 {
					avgHomeGoals = weightedHomeGoals / weightSum
					avgAwayGoals = weightedAwayGoals / weightSum
					h2hDiff = weightedDiff / weightSum
					drawRate = weightedDraws / weightSum
				}
			}

			baseH = (1.0-h2hWeight)*baseH + h2hWeight*avgHomeGoals
			baseA = (1.0-h2hWeight)*baseA + h2hWeight*avgAwayGoals

			if h2h.TotalMatches < 3 {
				// H2H 小样本影响系数同步衰减 80%（降至 ±3%）
				h2hDiff = h2hDiff * 0.20
			}
		}
	}

	// 安全上限与下限约束，防止异常边界值导致数学溢出
	if baseH <= 0.2 {
		baseH = 0.2
	}
	if baseA <= 0.2 {
		baseA = 0.2
	}

	// 世界杯锦标赛为完全中立场，无主客场优势概念
	homeAdv := 0.0

	// 最终复合进球期望 Lambda：将基础攻防与实时 Elo 实力差的指数级加权调节相叠加
	lambdaH := baseH * math.Exp(homeAdv + s.DiffMultiplier*diff)
	lambdaA := baseA * math.Exp(-homeAdv - s.DiffMultiplier*diff)

	// 如果存在历史交战统计，继续叠加上胜率克制修正系数
	if hasH2H {
		lambdaH = lambdaH * (1.0 + s.H2hMultiplier*h2hDiff)
		lambdaA = lambdaA * (1.0 - s.H2hMultiplier*h2hDiff)
	}

	// 对最终 Lambda 期望加入 2.8 的上限阀值，防止高分段计算发生数学溢出
	if lambdaH > 2.8 {
		lambdaH = 2.8
	}
	if lambdaA > 2.8 {
		lambdaA = 2.8
	}

	// 经典平局相关系数初始值
	rho := s.InitialRho
	if hasH2H && totalH2HMatches >= 3 {
		if drawRate >= 0.35 {
			// 平局倾向强：负向加深 rho，强化 0-0/1-1 概率倾向
			rho -= 0.10 * drawRate
		} else if drawRate == 0 {
			// 无平局倾向：削弱平局因子
			rho += 0.04
		}
	}

	return models.DixonColesParams{
		LambdaHome: lambdaH,
		LambdaAway: lambdaA,
		Rho:        rho,
	}
}

// ComputePoissonProb 经典泊松分布概率质量计算
func (s *DixonColesService) ComputePoissonProb(lambda float64, k int) float64 {
	if k < 0 {
		return 0
	}
	return (math.Pow(lambda, float64(k)) * math.Exp(-lambda)) / float64(factorial(k))
}

// ComputeJointProbability Dixon-Coles 修正后的主客队联合比分概率
func (s *DixonColesService) ComputeJointProbability(x, y int, p models.DixonColesParams) float64 {
	pHome := s.ComputePoissonProb(p.LambdaHome, x)
	pAway := s.ComputePoissonProb(p.LambdaAway, y)
	rawJoint := pHome * pAway

	// Dixon-Coles 修正因子 tau
	tau := 1.0
	if x == 0 && y == 0 {
		tau = 1.0 - p.LambdaHome*p.LambdaAway*p.Rho
	} else if x == 1 && y == 0 {
		tau = 1.0 + p.LambdaAway*p.Rho
	} else if x == 0 && y == 1 {
		tau = 1.0 + p.LambdaHome*p.Rho
	} else if x == 1 && y == 1 {
		tau = 1.0 - p.Rho
	}

	return math.Max(0, rawJoint*tau) // 确保概率非负
}

// GenerateProbabilityMatrix 生成 6x6 精确比分概率矩阵，并推算大小球概率
func (s *DixonColesService) GenerateProbabilityMatrix(p models.DixonColesParams) ([]models.ScoreProbability, float64, float64) {
	var matrix []models.ScoreProbability
	var over2_5, under2_5 float64

	for x := 0; x <= 5; x++ {
		for y := 0; y <= 5; y++ {
			prob := s.ComputeJointProbability(x, y, p)
			matrix = append(matrix, models.ScoreProbability{
				HomeScore: x,
				AwayScore: y,
				Prob:      prob,
			})
			if x+y > 2 {
				over2_5 += prob
			} else {
				under2_5 += prob
			}
		}
	}

	// 进行归一化，确保总体概率和等于 1 (针对 6x6 以外被截断的微小尾部概率)
	total := 0.0
	for _, cell := range matrix {
		total += cell.Prob
	}
	if total > 0 {
		for i := range matrix {
			matrix[i].Prob /= total
		}
		over2_5 /= total
		under2_5 /= total
	}

	return matrix, over2_5, under2_5
}

func factorial(n int) int {
	if n <= 0 {
		return 1
	}
	f := 1
	for i := 1; i <= n; i++ {
		f *= f
	}
	// 修复原先可能死循环或精度丢失：阶乘应为 f *= i
	f = 1
	for i := 2; i <= n; i++ {
		f *= i
	}
	return f
}

// RecalculateRhoOffset 根据所有已结算比赛的历史 Brier Score 纠偏自适应平局系数
func (s *DixonColesService) RecalculateRhoOffset() {
	reports, err := db.GetBacktestReports()
	if err != nil || len(reports) == 0 {
		s.rhoOffset = 0.0
		return
	}

	offset := 0.0
	learningRate := 0.05

	for _, r := range reports {
		m, err := db.GetMatch(r.MatchID)
		if err != nil {
			continue
		}

		pBase := s.CalculateParamsWithoutOffset(m.HomeTeam, m.AwayTeam)
		p := pBase
		p.Rho += offset
		if p.Rho > -0.01 {
			p.Rho = -0.01
		}
		if p.Rho < -0.15 {
			p.Rho = -0.15
		}

		matrix, _, _ := s.GenerateProbabilityMatrix(p)
		pDraw := 0.0
		for _, cell := range matrix {
			if cell.HomeScore == cell.AwayScore {
				pDraw += cell.Prob
			}
		}

		oDraw := 0.0
		if m.HomeScore == m.AwayScore {
			oDraw = 1.0
		}

		// 反馈自校准：如果实际平局而预测偏低，需要 rho 变得更负（即 offset 变小/负），以提高平局修正作用
		delta := learningRate * (oDraw - pDraw) * r.BrierScore
		offset -= delta

		if offset > 0.07 {
			offset = 0.07
		}
		if offset < -0.07 {
			offset = -0.07
		}
	}
	s.rhoOffset = offset
}

// isHostNation 判断该球队是否为 2026 世界杯东道主
func isHostNation(team string) bool {
	return team == "United States" || team == "Mexico" || team == "Canada"
}

// CalculateParamsWithVenue 根据两队 Elo 和赛场场馆所在的物理东道主国家，计算带有精确主场加成的 Dixon-Coles 参数
func (s *DixonColesService) CalculateParamsWithVenue(homeTeam, awayTeam, venue string) models.DixonColesParams {
	p := s.CalculateParams(homeTeam, awayTeam)

	// 判定场馆国家
	venueCountry := "United States" // 默认美国
	vLower := strings.ToLower(venue)
	if strings.Contains(vLower, "azteca") || strings.Contains(vLower, "akron") || strings.Contains(vLower, "bbva") {
		venueCountry = "Mexico"
	} else if strings.Contains(vLower, "bc place") || strings.Contains(vLower, "bmo") {
		venueCountry = "Canada"
	}

	homeAdv := 0.0
	// 只有东道主在自己国家踢球时，该东道主才享有主场优势
	if isHostNation(homeTeam) && homeTeam == venueCountry {
		homeAdv = 0.18
	} else if isHostNation(awayTeam) && awayTeam == venueCountry {
		homeAdv = -0.18 // 客队是东道主且在本国比赛，客队享有主场优势
	}

	if homeAdv != 0.0 {
		p.LambdaHome = p.LambdaHome * math.Exp(homeAdv)
		p.LambdaAway = p.LambdaAway * math.Exp(-homeAdv)

		// 限制 Lambda 期望在 2.8 安全上限内
		if p.LambdaHome > 2.8 {
			p.LambdaHome = 2.8
		}
		if p.LambdaAway > 2.8 {
			p.LambdaAway = 2.8
		}
	}

	return p
}

// RecalculateLambdaOffset 根据所有已结算比赛的历史误差及 Brier Score 自适应纠偏 Lambda 进球期望
func (s *DixonColesService) RecalculateLambdaOffset() {
	reports, err := db.GetBacktestReports()
	if err != nil || len(reports) == 0 {
		s.lambdaHomeOffset = 0.0
		s.lambdaAwayOffset = 0.0
		return
	}

	homeOffset := 0.0
	awayOffset := 0.0
	learningRate := 0.05

	for _, r := range reports {
		m, err := db.GetMatch(r.MatchID)
		if err != nil {
			continue
		}

		pBase := s.CalculateParamsWithoutOffset(m.HomeTeam, m.AwayTeam)

		// 累加当前的偏移量以便在增量推演中使用最新的自适应状态
		lambdaH := pBase.LambdaHome + homeOffset
		lambdaA := pBase.LambdaAway + awayOffset
		if lambdaH < 0.2 {
			lambdaH = 0.2
		}
		if lambdaA < 0.2 {
			lambdaA = 0.2
		}

		actualH := float64(m.HomeScore)
		actualA := float64(m.AwayScore)

		// 误差反馈自校准公式：误差乘以 Brier Score 加权因子（2.0 - BrierScore）
		// 预测越精准的正常赛事其权重越高，大冷门爆冷局其权重趋向于 0，保护核心定量模型免受极端偶然误差的污染
		deltaH := learningRate * (actualH - lambdaH) * (2.0 - r.BrierScore)
		deltaA := learningRate * (actualA - lambdaA) * (2.0 - r.BrierScore)

		homeOffset += deltaH
		awayOffset += deltaA
	}

	// 限制在 [-0.20, 0.20] 的自校准偏置裁剪阈值内，防范预测参数发散
	if homeOffset > 0.20 {
		homeOffset = 0.20
	}
	if homeOffset < -0.20 {
		homeOffset = -0.20
	}
	if awayOffset > 0.20 {
		awayOffset = 0.20
	}
	if awayOffset < -0.20 {
		awayOffset = -0.20
	}

	s.lambdaHomeOffset = homeOffset
	s.lambdaAwayOffset = awayOffset
}

// OptimizeParameters 执行已完赛比赛数据与预测概率的网格搜索，寻找最优预测参数并持久化与热更新
func (s *DixonColesService) OptimizeParameters() (float64, float64, float64, float64, float64, error) {
	// 1. 获取所有已完赛的比赛
	rows, err := db.DB.Query("SELECT id, home_team, away_team, home_score, away_score, venue, scheduled_at FROM matches WHERE status = 'FT' ORDER BY scheduled_at ASC")
	if err != nil {
		return 0, 0, 0, 0, 0, err
	}
	defer rows.Close()

	var matches []models.Match
	for rows.Next() {
		var m models.Match
		var schedStr string
		err := rows.Scan(&m.ID, &m.HomeTeam, &m.AwayTeam, &m.HomeScore, &m.AwayScore, &m.Venue, &schedStr)
		if err == nil {
			m.Status = "FT"
			m.ScheduledAt, _ = time.Parse(time.RFC3339, schedStr)
			if m.ScheduledAt.IsZero() {
				m.ScheduledAt, _ = time.Parse("2006-01-02 15:04:05", schedStr)
			}
			matches = append(matches, m)
		}
	}

	if len(matches) == 0 {
		return 0, 0, 0, 0, 0, fmt.Errorf("未找到任何已完赛比赛，无法执行参数反省优化")
	}

	bestBrier := 999.0
	var bestNormDiv, bestDiffMult, bestH2hMult, bestRho float64

	normDivs := []float64{0.95, 1.00, 1.05, 1.10}
	diffMults := []float64{0.20, 0.25, 0.30, 0.35, 0.40, 0.45}
	h2hMults := []float64{0.05, 0.10, 0.15, 0.20}
	rhos := []float64{-0.03, -0.05, -0.08, -0.10, -0.12}

	for _, nd := range normDivs {
		for _, dm := range diffMults {
			for _, hm := range h2hMults {
				for _, r := range rhos {
					// 备份当前参数并临时修改以跑仿真
					originalNd, originalDm, originalHm, originalR := s.NormDivulator, s.DiffMultiplier, s.H2hMultiplier, s.InitialRho
					s.NormDivulator, s.DiffMultiplier, s.H2hMultiplier, s.InitialRho = nd, dm, hm, r

					var totalBrier float64
					for _, m := range matches {
						params := s.CalculateParamsWithVenue(m.HomeTeam, m.AwayTeam, m.Venue)
						matrix, _, _ := s.GenerateProbabilityMatrix(params)
						var pH, pD, pA float64
						for _, cell := range matrix {
							if cell.HomeScore > cell.AwayScore {
								pH += cell.Prob
							} else if cell.HomeScore == cell.AwayScore {
								pD += cell.Prob
							} else {
								pA += cell.Prob
							}
						}

						var oH, oD, oA float64
						if m.HomeScore > m.AwayScore {
							oH = 1.0
						} else if m.HomeScore == m.AwayScore {
							oD = 1.0
						} else {
							oA = 1.0
						}
						totalBrier += math.Pow(pH-oH, 2) + math.Pow(pD-oD, 2) + math.Pow(pA-oA, 2)
					}

					avgBrier := totalBrier / float64(len(matches))
					if avgBrier < bestBrier {
						bestBrier = avgBrier
						bestNormDiv = nd
						bestDiffMult = dm
						bestH2hMult = hm
						bestRho = r
					}

					// 恢复原参数
					s.NormDivulator, s.DiffMultiplier, s.H2hMultiplier, s.InitialRho = originalNd, originalDm, originalHm, originalR
				}
			}
		}
	}

	// 2. 将最优参数保存入库
	_ = db.SaveSystemParam("NormDivulator", bestNormDiv)
	_ = db.SaveSystemParam("DiffMultiplier", bestDiffMult)
	_ = db.SaveSystemParam("H2hMultiplier", bestH2hMult)
	_ = db.SaveSystemParam("InitialRho", bestRho)

	// 3. 热更新当前内存中的参数，使其立即在主业务中生效
	s.NormDivulator = bestNormDiv
	s.DiffMultiplier = bestDiffMult
	s.H2hMultiplier = bestH2hMult
	s.InitialRho = bestRho

	// 重新执行学习率修正逻辑
	s.RecalculateRhoOffset()
	s.RecalculateLambdaOffset()

	return bestNormDiv, bestDiffMult, bestH2hMult, bestRho, bestBrier, nil
}
