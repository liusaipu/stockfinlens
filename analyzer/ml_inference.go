package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// MLSentimentPrediction A 舆情+量价高频预警结果
type MLSentimentPrediction struct {
	MovementLabel string  `json:"movement_label"`
	MovementProb  float64 `json:"movement_prob"`
	AnomalyProb   float64 `json:"anomaly_prob"`
}

// MLFinancialPrediction 财务趋势预警结果
type MLFinancialPrediction struct {
	ROEDirection     string  `json:"roe_direction"`
	ROEProb          float64 `json:"roe_prob"`
	RevenueDirection string  `json:"revenue_direction"`
	RevenueProb      float64 `json:"revenue_prob"`
	MScoreDirection  string  `json:"mscore_direction"`
	MScoreProb       float64 `json:"mscore_prob"`
	HealthScore      float64 `json:"health_score"`
}

// MLDRiskPrediction Engine-D 风险预警结果
type MLDRiskPrediction struct {
	RiskLabel   int      `json:"risk_label"`
	RiskProb    float64  `json:"risk_prob"`
	RiskLevel   string   `json:"risk_level"`
	TopFactors  []string `json:"top_factors"`
	ModelLoaded bool     `json:"model_loaded"`
}

// findProjectRoot 从指定目录向上查找项目根目录（通过 ml_models/inference.py 标记）
func findProjectRoot(start string) string {
	dir := start
	for i := 0; i < 10; i++ {
		if _, err := os.Stat(filepath.Join(dir, "ml_models", "inference.py")); err == nil {
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

// appBundleParentDir 检测 macOS .app bundle 结构，返回 .app 的父目录
// exeDir 通常是 .../stockfinlens.app/Contents/MacOS/
func appBundleParentDir(exeDir string) string {
	// exeDir -> .../Contents/MacOS/
	contentsDir := filepath.Dir(exeDir)
	if filepath.Base(contentsDir) != "Contents" {
		return ""
	}
	appDir := filepath.Dir(contentsDir)
	if !strings.HasSuffix(filepath.Base(appDir), ".app") {
		return ""
	}
	return filepath.Dir(appDir)
}

// projectRootCandidates 返回可能的项目根目录候选列表
func projectRootCandidates() []string {
	seen := make(map[string]bool)
	var roots []string

	add := func(p string) {
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		roots = append(roots, p)
	}

	// 1. runtime.Caller(0) 路径
	if _, b, _, ok := runtime.Caller(0); ok {
		add(findProjectRoot(filepath.Dir(b)))
	}

	// 2. 当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		add(findProjectRoot(cwd))
	}

	// 3. 可执行文件所在目录
	if exe, err := os.Executable(); err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		exeDir := filepath.Dir(exe)
		add(findProjectRoot(exeDir))
		// 4. macOS .app bundle 同级目录
		if parent := appBundleParentDir(exeDir); parent != "" {
			add(findProjectRoot(parent))
		}
	}

	return roots
}

// mlInferenceScriptPath 返回推理脚本绝对路径
func mlInferenceScriptPath() string {
	// 直接检查可执行文件所在目录（Windows 打包版本用）
	if exe, err := os.Executable(); err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		exeDir := filepath.Dir(exe)
		p := filepath.Join(exeDir, "ml_models", "inference.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
		// macOS .app bundle: ml_models 在 .app 同级目录
		if parent := appBundleParentDir(exeDir); parent != "" {
			p = filepath.Join(parent, "ml_models", "inference.py")
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
		// macOS .app bundle: ml_models 在 Contents/Resources 内部
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		p = filepath.Join(resourcesDir, "ml_models", "inference.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	for _, root := range projectRootCandidates() {
		p := filepath.Join(root, "ml_models", "inference.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	// fallback
	_, b, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(b), "..", "ml_models", "inference.py")
}

func resolveMLPythonExecutable() string {
	for _, root := range projectRootCandidates() {
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

	// macOS .app bundle: 检查 .app 同级目录下的 .venv
	if exe, err := os.Executable(); err == nil {
		exe, _ = filepath.EvalSymlinks(exe)
		exeDir := filepath.Dir(exe)
		if parent := appBundleParentDir(exeDir); parent != "" {
			venvPython := filepath.Join(parent, ".venv", "bin", "python3")
			if _, err := os.Stat(venvPython); err == nil {
				return venvPython
			}
		}
		// macOS .app bundle: 检查 Contents/Resources 下的 .venv
		resourcesDir := filepath.Join(exeDir, "..", "Resources")
		venvPython := filepath.Join(resourcesDir, ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
	}

	// Windows: 尝试多个可能的 Python 路径
	if runtime.GOOS == "windows" {
		// 常见 Python 安装路径
		pythonPaths := []string{
			`C:\Python314\python.exe`,
			`C:\Python313\python.exe`,
			`C:\Python312\python.exe`,
			`C:\Python311\python.exe`,
			`C:\Python310\python.exe`,
			`C:\Python39\python.exe`,
			`C:\Program Files\Python314\python.exe`,
			`C:\Program Files\Python313\python.exe`,
			`C:\Program Files\Python312\python.exe`,
			`C:\Program Files\Python311\python.exe`,
			`C:\Program Files\Python310\python.exe`,
			`C:\Program Files\Python39\python.exe`,
		}

		for _, p := range pythonPaths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}

		// 尝试使用 where 命令查找
		if out, err := exec.Command("where", "python").Output(); err == nil {
			paths := strings.Split(strings.TrimSpace(string(out)), "\n")
			if len(paths) > 0 && paths[0] != "" {
				return strings.TrimSpace(paths[0])
			}
		}

		// 最后 fallback 到命令名
		candidates := []string{"python", "python3", "py"}
		for _, py := range candidates {
			if _, err := exec.LookPath(py); err == nil {
				return py
			}
		}
		return "python"
	}
	return "python3"
}

// callMLInference 调用 Python 推理脚本
func callMLInference(engine string, payload map[string]any) (map[string]any, error) {
	script := mlInferenceScriptPath()
	python := resolveMLPythonExecutable()

	fmt.Printf("[ML] Engine %s: python=%s, script=%s\n", engine, python, script)

	if _, err := os.Stat(script); os.IsNotExist(err) {
		return nil, fmt.Errorf("推理脚本不存在: %s", script)
	}

	// 检查 Python 是否可执行
	if runtime.GOOS == "windows" {
		// Windows 上检查 python 命令是否在 PATH 中
		if _, err := exec.LookPath(python); err != nil {
			// 尝试使用绝对路径查找
			if absPath, err := exec.LookPath("python.exe"); err == nil {
				fmt.Printf("[ML] Found python.exe at: %s\n", absPath)
				python = absPath
			} else if absPath, err := exec.LookPath("python3.exe"); err == nil {
				fmt.Printf("[ML] Found python3.exe at: %s\n", absPath)
				python = absPath
			} else if absPath, err := exec.LookPath("py.exe"); err == nil {
				fmt.Printf("[ML] Found py.exe at: %s\n", absPath)
				python = absPath
			}
		}
	}

	req := map[string]any{
		"engine":  engine,
		"payload": payload,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	fmt.Printf("[ML] Executing: %s %s\n", python, script)
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
			return nil, fmt.Errorf("推理失败: %s | stderr: %s", err, string(ee.Stderr))
		}
		return nil, err
	}

	var resp map[string]any
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("解析推理结果失败: %w | raw: %s", err, string(out))
	}
	if e, ok := resp["error"].(string); ok && e != "" {
		return nil, fmt.Errorf("推理错误: %s", e)
	}
	return resp, nil
}

// RunMLEngineA 运行引擎 A 推理
func RunMLEngineA(textSeq [][]float64, priceSeq [][]float64) (*MLSentimentPrediction, error) {
	resp, err := callMLInference("A", map[string]any{
		"text_seq":  textSeq,
		"price_seq": priceSeq,
	})
	if err != nil {
		return nil, err
	}
	result := &MLSentimentPrediction{}
	if d, ok := resp["direction"].(string); ok {
		result.MovementLabel = d
	}
	if p, ok := resp["direction_probs"].(map[string]any); ok {
		key := result.MovementLabel
		if prob, ok := p[key].(float64); ok {
			result.MovementProb = prob
		}
	}
	if a, ok := resp["abnormal_prob"].(float64); ok {
		result.AnomalyProb = a
	}
	return result, nil
}

// BuildMLSummary 基于 Engine A/B + 技术面 + 活跃度 + 舆情 + A-Score 融合生成 2-4 周综合预测
func BuildMLSummary(ml *MLPredictionData, technical *TechnicalData, activity *ActivityData, sentiment *SentimentData, ascore float64) *MLSummary {
	if ml == nil || (ml.Sentiment == nil && ml.Financial == nil) {
		return nil
	}

	a := ml.Sentiment
	b := ml.Financial

	// 1. 方向得分 (-3 ~ +3)
	dirScore := 0.0

	if a != nil {
		w := a.MovementProb / 0.55
		if w > 1.0 {
			w = 1.0
		}
		switch a.MovementLabel {
		case "up":
			dirScore += 1.5 * w
		case "down":
			dirScore -= 1.5 * w
		}
	}

	if b != nil {
		if b.HealthScore >= 6.0 {
			dirScore += 1.0
		} else if b.HealthScore <= 4.0 {
			dirScore -= 1.0
		}
		if b.ROEDirection == "up" {
			dirScore += 0.5
		} else if b.ROEDirection == "down" {
			dirScore -= 0.5
		}
		if b.RevenueDirection == "up" {
			dirScore += 0.5
		} else if b.RevenueDirection == "down" {
			dirScore -= 0.5
		}
	}

	techBull := technical != nil && technical.Score >= 65 &&
		(technical.Trend == "上升" || technical.MAStatus == "多头排列" || technical.MACDStatus == "金叉" || technical.MACDStatus == "零轴上")
	techBear := technical != nil && technical.Score <= 45 &&
		(technical.Trend == "下降" || technical.MAStatus == "空头排列" || technical.MACDStatus == "死叉" || technical.MACDStatus == "零轴下")

	if techBull {
		dirScore += 1.0
	} else if techBear {
		dirScore -= 1.0
	}

	if activity != nil {
		if activity.Score >= 75 && activity.AmountScore >= 70 {
			dirScore += 0.8
		} else if activity.Score >= 60 {
			dirScore += 0.4
		} else if activity.Score <= 40 {
			dirScore -= 0.6
		}
	}

	sentimentHot := sentiment != nil && sentiment.HasData && sentiment.Score >= 0.4 && sentiment.HeatIndex >= 60
	sentimentWarm := sentiment != nil && sentiment.HasData && sentiment.Score >= 0.2
	sentimentCold := sentiment != nil && sentiment.HasData && sentiment.Score <= -0.3

	if sentimentHot {
		dirScore += 0.8
	} else if sentimentWarm {
		dirScore += 0.4
	} else if sentimentCold {
		dirScore -= 0.6
	}

	if dirScore > 3.0 {
		dirScore = 3.0
	}
	if dirScore < -3.0 {
		dirScore = -3.0
	}

	// 2. 波动宽度
	width := 6.0
	if math.Abs(dirScore) >= 2.0 {
		width = 10.0
	} else if math.Abs(dirScore) >= 1.2 {
		width = 7.0
	} else if math.Abs(dirScore) >= 0.5 {
		width = 5.0
	}

	if a != nil && a.AnomalyProb >= 0.35 {
		width += 3.0
	}
	if activity != nil && activity.Score >= 80 {
		width += 2.0
	}
	if technical != nil && (technical.Score >= 85 || technical.Score <= 25) {
		width += 2.0
	}

	// A-Score 财务风险修正
	if ascore >= 70 {
		dirScore -= 1.0
		width += 2.0
	} else if ascore >= 60 {
		dirScore -= 0.5
		width += 1.0
	} else if ascore < 40 {
		dirScore += 0.5
	}

	center := dirScore * 2.5
	if center > 8.0 {
		center = 8.0
	}
	if center < -8.0 {
		center = -8.0
	}

	half := width / 2.0
	low := center - half
	high := center + half

	// 3. 映射方向文案
	var direction string
	switch {
	case dirScore >= 1.8:
		direction = "上涨"
	case dirScore >= 1.0:
		direction = "谨慎观望（偏暖）"
	case dirScore >= -0.3 && dirScore <= 0.3:
		direction = "震荡"
	case dirScore > -1.8:
		direction = "谨慎观望（偏冷）"
	default:
		direction = "下跌"
	}

	// 4. 动态生成理由
	var parts []string
	if techBull {
		parts = append(parts, "技术面呈多头信号")
	} else if techBear {
		parts = append(parts, "技术面存在空头压力")
	} else if technical != nil && technical.Score > 0 {
		parts = append(parts, "技术形态"+technical.Grade)
	}

	if activity != nil && activity.Score >= 75 {
		parts = append(parts, "交易活跃度较高")
	} else if activity != nil && activity.Score <= 40 {
		parts = append(parts, "交易活跃度低迷")
	} else if activity != nil && activity.Score > 0 {
		parts = append(parts, "交易活跃度"+activity.Grade)
	}

	if sentimentHot {
		parts = append(parts, "舆情情绪偏暖且市场关注度较高")
	} else if sentimentCold {
		parts = append(parts, "舆情情绪偏冷")
	} else if sentimentWarm {
		parts = append(parts, "舆情情绪温和偏暖")
	}

	if b != nil {
		if b.HealthScore >= 6.0 && (b.ROEDirection == "up" || b.RevenueDirection == "up") {
			parts = append(parts, "财务基本面持续改善")
		} else if b.HealthScore <= 4.0 && (b.ROEDirection == "down" || b.RevenueDirection == "down") {
			parts = append(parts, "财务基本面承压")
		} else if b.HealthScore >= 6.0 {
			parts = append(parts, "财务健康度良好")
		} else if b.HealthScore <= 4.0 {
			parts = append(parts, "财务健康度偏弱")
		}
	}

	if ascore >= 70 {
		parts = append(parts, "A-Score 综合财务风险较高，基本面存在明显隐患")
	} else if ascore >= 60 {
		parts = append(parts, "A-Score 综合财务风险偏高，需警惕基本面隐患")
	} else if ascore < 40 {
		parts = append(parts, "A-Score 综合财务风险较低，基本面相对稳健")
	}

	if a != nil {
		switch a.MovementLabel {
		case "up":
			parts = append(parts, "Engine-A 情绪价格模型显示上涨动能")
		case "down":
			parts = append(parts, "Engine-A 情绪价格模型显示调整压力")
		case "flat":
			parts = append(parts, "Engine-A 情绪价格模型信号中性")
		}
	}

	reason := "未来2-4周"
	if len(parts) > 0 {
		reason += "，" + strings.Join(parts, "，")
	}
	reason += "。综合判断为" + direction + "。"

	return &MLSummary{
		HasData:   true,
		Direction: direction,
		RangeLow:  low,
		RangeHigh: high,
		Reason:    reason,
	}
}

// RunMLEngineB 运行引擎 B 推理
func RunMLEngineB(financialSeq [][]float64) (*MLFinancialPrediction, error) {
	resp, err := callMLInference("B", map[string]any{
		"financial_seq": financialSeq,
	})
	if err != nil {
		return nil, err
	}
	result := &MLFinancialPrediction{}
	parseDir := func(key string) (string, float64) {
		if m, ok := resp[key].(map[string]any); ok {
			var d string
			var c float64
			if dv, ok := m["direction"].(string); ok {
				d = dv
			}
			if cv, ok := m["confidence"].(float64); ok {
				c = cv
			}
			return d, c
		}
		return "", 0
	}
	result.ROEDirection, result.ROEProb = parseDir("roe")
	result.RevenueDirection, result.RevenueProb = parseDir("revenue")
	result.MScoreDirection, result.MScoreProb = parseDir("mscore")
	if h, ok := resp["health_score"].(float64); ok {
		result.HealthScore = h
	}
	return result, nil
}

// RunMLEngineD 运行引擎 D 风险预警推理
func RunMLEngineD(features []float64) (*MLDRiskPrediction, error) {
	resp, err := callMLInference("D", map[string]any{
		"features": features,
	})
	if err != nil {
		return nil, err
	}
	result := &MLDRiskPrediction{}
	if label, ok := resp["risk_label"].(float64); ok {
		result.RiskLabel = int(label)
	}
	if prob, ok := resp["risk_prob"].(float64); ok {
		result.RiskProb = prob
	}
	if level, ok := resp["risk_level"].(string); ok {
		result.RiskLevel = level
	}
	if factors, ok := resp["top_factors"].([]any); ok {
		for _, f := range factors {
			if s, ok := f.(string); ok {
				result.TopFactors = append(result.TopFactors, s)
			}
		}
	}
	if loaded, ok := resp["model_loaded"].(bool); ok {
		result.ModelLoaded = loaded
	}
	return result, nil
}
