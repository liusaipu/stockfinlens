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

// LitigationInfo 诉讼/担保/资金占用信息
type LitigationInfo struct {
	LitigationCount      int      `json:"litigation_count"`
	HasGuarantee         bool     `json:"has_guarantee"`
	HasHighRiskGuarantee bool     `json:"has_high_risk_guarantee"`
	HasFundOccupation    bool     `json:"has_fund_occupation"`
	History              []string `json:"history"`
	Error                string   `json:"error,omitempty"`
}

// FetchLitigationInfo 获取股票诉讼、违规担保、资金占用信息
func FetchLitigationInfo(symbol string) (*LitigationInfo, error) {
	script := resolveLitigationScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("诉讼担保查询脚本不存在: %s", script)
	}

	req := map[string]any{"symbol": symbol}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolveLitigationPython()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, script)
	cmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")

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
			return nil, fmt.Errorf("诉讼担保查询失败: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp LitigationInfo
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析诉讼担保查询结果失败: %w | raw: %s", err, string(out))
	}
	return &resp, nil
}

func resolveLitigationScriptPath() string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "fetch_litigation.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", "fetch_litigation.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	return filepath.Join(base, "..", "scripts", "fetch_litigation.py")
}

func resolveLitigationPython() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}
