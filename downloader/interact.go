package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// InteractQA 单条互动平台问答
type InteractQA struct {
	Question   string `json:"question"`
	Answer     string `json:"answer"`
	Questioner string `json:"questioner"`
	Date       string `json:"date"`       // 提问日期 YYYY-MM-DD
	AnswerDate string `json:"answerDate"` // 回答日期 YYYY-MM-DD
	Source     string `json:"source"`     // 数据来源：irm / guba / sse
}

// FetchStockInteractQA 获取指定股票在互动平台的问答列表
// 策略：
//   - 沪市(SH)：东财问董秘 + 上证e互动，合并去重
//   - 深市(SZ)：东财问董秘 + 深市互动易，合并去重
//
// 仅保留最近3个月（90天）内有回答的问答。
// 港股返回空列表。
func FetchStockInteractQA(ctx context.Context, market, code string) ([]InteractQA, error) {
	if strings.ToUpper(market) == "HK" {
		return nil, fmt.Errorf("港股暂不支持互动平台问答")
	}

	var all []InteractQA

	// 1. 东财问董秘（全市场通用）
	if qas, err := fetchGubaQA(ctx, code, 30); err == nil && len(qas) > 0 {
		all = append(all, qas...)
	}

	// 2. 按市场补充专用数据源
	switch strings.ToUpper(market) {
	case "SH":
		if qas, err := fetchSSEQA(ctx, code, 30); err == nil && len(qas) > 0 {
			all = append(all, qas...)
		}
	case "SZ":
		if qas, err := fetchIRMQuestionsWithFallback(ctx, code, 30); err == nil && len(qas) > 0 {
			all = append(all, qas...)
		}
	}

	if len(all) == 0 {
		return nil, fmt.Errorf("未能获取 %s 的问答数据", code)
	}

	// 去重 + 过滤90天内 + 排序
	return dedupAndFilterQAs(all), nil
}

// dedupAndFilterQAs 去重、过滤90天内、按日期降序
func dedupAndFilterQAs(qas []InteractQA) []InteractQA {
	cutoff := time.Now().AddDate(0, 0, -90)
	seen := make(map[string]bool)
	var filtered []InteractQA

	for _, qa := range qas {
		if qa.Answer == "" {
			continue
		}
		// 用问题内容去重（忽略首尾空格和标点差异）
		key := strings.TrimSpace(qa.Question)
		if seen[key] {
			continue
		}
		seen[key] = true

		// 时间过滤：以回答日期为准，如果没有则用提问日期
		checkDate := qa.AnswerDate
		if checkDate == "" {
			checkDate = qa.Date
		}
		if checkDate == "" {
			continue
		}
		t, err := time.Parse("2006-01-02", checkDate)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			continue
		}
		filtered = append(filtered, qa)
	}

	// 按回答日期降序
	sort.Slice(filtered, func(i, j int) bool {
		di, _ := time.Parse("2006-01-02", filtered[i].AnswerDate)
		dj, _ := time.Parse("2006-01-02", filtered[j].AnswerDate)
		if !di.IsZero() && !dj.IsZero() {
			return di.After(dj)
		}
		// fallback 到提问日期
		di, _ = time.Parse("2006-01-02", filtered[i].Date)
		dj, _ = time.Parse("2006-01-02", filtered[j].Date)
		return di.After(dj)
	})

	return filtered
}

// ========== 深市互动易（巨潮） ==========

func fetchIRMQuestionsWithFallback(ctx context.Context, code string, limit int) ([]InteractQA, error) {
	orgID, err := fetchIRMOrgID(ctx, code)
	if err != nil {
		return nil, err
	}
	return fetchIRMQuestions(ctx, code, orgID, limit)
}

