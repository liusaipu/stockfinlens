package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ========== 数据类型 ==========

// HotConcept 单个热门概念板块
type HotConcept struct {
	Code         string  `json:"code"`           // 板块代码，如 BK1432
	Name         string  `json:"name"`           // 板块名称，如 "固态电池"
	ChangePct    float64 `json:"change_pct"`     // 涨跌幅%
	ChangeAmt    float64 `json:"change_amt"`     // 涨跌额
	Volume       float64 `json:"volume"`         // 成交量（手）
	Turnover     float64 `json:"turnover"`       // 成交额
	MainInflow   float64 `json:"main_inflow"`    // 主力净流入净额
	MainInRatio  float64 `json:"main_in_ratio"`  // 主力净流入占比%
	TopStock     string  `json:"top_stock"`      // 今日流入最大股名称
	TopStockCode string  `json:"top_stock_code"` // 今日流入最大股代码
	Score        float64 `json:"score"`          // 综合得分（0-100）
}

// HotConceptBoard 当日热点看板数据
type HotConceptBoard struct {
	Date         string       `json:"date"`          // 日期 2026-04-28
	UpdatedAt    string       `json:"updated_at"`    // 更新时间
	Concepts     []HotConcept `json:"concepts"`      // 排序后的概念列表
	DataSource   string       `json:"data_source"`   // eastmoney
	CacheVersion int          `json:"cache_version"` // 缓存版本号
}

// HotConceptHistoryItem 历史热点摘要（用于"连续上榜"标记）
type HotConceptHistoryItem struct {
	Date     string   `json:"date"`
	TopNames []string `json:"top_names"` // 当天 Top 20 概念名称
}

// ConceptConstituent 概念板块成分股
type ConceptConstituent struct {
	Code              string  `json:"code"`                 // 股票代码
	Name              string  `json:"name"`                 // 股票名称
	Market            string  `json:"market"`               // 市场: SH / SZ / BJ
	ChangePct         float64 `json:"change_pct"`           // 涨跌幅%
	Price             float64 `json:"price"`                // 最新价
	MainInflow        float64 `json:"main_inflow"`          // 主力净流入
	MarketCap         float64 `json:"market_cap"`           // 总市值
	HalfYearChangePct float64 `json:"half_year_change_pct"` // 近半年涨跌幅%
}

// ========== 常量 ==========

const (
	emHotConceptFields = "f12,f14,f2,f3,f4,f5,f6,f10,f15,f62,f184,f204,f205"
	// 东财板块市场 m:90 下的 t 分类（经验证）：t:1=地域板块（河南/青海等省份）、t:2=行业板块（医疗研发外包/化学制药等 ≈500 个二级行业）、t:3=概念板块。
	// 历史踩坑：曾把 emFSIndustry 设为 t:1，结果 _concept_membership.json 里 sub_industry 全是省份板块（药明康德误判成"青海板块"）。
	emFSConcept        = "m:90+t:3"
	emFSIndustry       = "m:90+t:2"
	hotConceptCacheVer = 6 // 缓存版本号，字段映射/去重逻辑变更时递增使旧缓存失效
)

// sflHotConceptClient 用于热点概念降级的数据源客户端（由 app.go 设置）
var sflHotConceptClient *SFLClient

// SetSFLHotConceptClient 设置数据源客户端，供热点概念降级使用
func SetSFLHotConceptClient(client *SFLClient) {
	sflHotConceptClient = client
}

// ========== 核心入口 ==========

