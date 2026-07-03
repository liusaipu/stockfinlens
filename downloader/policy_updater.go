package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// PolicyUpdateResult Python脚本返回的更新结果
type PolicyUpdateResult struct {
	Success               bool     `json:"success"`
	Path                  string   `json:"path"`
	AddedIndustryKeywords int      `json:"added_industry_keywords"`
	AddedConceptKeywords  int      `json:"added_concept_keywords"`
	TotalIndustries       int      `json:"total_industries"`
	TotalConcepts         int      `json:"total_concepts"`
	Errors                []string `json:"errors"`
}

// updatePolicyScriptPath 返回 update_policy_library.py 绝对路径
func updatePolicyScriptPath() string {
	// Priority 1: Direct check in executable directory (for packaged Windows app)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "update_policy_library.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// macOS .app bundle: scripts 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", "update_policy_library.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	root := findProjectRootByMarker(base, filepath.Join("scripts", "update_policy_library.py"))
	if root != "" {
		return filepath.Join(root, "scripts", "update_policy_library.py")
	}
	return filepath.Join(base, "..", "scripts", "update_policy_library.py")
}

// UpdatePolicyLibrary 调用 Python 脚本更新政策库 JSON
func UpdatePolicyLibrary(dataDir string) (*PolicyUpdateResult, error) {
	script := updatePolicyScriptPath()
	python := resolvePythonExecutable()
	cmd := exec.Command(python, script, dataDir)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// Windows: 隐藏 CMD 窗口
	setHideWindow(cmd)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("政策库更新脚本执行失败: %v, stderr: %s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("政策库更新脚本执行失败: %v", err)
	}
	var result PolicyUpdateResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("解析政策库更新结果失败: %v, raw: %s", err, string(output))
	}
	if !result.Success {
		return &result, fmt.Errorf("政策库更新未成功")
	}
	return &result, nil
}