func fetchIRMOrgID(ctx context.Context, code string) (string, error) {
	u := "https://irm.cninfo.com.cn/newircs/index/queryKeyboardInfo"
	params := url.Values{}
	params.Set("_t", strconv.FormatInt(time.Now().Unix(), 10))

	req, err := http.NewRequestWithContext(ctx, "POST", u+"?"+params.Encode(), strings.NewReader("keyWord="+code))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	body, err := HTTPDo(req)
	if err != nil {
		return "", err
	}

	var result struct {
		Data []struct {
			Secid string `json:"secid"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if len(result.Data) == 0 {
		return "", fmt.Errorf("未找到股票 %s 的互动易组织代码", code)
	}
	return result.Data[0].Secid, nil
}

func fetchIRMQuestions(ctx context.Context, code, orgID string, limit int) ([]InteractQA, error) {
	const pageURL = "https://irm.cninfo.com.cn/newircs/company/question"
	var all []InteractQA
	pageNum := 1

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	for len(all) < limit {
		params := url.Values{}
		params.Set("_t", strconv.FormatInt(time.Now().Unix(), 10))
		params.Set("stockcode", code)
		params.Set("orgId", orgID)
		params.Set("pageSize", strconv.Itoa(minInt(limit, 100)))
		params.Set("pageNum", strconv.Itoa(pageNum))
		params.Set("keyWord", "")
		params.Set("startDay", "")
		params.Set("endDay", "")

		req, err := http.NewRequestWithContext(rctx, "POST", pageURL+"?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}

		body, err := HTTPDo(req)
		if err != nil {
			return nil, err
		}

		var result struct {
			Rows      []irmQuestionRow `json:"rows"`
			TotalPage int              `json:"totalPage"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("解析互动问答: %w", err)
		}
		if len(result.Rows) == 0 {
			break
		}

		for _, row := range result.Rows {
			if row.AttachedContent == "" {
				continue
			}
			qa := InteractQA{
				Question:   row.MainContent,
				Answer:     row.AttachedContent,
				Questioner: row.AuthorName,
				Date:       formatIRMDate(row.PubDate),
				AnswerDate: formatIRMDate(row.AttachedPubDate),
				Source:     "irm",
			}
			all = append(all, qa)
			if len(all) >= limit {
				break
			}
		}

		if pageNum >= result.TotalPage {
			break
		}
		pageNum++
	}

	return all, nil
}

type irmQuestionRow struct {
	MainContent     string  `json:"mainContent"`
	AttachedContent string  `json:"attachedContent"`
	AuthorName      string  `json:"authorName"`
	PubDate         float64 `json:"pubDate"`
	AttachedPubDate float64 `json:"attachedPubDate"`
}

func formatIRMDate(ts float64) string {
	if ts == 0 {
		return ""
	}
	return time.UnixMilli(int64(ts)).Format("2006-01-02")
}

// ========== 东方财富股吧问董秘 ==========

var gubaQAListRe = regexp.MustCompile(`qa_list\s*=\s*(\{.*?\});`)

func fetchGubaQA(ctx context.Context, code string, limit int) ([]InteractQA, error) {
	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// 默认（不加type）获取最新答复，不带type=2（传闻求证）
	u := fmt.Sprintf("https://guba.eastmoney.com/qa/search?code=%s", code)
	req, err := http.NewRequestWithContext(rctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	body, err := HTTPDo(req)
	if err != nil {
		return nil, err
	}

	m := gubaQAListRe.FindSubmatch(body)
	if m == nil {
		return nil, fmt.Errorf("未找到东方财富股吧 qa_list 数据")
	}

	var result struct {
		Re []gubaQARow `json:"re"`
	}
	if err := json.Unmarshal(m[1], &result); err != nil {
		return nil, fmt.Errorf("解析东方财富股吧 qa_list: %w", err)
	}

	var qas []InteractQA
	for _, row := range result.Re {
		if row.AskAnswer == "" || row.AskQuestion == "" {
			continue
		}
		if row.StockbarCode != "" && row.StockbarCode != code {
			continue
		}
		qa := InteractQA{
			Question:   row.AskQuestion,
			Answer:     cleanGubaAnswer(row.AskAnswer),
			Questioner: row.UserNickname,
			Date:       formatGubaDate(row.PostPublishTime),
			AnswerDate: formatGubaDate(row.PostLastTime),
			Source:     "guba",
		}
		qas = append(qas, qa)
		if len(qas) >= limit {
			break
		}
	}

	if len(qas) == 0 {
		return nil, fmt.Errorf("东方财富股吧 %s 无有效问答数据", code)
	}
	return qas, nil
}

type gubaQARow struct {
	StockbarCode    string `json:"stockbar_code"`
	UserNickname    string `json:"user_nickname"`
	PostPublishTime string `json:"post_publish_time"`
	PostLastTime    string `json:"post_last_time"`
	AskQuestion     string `json:"ask_question"`
	AskAnswer       string `json:"ask_answer"`
}

func cleanGubaAnswer(answer string) string {
	if idx := strings.Index(answer, "："); idx > 0 && idx < 20 {
		after := answer[idx+3:]
		after = strings.TrimPrefix(after, "您好，")
		after = strings.TrimPrefix(after, "您好!")
		after = strings.TrimPrefix(after, "您好！")
		after = strings.TrimPrefix(after, "您好")
		after = strings.TrimPrefix(after, "。")
		after = strings.TrimPrefix(after, "，")
		answer = strings.TrimSpace(after)
	}
	answer = strings.TrimPrefix(answer, "尊敬的投资者您好！")
	answer = strings.TrimPrefix(answer, "尊敬的投资者您好!")
	answer = strings.TrimPrefix(answer, "尊敬的投资者您好")
	answer = strings.TrimPrefix(answer, "尊敬的投资者：")
	answer = strings.TrimPrefix(answer, "投资者您好！")
	answer = strings.TrimPrefix(answer, "投资者您好!")
	answer = strings.TrimPrefix(answer, "投资者您好")
	answer = strings.TrimPrefix(answer, "您好！")
	answer = strings.TrimPrefix(answer, "您好!")
	answer = strings.TrimPrefix(answer, "您好")
	answer = strings.TrimPrefix(answer, "。")
	answer = strings.TrimPrefix(answer, "，")
	return strings.TrimSpace(answer)
}

func formatGubaDate(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return s
	}
	return t.Format("2006-01-02")
}

