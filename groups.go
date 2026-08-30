package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/liusaipu/stockfinlens/analyzer"
	"github.com/liusaipu/stockfinlens/downloader"
)

// StockGroup 自选股分组。一只股票可属多个组。
type StockGroup struct {
	ID          string   `json:"id"`                    // 创建时生成（如 "g_" + 时间戳/随机）
	Name        string   `json:"name"`                  // 组名
	Source      string   `json:"source"`                // "concept" | "manual"
	ConceptName string   `json:"conceptName,omitempty"` // source=concept 时对应东财概念板块名
	Codes       []string `json:"codes"`                 // 组内股票代码
}

// GroupSuggestion 基于概念反查表的分组建议
type GroupSuggestion struct {
	ConceptName string   `json:"conceptName"` // 概念板块名
	Codes       []string `json:"codes"`       // 属于该概念的自选股
	Score       float64  `json:"score"`       // IDF 权重，越小众的概念分越高，供前端排序展示
}

// GroupComparisonRow 组内股票对比行
type GroupComparisonRow struct {
	Code string `json:"code"`
	Name string `json:"name"`
	// 基本面（未分析过时 Analyzed=false，相关字段为 0，前端显示"未分析"）
	Analyzed  bool    `json:"analyzed"`
	YearScore float64 `json:"yearScore"` // 18步年度总分（最新年度）
	Grade     string  `json:"grade"`
	AScore    float64 `json:"aScore"` // 风险分，越高越危险
	// 同比与财务（来自 MarketCacheItem；无缓存时 InMarketCache=false）
	InMarketCache   bool    `json:"inMarketCache"`
	RevenueGrowth   float64 `json:"revenueGrowth"`
	NetProfitGrowth float64 `json:"netProfitGrowth"`
	ROE             float64 `json:"roe"`
	GrossMargin     float64 `json:"grossMargin"`
	DebtRatio       float64 `json:"debtRatio"`    // 资产负债率
	HasDebtRatio    bool    `json:"hasDebtRatio"` // 负债率是否有效
	CashRatio       float64 `json:"cashRatio"`    // 净利润现金含量（来自分析快照 step15）
	HasCashRatio    bool    `json:"hasCashRatio"` // 现金含量是否有效
	// 活跃度（复用 GetWatchlistActivity 的数据源：本地活跃度缓存）
	ActivityScore float64 `json:"activityScore"`
	HasActivity   bool    `json:"hasActivity"` // 活跃度是否来自真实缓存（ false=缺失用中位数替代）
	Stars         int     `json:"stars"`
	// 热度
	ChangePercent         float64 `json:"changePercent"`         // 当日涨跌幅（托盘报价内存缓存 / 本地行情缓存）
	HalfYearChangePercent float64 `json:"halfYearChangePercent"` // 半年涨幅（本地日K缓存，不发起网络请求）
	HasHalfYearChange     bool    `json:"hasHalfYearChange"`     // 半年涨幅是否有效
	ThsHotRank            int     `json:"thsHotRank"`            // 同花顺热搜排名，0=未上榜/不可用
	ThsHotValue           float64 `json:"thsHotValue"`
	// 综合得分（基于模块 4.2 固定档位加权算法，跨池可比）
	CompositeScore float64 `json:"compositeScore"` // 满分 100
	CompositeRank  int     `json:"compositeRank"`  // 组内排名，1 开始
}

// GroupHeat 概念组的板块热度
type GroupHeat struct {
	GroupID       string  `json:"groupId"`
	ConceptName   string  `json:"conceptName"`
	OnBoard       bool    `json:"onBoard"` // 是否在板块热度榜上
	Score         float64 `json:"score"`   // 板块热度分 0-100
	ChangePercent float64 `json:"changePercent"`
	MainInflow    float64 `json:"mainInflow"`
}

// newGroupID 生成分组 ID："g_" + 时间戳 + 随机后缀
func newGroupID() string {
	buf := make([]byte, 3)
	_, _ = rand.Read(buf)
	return fmt.Sprintf("g_%d_%s", time.Now().UnixNano(), hex.EncodeToString(buf))
}

// GetWatchlistGroups 获取全部自选分组
func (a *App) GetWatchlistGroups() ([]StockGroup, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	return a.storage.LoadWatchlistGroups()
}

