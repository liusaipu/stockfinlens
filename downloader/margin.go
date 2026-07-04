package downloader

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const marginBaseURL = "https://datacenter-web.eastmoney.com/api/data/v1/get"

// MarginData 单条融资融券数据（与东方财富字段对齐）
type MarginData struct {
	Date            string  `json:"date"`              // 交易日期 YYYY-MM-DD
	Code            string  `json:"code"`              // 证券代码（纯数字，如 000001）
	Name            string  `json:"name"`              // 证券简称
	Market          string  `json:"market"`            // 市场分类：融资融券_上海 / 融资融券_深证
	RZYE            float64 `json:"rzye"`              // 融资余额（元）
	RQMCL           float64 `json:"rqmcl"`             // 融券卖出量（股）
	RQYL            float64 `json:"rqyl"`              // 融券余量（股）
	RZRQYE          float64 `json:"rzrqye"`            // 融资融券余额（元）
	RQYE            float64 `json:"rqye"`              // 融券余额（元）
	RZMRE           float64 `json:"rzmre"`             // 融资买入额（元）
	RZCHE           float64 `json:"rzche"`             // 融资偿还额（元）
	RZJME           float64 `json:"rzjme"`             // 融资净买额（元）
	RZYEZB          float64 `json:"rzyezb"`            // 融资余额占流通市值比（%）
	SPJ             float64 `json:"spj"`               // 收盘价
	ZDF             float64 `json:"zdf"`               // 涨跌幅（%）
	TradeMarketCode string  `json:"trade_market_code"` // 二级市场代码
	TradeMarket     string  `json:"trade_market"`      // 二级市场
}

// MarginFetchOption 融资融券数据获取选项
type MarginFetchOption struct {
	Timeout time.Duration
}

// DefaultMarginFetchOption 返回默认选项
func DefaultMarginFetchOption() MarginFetchOption {
	return MarginFetchOption{Timeout: 30 * time.Second}
}

// FetchMarginByDate 按日期获取沪深两市全部融资融券明细
// date 格式：YYYY-MM-DD
func FetchMarginByDate(date string, opt ...MarginFetchOption) ([]MarginData, error) {
	date = normalizeMarginDate(date)
	option := DefaultMarginFetchOption()
	if len(opt) > 0 {
		option = opt[0]
	}

	client := &http.Client{Timeout: option.Timeout, Transport: sharedTransport}
	var all []MarginData
	page := 1
	for {
		params := map[string]string{
			"reportName":  "RPTA_WEB_RZRQ_GGMX",
			"columns":     "ALL",
			"source":      "WEB",
			"pageSize":    "500",
			"pageNumber":  strconv.Itoa(page),
			"sortColumns": "scode",
			"sortTypes":   "1",
			"filter":      fmt.Sprintf("(DATE='%s')", date),
		}
		items, pages, err := fetchMarginPage(client, params)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if page >= pages || len(items) == 0 {
			break
		}
		page++
	}
	return all, nil
}

// FetchMarginHistory 获取指定个股融资融券历史数据
// symbol 支持带点格式（000001.SZ）或纯数字代码
func FetchMarginHistory(symbol string, opt ...MarginFetchOption) ([]MarginData, error) {
	code := pureCodeFromSymbol(symbol)
	if code == "" {
		return nil, fmt.Errorf("证券代码为空")
	}
	fmt.Printf("[Margin] FetchMarginHistory symbol=%s code=%s\n", symbol, code)

	option := DefaultMarginFetchOption()
	if len(opt) > 0 {
		option = opt[0]
	}

	client := &http.Client{Timeout: option.Timeout, Transport: sharedTransport}
	var all []MarginData
	page := 1
	for {
		params := map[string]string{
			"reportName":  "RPTA_WEB_RZRQ_GGMX",
			"columns":     "ALL",
			"source":      "WEB",
			"pageSize":    "500",
			"pageNumber":  strconv.Itoa(page),
			"sortColumns": "DATE",
			"sortTypes":   "-1",
			"filter":      fmt.Sprintf(`(scode="%s")`, code),
		}
		items, pages, err := fetchMarginPage(client, params)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if page >= pages || len(items) == 0 {
			break
		}
		page++
	}

	// 按日期升序排列，方便前端绘制时间序列
	reverseMarginData(all)
	fmt.Printf("[Margin] FetchMarginHistory symbol=%s total=%d\n", symbol, len(all))
	return all, nil
}

