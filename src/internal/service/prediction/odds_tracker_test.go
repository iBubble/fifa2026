package prediction

import (
	"strings"
	"testing"
)

func TestOddsTrackerService_GetOddsShifts(t *testing.T) {
	service := NewOddsTrackerService()
	shifts := service.GetOddsShifts("Mexico", "South Africa")

	if len(shifts) != 3 {
		t.Fatalf("预期返回 3 个博彩巨头的赔率偏移，但实际返回了 %d 个", len(shifts))
	}

	shifts2 := service.GetOddsShifts("Canada", "Bosnia and Herzegovina")
	for _, s := range shifts2 {
		if s.MatchName != "加拿大 vs 波黑" {
			t.Errorf("预期 MatchName 为 '加拿大 vs 波黑'，但实际为 '%s'", s.MatchName)
		}
	}

	expectedBookmakers := map[string]bool{
		"Bet365":        true,
		"Pinnacle (平博)": true,
		"William Hill":  true,
	}

	for _, s := range shifts {
		if !expectedBookmakers[s.Bookmaker] {
			t.Errorf("未预期的博彩公司: %s", s.Bookmaker)
		}
		if s.MatchName == "" {
			t.Error("比赛名称不应为空")
		}
		if s.InitialOdds <= 0 || s.CurrentOdds <= 0 {
			t.Errorf("赔率初始值(%.2f) and 现值(%.2f) 均需大于0", s.InitialOdds, s.CurrentOdds)
		}
		if s.Direction != "UP" && s.Direction != "DOWN" {
			t.Errorf("未预期的赔率偏移方向: %s", s.Direction)
		}

		// 核心安全测试：检查链接中是否包含新浪的幻觉表达
		if strings.Contains(s.SourceURL, "sina.com.cn") {
			t.Errorf("发现不合规的新浪幻觉 URL: %s，所有博彩链接必须指向真实官方", s.SourceURL)
		}

		// 检查链接是否匹配真实的各博彩主站
		if s.Bookmaker == "Bet365" && s.SourceURL != "https://www.bet365.com/" {
			t.Errorf("Bet365 数据源不匹配: %s", s.SourceURL)
		}
		if s.Bookmaker == "Pinnacle (平博)" && s.SourceURL != "https://www.pinnacle.com/" {
			t.Errorf("Pinnacle 数据源不匹配: %s", s.SourceURL)
		}
		if s.Bookmaker == "William Hill" && s.SourceURL != "https://www.williamhill.com/" {
			t.Errorf("William Hill 数据源不匹配: %s", s.SourceURL)
		}
	}
}
