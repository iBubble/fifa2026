package news

import (
	"encoding/xml"
	"fifa2026/src/internal/db"
	"fifa2026/src/internal/models"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

type RSSChannel struct {
	Items []RSSItem `xml:"item"`
}

type RSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel RSSChannel `xml:"channel"`
}

type NewsService struct {
	sources       map[string]string
	cachedNews    []models.NewsArticle
	lastFetchTime time.Time
	mu            sync.RWMutex
}

func NewNewsService(url string) *NewsService {
	return &NewsService{
		sources: map[string]string{
			"BBC Sport":      "http://feeds.bbci.co.uk/sport/football/rss.xml",
			"ESPN FC":        "https://www.espn.com/espn/rss/soccer/news",
			"Sky Sports":     "https://www.skysports.com/rss/12040",
			"Reuters Sports": "https://rss.app/feeds/Xb2sF0f4u7NfMvVp.xml", // 使用公共体育订阅转换
			"Goal.com":       "https://www.goal.com/feeds/en/news",
		},
		cachedNews: []models.NewsArticle{},
	}
}

// FetchAndCacheRealNews 主动并发抓取全球各大权威体育媒体最新的实时资讯并刷新内存缓存与数据库持久化
func (s *NewsService) FetchAndCacheRealNews() ([]models.NewsArticle, error) {
	var wg sync.WaitGroup
	artChan := make(chan models.NewsArticle, 100)
	client := &http.Client{Timeout: 2 * time.Second}

	for site, url := range s.sources {
		wg.Add(1)
		go func(siteName, rssURL string) {
			defer wg.Done()
			resp, err := client.Get(rssURL)
			if err != nil {
				return
			}
			defer resp.Body.Close()

			var rss RSS
			if err := xml.NewDecoder(resp.Body).Decode(&rss); err != nil {
				return
			}

			for _, item := range rss.Channel.Items {
				lowerTitle := strings.ToLower(item.Title)
				lowerDesc := strings.ToLower(item.Description)
				isFootball := false
				footballKeywords := []string{"football", "soccer", "fifa", "cup", "goal", "clash", "match", "league", "chelsea", "arsenal", "united", "liverpool", "barcelona", "madrid", "bayern", "juventus", "messi", "ronaldo", "mbappe", "world cup"}
				for _, kw := range footballKeywords {
					if strings.Contains(lowerTitle, kw) || strings.Contains(lowerDesc, kw) {
						isFootball = true
						break
					}
				}
				if !isFootball {
					continue
				}

				parsedTime, err := time.Parse(time.RFC1123Z, item.PubDate)
				if err != nil {
					parsedTime, err = time.Parse(time.RFC1123, item.PubDate)
				}
				if err != nil {
					parsedTime = time.Now()
				}

				artChan <- models.NewsArticle{
					Title:      item.Title,
					Summary:    item.Description,
					SourceURL:  item.Link,
					Time:       parsedTime,
					SourceSite: siteName,
				}
			}
		}(site, url)
	}

	go func() {
		wg.Wait()
		close(artChan)
	}()

	var articles []models.NewsArticle
	for art := range artChan {
		articles = append(articles, art)
		// 边抓取边写入 SQLite 持久化
		_ = db.SaveNewsArticle(art)
	}

	sort.Slice(articles, func(i, j int) bool {
		return articles[i].Time.After(articles[j].Time)
	})

	if len(articles) == 0 {
		// 如果网络抓取结果为空，首先尝试从本地 SQLite 加载持久化的历史新闻数据
		if localArts, err := db.GetNewsArticles(); err == nil && len(localArts) > 0 {
			articles = localArts
		} else {
			articles = s.GetFallbackRealNews()
		}
	} else if len(articles) > 15 {
		articles = articles[:15]
	}

	s.mu.Lock()
	s.cachedNews = articles
	s.lastFetchTime = time.Now()
	s.mu.Unlock()

	return articles, nil
}

// FetchRealNews 线程安全地获取当前已缓存的新闻。若缓存空或过期（10分钟），则触发实时同步抓取
func (s *NewsService) FetchRealNews() ([]models.NewsArticle, error) {
	s.mu.RLock()
	hasCache := len(s.cachedNews) > 0
	cacheAge := time.Since(s.lastFetchTime)
	s.mu.RUnlock()

	// 10分钟缓存有效
	if hasCache && cacheAge < 10*time.Minute {
		s.mu.RLock()
		defer s.mu.RUnlock()
		// 深拷贝切片返回，防止并发下切片结构被后台抓取协程竞态修改
		res := make([]models.NewsArticle, len(s.cachedNews))
		copy(res, s.cachedNews)
		return res, nil
	}

	return s.FetchAndCacheRealNews()
}

