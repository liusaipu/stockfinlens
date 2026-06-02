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

// ConceptMembership 全市场概念板块/行业板块成分反查表
// 用于可比公司推荐：替代规则映射 industryConceptMap，提供精确的"股票→概念板块"映射，
// 并填充二级行业（SubIndustry）作为推荐的强匹配信号。
type ConceptMembership struct {
	Version      int                 `json:"version"`        // 缓存格式版本
	UpdatedAt    string              `json:"updated_at"`     // ISO8601
	ConceptCount int                 `json:"concept_count"`  // 概念板块总数
	IndustryCount int                `json:"industry_count"` // 行业板块总数
	SymbolCount  int                 `json:"symbol_count"`   // 覆盖股票总数
	// Concepts: symbol(带后缀如 600519.SH) → 概念板块名列表
	Concepts map[string][]string `json:"concepts"`
	// SubIndustry: symbol → 二级行业（东财行业板块名，如 "医疗研发外包"）
	SubIndustry map[string]string `json:"sub_industry"`
}

const (
	conceptMembershipFile    = "_concept_membership.json"
	conceptMembershipVersion = 1
	conceptMembershipMaxAge  = 7 * 24 * time.Hour
	// 并发拉取成分股的 goroutine 上限。东财对 push2 域名节流较松，
	// 但同时多 CDN 节点轮询会放大请求量，5 是经验值。
	conceptMembershipConcurrency = 5
)

// 板块名黑名单：跳过指数/ETF/区域类板块，避免污染反查表。
// 关键词命中即跳过（大小写敏感，纯中文场景够用）。
var conceptBoardBlacklist = []string{
	"指数", "ETF", "概念股", "融资融券",
	"江苏板块", "广东板块", "浙江板块", "山东板块",
	"福建板块", "湖南板块", "湖北板块", "四川板块",
	"安徽板块", "河北板块", "河南板块", "江西板块",
	"上海板块", "北京板块", "天津板块", "重庆板块",
	"东北板块", "西北板块", "西南板块", "华南板块",
	"华北板块", "华东板块", "华中板块",
	"深圳本地", "上海本地", "北京本地",
}

