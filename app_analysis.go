package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liusaipu/stockfinlens/analyzer"
	"github.com/liusaipu/stockfinlens/downloader"
)

// rimHash 给 customRIM 生成稳定 hash，用于构造 singleflight key
// （nil 返回空串，不同内容必然对应不同 key）
func rimHash(rim *analyzer.RIMData) string {
	if rim == nil {
		return ""
	}
	b, err := json.Marshal(rim)
	if err != nil {
		return "err"
	}
	sum := sha1.Sum(b)
	return hex.EncodeToString(sum[:])
}

// analyzeStockInternal 是 AnalyzeStock / AnalyzeStockWithRIM 的统一实现。
// 用 singleflight 合并同 key 的并发请求：相同 symbol+overwriteLatest+RIM 的并发调用
// 只跑一次完整分析，其它 waiter 共享结果；不同 flag 的并发调用各自独立运行。
func (a *App) analyzeStockInternal(symbol string, overwriteLatest bool, customRIM *analyzer.RIMData) (*analyzer.AnalysisReport, error) {
	key := fmt.Sprintf("%s|overwrite=%v|rim=%s", symbol, overwriteLatest, rimHash(customRIM))

	result, err, _ := a.analysisGroup.Do(key, func() (interface{}, error) {
		// 兜底：被调用链上任意一处 panic 都不会让所有 waiter 死锁
		var report *analyzer.AnalysisReport
		var runErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					runErr = fmt.Errorf("分析过程发生 panic: %v", r)
					debugLog("[AnalyzeStock] panic recovered for %s: %v", symbol, r)
				}
			}()
			report, runErr = a.runAnalysisLocked(symbol, overwriteLatest, customRIM)
		}()
		if runErr != nil {
			return nil, runErr
		}
		return report, nil
	})

	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return result.(*analyzer.AnalysisReport), nil
}