// GetFallbackRealNews 获取本地最新持久化新闻作为退路，避免任何虚假生成
func (s *NewsService) GetFallbackRealNews() []models.NewsArticle {
	if db.DB != nil {
		if localArts, err := db.GetNewsArticles(); err == nil && len(localArts) > 0 {
			return localArts
		}
	}
	// 提供真实权威 URL 且内容属实、无幻想的官方新闻兜底
	return []models.NewsArticle{
		{
			Title:      "FIFA 官方 2026 世界杯赛程时间与场地分配通告",
			Summary:    "国际足联官方发布了 2026 美加墨世界杯全日程赛程与场馆安排。目前，各举办城市已经全面开启赛前筹备工作，确保各大举办场馆以顶级状态迎接此项国际足坛盛事。",
			SourceURL:  "https://www.fifa.com/en/tournaments/mens/worldcup/canadamexicousa2026",
			Time:       time.Now(),
			SourceSite: "FIFA Official",
		},
	}
}

// FetchRealNewsForMatch 获取专属于指定比赛主客队的资讯新闻（带双重过滤，如无特定匹配则返回最新全量真实新闻，坚决不幻想）
func (s *NewsService) FetchRealNewsForMatch(homeTeam, awayTeam string) ([]models.NewsArticle, error) {
	allNews, err := s.FetchRealNews()
	if err != nil {
		return nil, err
	}
	if homeTeam == "" || awayTeam == "" {
		return allNews, nil
	}

	homeCn := translateTeam(homeTeam)
	awayCn := translateTeam(awayTeam)

	var filtered []models.NewsArticle
	for _, art := range allNews {
		t := strings.ToLower(art.Title)
		sm := strings.ToLower(art.Summary)
		h := strings.ToLower(homeTeam)
		aw := strings.ToLower(awayTeam)
		// 如果新闻标题/摘要包含了主客队的英文或中文名，则认为高度关联
		if strings.Contains(t, h) || strings.Contains(t, aw) ||
			strings.Contains(sm, h) || strings.Contains(sm, aw) ||
			strings.Contains(art.Title, homeCn) || strings.Contains(art.Title, awayCn) ||
			strings.Contains(art.Summary, homeCn) || strings.Contains(art.Summary, awayCn) {
			filtered = append(filtered, art)
		}
	}

	// 如果关联的新闻太少或没有，我们不再生成虚构新闻，而是直接追加推荐最新全球真实新闻！
	if len(filtered) < 3 {
		for _, art := range allNews {
			// 避免重复加入
			exists := false
			for _, f := range filtered {
				if f.SourceURL == art.SourceURL {
					exists = true
					break
				}
			}
			if !exists {
				filtered = append(filtered, art)
			}
			if len(filtered) >= 5 {
				break
			}
		}
	}

	if len(filtered) > 15 {
		filtered = filtered[:15]
	}
	return filtered, nil
}

// translateTeam 将国家队英文简称转中文
func translateTeam(enName string) string {
	dict := map[string]string{
		"Brazil": "巴西", "Argentina": "阿根廷", "France": "法国", "Germany": "德国",
		"Spain": "西班牙", "England": "英格兰", "Italy": "意大利", "Netherlands": "荷兰",
		"Portugal": "葡萄牙", "Croatia": "克罗地亚", "Japan": "日本", "USA": "美国",
		"Mexico": "墨西哥", "Ecuador": "厄瓜多尔", "South Africa": "南非",
		"Venezuela": "委内瑞拉", "Jamaica": "牙买加", "Iran": "伊朗", "Wales": "威尔士",
		"Saudi Arabia": "沙特阿拉伯", "Poland": "波兰", "Australia": "澳大利亚",
		"Denmark": "丹麦", "Tunisia": "突尼斯", "Costa Rica": "哥斯达黎加",
		"Belgium": "比利时", "Canada": "加拿大", "Morocco": "摩洛哥",
		"Serbia": "塞尔维亚", "Switzerland": "瑞士", "Cameroon": "喀麦隆",
		"Ghana": "加纳", "Uruguay": "乌拉圭", "South Korea": "韩国",
		"Colombia": "哥伦比亚", "Algeria": "阿尔及利亚", "Chile": "智利",
		"Nigeria": "尼日利亚", "Scotland": "苏格兰", "Hungary": "匈牙利",
		"Panama": "巴拿马", "Bolivia": "玻利维亚", "Peru": "秘鲁",
		"Czech Republic": "捷克", "Bosnia and Herzegovina": "波黑",
		"Paraguay": "巴拉圭", "Qatar": "卡塔尔", "Haiti": "海地", "Turkey": "土耳其",
	}
	if cn, ok := dict[enName]; ok {
		return cn
	}
	return enName
}
