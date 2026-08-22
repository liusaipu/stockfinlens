package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/liusaipu/stockfinlens/ai_researcher"
	"github.com/liusaipu/stockfinlens/analyzer"
	"github.com/liusaipu/stockfinlens/downloader"
)

// Storage 本地文件存储管理器
type Storage struct {
	dataDir string
}

// NewStorage 创建存储管理器，目录位于 ~/.config/stock-analyzer
func NewStorage() (*Storage, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}
	dataDir := filepath.Join(home, ".config", "stock-analyzer")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}
	return &Storage{dataDir: dataDir}, nil
}

// DataDir 返回数据根目录
func (s *Storage) DataDir() string {
	return s.dataDir
}

// WatchlistPath 返回自选列表文件路径
func (s *Storage) WatchlistPath() string {
	return filepath.Join(s.dataDir, "watchlist.json")
}

// LoadWatchlist 加载自选列表
func (s *Storage) LoadWatchlist() ([]WatchlistItem, error) {
	path := s.WatchlistPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []WatchlistItem{}, nil
		}
		return nil, err
	}
	var list []WatchlistItem
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}

// SaveWatchlist 保存自选列表
func (s *Storage) SaveWatchlist(list []WatchlistItem) error {
	path := s.WatchlistPath()
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// EnsureStockDataDir 确保某只股票的数据目录存在
func (s *Storage) EnsureStockDataDir(symbol string) (string, error) {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// EnsureReportDir 确保某只股票的报告目录存在
func (s *Storage) EnsureReportDir(symbol string) (string, error) {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// CleanStockData 删除某只股票的所有本地数据（财报、报告、缓存、可比公司等）
func (s *Storage) CleanStockData(symbol string) error {
	var errs []string
	// 删除财报数据目录
	if err := os.RemoveAll(filepath.Join(s.dataDir, "data", symbol)); err != nil {
		errs = append(errs, fmt.Sprintf("data: %v", err))
	}
	// 删除报告目录
	if err := os.RemoveAll(filepath.Join(s.dataDir, "reports", symbol)); err != nil {
		errs = append(errs, fmt.Sprintf("reports: %v", err))
	}
	// 删除可比公司缓存目录
	if err := os.RemoveAll(filepath.Join(s.dataDir, "comparables", symbol)); err != nil {
		errs = append(errs, fmt.Sprintf("comparables: %v", err))
	}
	if len(errs) > 0 {
		return fmt.Errorf("清理数据失败: %s", strings.Join(errs, "; "))
	}
	return nil
}

// HistoryMeta 历史数据批次元信息
type HistoryMeta struct {
	Timestamp  string   `json:"timestamp"`
	Source     string   `json:"source"`
	SourceName string   `json:"sourceName"`
	Years      []string `json:"years"`
}

// StockProfile 股票基本信息
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
	PoliticalAffiliation string  `json:"politicalAffiliation"`
	UpdatedAt            string  `json:"updatedAt"`
}

// ArchiveStockData 将当前股票数据归档为历史版本，并只保留最近3批
func (s *Storage) ArchiveStockData(symbol string, meta HistoryMeta) error {
	stockDir := filepath.Join(s.dataDir, "data", symbol)
	historyDir := filepath.Join(stockDir, "history", meta.Timestamp)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("创建历史目录失败: %w", err)
	}

	files := []string{"balance_sheet.json", "income_statement.json", "cash_flow.json"}
	for _, name := range files {
		src := filepath.Join(stockDir, name)
		dst := filepath.Join(historyDir, name)
		if err := copyFile(src, dst); err != nil {
			// 如果源文件不存在则跳过（可能只导入了两张表）
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("复制 %s 失败: %w", name, err)
		}
	}

	metaPath := filepath.Join(historyDir, "meta.json")
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return err
	}

	return s.cleanupOldHistory(symbol)
}

