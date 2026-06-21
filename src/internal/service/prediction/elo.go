package prediction

import (
	"encoding/json"
	"fifa2026/src/internal/models"
	"fmt"
	"math"
	"os"
	"sort"
	"sync"
)

type TeamElo struct {
	Name string  `json:"name"`
	Elo  float64 `json:"elo"`
}

type EloService struct {
	mu       sync.RWMutex
	teams    map[string]float64      // 球队名 -> Elo值
	features map[string]models.Team // 球队名 -> 完整特征数据
}

// NewEloService 从 history_features.json 载入初始 Elo 评分与完整的历史底蕴特征 (如进失球率)
func NewEloService(featuresPath string) (*EloService, error) {
	data, err := os.ReadFile(featuresPath)
	if err != nil {
		return nil, fmt.Errorf("读取历史底蕴特征文件失败: %w", err)
	}

	var rawFeatures struct {
		Teams map[string]models.Team `json:"teams"`
	}

	if err := json.Unmarshal(data, &rawFeatures); err != nil {
		return nil, fmt.Errorf("解析历史特征失败: %w", err)
	}

	teams := make(map[string]float64)
	features := make(map[string]models.Team)
	for name, team := range rawFeatures.Teams {
		// 回填英文标识符名
		team.Name = name
		teams[name] = team.InitialElo
		features[name] = team
	}

	return &EloService{
		teams:    teams,
		features: features,
	}, nil
}

// GetFeature 获取指定球队的完整特征数据 (如场均进失球、零封率)，若不存在则返回平滑中位数兜底
func (s *EloService) GetFeature(teamName string) models.Team {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if feat, ok := s.features[teamName]; ok {
		return feat
	}
	return models.Team{
		Name:             teamName,
		InitialElo:       1500.0,
		CurrentElo:       1500.0,
		AvgGoalsScored:   1.35, // 默认均势场均进球
		AvgGoalsConceded: 1.20, // 默认均势场均失球
		CleanSheetRate:   0.25,
	}
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

// GetEloRank 根据当前所有参赛队伍的实时 Elo 积分降序计算球队的“量化实力综合排名”
func (s *EloService) GetEloRank(teamName string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type teamScore struct {
		name string
		elo  float64
	}
	var list []teamScore
	for name, elo := range s.teams {
		list = append(list, teamScore{name: name, elo: elo})
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].elo > list[j].elo
	})

	for idx, ts := range list {
		if ts.name == teamName {
			return idx + 1 // 1-indexed 排名
		}
	}
	return len(list) + 1
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

// ResetToInitialElos 重置内存中所有球队的实时 Elo 评级到其冷启动初始 Elo 评分
func (s *EloService) ResetToInitialElos() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for name, feat := range s.features {
		s.teams[name] = feat.InitialElo
	}
}
