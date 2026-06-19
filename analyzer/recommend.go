package analyzer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// conceptMembership 概念板块/行业板块反查表（analyzer 侧只读视图）
// 数据来自 downloader 后台构建的 _concept_membership.json，避免 analyzer 反向依赖 downloader
type conceptMembership struct {
	Concepts    map[string][]string // symbol(带后缀) → 概念板块名列表
	SubIndustry map[string]string   // symbol → 二级行业（东财行业板块名）
	// DocFreq 每个概念在全市场出现次数，用于 IDF 加权
	// IDF = log(总股票数 / 概念命中数)
	// 命中越少（如 "3D玻璃" 15 家）IDF 越大；命中越多（如 "智能穿戴" 100+ 家，部分被 API 截断）IDF 越小。
	// 解决"主题型泛概念(智能穿戴/苹果概念)与业务型实质概念(3D玻璃/UTG)同权重"导致面板厂排在玻璃盖板厂之前的错配。
	DocFreq   map[string]int
	TotalDocs int // 反查表覆盖的总股票数（IDF 分母）
}

var (
	membershipMu    sync.RWMutex
	membershipData  *conceptMembership
	membershipMtime time.Time
	membershipPath  string // 缓存的路径，用于检测 dataDir 变化
)

// membershipFilePath 反查表文件路径：与 dataRoot(.../data) 同级
func membershipFilePath(dataRoot string) string {
	return filepath.Join(filepath.Dir(dataRoot), "_concept_membership.json")
}

