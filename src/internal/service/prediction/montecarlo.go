package prediction

import (
	"fifa2026/src/internal/models"
	"math/rand"
	"sort"
	"sync"
	"time"
)

type MonteCarloSimulator struct {
	dc  *DixonColesService
	elo *EloService
}

func NewMonteCarloSimulator(dc *DixonColesService, elo *EloService) *MonteCarloSimulator {
	rand.Seed(time.Now().UnixNano())
	return &MonteCarloSimulator{dc: dc, elo: elo}
}

// SimulateTournament 运行 N 次世界杯模拟，返回 48 支球队各轮晋级与夺冠期望
func (s *MonteCarloSimulator) SimulateTournament(groups map[string][]string, iterations int) []models.SimulationResult {
	// 初始化每队计数器
	stats := make(map[string]*models.SimulationResult)
	for _, teamList := range groups {
		for _, name := range teamList {
			stats[name] = &models.SimulationResult{TeamName: name}
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	concurrency := 8
	chunkSize := iterations / concurrency

	for c := 0; c < concurrency; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localStats := make(map[string]*models.SimulationResult)
			for name := range stats {
				localStats[name] = &models.SimulationResult{TeamName: name}
			}

			for i := 0; i < chunkSize; i++ {
				// 每次单场模拟前对分组列表进行深拷贝，保障多线程排序独立安全
				localGroups := make(map[string][]string)
				for k, v := range groups {
					temp := make([]string, len(v))
					copy(temp, v)
					localGroups[k] = temp
				}
				s.runSingleSimulation(localGroups, localStats)
			}

			mu.Lock()
			for name, res := range localStats {
				stats[name].GroupOutProb += res.GroupOutProb
				stats[name].Round16Prob += res.Round16Prob
				stats[name].QuarterProb += res.QuarterProb
				stats[name].SemiProb += res.SemiProb
				stats[name].FinalProb += res.FinalProb
				stats[name].WinnerProb += res.WinnerProb
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	// 折算为概率百分比
	var results []models.SimulationResult
	for _, res := range stats {
		res.GroupOutProb = (res.GroupOutProb / float64(iterations)) * 100
		res.Round16Prob = (res.Round16Prob / float64(iterations)) * 100
		res.QuarterProb = (res.QuarterProb / float64(iterations)) * 100
		res.SemiProb = (res.SemiProb / float64(iterations)) * 100
		res.FinalProb = (res.FinalProb / float64(iterations)) * 100
		res.WinnerProb = (res.WinnerProb / float64(iterations)) * 100
		results = append(results, *res)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].WinnerProb > results[j].WinnerProb
	})
	return results
}

// runSingleSimulation 运行单次世界杯推演流程
func (s *MonteCarloSimulator) runSingleSimulation(groups map[string][]string, localStats map[string]*models.SimulationResult) {
	var qualified32 []string

	// 1. 模拟小组赛
	var thirdPlaces []struct {
		Team   string
		Points int
	}
	for _, teams := range groups {
		if len(teams) < 4 {
			continue
		}
		pts := make(map[string]int)
		for _, t := range teams {
			pts[t] = 0
		}

		// 组内双循环或单循环 (此处模拟单循环 6 场)
		s.simulateMatchResult(teams[0], teams[1], pts)
		s.simulateMatchResult(teams[2], teams[3], pts)
		s.simulateMatchResult(teams[0], teams[2], pts)
		s.simulateMatchResult(teams[1], teams[3], pts)
		s.simulateMatchResult(teams[0], teams[3], pts)
		s.simulateMatchResult(teams[1], teams[2], pts)

		// 排序
		sort.Slice(teams, func(i, j int) bool { return pts[teams[i]] > pts[teams[j]] })
		qualified32 = append(qualified32, teams[0], teams[1]) // 每组前两名晋级
		thirdPlaces = append(thirdPlaces, struct {
			Team   string
			Points int
		}{Team: teams[2], Points: pts[teams[2]]})
	}

	// 成绩最好的 8 个小组第三晋级 32 强
	sort.Slice(thirdPlaces, func(i, j int) bool { return thirdPlaces[i].Points > thirdPlaces[j].Points })
	for i := 0; i < 8 && i < len(thirdPlaces); i++ {
		qualified32 = append(qualified32, thirdPlaces[i].Team)
	}

	for _, team := range qualified32 {
		localStats[team].GroupOutProb++
	}

	// 2. 模拟淘汰赛 (32强 -> 16强 -> 8强 -> 4强 -> 决赛 -> 冠军)
	r16 := s.simulateKnockoutRound(qualified32)
	for _, team := range r16 {
		localStats[team].Round16Prob++
	}

	r8 := s.simulateKnockoutRound(r16)
	for _, team := range r8 {
		localStats[team].QuarterProb++
	}

	r4 := s.simulateKnockoutRound(r8)
	for _, team := range r4 {
		localStats[team].SemiProb++
	}

	r2 := s.simulateKnockoutRound(r4)
	for _, team := range r2 {
		localStats[team].FinalProb++
	}

	winner := s.simulateKnockoutRound(r2)
	if len(winner) > 0 {
		localStats[winner[0]].WinnerProb++
	}
}

func (s *MonteCarloSimulator) simulateMatchResult(tA, tB string, pts map[string]int) {
	params := s.dc.CalculateParams(tA, tB)
	probWinA, probDraw, _ := s.calculateOutcomeProbs(params)
	r := rand.Float64()
	if r < probWinA {
		pts[tA] += 3
	} else if r < probWinA+probDraw {
		pts[tA] += 1
		pts[tB] += 1
	} else {
		pts[tB] += 3
	}
}

func (s *MonteCarloSimulator) calculateOutcomeProbs(p models.DixonColesParams) (winA, draw, winB float64) {
	for x := 0; x <= 5; x++ {
		for y := 0; y <= 5; y++ {
			prob := s.dc.ComputeJointProbability(x, y, p)
			if x > y {
				winA += prob
			} else if x == y {
				draw += prob
			} else {
				winB += prob
			}
		}
	}
	return
}

func (s *MonteCarloSimulator) simulateKnockoutRound(teams []string) []string {
	var winners []string
	for i := 0; i < len(teams)-1; i += 2 {
		tA := teams[i]
		tB := teams[i+1]
		params := s.dc.CalculateParams(tA, tB)
		winA, draw, _ := s.calculateOutcomeProbs(params)

		r := rand.Float64()
		if r < winA {
			winners = append(winners, tA)
		} else if r < winA+draw {
			// 平局点球决胜，以 Elo 实力定倾向 (点球有很大随机性，按期望55%~45%实力偏斜)
			probA := s.elo.CalculateExpectedWinProb(s.elo.GetElo(tA), s.elo.GetElo(tB))
			if rand.Float64() < (0.3 + 0.4*probA) {
				winners = append(winners, tA)
			} else {
				winners = append(winners, tB)
			}
		} else {
			winners = append(winners, tB)
		}
	}
	return winners
}
