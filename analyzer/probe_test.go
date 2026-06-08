package analyzer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// downloaderMarketCacheItem 复刻 downloader.MarketCacheItem 的 JSON 结构
// （analyzer 包不能反向依赖 downloader，所以测试里直接 mirror 字段）
type downloaderMarketCacheItem struct {
	Code              string   `json:"code"`
	Name              string   `json:"name"`
	Market            string   `json:"market"`
	Industry          string   `json:"industry"`
	SubIndustry       string   `json:"sub_industry,omitempty"`
	MarketCap         float64  `json:"market_cap"`
	ROE               float64  `json:"roe"`
	GrossprofitMargin float64  `json:"grossprofit_margin"`
	Concepts          []string `json:"concepts,omitempty"`
}

func TestProbeRanksLongxin(t *testing.T) {
	dataDir := os.ExpandEnv("$HOME/.config/stock-analyzer")
	type MCFile struct {
		Items map[string]downloaderMarketCacheItem `json:"items"`
	}
	data, _ := os.ReadFile(dataDir + "/market_cache.json")
	var mcFile MCFile
	json.Unmarshal(data, &mcFile)

	// 模拟 app.go:3514-3531 的真实转换流程
	cacheItems := make(map[string]MarketCacheItem, len(mcFile.Items))
	for s, item := range mcFile.Items {
		cacheItems[s] = MarketCacheItem{
			Symbol:      s,
			Name:        item.Name,
			Industry:    item.Industry,
			SubIndustry: item.SubIndustry,
			MarketCap:   item.MarketCap,
			ROE:         item.ROE,
			GM:          item.GrossprofitMargin,
			Concepts:    item.Concepts,
		}
	}

	pdata, _ := os.ReadFile(dataDir + "/data/300088.SZ/profile.json")
	var profile StockProfile
	json.Unmarshal(pdata, &profile)
	fd, _ := LoadFinancialData(dataDir+"/data", "300088.SZ")

	// 拿全量推荐结果（不限 maxResults）
	recs := RecommendComparables("300088.SZ", &profile, fd, dataDir, nil, cacheItems, 9999)
	ranks := map[string]int{}
	scores := map[string]float64{}
	reasons := map[string][]string{}
	sort.Slice(recs, func(i, j int) bool {
		if recs[i].Score != recs[j].Score {
			return recs[i].Score > recs[j].Score
		}
		return recs[i].Symbol < recs[j].Symbol
	})
	for i, r := range recs {
		ranks[r.Symbol] = i + 1
		scores[r.Symbol] = r.Score
		reasons[r.Symbol] = r.Reasons
	}
	t.Logf("通过门槛=%d/%d", len(recs), len(cacheItems))
	t.Logf("Top10:")
	for i := 0; i < 10 && i < len(recs); i++ {
		t.Logf("  #%d %s %s 得分=%.2f reasons=%v", i+1, recs[i].Symbol, recs[i].Name, recs[i].Score, recs[i].Reasons)
	}

	targets := []struct{ sym, name string }{
		{"300433.SZ", "蓝思"}, {"600552.SH", "凯盛"},
		{"002600.SZ", "领益"}, {"300709.SZ", "精研"},
		{"300162.SZ", "雷曼"}, {"000020.SZ", "深华发"},
		{"000050.SZ", "深天马"}, {"000100.SZ", "TCL"},
	}
	fmt.Println()
	dataRoot := filepath.Join(dataDir, "data")
	targetConcepts := loadConcepts(dataRoot, "300088.SZ")
	targetSubInd := lookupSubIndustry(dataRoot, "300088.SZ")
	for _, tg := range targets {
		r, ok := ranks[tg.sym]
		if !ok {
			item := cacheItems[tg.sym]
			industry := item.Industry
			pf := filepath.Join(dataRoot, tg.sym, "profile.json")
			if d, err := os.ReadFile(pf); err == nil {
				if li := extractJSONString(d, "industry"); li != "" {
					industry = li
				}
			}
			concepts := loadConcepts(dataRoot, tg.sym)
			if len(concepts) == 0 {
				concepts = item.Concepts
			}
			ci := candidateInfo{
				Symbol: tg.sym, Name: item.Name, Industry: industry,
				SubIndustry: item.SubIndustry, MarketCap: item.MarketCap,
				ROE: item.ROE, GM: item.GM, Concepts: concepts,
			}
			score, reasonsCalc, _ := computeSimilarity(
				profile.Industry, targetSubInd, profile.MarketCap, 0, 0, 0, targetConcepts, ci, loadConceptMembership(dataRoot),
			)
			fmt.Printf("%s %s: 未通过门槛 [DIAG] industry(t=%q c=%q) subInd(t=%q c=%q) concepts(t=%d c=%d) → score=%.2f reasons=%v\n",
				tg.name, tg.sym, profile.Industry, ci.Industry, targetSubInd, ci.SubIndustry,
				len(targetConcepts), len(ci.Concepts), score, reasonsCalc)
		} else {
			fmt.Printf("%s %s: 排名#%d 得分=%.2f reasons=%v\n", tg.name, tg.sym, r, scores[tg.sym], reasons[tg.sym])
		}
	}
}
