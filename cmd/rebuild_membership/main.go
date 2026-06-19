// Command rebuild_membership 本地重建 _concept_membership.json
//
// 用途：当 downloader 的成分股 API 翻页逻辑修复后，重新构建反查表以获取真实的概念覆盖范围。
// 默认输出到 ~/.config/stock-analyzer/_concept_membership.json，覆盖现有文件。
//
// 用法：
//
//	go run ./cmd/rebuild_membership/
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/liusaipu/stockfinlens/downloader"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法获取用户主目录: %v\n", err)
		os.Exit(1)
	}
	dataDir := filepath.Join(home, ".config", "stock-analyzer")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建数据目录失败: %v\n", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	start := time.Now()
	fmt.Printf("开始重建反查表 → %s\n", filepath.Join(dataDir, "_concept_membership.json"))
	m, err := downloader.RefreshConceptMembership(ctx, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "构建失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("完成: 概念 %d, 行业 %d, 覆盖股票 %d, 耗时 %s\n",
		m.ConceptCount, m.IndustryCount, m.SymbolCount, time.Since(start).Round(time.Second))
}
