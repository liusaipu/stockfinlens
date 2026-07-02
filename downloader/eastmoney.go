package downloader

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const (
	hsf10BaseURL = "https://emweb.securities.eastmoney.com/PC_HSF10/NewFinanceAnalysis"
	dcBaseURL    = "https://datacenter-web.eastmoney.com/api/data/v1/get"
)

// FinancialReportData 下载后的三张表数据
type FinancialReportData struct {
	Symbol          string
	Years           []string
	BalanceSheet    map[string]map[string]float64
	IncomeStatement map[string]map[string]float64
	CashFlow        map[string]map[string]float64
}

// DownloadFinancialReports 从东方财富下载指定股票的所有年报财务数据
// 东方财富失败时自动尝试腾讯财经作为备用数据源
// maxYears 可选，默认为 5 年
func DownloadFinancialReports(ctx context.Context, market, code string, maxYears ...int) (*FinancialReportData, error) {
	limit := 5
	if len(maxYears) > 0 && maxYears[0] > 0 {
		limit = maxYears[0]
	}

	if strings.ToUpper(market) == "HK" {
		// 港股通过 Python 脚本获取
		result, err := fetchHKFinancialsFromPython(code, limit)
		if err == nil {
			return result, nil
		}
		return nil, fmt.Errorf("港股财报下载失败:\n%v\n\n建议:\n1. 检查网络连接\n2. 确保已安装 akshare\n3. 使用CSV导入功能手动导入财报数据", err)
	}

	// 先尝试东方财富
	result, err := downloadFromEastMoney(ctx, market, code, limit)
	if err == nil {
		return result, nil
	}

	// 东方财富失败，尝试腾讯财经
	result, err2 := DownloadFromTencent(ctx, market, code)
	if err2 == nil {
		return result, nil
	}

	// 都失败了，返回详细错误信息
	return nil, fmt.Errorf("下载失败:\n\n【东方财富】%v\n\n【腾讯财经】%v\n\n建议:\n1. 检查网络连接\n2. 稍后重试\n3. 使用CSV导入功能手动导入财报数据", err, err2)
}

// downloadFromEastMoney 从东方财富下载财务数据
func downloadFromEastMoney(ctx context.Context, market, code string, maxYears int) (*FinancialReportData, error) {
	fullCode := market + code // e.g. SH603501

	// 1. 分别为三张表确定 companyType（保险/金融股各表可能不同）
	bsCT, err := getCompanyTypeForEndpoint(ctx, "zcfzbAjaxNew", fullCode)
	if err != nil {
		return nil, fmt.Errorf("无法确定资产负债表类型: %w\n可能原因: 网络连接问题或股票代码不存在", err)
	}
	isCT, err := getCompanyTypeForEndpoint(ctx, "lrbAjaxNew", fullCode)
	if err != nil {
		return nil, fmt.Errorf("无法确定利润表类型: %w\n可能原因: 网络连接问题或股票代码不存在", err)
	}
	cfCT, err := getCompanyTypeForEndpoint(ctx, "xjllbAjaxNew", fullCode)
	if err != nil {
		return nil, fmt.Errorf("无法确定现金流量表类型: %w\n可能原因: 网络连接问题或股票代码不存在", err)
	}

	// 2. 获取所有报告日期（年报+季报）
	allDates, err := getReportDates(ctx, code, maxYears)
	if err != nil {
		return nil, fmt.Errorf("获取报告日期列表失败: %w\n可能原因: 网络连接不稳定或该股票暂无财务数据", err)
	}
	if len(allDates) == 0 {
		return nil, fmt.Errorf("未找到任何财报数据，该股票可能尚未发布财报")
	}
	// Years 只保留年报（供年度分析使用）
	annualDates := filterAnnualDates(allDates)

	// 3. 并发下载三张表（遍历所有报告期，包括季报）
	result := &FinancialReportData{
		Symbol:          code,
		Years:           annualDates,
		BalanceSheet:    make(map[string]map[string]float64),
		IncomeStatement: make(map[string]map[string]float64),
		CashFlow:        make(map[string]map[string]float64),
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []string
	successCount := 0

	for _, date := range allDates {
		// balance sheet
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			data, err := fetchHSF10(ctx, "zcfzbAjaxNew", bsCT, d, fullCode)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("资产负债表[%s]: %v", d, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			mergeBalanceSheet(result.BalanceSheet, data, d)
			successCount++
			mu.Unlock()
		}(date)

		// income statement
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			data, err := fetchHSF10(ctx, "lrbAjaxNew", isCT, d, fullCode)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("利润表[%s]: %v", d, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			mergeIncomeStatement(result.IncomeStatement, data, d)
			successCount++
			mu.Unlock()
		}(date)

		// cash flow
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			data, err := fetchHSF10(ctx, "xjllbAjaxNew", cfCT, d, fullCode)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprintf("现金流量表[%s]: %v", d, err))
				mu.Unlock()
				return
			}
			mu.Lock()
			mergeCashFlow(result.CashFlow, data, d)
			successCount++
			mu.Unlock()
		}(date)
	}

	wg.Wait()

	// 只要有部分数据成功就返回，不完全失败
	if successCount == 0 {
		return nil, fmt.Errorf("下载失败，所有年份数据均无法获取:\n%s", strings.Join(errs, "\n"))
	}

	// 如果有部分失败，记录日志但不阻止返回
	if len(errs) > 0 {
		// 可以在这里添加日志记录
		_ = errs
	}

	return result, nil
}