// isBoardBlacklisted 判断板块名是否在黑名单中
func isBoardBlacklisted(name string) bool {
	if name == "" {
		return true
	}
	for _, kw := range conceptBoardBlacklist {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// conceptMembershipPath 返回缓存文件路径
func conceptMembershipPath(dataDir string) string {
	return filepath.Join(dataDir, conceptMembershipFile)
}

// LoadConceptMembership 从磁盘加载反查表
// 文件不存在或解析失败返回 nil（调用方应当作"尚未生成"处理）
func LoadConceptMembership(dataDir string) *ConceptMembership {
	path := conceptMembershipPath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var m ConceptMembership
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}
	if m.Version != conceptMembershipVersion {
		return nil
	}
	if m.Concepts == nil {
		m.Concepts = make(map[string][]string)
	}
	if m.SubIndustry == nil {
		m.SubIndustry = make(map[string]string)
	}
	return &m
}

// IsExpired 判断反查表是否已超过 TTL
func (m *ConceptMembership) IsExpired() bool {
	if m == nil || m.UpdatedAt == "" {
		return true
	}
	t, err := time.Parse(time.RFC3339, m.UpdatedAt)
	if err != nil {
		return true
	}
	return time.Since(t) > conceptMembershipMaxAge
}

// saveConceptMembership 原子写入磁盘
func saveConceptMembership(dataDir string, m *ConceptMembership) error {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	path := conceptMembershipPath(dataDir)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// fetchIndustryBoardsFromEastMoney 拉取东财全部行业板块（m:90+t:1）
// 字段映射与 fetchConceptBoardFromEastMoney 一致，但仅取代码/名称（行业板块不需要打分）
func fetchIndustryBoardsFromEastMoney(ctx context.Context) ([]HotConcept, error) {
	url := fmt.Sprintf(
		"http://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=500&po=1&np=1&ut=bd1d9ddb04089700cf9c27f6f7426281&fltt=2&invt=2&fid=f3&fs=%s&fields=%s&_=%d",
		emFSIndustry,
		emHotConceptFields,
		time.Now().UnixMilli(),
	)
	body, err := httpGetEastMoney(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("请求行业板块失败: %w", err)
	}
	body = stripJSONP(body)

	var resp struct {
		RC   int `json:"rc"`
		Data struct {
			Total int                      `json:"total"`
			Diff  []map[string]interface{} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("解析行业板块响应失败: %w", err)
	}
	if resp.RC != 0 && resp.RC != 200 {
		return nil, fmt.Errorf("行业板块 API rc=%d", resp.RC)
	}

	boards := make([]HotConcept, 0, len(resp.Data.Diff))
	for _, item := range resp.Data.Diff {
		code := parseString(item["f12"])
		name := parseString(item["f14"])
		if code != "" && !strings.HasPrefix(code, "BK") {
			code = "BK" + code
		}
		if code == "" || name == "" {
			continue
		}
		boards = append(boards, HotConcept{Code: code, Name: name})
	}
	return boards, nil
}

// fetchAllConceptBoards 拉取概念板块全集（不走 4 小时缓存，直接打 API）
// 该函数与 FetchHotConceptBoard 的差异：不打分、不走缓存、不应用 topN，要的就是全集
func fetchAllConceptBoards(ctx context.Context) ([]HotConcept, error) {
	concepts, err := fetchConceptBoardFromEastMoney(ctx)
	if err != nil {
		return nil, err
	}
	return concepts, nil
}

// constituentToSymbol 将成分股的 (Code, Market) 转换为带后缀的 symbol
// 东财成分股 API 的 f12 字段是 6 位数字代码，f13 给出市场（0=SZ, 1=SH, 2=BJ）
// 输出格式与 MarketCacheItem.Symbol（如 "600519.SH"）一致
func constituentToSymbol(c ConceptConstituent) string {
	market := c.Market
	if market == "" {
		market = inferMarketFromCode(c.Code)
	}
	if market == "" || c.Code == "" {
		return ""
	}
	return c.Code + "." + market
}

// BuildConceptMembership 拉取东财全部概念板块 + 行业板块的成分股，构建反查表
// 该过程耗时较长（每个板块一次 HTTP，500+ 板块 × 网络延迟），仅在后台调用
func BuildConceptMembership(ctx context.Context) (*ConceptMembership, error) {
	startAt := time.Now()
	// 1. 拉取板块列表
	concepts, err := fetchAllConceptBoards(ctx)
	if err != nil {
		return nil, fmt.Errorf("拉取概念板块列表失败: %w", err)
	}
	industries, err := fetchIndustryBoardsFromEastMoney(ctx)
	if err != nil {
		// 行业板块失败不致命，仍可只用概念板块构建反查表（SubIndustry 缺失而已）
		fmt.Printf("[ConceptMembership] 拉取行业板块失败（仅构建概念反查）: %v\n", err)
		industries = nil
	}

	// 黑名单过滤
	filteredConcepts := make([]HotConcept, 0, len(concepts))
	for _, c := range concepts {
		if isBoardBlacklisted(c.Name) {
			continue
		}
		filteredConcepts = append(filteredConcepts, c)
	}
	filteredIndustries := make([]HotConcept, 0, len(industries))
	for _, c := range industries {
		if isBoardBlacklisted(c.Name) {
			continue
		}
		filteredIndustries = append(filteredIndustries, c)
	}
	fmt.Printf("[ConceptMembership] 板块: 概念 %d (过滤后) + 行业 %d (过滤后)\n", len(filteredConcepts), len(filteredIndustries))

	// 2. 并发拉取成分股
	type boardJob struct {
		board     HotConcept
		isIndustry bool
	}
	jobs := make([]boardJob, 0, len(filteredConcepts)+len(filteredIndustries))
	for _, c := range filteredConcepts {
		jobs = append(jobs, boardJob{board: c, isIndustry: false})
	}
	for _, c := range filteredIndustries {
		jobs = append(jobs, boardJob{board: c, isIndustry: true})
	}

	resultConcepts := make(map[string]map[string]struct{}) // symbol -> set(boardName)
	resultSubInd := make(map[string]string)                // symbol -> 二级行业（行业板块名）
	var mu sync.Mutex

	sem := make(chan struct{}, conceptMembershipConcurrency)
	var wg sync.WaitGroup
	successCount := 0
	failCount := 0
	for _, job := range jobs {
		// 防止外层 ctx 取消时继续提交新 goroutine
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(j boardJob) {
			defer wg.Done()
			defer func() { <-sem }()

			// 单板块超时控制（成分股 API 平均 < 1s，给足 8s）
			rctx, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			stocks, err := FetchConceptConstituents(rctx, j.board.Code)
			if err != nil {
				mu.Lock()
				failCount++
				mu.Unlock()
				return
			}
			mu.Lock()
			defer mu.Unlock()
			successCount++
			for _, s := range stocks {
				sym := constituentToSymbol(s)
				if sym == "" {
					continue
				}
				if j.isIndustry {
					// 二级行业一只股票应只属一个东财行业板块；如果多个，后写覆盖前写（首次出现优先）
					if _, exists := resultSubInd[sym]; !exists {
						resultSubInd[sym] = j.board.Name
					}
				} else {
					set, ok := resultConcepts[sym]
					if !ok {
						set = make(map[string]struct{})
						resultConcepts[sym] = set
					}
					set[j.board.Name] = struct{}{}
				}
			}
		}(job)
	}
	wg.Wait()

	// 3. 整理结果
	conceptsMap := make(map[string][]string, len(resultConcepts))
	for sym, set := range resultConcepts {
		list := make([]string, 0, len(set))
		for name := range set {
			list = append(list, name)
		}
		conceptsMap[sym] = list
	}

	m := &ConceptMembership{
		Version:       conceptMembershipVersion,
		UpdatedAt:     time.Now().Format(time.RFC3339),
		ConceptCount:  len(filteredConcepts),
		IndustryCount: len(filteredIndustries),
		SymbolCount:   len(conceptsMap),
		Concepts:      conceptsMap,
		SubIndustry:   resultSubInd,
	}
	fmt.Printf("[ConceptMembership] 构建完成: 板块 %d 成功 / %d 失败, 覆盖股票 %d, 行业映射 %d, 耗时 %.1fs\n",
		successCount, failCount, len(conceptsMap), len(resultSubInd), time.Since(startAt).Seconds())
	return m, nil
}

// RefreshConceptMembership 拉取并保存反查表（顶层入口）
// 该函数会阻塞直到完成或 ctx 取消，预期由后台 goroutine 调用
func RefreshConceptMembership(ctx context.Context, dataDir string) (*ConceptMembership, error) {
	m, err := BuildConceptMembership(ctx)
	if err != nil {
		return nil, err
	}
	if err := saveConceptMembership(dataDir, m); err != nil {
		return m, fmt.Errorf("保存反查表失败: %w", err)
	}
	return m, nil
}
