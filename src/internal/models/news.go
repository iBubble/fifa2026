package models

import "time"

// NewsArticle 全球各大权威体育媒体最新的实时资讯
type NewsArticle struct {
	Title      string    `json:"title"`
	Summary    string    `json:"summary"`
	SourceURL  string    `json:"sourceUrl"`
	Time       time.Time `json:"time"`
	SourceSite string    `json:"sourceSite"`
}
