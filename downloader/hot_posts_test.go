package downloader

import (
	"strings"
	"testing"
)

// TestParseGubaHotPosts 验证股吧列表页 article_list 内嵌 JSON 解析正确
func TestParseGubaHotPosts(t *testing.T) {
	// 原始结构来自真实页面（list,600580.html 内嵌的 article_list）
	body := []byte(`<html><head></head><body><script>var article_list={"re":[{"post_id":1745850461,"post_title":"第一调到20元，二会到10元.","stockbar_code":"600580","user_nickname":"万里长城7","post_click_count":13,"post_comment_count":0,"post_publish_time":"2026-07-19 12:24:22"},{"post_id":1745850000,"post_title":"  板块全在跌，卧龙真的算扛住了  ","stockbar_code":"600580","user_nickname":"某股友","post_click_count":415,"post_comment_count":5,"post_publish_time":"2026-07-17 19:13:00"}],"count":80,"rc":1};    var other_list={"re":[]}</script></body></html>`)

	posts, err := parseGubaHotPosts(body, "600580")
	if err != nil {
		t.Fatalf("parseGubaHotPosts error: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("expected 2 posts, got %d", len(posts))
	}

	first := posts[0]
	if first.ID != 1745850461 {
		t.Errorf("ID = %d, want 1745850461", first.ID)
	}
	if first.Title != "第一调到20元，二会到10元." {
		t.Errorf("Title = %q", first.Title)
	}
	if first.Author != "万里长城7" {
		t.Errorf("Author = %q", first.Author)
	}
	if first.ClickCount != 13 {
		t.Errorf("ClickCount = %d, want 13", first.ClickCount)
	}
	if first.PublishTime != "2026-07-19 12:24:22" {
		t.Errorf("PublishTime = %q", first.PublishTime)
	}
	if first.URL != "https://guba.eastmoney.com/news,600580,1745850461.html" {
		t.Errorf("URL = %q", first.URL)
	}
	// 标题两侧空白应被裁剪
	if posts[1].Title != "板块全在跌，卧龙真的算扛住了" {
		t.Errorf("trimmed Title = %q", posts[1].Title)
	}
}

// TestParseGubaHotPostsBadInput 验证异常输入返回错误而非 panic
func TestParseGubaHotPostsBadInput(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"空响应", ""},
		{"无article_list", `<html>var foo={"re":[]};</html>`},
		{"空帖子列表", `var article_list={"re":[],"count":0};`},
		{"帖子缺标题和ID", `var article_list={"re":[{"post_id":0,"post_title":""}]};`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := parseGubaHotPosts([]byte(c.body), "600580"); err == nil {
				t.Errorf("expected error for %s, got nil", c.name)
			}
		})
	}
}

// TestParseGubaPostContent 验证帖子详情页 post_article 解析与 HTML 去标签
func TestParseGubaPostContent(t *testing.T) {
	// 模拟 post_article 中正文含 HTML、图片在 post_pic_url 与正文 img 标签各一份
	body := []byte(`<script>var post_article={"post_id":123456,"post_title":"测试帖","post_content":"<div class=\"content\"><p>第一段。</p><p>第二段<br/>换行</p><img src=\"https://example.com/a.jpg\"/><img src=\"https://example.com/a.jpg\"/></div>","post_pic_url":["https://example.com/a.jpg","https://example.com/b.jpg"],"post_publish_time":"2026-07-19 12:00:00"};</script>`)

	content, err := parseGubaPostContent(body)
	if err != nil {
		t.Fatalf("parseGubaPostContent error: %v", err)
	}
	if !strings.Contains(content.Content, "第一段。") || !strings.Contains(content.Content, "第二段") {
		t.Errorf("Content text incorrect: %q", content.Content)
	}
	if strings.Contains(content.Content, "<p>") || strings.Contains(content.Content, "<img") {
		t.Errorf("Content should not contain HTML tags: %q", content.Content)
	}
	if len(content.Images) != 2 {
		t.Errorf("Images = %v, want 2 unique urls", content.Images)
	}
}

// TestExtractPostArticleJSON 验证括号深度计数能正确处理正文字符串中的特殊字符
func TestExtractPostArticleJSON(t *testing.T) {
	body := []byte(`var post_article={"post_content":"这里有一个右花括号}和一个script结束</script>标签","post_pic_url":[]};`)
	got := extractPostArticleJSON(body)
	want := `{"post_content":"这里有一个右花括号}和一个script结束</script>标签","post_pic_url":[]}`
	if string(got) != want {
		t.Errorf("extractPostArticleJSON = %q, want %q", string(got), want)
	}
}

// TestStripHtmlTags 验证 HTML 转纯文本
func TestStripHtmlTags(t *testing.T) {
	in := `<div><p>第一行</p><p>第二行<br/>换行</p>&nbsp;空格</div>`
	out := stripHtmlTags(in)
	if !strings.Contains(out, "第一行") || !strings.Contains(out, "第二行") || !strings.Contains(out, "换行") {
		t.Errorf("stripHtmlTags text = %q", out)
	}
	if strings.Contains(out, "<") {
		t.Errorf("stripHtmlTags still has tags: %q", out)
	}
}

// TestFetchStockHotPostsInvalidSymbol 验证代码格式与港股拦截
func TestFetchStockHotPostsInvalidSymbol(t *testing.T) {
	if _, err := FetchStockHotPosts(t.Context(), "600580"); err == nil {
		t.Error("缺少市场后缀应报错")
	}
	if _, err := FetchStockHotPosts(t.Context(), "00700.HK"); err == nil || !strings.Contains(err.Error(), "港股") {
		t.Errorf("港股应返回不支持错误, got %v", err)
	}
	if _, err := FetchStockHotPostContent(t.Context(), "00700.HK", 123); err == nil || !strings.Contains(err.Error(), "港股") {
		t.Errorf("港股内容应返回不支持错误, got %v", err)
	}
}
