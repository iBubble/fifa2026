package prediction

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
)

type TeamElo struct {
	Name string  `json:"name"`
	Elo  float64 `json:"elo"`
}

type EloService struct {
	mu    sync.RWMutex
	teams map[string]float64 // 球队名 -> Elo值
}

// NewEloService 从 history_features.json 载入初始 Elo 评分
func NewEloService(featuresPath string) (*EloService, error) {
	data, err := os.ReadFile(featuresPath)
	if err != nil {
		return nil, fmt.Errorf("读取历史底蕴特征文件失败: %w", err)
	}

	var rawFeatures struct {
		Teams map[string]struct {
			InitialElo float64 `json:"initialElo"`
		} `json:"teams"`
	}

	if err := json.Unmarshal(data, &rawFeatures); err != nil {
		return nil, fmt.Errorf("解析历史特征失败: %w", err)
	}

	teams := make(map[string]float64)
	for name, details := range rawFeatures.Teams {
		teams[name] = details.InitialElo
	}

	return &EloService{
		teams: teams,
	}, nil
}

// GetElo 获取指定球队当前 Elo
func (s *EloService) GetElo(teamName string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if elo, ok := s.teams[teamName]; ok {
		return elo
	}
	return 1500.0 // 默认中位值
}

// CalculateExpectedWinProb 计算 A 队对 B 队的期望胜率倾向 (0~1)
func (s *EloService) CalculateExpectedWinProb(eloA, eloB float64) float64 {
	return 1.0 / (1.0 + math.Pow(10, (eloB-eloA)/400.0))
}

// UpdateElos 赛后根据实际赛果演化两队 Elo 积分 (K值世界杯默认为60)
func (s *EloService) UpdateElos(teamA, teamB string, scoreA, scoreB int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	eloA := s.teams[teamA]
	eloB := s.teams[teamB]
	if eloA == 0 {
		eloA = 1500.0
	}
	if eloB == 0 {
		eloB = 1500.0
	}

	expA := s.CalculateExpectedWinProb(eloA, eloB)
	expB := 1.0 - expA

	var actA, actB float64
	if scoreA > scoreB {
		actA = 1.0
		actB = 0.0
	} else if scoreA < scoreB {
		actA = 0.0
		actB = 1.0
	} else {
		actA = 0.5
		actB = 0.5
	}

	K := 60.0 // 世界杯加权值
	s.teams[teamA] = eloA + K*(actA-expA)
	s.teams[teamB] = eloB + K*(actB-expB)
}
