package downloader

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

// RiskCrawlerData 非财务风险爬虫结果
type RiskCrawlerData struct {
	PledgeRatio      *float64 `json:"pledge_ratio"`
	InquiryCount1Y   *int     `json:"inquiry_count_1y"`
	ReductionCount1Y *int     `json:"reduction_count_1y"`
	Error            string   `json:"error"`
}

func resolveRiskCrawlerPython() string {
	// Priority 1: Direct check in executable directory (for packaged Windows app)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		venvPython := filepath.Join(exeDir, ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
		venvPythonWin := filepath.Join(exeDir, ".venv", "Scripts", "python.exe")
		if _, err := os.Stat(venvPythonWin); err == nil {
			return venvPythonWin
		}
	}

	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("ml_models", "risk_crawler.py"))
	if root != "" {
		// Windows: .venv\Scripts\python.exe
		// Unix: .venv/bin/python3
		venvPython := filepath.Join(root, ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
		venvPythonWin := filepath.Join(root, ".venv", "Scripts", "python.exe")
		if _, err := os.Stat(venvPythonWin); err == nil {
			return venvPythonWin
		}
	}
	// Windows fallback
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

func riskCrawlerScriptPath() string {
	// Priority 1: Direct check in executable directory (for packaged Windows app)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "ml_models", "risk_crawler.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// macOS .app bundle: ml_models 在 .app 同级目录
		contentsDir := filepath.Dir(exeDir)
		if filepath.Base(contentsDir) == "Contents" {
			appDir := filepath.Dir(contentsDir)
			if strings.HasSuffix(filepath.Base(appDir), ".app") {
				parent := filepath.Dir(appDir)
				p = filepath.Join(parent, "ml_models", "risk_crawler.py")
				if _, err := os.Stat(p); err == nil {
					return p
				}
			}
		}
		// macOS .app bundle: ml_models 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "ml_models", "risk_crawler.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("ml_models", "risk_crawler.py"))
	if root != "" {
		return filepath.Join(root, "ml_models", "risk_crawler.py")
	}
	return filepath.Join(base, "..", "ml_models", "risk_crawler.py")
}

// FetchRiskCrawlerData 调用 Python 爬虫获取非财务风险数据
func FetchRiskCrawlerData(symbol string) (*RiskCrawlerData, error) {
	script := riskCrawlerScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("爬虫脚本不存在: %s", script)
	}

	req := map[string]any{"symbol": symbol}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolveRiskCrawlerPython()
	cmd := exec.Command(python, script)
	cmd.Env = append(os.Environ(), "TQDM_DISABLE=1", "PYTHONUNBUFFERED=1")

	// Windows: 隐藏 CMD 窗口
	setHideWindow(cmd)
	// Unix: 创建新进程组，便于超时后整体 kill
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		_, _ = stdin.Write(reqBytes)
		_ = stdin.Close()
	}()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动爬虫失败: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case <-time.After(30 * time.Second):
		if cmd.Process != nil {
			if runtime.GOOS == "windows" {
				_ = cmd.Process.Kill()
			} else {
				// 负 PID 表示 kill 整个进程组
				_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			}
		}
		return nil, fmt.Errorf("爬虫执行超时 (>30s): %s", symbol)
	case err := <-done:
		if err != nil {
			return nil, fmt.Errorf("爬虫失败: %w | stderr: %s", err, stderr.String())
		}
	}

	out := stdout.Bytes()
	var resp RiskCrawlerData
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析爬虫结果失败: %w | raw: %s", err, string(out))
	}
	return &resp, nil
}
