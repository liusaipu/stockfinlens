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
	return fmt.Sprintf(`你是一位资深的 A 股投研分析师，擅长结合公开信息、产业链逻辑和行业常识，输出有洞察力的结构化投研观点。

任务要求：
1. 综合用户提供的搜索结果与你对该公司及所在行业的理解，生成一份结构化投研报告。
2. 报告必须围绕以下 6 个维度展开：
   - 产品/业务催化剂：公司自身的新产品投产、重大订单、技术突破、产能释放、涨价等；
   - 政策与行业影响：影响公司营收的宏观政策、行业监管、补贴、关税、供需变化；
   - 风险事件与监管处罚：证监会/交易所立案调查、行政处罚、*ST/ST 风险、财务造假、退市风险、问询函、监管函、大股东违规、诉讼仲裁等；
   - 全球产业映射与预期差：海外龙头公司（如 Nvidia、AMD、台积电、美光、SK 海力士、OpenAI、Anthropic、Google、微软、特斯拉/xAI、ASML、三星、苹果 等）的重大动向、技术突破、财报指引，以及这些因素如何映射到国内相关产业链和标的上；
   - 国际对标：公司与美国、日本、欧洲或其他国际市场的主要竞争对手在估值、技术、市场份额上的对比；
   - 社交情绪摘要：投资者社区（雪球、股吧、Reddit、X/Twitter 等）对该股票的关注焦点和情绪倾向。
3. 每个维度输出：
   - title: 模块标题
   - summary: 2-4 句话综合概述，直接给出你的判断，不要总是以"搜索结果未..."开头
   - key_points: 3-5 条关键要点，简洁有力
   - sentiment: 整体情绪，只能取 "positive"、"neutral"、"negative" 之一
4. 写作风格：
   - 直接输出分析观点，不要先道歉或反复强调"搜索未提及"；
   - 当搜索结果可直接支撑结论时，基于搜索结果陈述；
   - 当搜索结果不足时，可基于行业常识和产业链逻辑进行合理推断，但需在相关要点中标注"基于行业常识推断"；
   - 严禁编造具体的订单金额、客户名称、财务数据、未经验证的监管事件。
5. 特别重要：
   - 分析「风险事件与监管处罚」时，必须识别证监会、交易所、公司公告中提到的立案调查、行政处罚、*ST/ST、财务造假、退市风险、问询函、监管函等；若存在此类风险，必须明确说明，不能遗漏。
   - 分析「全球产业映射与预期差」时，必须回答：海外发生了什么产业级突破或龙头动向？该事件对国内产业链/该公司是利好还是利空？市场是否已经充分定价？是否存在预期差？
6. 来源引用要求：
   - sources 数组中只保留高可信度来源（证监会/交易所官网、主流财经媒体、上市公司公告、知名国际媒体/科技媒体、主流券商研报）。
   - 不要引用色情、赌博、广告推广、无关社交媒体 spam、分类信息站等低质量来源。
   - 如果搜索结果中没有可信来源支撑某个结论，宁可不列出该来源，也不要编造 URL。
7. 输出严格为 JSON 格式，不要包含 markdown 代码块、注释或额外说明；
   - JSON 字符串值中严禁出现真实换行或回车，必须使用 \n 转义；
   - JSON 字符串值中严禁出现未转义的双引号 "，如需引用请使用单引号 ' 替代；
   - 输出前请自检 JSON 是否合法。
8. 语言：%s。

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
	if len(results) == 0 || countSearchItems(results) == 0 {
		b.WriteString("（本次未检索到有效公开信息，请基于你对该公司及所在行业的专业知识进行分析。）\n\n")
	} else {
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

	// 查询尽量简短，提升 Tavily 召回率；股票名称+代码保证主体聚焦
	queries := []string{
		fmt.Sprintf("%s %s 订单 产能 催化剂", stockName, symbol),
		fmt.Sprintf("%s %s 政策 补贴 关税 影响", stockName, symbol),
	}

	// 风险事件与监管处罚查询
	queries = append(queries, fmt.Sprintf("%s %s 证监会 处罚 立案 财务造假 ST", stockName, symbol))
	queries = append(queries, fmt.Sprintf("%s %s 问询函 监管函 退市风险", stockName, symbol))

	// 全球产业映射：股票自身 + 海外龙头独立查询，扩大素材面
	queries = append(queries, fmt.Sprintf("%s %s 产业链 全球映射 预期差", stockName, symbol))
	queries = append(queries, fmt.Sprintf("Nvidia 特斯拉 苹果 产业链 中国映射 2025 2026"))

	// 行业与竞争格局通用查询，提升对中小市值公司的召回
	queries = append(queries, fmt.Sprintf("%s %s 行业分析 竞争格局 主营业务 下游需求", stockName, symbol))
	queries = append(queries, fmt.Sprintf("%s %s 券商研报 深度分析 投资逻辑", stockName, symbol))

	// 国际对标查询
	regionLabels := regionNames(focusRegions)
	if len(regionLabels) > 0 {
		queries = append(queries, fmt.Sprintf("%s %s 竞争对手 对标 %s", stockName, symbol, strings.Join(regionLabels, " ")))
	} else {
		queries = append(queries, fmt.Sprintf("%s %s 竞争对手 对标 美国 日本 欧洲", stockName, symbol))
	}

	if enableSocial {
		queries = append(queries, fmt.Sprintf("%s %s 雪球 股吧 讨论 情绪", stockName, symbol))
	}

	return queries
}

// IncludeDomains 返回 Tavily 搜索的 include_domains 白名单。
// 实践证明严格白名单会严重限制中文财经新闻的召回；现不再向 Tavily 传递白名单，
// 改为全网搜索 + 本地 isQualitySource / SpamDomainKeywords 过滤，最大化有效素材。
func IncludeDomains() []string {
	return nil
}

// SpamDomainKeywords 返回用于本地过滤的域名/URL 垃圾关键词。
func SpamDomainKeywords() []string {
	return []string{
		"escort",
		"massage",
		"dating",
		"casino",
		"poker",
		"lottery",
		"bet365",
		"counterfeit",
		"replica",
		"fake-",
		"knockoff",
		"directory",
		"classifieds",
		"backpage",
	}
}

// ValidExcludeDomains 返回可传给 Tavily exclude_domains 参数的合法域名。
// Tavily 要求必须是带有效后缀的域名（如 example.com），不能是关键词。
func ValidExcludeDomains() []string {
	return []string{
		// 已知低质量/聚合/广告域名，可随实际 case 持续补充
	}
}

// SpamKeywords 返回用于过滤搜索结果的垃圾关键词。
func SpamKeywords() []string {
	return []string{
		"兼职上门",
		"小姐",
		"外围",
		"高端上门",
		"薇信",
		"微信",
		"高仿",
		"A货",
		"复刻",
		"原单",
		"一比一",
		"精仿",
		"顶级高仿",
		"赌",
		"博彩",
		"彩票",
		"六合彩",
		"百家乐",
		"色情",
		"援交",
		"约炮",
		"成人",
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