// FetchHotConceptBoard 获取当日热门概念看板（综合排序）
// dataDir 用于缓存；topN 返回前 N 个概念，若 <=0 则返回全部
func FetchHotConceptBoard(ctx context.Context, dataDir string, topN int) (*HotConceptBoard, error) {
	// 1. 检查缓存
	if board, ok := loadHotConceptCache(dataDir); ok {
		return board, nil
	}

	// 2. 请求东财 API
	concepts, err := fetchConceptBoardFromEastMoney(ctx)
	dataSource := "eastmoney"

	// 3. 东财失败，尝试 StockFinLens 同花顺热搜降级
	if err != nil && sflHotConceptClient != nil {
		fmt.Printf("[HotConcept] EastMoney failed (%v), trying StockFinLens ths_hot...\n", err)
		if sflConcepts, tErr := fetchHotConceptsFromSFL(ctx, sflHotConceptClient); tErr == nil && len(sflConcepts) > 0 {
			concepts = sflConcepts
			dataSource = "stockfinlens"
			err = nil
		} else {
			fmt.Printf("[HotConcept] StockFinLens ths_hot failed: %v\n", tErr)
		}
	}

	// 4. 都失败，使用演示数据
	if err != nil {
		fmt.Printf("[HotConcept] All APIs failed (%v)，使用演示数据\n", err)
		concepts = getMockHotConcepts()
		dataSource = "demo"
	}

	// 5. 综合打分并排序
	calcHotScore(concepts)

	// 5.5 去重：去掉名称末尾罗马数字后缀（如"体育II"和"体育III"），按基本名称保留得分最高的一个
	concepts = dedupHotConcepts(concepts)

	// 6. 截取 topN
	if topN > 0 && len(concepts) > topN {
		concepts = concepts[:topN]
	}

	// 7. 组装结果
	now := time.Now()
	board := &HotConceptBoard{
		Date:         now.Format("2006-01-02"),
		UpdatedAt:    now.Format("2006-01-02 15:04:05"),
		Concepts:     concepts,
		DataSource:   dataSource,
		CacheVersion: hotConceptCacheVer,
	}

	// 8. 保存缓存 + 归档历史
	_ = saveHotConceptCache(dataDir, board)
	_ = archiveHotConceptHistory(dataDir, board)

	return board, nil
}

// fetchHotConceptsFromSFL 通过 SFL 同花顺热搜获取热点概念
func fetchHotConceptsFromSFL(ctx context.Context, client *SFLClient) ([]HotConcept, error) {
	items, err := client.FetchThsHot(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("ths_hot 请求失败: %w", err)
	}

	// 过滤概念板块，按概念名称去重（保留热度最高的记录）
	seen := make(map[string]*HotConcept)
	for _, item := range items {
		if item.DataType != "概念板块" {
			continue
		}
		if item.Name == "" {
			continue
		}
		// 同概念保留热度最高的记录
		if existing, ok := seen[item.Name]; !ok || item.Hot > existing.MainInflow {
			seen[item.Name] = &HotConcept{
				Code:         item.TsCode,
				Name:         item.Name,
				ChangePct:    item.PctChange,
				ChangeAmt:    0,
				Volume:       0,
				Turnover:     0,
				MainInflow:   item.Hot, // 用热度值近似主力净流入
				MainInRatio:  0,
				TopStock:     "",
				TopStockCode: "",
				Score:        0,
			}
		}
	}

	if len(seen) == 0 {
		return nil, fmt.Errorf("未获取到概念板块数据")
	}

	result := make([]HotConcept, 0, len(seen))
	for _, v := range seen {
		result = append(result, *v)
	}

	// 按热度值降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].MainInflow > result[j].MainInflow
	})

	fmt.Printf("[HotConcept] StockFinLens ths_hot fetched %d concepts\n", len(result))
	return result, nil
}

