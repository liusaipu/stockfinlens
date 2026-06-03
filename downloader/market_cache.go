package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MarketCacheItem 单只股票的市场缓存数据
// 字段设计预留了未来扩展空间，新增字段使用 omitempty 保持向后兼容
type MarketCacheItem struct {
	// === 基础资料 (来自 stock_basic / profile) ===
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	Market      string  `json:"market"`                 // SH / SZ / BJ / HK
	Industry    string  `json:"industry"`               // 申万/东财一级行业
	SubIndustry string  `json:"sub_industry,omitempty"` // 二级行业（预留扩展）
	Area        string  `json:"area,omitempty"`         // 地区（预留扩展）
	MarketCap   float64 `json:"market_cap"`             // 总市值（元）
	ListDate    string  `json:"list_date,omitempty"`    // 上市日期 YYYYMMDD

	// === 财务指标 (来自 fina_indicator，最新一期) ===
	ROE               float64 `json:"roe"`
	ROEDiluted        float64 `json:"roe_diluted,omitempty"`
	ROEAvg            float64 `json:"roe_avg"`
	GrossprofitMargin float64 `json:"grossprofit_margin"`
	NetprofitMargin   float64 `json:"netprofit_margin,omitempty"` // 预留扩展
	DebtToAssets      float64 `json:"debt_to_assets,omitempty"`
	CurrentRatio      float64 `json:"current_ratio,omitempty"`
	QuickRatio        float64 `json:"quick_ratio,omitempty"`
	RevenueYoY        float64 `json:"revenue_yoy,omitempty"`    // 营收同比增速（预留）
	NetProfitYoY      float64 `json:"net_profit_yoy,omitempty"` // 净利润同比增速（预留）
	EPS               float64 `json:"eps,omitempty"`            // 每股收益（预留）
	BVPS              float64 `json:"bvps,omitempty"`           // 每股净资产（预留）

	// === 市场估值 (来自 daily_basic，预留扩展) ===
	PE float64 `json:"pe,omitempty"`
	PB float64 `json:"pb,omitempty"`
	PS float64 `json:"ps,omitempty"`

	// === 概念/风口 (来自 concept_detail) ===
	Concepts []string `json:"concepts,omitempty"`

	// === 元数据 ===
	UpdatedAt  string `json:"updated_at"`  // ISO8601
	DataSource string `json:"data_source"` // sfl / eastmoney / manual
}

// MarketCache 全市场缓存根结构
type MarketCache struct {
	Version   string                     `json:"version"`    // 缓存格式版本，如 "1.0"
	CreatedAt string                     `json:"created_at"` // 首次创建时间
	UpdatedAt string                     `json:"updated_at"` // 最后更新时间
	Count     int                        `json:"count"`      // 条目数
	Items     map[string]MarketCacheItem `json:"items"`      // key: symbol (如 600460.SH)
}

const (
	marketCacheVersion = "1.0"
	marketCacheFile    = "market_cache.json"
	cacheMaxAge        = 7 * 24 * time.Hour // 7天有效期
)

// MarketCacheManager 全市场缓存管理器
// 线程安全：读用 RLock，写用 Lock
type MarketCacheManager struct {
	dataDir string
	mu      sync.RWMutex
	cache   *MarketCache
}

// NewMarketCacheManager 创建缓存管理器
func NewMarketCacheManager(dataDir string) *MarketCacheManager {
	return &MarketCacheManager{
		dataDir: dataDir,
		cache: &MarketCache{
			Version: marketCacheVersion,
			Items:   make(map[string]MarketCacheItem),
		},
	}
}

// cachePath 返回缓存文件路径
func (m *MarketCacheManager) cachePath() string {
	return filepath.Join(m.dataDir, marketCacheFile)
}

// Load 从磁盘加载缓存，失败时返回空缓存（不影响使用）
func (m *MarketCacheManager) Load() error {
	path := m.cachePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 首次使用，允许不存在
		}
		return fmt.Errorf("读取缓存失败: %w", err)
	}

	var loaded MarketCache
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("解析缓存失败: %w", err)
	}

	if loaded.Items == nil {
		loaded.Items = make(map[string]MarketCacheItem)
	}
	m.mu.Lock()
	m.cache = &loaded
	m.mu.Unlock()
	return nil
}

// Save 将缓存写入磁盘（原子写入，防止写坏）
func (m *MarketCacheManager) Save() error {
	m.mu.RLock()
	cache := *m.cache
	m.mu.RUnlock()

	cache.Count = len(cache.Items)
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化缓存失败: %w", err)
	}

	path := m.cachePath()
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("写入临时缓存失败: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("替换缓存文件失败: %w", err)
	}
	return nil
}

