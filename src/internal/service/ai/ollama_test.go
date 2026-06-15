package ai

import (
	"fifa2026/src/internal/models"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewOllamaService_TimeoutParsing(t *testing.T) {
	// 设置环境变量
	os.Setenv("OLLAMA_PREDICT_TIMEOUT", "8")
	os.Setenv("OLLAMA_REVIEW_TIMEOUT", "25")
	defer func() {
		os.Unsetenv("OLLAMA_PREDICT_TIMEOUT")
		os.Unsetenv("OLLAMA_REVIEW_TIMEOUT")
	}()

	s := NewOllamaService("http://localhost:11434", "qwen3.6:35b-q4")
	if s.predictTimeout != 8*time.Second {
		t.Errorf("expected predictTimeout 8s, got %v", s.predictTimeout)
	}
	if s.reviewTimeout != 25*time.Second {
		t.Errorf("expected reviewTimeout 25s, got %v", s.reviewTimeout)
	}
}

func TestOllamaService_TimeoutTriggered(t *testing.T) {
	// 启动一个模拟 of Ollama HTTP 服务，延迟 100ms 响应
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer ts.Close()

	// 1. 测试前台预测超时 (确保能优雅降级返回默认值且不报错)
	s := NewOllamaService(ts.URL, "qwen3.6:35b-q4")
	s.predictTimeout = 10 * time.Millisecond // 设置超短的超时，必定触发超时

	match := models.Match{
		ID:           "test_match",
		HomeTeam:     "TeamA",
		AwayTeam:     "TeamB",
		TournamentID: "test_tour",
	}
	p := models.DixonColesParams{
		LambdaHome: 1.2,
		LambdaAway: 0.8,
		Rho:        0.01,
	}

	offsets, err := s.RefineParams(match, 50, p, "some info")
	if err != nil {
		t.Errorf("unexpected error on timeout refine: %v", err)
	}
	if !strings.Contains(offsets.ProponentOpinion, "主队具备基础的定位期望优势") {
		t.Errorf("expected degrade proponent opinion, got: %s", offsets.ProponentOpinion)
	}

	// 2. 测试后台复盘超时降级 (确保能优雅生成智能降级复盘描述)
	s.reviewTimeout = 10 * time.Millisecond // 设置超短的超时，必定触发超时
	review, err := s.ReviewPrediction(match, 0.25, "tactics", 2, 1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(review, "赛事精算") || !strings.Contains(review, "主队险胜") {
		t.Errorf("expected degrade fallback review text, got: %s", review)
	}
}