// getMockHotConcepts 演示数据（API 不可用时使用）
func getMockHotConcepts() []HotConcept {
	return []HotConcept{
		{Code: "BK1168", Name: "固态电池", ChangePct: 5.23, ChangeAmt: 61.34, Volume: 1523000, Turnover: 2860000000, MainInflow: 890000000, MainInRatio: 8.5, TopStock: "宁德时代", TopStockCode: "300750"},
		{Code: "BK0729", Name: "人工智能", ChangePct: 3.15, ChangeAmt: 71.56, Volume: 2100000, Turnover: 4500000000, MainInflow: 1200000000, MainInRatio: 12.3, TopStock: "科大讯飞", TopStockCode: "002230"},
		{Code: "BK0912", Name: "半导体", ChangePct: 2.80, ChangeAmt: 45.20, Volume: 1800000, Turnover: 3200000000, MainInflow: 650000000, MainInRatio: 7.8, TopStock: "中芯国际", TopStockCode: "688981"},
		{Code: "BK1023", Name: "创新药", ChangePct: 2.15, ChangeAmt: 18.50, Volume: 980000, Turnover: 2100000000, MainInflow: 420000000, MainInRatio: 5.6, TopStock: "恒瑞医药", TopStockCode: "600276"},
		{Code: "BK0854", Name: "光伏设备", ChangePct: 1.95, ChangeAmt: 22.30, Volume: 1200000, Turnover: 1850000000, MainInflow: 380000000, MainInRatio: 4.9, TopStock: "隆基绿能", TopStockCode: "601012"},
		{Code: "BK1101", Name: "机器人", ChangePct: 1.68, ChangeAmt: 15.80, Volume: 1450000, Turnover: 1680000000, MainInflow: 310000000, MainInRatio: 4.2, TopStock: "汇川技术", TopStockCode: "300124"},
		{Code: "BK1088", Name: "低空经济", ChangePct: 1.45, ChangeAmt: 12.60, Volume: 890000, Turnover: 1350000000, MainInflow: 250000000, MainInRatio: 3.8, TopStock: "万丰奥威", TopStockCode: "002085"},
		{Code: "BK1045", Name: "商业航天", ChangePct: 1.22, ChangeAmt: 9.80, Volume: 650000, Turnover: 980000000, MainInflow: 180000000, MainInRatio: 2.9, TopStock: "中国卫星", TopStockCode: "600118"},
		{Code: "BK0999", Name: "算力", ChangePct: 0.95, ChangeAmt: 7.50, Volume: 780000, Turnover: 1120000000, MainInflow: 150000000, MainInRatio: 2.5, TopStock: "浪潮信息", TopStockCode: "000977"},
		{Code: "BK1077", Name: "黄金概念", ChangePct: 0.82, ChangeAmt: 6.20, Volume: 520000, Turnover: 850000000, MainInflow: 120000000, MainInRatio: 2.1, TopStock: "山东黄金", TopStockCode: "600547"},
		{Code: "BK1133", Name: "消费电子", ChangePct: 0.65, ChangeAmt: 4.80, Volume: 680000, Turnover: 920000000, MainInflow: 95000000, MainInRatio: 1.8, TopStock: "立讯精密", TopStockCode: "002475"},
		{Code: "BK1066", Name: "智能驾驶", ChangePct: 0.48, ChangeAmt: 3.50, Volume: 450000, Turnover: 720000000, MainInflow: 70000000, MainInRatio: 1.5, TopStock: "德赛西威", TopStockCode: "002920"},
		{Code: "BK1011", Name: "军工", ChangePct: 0.35, ChangeAmt: 2.80, Volume: 380000, Turnover: 580000000, MainInflow: 55000000, MainInRatio: 1.2, TopStock: "中航沈飞", TopStockCode: "600760"},
		{Code: "BK1033", Name: "稀土永磁", ChangePct: 0.22, ChangeAmt: 1.60, Volume: 290000, Turnover: 420000000, MainInflow: 35000000, MainInRatio: 0.9, TopStock: "北方稀土", TopStockCode: "600111"},
		{Code: "BK1055", Name: "数据要素", ChangePct: 0.15, ChangeAmt: 1.10, Volume: 210000, Turnover: 310000000, MainInflow: 22000000, MainInRatio: 0.7, TopStock: "易华录", TopStockCode: "300212"},
		{Code: "BK1099", Name: "短剧游戏", ChangePct: -0.25, ChangeAmt: -1.80, Volume: 180000, Turnover: 250000000, MainInflow: -15000000, MainInRatio: -0.5, TopStock: "掌阅科技", TopStockCode: "603533"},
		{Code: "BK1144", Name: "跨境电商", ChangePct: -0.45, ChangeAmt: -3.20, Volume: 150000, Turnover: 190000000, MainInflow: -28000000, MainInRatio: -1.2, TopStock: "跨境通", TopStockCode: "002640"},
		{Code: "BK1111", Name: "预制菜", ChangePct: -0.68, ChangeAmt: -4.50, Volume: 120000, Turnover: 145000000, MainInflow: -35000000, MainInRatio: -1.8, TopStock: "味知香", TopStockCode: "605089"},
		{Code: "BK1155", Name: "房地产", ChangePct: -0.85, ChangeAmt: -5.60, Volume: 95000, Turnover: 110000000, MainInflow: -42000000, MainInRatio: -2.5, TopStock: "万科A", TopStockCode: "000002"},
		{Code: "BK1177", Name: "银行", ChangePct: -1.05, ChangeAmt: -7.80, Volume: 80000, Turnover: 95000000, MainInflow: -55000000, MainInRatio: -3.2, TopStock: "招商银行", TopStockCode: "600036"},
	}
}