// FetchCashFlowDividendFromEastMoney 从东方财富现金流量表获取分红支出数据
// 用于补充其他数据源（如StockFinLens）缺失的分红字段
func FetchCashFlowDividendFromEastMoney(ctx context.Context, market, code string, maxYears int) (map[string]float64, error) {
	if maxYears <= 0 {
		maxYears = 5
	}
	fullCode := market + code

	// 1. 确定现金流量表类型
	cfCT, err := getCompanyTypeForEndpoint(ctx, "xjllbAjaxNew", fullCode)
	if err != nil {
		return nil, fmt.Errorf("无法确定现金流量表类型: %w", err)
	}

	// 2. 获取年报日期列表
	allDates, err := getReportDates(ctx, code, maxYears)
	if err != nil {
		return nil, fmt.Errorf("获取报告日期失败: %w", err)
	}
	dates := filterAnnualDates(allDates)

	result := make(map[string]float64)
	for _, d := range dates {
		data, err := fetchHSF10(ctx, "xjllbAjaxNew", cfCT, d, fullCode)
		if err != nil {
			continue
		}
		if v, ok := data["ASSIGN_DIVIDEND_PORFIT"]; ok && v != nil {
			dividend := extractFloat(v)
			if dividend != 0 {
				result[d] = dividend
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("未找到分红数据")
	}
	return result, nil
}

// getCompanyTypeForEndpoint 尝试多个 companyType 直到指定 endpoint 返回有效数据
func getCompanyTypeForEndpoint(ctx context.Context, endpoint, fullCode string) (int, error) {
	companyTypes := []int{4, 1, 2, 3, 5, 6, 7, 8}
	var lastErr error

	for _, ct := range companyTypes {
		data, err := fetchHSF10(ctx, endpoint, ct, "2024-12-31", fullCode)
		if err != nil {
			lastErr = err
			continue
		}
		if data != nil && len(data) > 0 {
			return ct, nil
		}
	}

	if lastErr != nil {
		return 0, fmt.Errorf("无法确定companyType(已尝试类型%v): %w", companyTypes, lastErr)
	}
	return 0, fmt.Errorf("无法确定companyType(已尝试类型%v): 所有类型均无数据", companyTypes)
}

// getAnnualReportDates 通过 datacenter-web API 获取所有年报日期（带重试）
// getReportDates 获取所有报告日期（年报+季报），返回降序排列
func getReportDates(ctx context.Context, code string, maxYears int) ([]string, error) {
	url := fmt.Sprintf("%s?sortColumns=REPORT_DATE&sortTypes=-1&pageSize=500&pageNumber=1&reportName=RPT_DMSK_FN_BALANCE&columns=REPORT_DATE&source=WEB&filter=(SECURITY_CODE=\"%s\")", dcBaseURL, code)
	url = strings.ReplaceAll(url, `"`, "%22")

	var resp []byte
	var err error

	// 尝试主API
	resp, err = httpGetWithReferer(ctx, url, "https://data.eastmoney.com/")
	if err != nil {
		// 尝试备用接口
		backupURL := fmt.Sprintf("https://datacenter-web.eastmoney.com/api/data/v1/get?sortColumns=REPORT_DATE&sortTypes=-1&pageSize=100&pageNumber=1&reportName=RPT_FCI_PERFORMANCEE&columns=REPORT_DATE&filter=(SECURITY_CODE=\"%s\")", code)
		resp, err = httpGetWithRetry(ctx, backupURL, "https://data.eastmoney.com/", 2)
		if err != nil {
			return nil, fmt.Errorf("主API和备用API均失败: %w", err)
		}
	}

	var result dcResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}
	if !result.Success || result.Result == nil {
		return nil, fmt.Errorf("datacenter-web API 返回失败")
	}

	dateSet := make(map[string]struct{})
	for _, item := range result.Result.Data {
		dateStr, ok := item["REPORT_DATE"].(string)
		if !ok {
			continue
		}
		dateStr = strings.TrimSpace(dateStr)
		// 提取 yyyy-mm-dd 部分（去掉时间后缀）
		parts := strings.Fields(dateStr)
		if len(parts) > 0 {
			dateSet[parts[0]] = struct{}{}
		}
	}

	dates := make([]string, 0, len(dateSet))
	for d := range dateSet {
		dates = append(dates, d)
	}
	// 降序排序
	for i := 0; i < len(dates); i++ {
		for j := i + 1; j < len(dates); j++ {
			if dates[i] < dates[j] {
				dates[i], dates[j] = dates[j], dates[i]
			}
		}
	}
	// 限制数量：年报+季报，maxYears年约 maxYears*4+4 个报告期
	limit := maxYears*4 + 4
	if maxYears > 0 && len(dates) > limit {
		dates = dates[:limit]
	}
	return dates, nil
}

// filterAnnualDates 从所有报告日期中过滤出年报（12-31结尾）
func filterAnnualDates(dates []string) []string {
	var annuals []string
	for _, d := range dates {
		if strings.HasSuffix(d, "-12-31") {
			annuals = append(annuals, d)
		}
	}
	return annuals
}

func fetchHSF10(ctx context.Context, endpoint string, companyType int, date, fullCode string) (map[string]any, error) {
	url := fmt.Sprintf("%s/%s?companyType=%d&reportDateType=0&reportType=1&dates=%s&code=%s", hsf10BaseURL, endpoint, companyType, date, fullCode)
	resp, err := httpGetWithReferer(ctx, url, "https://emweb.securities.eastmoney.com/")
	if err != nil {
		return nil, err
	}

	var hsr hsf10Response
	if err := json.Unmarshal(resp, &hsr); err != nil {
		return nil, err
	}
	if hsr.Data == nil || len(hsr.Data) == 0 {
		return nil, fmt.Errorf("no data")
	}
	return hsr.Data[0], nil
}

// httpGetWithReferer 带重试机制的HTTP GET请求
func httpGetWithReferer(ctx context.Context, url, referer string) ([]byte, error) {
	return httpGetWithRetry(ctx, url, referer, 3)
}

// httpGetWithRetry 带重试的HTTP GET请求
// 此函数保留独立的 DisableKeepAlives client：东财部分反爬路径会主动关闭 keep-alive
// 复用连接，导致 EOF。其它东财调用一律走 HTTPGet 复用 sharedTransport。
func httpGetWithRetry(ctx context.Context, url, referer string, maxRetries int) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// 指数退避 + 随机抖动：避免同步请求风暴，也避免被服务端限流识别为固定节奏
			baseDelay := time.Duration(1<<attempt) * time.Second // 1s, 2s, 4s, 8s...
			jitter := time.Duration(rand.Intn(500)+100) * time.Millisecond
			select {
			case <-time.After(baseDelay + jitter):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		// 禁用 Keep-Alive，避免复用被服务端单方面关闭的陈旧连接导致 EOF
		client := &http.Client{
			Transport: &http.Transport{
				DisableKeepAlives: true,
			},
		}
		rctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		req, err := http.NewRequestWithContext(rctx, "GET", url, nil)
		if err != nil {
			cancel()
			return nil, err
		}
		req.Header.Set("User-Agent", defaultUA)
		req.Header.Set("Referer", referer)
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		req.Header.Set("Cache-Control", "no-cache")

		resp, err := client.Do(req)
		if err != nil {
			cancel()
			lastErr = fmt.Errorf("尝试 %d: 请求失败: %w", attempt+1, err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()

		if err != nil {
			lastErr = fmt.Errorf("尝试 %d: 读取响应失败: %w", attempt+1, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("尝试 %d: HTTP %d", attempt+1, resp.StatusCode)
			continue
		}

		// 去除可能的 UTF-8 BOM
		if len(body) >= 3 && body[0] == 0xEF && body[1] == 0xBB && body[2] == 0xBF {
			body = body[3:]
		}

		return body, nil
	}

	return nil, fmt.Errorf("重试 %d 次后仍然失败: %w", maxRetries, lastErr)
}

type dcResponse struct {
	Success bool `json:"success"`
	Result  *struct {
		Data []map[string]any `json:"data"`
	} `json:"result"`
}

type hsf10Response struct {
	Pages int              `json:"pages"`
	Count int              `json:"count"`
	Data  []map[string]any `json:"data"`
}

// StockProfile 股票基本资料
type StockProfile struct {
	Industry             string  `json:"industry"`
	ListingDate          string  `json:"listingDate"`
	TotalShares          float64 `json:"totalShares"`
	MarketCap            float64 `json:"marketCap"`
	PE                   float64 `json:"pe"`
	PB                   float64 `json:"pb"`
	EPS                  float64 `json:"eps"`
	Chairman             string  `json:"chairman"`
	Controller           string  `json:"controller"`
	ChairmanGender       string  `json:"chairmanGender"`
	ChairmanAge          string  `json:"chairmanAge"`
	ChairmanNationality  string  `json:"chairmanNationality"`
	ChairmanHoldRatio    string  `json:"chairmanHoldRatio"`
	PoliticalAffiliation string  `json:"politicalAffiliation"` // blue/green/空
}

// StockQuote 股票实时行情
type StockQuote struct {
	CurrentPrice          float64 `json:"currentPrice"`
	ChangePercent         float64 `json:"changePercent"`
	ChangeAmount          float64 `json:"changeAmount"`
	Volume                float64 `json:"volume"`
	TurnoverAmount        float64 `json:"turnoverAmount"`
	TurnoverRate          float64 `json:"turnoverRate"`
	Amplitude             float64 `json:"amplitude"`
	High                  float64 `json:"high"`
	Low                   float64 `json:"low"`
	Open                  float64 `json:"open"`
	PreviousClose         float64 `json:"previousClose"`
	CirculatingMarketCap  float64 `json:"circulatingMarketCap"`
	VolumeRatio           float64 `json:"volumeRatio"`
	PE                    float64 `json:"pe"`
	PB                    float64 `json:"pb"`
	DividendYield         float64 `json:"dividendYield"`
	ShareholderReturnRate float64 `json:"shareholderReturnRate"`
	MarketCap             float64 `json:"marketCap"`
	QuoteTime             string  `json:"quoteTime"`
}

// FetchStockProfile 获取股票基本资料（东方财富公司概况 + 腾讯行情补充数值）
func FetchStockProfile(ctx context.Context, market, code string) (*StockProfile, error) {
	profile := &StockProfile{}

	// 数据源1：HSF10 公司概况（行业、董事长、实控人、上市日期）
	csCode := toCsCode(market, code)
	csURL := fmt.Sprintf("https://emweb.securities.eastmoney.com/PC_HSF10/CompanySurvey/CompanySurveyAjax?code=%s", csCode)
	if csBody, csErr := httpGetWithReferer(ctx, csURL, "https://emweb.securities.eastmoney.com/"); csErr == nil {
		var csResp struct {
			Jbzl struct {
				Sshy string `json:"sshy"`
				Zjl  string `json:"zjl"`
				Dsz  string `json:"dsz"`
				Gsjj string `json:"gsjj"`
			} `json:"jbzl"`
			Fxxg struct {
				Ssrq string `json:"ssrq"`
			} `json:"fxxg"`
		}
		if json.Unmarshal(csBody, &csResp); csResp.Jbzl.Sshy != "" {
			profile.Industry = csResp.Jbzl.Sshy
		}
		if csResp.Jbzl.Dsz != "" {
			profile.Chairman = csResp.Jbzl.Dsz
		} else if csResp.Jbzl.Zjl != "" {
			profile.Chairman = csResp.Jbzl.Zjl
		}
		if csResp.Jbzl.Gsjj != "" {
			profile.Controller = extractController(csResp.Jbzl.Gsjj)
		}
		if csResp.Fxxg.Ssrq != "" {
			profile.ListingDate = csResp.Fxxg.Ssrq
		}
	}

	// 数据源2：高管管理信息（国籍）
	mgmtURL := fmt.Sprintf("https://emweb.securities.eastmoney.com/PC_HSF10/CompanyManagement/CompanyManagementAjax?code=%s", csCode)
	if mgmtBody, mgmtErr := httpGetWithReferer(ctx, mgmtURL, "https://emweb.securities.eastmoney.com/"); mgmtErr == nil {
		var mgmtResp struct {
			RptManagerList []struct {
				XM string `json:"xm"`
				ZW string `json:"zw"`
				JJ string `json:"jj"`
			} `json:"RptManagerList"`
		}
		if json.Unmarshal(mgmtBody, &mgmtResp); len(mgmtResp.RptManagerList) > 0 {
			// 需要匹配的人名：优先实控人，其次董事长/总经理
			targetName := profile.Controller
			if targetName == "" {
				targetName = profile.Chairman
			}
			for _, m := range mgmtResp.RptManagerList {
				if m.XM == targetName && m.JJ != "" {
					profile.ChairmanNationality = extractNationality(m.JJ)
					profile.PoliticalAffiliation = extractPoliticalAffiliation(m.JJ)
					break
				}
			}
			// 如果按姓名没匹配到，再按职务匹配（董事长/总经理）
			if profile.ChairmanNationality == "" {
				for _, m := range mgmtResp.RptManagerList {
					if strings.Contains(m.ZW, "董事长") || strings.Contains(m.ZW, "总经理") || strings.Contains(m.ZW, "法定代表人") {
						if m.JJ != "" {
							nat := extractNationality(m.JJ)
							if nat != "" {
								profile.ChairmanNationality = nat
								profile.PoliticalAffiliation = extractPoliticalAffiliation(m.JJ)
								break
							}
						}
					}
				}
			}
			// 若已判定为中国台湾但蓝绿属性仍为空，尝试内置映射表及百度百科推断
			if profile.ChairmanNationality == "中国台湾" && profile.PoliticalAffiliation == "" && targetName != "" {
				if pa, ok := knownTaiwanPoliticalMap[targetName]; ok && pa != "" {
					profile.PoliticalAffiliation = pa
				} else {
					profile.PoliticalAffiliation = inferPoliticalAffiliationFromBaike(ctx, targetName)
				}
			}
			// 若籍属仍为空，尝试从百度百科查找
			if profile.ChairmanNationality == "" && targetName != "" {
				profile.ChairmanNationality = inferNationalityFromBaike(ctx, targetName)
			}
		}
	}

	// 数据源3：腾讯行情接口补充市值、PE、PB 等数值字段
	if q, err := fetchQuoteFromTencent(ctx, market, code); err == nil && q != nil {
		if profile.MarketCap == 0 && q.MarketCap > 0 {
			profile.MarketCap = q.MarketCap
		}
		if profile.PE == 0 && q.PE > 0 {
			profile.PE = q.PE
		}
		if profile.PB == 0 && q.PB > 0 {
			profile.PB = q.PB
		}
		// 成交量字段作为总股本近似（腾讯接口 parts[55]/[56] 为股本，但通用性不强，暂用成交量字段占位）
		// 若后续有稳定总股本接口，可再替换
	}

	// 港股 fallback：若 HSF10 无数据，尝试 akshare
	if strings.ToUpper(market) == "HK" && profile.Industry == "" && profile.ListingDate == "" && profile.Chairman == "" {
		if hkProfile, err := fetchHKProfileFromPython(code); err == nil && hkProfile != nil {
			if profile.Industry == "" {
				profile.Industry = hkProfile.Industry
			}
			if profile.Chairman == "" {
				profile.Chairman = hkProfile.Chairman
			}
			if profile.ListingDate == "" {
				profile.ListingDate = hkProfile.ListingDate
			}
		}
	}

	// 只要有至少一项核心数据，即视为成功
	if profile.Industry != "" || profile.ListingDate != "" || profile.MarketCap > 0 || profile.PE > 0 || profile.Chairman != "" {
		return profile, nil
	}

	return nil, fmt.Errorf("未获取到股票资料数据")
}

func extractController(gsjj string) string {
	// 匹配 "控股股东为XXX" 或 "实控人为XXX"
	patterns := []string{"控股股东为([^，。；、]+)", "实际控制人为([^，。；、]+)", "实控人为([^，。；、]+)", "控股股东是([^，。；、]+)", "由([^，。；、]+)作为主发起人"}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		if m := re.FindStringSubmatch(gsjj); len(m) > 1 && strings.TrimSpace(m[1]) != "" {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func extractNationality(jj string) string {
	jj = strings.ReplaceAll(jj, " ", "")
	// 港澳台优先
	if strings.Contains(jj, "中国香港") || strings.Contains(jj, "香港居民") || strings.Contains(jj, "香港永久性居民") {
		return "中国香港"
	}
	if strings.Contains(jj, "中国澳门") || strings.Contains(jj, "澳门居民") || strings.Contains(jj, "澳门永久性居民") {
		return "中国澳门"
	}
	if strings.Contains(jj, "中国台湾") || strings.Contains(jj, "台湾居民") {
		return "中国台湾"
	}
	if strings.Contains(jj, "台湾籍") || strings.Contains(jj, "台湾人") || strings.Contains(jj, "出生于台湾") {
		return "中国台湾"
	}
	// 若简介中出现“台湾”字样（如台湾公司、台湾大学等），优先判定为中国台湾
	if strings.Contains(jj, "台湾") {
		return "中国台湾"
	}
	// 常见外籍
	if strings.Contains(jj, "美国国籍") || strings.Contains(jj, "美籍") {
		return "美国"
	}
	if strings.Contains(jj, "加拿大国籍") || strings.Contains(jj, "加拿大籍") {
		return "加拿大"
	}
	if strings.Contains(jj, "新加坡") {
		return "新加坡"
	}
	if strings.Contains(jj, "英国") {
		return "英国"
	}
	if strings.Contains(jj, "日本") {
		return "日本"
	}
	if strings.Contains(jj, "澳大利亚") {
		return "澳大利亚"
	}
	if strings.Contains(jj, "德国") {
		return "德国"
	}
	// 默认中国（东方财富接口常返回“中国籍”而非“中国国籍”）
	if strings.Contains(jj, "中国国籍") || strings.Contains(jj, "中国籍") || strings.Contains(jj, "中国公民") ||
		strings.Contains(jj, "无境外永久居留权") || strings.Contains(jj, "无永久境外居留权") {
		return "中国"
	}
	// 兜底正则：仅匹配连续汉字（避免跨越英文标点）
	re := regexp.MustCompile(`([\p{Han}]{1,10})国籍`)
	if m := re.FindStringSubmatch(jj); len(m) > 1 {
		n := strings.TrimSpace(m[1])
		if n != "" && n != "先生" && n != "女士" && n != "男" && n != "女" {
			return n
		}
	}
	return ""
}

func extractPoliticalAffiliation(jj string) string {
	jj = strings.ReplaceAll(jj, " ", "")
	green := []string{"民进党", "民主进步党", "台湾团结联盟", "台联", "时代力量", "台湾基进", "绿营"}
	blue := []string{"国民党", "中国国民党", "亲民党", "新党", "统促党", "蓝营"}
	for _, k := range green {
		if strings.Contains(jj, k) {
			return "green"
		}
	}
	for _, k := range blue {
		if strings.Contains(jj, k) {
			return "blue"
		}
	}
	return ""
}

// 已知台湾籍企业家的蓝绿属性映射表（程序内置兜底）
var knownTaiwanPoliticalMap = map[string]string{
	"郭台铭": "blue",
	"王永庆": "blue",
	"王文洋": "blue",
	"严凯泰": "blue",
}

// inferPoliticalAffiliationFromBaike 尝试从百度百科页面推断人物蓝绿属性
func inferPoliticalAffiliationFromBaike(ctx context.Context, name string) string {
	if name == "" {
		return ""
	}
	baikeURL := fmt.Sprintf("https://baike.baidu.com/item/%s", url.QueryEscape(name))
	body, err := httpGetWithReferer(ctx, baikeURL, "https://baike.baidu.com/")
	if err != nil {
		return ""
	}
	text := string(body)
	// 去掉 HTML 标签，只保留文本便于正则匹配
	reTag := regexp.MustCompile(`<[^>]+>`)
	text = reTag.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	green := []string{"民进党", "民主进步党", "绿营", "泛绿", "深绿"}
	blue := []string{"国民党", "中国国民党", "蓝营", "泛蓝", "深蓝"}

	for _, k := range green {
		pat := regexp.MustCompile(regexp.QuoteMeta(name) + `.{0,40}` + regexp.QuoteMeta(k) + `|` + regexp.QuoteMeta(k) + `.{0,40}` + regexp.QuoteMeta(name))
		if pat.MatchString(text) {
			return "green"
		}
	}
	for _, k := range blue {
		pat := regexp.MustCompile(regexp.QuoteMeta(name) + `.{0,40}` + regexp.QuoteMeta(k) + `|` + regexp.QuoteMeta(k) + `.{0,40}` + regexp.QuoteMeta(name))
		if pat.MatchString(text) {
			return "blue"
		}
	}
	return ""
}

// inferNationalityFromBaike 尝试从百度百科页面推断人物国籍/籍属
func inferNationalityFromBaike(ctx context.Context, name string) string {
	if name == "" {
		return ""
	}
	baikeURL := fmt.Sprintf("https://baike.baidu.com/item/%s", url.QueryEscape(name))
	body, err := httpGetWithReferer(ctx, baikeURL, "https://baike.baidu.com/")
	if err != nil {
		return ""
	}
	text := string(body)
	// 去掉 HTML 标签，只保留文本便于正则匹配
	reTag := regexp.MustCompile(`<[^>]+>`)
	text = reTag.ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")

	// 【第一步】优先从明确的"国籍"字段提取（最准确）
	// 匹配 国籍 中国 / 国籍：中国 / 国籍,中国 等格式
	re := regexp.MustCompile(`国籍[\s,，:：]*([\p{Han}]{1,10})(?:\s|$|，|,)`)
	if m := re.FindStringSubmatch(text); len(m) > 1 {
		n := strings.TrimSpace(m[1])
		if n != "" && n != "未知" && n != "不详" && n != "暂无" {
			return n
		}
	}

	// 【第二步】检查港澳台（要求明确的"台湾籍/香港籍"或"台湾居民"等）
	if strings.Contains(text, "中国香港") || strings.Contains(text, "香港居民") || strings.Contains(text, "香港籍") {
		return "中国香港"
	}
	if strings.Contains(text, "中国澳门") || strings.Contains(text, "澳门居民") || strings.Contains(text, "澳门籍") {
		return "中国澳门"
	}
	// 台湾：必须是"台湾籍"、"台湾居民"或"中国台湾"，不能只是"台湾"（避免"东芝家电"等误匹配）
	if strings.Contains(text, "中国台湾") || strings.Contains(text, "台湾居民") || strings.Contains(text, "台湾籍") {
		return "中国台湾"
	}
	// "出生于台湾"或"籍贯台湾"才算
	if strings.Contains(text, "出生于台湾") || strings.Contains(text, "籍贯台湾") {
		return "中国台湾"
	}

	// 【第三步】常见外籍
	if strings.Contains(text, "美国国籍") || strings.Contains(text, "美籍") || strings.Contains(text, "美国籍") {
		return "美国"
	}
	if strings.Contains(text, "加拿大国籍") || strings.Contains(text, "加拿大籍") {
		return "加拿大"
	}
	if strings.Contains(text, "新加坡国籍") || strings.Contains(text, "新加坡籍") {
		return "新加坡"
	}
	if strings.Contains(text, "英国国籍") || strings.Contains(text, "英国籍") {
		return "英国"
	}
	if strings.Contains(text, "日本国籍") || strings.Contains(text, "日本籍") {
		return "日本"
	}
	if strings.Contains(text, "澳大利亚国籍") || strings.Contains(text, "澳大利亚籍") {
		return "澳大利亚"
	}
	if strings.Contains(text, "德国国籍") || strings.Contains(text, "德国籍") {
		return "德国"
	}
	if strings.Contains(text, "法国国籍") || strings.Contains(text, "法国籍") {
		return "法国"
	}

	// 【第四步】中国籍（兜底）
	if strings.Contains(text, "中国国籍") || strings.Contains(text, "中国籍") || strings.Contains(text, "中国公民") {
		return "中国"
	}

	return ""
}

// FetchStockQuote 获取股票实时行情，先尝试腾讯财经，再东方财富
func FetchStockQuote(ctx context.Context, market, code string) (*StockQuote, error) {
	// 1. 腾讯财经（当前网络环境下最稳定）
	fmt.Printf("[FetchStockQuote] trying Tencent for %s.%s\n", market, code)
	quote, err := fetchQuoteFromTencent(ctx, market, code)
	if err == nil && quote != nil && quote.CurrentPrice > 0 {
		fmt.Printf("[FetchStockQuote] Tencent returned quote for %s.%s, price=%.2f\n", market, code, quote.CurrentPrice)
		return quote, nil
	}
	if err != nil {
		fmt.Printf("[FetchStockQuote] Tencent failed for %s.%s: %v\n", market, code, err)
	}

	// 2. 东方财富（当前网络环境下可能被屏蔽）
	var secid string
	switch strings.ToUpper(market) {
	case "SH":
		secid = "1." + code
	case "SZ":
		secid = "0." + code
	case "HK":
		secid = "116." + code
	default:
		secid = "0." + code
	}
	url := fmt.Sprintf("http://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=f2,f3,f4,f5,f6,f7,f8,f10,f15,f16,f17,f18,f20,f21,f9,f23,f133", secid)
	fmt.Printf("[FetchStockQuote] trying EastMoney: %s\n", url)
	body, err := httpGetWithReferer(ctx, url, "https://quote.eastmoney.com/")
	if err == nil {
		var resp struct {
			Data map[string]any `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil && resp.Data != nil {
			quote := &StockQuote{}
			quote.CurrentPrice = parseAnyFloat(resp.Data["f2"])
			quote.ChangePercent = parseAnyFloat(resp.Data["f3"])
			quote.ChangeAmount = parseAnyFloat(resp.Data["f4"])
			quote.Volume = parseAnyFloat(resp.Data["f5"])
			quote.TurnoverAmount = parseAnyFloat(resp.Data["f6"])
			quote.Amplitude = parseAnyFloat(resp.Data["f7"])
			quote.TurnoverRate = parseAnyFloat(resp.Data["f8"])
			quote.VolumeRatio = parseAnyFloat(resp.Data["f10"])
			quote.High = parseAnyFloat(resp.Data["f15"])
			quote.Low = parseAnyFloat(resp.Data["f16"])
			quote.Open = parseAnyFloat(resp.Data["f17"])
			quote.PreviousClose = parseAnyFloat(resp.Data["f18"])
			quote.MarketCap = parseAnyFloat(resp.Data["f20"]) * 1e8
			quote.CirculatingMarketCap = parseAnyFloat(resp.Data["f21"]) * 1e8
			quote.PE = parseAnyFloat(resp.Data["f9"])
			quote.PB = parseAnyFloat(resp.Data["f23"])
			quote.DividendYield = parseAnyFloat(resp.Data["f133"]) / 100
			if quote.CurrentPrice > 0 {
				fmt.Printf("[FetchStockQuote] EastMoney returned quote for %s.%s, price=%.2f\n", market, code, quote.CurrentPrice)
				return quote, nil
			}
			fmt.Printf("[FetchStockQuote] EastMoney returned invalid quote for %s.%s (price=0)\n", market, code)
		} else {
			fmt.Printf("[FetchStockQuote] EastMoney unmarshal fail for %s.%s\n", market, code)
		}
	} else {
		fmt.Printf("[FetchStockQuote] EastMoney request error for %s.%s: %v\n", market, code, err)
	}

	return nil, fmt.Errorf("所有行情数据源均不可用 (腾讯/东财)")
}

func fetchQuoteFromTencent(ctx context.Context, market, code string) (*StockQuote, error) {
	prefix := strings.ToLower(market)
	url := fmt.Sprintf("http://qt.gtimg.cn/q=%s%s", prefix, code)
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, err
	}
	// 腾讯接口返回 GB18030 编码
	utf8Body, err := decodeGB18030(body)
	if err != nil {
		return nil, err
	}

	text := strings.TrimSpace(string(utf8Body))
	// 格式：v_sh603501="1~豪威集团~603501~90.15~91.35~...";
	start := strings.Index(text, "=\"")
	end := strings.LastIndex(text, "\";")
	if start == -1 || end == -1 || start+2 >= end {
		return nil, fmt.Errorf("无法解析腾讯行情数据")
	}
	content := text[start+2 : end]
	parts := strings.Split(content, "~")
	if len(parts) < 47 {
		return nil, fmt.Errorf("腾讯行情数据字段不足")
	}

	isHK := strings.ToUpper(market) == "HK"

	quote := &StockQuote{}
	quote.CurrentPrice = parseStrFloat(parts[3])
	quote.PreviousClose = parseStrFloat(parts[4])
	quote.Open = parseStrFloat(parts[5])
	quote.Volume = parseStrFloat(parts[6])
	// 腾讯接口对科创板返回"股"，其他A股返回"手"，统一转为"手"
	if !isHK && strings.ToUpper(market) == "SH" && strings.HasPrefix(code, "688") {
		quote.Volume = quote.Volume / 100
	}
	quote.ChangeAmount = parseStrFloat(parts[31])
	quote.ChangePercent = parseStrFloat(parts[32])
	quote.High = parseStrFloat(parts[33])
	quote.Low = parseStrFloat(parts[34])
	if isHK {
		// 港股：parts[37] 直接是成交额
		quote.TurnoverAmount = parseStrFloat(parts[37])
	} else {
		// A股：成交额在 parts[35] 中，格式：当前价/成交量/成交额（单位：元）
		if len(parts) > 35 {
			triplet := strings.Split(parts[35], "/")
			if len(triplet) >= 3 {
				quote.TurnoverAmount = parseStrFloat(triplet[2])
			}
		}
	}
	quote.TurnoverRate = parseStrFloat(parts[38])
	quote.PE = parseStrFloat(parts[39])
	quote.Amplitude = parseStrFloat(parts[43])
	// 流通市值 / 总市值
	if len(parts) > 44 {
		quote.CirculatingMarketCap = parseStrFloat(parts[44]) * 1e8
	}
	if len(parts) > 45 {
		quote.MarketCap = parseStrFloat(parts[45]) * 1e8
	}
	// PB 字段：A股在 parts[46]，港股因为插入了英文名称，PB 在 parts[47]
	if isHK {
		if len(parts) > 47 {
			quote.PB = parseStrFloat(parts[47])
		}
	} else {
		if len(parts) > 46 {
			quote.PB = parseStrFloat(parts[46])
		}
	}
	if !isHK && len(parts) > 49 {
		quote.VolumeRatio = parseStrFloat(parts[49])
	}
	// 股息率：腾讯接口 parts[62] 为股息率（百分比数值）
	if len(parts) > 62 {
		quote.DividendYield = parseStrFloat(parts[62]) / 100
	}
	// 时间格式：A股为 YYYYMMDDHHMMSS(14位)，港股为 YYYY/MM/DD HH:MM:SS
	if len(parts) > 30 && parts[30] != "" {
		timeStr := parts[30]
		if len(timeStr) == 14 {
			quote.QuoteTime = fmt.Sprintf("%s-%s-%s %s:%s:%s",
				timeStr[0:4], timeStr[4:6], timeStr[6:8],
				timeStr[8:10], timeStr[10:12], timeStr[12:14])
		} else if len(timeStr) == 19 && timeStr[4] == '/' && timeStr[7] == '/' {
			// 港股格式 2026/04/10 16:09:25 -> 2026-04-10 16:09:25
			quote.QuoteTime = strings.Replace(timeStr, "/", "-", 2)
		}
	}

	if quote.CurrentPrice == 0 {
		return nil, fmt.Errorf("腾讯行情数据无效")
	}
	return quote, nil
}

func decodeGB18030(data []byte) ([]byte, error) {
	// 使用 golang.org/x/text/encoding/simplifiedchinese
	decoder := simplifiedchinese.GB18030.NewDecoder()
	return decoder.Bytes(data)
}

func toCsCode(market, code string) string {
	switch strings.ToUpper(market) {
	case "SH":
		return "SH" + code
	case "SZ":
		return "SZ" + code
	case "HK":
		return "HK" + code
	default:
		return "SZ" + code
	}
}

// hkProfileResult Python 脚本返回的港股资料
type hkProfileResult struct {
	Industry    string `json:"industry"`
	Chairman    string `json:"chairman"`
	ListingDate string `json:"listing_date"`
}

// fetchHKProfileScriptPath 返回 fetch_hk_profile.py 绝对路径
func fetchHKProfileScriptPath() string {
	// Priority 1: Direct check in executable directory (for packaged Windows app)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "fetch_hk_profile.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "fetch_hk_profile.py"))
	if root != "" {
		return filepath.Join(root, "scripts", "fetch_hk_profile.py")
	}
	return filepath.Join(base, "..", "scripts", "fetch_hk_profile.py")
}

// fetchHKProfileFromPython 调用 Python 脚本获取港股基本资料
func fetchHKProfileFromPython(code string) (*hkProfileResult, error) {
	script := fetchHKProfileScriptPath()
	python := resolvePythonExecutable()
	cmd := exec.Command(python, script, code)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// Windows: 隐藏 CMD 窗口
	setHideWindow(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("fetch_hk_profile.py 执行失败: %v, output: %s", err, string(output))
	}
	var result hkProfileResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析港股资料失败: %v, raw: %s", err, string(output))
	}
	return &result, nil
}

// hkFinancialsResult Python 脚本返回的港股财务数据
type hkFinancialsResult struct {
	Symbol          string                        `json:"symbol"`
	Years           []string                      `json:"years"`
	BalanceSheet    map[string]map[string]float64 `json:"balanceSheet"`
	IncomeStatement map[string]map[string]float64 `json:"incomeStatement"`
	CashFlow        map[string]map[string]float64 `json:"cashFlow"`
	Errors          []string                      `json:"errors"`
}

// fetchHKFinancialsScriptPath 返回 fetch_hk_financials.py 绝对路径
func fetchHKFinancialsScriptPath() string {
	// Priority 1: Direct check in executable directory (for packaged Windows app)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "fetch_hk_financials.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// macOS .app bundle: scripts 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", "fetch_hk_financials.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "fetch_hk_financials.py"))
	if root != "" {
		return filepath.Join(root, "scripts", "fetch_hk_financials.py")
	}
	return filepath.Join(base, "..", "scripts", "fetch_hk_financials.py")
}

// fetchHKFinancialsFromPython 调用 Python 脚本获取港股财务数据
func fetchHKFinancialsFromPython(code string, maxYears int) (*FinancialReportData, error) {
	script := fetchHKFinancialsScriptPath()
	python := resolvePythonExecutable()
	cmd := exec.Command(python, script, code, fmt.Sprintf("%d", maxYears))
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	setHideWindow(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("fetch_hk_financials.py 执行失败: %v, output: %s", err, string(output))
	}
	var result hkFinancialsResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析港股财务数据失败: %v, raw: %s", err, string(output))
	}
	if len(result.Years) == 0 {
		return nil, fmt.Errorf("未获取到任何年报数据")
	}
	return &FinancialReportData{
		Symbol:          result.Symbol,
		Years:           result.Years,
		BalanceSheet:    result.BalanceSheet,
		IncomeStatement: result.IncomeStatement,
		CashFlow:        result.CashFlow,
	}, nil
}

func parseStrFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseAnyFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	}
	return 0
}

// KlineData 单根K线数据
type KlineData struct {
	Time         string  `json:"time"`
	Open         float64 `json:"open"`
	Close        float64 `json:"close"`
	Low          float64 `json:"low"`
	High         float64 `json:"high"`
	Volume       float64 `json:"volume"`
	Amount       float64 `json:"amount"`       // 成交额（元）
	TurnoverRate float64 `json:"turnoverRate"` // 换手率（%）
}

// periodToEastMoneyKlt 将周期转换为东方财富 klt 参数。
// daily→101, weekly→102, monthly→103。
func periodToEastMoneyKlt(period string) string {
	switch period {
	case "weekly":
		return "102"
	case "monthly":
		return "103"
	default:
		return "101"
	}
}

// FetchStockKlines 获取股票历史K线数据，先尝试腾讯财经，再网易财经，最后东方财富。
// period 支持 "daily"(默认)、"weekly"、"monthly"。
func FetchStockKlines(ctx context.Context, market, code string, limit int, period string) ([]KlineData, error) {
	if period == "" {
		period = "daily"
	}

	// 1. 腾讯财经 HTTPS（当前网络环境下最稳定）
	fmt.Printf("[FetchStockKlines] trying Tencent for %s.%s, limit=%d, period=%s\n", market, code, limit, period)
	klines, err := fetchKlinesFromTencent(ctx, market, code, limit, period)
	if err == nil && len(klines) > 0 {
		fmt.Printf("[FetchStockKlines] Tencent returned %d klines for %s.%s\n", len(klines), market, code)
		return klines, nil
	}
	if err != nil {
		fmt.Printf("[FetchStockKlines] Tencent failed for %s.%s: %v\n", market, code, err)
	}

	// 2. 网易财经 CSV（仅支持日线）
	if period == "daily" || period == "" {
		fmt.Printf("[FetchStockKlines] trying NetEase for %s.%s\n", market, code)
		klines, err = fetchKlinesFromNetEase(ctx, market, code, limit)
		if err == nil && len(klines) > 0 {
			fmt.Printf("[FetchStockKlines] NetEase returned %d klines for %s.%s\n", len(klines), market, code)
			return klines, nil
		}
		if err != nil {
			fmt.Printf("[FetchStockKlines] NetEase failed for %s.%s: %v\n", market, code, err)
		}
	}

	// 3. 东方财富 HTTPS（当前网络环境下可能被屏蔽）
	var secid string
	switch strings.ToUpper(market) {
	case "SH":
		secid = "1." + code
	case "SZ":
		secid = "0." + code
	case "HK":
		secid = "116." + code
	default:
		secid = "0." + code
	}
	klt := periodToEastMoneyKlt(period)
	url := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?ut=fa5fd1943c7b386f172d6893dbfba10b&secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f116&klt=%s&fqt=1&beg=0&end=20500101&rtntype=6&lmt=%d", secid, klt, limit)
	fmt.Printf("[FetchStockKlines] trying EastMoney: %s\n", url)
	body, err := httpGetWithReferer(ctx, url, "https://quote.eastmoney.com/")
	if err == nil {
		var resp struct {
			Data struct {
				Klines []string `json:"klines"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil && len(resp.Data.Klines) > 0 {
			fmt.Printf("[FetchStockKlines] EastMoney returned %d klines, first=%s\n", len(resp.Data.Klines), resp.Data.Klines[0])
			return parseEastMoneyKlines(resp.Data.Klines), nil
		}
		prefixLen := 200
		if len(body) < prefixLen {
			prefixLen = len(body)
		}
		fmt.Printf("[FetchStockKlines] EastMoney unmarshal fail or empty klines, body prefix=%s\n", string(body)[:prefixLen])
	} else {
		fmt.Printf("[FetchStockKlines] EastMoney request error: %v\n", err)
	}

	return nil, fmt.Errorf("所有K线数据源均不可用 (腾讯/网易/东财)")
}

// FetchIndexKlines 专门拉取大盘指数 K 线，绕过 SFL/网易/Yahoo 等可能不支持指数的源，
// 直接走腾讯财经（sh000001/sz399001/sz399006 均支持）。
// 注意：腾讯指数 K 线对 limit>2000 会返回空 schema，因此内部会限制为 2000。
func FetchIndexKlines(ctx context.Context, market, code string, limit int, period string) ([]KlineData, error) {
	if period == "" {
		period = "daily"
	}
	// 腾讯指数接口 limit 超过 2000 会返回空/异常 schema
	if limit > 2000 {
		limit = 2000
	}
	fmt.Printf("[FetchIndexKlines] trying Tencent for %s.%s, limit=%d, period=%s\n", market, code, limit, period)
	klines, err := fetchKlinesFromTencent(ctx, market, code, limit, period)
	if err == nil && len(klines) > 0 {
		fmt.Printf("[FetchIndexKlines] Tencent returned %d klines for %s.%s\n", len(klines), market, code)
		return klines, nil
	}
	if err != nil {
		fmt.Printf("[FetchIndexKlines] Tencent failed for %s.%s: %v\n", market, code, err)
	}

	// 兜底：东方财富指数 K 线（不复权，klt=101/102/103）
	var secid string
	switch strings.ToUpper(market) {
	case "SH":
		secid = "1." + code
	case "SZ":
		secid = "0." + code
	default:
		secid = "0." + code
	}
	klt := periodToEastMoneyKlt(period)
	url := fmt.Sprintf("https://push2his.eastmoney.com/api/qt/stock/kline/get?ut=fa5fd1943c7b386f172d6893dbfba10b&secid=%s&fields1=f1,f2,f3,f4,f5,f6&fields2=f51,f52,f53,f54,f55,f56,f57,f58,f59,f60,f61,f116&klt=%s&fqt=0&beg=0&end=20500101&rtntype=6&lmt=%d", secid, klt, limit)
	fmt.Printf("[FetchIndexKlines] trying EastMoney: %s\n", url)
	body, err := httpGetWithReferer(ctx, url, "https://quote.eastmoney.com/")
	if err == nil {
		var resp struct {
			Data struct {
				Klines []string `json:"klines"`
			} `json:"data"`
		}
		if json.Unmarshal(body, &resp) == nil && len(resp.Data.Klines) > 0 {
			fmt.Printf("[FetchIndexKlines] EastMoney returned %d klines, first=%s\n", len(resp.Data.Klines), resp.Data.Klines[0])
			return parseEastMoneyKlines(resp.Data.Klines), nil
		}
	}
	if err != nil {
		fmt.Printf("[FetchIndexKlines] EastMoney failed for %s.%s: %v\n", market, code, err)
	}

	return nil, fmt.Errorf("指数 K 线数据源均不可用 (腾讯/东财)")
}

func parseEastMoneyKlines(lines []string) []KlineData {
	var result []KlineData
	if len(lines) == 0 {
		return result
	}

	// 自动检测偏移：东方财富 push2his 接口在不同参数下可能返回两种格式
	// 偏移格式（fields2未生效时）：日期,天数计数,开盘,收盘,最低,最高,成交量,成交额,振幅,涨跌幅,涨跌额,换手率
	// 标准格式（fields2生效时）：   日期,开盘,收盘,最高,最低,成交量,成交额,振幅,涨跌幅,涨跌额,换手率,总市值
	// 注意：两种格式中 "最低" 与 "最高" 的相对顺序是相反的！
	offset := detectEastMoneyOffset(lines[0])

	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 6+offset {
			continue
		}

		open := parseStrFloat(parts[1+offset])
		close := parseStrFloat(parts[2+offset])

		var low, high float64
		if offset == 0 {
			// 标准格式: f54=最高(parts[3]), f55=最低(parts[4])
			high = parseStrFloat(parts[3])
			low = parseStrFloat(parts[4])
		} else {
			// 偏移格式: parts[4]=最低, parts[5]=最高
			low = parseStrFloat(parts[4])
			high = parseStrFloat(parts[5])
		}

		// 基本数据校验：过滤明显异常的K线（如价格为0、high<low 等）
		if open <= 0 || close <= 0 || high <= 0 || low <= 0 || high < low {
			continue
		}

		amount := 0.0
		if len(parts) > 6+offset {
			amount = parseStrFloat(parts[6+offset])
		}
		turnoverRate := 0.0
		if len(parts) > 10+offset {
			turnoverRate = parseStrFloat(parts[10+offset])
		} else if len(parts) > 8+offset {
			turnoverRate = parseStrFloat(parts[8+offset])
		}

		result = append(result, KlineData{
			Time:         parts[0],
			Open:         open,
			Close:        close,
			High:         high,
			Low:          low,
			Volume:       parseStrFloat(parts[5+offset]),
			Amount:       amount,
			TurnoverRate: turnoverRate,
		})
	}
	if len(result) > 0 {
		fmt.Printf("[parseEastMoneyKlines] offset=%d, parsed %d klines, first=%+v\n", offset, len(result), result[0])
	}
	return result
}

// detectEastMoneyOffset 根据首行数据自动检测是否存在"天数计数"偏移列。
// 偏移格式：parts[1] 为纯整数天数计数，parts[2] 为开盘价（含小数点）
// 标准格式：parts[1] 为开盘价（含小数点）
func detectEastMoneyOffset(firstLine string) int {
	parts := strings.Split(firstLine, ",")
	if len(parts) < 7 {
		return 0
	}
	// 检查 parts[1] 是否为纯整数（不含小数点）→ 天数计数
	if !strings.Contains(parts[1], ".") {
		if _, err := strconv.Atoi(parts[1]); err == nil {
			// 再确认 parts[2] 包含小数点（开盘价通常有两位小数）
			if strings.Contains(parts[2], ".") {
				return 1
			}
		}
	}
	return 0
}

func fetchKlinesFromTencent(ctx context.Context, market, code string, limit int, period string) ([]KlineData, error) {
	if period == "" {
		period = "daily"
	}
	// 腾讯周期标识：day/week/month
	tencPeriod := "day"
	switch period {
	case "weekly":
		tencPeriod = "week"
	case "monthly":
		tencPeriod = "month"
	}

	prefix := strings.ToLower(market)
	isHK := strings.ToUpper(market) == "HK"
	var url string
	if isHK {
		// 港股腾讯接口末尾不能加 qfq
		url = fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s%s,%s,,,%d,", prefix, code, tencPeriod, limit)
	} else {
		url = fmt.Sprintf("https://web.ifzq.gtimg.cn/appstock/app/fqkline/get?param=%s%s,%s,,,%d,qfq", prefix, code, tencPeriod, limit)
	}
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, err
	}

	// 先用 RawMessage 接住 data，区分 A 股(map) 和港股(可能 array / null) 两种 schema，
	// 避免对未知 schema 直接 Unmarshal panic 中断 fallback 链
	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	if envelope.Code != 0 {
		return nil, fmt.Errorf("Tencent kline error code=%d msg=%s", envelope.Code, envelope.Msg)
	}
	// 检测 data 是 map 还是其它形式;非 map 时直接 graceful 报错让 fallback 继续
	trimmedData := bytes.TrimSpace(envelope.Data)
	if len(trimmedData) == 0 || trimmedData[0] != '{' {
		return nil, fmt.Errorf("Tencent kline unsupported data schema (likely HK or empty)")
	}

	// 根据 period 选择对应的 JSON 字段名
	var qfqKey, rawKey string
	switch period {
	case "weekly":
		qfqKey = "qfqweek"
		rawKey = "week"
	case "monthly":
		qfqKey = "qfqmonth"
		rawKey = "month"
	default:
		qfqKey = "qfqday"
		rawKey = "day"
	}

	var result struct {
		Data map[string]map[string]json.RawMessage
	}
	if err := json.Unmarshal([]byte(`{"Data":`+string(envelope.Data)+`}`), &result); err != nil {
		return nil, err
	}

	key := prefix + code
	stockData, ok := result.Data[key]
	if !ok {
		return nil, fmt.Errorf("腾讯K线数据为空")
	}

	// 尝试 qfq 字段，fallback 到原始字段
	var barsRaw json.RawMessage
	if isHK {
		barsRaw = stockData[rawKey]
	} else {
		barsRaw = stockData[qfqKey]
		if len(barsRaw) == 0 || string(barsRaw) == "null" {
			barsRaw = stockData[rawKey]
		}
	}
	if len(barsRaw) == 0 || string(barsRaw) == "null" {
		return nil, fmt.Errorf("腾讯K线数据为空")
	}

	var days [][]any
	if err := json.Unmarshal(barsRaw, &days); err != nil {
		return nil, fmt.Errorf("腾讯K线数据解析失败: %w", err)
	}
	if len(days) == 0 {
		return nil, fmt.Errorf("腾讯K线数据为空")
	}

	var klines []KlineData
	for _, item := range days {
		if len(item) < 6 {
			continue
		}
		date, _ := item[0].(string)
		close := parseAnyFloat(item[2])
		// 腾讯接口的成交量单位有市场差异：科创板(SH 688)返回"股"，其他A股返回"手"
		isKCB := strings.ToUpper(market) == "SH" && strings.HasPrefix(code, "688")
		volumeRaw := parseAnyFloat(item[5])
		volume := volumeRaw
		amount := close * volumeRaw * 100 // 成交额 = 收盘价 × 手 × 100
		if isKCB {
			volume = volumeRaw / 100   // 科创板：股 → 手
			amount = close * volumeRaw // 成交额 = 收盘价 × 股数
		}
		klines = append(klines, KlineData{
			Time:   date,
			Open:   parseAnyFloat(item[1]),
			Close:  close,
			High:   parseAnyFloat(item[3]),
			Low:    parseAnyFloat(item[4]),
			Volume: volume,
			Amount: amount,
		})
	}
	return klines, nil
}

func fetchKlinesFromNetEase(ctx context.Context, market, code string, limit int) ([]KlineData, error) {
	if strings.ToUpper(market) == "HK" {
		return nil, fmt.Errorf("网易财经暂不支持港股K线")
	}
	var neteaseCode string
	if strings.ToUpper(market) == "SH" {
		neteaseCode = "0" + code
	} else {
		neteaseCode = "1" + code
	}
	end := time.Now().Format("20060102")
	start := time.Now().AddDate(-1, 0, 0).Format("20060102")
	url := fmt.Sprintf("http://quotes.money.163.com/service/chddata.html?code=%s&start=%s&end=%s&fields=TCLOSE;HIGH;LOW;TOPEN;LCLOSE;VOTURNOVER;VATURNOVER", neteaseCode, start, end)

	rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	body, err := HTTPGet(rctx, url)
	if err != nil {
		return nil, err
	}

	// 网易返回 GB2312 编码的 CSV
	reader := transform.NewReader(bytes.NewReader(body), simplifiedchinese.GBK.NewDecoder())
	csvReader := csv.NewReader(reader)
	csvReader.LazyQuotes = true
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) <= 1 {
		return nil, fmt.Errorf("网易K线数据为空")
	}

	var klines []KlineData
	// 跳过表头，从最后一行往前取 limit 条
	for i := len(records) - 1; i >= 1; i-- {
		row := records[i]
		if len(row) < 11 {
			continue
		}
		// 日期,股票代码,名称,收盘价,最高价,最低价,开盘价,前收盘,涨跌额,涨跌幅,成交量,成交金额
		date := strings.TrimSpace(row[0])
		if date == "" {
			continue
		}
		// 转换日期格式 2024-01-01 -> 20240101（Time 字段统一使用 YYYY-MM-DD）
		date = strings.ReplaceAll(date, "-", "")
		if len(date) == 8 {
			date = date[:4] + "-" + date[4:6] + "-" + date[6:]
		}
		amount := 0.0
		if len(row) > 11 {
			amount = parseStrFloat(row[11]) // 成交金额（元）
		}
		klines = append(klines, KlineData{
			Time:   date,
			Open:   parseStrFloat(row[6]),
			High:   parseStrFloat(row[4]),
			Low:    parseStrFloat(row[5]),
			Close:  parseStrFloat(row[3]),
			Volume: parseStrFloat(row[10]) / 100, // 网易成交量是股数，转为手
			Amount: amount,
		})
		if len(klines) >= limit {
			break
		}
	}
	// 恢复时间正序
	for i, j := 0, len(klines)-1; i < j; i, j = i+1, j-1 {
		klines[i], klines[j] = klines[j], klines[i]
	}
	if len(klines) == 0 {
		return nil, fmt.Errorf("网易K线数据为空")
	}
	return klines, nil
}