// ListStockDataHistory 列出某只股票的历史数据批次
func (s *Storage) ListStockDataHistory(symbol string) ([]HistoryMeta, error) {
	historyRoot := filepath.Join(s.dataDir, "data", symbol, "history")
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var metas []HistoryMeta
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(historyRoot, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta HistoryMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		metas = append(metas, meta)
	}
	// 按时间戳从新到旧排序
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Timestamp > metas[j].Timestamp
	})
	return metas, nil
}

// cleanupOldHistory 清理旧历史数据，只保留最近3批
func (s *Storage) cleanupOldHistory(symbol string) error {
	historyRoot := filepath.Join(s.dataDir, "data", symbol, "history")
	entries, err := os.ReadDir(historyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name())
		}
	}
	if len(dirs) <= 3 {
		return nil
	}

	// 按目录名（即时间戳字符串）排序，旧的在后面
	sort.Strings(dirs)
	// 删除最旧的目录，直到只剩3个
	for i := 0; i < len(dirs)-3; i++ {
		target := filepath.Join(historyRoot, dirs[i])
		if err := os.RemoveAll(target); err != nil {
			return err
		}
	}
	return nil
}

// SaveReport 将 Markdown 报告保存到 reports/{symbol}/latest.md
func (s *Storage) SaveReport(symbol string, content string, overwriteLatest bool) (string, error) {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建报告目录失败: %w", err)
	}
	// 清理所有旧报告文件，只保留 latest.md
	_ = s.cleanupOldReports(symbol)
	filename := "latest.md"
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("保存报告失败: %w", err)
	}
	return filename, nil
}

// ListReports 列出某只股票的历史报告文件名（始终只返回 latest.md）
func (s *Storage) ListReports(symbol string) ([]string, error) {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	path := filepath.Join(dir, "latest.md")
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return []string{"latest.md"}, nil
}

