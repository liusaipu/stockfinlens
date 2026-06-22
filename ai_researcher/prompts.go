package ai_researcher

import (
	"fmt"
	"strings"
)

// SystemPrompt 返回系统提示词
func SystemPrompt(language string) string {
	if language == "" {
		language = "zh-CN"
	}
	return fmt.Sprintf(`你是一位专业的 A 股投研分析师，擅长从全球产业突破与国内映射的角度挖掘预期差，并对风险事件保持高度敏感。

任务要求：
1. 基于用户提供的搜索结果，生成一份结构化投研报告。
2. 报告必须围绕以下 6 个维度展开：
   - 产品/业务催化剂：公司自身的新产品投产、重大订单、技术突破、产能释放、涨价等；
   - 政策与行业影响：影响公司营收的宏观政策、行业监管、补贴、关税、供需变化；
   - 风险事件与监管处罚：证监会/交易所立案调查、行政处罚、*ST/ST 风险、财务造假、退市风险、问询函、监管函、大股东违规、诉讼仲裁等；
   - 全球产业映射与预期差：海外龙头公司（如 Nvidia、AMD、台积电、美光、SK 海力士、OpenAI、Anthropic、Google、微软、特斯拉/xAI、ASML、三星、苹果 等）的重大动向、技术突破、财报指引，以及这些因素如何映射到国内相关产业链和标的上；
   - 国际对标：公司与美国、日本、欧洲或其他国际市场的主要竞争对手在估值、技术、市场份额上的对比；
   - 社交情绪摘要：投资者社区（雪球、股吧、Reddit、X/Twitter 等）对该股票的关注焦点和情绪倾向。
3. 每个维度输出：
   - title: 模块标题
   - summary: 2-4 句话综合概述
   - key_points: 3-5 条关键要点，每条用简洁的一句话说明
   - sentiment: 整体情绪，只能取 "positive"、"neutral"、"negative" 之一
4. 特别重要：
   - 分析「风险事件与监管处罚」时，必须识别证监会、交易所、公司公告中提到的立案调查、行政处罚、*ST/ST、财务造假、退市风险、问询函、监管函等；
   - 如果存在 *ST/ST 或财务造假等风险，必须在summary和key_points中明确说明，不能遗漏；
   - 分析「全球产业映射与预期差」时，必须回答：海外发生了什么产业级突破或龙头动向？该事件对国内产业链/该公司是利好还是利空？市场是否已经充分定价？是否存在预期差？
   - 如果信息不足，明确说明"信息有限"，不要编造。
5. 所有结论必须基于提供的搜索结果，不要编造数据。
6. 输出严格为 JSON 格式，不要包含 markdown 代码块、注释或额外说明。
7. 语言：%s。

JSON Schema：
{
  "sections": [
    {
      "title": "string",
      "summary": "string",
      "key_points": ["string"],
      "sentiment": "positive|neutral|negative"
    }
  ],
  "sources": [
    {
      "title": "string",
      "url": "string",
      "date": "YYYY-MM-DD"
    }
  ]
}
`, languageName(language))
}