// runAnalysisLocked 执行实际的分析流程。已在 singleflight key 保护下，不允许同 key 并发。
func (a *App) runAnalysisLocked(symbol string, overwriteLatest bool, customRIM *analyzer.RIMData) (*analyzer.AnalysisReport, error) {
	debugLog("[AnalyzeStock] Starting analysis for %s, overwriteLatest=%v", symbol, overwriteLatest)
	if a.storage == nil {
		debugLog("[AnalyzeStock] Error: storage not initialized")
		return nil, fmt.Errorf("存储未初始化")
	}
	debugLog("[AnalyzeStock] Storage initialized, dataDir=%s", a.storage.DataDir())
	comparables, _ := a.storage.GetComparables(symbol)
	nameMap := make(map[string]string, len(a.stocks))
	for _, s := range a.stocks {
		nameMap[s.Code] = s.Name
	}
	compAnalysis, _ := analyzer.BuildComparableAnalysis(a.storage.DataDir(), comparables, nameMap)

	// 并发获取网络数据：实时行情、K线、舆情情绪、资金流向
	var quoteData *analyzer.QuoteData
	var klines []downloader.KlineData
	var sentimentData *analyzer.SentimentData
	var moneyflowData *analyzer.MoneyflowData
	var wgNet sync.WaitGroup
	wgNet.Add(4)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] quote goroutine: %v", r)
			}
			wgNet.Done()
		}()
		if q, err := a.GetStockQuote(symbol); err == nil && q != nil {
			quoteData = &analyzer.QuoteData{
				CurrentPrice:         q.CurrentPrice,
				ChangePercent:        q.ChangePercent,
				ChangeAmount:         q.ChangeAmount,
				Volume:               q.Volume,
				TurnoverAmount:       q.TurnoverAmount,
				TurnoverRate:         q.TurnoverRate,
				Amplitude:            q.Amplitude,
				High:                 q.High,
				Low:                  q.Low,
				Open:                 q.Open,
				PreviousClose:        q.PreviousClose,
				CirculatingMarketCap: q.CirculatingMarketCap,
				VolumeRatio:          q.VolumeRatio,
				PE:                   q.PE,
				PB:                   q.PB,
				MarketCap:            q.MarketCap,
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] klines goroutine: %v", r)
			}
			wgNet.Done()
		}()
		parts := strings.Split(symbol, ".")
		if len(parts) == 2 {
			market := strings.ToUpper(parts[1])
			code := parts[0]
			var list []downloader.KlineData
			var err error
			// 拉 2500 条（约 10 年）保证分析后缓存即覆盖 K线 view 的全量需求;
			// SFL 启用时 DataRouter 内部会忽略 limit 直接拉上市以来全部历史
			if a.dataRouter != nil {
				list, err = a.dataRouter.FetchKlines(a.ctx, market, code, 2500, "daily")
			} else {
				list, err = downloader.FetchStockKlines(a.ctx, market, code, 2500, "daily")
			}
			if err == nil && len(list) >= 20 {
				klines = list
			}
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] sentiment goroutine: %v", r)
			}
			wgNet.Done()
		}()
		if cachedSentiment, err := a.storage.LoadStockSentiment(symbol); err == nil && cachedSentiment != nil {
			path := filepath.Join(a.storage.DataDir(), "data", symbol, "sentiment.json")
			if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < 60*time.Minute {
				sentimentData = &analyzer.SentimentData{
					Score:         cachedSentiment.Score,
					HeatIndex:     cachedSentiment.HeatIndex,
					PositiveWords: cachedSentiment.PositiveWords,
					NegativeWords: cachedSentiment.NegativeWords,
					Summaries:     make([]analyzer.SentimentSummary, len(cachedSentiment.Summaries)),
					HasData:       cachedSentiment.HasData,
				}
				for i, s := range cachedSentiment.Summaries {
					sentimentData.Summaries[i] = analyzer.SentimentSummary{
						Title:     s.Title,
						Source:    s.Source,
						Date:      s.Date,
						Sentiment: s.Sentiment,
					}
				}
			}
		}
		if sentimentData == nil {
			parts := strings.Split(symbol, ".")
			if len(parts) == 2 {
				code := parts[0]
				market := strings.ToUpper(parts[1])
				debugLog("[Sentiment] fetching for %s %s", market, code)
				if s, err := downloader.FetchStockSentiment(a.ctx, market, code); err == nil && s != nil {
					debugLog("[Sentiment] fetched ok for %s, summaries=%d", symbol, len(s.Summaries))
					sentimentData = &analyzer.SentimentData{
						Score:         s.Score,
						HeatIndex:     s.HeatIndex,
						PositiveWords: s.PositiveWords,
						NegativeWords: s.NegativeWords,
						Summaries:     make([]analyzer.SentimentSummary, len(s.Summaries)),
						HasData:       s.HasData,
					}
					for i, summary := range s.Summaries {
						sentimentData.Summaries[i] = analyzer.SentimentSummary{
							Title:     summary.Title,
							Source:    summary.Source,
							Date:      summary.Date,
							Sentiment: summary.Sentiment,
						}
					}
					_ = a.storage.SaveStockSentiment(symbol, s)
				} else {
					debugLog("[Sentiment] fetch failed for %s: %v", symbol, err)
				}
			}
		}
	}()

	// 4. 获取资金流向（独立 goroutine，禁止嵌套在 sentiment 内部）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] moneyflow goroutine: %v", r)
			}
			wgNet.Done()
		}()
		if a.dataRouter != nil && a.dataRouter.IsUseForMoneyflow() {
			parts := strings.Split(symbol, ".")
			if len(parts) == 2 {
				market := strings.ToUpper(parts[1])
				code := parts[0]
				end := time.Now().Format("20060102")
				start := time.Now().AddDate(0, 0, -5).Format("20060102")
				items, err := a.dataRouter.FetchMoneyflow(a.ctx, market, code, start, end)
				if err == nil && len(items) > 0 {
					mfItems := make([]analyzer.MoneyflowItem, 0, len(items))
					for _, item := range items {
						mfItems = append(mfItems, analyzer.MoneyflowItem{
							Date:         item.TradeDate,
							MainInflow:   item.BuyLgAmount + item.BuyElgAmount - item.SellLgAmount - item.SellElgAmount,
							SmNetAmount:  item.BuySmAmount - item.SellSmAmount,
							MdNetAmount:  item.BuyMdAmount - item.SellMdAmount,
							LgNetAmount:  item.BuyLgAmount - item.SellLgAmount,
							ElgNetAmount: item.BuyElgAmount - item.SellElgAmount,
						})
					}
					moneyflowData = &analyzer.MoneyflowData{
						HasData: true,
						Items:   mfItems,
					}
					var totalMain float64
					var inflowDays int
					for _, item := range mfItems {
						totalMain += item.MainInflow
						if item.MainInflow > 0 {
							inflowDays++
						}
					}
					dayCount := len(mfItems)
					if inflowDays == dayCount {
						moneyflowData.Summary = fmt.Sprintf("近%d日主力持续流入，累计 %.2f 亿元", dayCount, totalMain/1e8)
					} else if inflowDays == 0 {
						moneyflowData.Summary = fmt.Sprintf("近%d日主力持续流出，累计 %.2f 亿元", dayCount, totalMain/1e8)
					} else if totalMain > 0 {
						moneyflowData.Summary = fmt.Sprintf("近%d日主力%d日流入，累计净流入 %.2f 亿元", dayCount, inflowDays, totalMain/1e8)
					} else {
						moneyflowData.Summary = fmt.Sprintf("近%d日主力%d日流入，累计净流出 %.2f 亿元", dayCount, inflowDays, -totalMain/1e8)
					}
				}
			}
		}
	}()

	wgNet.Wait()

	// 构建十五五政策匹配数据
	var policyData *analyzer.PolicyMatchData
	profile, err := a.GetStockProfile(symbol)
	if err == nil && profile != nil && profile.Industry == "" {
		// 缓存中行业为空，强制刷新
		profile, _ = a.RefreshStockProfile(symbol)
	}
	if profile != nil {
		conceptList := []string{}
		if concepts, err := a.GetStockConcepts(symbol); err == nil && concepts != nil {
			conceptList = concepts.Concepts
		}
		policyData = analyzer.BuildPolicyMatch(profile.Industry, conceptList)
	}

	// 技术形态分析
	var technicalData *analyzer.TechnicalData
	if len(klines) >= 30 {
		tklines := make([]analyzer.TechnicalKline, len(klines))
		for i, k := range klines {
			tklines[i] = analyzer.TechnicalKline{
				Time:   k.Time,
				Open:   k.Open,
				Close:  k.Close,
				Low:    k.Low,
				High:   k.High,
				Volume: k.Volume,
			}
		}
		technicalData = analyzer.AnalyzeTechnical(tklines)
	}

	// 交易活跃度分析
	var activityData *analyzer.ActivityData
	if len(klines) >= 20 && quoteData != nil {
		baselines, _ := a.storage.LoadIndustryBaselines()
		industry := ""
		if profile != nil {
			industry = profile.Industry
		}
		aklines := make([]analyzer.ActivityKline, len(klines))
		for i, k := range klines {
			aklines[i] = analyzer.ActivityKline{
				Time:   k.Time,
				Open:   k.Open,
				Close:  k.Close,
				Low:    k.Low,
				High:   k.High,
				Volume: k.Volume,
				Amount: k.Amount,
			}
		}
		qLite := &analyzer.StockQuoteLite{CirculatingMarketCap: quoteData.CirculatingMarketCap}
		activityData = analyzer.CalculateActivity(aklines, qLite, industry, baselines)
	}
	// 加载财务数据（ML 和 RIM fallback 都需要）
	var finData *analyzer.FinancialData
	if fd, err := analyzer.LoadFinancialData(a.storage.DataDir(), symbol); err == nil && fd != nil {
		finData = fd
	}

	// 补充缺失的分红数据：若现金流量表中分红字段全为0，尝试从东财API获取
	if finData != nil {
		allZero := true
		hasDividendField := false
		for _, year := range finData.Years {
			if strings.HasSuffix(year, "-12-31") || len(year) == 4 {
				v := finData.GetValueOrZero(finData.CashFlow, "分配股利、利润或偿付利息支付的现金", year)
				if v != 0 {
					allZero = false
					break
				}
				hasDividendField = true
			}
		}
		if allZero && hasDividendField {
			parts := strings.Split(symbol, ".")
			if len(parts) == 2 {
				market := strings.ToUpper(parts[1])
				code := parts[0]
				if dividendMap, err := downloader.FetchCashFlowDividendFromEastMoney(a.ctx, market, code, len(finData.Years)); err == nil && len(dividendMap) > 0 {
					for year, val := range dividendMap {
						if _, ok := finData.CashFlow["分配股利、利润或偿付利息支付的现金"]; !ok {
							finData.CashFlow["分配股利、利润或偿付利息支付的现金"] = make(map[string]float64)
						}
						finData.CashFlow["分配股利、利润或偿付利息支付的现金"][year] = val
						debugLog("[AnalyzeStock] %s 补充分红数据 %s=%.0f", symbol, year, val)
					}
				} else {
					debugLog("[AnalyzeStock] %s 尝试从东财补充分红数据失败: %v", symbol, err)
				}
			}
		}
	}

	// ML 双引擎预测 + RIM 外部数据获取 并发执行
	var mlData *analyzer.MLPredictionData
	var extRIM *downloader.RIMExternalData
	var rimErr error
	var wg sync.WaitGroup

	// 并发 1: ML Engine B + Engine A
	debugLog("[AnalyzeStock] Starting ML engines, finData=%v, klines=%d", finData != nil, len(klines))
	if finData != nil {
		wg.Add(1)
		go func() {
			defer func() {
				if r := recover(); r != nil {
					debugLog("[PANIC] ML engines goroutine: %v", r)
				}
				wg.Done()
			}()
			mlLocal := &analyzer.MLPredictionData{}
			// Engine B
			if finSeq := analyzer.BuildMLEngineBInput(finData); len(finSeq) > 0 {
				if fp, err := analyzer.RunMLEngineB(finSeq); err == nil {
					mlLocal.Financial = fp
				} else {
					debugLog("[ML] Engine B failed for %s: %v", symbol, err)
					if mlLocal.MLError == "" {
						mlLocal.MLError = err.Error()
					}
				}
			}
			// Engine D: 风险预警（优先使用实时行情填充市场指标）
			if dFeatures := analyzer.BuildMLEngineDInput(finData, quoteData); len(dFeatures) > 0 {
				if dp, err := analyzer.RunMLEngineD(dFeatures); err == nil {
					mlLocal.EngineD = dp
				} else {
					debugLog("[ML] Engine D failed for %s: %v", symbol, err)
					if mlLocal.MLError == "" {
						mlLocal.MLError = err.Error()
					}
				}
			}
			// Engine A（价格序列始终可用；sentiment 为 nil 时 text_seq 补 0）
			if len(klines) >= 16 {
				mlKlines := make([]analyzer.MLKlineData, len(klines))
				for i, k := range klines {
					mlKlines[i] = analyzer.MLKlineData{
						Time: k.Time, Open: k.Open, Close: k.Close,
						Low: k.Low, High: k.High, Volume: k.Volume, Amount: k.Amount,
					}
				}
				textSeq, priceSeq := analyzer.BuildMLEngineAInputFromKlines(mlKlines, sentimentData)
				if textSeq != nil && priceSeq != nil {
					if sp, err := analyzer.RunMLEngineA(textSeq, priceSeq); err == nil {
						mlLocal.Sentiment = sp
					} else {
						debugLog("[ML] Engine A failed for %s: %v", symbol, err)
						if mlLocal.MLError == "" {
							mlLocal.MLError = err.Error()
						}
					}
				}
			}
			// 计算 ML 置信等级
			mlLocal.Confidence = computeMLConfidence(finData, klines, sentimentData, quoteData)
			if mlLocal.Financial != nil || mlLocal.Sentiment != nil || mlLocal.EngineD != nil || mlLocal.MLError != "" {
				mlData = mlLocal
			}
		}()
	}

	// 并发 2: RIM 外部数据获取（带缓存）
	if customRIM == nil && quoteData != nil {
		pureCode := symbol
		if idx := strings.Index(symbol, "."); idx > 0 {
			pureCode = symbol[:idx]
		}
		// 先读缓存
		if cached, err := a.storage.LoadRIMCache(symbol); err == nil && cached != nil {
			extRIM = cached
		} else {
			wg.Add(1)
			go func() {
				defer func() {
					if r := recover(); r != nil {
						debugLog("[PANIC] RIM fetch goroutine: %v", r)
					}
					wg.Done()
				}()
				extRIM, rimErr = downloader.FetchRIMExternalData(pureCode)
				if rimErr != nil {
					fmt.Printf("[RIM] fetch failed for %s: %v\n", symbol, rimErr)
				}
				if extRIM != nil {
					_ = a.storage.SaveRIMCache(symbol, extRIM)
				}
			}()
		}
	}

	// 并发 3: A-Score 非财务风险爬虫（股权质押、问询函、减持）
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] risk crawler goroutine: %v", r)
			}
			wg.Done()
		}()
		if rc, err := downloader.FetchRiskCrawlerData(symbol); err == nil {
			if finData != nil {
				if finData.Extras == nil {
					finData.Extras = make(map[string]float64)
				}
				if rc.PledgeRatio != nil {
					finData.Extras["pledge_ratio"] = *rc.PledgeRatio
				}
				if rc.InquiryCount1Y != nil {
					finData.Extras["inquiry_count_1y"] = float64(*rc.InquiryCount1Y)
				}
				if rc.ReductionCount1Y != nil {
					finData.Extras["reduction_count_1y"] = float64(*rc.ReductionCount1Y)
				}
			}
		} else {
			fmt.Printf("[RiskCrawler] failed for %s: %v\n", symbol, err)
		}
	}()

	// 并发 4: 外部风险数据查询（审计机构、高管变动、诉讼）
	externalRiskData := &analyzer.ExternalRiskData{}
	wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				debugLog("[PANIC] external risk goroutine: %v", r)
			}
			wg.Done()
		}()
		// 审计机构变更 + 审计意见
		if ah, err := downloader.FetchAuditorHistory(symbol); err == nil && ah != nil {
			externalRiskData.AuditorChanged = ah.AuditorChanged
			externalRiskData.AuditorName = ah.AuditorName
			externalRiskData.AuditorChangeDetails = make([]analyzer.AuditorChangeDetail, 0, len(ah.ChangeDetails))
			for _, cd := range ah.ChangeDetails {
				externalRiskData.AuditorChangeDetails = append(externalRiskData.AuditorChangeDetails, analyzer.AuditorChangeDetail{
					Date:                 cd.Date,
					OldAuditor:           cd.OldAuditor,
					NewAuditor:           cd.NewAuditor,
					Reason:               cd.Reason,
					IsBeforeAnnualReport: cd.IsBeforeAnnualReport,
					AnnualReportDeadline: cd.AnnualReportDeadline,
					RawTitle:             cd.RawTitle,
					IsPolicyCompliance:   cd.IsPolicyCompliance,
					IsAbnormal:           cd.IsAbnormal,
					IsPassiveChange:      cd.IsPassiveChange,
					IsNormalRotation:     cd.IsNormalRotation,
				})
			}
			// 将审计意见回填到 finData（供 step1Audit 使用）
			if finData != nil && len(ah.AuditOpinions) > 0 {
				if finData.AuditOpinions == nil {
					finData.AuditOpinions = make(map[string]*analyzer.AuditOpinion)
				}
				for _, ao := range ah.AuditOpinions {
					if ao.Error != "" {
						continue
					}
					finData.AuditOpinions[ao.Year] = &analyzer.AuditOpinion{
						Year:        ao.Year,
						Opinion:     ao.Opinion,
						Auditor:     ao.Auditor,
						IsStandard:  ao.IsStandard,
						NeedsReview: ao.NeedsReview,
					}
				}
			}
		}
		// 高管变动
		if ec, err := downloader.FetchExecChanges(symbol); err == nil && ec != nil {
			// 二次过滤：只保留明确标记为 CFO/审计相关的条目（兼容旧版脚本）
			filteredHistory := make([]string, 0, len(ec.History))
			for _, h := range ec.History {
				if strings.Contains(h, "[CFO]") || strings.Contains(h, "[审计]") {
					filteredHistory = append(filteredHistory, h)
				}
			}
			execCount := len(filteredHistory)
			externalRiskData.ExecChangeCount = execCount
			externalRiskData.ExecHistory = filteredHistory
			externalRiskData.ExecChanged = execCount >= 3
		}
		// 诉讼/担保
		if li, err := downloader.FetchLitigationInfo(symbol); err == nil && li != nil {
			// 二次过滤：排除正常担保额度类公告（兼容旧版脚本）
			filteredHistory := make([]string, 0, len(li.History))
			for _, h := range li.History {
				if strings.Contains(h, "担保额度") || strings.Contains(h, "预计担保") || strings.Contains(h, "为控股子公司提供担保") || strings.Contains(h, "为全资子公司提供担保") || strings.Contains(h, "为子公司提供担保") {
					continue
				}
				filteredHistory = append(filteredHistory, h)
			}
			// 从过滤后的 history 重新判断风险等级
			hasHighRisk := false
			hasFundOccupation := false
			highRiskCount := 0
			hasNormalGuarantee := false
			for _, h := range filteredHistory {
				if strings.Contains(h, "[高风险担保]") || strings.Contains(h, "[诉讼仲裁]") || strings.Contains(h, "[违规处罚]") {
					hasHighRisk = true
					highRiskCount++
				}
				if strings.Contains(h, "资金占用") || strings.Contains(h, "占用资金") {
					hasFundOccupation = true
					highRiskCount++
				}
				if strings.Contains(h, "[担保]") && !strings.Contains(h, "[高风险担保]") {
					hasNormalGuarantee = true
				}
			}
			// 兼容旧版脚本：如果过滤后没有普通担保，但 li.HasGuarantee 为 true，
			// 检查原始 history 中是否真的有非"担保额度"的普通担保
			if !hasNormalGuarantee && li.HasGuarantee {
				for _, h := range li.History {
					if strings.Contains(h, "[担保]") && !strings.Contains(h, "[高风险担保]") &&
						!strings.Contains(h, "担保额度") && !strings.Contains(h, "预计担保") &&
						!strings.Contains(h, "为控股子公司提供担保") && !strings.Contains(h, "为全资子公司提供担保") {
						hasNormalGuarantee = true
						break
					}
				}
			}
			externalRiskData.LitigationCount = highRiskCount
			externalRiskData.LitigationHistory = filteredHistory
			externalRiskData.HasHighRiskGuarantee = hasHighRisk
			externalRiskData.HasFundOccupation = hasFundOccupation
			externalRiskData.HasLitigation = hasHighRisk || hasFundOccupation
			externalRiskData.HasGuarantee = hasNormalGuarantee && !hasHighRisk && !hasFundOccupation
		}
	}()

	wg.Wait()
	debugLog("[AnalyzeStock] ML and RIM data fetching completed, mlData=%v, extRIM=%v", mlData != nil, extRIM != nil)

	// RIM 多期估值数据组装
	var rimData *analyzer.RIMData
	if customRIM != nil {
		rimData = customRIM
	} else if quoteData != nil {
		rimData = &analyzer.RIMData{}
		if extRIM != nil {
			rimData.Rf = extRIM.Rf
			rimData.Beta = extRIM.Beta
			rimData.RmRf = extRIM.RmRf
			rimData.EPSRaw = extRIM.EPSForecast
		} else {
			// 使用默认参数（即使外部数据获取失败也尝试计算）
			rimData.Rf = 0.0183
			rimData.Beta = 0.98
			rimData.RmRf = 0.0517
		}

		bps0 := 0.0
		if quoteData.PB > 0 && quoteData.CurrentPrice > 0 {
			bps0 = quoteData.CurrentPrice / quoteData.PB
		}
		if bps0 <= 0 && extRIM != nil && extRIM.PB > 0 && extRIM.Price > 0 {
			bps0 = extRIM.Price / extRIM.PB
		}
		if bps0 <= 0 {
			totalShares := 0.0
			if extRIM != nil && extRIM.TotalShares > 0 {
				totalShares = extRIM.TotalShares
			} else if quoteData.CurrentPrice > 0 && quoteData.MarketCap > 0 {
				totalShares = quoteData.MarketCap / quoteData.CurrentPrice
			}
			if finData != nil && len(finData.Years) > 0 {
				year := finData.Years[0]
				equity := finData.GetValueOrZero(finData.BalanceSheet, "归母所有者权益合计", year)
				if equity == 0 {
					equity = finData.GetValueOrZero(finData.BalanceSheet, "所有者权益合计", year)
				}
				if equity > 0 && totalShares > 0 {
					bps0 = equity / 1e8 / (totalShares / 1e8) // 元/股
				}
			}
		}

		var epsSeq []float64
		var yearLabels []string
		if extRIM != nil && len(extRIM.EPSForecast) > 0 {
			years := make([]string, 0, len(extRIM.EPSForecast))
			for y := range extRIM.EPSForecast {
				years = append(years, y)
			}
			for i := 0; i < len(years)-1; i++ {
				for j := i + 1; j < len(years); j++ {
					if years[i] > years[j] {
						years[i], years[j] = years[j], years[i]
					}
				}
			}
			currentYear := time.Now().Year()
			for _, y := range years {
				yearInt, err := strconv.Atoi(y)
				if err != nil || yearInt < currentYear {
					continue // 跳过已披露的历史年份，RIM只预测未来
				}
				if v, ok := extRIM.EPSForecast[y]; ok && v > 0 {
					epsSeq = append(epsSeq, v)
					yearLabels = append(yearLabels, y)
				}
			}
		}
		// 如果外部没有预测数据，用 trailing EPS 做起点然后外推
		if len(epsSeq) == 0 {
			trailingEPS := 0.0
			if finData != nil && len(finData.Years) > 0 {
				finYear := finData.Years[0]
				finProfit := finData.GetValueOrZero(finData.IncomeStatement, "归母净利润", finYear)
				if finProfit == 0 {
					finProfit = finData.GetValueOrZero(finData.IncomeStatement, "净利润", finYear)
				}
				totalShares := 0.0
				if extRIM != nil && extRIM.TotalShares > 0 {
					totalShares = extRIM.TotalShares
				} else if quoteData.CurrentPrice > 0 && quoteData.MarketCap > 0 {
					totalShares = quoteData.MarketCap / quoteData.CurrentPrice
				}
				if finProfit > 0 && totalShares > 0 {
					trailingEPS = finProfit / 1e8 / (totalShares / 1e8)
				}
				if trailingEPS <= 0 && bps0 > 0 {
					netProfit := finData.GetValueOrZero(finData.IncomeStatement, "归母净利润", finYear)
					if netProfit == 0 {
						netProfit = finData.GetValueOrZero(finData.IncomeStatement, "净利润", finYear)
					}
					equity := finData.GetValueOrZero(finData.BalanceSheet, "归母所有者权益合计", finYear)
					if equity == 0 {
						equity = finData.GetValueOrZero(finData.BalanceSheet, "所有者权益合计", finYear)
					}
					roe := 0.0
					if equity > 0 {
						roe = netProfit / equity * 100
					}
					if roe > 0 {
						trailingEPS = bps0 * (roe / 100)
					}
				}
			}
			if trailingEPS > 0 {
				epsSeq = append(epsSeq, trailingEPS)
				yearLabels = append(yearLabels, fmt.Sprintf("%d", time.Now().Year()))
			}
		}
		// 如果预测年份不足6年，用最后一年增长率外推（默认增长率 10% -> 5%）
		growthRates := []float64{0.10, 0.10, 0.08, 0.05, 0.05, 0.05}
		for len(epsSeq) < 6 {
			last := 0.0
			if len(epsSeq) > 0 {
				last = epsSeq[len(epsSeq)-1]
			}
			g := growthRates[len(epsSeq)%len(growthRates)]
			if last > 0 {
				epsSeq = append(epsSeq, last*(1+g))
			} else {
				epsSeq = append(epsSeq, 0)
			}
			// 外推年份
			if len(yearLabels) > 0 {
				if lastY, err := strconv.Atoi(yearLabels[len(yearLabels)-1]); err == nil {
					yearLabels = append(yearLabels, fmt.Sprintf("%d", lastY+1))
				} else {
					yearLabels = append(yearLabels, fmt.Sprintf("%d", time.Now().Year()+len(yearLabels)))
				}
			} else {
				yearLabels = append(yearLabels, fmt.Sprintf("%d", time.Now().Year()+len(yearLabels)))
			}
		}

		ke := rimData.Rf + rimData.Beta*rimData.RmRf
		gTerminal := 0.05
		price := quoteData.CurrentPrice
		if price <= 0 && extRIM != nil {
			price = extRIM.Price
		}

		if bps0 > 0 && ke > gTerminal {
			params := analyzer.RIMParams{
				BPS0:         bps0,
				KE:           ke,
				GTerminal:    gTerminal,
				Forecast:     analyzer.RIMForecast{EPS: epsSeq, Years: yearLabels},
				CurrentPrice: price,
			}
			rimData.Params = params
			rimData.Result = analyzer.CalculateMultiPeriodRIM(params)
			if rimData.Result != nil {
				rimData.HasData = true
			}
		}
		if !rimData.HasData {
			rimData = nil
		}
	}

	// 获取用户设置的风险敏感度
	sensitivity := analyzer.SensitivityStandard
	if s := a.getRiskSensitivity(); s != "" {
		sensitivity = analyzer.SensitivityLevel(s)
	}

	report, err := analyzer.RunAnalysis(a.storage.DataDir(), symbol, analyzer.AnalysisOptions{
		Comparable:  compAnalysis,
		Quote:       quoteData,
		Sentiment:   sentimentData,
		Policy:      policyData,
		Technical:   technicalData,
		Activity:    activityData,
		Moneyflow:   moneyflowData,
		ML:          mlData,
		RIM:         rimData,
		Extras:      finData.Extras,
		External:    externalRiskData,
		Sensitivity: sensitivity,
	})
	if err != nil {
		return nil, err
	}
	// 计算与上次分析的差异
	if prevReport, _ := a.storage.LoadSnapshot(symbol); prevReport != nil {
		diff := analyzer.ComputeAnalysisDiff(report, prevReport)
		report.Diff = diff
		// 重新生成包含 diff 的 Markdown 报告
		var industryComp *analyzer.IndustryComparison
		if policyData != nil && policyData.Industry != "" {
			industryComp = analyzer.CompareWithIndustry(policyData.Industry, report.StepResults, report.Years[0])
		}
		report.MarkdownContent = analyzer.GenerateMarkdown(symbol, report.Years, report.StepResults, report.ScoreDetails, compAnalysis, industryComp, quoteData, sentimentData, policyData, technicalData, activityData, moneyflowData, mlData, rimData, report.RiskAlert, report.QualityWarnings, diff, report.QuarterlyAlert, report.TTMMetrics)
	}
	// 自动保存报告到本地
	_, _ = a.storage.SaveReport(symbol, report.MarkdownContent, overwriteLatest)
	// 保存分析快照（用于前端亮点与风险恢复）
	_ = a.storage.SaveSnapshot(symbol, report)
	// 保存K线数据缓存（供报告图表展示使用）
	if len(klines) > 0 {
		// 如果K线数据里没有换手率，用 quote 数据补算
		hasTurnover := false
		for _, k := range klines {
			if k.TurnoverRate > 0 {
				hasTurnover = true
				break
			}
		}
		if !hasTurnover && quoteData != nil && quoteData.CirculatingMarketCap > 0 && quoteData.CurrentPrice > 0 {
			circulatingShares := quoteData.CirculatingMarketCap / quoteData.CurrentPrice
			for i := range klines {
				klines[i].TurnoverRate = (klines[i].Volume * 100 / circulatingShares) * 100
			}
			debugLog("[AnalyzeStock] %s computed turnoverRate for %d klines, first=%.2f%%", symbol, len(klines), klines[0].TurnoverRate)
		}
		debugLog("[AnalyzeStock] %s saving klines cache, len=%d, first turnoverRate=%.2f", symbol, len(klines), klines[0].TurnoverRate)
		if err := a.storage.SaveStockKlines(symbol, "daily", klines); err != nil {
			debugLog("[AnalyzeStock] %s save klines cache error: %v", symbol, err)
		} else {
			debugLog("[AnalyzeStock] %s klines cache saved", symbol)
		}
	} else {
		debugLog("[AnalyzeStock] %s no klines to cache", symbol)
	}
	// 保存分析缓存
	if hash, err := a.storage.ComputeDataHash(symbol); err == nil {
		if compHash, err := a.storage.ComputeComparablesHash(symbol); err == nil {
			_ = a.storage.SaveAnalysisCache(symbol, hash, compHash)
		}
	}
	debugLog("[AnalyzeStock] Analysis completed successfully for %s", symbol)
	return report, nil
}

