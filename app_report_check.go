package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/liusaipu/stockfinlens/downloader"
)

// NewReportInfo 新财报检测结果
type NewReportInfo struct {
	Supported        bool   `json:"supported"`        // 是否支持检测（仅 A 股）
	HasCache         bool   `json:"hasCache"`         // 本地是否有缓存财报
	HasNew           bool   `json:"hasNew"`           // 线上是否有比缓存更新的财报
	CachedLatest     string `json:"cachedLatest"`     // 本地缓存的最新报告期（yyyy-mm-dd）
	CachedPeriodName string `json:"cachedPeriodName"` // 本地缓存最新报告期名称（如 2025年三季报）
	LatestReportDate string `json:"latestReportDate"` // 线上最新报告期（yyyy-mm-dd）
	LatestPeriodName string `json:"latestPeriodName"` // 线上最新报告期名称
}

// reportPeriodName 将报告期（yyyy-mm-dd）转换为中文名称，如 2025-09-30 → 2025年三季报
func reportPeriodName(date string) string {
	if len(date) < 10 {
		return date
	}
	year := date[:4]
	switch date[5:10] {
	case "03-31":
		return year + "年一季报"
	case "06-30":
		return year + "年半年报"
	case "09-30":
		return year + "年三季报"
	case "12-31":
		return year + "年年报"
	default:
		return date
	}
}

// CheckNewReport 检测选中股票是否有新发布的季报/年报（对比本地缓存的最新报告期）。
// 仅在本地已有缓存财报时才有意义；检测失败（如网络问题）返回错误，前端静默忽略。
func (a *App) CheckNewReport(symbol string) (*NewReportInfo, error) {
	info := &NewReportInfo{}
	if a.storage == nil {
		return info, fmt.Errorf("存储未初始化")
	}

	// symbol 格式如 603501.SH；港股（.HK）暂不支持检测
	parts := strings.Split(symbol, ".")
	if len(parts) != 2 {
		return info, fmt.Errorf("无效的股票代码格式: %s", symbol)
	}
	code := parts[0]
	market := strings.ToUpper(parts[1])
	if market != "SH" && market != "SZ" {
		return info, nil
	}
	info.Supported = true

	// 本地缓存的最新报告期：取利润表所有期间键的最大值
	cachedLatest := ""
	isPath := filepath.Join(a.storage.DataDir(), "data", symbol, "income_statement.json")
	if data, err := os.ReadFile(isPath); err == nil {
		var stmt map[string]map[string]float64
		if json.Unmarshal(data, &stmt) == nil {
			for _, periods := range stmt {
				for period := range periods {
					if period > cachedLatest {
						cachedLatest = period
					}
				}
			}
		}
	}
	if cachedLatest == "" {
		// 本地没有缓存财报，无需提醒（用户本来就需要先下载）
		return info, nil
	}
	info.HasCache = true
	info.CachedLatest = cachedLatest
	info.CachedPeriodName = reportPeriodName(cachedLatest)

	// 线上最新报告期（东方财富数据中心，轻量查询）。
	// 网络失败不视为错误：仍返回缓存信息，前端只显示缓存日期。
	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()
	latest, err := downloader.FetchLatestReportDate(ctx, code)
	if err != nil {
		fmt.Printf("[CheckNewReport] %s 查询最新报告期失败: %v\n", symbol, err)
		return info, nil
	}
	info.LatestReportDate = latest
	info.LatestPeriodName = reportPeriodName(latest)
	// yyyy-mm-dd 格式可直接按字符串比较
	info.HasNew = latest > cachedLatest
	return info, nil
}
