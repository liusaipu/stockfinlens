package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestReportPeriodName(t *testing.T) {
	cases := map[string]string{
		"2026-03-31": "2026年一季报",
		"2026-06-30": "2026年半年报",
		"2026-09-30": "2026年三季报",
		"2025-12-31": "2025年年报",
		"2026-02-28": "2026-02-28", // 非标准报告期原样返回
		"2026":       "2026",       // 长度不足原样返回
	}
	for date, want := range cases {
		if got := reportPeriodName(date); got != want {
			t.Errorf("reportPeriodName(%q) = %q, want %q", date, got, want)
		}
	}
}

func TestCheckNewReportUnsupportedMarket(t *testing.T) {
	app := &App{ctx: context.Background(), storage: &Storage{dataDir: t.TempDir()}}
	info, err := app.CheckNewReport("00700.HK")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Supported {
		t.Error("港股不应支持新财报检测")
	}
	if info.HasNew {
		t.Error("不支持的市场不应报告新财报")
	}
}

func TestCheckNewReportInvalidSymbol(t *testing.T) {
	app := &App{ctx: context.Background(), storage: &Storage{dataDir: t.TempDir()}}
	if _, err := app.CheckNewReport("603501"); err == nil {
		t.Error("无效代码格式应返回错误")
	}
}

func TestCheckNewReportNoCache(t *testing.T) {
	app := &App{ctx: context.Background(), storage: &Storage{dataDir: t.TempDir()}}
	info, err := app.CheckNewReport("603501.SH")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.Supported {
		t.Error("A 股应支持新财报检测")
	}
	if info.HasCache || info.HasNew {
		t.Error("无缓存时不应触发新财报提醒，也不应访问网络")
	}
}

func TestCheckNewReportCachedLatest(t *testing.T) {
	dataDir := t.TempDir()
	s := &Storage{dataDir: dataDir}
	symbol := "603501.SH"
	stockDir, err := s.EnsureStockDataDir(symbol)
	if err != nil {
		t.Fatal(err)
	}
	// 写入带两个报告期的利润表缓存，验证取最大期间键
	stmt := `{"归母净利润":{"2025-06-30":1.0,"2025-09-30":2.0},"营业收入":{"2025-03-31":3.0}}`
	if err := os.WriteFile(filepath.Join(stockDir, "income_statement.json"), []byte(stmt), 0644); err != nil {
		t.Fatal(err)
	}

	app := &App{ctx: context.Background(), storage: s}
	// 有缓存后会继续访问网络查询线上报告期；此处网络必然不可达或返回结果，
	// 只验证缓存解析部分：要么报错（网络失败），要么缓存字段解析正确。
	info, err := app.CheckNewReport(symbol)
	if err != nil {
		t.Logf("网络查询失败（可接受）: %v", err)
		return
	}
	if !info.HasCache {
		t.Error("应识别到本地缓存")
	}
	if info.CachedLatest != "2025-09-30" {
		t.Errorf("CachedLatest = %q, want %q", info.CachedLatest, "2025-09-30")
	}
	if info.CachedPeriodName != "2025年三季报" {
		t.Errorf("CachedPeriodName = %q, want %q", info.CachedPeriodName, "2025年三季报")
	}
}