// loadConceptMembership 加载反查表，带 mtime 缓存
// 文件不存在或解析失败时返回 nil（调用方按"反查表暂不可用"处理）
func loadConceptMembership(dataRoot string) *conceptMembership {
	path := membershipFilePath(dataRoot)
	info, err := os.Stat(path)
	if err != nil {
		// 文件不存在：但若旧路径还缓存着上次成功的数据，仍返回（避免文件被误删导致功能突然失效）
		membershipMu.RLock()
		defer membershipMu.RUnlock()
		if membershipPath == path {
			return membershipData
		}
		return nil
	}

	// 先用读锁快速判断是否命中缓存
	membershipMu.RLock()
	if membershipPath == path && membershipData != nil && membershipMtime.Equal(info.ModTime()) {
		d := membershipData
		membershipMu.RUnlock()
		return d
	}
	membershipMu.RUnlock()

	// 需要重新加载
	membershipMu.Lock()
	defer membershipMu.Unlock()
	// 写锁内再次确认（避免重复 IO）
	if membershipPath == path && membershipData != nil && membershipMtime.Equal(info.ModTime()) {
		return membershipData
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return membershipData
	}
	var raw struct {
		Concepts    map[string][]string `json:"concepts"`
		SubIndustry map[string]string   `json:"sub_industry"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return membershipData
	}
	if raw.Concepts == nil {
		raw.Concepts = make(map[string][]string)
	}
	if raw.SubIndustry == nil {
		raw.SubIndustry = make(map[string]string)
	}
	// 计算每个概念的全市场命中次数（DocFreq），供 IDF 加权使用
	docFreq := make(map[string]int, 256)
	for _, concepts := range raw.Concepts {
		for _, c := range concepts {
			docFreq[c]++
		}
	}
	membershipData = &conceptMembership{
		Concepts:    raw.Concepts,
		SubIndustry: raw.SubIndustry,
		DocFreq:     docFreq,
		TotalDocs:   len(raw.Concepts),
	}
	membershipMtime = info.ModTime()
	membershipPath = path
	return membershipData
}

// lookupSubIndustry 查询 symbol 的二级行业；无数据返回空串
func lookupSubIndustry(dataRoot, symbol string) string {
	m := loadConceptMembership(dataRoot)
	if m == nil {
		return ""
	}
	return m.SubIndustry[symbol]
}

// conceptIDF 计算概念的 IDF 权重，越稀有的概念权重越大
// 反查表未加载时返回 1.0（退化为不加权，行为与未引入 IDF 时相同）
// API 截断导致部分热门概念命中数被 clamp 到 100，IDF 区分度仍然有效（3D玻璃 15家 vs 智能穿戴 100家）
func conceptIDF(m *conceptMembership, concept string) float64 {
	if m == nil || m.TotalDocs == 0 {
		return 1.0
	}
	freq := m.DocFreq[concept]
	if freq <= 0 {
		// 反查表里没出现的概念（来自 industryConceptBoost 等本地补充），按中位 IDF 给分
		return 1.0
	}
	// 标准 IDF = log(N / df)
	// 例：N=4296, "3D玻璃" df=15 → IDF=5.657；"智能穿戴" df=100 → IDF=3.760
	// 比值 5.657/3.760 ≈ 1.5，足够拉开稀有/泛主题概念的差距
	return math.Log(float64(m.TotalDocs) / float64(freq))
}

// ComparableRecommendation 可比公司推荐项
type ComparableRecommendation struct {
	Symbol      string   `json:"symbol"`
	Name        string   `json:"name"`
	Score       float64  `json:"score"`       // 0-100 相似度得分
	Reasons     []string `json:"reasons"`     // 推荐理由
	DataQuality string   `json:"dataQuality"` // high/medium/low（本地是否有财报数据）
}

// MarketCacheItem 市场缓存条目（由 app 层从 downloader.MarketCache 转换而来）
// analyzer 包独立定义，避免循环导入 downloader
type MarketCacheItem struct {
	Symbol      string
	Name        string
	Industry    string
	SubIndustry string // 二级行业（东财行业板块名），如 "医疗研发外包"
	MarketCap   float64
	ROE         float64
	GM          float64
	Concepts    []string
}

// RecommendComparables 基于多维度相似度自动推荐可比公司
// targetSymbol: 目标股票代码（带点格式，如 "000001.SZ"）
// targetProfile: 目标股票资料（行业、市值等）
// targetData: 目标股票财务数据（用于提取 ROE、毛利率）
// dataDir: 本地数据根目录（用于扫描候选股票，当 cache 为空时 fallback）
// allSymbols: 全市场代码列表（扩大候选池，当 cache 为空时 fallback）
// cacheItems: 全市场缓存数据（优先使用，避免推荐结果局限于自选股）
// maxResults: 最大返回数量
func RecommendComparables(targetSymbol string, targetProfile *StockProfile, targetData *FinancialData, dataDir string, allSymbols []string, cacheItems map[string]MarketCacheItem, maxResults int) []ComparableRecommendation {
	if maxResults <= 0 {
		maxResults = 5
	}

	// 提取目标股票的关键特征
	targetIndustry := ""
	targetMarketCap := 0.0
	if targetProfile != nil {
		targetIndustry = targetProfile.Industry
		targetMarketCap = targetProfile.MarketCap
	}

	targetROE := 0.0
	targetGM := 0.0
	if targetData != nil && len(targetData.Years) > 0 {
		year := targetData.Years[0]
		equity := targetData.GetValueOrZero(targetData.BalanceSheet, "所有者权益合计", year)
		if equity == 0 {
			totalAssets := targetData.GetValueOrZero(targetData.BalanceSheet, "总资产", year)
			totalLiabilities := targetData.GetValueOrZero(targetData.BalanceSheet, "总负债", year)
			equity = totalAssets - totalLiabilities
		}
		netProfit := targetData.GetValueOrZero(targetData.IncomeStatement, "净利润", year)
		revenue := targetData.GetValueOrZero(targetData.IncomeStatement, "营业收入", year)
		cost := targetData.GetValueOrZero(targetData.IncomeStatement, "营业成本", year)
		if equity > 0 {
			targetROE = netProfit / equity
		}
		if revenue > 0 {
			targetGM = (revenue - cost) / revenue
		}
	}

	// 读取目标股票活跃度
	targetActivity := loadActivityScore(filepath.Join(dataDir, "data"), targetSymbol)

	fmt.Printf("[RecommendComparables] target=%s industry=%s ROE=%.4f GM=%.4f activity=%.1f\n", targetSymbol, targetIndustry, targetROE, targetGM, targetActivity)

	// 读取目标股票的概念标签
	targetConcepts := loadConcepts(filepath.Join(dataDir, "data"), targetSymbol)
	// 读取目标股票二级行业（来自东财行业板块反查表）
	targetSubIndustry := lookupSubIndustry(filepath.Join(dataDir, "data"), targetSymbol)
	// 预加载反查表，传给 computeSimilarity 用于 IDF 加权（避免每个候选都触发 stat）
	membership := loadConceptMembership(filepath.Join(dataDir, "data"))
	fmt.Printf("[RecommendComparables] target concepts=%d subIndustry=%s\n", len(targetConcepts), targetSubIndustry)

	// 扫描候选股票（优先从全市场缓存读取，避免推荐结果局限于自选股）
	candidates := scanCandidates(dataDir, targetSymbol, allSymbols, cacheItems)
	fmt.Printf("[RecommendComparables] candidates=%d\n", len(candidates))

	// 计算每个候选股票的相似度
	var scored []ComparableRecommendation
	startCompute := time.Now()
	for i, c := range candidates {
		score, reasons, dataQuality := computeSimilarity(
			targetIndustry, targetSubIndustry, targetMarketCap, targetROE, targetGM, targetActivity, targetConcepts,
			c, membership,
		)
		// 设置最低得分门槛：目标有行业信息时至少15分，否则至少8分
		minScore := 8.0
		if targetIndustry != "" {
			minScore = 15.0
		}
		if score >= minScore {
			scored = append(scored, ComparableRecommendation{
				Symbol:      c.Symbol,
				Name:        c.Name,
				Score:       score,
				Reasons:     reasons,
				DataQuality: dataQuality,
			})
		}
		if i < 3 || i == len(candidates)-1 {
			fmt.Printf("[RecommendComparables] candidate[%d] %s score=%.2f reasons=%v\n", i, c.Symbol, score, reasons)
		}
	}
	fmt.Printf("[RecommendComparables] compute done: %d/%d passed门槛 in %.3fs\n", len(scored), len(candidates), time.Since(startCompute).Seconds())

	// 按得分降序排序（全市场候选可能达 5000+，必须用 O(n log n) 排序）
	// 得分相同时按股票代码字典序稳定排序，消除 map 遍历随机性导致的推荐结果抖动
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Symbol < scored[j].Symbol
	})
	fmt.Printf("[RecommendComparables] sort done, top5: ")
	for i := 0; i < 5 && i < len(scored); i++ {
		fmt.Printf("%s(%.0f) ", scored[i].Symbol, scored[i].Score)
	}
	fmt.Println()

	if len(scored) > maxResults {
		scored = scored[:maxResults]
	}
	return scored
}

// candidateInfo 候选股票内部信息
type candidateInfo struct {
	Symbol        string
	Name          string
	Industry      string
	SubIndustry   string // 二级行业（东财行业板块名）
	MarketCap     float64
	ROE           float64
	GM            float64
	ActivityScore float64  // 活跃度分数 (0-100)
	HasData       bool     // 是否有本地财报数据
	Concepts      []string // 概念/风口标签
}

// StockProfile 推荐算法使用的股票资料子集（避免循环导入）
type StockProfile struct {
	Industry  string
	MarketCap float64
}

// scanCandidates 扫描候选股票
// 优先从全市场缓存读取（避免推荐结果局限于自选股），缓存为空时 fallback 到本地扫描
func scanCandidates(dataDir, excludeSymbol string, allSymbols []string, cacheItems map[string]MarketCacheItem) []candidateInfo {
	// 优先使用全市场缓存
	if len(cacheItems) > 0 {
		// 对 map key 排序后遍历，消除 Go map 遍历随机性，确保推荐结果稳定
		var keys []string
		for k := range cacheItems {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var candidates []candidateInfo
		dataRoot := filepath.Join(dataDir, "data")
		for _, symbol := range keys {
			if symbol == excludeSymbol {
				continue
			}
			item := cacheItems[symbol]
			industry := item.Industry
			// 如果本地有 profile.json，用其行业覆盖缓存行业
			// 保证候选股票与目标股票的行业来自同一数据源（东财API），命名体系一致
			profilePath := filepath.Join(dataRoot, symbol, "profile.json")
			if data, err := os.ReadFile(profilePath); err == nil {
				localIndustry := extractJSONString(data, "industry")
				if localIndustry != "" {
					industry = localIndustry
				}
			}
			// 优先使用本地 loadConcepts（含行业硬概念增强），本地无数据时 fallback 到缓存概念
			concepts := loadConcepts(dataRoot, symbol)
			if len(concepts) == 0 {
				concepts = item.Concepts
			}
			// 二级行业：优先取 cache 字段；cache 为空时直接查反查表（覆盖 cache 比反查表早构建的情况）
			subInd := item.SubIndustry
			if subInd == "" {
				subInd = lookupSubIndustry(dataRoot, symbol)
			}
			candidates = append(candidates, candidateInfo{
				Symbol:        item.Symbol,
				Name:          item.Name,
				Industry:      industry,
				SubIndustry:   subInd,
				MarketCap:     item.MarketCap,
				ROE:           item.ROE,
				GM:            item.GM,
				ActivityScore: loadActivityScore(dataRoot, symbol),
				HasData:       item.ROE != 0 || item.GM != 0, // 缓存中有财务指标视为有数据
				Concepts:      concepts,
			})
		}
		return candidates
	}

	// fallback: 本地数据目录扫描（旧逻辑，兼容无缓存场景）
	return scanLocalCandidates(dataDir, excludeSymbol, allSymbols)
}

// scanLocalCandidates 扫描本地数据目录获取候选股票，同时补充全市场代码
func scanLocalCandidates(dataDir, excludeSymbol string, allSymbols []string) []candidateInfo {
	dataRoot := filepath.Join(dataDir, "data")

	// 先扫描本地有数据的股票
	localMap := make(map[string]candidateInfo)
	entries, err := os.ReadDir(dataRoot)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			symbol := entry.Name()
			if symbol == excludeSymbol {
				continue
			}

			info := candidateInfo{Symbol: symbol}

			// 尝试读取 profile.json 获取行业和市值
			profilePath := filepath.Join(dataRoot, symbol, "profile.json")
			if data, err := os.ReadFile(profilePath); err == nil {
				info.Industry = extractJSONString(data, "industry")
				info.Name = extractJSONString(data, "name")
				if mc := extractJSONFloat(data, "market_cap"); mc > 0 {
					info.MarketCap = mc
				}
			}

			// 尝试读取财务数据获取 ROE 和毛利率
			bsPath := filepath.Join(dataRoot, symbol, "balance_sheet.json")
			isPath := filepath.Join(dataRoot, symbol, "income_statement.json")
			if _, err1 := os.Stat(bsPath); err1 == nil {
				if _, err2 := os.Stat(isPath); err2 == nil {
					info.HasData = true
					info.ROE, info.GM = extractLatestMetrics(dataRoot, symbol)
				}
			}

			// 尝试读取概念标签
			info.Concepts = loadConcepts(dataRoot, symbol)
			info.SubIndustry = lookupSubIndustry(dataRoot, symbol)

			// 读取活跃度（loadActivityScore 在 comparable.go 中定义，baseDir 为 dataDir）
			act := loadActivityScore(dataDir, symbol)
			if act < 0 {
				act = 0
			}
			info.ActivityScore = act

			localMap[symbol] = info
		}
	}

	// 如果有全市场代码列表，补充没有本地资料的候选
	if len(allSymbols) > 0 {
		for _, symbol := range allSymbols {
			if symbol == excludeSymbol {
				continue
			}
			if _, ok := localMap[symbol]; ok {
				continue
			}
			// 尝试读取 profile.json（可能由 batchFetchCandidateProfiles 缓存）
			info := candidateInfo{Symbol: symbol}
			profilePath := filepath.Join(dataRoot, symbol, "profile.json")
			if data, err := os.ReadFile(profilePath); err == nil {
				info.Industry = extractJSONString(data, "industry")
				info.Name = extractJSONString(data, "name")
				if mc := extractJSONFloat(data, "market_cap"); mc > 0 {
					info.MarketCap = mc
				}
			}
			// 补充概念标签
			info.Concepts = loadConcepts(dataRoot, symbol)
			info.SubIndustry = lookupSubIndustry(dataRoot, symbol)
			// 读取活跃度（loadActivityScore 在 comparable.go 中定义，baseDir 为 dataDir）
			act := loadActivityScore(dataDir, symbol)
			if act < 0 {
				act = 0
			}
			info.ActivityScore = act
			localMap[symbol] = info
		}
	}

	var candidates []candidateInfo
	for _, info := range localMap {
		candidates = append(candidates, info)
	}
	return candidates
}

// industrySynonyms 行业同义词映射（同一产业链的不同表述）
var industrySynonyms = map[string][]string{
	"半导体":   {"集成电路", "芯片", "晶圆", "封测", "IC"},
	"集成电路":  {"半导体", "芯片", "晶圆"},
	"芯片":    {"半导体", "集成电路"},
	"新能源":   {"光伏", "风电", "储能", "锂电池", "动力电池"},
	"光伏":    {"新能源", "太阳能"},
	"锂电池":   {"新能源", "动力电池", "储能"},
	"动力电池":  {"新能源", "锂电池"},
	"医药":    {"制药", "生物", "医疗器械", "中药", "化学药"},
	"制药":    {"医药", "生物", "化学药"},
	"生物":    {"医药", "制药", "生物技术"},
	"医疗器械":  {"医药", "医疗"},
	"银行":    {"金融", "商业银行", "城商行"},
	"保险":    {"金融", "寿险", "财险"},
	"证券":    {"金融", "券商", "投行"},
	"房地产":   {"地产", "房地产开发", "物业管理"},
	"地产":    {"房地产", "房地产开发"},
	"汽车":    {"整车", "新能源汽车", "汽车零部件"},
	"整车":    {"汽车", "新能源汽车"},
	"汽车零部件": {"汽车", "汽配"},
	"电子":    {"消费电子", "元器件", "PCB", "被动元件"},
	"消费电子":  {"电子", "手机", "可穿戴"},
	"通信":    {"电信", "5G", "光模块", "光纤"},
	"电信运营":  {"通信", "通信服务", "电信", "5G", "光模块"},
	"通信服务":  {"通信", "电信运营", "电信", "5G", "光模块"},
	"5G":    {"通信", "电信"},
	"计算机":   {"软件", "IT", "云计算", "人工智能", "AI"},
	"软件":    {"计算机", "IT", "云计算"},
	"人工智能":  {"计算机", "AI", "软件"},
	"化工":    {"化学", "化学制品", "精细化工", "石化", "化工原料", "电子化学品"},
	"化学制品":  {"化工", "精细化工", "化工原料", "电子化学品"},
	"化工原料":  {"化工", "化学制品", "精细化工", "电子化学品"},
	"电子化学品": {"化工", "化学制品", "化工原料", "精细化工"},
	"食品饮料":  {"食品", "饮料", "白酒", "啤酒", "乳制品"},
	"白酒":    {"食品饮料", "酒类"},
	"家电":    {"家用电器", "白色家电", "厨电"},
	"有色金属":  {"有色", "铜", "铝", "稀土", "锂"},
	"钢铁":    {"黑色金属", "特钢"},
	"煤炭":    {"能源", "焦煤", "动力煤"},
	"石油":    {"能源", "石化", "油气"},
	"电力":    {"公用事业", "火电", "水电", "核电"},
	"交通运输":  {"物流", "航空", "航运", "港口", "铁路"},
	"物流":    {"交通运输", "快递", "供应链"},
	"传媒":    {"媒体", "广告", "影视", "游戏", "互联网"},
	"游戏":    {"传媒", "互联网", "电竞"},
	"互联网":   {"传媒", "软件", "IT"},
	"建筑":    {"基建", "建筑工程", "建材", "装饰"},
	"建材":    {"建筑", "水泥", "玻璃"},
	"农林牧渔":  {"农业", "养殖", "种植", "畜牧"},
	"养殖":    {"农林牧渔", "畜牧", "猪"},
}

// 等效行业映射（不同数据源/平台对同一行业的不同命名，或同一产业链强相关的细分行业）
// 关键差异 vs industrySynonyms：等价对会拿到与"完全同行业"相近的赛道分（25→35 提到 30），
// 而 synonyms 只给 15 分。这里收录的是"同业务领域、不同命名"或"高度产业链耦合"的组合。
//
// 消费电子链：东财把整机/模组/触显/玻璃盖板细分到 4 个一级行业，但业务实质上是一个生态。
// 长信科技(光学光电子)/蓝思(消费电子)/领益(消费电子)/精研(消费电子) 都是苹果链同业。
var equivalentIndustries = map[string][]string{
	"电信运营":  {"通信服务"},
	"通信服务":  {"电信运营"},
	"光学光电子": {"消费电子", "元器件", "电子元件"},
	"消费电子":  {"光学光电子", "元器件", "电子元件"},
	"元器件":   {"光学光电子", "消费电子", "电子元件"},
	"电子元件":  {"光学光电子", "消费电子", "元器件"},
	// 半导体材料链：电子化学品/化工原料/半导体材料三者实质同业
	"电子化学品": {"半导体材料", "化工原料"},
	"半导体材料": {"电子化学品", "化工原料", "半导体"},
	"化工原料":  {"电子化学品", "半导体材料"},
}

// computeSimilarity 计算候选股票与目标股票的相似度
// 赛道匹配 = max(二级行业 75, 一级行业 35, 概念 IDF 加权 70)
//
//   - 强概念叠加奖励 (≥3 共享高 IDF 概念时每个 4 分，封顶 20)
//   - 市值5% + ROE10% + 毛利率15% + 活跃度10% + 数据2%
//
// 二级行业升级为主信号：药明康德这类公司一级行业"医疗行业"过宽，二级"医疗研发外包"才区分CXO
// 强概念叠加奖励：解决"长信(光学光电子) vs 蓝思(消费电子)"这类一级行业不同但业务高度同业的错配
// IDF 加权：让"3D玻璃(15家)"比"智能穿戴(100+家)"更有区分度，避免面板厂排在玻璃盖板厂之前
func computeSimilarity(targetIndustry, targetSubIndustry string, targetMarketCap, targetROE, targetGM, targetActivity float64, targetConcepts []string, c candidateInfo, m *conceptMembership) (float64, []string, string) {
	score := 0.0
	var reasons []string

	// ===== 1. 赛道匹配 = max(二级行业, 一级行业, 概念匹配) =====
	// 1a. 二级行业精确匹配（最高权重 75）
	subIndustryScore := 0.0
	var subIndustryReasons []string
	if targetSubIndustry != "" && c.SubIndustry != "" && targetSubIndustry == c.SubIndustry {
		subIndustryScore = 75
		subIndustryReasons = append(subIndustryReasons, "同属"+targetSubIndustry)
	}

	// 1b. 一级行业匹配
	industryScore := 0.0
	var industryReasons []string
	if targetIndustry != "" && c.Industry != "" {
		if targetIndustry == c.Industry {
			industryScore = 35
			industryReasons = append(industryReasons, "同属"+targetIndustry)
		} else if isEquivalentIndustry(targetIndustry, c.Industry) {
			industryScore = 30
			industryReasons = append(industryReasons, "同属"+targetIndustry+"链")
		} else {
			targetKeys := extractIndustryKeywords(targetIndustry)
			candidateKeys := extractIndustryKeywords(c.Industry)
			matchCount := 0
			for _, tk := range targetKeys {
				for _, ck := range candidateKeys {
					if tk == ck && len(tk) >= 2 {
						matchCount++
					}
				}
			}
			if matchCount > 0 {
				partialScore := 35.0 * float64(matchCount) / float64(len(targetKeys))
				if partialScore > 35 {
					partialScore = 35
				}
				industryScore = partialScore
				industryReasons = append(industryReasons, "行业相近")
			} else {
				if isSynonymIndustry(targetIndustry, c.Industry) {
					industryScore = 15
					industryReasons = append(industryReasons, "产业链相关")
				}
			}
		}
	}

	// 1c. 概念匹配（IDF 加权）
	// 普通 overlap/count 把"智能穿戴(100+家)"和"3D玻璃(15家)"等权，长信场景下面板厂(京东方/深天马)
	// 共享 5 个泛主题概念分高于玻璃厂(凯盛/宜安)共享 2 个业务概念。
	// IDF 加权后，稀有的业务标签(3D玻璃 IDF≈5.7)权重 ≈ 泛主题(智能穿戴 IDF≈3.8)的 1.5 倍。
	conceptScore := 0.0
	conceptOverlap := 0          // 共享概念总数（兼容回归测试）
	rareConceptOverlap := 0      // 共享中高 IDF（业务标签）概念数
	const rareIDFThreshold = 4.5 // 经验值：≥4.5 视为业务标签型（命中<约 50 家）
	var conceptReasons []string
	if len(targetConcepts) > 0 && len(c.Concepts) > 0 {
		// candidate concepts 转 set 加速查找
		candSet := make(map[string]struct{}, len(c.Concepts))
		for _, cc := range c.Concepts {
			candSet[cc] = struct{}{}
		}
		sharedIDF := 0.0
		targetIDF := 0.0
		for _, tc := range targetConcepts {
			idf := conceptIDF(m, tc)
			targetIDF += idf
			if _, ok := candSet[tc]; ok {
				conceptOverlap++
				sharedIDF += idf
				if idf >= rareIDFThreshold {
					rareConceptOverlap++
				}
			}
		}
		if conceptOverlap > 0 && targetIDF > 0 {
			conceptScore = 70.0 * sharedIDF / targetIDF
			if conceptScore > 70 {
				conceptScore = 70
			}
			conceptReasons = append(conceptReasons, fmt.Sprintf("共享%d个概念", conceptOverlap))
		}
	}

	// 1d. 取赛道匹配最高分（subIndustry > industry > concept 的优先级仅影响 reasons 展示）
	trackScore := subIndustryScore
	reasons = subIndustryReasons
	if industryScore > trackScore {
		trackScore = industryScore
		reasons = industryReasons
	}
	if conceptScore > trackScore {
		trackScore = conceptScore
		reasons = conceptReasons
	}
	score += trackScore

	// 1e. 强概念重叠叠加奖励（不进 max，独立加分）
	// 场景：长信(光学光电子) vs 蓝思(消费电子)——一级行业不同(光学光电子的等价表给 30 分)，
	// 但 3D玻璃/混合现实等业务标签重叠说明业务高度同业。
	// 加分条件改为"≥2 个高 IDF (业务标签型) 概念"，避免泛主题概念(智能穿戴/苹果概念)堆数刷分。
	// 每多 1 个加 5 分，封顶 20 分。
	if rareConceptOverlap >= 2 && trackScore != conceptScore {
		bonus := float64(rareConceptOverlap-1) * 5
		if bonus > 20 {
			bonus = 20
		}
		score += bonus
		// reason 已通过 trackScore 路径展示；这里追加业务关联信号，避免被一级行业 reason 吞掉
		hasConceptReason := false
		for _, r := range reasons {
			if strings.Contains(r, "概念") {
				hasConceptReason = true
				break
			}
		}
		if !hasConceptReason {
			reasons = append(reasons, fmt.Sprintf("共享%d个概念", conceptOverlap))
		}
	}

	// 2. 市值相近 (5%)
	if targetMarketCap > 0 && c.MarketCap > 0 {
		ratio := targetMarketCap / c.MarketCap
		if ratio < 1 {
			ratio = c.MarketCap / targetMarketCap
		}
		if ratio <= 2 {
			score += 5
			reasons = append(reasons, "市值相近")
		} else if ratio <= 5 {
			ms := 5 * (1 - (ratio-2)/3)
			if ms < 0 {
				ms = 0
			}
			score += ms
			reasons = append(reasons, "市值相近")
		}
	}

	// 3. ROE 结构相似 (10%)
	if targetROE != 0 && c.ROE != 0 {
		diff := math.Abs(targetROE - c.ROE)
		if diff < 0.03 {
			score += 10
			reasons = append(reasons, "ROE结构相似")
		} else if diff < 0.10 {
			score += 10 * (1 - (diff-0.03)/0.07)
			reasons = append(reasons, "ROE结构相似")
		}
	} else if targetROE != 0 && trackScore >= 40 {
		score += 4
		reasons = append(reasons, "同行业ROE预期相近")
	}

	// 4. 毛利率结构相似 (15%)
	if targetGM != 0 && c.GM != 0 {
		diff := math.Abs(targetGM - c.GM)
		if diff < 0.05 {
			score += 15
			reasons = append(reasons, "毛利率结构相似")
		} else if diff < 0.15 {
			score += 15 * (1 - (diff-0.05)/0.10)
			reasons = append(reasons, "毛利率结构相似")
		}
	} else if targetGM != 0 && trackScore >= 40 {
		score += 4
		reasons = append(reasons, "同行业毛利率预期相近")
	}

	// 5. 活跃度相近 (10%)
	if targetActivity > 0 && c.ActivityScore > 0 {
		diff := math.Abs(targetActivity - c.ActivityScore)
		if diff < 10 {
			score += 10
			reasons = append(reasons, "活跃度相近")
		} else if diff < 30 {
			as := 10 * (1 - (diff-10)/20)
			if as < 0 {
				as = 0
			}
			score += as
			reasons = append(reasons, "活跃度相近")
		}
	}

	// 6. 数据质量 (2%)
	dataQuality := "low"
	if c.HasData {
		score += 2
		dataQuality = "high"
		reasons = append(reasons, "本地有财报数据")
	} else if c.Industry != "" {
		score += 1
		dataQuality = "medium"
	}

	// 惩罚1：目标有行业但候选无行业数据，得分打 3 折
	if targetIndustry != "" && c.Industry == "" {
		score = score * 0.3
	}

	// 惩罚2：赛道匹配分 < 15 视为业务不相关，得分打 2 折
	// （原逻辑：industryScore==0 打2折；新逻辑：trackScore<15 打2折，允许概念匹配兜底）
	if targetIndustry != "" && c.Industry != "" && trackScore < 15 {
		score = score * 0.2
	}

	return score, reasons, dataQuality
}

// isEquivalentIndustry 检查两个行业是否为同一行业的不同命名
func isEquivalentIndustry(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if eqs, ok := equivalentIndustries[a]; ok {
		for _, e := range eqs {
			if e == b {
				return true
			}
		}
	}
	if eqs, ok := equivalentIndustries[b]; ok {
		for _, e := range eqs {
			if e == a {
				return true
			}
		}
	}
	return false
}

// isSynonymIndustry 检查两个行业是否通过同义词映射相关
func isSynonymIndustry(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	// 直接检查映射表
	if syns, ok := industrySynonyms[a]; ok {
		for _, s := range syns {
			if strings.Contains(b, s) || strings.Contains(s, b) {
				return true
			}
		}
	}
	// 反向检查
	if syns, ok := industrySynonyms[b]; ok {
		for _, s := range syns {
			if strings.Contains(a, s) || strings.Contains(s, a) {
				return true
			}
		}
	}
	return false
}

// extractIndustryKeywords 从行业名称中提取关键词
func extractIndustryKeywords(industry string) []string {
	parts := strings.FieldsFunc(industry, func(r rune) bool {
		return r == '、' || r == '/' || r == '·' || r == ' ' || r == '，' || r == ','
	})
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 {
			result = append(result, p)
		}
	}
	return result
}

// extractJSONString 从 JSON 字节中简单提取字符串值（无引号转义处理，仅适用于简单 JSON）
func extractJSONString(data []byte, key string) string {
	keyPattern := `"` + key + `"`
	idx := strings.Index(string(data), keyPattern)
	if idx < 0 {
		return ""
	}
	rest := string(data)[idx+len(keyPattern):]
	i := 0
	for i < len(rest) && (rest[i] == ':' || rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r') {
		i++
	}
	if i < len(rest) && rest[i] == '"' {
		i++
		start := i
		for i < len(rest) && rest[i] != '"' {
			i++
		}
		return rest[start:i]
	}
	return ""
}

// extractJSONFloat 从 JSON 字节中简单提取 float64 值
func extractJSONFloat(data []byte, key string) float64 {
	keyPattern := `"` + key + `"`
	idx := strings.Index(string(data), keyPattern)
	if idx < 0 {
		return 0
	}
	rest := string(data)[idx+len(keyPattern):]
	i := 0
	for i < len(rest) && (rest[i] == ':' || rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r') {
		i++
	}
	start := i
	for i < len(rest) && (rest[i] == '-' || rest[i] == '.' || (rest[i] >= '0' && rest[i] <= '9')) {
		i++
	}
	var val float64
	if _, err := fmt.Sscanf(rest[start:i], "%f", &val); err == nil {
		return val
	}
	return 0
}

// extractLatestMetrics 从本地数据中提取最新年份的 ROE 和毛利率
func extractLatestMetrics(dataRoot, symbol string) (roe, gm float64) {
	fd, err := LoadFinancialData(dataRoot, symbol)
	if err != nil || len(fd.Years) == 0 {
		return 0, 0
	}
	year := fd.Years[0]
	equity := fd.GetValueOrZero(fd.BalanceSheet, "所有者权益合计", year)
	if equity == 0 {
		totalAssets := fd.GetValueOrZero(fd.BalanceSheet, "总资产", year)
		totalLiabilities := fd.GetValueOrZero(fd.BalanceSheet, "总负债", year)
		equity = totalAssets - totalLiabilities
	}
	netProfit := fd.GetValueOrZero(fd.IncomeStatement, "净利润", year)
	revenue := fd.GetValueOrZero(fd.IncomeStatement, "营业收入", year)
	cost := fd.GetValueOrZero(fd.IncomeStatement, "营业成本", year)
	if equity > 0 {
		roe = netProfit / equity
	}
	if revenue > 0 {
		gm = (revenue - cost) / revenue
	}
	return
}

// 行业到硬业务概念的增强映射：数据源概念标签往往过于宽泛，根据行业补充细分赛道概念
var industryConceptBoost = map[string][]string{
	"电子化学品": {"光刻胶", "CMP", "湿电子化学品", "电子特气", "半导体材料", "封装材料"},
	"半导体":   {"晶圆", "光刻胶", "CMP", "封测", "刻蚀", "薄膜沉积", "芯片设计"},
	"化工原料":  {"光刻胶", "CMP", "电子化学品", "新材料", "半导体材料"},
	"集成电路":  {"晶圆", "光刻胶", "CMP", "封测", "芯片设计", "EDA"},
	"芯片":    {"晶圆", "光刻胶", "封测", "芯片设计", "AI芯片"},
	"医药":    {"创新药", "仿制药", "CXO", "生物药"},
	"制药":    {"创新药", "仿制药", "CXO", "生物药"},
	"化学制药":  {"创新药", "仿制药", "原料药", "CXO"},
	"生物制药":  {"创新药", "生物药", "疫苗", "CXO"},
	"医疗器械":  {"高值耗材", "医疗设备", "IVD", "康复器械"},
	"新能源":   {"锂电池", "光伏", "风电", "储能", "氢能"},
	"光伏":    {"硅料", "硅片", "电池片", "组件", "逆变器"},
	"锂电池":   {"正极材料", "负极材料", "电解液", "隔膜", "固态电池"},
	"汽车":    {"新能源汽车", "智能驾驶", "汽车零部件", "车联网"},
	"整车":    {"新能源汽车", "智能驾驶", "出海"},
	"汽车零部件": {"智能驾驶", "轻量化", "热管理", "线控底盘"},
	"消费电子":  {"手机", "可穿戴", "AR/VR", "智能家居"},
	"通信":    {"5G", "光模块", "卫星通信", "算力网络"},
	"通信设备":  {"5G", "光模块", "基站", "卫星通信"},
	"计算机":   {"云计算", "人工智能", "信创", "数据要素"},
	"软件":    {"SaaS", "人工智能", "信创", "工业软件"},
	"人工智能":  {"大模型", "算力", "AI应用", "机器人"},
	"化工":    {"新材料", "精细化工", "电子化学品", "可降解"},
	"化学制品":  {"新材料", "精细化工", "电子化学品"},
	"食品饮料":  {"白酒", "乳制品", "预制菜", "调味品"},
	"白酒":    {"高端白酒", "次高端白酒", "酱酒"},
	"有色金属":  {"锂", "稀土", "铜", "铝", "黄金"},
	"电力":    {"火电", "水电", "核电", "新能源发电"},
	"传媒":    {"游戏", "影视", "广告", "短剧"},
	"游戏":    {"手游", "端游", "出海", "AI游戏"},
	"互联网":   {"电商", "本地生活", "云计算", "出海"},
	"建筑":    {"基建", "房建", "出海", "装配式建筑"},
	"建材":    {"水泥", "玻璃", "消费建材", "新材料"},
	"交通运输":  {"快递", "航空", "港口", "物流"},
	"物流":    {"快递", "供应链", "冷链", "跨境物流"},
}

// loadConcepts 读取股票的概念/风口标签，按优先级合并多源
//  1. 全局反查表 _concept_membership.json：东财概念板块成分股反向索引，最权威
//  2. 本地 concepts.json：数据源 API 返回，覆盖目标股票自身
//  3. industryConceptBoost：行业到硬业务概念的规则映射兜底
//
// 反查表/本地文件不存在时，分别静默退化为只用其余来源；规则映射始终生效。
func loadConcepts(dataRoot, symbol string) []string {
	var result []string
	seen := make(map[string]struct{})
	add := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" {
			return
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		result = append(result, c)
	}

	// 1. 全局反查表（如已构建）
	if m := loadConceptMembership(dataRoot); m != nil {
		for _, c := range m.Concepts[symbol] {
			add(c)
		}
	}

	// 2. 本地 concepts.json
	if data, err := os.ReadFile(filepath.Join(dataRoot, symbol, "concepts.json")); err == nil {
		for _, c := range extractJSONStringArray(data, "concepts") {
			if strings.Contains(c, "、") {
				for _, part := range strings.Split(c, "、") {
					add(part)
				}
			} else {
				add(c)
			}
		}
	}

	// 3. industryConceptBoost：基于本地 profile.json 的行业，补充硬业务概念
	if pdata, err := os.ReadFile(filepath.Join(dataRoot, symbol, "profile.json")); err == nil {
		industry := extractJSONString(pdata, "industry")
		if industry != "" {
			for _, c := range industryConceptBoost[industry] {
				add(c)
			}
		}
	}

	return result
}

// extractJSONStringArray 从 JSON 字节中简单提取字符串数组
func extractJSONStringArray(data []byte, key string) []string {
	keyPattern := `"` + key + `"`
	idx := strings.Index(string(data), keyPattern)
	if idx < 0 {
		return nil
	}
	rest := string(data)[idx+len(keyPattern):]
	i := 0
	for i < len(rest) && (rest[i] == ':' || rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r') {
		i++
	}
	if i >= len(rest) || rest[i] != '[' {
		return nil
	}
	i++ // skip [
	var result []string
	for i < len(rest) {
		// skip whitespace
		for i < len(rest) && (rest[i] == ' ' || rest[i] == '\t' || rest[i] == '\n' || rest[i] == '\r' || rest[i] == ',') {
			i++
		}
		if i < len(rest) && rest[i] == ']' {
			break
		}
		if i < len(rest) && rest[i] == '"' {
			i++
			start := i
			for i < len(rest) && rest[i] != '"' {
				i++
			}
			result = append(result, rest[start:i])
			if i < len(rest) && rest[i] == '"' {
				i++
			}
		} else {
			i++
		}
	}
	return result
}