// FetchHotConceptHistory 获取最近 days 天的历史热点摘要
func FetchHotConceptHistory(dataDir string, days int) ([]HotConceptHistoryItem, error) {
	historyDir := filepath.Join(dataDir, "hot_concepts", "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []HotConceptHistoryItem{}, nil
		}
		return nil, err
	}

	// 收集所有日期文件
	var dates []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".json") {
			dates = append(dates, strings.TrimSuffix(name, ".json"))
		}
	}
	// 降序排列
	sort.Slice(dates, func(i, j int) bool {
		return dates[i] > dates[j]
	})
	if len(dates) > days {
		dates = dates[:days]
	}

	result := make([]HotConceptHistoryItem, 0, len(dates))
	for _, d := range dates {
		path := filepath.Join(historyDir, d+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var board HotConceptBoard
		if err := json.Unmarshal(data, &board); err != nil {
			continue
		}
		names := make([]string, 0, len(board.Concepts))
		for _, c := range board.Concepts {
			names = append(names, c.Name)
		}
		result = append(result, HotConceptHistoryItem{
			Date:     d,
			TopNames: names,
		})
	}
	return result, nil
}

// FetchConceptConstituents 获取某概念板块的成分股列表（按涨跌幅截 top 50，用于热点概念展示）
func FetchConceptConstituents(ctx context.Context, conceptCode string) ([]ConceptConstituent, error) {
	result, err := fetchConceptConstituentsRaw(ctx, conceptCode, 200)
	if err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ChangePct > result[j].ChangePct
	})
	if len(result) > 50 {
		result = result[:50]
	}
	return result, nil
}

// FetchAllConceptConstituents 获取某板块的全量成分股（不排序、不截断，用于反查表构建）
// 与 FetchConceptConstituents 的差异：分页翻页 + 不做 top-50 截断。
//
// 历史踩坑：原来单次 pz=500 实测被东财 API clamp 到 100 条，导致"智能穿戴"等热门板块
// 反查表里只记录了 100 家而非真实 500+ 家，IDF 算出来全是 ~3.76 分不出区分度。
// 改为 pn=1,2,3... 翻页，直到 len(diff)<pageSize 或累计 ≥ total。
func FetchAllConceptConstituents(ctx context.Context, conceptCode string) ([]ConceptConstituent, error) {
	return fetchConceptConstituentsPaginated(ctx, conceptCode, 100)
}

// fetchConceptConstituentsPaginated 翻页拉取板块成分股全集
// pageSize 实测 100 是 API 隐式 clamp 的最大值，传更大也不会增加返回数量
func fetchConceptConstituentsPaginated(ctx context.Context, conceptCode string, pageSize int) ([]ConceptConstituent, error) {
	if mock := getMockConstituents(conceptCode); len(mock) > 0 {
		return mock, nil
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 100
	}

	const maxPages = 30 // 防御性上限：单板块成分股 ~3000，30 页足够覆盖
	var all []ConceptConstituent
	seen := make(map[string]struct{})
	total := -1

	for pn := 1; pn <= maxPages; pn++ {
		page, pageTotal, err := fetchConstituentsOnePage(ctx, conceptCode, pn, pageSize)
		if err != nil {
			if pn == 1 {
				return nil, err
			}
			break // 中途失败：返回已拿到的部分，不整体失败
		}
		if total < 0 {
			total = pageTotal
		}
		if len(page) == 0 {
			break
		}
		for _, c := range page {
			if c.Code == "" {
				continue
			}
			if _, ok := seen[c.Code]; ok {
				continue
			}
			seen[c.Code] = struct{}{}
			all = append(all, c)
		}
		// 终止条件：本页返回不足 pageSize（最后一页）或已累计满 total
		if len(page) < pageSize {
			break
		}
		if total > 0 && len(all) >= total {
			break
		}
	}
	return all, nil
}

