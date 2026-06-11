package prediction

import (
	"fmt"
	"math"
)

type ShinService struct{}

func NewShinService() *ShinService {
	return &ShinService{}
}

// DevigOdds 辛氏去抽水折算。输入胜、平、负（或大小球）赔率，输出无抽水真实共识概率
func (s *ShinService) DevigOdds(odds []float64) ([]float64, float64, error) {
	n := len(odds)
	if n < 2 {
		return nil, 0, fmt.Errorf("赔率选项少于2个，无法计算")
	}

	// 转换为隐含概率（带抽水） π_i = 1 / odds_i
	pi := make([]float64, n)
	sumPi := 0.0
	for i, o := range odds {
		if o <= 1.0 {
			return nil, 0, fmt.Errorf("非法赔率: %.2f", o)
		}
		pi[i] = 1.0 / o
		sumPi += pi[i]
	}

	// 若赔率本身未含抽水 (sumPi <= 1.0)，等比例归一化即可
	if sumPi <= 1.0 {
		norm := make([]float64, n)
		for i := range pi {
			norm[i] = pi[i] / sumPi
		}
		return norm, 0.0, nil
	}

	// 使用二分法数值迭代求解知情交易占比 z, z 范围在 [0, 1)
	low := 0.0
	high := 1.0 - 1e-9
	z := 0.0
	probs := make([]float64, n)

	for iter := 0; iter < 100; iter++ {
		mid := (low + high) / 2.0
		sumP := 0.0

		// Shin 求解公式: f(z) = ∑ p_i = 1
		// p_i = (sqrt(z^2 + 4*(1-z)*(pi_i^2 / sum(pi_k^2))) - z) / (2*(1-z))
		// 这里简化计算，利用 Shin 的二次逼近
		denom := 2.0 * (1.0 - mid)
		sumPiSq := 0.0
		for _, pVal := range pi {
			sumPiSq += pVal * pVal
		}

		for i := 0; i < n; i++ {
			term1 := mid * mid
			term2 := 4.0 * (1.0 - mid) * (pi[i] * pi[i]) / sumPiSq
			probs[i] = (math.Sqrt(term1+term2) - mid) / denom
			sumP += probs[i]
		}

		// 根据 f(z) 偏离度调整二分区间
		if math.Abs(sumP-1.0) < 1e-7 {
			z = mid
			break
		}
		if sumP > 1.0 {
			low = mid // 调大 z 以降低计算出的概率和
		} else {
			high = mid
		}
		z = mid
	}

	// 再次校准保证概率和严格为 1
	sumProbs := 0.0
	for _, p := range probs {
		sumProbs += p
	}
	for i := range probs {
		probs[i] /= sumProbs
	}

	return probs, z, nil
}