// fetchMarginPage 单页请求
func fetchMarginPage(client *http.Client, params map[string]string) ([]MarginData, int, error) {
	req, err := http.NewRequest("GET", marginBaseURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("创建请求失败: %w", err)
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("User-Agent", defaultUA)
	req.Header.Set("Referer", "https://data.eastmoney.com/rzrq/")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("请求融资融券接口失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("融资融券接口返回状态码 %d", resp.StatusCode)
	}

	var wrapper struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Result  *struct {
			Pages int             `json:"pages"`
			Data  json.RawMessage `json:"data"`
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wrapper); err != nil {
		return nil, 0, fmt.Errorf("解析融资融券响应失败: %w", err)
	}
	if !wrapper.Success {
		return nil, 0, fmt.Errorf("融资融券接口错误: %s", wrapper.Message)
	}
	if wrapper.Result == nil || len(wrapper.Result.Data) == 0 {
		fmt.Printf("[Margin] fetchMarginPage result empty, filter=%s\n", params["filter"])
		return nil, 0, nil
	}

	var raw []map[string]interface{}
	if err := json.Unmarshal(wrapper.Result.Data, &raw); err != nil {
		return nil, 0, fmt.Errorf("解析融资融券数据失败: %w", err)
	}

	items := make([]MarginData, 0, len(raw))
	for _, r := range raw {
		items = append(items, parseMarginItem(r))
	}
	return items, wrapper.Result.Pages, nil
}

// parseMarginItem 解析单条原始数据
func parseMarginItem(r map[string]interface{}) MarginData {
	item := MarginData{
		Date:            parseMarginDate(getString(r, "DATE")),
		Code:            getString(r, "SCODE"),
		Name:            getString(r, "SECNAME"),
		Market:          getString(r, "MARKET"),
		TradeMarketCode: getString(r, "TRADE_MARKET_CODE"),
		TradeMarket:     getString(r, "TRADE_MARKET"),
	}
	item.RZYE = getFloat64(r, "RZYE")
	item.RQMCL = getFloat64(r, "RQMCL")
	item.RQYL = getFloat64(r, "RQYL")
	item.RZRQYE = getFloat64(r, "RZRQYE")
	item.RQYE = getFloat64(r, "RQYE")
	item.RZMRE = getFloat64(r, "RZMRE")
	item.RZCHE = getFloat64(r, "RZCHE")
	item.RZJME = getFloat64(r, "RZJME")
	item.RZYEZB = getFloat64(r, "RZYEZB")
	item.SPJ = getFloat64(r, "SPJ")
	item.ZDF = getFloat64(r, "ZDF")
	return item
}

// normalizeMarginDate 统一日期格式为 YYYY-MM-DD
func normalizeMarginDate(date string) string {
	date = strings.TrimSpace(date)
	if len(date) == 8 {
		return date[:4] + "-" + date[4:6] + "-" + date[6:]
	}
	return date
}

// parseMarginDate 把东方财富返回的日期解析为 YYYY-MM-DD
func parseMarginDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

// pureCodeFromSymbol 从 000001.SZ / 600519.SH 中提取纯数字代码
func pureCodeFromSymbol(symbol string) string {
	parts := strings.Split(symbol, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return symbol
}

// reverseMarginData 反转切片（DATE 降序变升序）
func reverseMarginData(data []MarginData) {
	for i, j := 0, len(data)-1; i < j; i, j = i+1, j-1 {
		data[i], data[j] = data[j], data[i]
	}
}

// getString 安全读取字符串字段
func getString(r map[string]interface{}, key string) string {
	v, ok := r[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// getFloat64 安全读取浮点字段
func getFloat64(r map[string]interface{}, key string) float64 {
	v, ok := r[key]
	if !ok || v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		f, _ := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		return f
	}
}