// fetchConstituentsOnePage 拉取成分股单页，返回 (本页数据, total, err)
func fetchConstituentsOnePage(ctx context.Context, conceptCode string, pn, pageSize int) ([]ConceptConstituent, int, error) {
	url := fmt.Sprintf(
		"http://push2.eastmoney.com/api/qt/clist/get?pn=%d&pz=%d&po=1&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=f3&fs=b:%s&fields=f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f12,f13,f14,f15,f16,f17,f18,f20,f21,f23,f24,f25,f22,f11,f62,f128,f136,f115,f152&_=%d",
		pn, pageSize, conceptCode, time.Now().UnixMilli(),
	)
	body, err := httpGetEastMoney(ctx, url)
	if err != nil {
		return nil, 0, fmt.Errorf("成分股 API 请求失败: %w", err)
	}
	body = stripJSONP(body)

	var check struct {
		RC   int `json:"rc"`
		Data struct {
			Total int           `json:"total"`
			Diff  []interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &check); err != nil {
		return nil, 0, fmt.Errorf("解析成分股响应失败: %w", err)
	}
	if check.RC != 0 && check.RC != 200 {
		return nil, 0, fmt.Errorf("成分股 API 返回错误码: %d", check.RC)
	}
	if pn == 1 && check.Data.Total == 0 && len(check.Data.Diff) == 0 {
		return nil, 0, fmt.Errorf("该概念暂无成分股数据")
	}

	result := make([]ConceptConstituent, 0, len(check.Data.Diff))
	for _, raw := range check.Data.Diff {
		var c ConceptConstituent
		switch item := raw.(type) {
		case map[string]interface{}:
			c = ConceptConstituent{
				Code:              parseString(item["f12"]),
				Name:              parseString(item["f14"]),
				Market:            parseMarket(parseString(item["f13"]), parseString(item["f12"])),
				Price:             parseFloat(item["f2"]),
				ChangePct:         parseFloat(item["f3"]),
				MarketCap:         parseFloat(item["f20"]),
				HalfYearChangePct: parseFloat(item["f130"]),
				MainInflow:        parseFloat(item["f62"]),
			}
		case []interface{}:
			if len(item) >= 7 {
				code := parseString(item[0])
				c = ConceptConstituent{
					Code:              code,
					Name:              parseString(item[1]),
					Market:            inferMarketFromCode(code),
					Price:             parseFloat(item[2]),
					ChangePct:         parseFloat(item[3]),
					MarketCap:         parseFloat(item[4]),
					HalfYearChangePct: parseFloat(item[5]),
					MainInflow:        parseFloat(item[6]),
				}
			}
		}
		if c.Code == "" {
			continue
		}
		result = append(result, c)
	}
	return result, check.Data.Total, nil
}

// fetchConceptConstituentsRaw 内部公共实现：拉取成分股原始列表，不排序、不截断。
// 用于"非反查表"场景（如热门板块前 50 成分股展示），不翻页。
func fetchConceptConstituentsRaw(ctx context.Context, conceptCode string, pageSize int) ([]ConceptConstituent, error) {
	// 演示数据优先：当概念列表使用演示数据时，成分股也用演示数据，
	// 避免用演示数据的假代码去查东财真API（代码映射不匹配会导致成分股完全错误）
	if mock := getMockConstituents(conceptCode); len(mock) > 0 {
		return mock, nil
	}

	if pageSize <= 0 {
		pageSize = 200
	}

	url := fmt.Sprintf(
		"http://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=%d&po=1&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=f3&fs=b:%s&fields=f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f12,f13,f14,f15,f16,f17,f18,f20,f21,f23,f24,f25,f22,f11,f62,f128,f136,f115,f152&_=%d",
		pageSize,
		conceptCode,
		time.Now().UnixMilli(),
	)

	body, err := httpGetEastMoney(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("成分股 API 请求失败: %w", err)
	}

	// 东财偶尔返回 JSONP，去除 callback 包装
	body = stripJSONP(body)

	// 先解析为通用结构，检查 total
	var check struct {
		RC   int `json:"rc"`
		Data struct {
			Total int           `json:"total"`
			Diff  []interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &check); err != nil {
		return nil, fmt.Errorf("解析成分股响应失败: %w", err)
	}
	if check.RC != 0 && check.RC != 200 {
		return nil, fmt.Errorf("成分股 API 返回错误码: %d", check.RC)
	}
	if check.Data.Total == 0 && len(check.Data.Diff) == 0 {
		return nil, fmt.Errorf("该概念暂无成分股数据")
	}

	result := make([]ConceptConstituent, 0, len(check.Data.Diff))
	for _, raw := range check.Data.Diff {
		var c ConceptConstituent
		switch item := raw.(type) {
		case map[string]interface{}:
			// 对象格式: {"f12":"300750","f14":"宁德时代","f13":0,...}
			c = ConceptConstituent{
				Code:              parseString(item["f12"]),
				Name:              parseString(item["f14"]),
				Market:            parseMarket(parseString(item["f13"]), parseString(item["f12"])),
				Price:             parseFloat(item["f2"]),
				ChangePct:         parseFloat(item["f3"]),
				MarketCap:         parseFloat(item["f20"]),
				HalfYearChangePct: parseFloat(item["f130"]),
				MainInflow:        parseFloat(item["f62"]),
			}
		case []interface{}:
			// 数组格式: ["300750","宁德时代",185.2,4.52,320000000]
			if len(item) >= 7 {
				code := parseString(item[0])
				c = ConceptConstituent{
					Code:              code,
					Name:              parseString(item[1]),
					Market:            inferMarketFromCode(code),
					Price:             parseFloat(item[2]),
					ChangePct:         parseFloat(item[3]),
					MarketCap:         parseFloat(item[4]),
					HalfYearChangePct: parseFloat(item[5]),
					MainInflow:        parseFloat(item[6]),
				}
			}
		}
		if c.Code == "" {
			continue
		}
		result = append(result, c)
	}
	return result, nil
}

// getMockConstituents 演示成分股数据
func getMockConstituents(conceptCode string) []ConceptConstituent {
	// 根据概念代码返回不同的演示数据
	mocks := map[string][]ConceptConstituent{
		"BK1168": { // 固态电池
			{Code: "300750", Name: "宁德时代", Market: "SZ", Price: 185.20, ChangePct: 4.52, MarketCap: 185200000000, HalfYearChangePct: 14.04, MainInflow: 320000000},
			{Code: "002074", Name: "国轩高科", Market: "SZ", Price: 21.35, ChangePct: 3.18, MarketCap: 21350000000, HalfYearChangePct: 11.36, MainInflow: 85000000},
			{Code: "300014", Name: "亿纬锂能", Market: "SZ", Price: 42.10, ChangePct: 2.95, MarketCap: 42100000000, HalfYearChangePct: 10.9, MainInflow: 62000000},
			{Code: "002709", Name: "天赐材料", Market: "SZ", Price: 18.60, ChangePct: 2.30, MarketCap: 18600000000, HalfYearChangePct: 9.6, MainInflow: 45000000},
			{Code: "603659", Name: "璞泰来", Market: "SH", Price: 15.80, ChangePct: 1.85, MarketCap: 15800000000, HalfYearChangePct: 8.7, MainInflow: 28000000},
		},
		"BK0729": { // 人工智能
			{Code: "002230", Name: "科大讯飞", Market: "SZ", Price: 48.50, ChangePct: 3.80, MarketCap: 48500000000, HalfYearChangePct: 12.6, MainInflow: 210000000},
			{Code: "000938", Name: "中芯国际", Market: "SZ", Price: 52.30, ChangePct: 2.90, MarketCap: 52300000000, HalfYearChangePct: 10.8, MainInflow: 150000000},
			{Code: "300033", Name: "同花顺", Market: "SZ", Price: 128.00, ChangePct: 2.50, MarketCap: 128000000000, HalfYearChangePct: 10.0, MainInflow: 98000000},
			{Code: "002415", Name: "海康威视", Market: "SZ", Price: 32.10, ChangePct: 1.80, MarketCap: 32100000000, HalfYearChangePct: 8.6, MainInflow: 65000000},
			{Code: "000977", Name: "浪潮信息", Market: "SZ", Price: 28.50, ChangePct: 1.20, MarketCap: 28500000000, HalfYearChangePct: 7.4, MainInflow: 42000000},
		},
		"BK0912": { // 半导体
			{Code: "688981", Name: "中芯国际", Market: "SH", Price: 52.30, ChangePct: 3.50, MarketCap: 52300000000, HalfYearChangePct: 12.0, MainInflow: 180000000},
			{Code: "002371", Name: "北方华创", Market: "SZ", Price: 245.00, ChangePct: 2.80, MarketCap: 245000000000, HalfYearChangePct: 10.6, MainInflow: 120000000},
			{Code: "603501", Name: "韦尔股份", Market: "SH", Price: 98.50, ChangePct: 2.10, MarketCap: 98500000000, HalfYearChangePct: 9.2, MainInflow: 85000000},
			{Code: "688012", Name: "中微公司", Market: "SH", Price: 142.00, ChangePct: 1.60, MarketCap: 142000000000, HalfYearChangePct: 8.2, MainInflow: 55000000},
			{Code: "300782", Name: "卓胜微", Market: "SZ", Price: 78.20, ChangePct: 0.90, MarketCap: 78200000000, HalfYearChangePct: 6.8, MainInflow: 28000000},
		},
	}
	if list, ok := mocks[conceptCode]; ok {
		return list
	}
	// 未知代码必须返回 nil，让上层走真 API。
	// 原本这里返回 {平安银行, 万科A, 茅台} 默认三只兜底——结果反查表里所有 500+ 板块的
	// "成分股"都被替换成这三只，药明康德的二级行业全错。
	return nil
}

// stripJSONP 去除 JSONP callback 包装，如 jQuery123({...}) -> {...}
func stripJSONP(body []byte) []byte {
	s := string(body)
	s = strings.TrimSpace(s)
	if idx := strings.Index(s, "("); idx > 0 && strings.HasSuffix(s, ")") {
		return []byte(s[idx+1 : len(s)-1])
	}
	return body
}

// httpGetEastMoney 东财专用 HTTP GET：多个 CDN 节点轮询，统一使用共享 client。
func httpGetEastMoney(ctx context.Context, url string) ([]byte, error) {
	// 尝试多个东财CDN节点
	urls := []string{url}
	// 如果原始URL包含 push2.eastmoney.com，额外尝试 31-50 号CDN
	if strings.Contains(url, "push2.eastmoney.com") {
		for i := 31; i <= 50; i++ {
			urls = append(urls, strings.Replace(url, "push2.eastmoney.com", fmt.Sprintf("%d.push2.eastmoney.com", i), 1))
		}
	}

	for _, u := range urls {
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		body, err := HTTPGet(rctx, u,
			WithReferer("https://quote.eastmoney.com/"),
			WithHeader("Accept", "*/*"),
		)
		cancel()
		if err == nil && len(body) > 0 {
			return body, nil
		}
	}
	return nil, fmt.Errorf("所有CDN节点请求失败")
}

// ========== 内部：东财 API 封装 ==========

func fetchConceptBoardFromEastMoney(ctx context.Context) ([]HotConcept, error) {
	url := fmt.Sprintf(
		"http://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=500&po=1&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=f3&fs=%s&fields=%s&_=%d",
		emFSConcept,
		emHotConceptFields,
		time.Now().UnixMilli(),
	)

	body, err := httpGetEastMoney(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("请求概念板块失败: %w", err)
	}

	// 东财偶尔返回 JSONP，去除 callback 包装
	body = stripJSONP(body)

	var resp struct {
		RC   int `json:"rc"`
		Data struct {
			Total int                      `json:"total"`
			Diff  []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, raw=%s", err, string(body[:min(len(body), 200)]))
	}
	if resp.RC != 0 && resp.RC != 200 {
		return nil, fmt.Errorf("API 返回错误码 rc=%d", resp.RC)
	}

	concepts := make([]HotConcept, 0, len(resp.Data.Diff))
	for _, item := range resp.Data.Diff {
		code := parseString(item["f12"])
		name := parseString(item["f14"])
		// m:90+t:3 可能返回纯数字，成分股查询 fs=b: 需要 BK 前缀
		if code != "" && !strings.HasPrefix(code, "BK") {
			code = "BK" + code
		}
		c := HotConcept{
			Code:         code,
			Name:         name,
			ChangePct:    parseFloat(item["f3"]),
			ChangeAmt:    parseFloat(item["f4"]),
			Volume:       parseFloat(item["f5"]),
			Turnover:     parseFloat(item["f6"]),
			MainInflow:   parseFloat(item["f62"]),
			MainInRatio:  parseFloat(item["f184"]),
			TopStock:     parseString(item["f204"]),
			TopStockCode: parseString(item["f205"]),
		}
		if c.Code == "" {
			continue
		}
		concepts = append(concepts, c)
	}
	return concepts, nil
}

// ========== 内部：综合打分 ==========

func calcHotScore(concepts []HotConcept) {
	if len(concepts) == 0 {
		return
	}

	// 求三个维度的最大值（用于归一化）
	maxChange := 0.0
	maxInflow := 0.0
	maxTurnover := 0.0
	for _, c := range concepts {
		if abs(c.ChangePct) > maxChange {
			maxChange = abs(c.ChangePct)
		}
		if abs(c.MainInflow) > maxInflow {
			maxInflow = abs(c.MainInflow)
		}
		if c.Turnover > maxTurnover {
			maxTurnover = c.Turnover
		}
	}

	// 避免除以 0
	if maxChange == 0 {
		maxChange = 1
	}
	if maxInflow == 0 {
		maxInflow = 1
	}
	if maxTurnover == 0 {
		maxTurnover = 1
	}

	const (
		wChange   = 0.40
		wInflow   = 0.40
		wTurnover = 0.20
	)

	for i := range concepts {
		c := &concepts[i]
		normChange := abs(c.ChangePct) / maxChange
		normInflow := abs(c.MainInflow) / maxInflow
		normTurnover := c.Turnover / maxTurnover

		// 主力净流入方向修正：若为负，权重减半
		inflowDir := 1.0
		if c.MainInflow < 0 {
			inflowDir = 0.5
		}

		score := wChange*normChange + wInflow*normInflow*inflowDir + wTurnover*normTurnover
		c.Score = math.Round(score*100*100) / 100 // 0-100，保留两位小数
	}

	// 按 Score 降序排序
	sort.Slice(concepts, func(i, j int) bool {
		return concepts[i].Score > concepts[j].Score
	})
}

// dedupHotConcepts 去掉名称末尾罗马数字后缀的重复概念（如"体育II"和"体育III"），
// 按基本名称保留得分最高的一个。
func dedupHotConcepts(concepts []HotConcept) []HotConcept {
	if len(concepts) == 0 {
		return concepts
	}
	seen := make(map[string]*HotConcept)
	for i := range concepts {
		c := &concepts[i]
		base := stripRomanSuffix(c.Name)
		if existing, ok := seen[base]; !ok || c.Score > existing.Score {
			seen[base] = c
		}
	}
	result := make([]HotConcept, 0, len(seen))
	for _, c := range seen {
		result = append(result, *c)
	}
	// 重新按得分降序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	return result
}

// stripRomanSuffix 去掉概念名称末尾的罗马数字/中文数字后缀
func stripRomanSuffix(name string) string {
	// 匹配末尾的罗马数字或中文数字：ⅠⅡⅢⅣⅤ一二三四五六七八九十
	for len(name) > 0 {
		r := []rune(name)
		last := r[len(r)-1]
		if last == 'Ⅰ' || last == 'Ⅱ' || last == 'Ⅲ' || last == 'Ⅳ' || last == 'Ⅴ' ||
			last == '一' || last == '二' || last == '三' || last == '四' || last == '五' ||
			last == '六' || last == '七' || last == '八' || last == '九' || last == '十' {
			name = string(r[:len(r)-1])
			continue
		}
		break
	}
	return name
}

// ========== 内部：缓存与归档 ==========

func hotConceptCachePath(dataDir string) string {
	return filepath.Join(dataDir, "hot_concepts", "latest.json")
}

func hotConceptHistoryPath(dataDir, date string) string {
	return filepath.Join(dataDir, "hot_concepts", "history", date+".json")
}

func loadHotConceptCache(dataDir string) (*HotConceptBoard, bool) {
	path := hotConceptCachePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var board HotConceptBoard
	if err := json.Unmarshal(data, &board); err != nil {
		return nil, false
	}
	// 版本号不匹配：字段映射变更后旧缓存失效
	if board.CacheVersion != hotConceptCacheVer {
		fmt.Printf("[HotConcept] cache version mismatch (%d != %d), ignoring\n", board.CacheVersion, hotConceptCacheVer)
		return nil, false
	}
	// 检查是否过期：4 小时（数据源 ths_hot 限 2 次/天，延长缓存减少 API 调用）
	if t, err := time.ParseInLocation("2006-01-02 15:04:05", board.UpdatedAt, time.Local); err == nil {
		if time.Since(t) < 4*time.Hour {
			return &board, true
		}
	}
	return nil, false
}

func saveHotConceptCache(dataDir string, board *HotConceptBoard) error {
	path := hotConceptCachePath(dataDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func archiveHotConceptHistory(dataDir string, board *HotConceptBoard) error {
	path := hotConceptHistoryPath(dataDir, board.Date)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	// 若当天已归档，覆盖
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ========== 工具函数 ==========

func parseString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func parseFloat(v interface{}) float64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return val
	case string:
		if val == "-" || val == "" {
			return 0
		}
		f, _ := strconv.ParseFloat(val, 64)
		return f
	default:
		return 0
	}
}

// parseMarket 将东财 f13 字段解析为市场代码
// f13: 0=深圳, 1=上海, 2=北京
func parseMarket(f13, code string) string {
	switch f13 {
	case "1":
		return "SH"
	case "0":
		return "SZ"
	case "2":
		return "BJ"
	}
	// fallback: 通过代码前缀推断
	return inferMarketFromCode(code)
}

// inferMarketFromCode 通过 A 股代码前缀推断市场
func inferMarketFromCode(code string) string {
	if len(code) == 0 {
		return "SH"
	}
	prefix := code[0]
	if prefix == '6' || prefix == '5' {
		return "SH"
	}
	if prefix == '0' || prefix == '3' {
		return "SZ"
	}
	if prefix == '8' || prefix == '4' {
		return "BJ"
	}
	return "SH"
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