// LoadReport 读取指定历史报告的 Markdown 内容
func (s *Storage) LoadReport(symbol, filename string) (string, error) {
	path := filepath.Join(s.dataDir, "reports", symbol, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeleteReport 删除指定历史报告
func (s *Storage) DeleteReport(symbol, filename string) error {
	path := filepath.Join(s.dataDir, "reports", symbol, filename)
	if err := os.Remove(path); err != nil {
		return err
	}
	return nil
}

// AnalysisCache 分析缓存元数据
type AnalysisCache struct {
	DataHash        string `json:"dataHash"`
	ComparablesHash string `json:"comparablesHash"`
	LastAnalysisAt  string `json:"lastAnalysisAt"`
}

// SaveAnalysisCache 保存分析缓存
func (s *Storage) SaveAnalysisCache(symbol, dataHash, comparablesHash string) error {
	path := filepath.Join(s.dataDir, "data", symbol, "analysis_cache.json")
	cache := AnalysisCache{
		DataHash:        dataHash,
		ComparablesHash: comparablesHash,
		LastAnalysisAt:  time.Now().Format("2006-01-02 15:04:05"),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// LoadAnalysisCache 读取分析缓存
func (s *Storage) LoadAnalysisCache(symbol string) (*AnalysisCache, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "analysis_cache.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var cache AnalysisCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

// ComputeDataHash 计算股票原始数据的哈希（用于判断是否需要重新分析）
func (s *Storage) ComputeDataHash(symbol string) (string, error) {
	stockDir := filepath.Join(s.dataDir, "data", symbol)
	files := []string{"balance_sheet.json", "income_statement.json", "cash_flow.json", "profile.json", "quote.json", "sentiment.json"}
	h := sha256.New()
	for _, name := range files {
		path := filepath.Join(stockDir, name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		h.Write([]byte(name))
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputeComparablesHash 计算可比公司列表的哈希
func (s *Storage) ComputeComparablesHash(symbol string) (string, error) {
	comps, err := s.GetComparables(symbol)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write([]byte(symbol))
	for _, c := range comps {
		h.Write([]byte(c))
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// cleanupOldReports 清理旧报告文件，保留 latest.md
func (s *Storage) cleanupOldReports(symbol string) error {
	dir := filepath.Join(s.dataDir, "reports", symbol)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".md") && name != "latest.md" {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

// SaveStockProfile 保存股票基本资料缓存
func (s *Storage) SaveStockProfile(symbol string, profile *StockProfile) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "profile.json")
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockProfile 读取股票基本资料缓存
func (s *Storage) LoadStockProfile(symbol string) (*StockProfile, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "profile.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var profile StockProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}
	return &profile, nil
}

// SaveStockConcepts 保存股票概念与风口缓存
func (s *Storage) SaveStockConcepts(symbol string, concepts *downloader.StockConcepts) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "concepts.json")
	data, err := json.MarshalIndent(concepts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockConcepts 读取股票概念与风口缓存
func (s *Storage) LoadStockConcepts(symbol string) (*downloader.StockConcepts, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "concepts.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var concepts downloader.StockConcepts
	if err := json.Unmarshal(data, &concepts); err != nil {
		return nil, err
	}
	return &concepts, nil
}

// SaveStockQuote 保存股票实时行情缓存
func (s *Storage) SaveStockQuote(symbol string, quote *downloader.StockQuote) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "quote.json")
	data, err := json.MarshalIndent(quote, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockQuote 读取股票实时行情缓存
func (s *Storage) LoadStockQuote(symbol string) (*downloader.StockQuote, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "quote.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var quote downloader.StockQuote
	if err := json.Unmarshal(data, &quote); err != nil {
		return nil, err
	}
	return &quote, nil
}

// SaveStockKlines 保存股票K线数据缓存。period 为空时默认用 "daily"。
func (s *Storage) SaveStockKlines(symbol string, period string, klines []downloader.KlineData) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if period == "" {
		period = "daily"
	}
	filename := "klines.json"
	if period != "daily" {
		filename = fmt.Sprintf("klines_%s.json", period)
	}
	path := filepath.Join(dir, filename)
	data, err := json.MarshalIndent(klines, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockKlines 读取股票K线数据缓存。period 为空时默认用 "daily"。
func (s *Storage) LoadStockKlines(symbol string, period string) ([]downloader.KlineData, error) {
	if period == "" {
		period = "daily"
	}
	filename := "klines.json"
	if period != "daily" {
		filename = fmt.Sprintf("klines_%s.json", period)
	}
	path := filepath.Join(s.dataDir, "data", symbol, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var klines []downloader.KlineData
	if err := json.Unmarshal(data, &klines); err != nil {
		return nil, err
	}
	return klines, nil
}

// SaveStockSentiment 保存舆情情绪缓存
func (s *Storage) SaveStockSentiment(symbol string, sentiment *downloader.SentimentData) error {
	dir := filepath.Join(s.dataDir, "data", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "sentiment.json")
	data, err := json.MarshalIndent(sentiment, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadStockSentiment 读取舆情情绪缓存
func (s *Storage) LoadStockSentiment(symbol string) (*downloader.SentimentData, error) {
	path := filepath.Join(s.dataDir, "data", symbol, "sentiment.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sentiment downloader.SentimentData
	if err := json.Unmarshal(data, &sentiment); err != nil {
		return nil, err
	}
	return &sentiment, nil
}

// EnsureComparableDataDir 确保可比公司数据目录存在
func (s *Storage) EnsureComparableDataDir(symbol string) (string, error) {
	dir := filepath.Join(s.dataDir, "comparables", symbol)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	return dir, nil
}

// ComparablesConfigPath 返回可比公司配置文件路径
func (s *Storage) ComparablesConfigPath() string {
	return filepath.Join(s.dataDir, "comparables.json")
}

// LoadComparablesConfig 加载可比公司配置
func (s *Storage) LoadComparablesConfig() (map[string][]string, error) {
	path := s.ComparablesConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string][]string), nil
		}
		return nil, err
	}
	var config map[string][]string
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

// SaveComparablesConfig 保存可比公司配置
func (s *Storage) SaveComparablesConfig(config map[string][]string) error {
	path := s.ComparablesConfigPath()
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetComparables 获取某只股票的可比公司列表
func (s *Storage) GetComparables(symbol string) ([]string, error) {
	config, err := s.LoadComparablesConfig()
	if err != nil {
		return nil, err
	}
	return config[symbol], nil
}

// AddComparable 添加可比公司
func (s *Storage) AddComparable(symbol, comparable string) error {
	config, err := s.LoadComparablesConfig()
	if err != nil {
		return err
	}
	list := config[symbol]
	for _, c := range list {
		if c == comparable {
			return nil // 已存在
		}
	}
	if len(list) >= 7 {
		return fmt.Errorf("可比公司最多7家")
	}
	config[symbol] = append(list, comparable)
	return s.SaveComparablesConfig(config)
}

// RemoveComparable 移除可比公司
func (s *Storage) RemoveComparable(symbol, comparable string) error {
	config, err := s.LoadComparablesConfig()
	if err != nil {
		return err
	}
	list := config[symbol]
	filtered := make([]string, 0, len(list))
	for _, c := range list {
		if c != comparable {
			filtered = append(filtered, c)
		}
	}
	config[symbol] = filtered
	return s.SaveComparablesConfig(config)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func mergeYears(yearsList ...[]string) []string {
	seen := make(map[string]struct{})
	for _, list := range yearsList {
		for _, y := range list {
			seen[y] = struct{}{}
		}
	}
	var result []string
	for y := range seen {
		result = append(result, y)
	}
	// 尝试按字符串排序，通常时间格式字符串排序有效
	sort.Strings(result)
	// 反转成从新到旧
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return result
}

// IndustryBaselinePath 返回行业基准文件路径
func (s *Storage) IndustryBaselinePath() string {
	return filepath.Join(s.dataDir, "industry_baseline.json")
}

// LoadIndustryBaselines 加载行业基准数据
func (s *Storage) LoadIndustryBaselines() (map[string]*analyzer.IndustryBaseline, error) {
	path := s.IndustryBaselinePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var baselines map[string]*analyzer.IndustryBaseline
	if err := json.Unmarshal(data, &baselines); err != nil {
		return nil, err
	}
	return baselines, nil
}

// SaveIndustryBaselines 保存行业基准数据
func (s *Storage) SaveIndustryBaselines(baselines map[string]*analyzer.IndustryBaseline) error {
	path := s.IndustryBaselinePath()
	data, err := json.MarshalIndent(baselines, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ActivityCachePath 返回某只股票的活跃度缓存文件路径
func (s *Storage) ActivityCachePath(symbol string) string {
	return filepath.Join(s.DataDir(), "data", symbol, "activity.json")
}

// SaveActivityCache 保存活跃度缓存
func (s *Storage) SaveActivityCache(symbol string, data *analyzer.ActivityData) error {
	path := s.ActivityCachePath(symbol)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// LoadActivityCache 加载活跃度缓存（同时校验时效，默认当天有效）
func (s *Storage) LoadActivityCache(symbol string) (*analyzer.ActivityData, error) {
	path := s.ActivityCachePath(symbol)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > 24*time.Hour {
		return nil, fmt.Errorf("缓存过期")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data analyzer.ActivityData
	if err := json.Unmarshal(b, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// RIMCachePath 返回某只股票的RIM数据缓存文件路径
func (s *Storage) RIMCachePath(symbol string) string {
	return filepath.Join(s.DataDir(), "data", symbol, "rim_cache.json")
}

// rimCacheWrapper 带时间戳的RIM缓存包装器
type rimCacheWrapper struct {
	Timestamp time.Time                   `json:"timestamp"`
	Data      *downloader.RIMExternalData `json:"data"`
}

// SaveRIMCache 保存RIM外部数据缓存
func (s *Storage) SaveRIMCache(symbol string, data *downloader.RIMExternalData) error {
	path := s.RIMCachePath(symbol)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	wrapper := rimCacheWrapper{
		Timestamp: time.Now(),
		Data:      data,
	}
	b, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// LoadRIMCache 加载RIM外部数据缓存（默认12小时有效）
func (s *Storage) LoadRIMCache(symbol string) (*downloader.RIMExternalData, error) {
	path := s.RIMCachePath(symbol)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if time.Since(info.ModTime()) > 12*time.Hour {
		return nil, fmt.Errorf("缓存过期")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wrapper rimCacheWrapper
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return nil, err
	}
	return wrapper.Data, nil
}

// SnapshotInfo 快照元数据
type SnapshotInfo struct {
	Timestamp string `json:"timestamp"`
	DateTime  string `json:"date_time"`
}

// SaveSnapshot 保存分析报告快照（保留最近10份历史）
func (s *Storage) SaveSnapshot(symbol string, report *analyzer.AnalysisReport) error {
	historyDir := filepath.Join(s.dataDir, "snapshots", symbol)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return err
	}
	timestamp := time.Now().Format("20060102_150405")
	path := filepath.Join(historyDir, timestamp+".json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}
	// 同时更新 latest.json 快捷访问
	latestPath := filepath.Join(historyDir, "latest.json")
	if err := os.WriteFile(latestPath, data, 0600); err != nil {
		return err
	}
	return s.cleanupOldSnapshots(symbol)
}

// cleanupOldSnapshots 清理旧快照，保留最近 10 份
func (s *Storage) cleanupOldSnapshots(symbol string) error {
	historyDir := filepath.Join(s.dataDir, "snapshots", symbol)
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var files []os.DirEntry
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "latest.json" {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".json") {
			files = append(files, entry)
		}
	}
	if len(files) <= 10 {
		return nil
	}
	// 按文件名排序（时间戳格式 20060102_150405，字典序即时间序）
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})
	// 删除最旧的
	for i := 0; i < len(files)-10; i++ {
		if err := os.Remove(filepath.Join(historyDir, files[i].Name())); err != nil {
			return err
		}
	}
	return nil
}

// ListSnapshotHistory 列出某只股票的分析快照历史
func (s *Storage) ListSnapshotHistory(symbol string) ([]SnapshotInfo, error) {
	historyDir := filepath.Join(s.dataDir, "snapshots", symbol)
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var infos []SnapshotInfo
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "latest.json" {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		// 解析时间戳 20060102_150405
		ts := strings.TrimSuffix(name, ".json")
		if t, err := time.ParseInLocation("20060102_150405", ts, time.Local); err == nil {
			infos = append(infos, SnapshotInfo{
				Timestamp: ts,
				DateTime:  t.Format("2006-01-02 15:04:05"),
			})
		}
	}
	// 降序排列（最新的在前）
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Timestamp > infos[j].Timestamp
	})
	return infos, nil
}

// LoadSnapshotByTime 按时间戳加载指定快照
func (s *Storage) LoadSnapshotByTime(symbol string, timestamp string) (*analyzer.AnalysisReport, error) {
	path := filepath.Join(s.dataDir, "snapshots", symbol, timestamp+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var report analyzer.AnalysisReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("解析快照失败: %w", err)
	}
	return &report, nil
}

// LoadSnapshot 加载分析报告快照
func (s *Storage) LoadSnapshot(symbol string) (*analyzer.AnalysisReport, error) {
	// 兼容旧版单文件结构
	oldPath := filepath.Join(s.dataDir, "snapshots", symbol+".json")
	if data, err := os.ReadFile(oldPath); err == nil {
		var report analyzer.AnalysisReport
		if err := json.Unmarshal(data, &report); err == nil {
			// 老快照没有 GeneratedAt 字段，用文件修改时间兜底
			if report.GeneratedAt == "" {
				if fi, err := os.Stat(oldPath); err == nil {
					report.GeneratedAt = fi.ModTime().Format("2006-01-02 15:04")
				}
			}
			return &report, nil
		}
	}
	// 新版目录结构
	path := filepath.Join(s.dataDir, "snapshots", symbol, "latest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var report analyzer.AnalysisReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	// 老版本快照没有 GeneratedAt 字段，用快照文件修改时间兜底，保证「与上次分析对比」能显示时间
	if report.GeneratedAt == "" {
		if fi, err := os.Stat(path); err == nil {
			report.GeneratedAt = fi.ModTime().Format("2006-01-02 15:04")
		}
	}
	return &report, nil
}

// DeleteSnapshot 删除分析报告快照
func (s *Storage) DeleteSnapshot(symbol string) error {
	// 删除旧版单文件
	oldPath := filepath.Join(s.dataDir, "snapshots", symbol+".json")
	_ = os.Remove(oldPath)
	// 删除新版目录
	dir := filepath.Join(s.dataDir, "snapshots", symbol)
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ========== 热门概念/风口数据存储 ==========

// HotConceptDir 返回热点数据根目录
func (s *Storage) HotConceptDir() string {
	return filepath.Join(s.dataDir, "hot_concepts")
}

// HotConceptLatestPath 返回当日热点缓存路径
func (s *Storage) HotConceptLatestPath() string {
	return filepath.Join(s.HotConceptDir(), "latest.json")
}

// HotConceptHistoryPath 返回某日期历史热点路径
func (s *Storage) HotConceptHistoryPath(date string) string {
	return filepath.Join(s.HotConceptDir(), "history", date+".json")
}

// SaveHotConceptBoard 保存热点看板数据（同时归档历史）
func (s *Storage) SaveHotConceptBoard(board *downloader.HotConceptBoard) error {
	// 保存最新缓存
	latestPath := s.HotConceptLatestPath()
	if err := os.MkdirAll(filepath.Dir(latestPath), 0755); err != nil {
		return fmt.Errorf("创建热点缓存目录失败: %w", err)
	}
	data, err := json.MarshalIndent(board, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化热点数据失败: %w", err)
	}
	if err := os.WriteFile(latestPath, data, 0644); err != nil {
		return fmt.Errorf("写入热点缓存失败: %w", err)
	}

	// 归档到历史
	historyPath := s.HotConceptHistoryPath(board.Date)
	if err := os.MkdirAll(filepath.Dir(historyPath), 0755); err != nil {
		return fmt.Errorf("创建热点历史目录失败: %w", err)
	}
	if err := os.WriteFile(historyPath, data, 0644); err != nil {
		return fmt.Errorf("写入热点历史失败: %w", err)
	}

	return nil
}

// LoadHotConceptBoard 加载指定日期的热点看板数据
// date 为空字符串时读取最新缓存
func (s *Storage) LoadHotConceptBoard(date string) (*downloader.HotConceptBoard, error) {
	var path string
	if date == "" {
		path = s.HotConceptLatestPath()
	} else {
		path = s.HotConceptHistoryPath(date)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var board downloader.HotConceptBoard
	if err := json.Unmarshal(data, &board); err != nil {
		return nil, fmt.Errorf("解析热点数据失败: %w", err)
	}
	return &board, nil
}

// ListHotConceptHistory 列出可用的历史日期列表（降序）
func (s *Storage) ListHotConceptHistory(maxDays int) ([]string, error) {
	historyDir := filepath.Join(s.HotConceptDir(), "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

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
	if maxDays > 0 && len(dates) > maxDays {
		dates = dates[:maxDays]
	}
	return dates, nil
}

// CleanOldHotConceptHistory 清理超过 maxDays 天的历史数据
func (s *Storage) CleanOldHotConceptHistory(maxDays int) error {
	historyDir := filepath.Join(s.HotConceptDir(), "history")
	entries, err := os.ReadDir(historyDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().AddDate(0, 0, -maxDays)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		dateStr := strings.TrimSuffix(name, ".json")
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			_ = os.Remove(filepath.Join(historyDir, name))
		}
	}
	return nil
}

// ========== App 配置存储 ==========

// ProxyConfig 网络代理配置
type ProxyConfig struct {
	Enabled  bool   `json:"enabled"`  // 是否启用代理
	URL      string `json:"url"`      // 代理地址，如 http://127.0.0.1:7890 或 socks5://127.0.0.1:1080
	Username string `json:"username"` // 可选认证用户名
	Password string `json:"password"` // 可选认证密码
}

// AppConfig 应用级配置（自动更新等）
type AppConfig struct {
	AutoCheckUpdate bool        `json:"autoCheckUpdate"` // 默认 true
	LastCheckDate   string      `json:"lastCheckDate"`   // YYYY-MM-DD
	SkipVersion     string      `json:"skipVersion"`     // 用户选择"跳过此版本"
	Proxy           ProxyConfig `json:"proxy"`           // 网络代理配置
}

// AppConfigPath 返回 App 配置文件路径
func (s *Storage) AppConfigPath() string {
	return filepath.Join(s.dataDir, "app_config.json")
}

// LoadAppConfig 加载 App 配置
func (s *Storage) LoadAppConfig() (*AppConfig, error) {
	path := s.AppConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &AppConfig{AutoCheckUpdate: true}, nil
		}
		return nil, err
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 App 配置失败: %w", err)
	}
	return &cfg, nil
}

// SaveAppConfig 保存 App 配置
func (s *Storage) SaveAppConfig(cfg *AppConfig) error {
	path := s.AppConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 App 配置失败: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// ========== SFL 配置存储 ==========

// SFLConfig SFL 配置
type SFLConfig struct {
	Enabled         bool   `json:"enabled"`
	Token           string `json:"token"`
	Verified        bool   `json:"verified"`
	VerifiedAt      string `json:"verified_at"`
	UseForFinancial bool   `json:"use_for_financial"`
	UseForKline     bool   `json:"use_for_kline"`
	UseForQuote     bool   `json:"use_for_quote"`
	UseForMoneyflow bool   `json:"use_for_moneyflow"`
	MoneyflowDays   int    `json:"moneyflow_days"`
}

// SFLConfigPath 返回 SFL 配置文件路径
func (s *Storage) SFLConfigPath() string {
	return filepath.Join(s.dataDir, "tushare_config.json")
}

// LoadSFLConfig 加载SFL 配置
func (s *Storage) LoadSFLConfig() (*SFLConfig, error) {
	path := s.SFLConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SFLConfig{
				Enabled:         false,
				UseForFinancial: true,
				UseForKline:     true,
				UseForQuote:     true,
				UseForMoneyflow: true,
				MoneyflowDays:   3,
			}, nil
		}
		return nil, err
	}
	var cfg SFLConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析SFL 配置失败: %w", err)
	}
	if cfg.MoneyflowDays <= 0 {
		cfg.MoneyflowDays = 3
	}
	return &cfg, nil
}

// SaveSFLConfig 保存 SFL 配置
func (s *Storage) SaveSFLConfig(cfg *SFLConfig) error {
	path := s.SFLConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化SFL 配置失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ========== AI 投研配置存储 ==========

// AIConfigPath 返回 AI 投研配置文件路径
func (s *Storage) AIConfigPath() string {
	return filepath.Join(s.dataDir, "ai_config.json")
}

// LoadAIConfig 加载 AI 投研配置
func (s *Storage) LoadAIConfig() (*ai_researcher.AIConfig, error) {
	path := s.AIConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ai_researcher.DefaultAIConfig(), nil
		}
		return nil, err
	}
	var cfg ai_researcher.AIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析 AI 配置失败: %w", err)
	}
	cfg.Normalize()
	return &cfg, nil
}

// SaveAIConfig 保存 AI 投研配置
func (s *Storage) SaveAIConfig(cfg *ai_researcher.AIConfig) error {
	if cfg == nil {
		return fmt.Errorf("配置不能为空")
	}
	cfg.Normalize()
	path := s.AIConfigPath()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 AI 配置失败: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// AIResearchCachePath 返回某只股票的 AI 投研缓存路径
func (s *Storage) AIResearchCachePath(symbol string) string {
	return filepath.Join(s.dataDir, "data", symbol, "ai_research_cache.json")
}

// LoadAIResearchCache 加载 AI 投研缓存
func (s *Storage) LoadAIResearchCache(symbol string) (*ai_researcher.AIResearchReport, error) {
	path := s.AIResearchCachePath(symbol)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var report ai_researcher.AIResearchReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("解析 AI 投研缓存失败: %w", err)
	}
	return &report, nil
}

// SaveAIResearchCache 保存 AI 投研缓存
func (s *Storage) SaveAIResearchCache(symbol string, report *ai_researcher.AIResearchReport) error {
	if report == nil {
		return nil
	}
	dir, err := s.EnsureStockDataDir(symbol)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "ai_research_cache.json")
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化 AI 投研缓存失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// MarginDailyDir 返回每日全市场融资融券数据目录
func (s *Storage) MarginDailyDir() string {
	return filepath.Join(s.dataDir, "data", "margin", "daily")
}

// MarginDailyPath 返回指定日期的全市场融资融券缓存路径
func (s *Storage) MarginDailyPath(date string) string {
	date = strings.ReplaceAll(date, "-", "")
	return filepath.Join(s.MarginDailyDir(), date+".json")
}

// LoadMarginDaily 加载指定日期的全市场融资融券缓存
func (s *Storage) LoadMarginDaily(date string) ([]downloader.MarginData, error) {
	path := s.MarginDailyPath(date)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []downloader.MarginData
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("解析融资融券日缓存失败: %w", err)
	}
	return list, nil
}

// SaveMarginDaily 保存指定日期的全市场融资融券缓存
func (s *Storage) SaveMarginDaily(date string, list []downloader.MarginData) error {
	if len(list) == 0 {
		return nil
	}
	dir := s.MarginDailyDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建融资融券缓存目录失败: %w", err)
	}
	path := s.MarginDailyPath(date)
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化融资融券日缓存失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// MarginHistoryPath 返回个股融资融券历史缓存路径
func (s *Storage) MarginHistoryPath(symbol string) string {
	return filepath.Join(s.dataDir, "data", symbol, "margin_history.json")
}

// LoadMarginHistory 加载个股融资融券历史缓存
func (s *Storage) LoadMarginHistory(symbol string) ([]downloader.MarginData, error) {
	path := s.MarginHistoryPath(symbol)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []downloader.MarginData
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("解析融资融券历史缓存失败: %w", err)
	}
	return list, nil
}

// SaveMarginHistory 保存个股融资融券历史缓存
func (s *Storage) SaveMarginHistory(symbol string, list []downloader.MarginData) error {
	if len(list) == 0 {
		return nil
	}
	dir, err := s.EnsureStockDataDir(symbol)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "margin_history.json")
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化融资融券历史缓存失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

// ========== 自选分组存储 ==========

// WatchlistGroupsPath 返回自选分组文件路径
func (s *Storage) WatchlistGroupsPath() string {
	return filepath.Join(s.dataDir, "watchlist_groups.json")
}

// LoadWatchlistGroups 加载自选分组，文件不存在时返回空切片
func (s *Storage) LoadWatchlistGroups() ([]StockGroup, error) {
	path := s.WatchlistGroupsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []StockGroup{}, nil
		}
		return nil, err
	}
	var groups []StockGroup
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}

// SaveWatchlistGroups 保存自选分组（整组替换）
func (s *Storage) SaveWatchlistGroups(groups []StockGroup) error {
	path := s.WatchlistGroupsPath()
	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}
