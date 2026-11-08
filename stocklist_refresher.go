package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// StockListRefreshResult 股票列表刷新结果
type StockListRefreshResult struct {
	Success     bool   `json:"success"`
	Total       int    `json:"total"`
	Added       int    `json:"added"`
	Removed     int    `json:"removed"`
	RefreshedAt string `json:"refreshedAt"`
	Error       string `json:"error,omitempty"`
}

// stockListScriptPath 返回 fetch_stock_list.py 绝对路径
// 4 级回退：exe 同级 scripts → .app Resources/scripts → 项目根 scripts（开发模式）→ runtime.Caller
func stockListScriptPath() string {
	const scriptName = "fetch_stock_list.py"
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", scriptName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// macOS .app bundle: scripts 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", scriptName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// 开发模式：exe 位于项目根（go run / wails dev）
		p = filepath.Join(exeDir, "scripts", scriptName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// runtime.Caller 回退（测试等场景）
	_, b, _, ok := runtime.Caller(0)
	if ok {
		root := filepath.Dir(filepath.Dir(b))
		p := filepath.Join(root, "scripts", scriptName)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// userStockListPath 用户数据目录中的股票列表（运行时刷新产物，优先级高于内置库）
func userStockListPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "stock-analyzer", "stocks.json")
}

// runStockListFetch 调用 Python 脚本获取最新股票列表
func runStockListFetch() ([]StockInfo, error) {
	script := stockListScriptPath()
	if script == "" {
		return nil, fmt.Errorf("未找到 fetch_stock_list.py 脚本")
	}
	python := findPythonExecutable()
	if python == "" {
		return nil, fmt.Errorf("未找到 Python 可执行文件，请先安装 Python 3.10+")
	}

	cmd := exec.Command(python, script)
	cmd.Env = append(os.Environ(),
		"PYTHONIOENCODING=utf-8",
		"PYTHONUNBUFFERED=1",
		"TQDM_DISABLE=1",
	)
	// Windows: 隐藏 CMD 窗口
	setHideWindow(cmd)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("股票列表刷新脚本执行失败: %v, stderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("股票列表刷新脚本执行失败: %v", err)
	}

	var result struct {
		Success bool        `json:"success"`
		Error   string      `json:"error"`
		Stocks  []StockInfo `json:"stocks"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析股票列表刷新结果失败: %v, raw: %.200s", err, string(output))
	}
	if !result.Success {
		return nil, fmt.Errorf("股票列表刷新失败: %s", result.Error)
	}
	if len(result.Stocks) < 4000 {
		return nil, fmt.Errorf("刷新得到的股票数量异常: %d", len(result.Stocks))
	}
	return result.Stocks, nil
}

// RefreshStockList 在线刷新股票代码库：
// 1. 调用 Python 脚本拉取交易所最新名单
// 2. 写入用户数据目录 stocks.json（下次启动优先加载）
// 3. 热更新内存中的 a.stocks
func (a *App) RefreshStockList() *StockListRefreshResult {
	result := &StockListRefreshResult{RefreshedAt: time.Now().Format("2006-01-02 15:04:05")}

	stocks, err := runStockListFetch()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	// 与当前列表对比，统计新增/移除
	current := a.getStocks()
	oldSet := make(map[string]bool, len(current))
	for _, s := range current {
		oldSet[s.Code] = true
	}

	newSet := make(map[string]bool, len(stocks))
	for _, s := range stocks {
		newSet[s.Code] = true
	}
	for code := range newSet {
		if !oldSet[code] {
			result.Added++
		}
	}
	for code := range oldSet {
		if !newSet[code] {
			result.Removed++
		}
	}
	result.Total = len(stocks)

	// 写入用户数据目录（临时文件 + 原子重命名，避免写坏旧文件）
	if path := userStockListPath(); path != "" {
		data, err := json.MarshalIndent(stocks, "", "  ")
		if err != nil {
			result.Error = fmt.Sprintf("序列化股票列表失败: %v", err)
			return result
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, append(data, '\n'), 0644); err != nil {
			result.Error = fmt.Sprintf("写入股票列表失败: %v", err)
			return result
		}
		if err := os.Rename(tmp, path); err != nil {
			result.Error = fmt.Sprintf("替换股票列表失败: %v", err)
			return result
		}
	}

	// 热更新内存代码库
	a.stocksMu.Lock()
	a.stocks = stocks
	a.stocksMu.Unlock()

	result.Success = true
	return result
}
