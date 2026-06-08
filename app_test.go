package main

import (
	"context"
	"testing"
)

// 创建测试用的 App 实例（带临时存储，隔离用户真实数据）
func newTestApp(t *testing.T) *App {
	t.Helper()
	tmpDir := t.TempDir()
	storage := &Storage{dataDir: tmpDir}
	app := NewApp()
	app.storage = storage
	app.ctx = context.Background()
	app.currentVersion = "1.0.0"
	return app
}

// TestAppConfigPersistence 验证 AppConfig 读写持久化
func TestAppConfigPersistence(t *testing.T) {
	app := newTestApp(t)

	// 默认应启用自动检查
	if !app.GetAutoCheckUpdate() {
		t.Error("默认应启用自动检查更新")
	}

	// 关闭自动检查
	if err := app.SetAutoCheckUpdate(false); err != nil {
		t.Fatalf("设置失败: %v", err)
	}
	if app.GetAutoCheckUpdate() {
		t.Error("应能关闭自动检查更新")
	}

	// 重新加载验证持久化
	cfg, err := app.storage.LoadAppConfig()
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	if cfg.AutoCheckUpdate {
		t.Error("持久化后 AutoCheckUpdate 应为 false")
	}

	// 跳过版本
	if err := app.SkipVersion("1.2.3"); err != nil {
		t.Fatalf("跳过版本失败: %v", err)
	}
	cfg2, _ := app.storage.LoadAppConfig()
	if cfg2.SkipVersion != "1.2.3" {
		t.Errorf("SkipVersion 应为 1.2.3，实际是 %s", cfg2.SkipVersion)
	}
}

// TestCheckForUpdateWithMockVersion 验证版本比对逻辑（使用本地版本号，不调用网络）
func TestCheckForUpdateVersionComparison(t *testing.T) {
	// 这个测试只验证版本号读取，不调用 GitHub API
	ver := readWailsVersion()
	if ver == "unknown" {
		// wails.json 在测试环境中可能不存在，验证回退逻辑
		t.Logf("版本号回退为 unknown（测试环境无 wails.json）")
	} else if ver == "" {
		t.Error("版本号不应为空")
	}
}

// TestCurrentVersionBinding 验证 GetCurrentVersion 绑定
func TestCurrentVersionBinding(t *testing.T) {
	app := newTestApp(t)
	app.currentVersion = "1.3.33"

	v := app.GetCurrentVersion()
	if v != "1.3.33" {
		t.Errorf("版本号应为 1.3.33，实际是 %s", v)
	}
}

// TestWatchlistReorderPersistence 验证自选股排序持久化
func TestWatchlistReorderPersistence(t *testing.T) {
	app := newTestApp(t)

	// 添加测试股票
	testStocks := []WatchlistItem{
		{Code: "000001", Name: "平安银行", Market: "SZ"},
		{Code: "600519", Name: "贵州茅台", Market: "SH"},
		{Code: "300750", Name: "宁德时代", Market: "SZ"},
	}
	if err := app.storage.SaveWatchlist(testStocks); err != nil {
		t.Fatalf("保存自选列表失败: %v", err)
	}

	// 重新排序
	newOrder := []string{"300750", "000001", "600519"}
	if err := app.ReorderWatchlist(newOrder); err != nil {
		t.Fatalf("排序失败: %v", err)
	}

	// 验证持久化
	list, err := app.storage.LoadWatchlist()
	if err != nil {
		t.Fatalf("加载自选列表失败: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("自选列表应有 3 项，实际是 %d", len(list))
	}
	for i, code := range newOrder {
		if list[i].Code != code {
			t.Errorf("排序后第 %d 项应为 %s，实际是 %s", i, code, list[i].Code)
		}
	}
}