// ========== 上证e互动 ==========

var (
	sseQASplitRe = regexp.MustCompile(`<div class="m_feed_item[^"]*" id="qa-item-\d+">`)
	sseQTextRe   = regexp.MustCompile(`<div class="m_feed_txt"[^>]*>(.*?)</div>`)
	sseAskerRe   = regexp.MustCompile(`<div class="m_feed_face"[^>]*>.*?<p>(.*?)</p>`)
	sseTimeRe    = regexp.MustCompile(`<span>([^<]*(?:年|昨天|今天)[^<]*)</span>`)
)

func fetchSSEQA(ctx context.Context, code string, limit int) ([]InteractQA, error) {
	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var all []InteractQA
	page := 1

	for len(all) < limit && page <= 3 {
		u := "https://sns.sseinfo.com/qasearchFullText.do"
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("keyword", code)
		params.Set("sdate", "")
		params.Set("edate", "")

		req, err := http.NewRequestWithContext(rctx, "POST", u, strings.NewReader(params.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")

		body, err := HTTPDo(req)
		if err != nil {
			return nil, err
		}

		qas, err := parseSSEQA(string(body), code)
		if err != nil {
			return nil, err
		}
		if len(qas) == 0 {
			break
		}
		all = append(all, qas...)
		page++
	}

	if len(all) == 0 {
		return nil, fmt.Errorf("上证e互动 %s 无有效问答数据", code)
	}
	return all, nil
}

func parseSSEQA(html, code string) ([]InteractQA, error) {
	parts := sseQASplitRe.Split(html, -1)
	if len(parts) <= 1 {
		return nil, fmt.Errorf("未找到上证e互动问答条目")
	}

	var qas []InteractQA
	now := time.Now()

	for _, part := range parts[1:] {
		// 判断是否包含回答
		hasAnswer := strings.Contains(part, `class="m_feed_detail m_qa"`)
		if !hasAnswer {
			continue
		}

		// 提取问题文本（第一个 m_feed_txt）
		qMatch := sseQTextRe.FindStringSubmatch(part)
		if qMatch == nil {
			continue
		}
		question := stripHTML(qMatch[1])
		question = strings.TrimPrefix(question, ":"+code)
		question = strings.TrimPrefix(question, "("+code+")")
		question = strings.TrimSpace(question)
		// 去掉开头的股票名前缀，如 "万东医疗(600055)"
		question = regexp.MustCompile(`^[^:：]+[：:]\s*`).ReplaceAllString(question, "")

		// 提取回答文本（m_qa 后面的 m_feed_txt）
		qaIdx := strings.Index(part, `class="m_feed_detail m_qa"`)
		answer := ""
		if qaIdx > 0 {
			aPart := part[qaIdx:]
			aMatch := sseQTextRe.FindStringSubmatch(aPart)
			if aMatch != nil {
				answer = stripHTML(aMatch[1])
				answer = strings.TrimSpace(answer)
			}
		}
		if answer == "" {
			continue
		}

		// 提取提问者
		asker := ""
		askerMatch := sseAskerRe.FindStringSubmatch(part)
		if askerMatch != nil {
			asker = stripHTML(askerMatch[1])
		}

		// 提取时间
		times := sseTimeRe.FindAllStringSubmatch(part, -1)
		qDate, aDate := "", ""
		for i, tm := range times {
			parsed := parseSSETime(tm[1], now)
			if parsed != "" {
				if i == 0 {
					qDate = parsed
				} else {
					aDate = parsed
				}
			}
		}
		if aDate == "" && qDate != "" {
			aDate = qDate
		}

		qas = append(qas, InteractQA{
			Question:   question,
			Answer:     cleanGubaAnswer(answer),
			Questioner: asker,
			Date:       qDate,
			AnswerDate: aDate,
			Source:     "sse",
		})
	}

	return qas, nil
}

func stripHTML(s string) string {
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "&nbsp;", " ")
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return strings.TrimSpace(s)
}

func parseSSETime(s string, now time.Time) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// "昨天 10:26"
	if strings.HasPrefix(s, "昨天") {
		return now.AddDate(0, 0, -1).Format("2006-01-02")
	}
	// "今天 XX:XX"
	if strings.HasPrefix(s, "今天") {
		return now.Format("2006-01-02")
	}
	// "2026年05月26日 16:07"
	if strings.Contains(s, "年") {
		t, err := time.Parse("2006年01月02日 15:04", s)
		if err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
