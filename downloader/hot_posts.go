package downloader

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// HotPost 东方财富股吧个股帖
type HotPost struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Author       string `json:"author"`
	ClickCount   int    `json:"clickCount"`
	CommentCount int    `json:"commentCount"`
	PublishTime  string `json:"publishTime"` // "2006-01-02 15:04:05"
	URL          string `json:"url"`
}

var gubaArticleListRe = regexp.MustCompile(`article_list=(\{.*?\});`)

// HotPostContent 单帖详情（文字+图片）
type HotPostContent struct {
	Content string   `json:"content"` // 主贴正文（HTML，由调用方决定渲染方式）
	Images  []string `json:"images"`  // 图片 URL 集合（来自 post_pic_url 与正文 img 标签）
}

var gubaPostArticleRe = regexp.MustCompile(`var post_article=(\{.*\});?\s*</script>`)

// FetchStockHotPosts 抓取东方财富股吧个股帖列表（约 80 条/页，默认按最后回复时间排序）。
// symbol 形如 600580.SH / 000001.SZ / 830799.BJ；港股吧页面结构不同，暂不支持。
// 排序（最新/最热）由调用方处理，这里原样返回页面顺序。
func FetchStockHotPosts(ctx context.Context, symbol string) ([]HotPost, error) {
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	market := strings.ToUpper(parts[1])
	if market == "HK" {
		return nil, fmt.Errorf("港股暂不支持股吧热帖")
	}
	code := parts[0]

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://guba.eastmoney.com/list,%s.html", code)
	req, err := http.NewRequestWithContext(rctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	body, err := HTTPDo(req)
	if err != nil {
		return nil, err
	}
	return parseGubaHotPosts(body, code)
}

// FetchStockHotPostContent 抓取东方财富股吧单帖正文与图片。
func FetchStockHotPostContent(ctx context.Context, symbol string, postID int64) (*HotPostContent, error) {
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	market := strings.ToUpper(parts[1])
	if market == "HK" {
		return nil, fmt.Errorf("港股暂不支持股吧热帖")
	}
	code := parts[0]

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://guba.eastmoney.com/news,%s,%d.html", code, postID)
	req, err := http.NewRequestWithContext(rctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	body, err := HTTPDo(req)
	if err != nil {
		return nil, err
	}
	return parseGubaPostContent(body)
}

// parseGubaPostContent 从帖子详情页 HTML 中解析 post_article 内嵌 JSON，
// 提取正文与图片。采用括号深度计数，避免正文中的字符串干扰闭合。
func parseGubaPostContent(body []byte) (*HotPostContent, error) {
	jsonBytes := extractPostArticleJSON(body)
	if jsonBytes == nil {
		m := gubaPostArticleRe.FindSubmatch(body)
		if m != nil {
			jsonBytes = m[1]
		} else {
			return nil, fmt.Errorf("未找到东方财富股吧 post_article 数据")
		}
	}

	var result struct {
		PostContent string   `json:"post_content"`
		PostPicURL  []string `json:"post_pic_url"`
	}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, fmt.Errorf("解析东方财富股吧 post_article: %w", err)
	}

	content := strings.TrimSpace(result.PostContent)
	if content == "" && len(result.PostPicURL) == 0 {
		return nil, fmt.Errorf("该帖子无正文内容")
	}

	var images []string
	seen := make(map[string]bool)
	for _, u := range result.PostPicURL {
		u = strings.TrimSpace(u)
		if u != "" && !seen[u] {
			seen[u] = true
			images = append(images, u)
		}
	}

	// 正文中可能还含有 <img> 标签
	imgRe := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
	for _, sm := range imgRe.FindAllStringSubmatch(content, -1) {
		u := strings.TrimSpace(sm[1])
		if u != "" && !seen[u] {
			seen[u] = true
			images = append(images, u)
		}
	}

	return &HotPostContent{Content: stripHtmlTags(content), Images: images}, nil
}

// stripHtmlTags 去除正文中的 HTML 标签，返回纯文本（保留段落换行）。
func stripHtmlTags(content string) string {
	replacer := strings.NewReplacer(
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"</p>", "\n",
		"</div>", "\n",
	)
	text := replacer.Replace(content)
	// 去掉所有剩余标签
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text = tagRe.ReplaceAllString(text, "")
	// 解码常见 HTML 实体
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	// 压缩连续空行
	blankRe := regexp.MustCompile(`\n\s*\n`)
	text = blankRe.ReplaceAllString(text, "\n")
	return strings.TrimSpace(text)
}

// extractPostArticleJSON 从页面脚本中提取 var post_article={...} 的 JSON 对象字节。
// 通过括号深度计数，正确处理字符串中可能含有的 } 或 </script>。
func extractPostArticleJSON(body []byte) []byte {
	prefix := []byte("var post_article=")
	i := bytes.Index(body, prefix)
	if i < 0 {
		return nil
	}
	start := i + len(prefix)
	if start >= len(body) || body[start] != '{' {
		return nil
	}
	depth := 0
	inString := false
	escaped := false
	for j := start; j < len(body); j++ {
		b := body[j]
		if inString {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return body[start : j+1]
			}
		}
	}
	return nil
}

// parseGubaHotPosts 从股吧列表页 HTML 中解析 article_list 内嵌 JSON。
func parseGubaHotPosts(body []byte, code string) ([]HotPost, error) {
	m := gubaArticleListRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("未找到东方财富股吧 article_list 数据 (%s)", code)
	}

	var result struct {
		Re []gubaPostRow `json:"re"`
	}
	if err := json.Unmarshal(m[1], &result); err != nil {
		return nil, fmt.Errorf("解析东方财富股吧 article_list: %w", err)
	}

	posts := make([]HotPost, 0, len(result.Re))
	for _, row := range result.Re {
		title := strings.TrimSpace(row.PostTitle)
		if title == "" || row.PostID == 0 {
			continue
		}
		posts = append(posts, HotPost{
			ID:           row.PostID,
			Title:        title,
			Author:       strings.TrimSpace(row.UserNickname),
			ClickCount:   row.PostClickCount,
			CommentCount: row.PostCommentCount,
			PublishTime:  row.PostPublishTime,
			URL:          fmt.Sprintf("https://guba.eastmoney.com/news,%s,%d.html", code, row.PostID),
		})
	}
	if len(posts) == 0 {
		return nil, fmt.Errorf("东方财富股吧 %s 无帖子数据", code)
	}
	return posts, nil
}

type gubaPostRow struct {
	PostID           int64  `json:"post_id"`
	PostTitle        string `json:"post_title"`
	UserNickname     string `json:"user_nickname"`
	PostClickCount   int    `json:"post_click_count"`
	PostCommentCount int    `json:"post_comment_count"`
	PostPublishTime  string `json:"post_publish_time"`
}
