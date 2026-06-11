package db

import (
	"fifa2026/src/internal/models"
	"fmt"
	"time"
)

// SaveNewsArticle 插入或更新外部新闻
func SaveNewsArticle(art models.NewsArticle) error {
	if DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	query := `INSERT INTO news_articles (source_url, title, summary, publish_time, source_site)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(source_url) DO UPDATE SET
			title = excluded.title,
			summary = excluded.summary,
			publish_time = excluded.publish_time,
			source_site = excluded.source_site`
	_, err := DB.Exec(query, art.SourceURL, art.Title, art.Summary, art.Time.Format("2006-01-02 15:04:05"), art.SourceSite)
	return err
}

// GetNewsArticles 获取全部已缓存的持久化外部新闻
func GetNewsArticles() ([]models.NewsArticle, error) {
	if DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	query := `SELECT source_url, title, summary, publish_time, source_site
		FROM news_articles ORDER BY publish_time DESC LIMIT 50`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var articles []models.NewsArticle
	for rows.Next() {
		var art models.NewsArticle
		var timeStr string
		err := rows.Scan(&art.SourceURL, &art.Title, &art.Summary, &timeStr, &art.SourceSite)
		if err != nil {
			return nil, err
		}
		loc := time.FixedZone("CST", 8*3600)
		art.Time, _ = time.ParseInLocation("2006-01-02 15:04:05", timeStr, loc)
		if art.Time.IsZero() {
			art.Time, _ = time.ParseInLocation(time.RFC3339, timeStr, loc)
		}
		articles = append(articles, art)
	}
	return articles, nil
}