// SaveWatchlistGroups 整组替换保存自选分组（与 ReorderWatchlist 同模式）
// 校验：组名非空；code 必须在自选股中，不存在的 code 直接剔除
func (a *App) SaveWatchlistGroups(groups []StockGroup) error {
	if a.storage == nil {
		return fmt.Errorf("存储未初始化")
	}
	watchlist, err := a.storage.LoadWatchlist()
	if err != nil {
		return fmt.Errorf("加载自选列表失败: %w", err)
	}
	// 自选股 code 集合
	valid := make(map[string]struct{}, len(watchlist))
	for _, item := range watchlist {
		valid[item.Code] = struct{}{}
	}
	cleaned := make([]StockGroup, 0, len(groups))
	for _, g := range groups {
		if strings.TrimSpace(g.Name) == "" {
			return fmt.Errorf("组名不能为空")
		}
		if g.ID == "" {
			g.ID = newGroupID()
		}
		if g.Source == "" {
			g.Source = "manual"
		}
		// 剔除不在自选股中的 code，并去重
		seen := make(map[string]struct{}, len(g.Codes))
		codes := make([]string, 0, len(g.Codes))
		for _, c := range g.Codes {
			if _, ok := valid[c]; !ok {
				continue
			}
			if _, dup := seen[c]; dup {
				continue
			}
			seen[c] = struct{}{}
			codes = append(codes, c)
		}
		g.Codes = codes
		cleaned = append(cleaned, g)
	}
	return a.storage.SaveWatchlistGroups(cleaned)
}

// conceptIDFScore 计算概念的 IDF 权重（越小众分越高）
// analyzer.recommend 内部 conceptIDF 的精简版：直接基于 downloader.ConceptMembership 计算，
// 避免导出 analyzer 未导出类型。df<=0 时返回 1.0（与该概念未出现在反查表时的行为一致）
func conceptIDFScore(docFreq map[string]int, totalDocs int, concept string) float64 {
	if totalDocs <= 0 {
		return 1.0
	}
	freq := docFreq[concept]
	if freq <= 0 {
		return 1.0
	}
	return math.Log(float64(totalDocs) / float64(freq))
}

// SuggestWatchlistGroups 基于概念反查表推荐分组：
// 找自选股中 ≥2 只共享的概念，按 IDF 权重降序（越小众排越前），
// 过滤掉已有同名概念组，最多返回 10 条。反查表缓存不存在时返回空列表。
func (a *App) SuggestWatchlistGroups() ([]GroupSuggestion, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	watchlist, err := a.storage.LoadWatchlist()
	if err != nil {
		return nil, fmt.Errorf("加载自选列表失败: %w", err)
	}
	if len(watchlist) < 2 {
		return []GroupSuggestion{}, nil
	}
	membership := downloader.LoadConceptMembership(a.storage.DataDir())
	if membership == nil {
		// 反查表尚未生成，降级为空建议
		return []GroupSuggestion{}, nil
	}

	// 全市场文档频率（用于 IDF）
	docFreq := make(map[string]int, membership.ConceptCount)
	for _, concepts := range membership.Concepts {
		for _, c := range concepts {
			docFreq[c]++
		}
	}

	// 自选股内每个概念命中的股票（保持自选顺序）
	conceptCodes := make(map[string][]string)
	for _, item := range watchlist {
		for _, c := range membership.Concepts[item.Code] {
			conceptCodes[c] = append(conceptCodes[c], item.Code)
		}
	}

	// 已有概念组名集合（同名概念不再建议）
	groups, err := a.storage.LoadWatchlistGroups()
	if err != nil {
		groups = nil // 读取失败不阻塞建议
	}
	existing := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		if g.Source == "concept" && g.ConceptName != "" {
			existing[g.ConceptName] = struct{}{}
		}
	}

	suggestions := make([]GroupSuggestion, 0)
	for concept, codes := range conceptCodes {
		if len(codes) < 2 {
			continue
		}
		if _, ok := existing[concept]; ok {
			continue
		}
		suggestions = append(suggestions, GroupSuggestion{
			ConceptName: concept,
			Codes:       codes,
			Score:       conceptIDFScore(docFreq, len(membership.Concepts), concept),
		})
	}
	// IDF 降序：越小众的概念越值得单独成组
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Score != suggestions[j].Score {
			return suggestions[i].Score > suggestions[j].Score
		}
		return suggestions[i].ConceptName < suggestions[j].ConceptName
	})
	if len(suggestions) > 10 {
		suggestions = suggestions[:10]
	}
	return suggestions, nil
}

