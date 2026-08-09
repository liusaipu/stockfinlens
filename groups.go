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
	// 活跃度（复用 GetWatchlistActivity 的数据源：本地活跃度缓存）
	ActivityScore float64 `json:"activityScore"`
	Stars         int     `json:"stars"`
	// 热度
	ChangePercent float64 `json:"changePercent"` // 当日涨跌幅（托盘报价内存缓存 / 本地行情缓存）
	ThsHotRank    int     `json:"thsHotRank"`    // 同花顺热搜排名，0=未上榜/不可用
	ThsHotValue   float64 `json:"thsHotValue"`
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
	return rows, nil
}

// buildGroupComparisonRow 装配单只股票的对比数据（全部读本地缓存，失败字段降级为 0）
func (a *App) buildGroupComparisonRow(code, name string, trayQuotes map[string]float64, thsHot map[string]downloader.SFLThsHotItem) GroupComparisonRow {
	row := GroupComparisonRow{Code: code, Name: name}

	// 1. 基本面：本地分析快照（18步评分 / A-Score）
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
					break
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
			if row.Name == "" {
				row.Name = item.Name
			}
		}
	}

	// 3. 活跃度：本地活跃度缓存（24h TTL，过期/缺失降级为 0，不在此发起网络请求）
	if activity, err := a.storage.LoadActivityCache(code); err == nil && activity != nil {
		row.ActivityScore = activity.Score
		row.Stars = activity.Stars
	}

	// 4. 当日涨跌幅：托盘内存缓存优先，其次本地行情缓存
	if cp, ok := trayQuotes[code]; ok {
		row.ChangePercent = cp
	} else if quote, err := a.storage.LoadStockQuote(code); err == nil && quote != nil {
		row.ChangePercent = quote.ChangePercent
	}

	// 5. 同花顺热搜（榜单已在 GetGroupComparison 拉取一次）
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