// UserPrompt 根据搜索结果构造用户提示词
func UserPrompt(symbol, name string, focusRegions []string, enableSocial bool, results []SearchResult) string {
	var b strings.Builder
	if name != "" {
		b.WriteString(fmt.Sprintf("请分析 A 股股票 %s（%s）。\n\n", name, symbol))
	} else {
		b.WriteString(fmt.Sprintf("请分析 A 股股票 %s。\n\n", symbol))
	}

	b.WriteString("分析重点：\n")
	b.WriteString("1. 必须识别证监会、交易所对该公司的立案调查、行政处罚、*ST/ST、财务造假、退市风险、问询函、监管函等风险事件；\n")
	b.WriteString("2. 请特别关注海外龙头公司（Nvidia、AMD、台积电、美光、SK 海力士、OpenAI、Anthropic、Google、微软、特斯拉/xAI、ASML、三星、苹果 等）以及马斯克相关动向对国内产业链的映射影响，挖掘预期差。\n\n")

	if len(focusRegions) > 0 {
		b.WriteString("重点关注国际市场：")
		b.WriteString(strings.Join(regionNames(focusRegions), "、"))
		b.WriteString("。\n\n")
	}

	if !enableSocial {
		b.WriteString("注意：本次分析不抓取社交情绪数据，请仅基于新闻和公告信息进行分析。\n\n")
	}

	b.WriteString("===== 搜索结果 =====\n\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("【查询】%s\n", r.Query))
		for _, item := range r.Items {
			b.WriteString(fmt.Sprintf("标题：%s\n", item.Title))
			b.WriteString(fmt.Sprintf("链接：%s\n", item.URL))
			if item.Published != "" {
				b.WriteString(fmt.Sprintf("日期：%s\n", item.Published))
			}
			content := item.Content
			if len(content) > 800 {
				content = content[:800] + "..."
			}
			b.WriteString(fmt.Sprintf("摘要：%s\n", content))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("请直接输出 JSON。")
	return b.String()
}

// BuildQueries 根据 symbol/name 构造 Tavily 搜索查询列表
func BuildQueries(symbol, name string, focusRegions []string, enableSocial bool, recencyDays int) []string {
	stockName := name
	if stockName == "" {
		parts := strings.Split(symbol, ".")
		if len(parts) > 0 {
			stockName = parts[0]
		}
	}

	queries := []string{
		fmt.Sprintf("%s %s 新产品 投产 订单 中标 产能 股价催化", stockName, symbol),
		fmt.Sprintf("%s %s 政策影响 行业监管 补贴 营收 关税", stockName, symbol),
	}

	// 风险事件与监管处罚查询（A 股重点）
	queries = append(queries, fmt.Sprintf("%s %s 证监会 处罚 立案调查 财务造假 *ST ST", stockName, symbol))
	queries = append(queries, fmt.Sprintf("%s %s 交易所 问询函 监管函 退市风险 警示", stockName, symbol))

	// 全球产业映射查询：海外龙头 + 国内映射
	queries = append(queries, fmt.Sprintf("%s %s 全球产业映射 Nvidia OpenAI 马斯克 国内映射 预期差", stockName, symbol))
	queries = append(queries, fmt.Sprintf("%s %s 半导体芯片 AI算力 海外龙头 技术突破 产业链", stockName, symbol))

	// 国际对标查询
	regionLabels := regionNames(focusRegions)
	if len(regionLabels) > 0 {
		queries = append(queries, fmt.Sprintf("%s %s 竞争对手 对标 %s 国际市场 估值", stockName, symbol, strings.Join(regionLabels, " ")))
	} else {
		queries = append(queries, fmt.Sprintf("%s %s 竞争对手 对标 美国 日本 欧洲 国际市场", stockName, symbol))
	}

	if enableSocial {
		queries = append(queries, fmt.Sprintf("%s %s 雪球 股吧 讨论 情绪 机构调研", stockName, symbol))
	}

	return queries
}

// IncludeDomains 返回金融/社交/科技网站域名白名单
func IncludeDomains() []string {
	return []string{
		// A 股/港股金融
		"eastmoney.com",
		"xueqiu.com",
		"finance.sina.com.cn",
		"cninfo.com.cn",
		"cs.com.cn",
		"hexun.com",
		"stcn.com",
		"cls.cn",
		"jiemian.com",
		"caixin.com",
		"wallstreetcn.com",
		"gelonghui.com",
		"10jqka.com.cn",
		// 监管/交易所官方
		"csrc.gov.cn",
		"szse.cn",
		"sse.com.cn",
		"bse.cn",
		// 国际市场/科技
		"bloomberg.com",
		"reuters.com",
		"yahoo.com",
		"marketwatch.com",
		"seekingalpha.com",
		"investing.com",
		"ft.com",
		"nikkei.com",
		"techcrunch.com",
		"theverge.com",
		"ars technica.com",
		"wired.com",
		"cnbc.com",
		"semianalysis.com",
		"tomshardware.com",
		"anandtech.com",
		// 公司/机构官方
		"nvidia.com",
		"openai.com",
		"anthropic.com",
		"microsoft.com",
		"google.com",
		"tesla.com",
		"x.ai",
		"tsmc.com",
		"samsung.com",
		"asml.com",
		"micron.com",
		"skhynix.com",
		// 社交/社区
		"reddit.com",
		"twitter.com",
		"x.com",
	}
}

func languageName(code string) string {
	switch code {
	case "zh-CN", "zh":
		return "简体中文"
	case "zh-TW":
		return "繁体中文"
	case "en":
		return "English"
	default:
		return "简体中文"
	}
}

func regionNames(codes []string) []string {
	m := map[string]string{
		"us": "美国", "jp": "日本", "eu": "欧洲",
		"hk": "香港", "kr": "韩国", "tw": "台湾",
	}
	out := make([]string, 0, len(codes))
	for _, c := range codes {
		if name, ok := m[strings.ToLower(c)]; ok {
			out = append(out, name)
		}
	}
	return out
}
