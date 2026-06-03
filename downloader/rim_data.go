package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// RIMExternalData Python脚本返回的原始数据
type RIMExternalData struct {
	Symbol           string             `json:"symbol"`
	EPSForecast      map[string]float64 `json:"eps_forecast"`
	EPSForecastError string             `json:"eps_forecast_error,omitempty"`
	Rf               float64            `json:"rf"`
	RfDate           string             `json:"rf_date,omitempty"`
	RfError          string             `json:"rf_error,omitempty"`
	Price            float64            `json:"price"`
	TotalShares      float64            `json:"total_shares"`
	MarketCap        float64            `json:"market_cap"`
	PB               float64            `json:"pb"`
	Beta             float64            `json:"beta"`
	RmRf             float64            `json:"rm_rf"`
	Error            string             `json:"error,omitempty"`
}

// findProjectRootByMarker 从 binary 所在目录向上查找项目根目录（通过指定标记文件）
func findProjectRootByMarker(start string, marker string) string {
	dir := start
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// fetchRIMScriptPath 返回 fetch_rim_data.py 绝对路径
func fetchRIMScriptPath() string {
	// Priority 1: Direct check in executable directory (for packaged Windows app)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "fetch_rim_data.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// macOS .app bundle: scripts 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", "fetch_rim_data.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "fetch_rim_data.py"))
	if root != "" {
		return filepath.Join(root, "scripts", "fetch_rim_data.py")
	}
	return filepath.Join(base, "..", "scripts", "fetch_rim_data.py")
}

// resolvePythonExecutable 优先使用项目 .venv 中的 Python
func resolvePythonExecutable() string {
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
		// macOS .app bundle: .venv 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		venvPython = filepath.Join(resourcesDir, ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
	}

	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "fetch_rim_data.py"))
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

// FetchRIMExternalData 调用 Python 脚本获取 RIM 外部数据
func FetchRIMExternalData(symbol string) (*RIMExternalData, error) {
	script := fetchRIMScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("fetch_rim_data.py not found: %s", script)
	}

	req := map[string]string{"symbol": symbol}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolvePythonExecutable()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, script)
	cmd.Env = append(os.Environ(), "TQDM_DISABLE=1", "PYTHONUNBUFFERED=1")

	// Windows: 隐藏 CMD 窗口
	setHideWindow(cmd)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		_, _ = stdin.Write(reqBytes)
		_ = stdin.Close()
	}()

	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("fetch rim data failed: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp RIMExternalData
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("parse rim data failed: %w | raw: %s", err, string(out))
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("fetch rim data error: %s", resp.Error)
	}
	return &resp, nil
}
