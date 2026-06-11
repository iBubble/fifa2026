package news

import (
	"fifa2026/src/internal/models"
	"testing"
	"time"
)

func TestNewNewsService(t *testing.T) {
	ns := NewNewsService("")
	if ns == nil {
		t.Fatal("期望实例化 NewsService 成功，但返回了 nil")
	}
	if len(ns.sources) == 0 {
		t.Error("期望 sources 不为空，但长度为 0")
	}
}

func TestGetFallbackRealNews(t *testing.T) {
	ns := NewNewsService("")
	newsList := ns.GetFallbackRealNews()
	if len(newsList) == 0 {
		t.Fatal("期望 fallback 新闻不为空，但长度为 0")
	}

	// 验证文章是否具备必要属性
	for _, art := range newsList {
		if art.Title == "" {
			t.Error("新闻 Title 不应为空")
		}
		if art.SourceURL == "" {
			t.Error("新闻 SourceURL 不应为空")
		}
		if art.SourceSite == "" {
			t.Error("新闻 SourceSite 不应为空")
		}
	}
}

func TestFetchRealNewsCache(t *testing.T) {
	ns := NewNewsService("")

	// 1. 初始状态下缓存应为空
	if len(ns.cachedNews) != 0 {
		t.Error("初始状态下 cachedNews 应当为空")
	}

	// 2. 注入模拟数据并设置缓存时间
	mockNews := []models.NewsArticle{
		{
			Title:      "测试新闻标题",
			Summary:    "测试摘要内容",
			SourceURL:  "https://www.test.com",
			Time:       time.Now(),
			SourceSite: "TestSite",
		},
	}

	ns.mu.Lock()
	ns.cachedNews = mockNews
	ns.lastFetchTime = time.Now()
	ns.mu.Unlock()

	// 3. 读取缓存
	res, err := ns.FetchRealNews()
	if err != nil {
		t.Fatalf("读取新闻发生异常: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("预期返回 1 条缓存新闻，但实际返回了 %d 条", len(res))
	}
	if res[0].Title != "测试新闻标题" {
		t.Errorf("预期标题为 '测试新闻标题'，但实际为 '%s'", res[0].Title)
	}
}
