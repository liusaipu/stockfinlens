package downloader

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsBoardBlacklisted(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"医疗研发外包", false},
		{"CXO", false},
		{"白酒", false},
		{"沪深300指数", true}, // 含"指数"
		{"医药ETF", true},   // 含"ETF"
		{"江苏板块", true},
		{"深圳本地", true},
		{"概念股", true},
		{"", true}, // 空名也算黑名单
	}
	for _, c := range cases {
		got := isBoardBlacklisted(c.name)
		if got != c.want {
			t.Errorf("isBoardBlacklisted(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestConstituentToSymbol(t *testing.T) {
	cases := []struct {
		c    ConceptConstituent
		want string
	}{
		{ConceptConstituent{Code: "600519", Market: "SH"}, "600519.SH"},
		{ConceptConstituent{Code: "300750", Market: "SZ"}, "300750.SZ"},
		{ConceptConstituent{Code: "688981", Market: "SH"}, "688981.SH"},
		// market 缺失时按前缀推断
		{ConceptConstituent{Code: "603259"}, "603259.SH"}, // 6 开头 → SH
		{ConceptConstituent{Code: "002074"}, "002074.SZ"}, // 0 开头 → SZ
		{ConceptConstituent{Code: ""}, ""},                // 空代码无法构造
	}
	for _, c := range cases {
		got := constituentToSymbol(c.c)
		if got != c.want {
			t.Errorf("constituentToSymbol(%+v) = %q, want %q", c.c, got, c.want)
		}
	}
}

func TestConceptMembershipRoundtrip(t *testing.T) {
	dir := t.TempDir()
	m := &ConceptMembership{
		Version:       conceptMembershipVersion,
		UpdatedAt:     time.Now().Format(time.RFC3339),
		ConceptCount:  3,
		IndustryCount: 2,
		SymbolCount:   2,
		Concepts: map[string][]string{
			"603259.SH": {"CXO", "创新药", "医疗外包"},
			"002821.SZ": {"CXO", "原料药"},
		},
		SubIndustry: map[string]string{
			"603259.SH": "医疗研发外包",
			"002821.SZ": "医疗研发外包",
		},
	}
	if err := saveConceptMembership(dir, m); err != nil {
		t.Fatalf("saveConceptMembership err: %v", err)
	}
	// 确认文件存在
	if _, err := os.Stat(filepath.Join(dir, conceptMembershipFile)); err != nil {
		t.Fatalf("expected file at %s: %v", conceptMembershipFile, err)
	}
	loaded := LoadConceptMembership(dir)
	if loaded == nil {
		t.Fatal("LoadConceptMembership returned nil")
	}
	if loaded.SymbolCount != 2 {
		t.Errorf("SymbolCount=%d, want 2", loaded.SymbolCount)
	}
	if got := loaded.SubIndustry["603259.SH"]; got != "医疗研发外包" {
		t.Errorf("SubIndustry[603259.SH]=%q, want 医疗研发外包", got)
	}
	if got := loaded.Concepts["603259.SH"]; len(got) != 3 {
		t.Errorf("Concepts[603259.SH] len=%d, want 3", len(got))
	}
}

func TestConceptMembershipExpiry(t *testing.T) {
	cases := []struct {
		name    string
		updated string
		want    bool
	}{
		{"nil safe", "", true},
		{"fresh (now)", time.Now().Format(time.RFC3339), false},
		{"8 days ago", time.Now().Add(-8 * 24 * time.Hour).Format(time.RFC3339), true},
		{"6 days ago", time.Now().Add(-6 * 24 * time.Hour).Format(time.RFC3339), false},
		{"malformed", "not-a-timestamp", true},
	}
	for _, c := range cases {
		m := &ConceptMembership{UpdatedAt: c.updated}
		if got := m.IsExpired(); got != c.want {
			t.Errorf("%s: IsExpired()=%v, want %v", c.name, got, c.want)
		}
	}
	// 显式测 nil 接收者
	var nilM *ConceptMembership
	if !nilM.IsExpired() {
		t.Error("nil membership should be considered expired")
	}
}

func TestLoadConceptMembershipMissing(t *testing.T) {
	dir := t.TempDir()
	// 文件不存在应返回 nil
	if m := LoadConceptMembership(dir); m != nil {
		t.Errorf("LoadConceptMembership on empty dir = %+v, want nil", m)
	}
	// 文件损坏（无效 JSON）应返回 nil
	path := filepath.Join(dir, conceptMembershipFile)
	if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
		t.Fatal(err)
	}
	if m := LoadConceptMembership(dir); m != nil {
		t.Errorf("LoadConceptMembership on corrupt file = %+v, want nil", m)
	}
	// 版本号不匹配应返回 nil
	if err := os.WriteFile(path, []byte(`{"version":99,"concepts":{}}`), 0644); err != nil {
		t.Fatal(err)
	}
	if m := LoadConceptMembership(dir); m != nil {
		t.Errorf("LoadConceptMembership with version=99 = %+v, want nil", m)
	}
}
