package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadStockJSONPrefersUserDir 验证股票列表读取优先级：用户数据目录 > 本地 data/ > embed
func TestReadStockJSONPrefersUserDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	userDir := filepath.Join(tmp, ".config", "stock-analyzer")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatal(err)
	}
	userData := []byte(`[{"code":"000001.USER","name":"用户目录股票","market":"SH"}]`)
	if err := os.WriteFile(filepath.Join(userDir, "stocks.json"), userData, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := readStockJSON()
	if err != nil {
		t.Fatalf("readStockJSON() error = %v", err)
	}
	if string(got) != string(userData) {
		t.Fatalf("应优先读取用户数据目录 stocks.json, got %s", string(got))
	}

	// 删除用户目录文件后应回退到本地 data/stocks.json（开发模式存在该文件）
	if err := os.Remove(filepath.Join(userDir, "stocks.json")); err != nil {
		t.Fatal(err)
	}
	got, err = readStockJSON()
	if err != nil {
		t.Fatalf("readStockJSON() error = %v", err)
	}
	local, err := os.ReadFile(filepath.Join("data", "stocks.json"))
	if err != nil {
		t.Skip("本地 data/stocks.json 不存在，跳过回退断言")
	}
	if string(got) != string(local) {
		t.Fatalf("应回退读取本地 data/stocks.json")
	}
}