// GetReportHistory 获取某只股票的历史报告文件名列表

func computeMLConfidence(finData *analyzer.FinancialData, klines []downloader.KlineData, sentiment *analyzer.SentimentData, quote *analyzer.QuoteData) string {
	if finData == nil {
		return "low"
	}

	// Engine B 置信：基于财务数据年份数
	bConfidence := "low"
	if len(finData.Years) >= 5 {
		bConfidence = "high"
	} else if len(finData.Years) >= 3 {
		bConfidence = "medium"
	}

	// Engine D 置信：基于行情数据完整度
	dConfidence := "low"
	if quote != nil {
		validCount := 0
		if quote.PE > 0 {
			validCount++
		}
		if quote.PB > 0 {
			validCount++
		}
		if quote.MarketCap > 0 {
			validCount++
		}
		if quote.TurnoverRate > 0 {
			validCount++
		}
		if validCount >= 3 {
			dConfidence = "high"
		} else if validCount >= 1 {
			dConfidence = "medium"
		}
	}

	// Engine A 置信：基于 K线+舆情
	aConfidence := "low"
	if len(klines) >= 16 {
		if sentiment != nil && sentiment.HasData {
			aConfidence = "high"
		} else {
			aConfidence = "medium"
		}
	}

	// 综合判定：取最差有效等级
	scores := make(map[string]int)
	scores[bConfidence]++
	scores[dConfidence]++
	// Engine A 只在有足够 K线时才参与判定
	if len(klines) >= 16 {
		scores[aConfidence]++
	}

	if scores["low"] > 0 {
		return "low"
	}
	if scores["medium"] > 0 {
		return "medium"
	}
	return "high"
}


// RecommendComparables 自动推荐可比公司（Wails 绑定）
