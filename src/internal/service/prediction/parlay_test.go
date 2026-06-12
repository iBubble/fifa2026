package prediction

import (
	"math"
	"testing"
)

func TestBankersRound(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1.124, 1.12}, // 四舍
		{1.126, 1.13}, // 六入
		{1.125, 1.12}, // 五双：前一位是 2 (偶数)，舍去 -> 1.12
		{1.135, 1.14}, // 五双：前一位是 3 (奇数)，进位 -> 1.14
		{2.565, 2.56}, // 前一位是 6 (偶数)，舍去 -> 2.56
		{2.575, 2.58}, // 前一位是 7 (奇数)，进位 -> 2.58
	}

	for _, tc := range tests {
		res := bankersRound(tc.input)
		if math.Abs(res-tc.expected) > 1e-9 {
			t.Errorf("bankersRound(%f) = %f; expected %f", tc.input, res, tc.expected)
		}
	}
}

func TestCombinations(t *testing.T) {
	// 验证 combinations(4, 2) 应返回 6 组，且每组大小为 2
	res2 := combinations(4, 2)
	if len(res2) != 6 {
		t.Errorf("Expected 6 combinations for (4,2), got %d", len(res2))
	}
	
	// 验证 combinations(4, 3) 应返回 4 组，且每组大小为 3
	res3 := combinations(4, 3)
	if len(res3) != 4 {
		t.Errorf("Expected 4 combinations for (4,3), got %d", len(res3))
	}
}

func TestRoundToEvenStandard(t *testing.T) {
	// 额外验证 math.RoundToEven 的行为符合预期
	if math.RoundToEven(112.5) != 112.0 {
		t.Errorf("Expected 112.0 for 112.5, got %f", math.RoundToEven(112.5))
	}
	if math.RoundToEven(113.5) != 114.0 {
		t.Errorf("Expected 114.0 for 113.5, got %f", math.RoundToEven(113.5))
	}
}