// fetchThsHotOnce 拉取一次同花顺热搜榜单（整个榜单一次请求），
// 返回 个股 ts_code -> 热搜项 的映射。失败/未配置时返回 nil，调用方降级为未上榜。
func (a *App) fetchThsHotOnce() map[string]downloader.SFLThsHotItem {
	if a.dataRouter == nil {
		return nil
	}
	client := a.dataRouter.GetSFLClient()
	if client == nil || a.ctx == nil {
		return nil
	}
	items, err := client.FetchThsHot(a.ctx, "")
	if err != nil {
		fmt.Printf("[GroupComparison] ths_hot 拉取失败（降级为未上榜）: %v\n", err)
		return nil
	}
	result := make(map[string]downloader.SFLThsHotItem, len(items))
	for _, item := range items {
		if item.DataType != "个股" || item.TsCode == "" {
			continue
		}
		// 同一代码保留排名最靠前的一条
		if old, ok := result[item.TsCode]; ok && old.Rank <= item.Rank {
			continue
		}
		result[item.TsCode] = item
	}
	return result
}

// GetGroupComparison 批量装配一组股票的对比数据
// 数据全部优先读本地缓存；ths_hot 整个榜单拉一次后按 code 匹配，失败降级为 ThsHotRank=0；
// 当日涨跌幅用托盘报价内存缓存 / 本地行情缓存，不为每股发起新的同步网络请求。
func (a *App) GetGroupComparison(codes []string) ([]GroupComparisonRow, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	watchlist, err := a.storage.LoadWatchlist()
	if err != nil {
		return nil, fmt.Errorf("加载自选列表失败: %w", err)
	}
	nameMap := make(map[string]string, len(watchlist))
	for _, item := range watchlist {
		nameMap[item.Code] = item.Name
	}

	// 托盘报价内存缓存（后台定时刷新），取一次快照避免长时间持锁
	a.trayMu.Lock()
	trayQuotes := make(map[string]float64, len(a.trayQuotes))
	for _, q := range a.trayQuotes {
		trayQuotes[q.Code] = q.ChangePercent
	}
	a.trayMu.Unlock()

	// 同花顺热搜：整个榜单只拉一次
	thsHot := a.fetchThsHotOnce()

	rows := make([]GroupComparisonRow, len(codes))
	var wg sync.WaitGroup
	for i, code := range codes {
		wg.Add(1)
		go func(idx int, c string) {
			defer wg.Done()
			// 单股装配失败不应拖垮整个接口
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[GroupComparison] %s 装配 panic（已跳过）: %v\n", c, r)
					rows[idx] = GroupComparisonRow{Code: c, Name: nameMap[c]}
				}
			}()
			rows[idx] = a.buildGroupComparisonRow(c, nameMap[c], trayQuotes, thsHot)
		}(i, code)
	}
	wg.Wait()
	fillCompositeScores(rows)
	return rows, nil
}