// Get 线程安全地获取单只股票缓存
func (m *MarketCacheManager) Get(symbol string) (MarketCacheItem, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	item, ok := m.cache.Items[symbol]
	return item, ok
}

// GetAll 线程安全地返回所有缓存项的副本（用于推荐算法批量读取）
func (m *MarketCacheManager) GetAll() map[string]MarketCacheItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]MarketCacheItem, len(m.cache.Items))
	for k, v := range m.cache.Items {
		result[k] = v
	}
	return result
}

// Len 返回缓存条目数
func (m *MarketCacheManager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.cache.Items)
}

// IsExpired 检查缓存是否超过有效期
func (m *MarketCacheManager) IsExpired() bool {
	m.mu.RLock()
	updatedAt := m.cache.UpdatedAt
	m.mu.RUnlock()

	if updatedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > cacheMaxAge
}

// IsEmpty 检查缓存是否为空
func (m *MarketCacheManager) IsEmpty() bool {
	return m.Len() == 0
}

// Upsert 批量写入或更新缓存项（内部加锁）
func (m *MarketCacheManager) Upsert(items map[string]MarketCacheItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range items {
		m.cache.Items[k] = v
	}
	m.cache.UpdatedAt = time.Now().Format(time.RFC3339)
	m.cache.Count = len(m.cache.Items)
}

// SetItem 写入单只股票缓存
func (m *MarketCacheManager) SetItem(symbol string, item MarketCacheItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache.Items[symbol] = item
	m.cache.UpdatedAt = time.Now().Format(time.RFC3339)
	m.cache.Count = len(m.cache.Items)
}

// Clear 清空缓存
func (m *MarketCacheManager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache.Items = make(map[string]MarketCacheItem)
	m.cache.Count = 0
	m.cache.UpdatedAt = time.Now().Format(time.RFC3339)
}

