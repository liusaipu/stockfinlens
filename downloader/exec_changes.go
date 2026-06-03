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

// ExecChanges 高管变动信息
type ExecChanges struct {
	ExecChangeCount  int      `json:"exec_change_count"`
	CFOChanged       bool     `json:"cfo_changed"`
	AuditHeadChanged bool     `json:"audit_head_changed"`
	History          []string `json:"history"`
	Error            string   `json:"error,omitempty"`
}

// FetchExecChanges 获取股票高管变动信息
func FetchExecChanges(symbol string) (*ExecChanges, error) {
	script := resolveExecChangesScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("高管变动查询脚本不存在: %s", script)
	}

	req := map[string]any{"symbol": symbol}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolveExecChangesPython()
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
			return nil, fmt.Errorf("高管变动查询失败: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp ExecChanges
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析高管变动查询结果失败: %w | raw: %s", err, string(out))
	}
	return &resp, nil
}

func resolveExecChangesScriptPath() string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "fetch_exec_changes.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", "fetch_exec_changes.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	return filepath.Join(base, "..", "scripts", "fetch_exec_changes.py")
}

func resolveExecChangesPython() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}