// fillCompositeScores 基于模块 4.2 固定档位算法为每只股票计算综合得分与组内排名。
// 缺失的活跃度/现金含量/负债率/半年涨幅使用组内有效样本中位数替代，保证排名连续性。
func fillCompositeScores(rows []GroupComparisonRow) {
	if len(rows) == 0 {
		return
	}

	// 收集有效样本用于中位数替代
	var cashVals, actVals, debtVals, halfYearVals []float64
	for i := range rows {
		r := &rows[i]
		if r.HasCashRatio {
			cashVals = append(cashVals, r.CashRatio)
		}
		if r.HasActivity {
			actVals = append(actVals, r.ActivityScore)
		}
		if r.HasDebtRatio {
			debtVals = append(debtVals, r.DebtRatio)
		}
		if r.HasHalfYearChange {
			halfYearVals = append(halfYearVals, r.HalfYearChangePercent)
		}
	}
	medianCash := medianFloat64(cashVals)
	medianAct := medianFloat64(actVals)
	medianDebt := medianFloat64(debtVals)
	medianHalfYear := medianFloat64(halfYearVals)

	// 用中位数替代缺失值，并构造 ComparableMetrics 计算得分
	type scored struct {
		idx   int
		score float64
	}
	list := make([]scored, len(rows))
	for i := range rows {
		r := &rows[i]
		if !r.HasCashRatio {
			r.CashRatio = medianCash
		}
		if !r.HasActivity {
			r.ActivityScore = medianAct
		}
		if !r.HasDebtRatio {
			r.DebtRatio = medianDebt
		}
		if !r.HasHalfYearChange {
			r.HalfYearChangePercent = medianHalfYear
		}
		m := &analyzer.ComparableMetrics{
			Symbol:        r.Code,
			Name:          r.Name,
			ROE:           r.ROE,
			GrossMargin:   r.GrossMargin,
			RevenueGrowth: r.RevenueGrowth,
			DebtRatio:     r.DebtRatio,
			CashRatio:     r.CashRatio,
			AScore:        r.AScore,
			ActivityScore: r.ActivityScore,
		}
		list[i] = scored{idx: i, score: analyzer.CalcGroupComparisonScore(m, medianAct, r.HalfYearChangePercent)}
	}

	// 按得分降序排序（稳定排序，同分保持原顺序）
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})
	for rank, s := range list {
		rows[s.idx].CompositeScore = s.score
		rows[s.idx].CompositeRank = rank + 1
	}
}

