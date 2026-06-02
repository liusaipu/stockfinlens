package analyzer

import (
	"strings"
	"testing"
)

// TestComputeSimilarity_SubIndustryBeatsIndustry 核心回归：
// 当目标和候选 A 同属二级行业（医疗研发外包），候选 B 仅同一级（医疗行业）时，
// A 的得分必须显著高于 B —— 这是修复药明康德推荐错配的根本目的。
func TestComputeSimilarity_SubIndustryBeatsIndustry(t *testing.T) {
	targetIndustry := "医疗行业"
	targetSubIndustry := "医疗研发外包"
	targetConcepts := []string{"CXO", "创新药"}

	// 候选 A：药明康德的真正同行（凯莱英）—— 同二级 + 同概念
	candA := candidateInfo{
		Symbol:      "002821.SZ",
		Name:        "凯莱英",
		Industry:    "医疗行业",
		SubIndustry: "医疗研发外包",
		Concepts:    []string{"CXO", "原料药"},
	}
	// 候选 B：医院类（国际医学）—— 仅同一级，无 CXO 概念
	candB := candidateInfo{
		Symbol:      "000516.SZ",
		Name:        "国际医学",
		Industry:    "医疗行业",
		SubIndustry: "医疗服务",
		Concepts:    []string{"民营医院", "AI医疗"},
	}

	scoreA, reasonsA, _ := computeSimilarity(targetIndustry, targetSubIndustry, 0, 0, 0, 0, targetConcepts, candA)
	scoreB, reasonsB, _ := computeSimilarity(targetIndustry, targetSubIndustry, 0, 0, 0, 0, targetConcepts, candB)

	t.Logf("候选A(同二级+同概念): score=%.2f reasons=%v", scoreA, reasonsA)
	t.Logf("候选B(仅同一级): score=%.2f reasons=%v", scoreB, reasonsB)

	if scoreA <= scoreB {
		t.Errorf("二级行业相同的候选(A=%.2f)应当显著高于仅一级行业相同的候选(B=%.2f)", scoreA, scoreB)
	}
	if scoreA-scoreB < 30 {
		t.Errorf("分差太小：A=%.2f B=%.2f 差=%.2f，期望≥30（subIndustry 75 vs primary 35）", scoreA, scoreB, scoreA-scoreB)
	}
	// 候选 A 的理由里必须出现二级行业名
	gotSubReason := false
	for _, r := range reasonsA {
		if strings.Contains(r, targetSubIndustry) {
			gotSubReason = true
			break
		}
	}
	if !gotSubReason {
		t.Errorf("候选 A 的 reasons 中应当包含二级行业 %q，实际 %v", targetSubIndustry, reasonsA)
	}
}

// TestComputeSimilarity_ConceptOverlapBeatsPrimary 当候选有强概念重叠但行业不同时，
// 概念匹配（scaled 70）应当能超过一级行业匹配（35）。
func TestComputeSimilarity_ConceptOverlapBeatsPrimary(t *testing.T) {
	targetIndustry := "半导体"
	targetConcepts := []string{"AI算力", "光模块", "数据中心", "信创"} // 4 个概念

	// 候选 A：同行业，0 个概念重叠 → industry 35
	candA := candidateInfo{
		Symbol:   "688981.SH",
		Industry: "半导体",
		Concepts: []string{"晶圆", "封测"},
	}
	// 候选 B：行业不同（通信），但 4 个概念全部重叠 → concept 70
	candB := candidateInfo{
		Symbol:   "300308.SZ",
		Industry: "通信设备",
		Concepts: []string{"AI算力", "光模块", "数据中心", "信创"},
	}

	scoreA, reasonsA, _ := computeSimilarity(targetIndustry, "", 0, 0, 0, 0, targetConcepts, candA)
	scoreB, reasonsB, _ := computeSimilarity(targetIndustry, "", 0, 0, 0, 0, targetConcepts, candB)
	t.Logf("候选A(同行业,0概念): %.2f %v", scoreA, reasonsA)
	t.Logf("候选B(异行业,4概念全重叠): %.2f %v", scoreB, reasonsB)

	if scoreB <= scoreA {
		t.Errorf("强概念重叠(B=%.2f)应当高于仅一级行业匹配(A=%.2f)", scoreB, scoreA)
	}
}

// TestComputeSimilarity_PrimaryStillWorksWithoutSubIndustry 反查表未构建时（targetSubIndustry=""），
// 一级行业匹配仍应正常生效，不应回退到 0 分。
func TestComputeSimilarity_PrimaryStillWorksWithoutSubIndustry(t *testing.T) {
	score, reasons, _ := computeSimilarity("白酒", "", 0, 0, 0, 0, nil, candidateInfo{
		Industry: "白酒",
	})
	if score < 30 {
		t.Errorf("无二级行业时，同一级行业(白酒)应至少拿到一级行业的 35 分，实际 %.2f reasons=%v", score, reasons)
	}
}
