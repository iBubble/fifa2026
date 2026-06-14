package ai

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// WebSearch 免 Key 获取网页搜索摘要
func WebSearch(query string) (string, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(query))
	
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", err
	}
	
	// 设置 User-Agent 伪装浏览器防止拦截
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	client := &http.Client{Timeout: 12*time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("全网搜索请求超时: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("全网搜索响应异常状态码: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	html := string(bodyBytes)

	// 用简单的正则或子串截取提取前三条记录
	// 结果的 HTML 格式大致为：
	// <a class="result__snippet" href="LINK">TITLE</a>
	// 我们用正则模糊匹配结果项
	reResult := regexp.MustCompile(`(?s)<div class="result__body">.*?<a class="result__snippet".*?href="([^"]+)"[^>]*>(.*?)</a>.*?<a class="result__snippet"[^>]*>(.*?)</a>`)
	matches := reResult.FindAllStringSubmatch(html, 3)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- 关于 '%s' 的全网搜索结果 ---", query))
	
	if len(matches) == 0 {
		// 备用正则，防范 DuckDuckGo 布局微调
		reBackup := regexp.MustCompile(`(?s)<a class="result__url"[^>]*href="([^"]+)"[^>]*>(.*?)</a>`)
		matchesBackup := reBackup.FindAllStringSubmatch(html, 3)
		if len(matchesBackup) > 0 {
			for idx, m := range matchesBackup {
				link := m[1]
				if strings.Contains(link, "uddg=") {
					parts := strings.Split(link, "uddg=")
					if len(parts) > 1 {
						link = parts[1]
						if decoded, errDec := url.QueryUnescape(link); errDec == nil {
							link = decoded
						}
					}
				}
				title := cleanHTMLTags(m[2])
				sb.WriteString(fmt.Sprintf("\n[%d] %s\n链接: %s\n", idx+1, title, link))
			}
			return sb.String(), nil
		}
		
		return fmt.Sprintf("未检索到与 '%s' 相关的外网资讯，建议使用本地数据分析。", query), nil
	}

	for idx, m := range matches {
		link := m[1]
		if strings.Contains(link, "uddg=") {
			parts := strings.Split(link, "uddg=")
			if len(parts) > 1 {
				link = parts[1]
				if decoded, errDec := url.QueryUnescape(link); errDec == nil {
					link = decoded
				}
			}
		}
		title := cleanHTMLTags(m[2])
		snippet := cleanHTMLTags(m[3])
		sb.WriteString(fmt.Sprintf("\n[%d] 标题: %s\n摘要: %s\n链接: %s\n", idx+1, title, snippet, link))
	}

	return sb.String(), nil
}

// cleanHTMLTags 辅助清除 HTML 标签与空白
func cleanHTMLTags(src string) string {
	re := regexp.MustCompile(`<[^>]*>`)
	dst := re.ReplaceAllString(src, "")
	dst = strings.ReplaceAll(dst, "\n", "")
	dst = strings.ReplaceAll(dst, "\r", "")
	dst = strings.TrimSpace(dst)
	
	// 解码常见字符实体
	dst = strings.ReplaceAll(dst, "&amp;", "&")
	dst = strings.ReplaceAll(dst, "&lt;", "<")
	dst = strings.ReplaceAll(dst, "&gt;", ">")
	dst = strings.ReplaceAll(dst, "&quot;", "\"")
	dst = strings.ReplaceAll(dst, "&#x27;", "'")
	
	return dst
}