// medianFloat64 计算浮点切片中位数，空切片返回 0。
func medianFloat64(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

// parseKlineTime 解析 K 线时间，兼容 "20060102" 与 "2006-01-02" 两种格式。
func parseKlineTime(s string) (time.Time, bool) {
	if t, err := time.Parse("20060102", s); err == nil {
		return t, true
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

// calcHalfYearChange 从本地日K缓存计算近半年涨幅。
// 取最新收盘价与 6 个月前（含当日）最近一根有效 K 线收盘价对比，失败返回 (0, false)。
func calcHalfYearChange(klines []downloader.KlineData) (float64, bool) {
	if len(klines) < 2 {
		return 0, false
	}
	latest := klines[len(klines)-1]
	if latest.Close <= 0 {
		return 0, false
	}
	latestTime, ok := parseKlineTime(latest.Time)
	if !ok {
		return 0, false
	}
	targetTime := latestTime.AddDate(0, -6, 0)
	var best downloader.KlineData
	bestDiff := time.Duration(1<<63 - 1)
	found := false
	for _, k := range klines {
		if k.Close <= 0 {
			continue
		}
		kt, ok := parseKlineTime(k.Time)
		if !ok {
			continue
		}
		// 只取 6 个月前及更早的 K 线，避免用不足半年的数据
		if kt.After(targetTime) {
			continue
		}
		diff := targetTime.Sub(kt)
		if diff < bestDiff {
			bestDiff = diff
			best = k
			found = true
		}
	}
	if !found || best.Close <= 0 {
		return 0, false
	}
	return (latest.Close - best.Close) / best.Close * 100, true
}

// buildGroupComparisonRow 装配单只股票的对比数据（全部读本地缓存，失败字段降级为 0）
func (a *App) buildGroupComparisonRow(code, name string, trayQuotes map[string]float64, thsHot map[string]downloader.SFLThsHotItem) GroupComparisonRow {
	row := GroupComparisonRow{Code: code, Name: name}

	// 1. 基本面：本地分析快照（18步评分 / A-Score / 现金含量）
	if snapshot, err := a.LoadAnalysisSnapshot(code); err == nil && snapshot != nil {
		row.Analyzed = true
		row.Grade = snapshot.OverallGrade
		if len(snapshot.Years) > 0 {
			row.YearScore = snapshot.Score[snapshot.Years[0]]
			latest := snapshot.Years[0]
			for _, step := range snapshot.StepResults {
				if step.StepNum == 8 {
					if yd, ok := step.YearlyData[latest]; ok && yd != nil {
						if v, ok2 := yd["AScore"].(float64); ok2 {
							row.AScore = v
						}
					}
				}
				if step.StepNum == 15 {
					if yd, ok := step.YearlyData[latest]; ok && yd != nil {
						if v, ok2 := yd["cashRatio"].(float64); ok2 {
							row.CashRatio = v
							row.HasCashRatio = true
						}
					}
				}
			}
		}
	}

	// 2. 同比与财务：全市场缓存
	if a.marketCache != nil {
		if item, ok := a.marketCache.Get(code); ok {
			row.InMarketCache = true
			row.RevenueGrowth = item.RevenueYoY
			row.NetProfitGrowth = item.NetProfitYoY
			row.ROE = item.ROE
			row.GrossMargin = item.GrossprofitMargin
			row.DebtRatio = item.DebtToAssets
			row.HasDebtRatio = item.DebtToAssets > 0
			if row.Name == "" {
				row.Name = item.Name
			}
		}
	}

	// 3. 活跃度：本地活跃度缓存（24h TTL，过期/缺失降级为 0，不在此发起网络请求）
	if activity, err := a.storage.LoadActivityCache(code); err == nil && activity != nil {
		row.ActivityScore = activity.Score
		row.HasActivity = true
		row.Stars = activity.Stars
	}

	// 4. 当日涨跌幅：托盘内存缓存优先，其次本地行情缓存
	if cp, ok := trayQuotes[code]; ok {
		row.ChangePercent = cp
	} else if quote, err := a.storage.LoadStockQuote(code); err == nil && quote != nil {
		row.ChangePercent = quote.ChangePercent
	}

	// 5. 半年涨幅：本地日K缓存，不发起网络请求
	if klines, err := a.storage.LoadStockKlines(code, "daily"); err == nil && len(klines) >= 2 {
		if v, ok := calcHalfYearChange(klines); ok {
			row.HalfYearChangePercent = v
			row.HasHalfYearChange = true
		}
	}

	// 6. 同花顺热搜（榜单已在 GetGroupComparison 拉取一次）
	if item, ok := thsHot[code]; ok {
		row.ThsHotRank = item.Rank
		row.ThsHotValue = item.Hot
	}

	return row
}

// GetGroupHeat 读取板块热度榜缓存（FetchHotConceptBoard 写入的同一份缓存，纯本地读），
// 对每个 source=concept 的组按 ConceptName 精确匹配板块名。
// 匹配不到的 OnBoard=false；manual 组不出现在结果里（前端用组内活跃度兜底）。
func (a *App) GetGroupHeat() ([]GroupHeat, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	groups, err := a.storage.LoadWatchlistGroups()
	if err != nil {
		return nil, fmt.Errorf("加载自选分组失败: %w", err)
	}

	// 板块名 -> 热度项
	boardByName := make(map[string]downloader.HotConcept)
	if board, err := a.storage.LoadHotConceptBoard(""); err == nil && board != nil {
		for _, c := range board.Concepts {
			boardByName[c.Name] = c
		}
	} else if err != nil {
		fmt.Printf("[GroupHeat] 读取板块热度缓存失败（降级为全部未上榜）: %v\n", err)
	}

	result := make([]GroupHeat, 0, len(groups))
	for _, g := range groups {
		if g.Source != "concept" || g.ConceptName == "" {
			continue
		}
		heat := GroupHeat{
			GroupID:     g.ID,
			ConceptName: g.ConceptName,
		}
		if c, ok := boardByName[g.ConceptName]; ok {
			heat.OnBoard = true
			heat.Score = c.Score
			heat.ChangePercent = c.ChangePct
			heat.MainInflow = c.MainInflow
		}
		result = append(result, heat)
	}
	return result, nil
}

// FetchMissingCompositeDataResult 补齐组内综合得分缺失数据的结果
type FetchMissingCompositeDataResult struct {
	AnalyzedCodes   []string `json:"analyzedCodes"`
	FailedCodes     []string `json:"failedCodes"`
	RefreshedCache  bool     `json:"refreshedCache"`
	ActivityMessage string   `json:"activityMessage"`
	Message         string   `json:"message"`
}

// FetchMissingCompositeData 为组内对比补齐缺失的财务/活跃度/半年涨幅数据（只补缺失的）。
// - 现金含量缺失：触发财报分析（AnalyzeStock）
// - 市场缓存缺失/负债率缺失：精准刷新单只股票市场缓存（MarketCacheManager.UpdateItem）
// - 活跃度缺失：调用 FetchMissingActivity
// - 半年涨幅缺失：刷新本地日K缓存（GetStockKlines）
func (a *App) FetchMissingCompositeData(codes []string) (*FetchMissingCompositeDataResult, error) {
	if a.storage == nil {
		return nil, fmt.Errorf("存储未初始化")
	}
	if len(codes) == 0 {
		return &FetchMissingCompositeDataResult{Message: "无股票需要补齐"}, nil
	}

	rows, err := a.GetGroupComparison(codes)
	if err != nil {
		return nil, fmt.Errorf("加载组内对比失败: %w", err)
	}

	var needAnalyze []string
	var needActivity []string
	var needHalfYear []string
	needRefreshCodes := make(map[string]struct{})
	for _, r := range rows {
		if !r.HasCashRatio {
			needAnalyze = append(needAnalyze, r.Code)
		}
		// 市场缓存缺失时，营收/净利/ROE/毛利率/负债率全部不可用，需要刷新该股票的市场缓存
		if !r.InMarketCache || !r.HasDebtRatio {
			needRefreshCodes[r.Code] = struct{}{}
		}
		if !r.HasActivity {
			needActivity = append(needActivity, r.Code)
		}
		if !r.HasHalfYearChange {
			needHalfYear = append(needHalfYear, r.Code)
		}
	}

	result := &FetchMissingCompositeDataResult{}
	var marketCacheMsg string // 市场缓存刷新结果，单独记录避免覆盖活跃度消息

	// 1. 精准刷新缺失市场缓存的股票（负债率/营收同比/净利同比来源）
	// 必须同步等待更新完成，否则前端立即重新加载时缓存仍是旧数据，
	// 会出现“点了补齐缺失数据但指标还是缺失”的现象。
	if len(needRefreshCodes) > 0 {
		if a.marketCache == nil {
			marketCacheMsg = "缓存管理器未初始化，市场缓存无法补齐"
			fmt.Printf("[FetchMissingCompositeData] 缓存管理器未初始化\n")
		} else {
			var sflClient *downloader.SFLClient
			if a.dataRouter != nil {
				sflClient = a.dataRouter.GetSFLClient()
			}
			if sflClient == nil {
				marketCacheMsg = "SFL 客户端不可用，市场缓存无法补齐"
				fmt.Printf("[FetchMissingCompositeData] SFL 客户端不可用\n")
			} else {
				var refreshWg sync.WaitGroup
				var refreshMu sync.Mutex
				refreshOK := 0
				for code := range needRefreshCodes {
					refreshWg.Add(1)
					go func(c string) {
						defer refreshWg.Done()
						defer func() {
							if r := recover(); r != nil {
								fmt.Printf("[FetchMissingCompositeData] %s 刷新市场缓存 panic: %v\n", c, r)
								refreshMu.Lock()
								result.FailedCodes = append(result.FailedCodes, c)
								refreshMu.Unlock()
							}
						}()
						if err := a.marketCache.UpdateItem(a.ctx, sflClient, c); err != nil {
							fmt.Printf("[FetchMissingCompositeData] %s 刷新市场缓存失败: %v\n", c, err)
							refreshMu.Lock()
							result.FailedCodes = append(result.FailedCodes, c)
							refreshMu.Unlock()
						} else {
							refreshMu.Lock()
							refreshOK++
							refreshMu.Unlock()
						}
					}(code)
				}
				refreshWg.Wait()
				if refreshOK > 0 {
					result.RefreshedCache = true
					// 所有 UpdateItem 只更新内存，这里统一落盘一次，避免并发 Save 遍历 map 触发 panic
					if err := a.marketCache.Save(); err != nil {
						fmt.Printf("[FetchMissingCompositeData] 保存市场缓存失败: %v\n", err)
						marketCacheMsg = fmt.Sprintf("保存市场缓存失败: %v", err)
					}
				}
				if len(result.FailedCodes) > 0 && refreshOK == 0 {
					marketCacheMsg = fmt.Sprintf("市场缓存刷新失败 %d 只", len(result.FailedCodes))
				}
			}
		}
	}

	// 2. 并发分析缺失现金含量的股票
	// 现金含量来自分析快照 step15，而 AnalyzeStock 需要本地财报 JSON；
	// 若财务数据不存在，先调用 DownloadReports 自动下载，再分析。
	if len(needAnalyze) > 0 {
		var wg sync.WaitGroup
		var mu sync.Mutex
		for _, code := range needAnalyze {
			wg.Add(1)
			go func(c string) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("[FetchMissingCompositeData] %s 分析 panic: %v\n", c, r)
						mu.Lock()
						result.FailedCodes = append(result.FailedCodes, c)
						mu.Unlock()
					}
				}()
				// 先尝试下载/补全财务数据
				if _, err := a.DownloadReports(c, 0); err != nil {
					fmt.Printf("[FetchMissingCompositeData] %s 下载财报失败: %v\n", c, err)
					// 下载失败仍尝试分析，可能本地已有旧数据
				}
				_, err := a.AnalyzeStock(c, false)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					fmt.Printf("[FetchMissingCompositeData] %s 分析失败: %v\n", c, err)
					result.FailedCodes = append(result.FailedCodes, c)
				} else {
					result.AnalyzedCodes = append(result.AnalyzedCodes, c)
				}
			}(code)
		}
		wg.Wait()
	}

	// 3. 补齐缺失活跃度
	if len(needActivity) > 0 {
		actRes, err := a.FetchMissingActivity(needActivity)
		if err != nil {
			result.ActivityMessage = fmt.Sprintf("活跃度获取失败: %v", err)
		} else if actRes != nil {
			result.ActivityMessage = fmt.Sprintf("成功 %d 只，失败 %d 只", actRes.SuccessCount, len(actRes.FailedCodes))
		}
	}

	// 4. 补齐缺失半年涨幅：刷新本地日K缓存（GetStockKlines 会自动处理缓存缺失/过期）
	halfYearMsg := ""
	if len(needHalfYear) > 0 {
		var halfYearWg sync.WaitGroup
		var halfYearMu sync.Mutex
		halfYearOK := 0
		halfYearFailed := make(map[string]struct{})
		for _, code := range needHalfYear {
			halfYearWg.Add(1)
			go func(c string) {
				defer halfYearWg.Done()
				defer func() {
					if r := recover(); r != nil {
						fmt.Printf("[FetchMissingCompositeData] %s 刷新K线 panic: %v\n", c, r)
						halfYearMu.Lock()
						halfYearFailed[c] = struct{}{}
						halfYearMu.Unlock()
					}
				}()
				if _, err := a.GetStockKlines(c, "daily"); err != nil {
					fmt.Printf("[FetchMissingCompositeData] %s 刷新K线失败: %v\n", c, err)
					halfYearMu.Lock()
					halfYearFailed[c] = struct{}{}
					halfYearMu.Unlock()
				} else {
					halfYearMu.Lock()
					halfYearOK++
					halfYearMu.Unlock()
				}
			}(code)
		}
		halfYearWg.Wait()
		if halfYearOK > 0 {
			halfYearMsg = fmt.Sprintf("刷新K线 %d 只", halfYearOK)
		}
		if len(halfYearFailed) > 0 {
			if halfYearMsg != "" {
				halfYearMsg += fmt.Sprintf("，失败 %d 只", len(halfYearFailed))
			} else {
				halfYearMsg = fmt.Sprintf("K线刷新失败 %d 只", len(halfYearFailed))
			}
			for c := range halfYearFailed {
				result.FailedCodes = append(result.FailedCodes, c)
			}
		}
	}

	parts := []string{}
	if len(result.AnalyzedCodes) > 0 {
		parts = append(parts, fmt.Sprintf("分析 %d 只", len(result.AnalyzedCodes)))
	}
	if result.RefreshedCache {
		parts = append(parts, "刷新市场缓存")
	}
	if marketCacheMsg != "" {
		parts = append(parts, marketCacheMsg)
	}
	if len(needActivity) > 0 && result.ActivityMessage != "" {
		parts = append(parts, result.ActivityMessage)
	}
	if halfYearMsg != "" {
		parts = append(parts, halfYearMsg)
	}
	if len(result.FailedCodes) > 0 {
		parts = append(parts, fmt.Sprintf("失败 %d 只", len(result.FailedCodes)))
	}
	if len(parts) == 0 {
		result.Message = "无缺失数据，无需补齐"
	} else {
		result.Message = strings.Join(parts, "；")
	}
	return result, nil
}
