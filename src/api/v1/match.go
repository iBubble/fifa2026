package v1

import (
	"encoding/json"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"fifa2026/src/internal/service/prediction"
	"fifa2026/src/internal/service/scheduler"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var underReviewMatches sync.Map

// GetMatches 获取赛事列表，并异步触发比分同步与完赛场次的复盘
func (ctrl *APIController) GetMatches(c *gin.Context) {
	// 每天第一次页面被访问时，异步触发比分同步与蒙特卡洛推演
	go func() {
		today := time.Now().Format("2006-01-02")
		lastDate, found, err := db.GetSystemConfig("LastSimulatedDate")
		if err != nil {
			return
		}
		if !found || lastDate != today {
			_ = db.SaveSystemConfig("LastSimulatedDate", today)
			log.Printf("[MonteCarlo] 🎲 今日首次访问，向调度器提交同步与蒙特卡洛推演任务...")
			scheduler.GetGlobalScheduler().Submit("MonteCarlo_Sync_And_Simulation", func() error {
				wcSync := prediction.NewWorldCup26SyncService()
				if _, errSync := wcSync.SyncFinishedMatches(); errSync != nil {
					log.Printf("[MonteCarlo] ⚠️ 模拟前同步完赛比分异常: %v", errSync)
				}
				runAndCacheMonteCarlo(ctrl.MCSimulator)
				return nil
			})
		}
	}()

	hasSporttery := false
	initialMatches, errInit := db.GetMatchesByTournament("fifa_2026")
	if errInit == nil {
		for _, m := range initialMatches {
			if strings.HasPrefix(m.ID, "sporttery_") {
				hasSporttery = true
				break
			}
		}
	}

	if !hasSporttery {
		log.Println("[Server] 数据库尚未缓存竞彩数据，执行同步拉取...")
		ctrl.SportteryService.FetchAllOdds()
	} else {
		go ctrl.SportteryService.FetchAllOdds()
	}

	go ctrl.LiveSyncService.SyncMatches()

	matches, err := db.GetMatchesByTournament("fifa_2026")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	seen := make(map[string]models.Match)
	for _, m := range matches {
		key := fmt.Sprintf("%s_%s_%d", m.HomeTeam, m.AwayTeam, m.ScheduledAt.Unix())
		existing, ok := seen[key]
		if !ok {
			seen[key] = m
			continue
		}
		if strings.HasPrefix(m.ID, "sporttery_") && !strings.HasPrefix(existing.ID, "sporttery_") {
			seen[key] = m
		}
	}

	var uniqueMatches []models.Match
	for _, m := range seen {
		uniqueMatches = append(uniqueMatches, m)
	}
	sort.Slice(uniqueMatches, func(i, j int) bool {
		return uniqueMatches[i].ScheduledAt.Before(uniqueMatches[j].ScheduledAt)
	})
	matches = uniqueMatches

	for _, m := range matches {
		if m.Status == "FT" {
			rep, errReview := db.GetBacktestReport(m.ID)
			if errReview != nil || rep.TacticsReview == "" || strings.Contains(rep.TacticsReview, "超时降级") {
				if _, loading := underReviewMatches.LoadOrStore(m.ID, true); !loading {
					log.Printf("[Server] 检测到比赛 %s 尚未复盘，提交至调度器排队复盘...", m.ID)
					matchToReview := m
					scheduler.GetGlobalScheduler().Submit("ReviewMatch_"+matchToReview.ID, func() error {
						defer underReviewMatches.Delete(matchToReview.ID)
						params := ctrl.DCService.CalculateParams(matchToReview.HomeTeam, matchToReview.AwayTeam)
						matrix, over25, under25 := ctrl.DCService.GenerateProbabilityMatrixWithTeams(params, matchToReview.HomeTeam, matchToReview.AwayTeam)
						r := models.PredictionReport{
							MatchID:        matchToReview.ID,
							OriginalParams: params,
							RefinedParams:  params,
							ScoreMatrix:    matrix,
							Over2_5Prob:    over25,
							Under2_5Prob:   under25,
						}
						res, err := ctrl.BacktestService.ReviewMatch(matchToReview, &r)
						if err != nil {
							log.Printf("[Server] ❌ 比赛 %s 异步复盘失败: %v", matchToReview.ID, err)
							return err
						}
						log.Printf("[Server] ✅ 比赛 %s 异步复盘成功: %s", matchToReview.ID, res.TacticsReview)
						return nil
					})
				}
			}
		}
	}
	c.JSON(http.StatusOK, matches)
}

// StreamMatches 推送实时赛程比分的 SSE 长连接通道
func (ctrl *APIController) StreamMatches(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	c.SSEvent("open", "connected")
	c.Writer.Flush()

	ch := ctrl.LiveSyncService.RegisterListener()
	defer ctrl.LiveSyncService.RemoveListener(ch)

	clientGone := c.Request.Context().Done()
	c.Stream(func(w io.Writer) bool {
		select {
		case <-clientGone:
			return false
		case msg, ok := <-ch:
			if !ok {
				return false
			}
			c.SSEvent("message", msg)
			return true
		}
	})
}

// runAndCacheMonteCarlo 将蒙特卡洛全赛事模拟运行一次并序列化存入系统参数缓存中
func runAndCacheMonteCarlo(mcSimulator *prediction.MonteCarloSimulator) {
	log.Println("[MonteCarlo Job] 🎲 开始运行蒙特卡洛全量赛事模拟推演并刷新缓存...")
	fileData, err := os.ReadFile("./data/seasons/fifa_2026.json")
	if err != nil {
		log.Printf("[MonteCarlo Job] ❌ 读取世界杯分组配置失败: %v", err)
		return
	}
	var rawSeason struct {
		Groups map[string][]string `json:"groups"`
	}
	if err := json.Unmarshal(fileData, &rawSeason); err != nil {
		log.Printf("[MonteCarlo Job] ❌ 解析分组配置失败: %v", err)
		return
	}

	results := mcSimulator.SimulateTournament(rawSeason.Groups, 10000)
	resultsBytes, errMarshal := json.Marshal(results)
	if errMarshal != nil {
		log.Printf("[MonteCarlo Job] ❌ 序列化模拟结果失败: %v", errMarshal)
		return
	}

	errSave := db.SaveSystemConfig("montecarlo_results", string(resultsBytes))
	if errSave != nil {
		log.Printf("[MonteCarlo Job] ❌ 保存模拟结果至系统配置表失败: %v", errSave)
	} else {
		log.Println("[MonteCarlo Job] ✅ 蒙特卡洛模拟完成并成功缓存")
	}
}
