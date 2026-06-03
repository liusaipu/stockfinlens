package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

const yahooBaseURL = "https://query1.finance.yahoo.com/v8/finance/chart"

// toYahooSymbol 将 market+code 转为 Yahoo 的 symbol 格式
func toYahooSymbol(market, code string) string {
	switch market {
	case "SH":
		return code + ".SS"
	case "SZ":
		return code + ".SZ"
	case "HK":
		return code + ".HK"
	default:
		return code + ".SS"
	}
}

// fetchKlinesFromYahoo 从 Yahoo Finance 获取历史 K线
func fetchKlinesFromYahoo(ctx context.Context, market, code string, days int, period string) ([]KlineData, error) {
	if period == "" {
		period = "daily"
	}
	symbol := toYahooSymbol(market, code)
	end := time.Now().Unix()
	// 周线/月线需要扩大时间窗口以获取足够条数
	multiplier := 1
	switch period {
	case "weekly":
		multiplier = 7
	case "monthly":
		multiplier = 30
	}
	start := time.Now().AddDate(0, 0, -days*multiplier).Unix()

	interval := "1d"
	switch period {
	case "weekly":
		interval = "1wk"
	case "monthly":
		interval = "1mo"
	}

	url := fmt.Sprintf("%s/%s?period1=%d&period2=%d&interval=%s&events=history", yahooBaseURL, symbol, start, end, interval)
	fmt.Printf("[Yahoo] fetching klines: %s\n", url)

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, fmt.Errorf("Yahoo 请求失败: %w", err)
	}

	var result struct {
		Chart struct {
			Result []struct {
				Meta struct {
					Currency           string  `json:"currency"`
					Symbol             string  `json:"symbol"`
					RegularMarketPrice float64 `json:"regularMarketPrice"`
					PreviousClose      float64 `json:"previousClose"`
					ChartPreviousClose float64 `json:"chartPreviousClose"`
				} `json:"meta"`
				Timestamp  []int64 `json:"timestamp"`
				Indicators struct {
					Quote []struct {
						Open   []float64 `json:"open"`
						High   []float64 `json:"high"`
						Low    []float64 `json:"low"`
						Close  []float64 `json:"close"`
						Volume []int64   `json:"volume"`
					} `json:"quote"`
					Adjclose []struct {
						Adjclose []float64 `json:"adjclose"`
					} `json:"adjclose"`
				} `json:"indicators"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 Yahoo 响应失败: %w", err)
	}

	if result.Chart.Error != nil {
		return nil, fmt.Errorf("Yahoo API 错误: %s - %s", result.Chart.Error.Code, result.Chart.Error.Description)
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("Yahoo 无数据")
	}

	r := result.Chart.Result[0]
	timestamps := r.Timestamp
	quotes := r.Indicators.Quote
	if len(quotes) == 0 {
		return nil, fmt.Errorf("Yahoo 无行情数据")
	}
	q := quotes[0]

	// 使用前复权价格（如有）
	adjCloses := q.Close
	if len(r.Indicators.Adjclose) > 0 && len(r.Indicators.Adjclose[0].Adjclose) == len(timestamps) {
		adjCloses = r.Indicators.Adjclose[0].Adjclose
	}

	resultKlines := make([]KlineData, 0, len(timestamps))
	for i := 0; i < len(timestamps); i++ {
		if i >= len(q.Open) || i >= len(q.High) || i >= len(q.Low) || i >= len(adjCloses) || i >= len(q.Volume) {
			continue
		}
		t := time.Unix(timestamps[i], 0).Format("2006-01-02")
		resultKlines = append(resultKlines, KlineData{
			Time:   t,
			Open:   q.Open[i],
			High:   q.High[i],
			Low:    q.Low[i],
			Close:  adjCloses[i],
			Volume: float64(q.Volume[i]),
		})
	}

	fmt.Printf("[Yahoo] returned %d klines for %s\n", len(resultKlines), symbol)
	return resultKlines, nil
}

// fetchQuoteFromYahoo 从 Yahoo Finance 获取实时行情
func fetchQuoteFromYahoo(ctx context.Context, market, code string) (*StockQuote, error) {
	symbol := toYahooSymbol(market, code)
	url := fmt.Sprintf("%s/%s?interval=1d&range=1d", yahooBaseURL, symbol)
	fmt.Printf("[Yahoo] fetching quote: %s\n", url)

	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, fmt.Errorf("Yahoo 请求失败: %w", err)
	}

	var result struct {
		Chart struct {
			Result []struct {
				Meta struct {
					RegularMarketPrice         float64 `json:"regularMarketPrice"`
					RegularMarketChange        float64 `json:"regularMarketChange"`
					RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
					PreviousClose              float64 `json:"previousClose"`
					RegularMarketOpen          float64 `json:"regularMarketOpen"`
					RegularMarketDayHigh       float64 `json:"regularMarketDayHigh"`
					RegularMarketDayLow        float64 `json:"regularMarketDayLow"`
					RegularMarketVolume        int64   `json:"regularMarketVolume"`
					Currency                   string  `json:"currency"`
					Symbol                     string  `json:"symbol"`
					ExchangeName               string  `json:"exchangeName"`
				} `json:"meta"`
			} `json:"result"`
			Error *struct {
				Code        string `json:"code"`
				Description string `json:"description"`
			} `json:"error"`
		} `json:"chart"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析 Yahoo 响应失败: %w", err)
	}

	if result.Chart.Error != nil {
		return nil, fmt.Errorf("Yahoo API 错误: %s - %s", result.Chart.Error.Code, result.Chart.Error.Description)
	}

	if len(result.Chart.Result) == 0 {
		return nil, fmt.Errorf("Yahoo 无数据")
	}

	m := result.Chart.Result[0].Meta
	return &StockQuote{
		CurrentPrice:  m.RegularMarketPrice,
		ChangePercent: m.RegularMarketChangePercent,
		ChangeAmount:  m.RegularMarketChange,
		PreviousClose: m.PreviousClose,
		Open:          m.RegularMarketOpen,
		High:          m.RegularMarketDayHigh,
		Low:           m.RegularMarketDayLow,
		Volume:        float64(m.RegularMarketVolume),
		QuoteTime:     time.Now().Format("2006-01-02 15:04:05"),
	}, nil
}
