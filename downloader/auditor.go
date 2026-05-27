package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// AuditorChangeDetail 审计机构变更详情
type AuditorChangeDetail struct {
	Date                 string `json:"date"`
	OldAuditor           string `json:"old_auditor"`
	NewAuditor           string `json:"new_auditor"`
	Reason               string `json:"reason"`
	IsBeforeAnnualReport bool   `json:"is_before_annual_report"`
	AnnualReportDeadline string `json:"annual_report_deadline"`
	RawTitle             string `json:"raw_title"`
	IsPolicyCompliance   bool   `json:"is_policy_compliance"` // 是否为政策合规更换（如国企8年强制轮换）
	IsAbnormal           bool   `json:"is_abnormal"`          // 是否为异常更换（需警惕）
	IsPassiveChange      bool   `json:"is_passive_change"`    // 是否为被动更换（原事务所被处罚/禁入等，非公司自身问题）
	IsNormalRotation     bool   `json:"is_normal_rotation"`   // 是否为正常轮换（看似异常实则正常，如招标选聘、连续服务多年后更换）
}

// AuditOpinion 单年度审计意见
type AuditOpinion struct {
	Year             string `json:"year"`
	Opinion          string `json:"opinion"`
	Auditor          string `json:"auditor"`
	IsStandard       bool   `json:"is_standard"`
	NeedsReview      bool   `json:"needs_review"`
	AnnouncementDate string `json:"announcement_date"`
	RawTitle         string `json:"raw_title"`
	Error            string `json:"error,omitempty"`
}

// AuditorHistory 审计机构历史信息
type AuditorHistory struct {
	AuditorName     string                `json:"auditor_name"`
	AuditorChanged  bool                  `json:"auditor_changed"`
	History         []string              `json:"history"`
	ChangeDetails   []AuditorChangeDetail `json:"change_details"`
	AuditOpinions   []AuditOpinion        `json:"audit_opinions"`
	Error           string                `json:"error,omitempty"`
}

// FetchAuditorHistory 获取股票历年审计机构信息
func FetchAuditorHistory(symbol string) (*AuditorHistory, error) {
	script := resolveAuditorScriptPath()
	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("审计机构查询脚本不存在: %s", script)
	}

	req := map[string]any{"symbol": symbol}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	python := resolveAuditorPython()
	cmd := exec.Command(python, script)
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
			return nil, fmt.Errorf("审计机构查询失败: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp AuditorHistory
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析审计机构查询结果失败: %w | raw: %s", err, string(out))
	}
	return &resp, nil
}

func resolveAuditorScriptPath() string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "scripts", "fetch_auditor_history.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "scripts", "fetch_auditor_history.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	_, b, _, _ := runtime.Caller(0)
	base := filepath.Dir(b)
	return filepath.Join(base, "..", "scripts", "fetch_auditor_history.py")
}

func resolveAuditorPython() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}
