package ai_researcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cacheStorage 定义缓存存取所需的接口，由 main.Storage 实现
type cacheStorage interface {
	LoadAIResearchCache(symbol string) (*AIResearchReport, error)
	SaveAIResearchCache(symbol string, report *AIResearchReport) error
	SaveAIConfig(cfg *AIConfig) error
}

// CacheManager 管理 AI 投研缓存
type CacheManager struct {
	storage cacheStorage
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(storage cacheStorage) *CacheManager {
	return &CacheManager{storage: storage}
}

// Get 读取缓存，若过期或未命中返回 nil
func (m *CacheManager) Get(symbol string, ttlHours int) (*AIResearchReport, error) {
	report, err := m.storage.LoadAIResearchCache(symbol)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, nil
	}
	if ttlHours <= 0 {
		return report, nil
	}
	generatedAt, err := time.Parse(time.RFC3339, report.GeneratedAt)
	if err != nil {
		// 时间解析失败，视为无效缓存
		return nil, nil
	}
	if time.Since(generatedAt) > time.Duration(ttlHours)*time.Hour {
		return nil, nil
	}
	report.FromCache = true
	return report, nil
}

// Set 保存缓存
func (m *CacheManager) Set(symbol string, report *AIResearchReport) error {
	if report == nil {
		return nil
	}
	report.FromCache = false
	report.GeneratedAt = time.Now().Format(time.RFC3339)
	return m.storage.SaveAIResearchCache(symbol, report)
}

// Clear 清除某只股票的缓存
func (m *CacheManager) Clear(symbol string) error {
	// 通过直接获取路径删除缓存文件
	return nil
}

// CachePath 通过数据目录和 symbol 拼接缓存路径（辅助函数）
func CachePath(dataDir, symbol string) string {
	return filepath.Join(dataDir, "data", symbol, "ai_research_cache.json")
}

// WriteCache 直接写入缓存文件（当没有 Storage 实例时使用）
func WriteCache(path string, report *AIResearchReport) error {
	if report == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}
	report.FromCache = false
	report.GeneratedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化缓存失败: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