// UpdateProgress 更新进度回调
// stage: "stock_basic" | "fina_indicator" | "concepts" | "done"
// current: 当前进度, total: 总数
type UpdateProgress struct {
	Stage   string `json:"stage"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

// UpdateAll 后台全量更新缓存（stock_basic → fina_indicator → concepts）
// 分三阶段执行，每阶段完成后自动保存，防止中断丢失
func (m *MarketCacheManager) UpdateAll(ctx context.Context, sflClient *SFLClient, progressFn func(UpdateProgress)) error {
	if sflClient == nil {
		return fmt.Errorf("SFL 客户端未初始化")
	}

	now := time.Now().Format(time.RFC3339)
	m.mu.Lock()
	m.cache.Version = marketCacheVersion
	if m.cache.CreatedAt == "" {
		m.cache.CreatedAt = now
	}
	m.mu.Unlock()

	// ========== 阶段 1: 全市场基础资料 ==========
	if progressFn != nil {
		progressFn(UpdateProgress{Stage: "stock_basic", Current: 0, Total: 0, Message: "正在获取全市场基础资料..."})
	}
	basics, err := sflClient.FetchAllStockBasic(ctx)
	if err != nil {
		return fmt.Errorf("获取 stock_basic 失败: %w", err)
	}
	if progressFn != nil {
		progressFn(UpdateProgress{Stage: "stock_basic", Current: len(basics), Total: len(basics), Message: fmt.Sprintf("已获取 %d 只股票基础资料", len(basics))})
	}

	// 初始化缓存条目
	items := make(map[string]MarketCacheItem, len(basics))
	for _, b := range basics {
		items[b.TsCode] = MarketCacheItem{
			Code:       b.Symbol,
			Name:       b.Name,
			Market:     b.Market,
			Industry:   b.Industry,
			Area:       b.Area,
			ListDate:   b.ListDate,
			UpdatedAt:  now,
			DataSource: "sfl",
		}
	}

	// ========== 阶段 2: 全市场财务指标 ==========
	if progressFn != nil {
		progressFn(UpdateProgress{Stage: "fina_indicator", Current: 0, Total: len(items), Message: "正在获取全市场财务指标..."})
	}

	// 策略：先尝试一次性获取全市场最新财务指标
	// 如果失败，则回退到逐只获取（并发限制）
	allFinas, err := sflClient.FetchAllLatestFinaIndicator(ctx)
	if err == nil && len(allFinas) > 0 {
		for _, f := range allFinas {
			if item, ok := items[f.TsCode]; ok {
				item.ROE = f.ROE
				item.ROEDiluted = f.ROEDiluted
				item.ROEAvg = f.ROEAvg
				item.GrossprofitMargin = f.GrossprofitMargin
				item.NetprofitMargin = f.NetprofitMargin
				item.DebtToAssets = f.DebtToAssets
				item.CurrentRatio = f.CurrentRatio
				item.QuickRatio = f.QuickRatio
				item.UpdatedAt = now
				items[f.TsCode] = item
			}
		}
		if progressFn != nil {
			progressFn(UpdateProgress{Stage: "fina_indicator", Current: len(allFinas), Total: len(items), Message: fmt.Sprintf("已批量获取 %d 只财务指标", len(allFinas))})
		}
	} else {
		// 回退：逐只获取（并发限制 10）
		var wg sync.WaitGroup
		sem := make(chan struct{}, 10)
		mu := sync.Mutex{}
		current := 0

		symbols := make([]string, 0, len(items))
		for sym := range items {
			symbols = append(symbols, sym)
		}

		for _, sym := range symbols {
			wg.Add(1)
			sem <- struct{}{}
			go func(s string) {
				defer wg.Done()
				defer func() { <-sem }()
				parts := strings.Split(s, ".")
				if len(parts) != 2 {
					return
				}
				market := strings.ToUpper(parts[1])
				code := parts[0]
				fina, _ := sflClient.FetchFinaIndicator(ctx, market, code, "", "")
				if len(fina) > 0 {
					f := fina[0]
					mu.Lock()
					if item, ok := items[s]; ok {
						item.ROE = f.ROE
						item.ROEDiluted = f.ROEDiluted
						item.ROEAvg = f.ROEAvg
						item.GrossprofitMargin = f.GrossprofitMargin
						item.NetprofitMargin = f.NetprofitMargin
						item.DebtToAssets = f.DebtToAssets
						item.CurrentRatio = f.CurrentRatio
						item.QuickRatio = f.QuickRatio
						item.UpdatedAt = now
						items[s] = item
					}
					current++
					if progressFn != nil && current%50 == 0 {
						progressFn(UpdateProgress{Stage: "fina_indicator", Current: current, Total: len(symbols), Message: fmt.Sprintf("已获取 %d/%d 只财务指标", current, len(symbols))})
					}
					mu.Unlock()
				}
			}(sym)
		}
		wg.Wait()
		if progressFn != nil {
			progressFn(UpdateProgress{Stage: "fina_indicator", Current: current, Total: len(symbols), Message: fmt.Sprintf("已获取 %d/%d 只财务指标", current, len(symbols))})
		}
	}

	// ========== 阶段 3: 全市场概念映射 ==========
	if progressFn != nil {
		progressFn(UpdateProgress{Stage: "concepts", Current: 0, Total: 0, Message: "正在获取概念映射..."})
	}
	// 策略：获取所有概念列表，再获取每个概念的成分股，反向构建 股票→概念 映射
	conceptMap, err := sflClient.FetchAllConceptMappings(ctx)
	if err == nil && len(conceptMap) > 0 {
		for sym, concepts := range conceptMap {
			if item, ok := items[sym]; ok {
				item.Concepts = concepts
				item.UpdatedAt = now
				items[sym] = item
			}
		}
		if progressFn != nil {
			progressFn(UpdateProgress{Stage: "concepts", Current: len(conceptMap), Total: len(conceptMap), Message: fmt.Sprintf("已构建 %d 只股票概念映射", len(conceptMap))})
		}
	} else {
		if progressFn != nil {
			progressFn(UpdateProgress{Stage: "concepts", Current: 0, Total: 0, Message: "概念映射获取失败，跳过"})
		}
	}

	// ========== 阶段 4: 二级行业回填（来自东财行业板块反查表）==========
	// 反查表由 startBackgroundConceptMembershipUpdate 在后台异步构建。
	// 若文件已存在（非首次启动），合并 SubIndustry 字段进入缓存；首次启动时反查表可能尚未生成，
	// 此处会静默跳过 —— recommend 侧亦会通过 lookupSubIndustry 直接读反查表兜底。
	if membership := LoadConceptMembership(m.dataDir); membership != nil && len(membership.SubIndustry) > 0 {
		filled := 0
		for sym, subInd := range membership.SubIndustry {
			if item, ok := items[sym]; ok {
				if item.SubIndustry == "" || item.SubIndustry != subInd {
					item.SubIndustry = subInd
					item.UpdatedAt = now
					items[sym] = item
					filled++
				}
			}
		}
		if progressFn != nil {
			progressFn(UpdateProgress{Stage: "sub_industry", Current: filled, Total: len(items), Message: fmt.Sprintf("回填 %d 只股票二级行业", filled)})
		}
	}

	// 写入缓存并保存
	m.Upsert(items)
	if err := m.Save(); err != nil {
		return fmt.Errorf("保存缓存失败: %w", err)
	}

	if progressFn != nil {
		progressFn(UpdateProgress{Stage: "done", Current: len(items), Total: len(items), Message: fmt.Sprintf("全市场缓存更新完成，共 %d 只股票", len(items))})
	}
	return nil
}
