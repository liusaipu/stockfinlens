package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// SentimentSummary 单条舆情摘要
type SentimentSummary struct {
	Title     string  `json:"title"`
	Source    string  `json:"source"`
	Date      string  `json:"date"`
	Sentiment float64 `json:"sentiment"`
}

// SentimentData 舆情情绪数据
type SentimentData struct {
	Score         float64            `json:"score"`
	HeatIndex     int                `json:"heatIndex"`
	PositiveWords []string           `json:"positiveWords"`
	NegativeWords []string           `json:"negativeWords"`
	Summaries     []SentimentSummary `json:"summaries"`
	HasData       bool               `json:"hasData"`
	InteractQAs   []InteractQA       `json:"interactQAs,omitempty"`
}

// 预置中文情感词典
var (
	positiveDict = []string{
		"预增", "增长", "中标", "扩产", "回购", "创新高", "利好", "突破", "盈利", "上升",
		"提升", "优化", "稳健", "强劲", "亮眼", "超预期", "分红", "增持", "入驻", "签约",
		"获批", "认证", "荣誉", "领先", "冠军", "第一", "龙头", "壁垒", "稀缺", "刚需",
		"高增", "向好", "改善", "转型升级", "改革", "提价", "增厚", "确定性", "护城河",
	}
	negativeDict = []string{
		"减持", "亏损", "下滑", "下降", "暴雷", "退市", "立案", "警示", "债务违约", "裁员",
		"停产", "查封", "冻结", "质押", "爆仓", "逾期", "诉讼", "仲裁", "处罚", "罚款",
		"吊销", "破产", "重组失败", "业绩变脸", "不及预期", "腰斩", "跌停", "跳水", "沦陷",
		"崩盘", "踩雷", "套现", "跑路", "造假", "虚增", "隐瞒", "关联交易", "资金占用",
	}
)

