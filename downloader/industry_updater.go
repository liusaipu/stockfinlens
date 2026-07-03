package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// IndustryUpdateResult 行业数据库更新结果
type IndustryUpdateResult struct {
	Success         bool     `json:"success"`
	Path            string   `json:"path"`
	TotalIndustries int      `json:"total_industries"`
	UpdatedCount    int      `json:"updated_count"`
	SkippedCount    int      `json:"skipped_count"`
	Errors          []string `json:"errors,omitempty"`
	Error           string   `json:"error,omitempty"`
}

// updateIndustryScriptPath 返回 update_industry_database.py 绝对路径
func updateIndustryScriptPath() string {
	// Priority 1: Direct check in executable directory (for packaged Windows app)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "update_industry_database.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// macOS .app bundle: scripts 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", "update_industry_database.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "update_industry_database.py"))
	if root != "" {
		return filepath.Join(root, "scripts", "update_industry_database.py")
	}
	return filepath.Join(base, "..", "scripts", "update_industry_database.py")
}

// UpdateIndustryDatabase 调用 Python 脚本更新行业均值数据库
func UpdateIndustryDatabase(dataDir string) (*IndustryUpdateResult, error) {
	script := updateIndustryScriptPath()
	python := resolvePythonExecutable()
	cmd := exec.Command(python, script, dataDir)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// Windows: 隐藏 CMD 窗口
	setHideWindow(cmd)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("行业数据库更新脚本执行失败: %v, stderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("行业数据库更新脚本执行失败: %v", err)
	}
	var result IndustryUpdateResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析行业数据库更新结果失败: %v, raw: %s", err, string(output))
	}
	if !result.Success {
		return &result, fmt.Errorf("行业数据库更新未成功: %s", result.Error)
	}
	return &result, nil
}
