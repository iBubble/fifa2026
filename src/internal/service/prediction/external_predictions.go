package prediction

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// ExternalPrediction 外部模型的胜平负概率预测
type ExternalPrediction struct {
	Source   string  `json:"source"`
	HomeWin float64 `json:"homeWin"` // 主胜概率 0~1
	Draw    float64 `json:"draw"`    // 平局概率
	AwayWin float64 `json:"awayWin"` // 客胜概率
}

// ExternalPredictionService 聚合外部预测模型共识
type ExternalPredictionService struct {
	client *http.Client
	cache  map[string]extCacheEntry
	mu     sync.RWMutex
}

type extCacheEntry struct {
	preds     []ExternalPrediction
	fetchedAt time.Time
}

func NewExternalPredictionService() *ExternalPredictionService {
	return &ExternalPredictionService{
		client: &http.Client{Timeout: 8 * time.Second},
		cache:  make(map[string]extCacheEntry),
	}
}

// GetExternalConsensus 获取外部模型的共识预测
// 静默降级：任何外部源不可达时不影响本地预测
func (s *ExternalPredictionService) GetExternalConsensus(homeTeam, awayTeam string) []ExternalPrediction {
	cacheKey := fmt.Sprintf("%s_vs_%s", homeTeam, awayTeam)

	s.mu.RLock()
	if entry, ok := s.cache[cacheKey]; ok && time.Since(entry.fetchedAt) < 10*time.Minute {
		s.mu.RUnlock()
		return entry.preds
	}
	s.mu.RUnlock()

	var preds []ExternalPrediction

	// Hicruben Dixon-Coles 预测
	if p := s.fetchHicruben(homeTeam, awayTeam); p != nil {
		preds = append(preds, *p)
	}

	s.mu.Lock()
	s.cache[cacheKey] = extCacheEntry{preds: preds, fetchedAt: time.Now()}
	s.mu.Unlock()

	return preds
}

// fetchHicruben 尝试从 cup26matches.com 获取预测
func (s *ExternalPredictionService) fetchHicruben(homeTeam, awayTeam string) *ExternalPrediction {
	url := fmt.Sprintf("https://cup26matches.com/api/predict?home=%s&away=%s", homeTeam, awayTeam)

	resp, err := s.client.Get(url)
	if err != nil {
		log.Printf("[ExtPredict] ⚠️ Hicruben 不可达，静默降级: %v", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var result struct {
		HomeWin float64 `json:"home_win"`
		Draw    float64 `json:"draw"`
		AwayWin float64 `json:"away_win"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil
	}

	// 校验概率合理性
	total := result.HomeWin + result.Draw + result.AwayWin
	if total < 0.9 || total > 1.1 {
		return nil
	}

	return &ExternalPrediction{
		Source:  "Hicruben",
		HomeWin: result.HomeWin,
		Draw:    result.Draw,
		AwayWin: result.AwayWin,
	}
}

// BuildExternalConsensusSummary 生成外部共识摘要（用于 LLM Prompt）
func (s *ExternalPredictionService) BuildExternalConsensusSummary(homeTeam, awayTeam string) string {
	preds := s.GetExternalConsensus(homeTeam, awayTeam)
	if len(preds) == 0 {
		return ""
	}

	summary := "【外部模型共识】\n"
	for _, p := range preds {
		summary += fmt.Sprintf("- %s: 主胜%.1f%% 平%.1f%% 客胜%.1f%%\n",
			p.Source, p.HomeWin*100, p.Draw*100, p.AwayWin*100)
	}
	return summary
}
