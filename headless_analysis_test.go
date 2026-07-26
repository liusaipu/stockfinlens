package main

import (
	"context"
	"os"
	"testing"

	"github.com/liusaipu/stockfinlens/analyzer"
)

// TestHeadlessAnalyze 一次性 headless 分析工具（不启动 Wails/托盘）。
// 用法: HEADLESS_ANALYZE=600531.SH go test . -run TestHeadlessAnalyze -v
func TestHeadlessAnalyze(t *testing.T) {
	symbol := os.Getenv("HEADLESS_ANALYZE")
	if symbol == "" {
		t.Skip("设置 HEADLESS_ANALYZE=股票代码 后运行")
	}

	a := NewApp()
	a.ctx = context.Background()

	storage, err := NewStorage()
	if err != nil {
		t.Fatalf("初始化存储失败: %v", err)
	}
	a.storage = storage

	if err := a.loadStockDB(); err != nil {
		t.Logf("加载股票代码库失败（继续）: %v", err)
	}
	a.reloadDataRouter()
	if err := analyzer.InitIndustryDatabase(storage.DataDir()); err != nil {
		t.Logf("初始化行业数据库失败（继续）: %v", err)
	}
	if err := analyzer.InitPolicyLibrary(storage.DataDir()); err != nil {
		t.Logf("初始化政策库失败（继续）: %v", err)
	}

	report, err := a.AnalyzeStock(symbol, false)
	if err != nil {
		t.Fatalf("分析失败: %v", err)
	}
	if report == nil {
		t.Fatal("分析结果为空")
	}
	t.Logf("分析完成: %s, 评级=%s, 年份=%v", symbol, report.OverallGrade, report.Years)
}