// FetchStockSentiment 获取指定股票的舆情情绪数据
func FetchStockSentiment(ctx context.Context, market, code string) (*SentimentData, error) {
	// 1. 优先东财研报（带重试）
	var data *SentimentData
	var err error
	for i := 0; i < 2; i++ {
		data, err = fetchSentimentFromEastMoneyReports(ctx, code)
		if err == nil && data != nil && data.HasData {
			return data, nil
		}
		if i < 1 { // 第一次失败时短暂等待再试
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 2. fallback 东财公司公告
	data, err = fetchSentimentFromEastMoneyNotices(ctx, code)
	if err == nil && data != nil && data.HasData {
		return data, nil
	}

	// 3. fallback 新浪财经新闻
	data, err = fetchSentimentFromSinaNews(ctx, code)
	if err == nil && data != nil && data.HasData {
		return data, nil
	}

	// 4. 所有数据源失败，返回空数据结构（带提示）
	return &SentimentData{
		Score:         0,
		HeatIndex:     0,
		PositiveWords: []string{},
		NegativeWords: []string{},
		Summaries: []SentimentSummary{{
			Title:     "暂无舆情数据",
			Source:    "系统提示",
			Date:      time.Now().Format("2006-01-02"),
			Sentiment: 0,
		}},
		HasData: false,
	}, nil
}

// ========== 东财研报接口 ==========
func fetchSentimentFromEastMoneyReports(ctx context.Context, code string) (*SentimentData, error) {
	// 查询最近6个月的研报
	begin := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
	end := time.Now().Format("2006-01-02")
	url := fmt.Sprintf("https://reportapi.eastmoney.com/report/list?industryCode=*&pageNo=1&pageSize=20&code=%s&beginTime=%s&endTime=%s&qType=0", code, begin, end)

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data []struct {
			Title       string `json:"title"`
			OrgSName    string `json:"orgSName"`
			PublishDate string `json:"publishDate"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("东财研报数据为空")
	}

	var titles []string
	var summaries []SentimentSummary
	for _, item := range result.Data {
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		titles = append(titles, item.Title)
		date := strings.Split(item.PublishDate, " ")[0]
		summaries = append(summaries, SentimentSummary{
			Title:  item.Title,
			Source: item.OrgSName,
			Date:   date,
		})
	}

	data := analyzeSentiment(titles)
	data.Summaries = summaries
	// 回填每条摘要的情绪得分
	for i := range data.Summaries {
		s := calcSingleSentiment(data.Summaries[i].Title)
		data.Summaries[i].Sentiment = s
	}
	return data, nil
}

// ========== 东财公告接口 ==========
func fetchSentimentFromEastMoneyNotices(ctx context.Context, code string) (*SentimentData, error) {
	begin := time.Now().AddDate(0, -6, 0).Format("2006-01-02")
	end := time.Now().Format("2006-01-02")
	url := fmt.Sprintf(
		"https://np-anotice-stock.eastmoney.com/api/security/ann?sr=-1&page_size=20&page_index=1&ann_type=A&client_source=web&stock_list=%s&begin_time=%s&end_time=%s",
		code, begin, end,
	)

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Data struct {
			List []struct {
				Title      string `json:"title"`
				NoticeDate string `json:"notice_date"`
			} `json:"list"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	if len(result.Data.List) == 0 {
		return nil, fmt.Errorf("东财公告数据为空")
	}

	var titles []string
	var summaries []SentimentSummary
	for _, item := range result.Data.List {
		if strings.TrimSpace(item.Title) == "" {
			continue
		}
		titles = append(titles, item.Title)
		date := strings.Split(item.NoticeDate, " ")[0]
		summaries = append(summaries, SentimentSummary{
			Title:  item.Title,
			Source: "东方财富公告",
			Date:   date,
		})
	}

	data := analyzeSentiment(titles)
	data.Summaries = summaries
	for i := range data.Summaries {
		data.Summaries[i].Sentiment = calcSingleSentiment(data.Summaries[i].Title)
	}
	return data, nil
}

// ========== 新浪新闻接口 ==========
func fetchSentimentFromSinaNews(ctx context.Context, code string) (*SentimentData, error) {
	url := "https://feed.mix.sina.com.cn/api/roll/get?pageid=153&lid=2516&num=50&page=1"
	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, err
	}

	var result struct {
		Result struct {
			Data []struct {
				Title string `json:"title"`
				URL   string `json:"url"`
				FTime string `json:"ftime"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var titles []string
	var summaries []SentimentSummary
	for _, item := range result.Result.Data {
		if strings.Contains(item.Title, code) {
			titles = append(titles, item.Title)
			summaries = append(summaries, SentimentSummary{
				Title:  item.Title,
				Source: "新浪财经",
				Date:   item.FTime,
			})
		}
	}
	if len(titles) == 0 {
		return nil, fmt.Errorf("新浪新闻中未找到该股票相关资讯")
	}

	data := analyzeSentiment(titles)
	data.Summaries = summaries
	for i := range data.Summaries {
		data.Summaries[i].Sentiment = calcSingleSentiment(data.Summaries[i].Title)
	}
	return data, nil
}

// ========== 情感分析核心 ==========
func analyzeSentiment(titles []string) *SentimentData {
	totalPos := 0
	totalNeg := 0
	posSet := make(map[string]bool)
	negSet := make(map[string]bool)

	for _, t := range titles {
		p, n := countSentimentWords(t)
		totalPos += p
		totalNeg += n
	}

	// 收集命中过的关键词（去重）
	for _, t := range titles {
		for _, w := range positiveDict {
			if strings.Contains(t, w) {
				posSet[w] = true
			}
		}
		for _, w := range negativeDict {
			if strings.Contains(t, w) {
				negSet[w] = true
			}
		}
	}

	posWords := make([]string, 0, len(posSet))
	for w := range posSet {
		posWords = append(posWords, w)
	}
	sort.Strings(posWords)

	negWords := make([]string, 0, len(negSet))
	for w := range negSet {
		negWords = append(negWords, w)
	}
	sort.Strings(negWords)

	score := 0.0
	total := totalPos + totalNeg
	if total > 0 {
		score = float64(totalPos-totalNeg) / float64(total)
		// 平滑到 [-1, 1] 并考虑样本量
		score = clamp(score, -1, 1)
	}

	return &SentimentData{
		Score:         score,
		HeatIndex:     len(titles),
		PositiveWords: posWords,
		NegativeWords: negWords,
		HasData:       len(titles) > 0,
	}
}

func calcSingleSentiment(title string) float64 {
	p, n := countSentimentWords(title)
	total := p + n
	if total == 0 {
		return 0
	}
	return clamp(float64(p-n)/float64(total), -1, 1)
}

func countSentimentWords(text string) (pos int, neg int) {
	for _, w := range positiveDict {
		if strings.Contains(text, w) {
			pos++
		}
	}
	for _, w := range negativeDict {
		if strings.Contains(text, w) {
			neg++
		}
	}
	return
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// htmlStripTags 简单去除 HTML 标签
func htmlStripTags(input string) string {
	re := regexp.MustCompile("<[^>]*>")
	return re.ReplaceAllString(input, "")
}
