# Changelog

## [v1.8.9] - 2026-07-05

### 修复 (Fixes)
- **修复 AI 投研 LLM HTTP 响应解析失败**（`ai_researcher/llm.go`）
  - 某些模型返回的 HTTP 响应体中 `content`/`reasoning_content` 字段含未转义真实换行，导致 `invalid character '\n' in string literal`。
  - 对 LLM API 响应体也应用 `sanitizeJSON` 与兜底空白规范化，修复后立即解析。
- **修复 AI 投研报告 JSON 多种转义异常**（`ai_researcher/researcher.go`）
  - 增强字符串值内裸换行、反斜杠+换行、Tab 等控制字符的自动转义。
  - 改进 JSON 对象提取逻辑，按大括号深度匹配，避免字符串内 `}` 被误当对象结尾。
  - 解析失败时返回最后错误并附带原始响应摘要，方便排查。

### 优化 (Improvements)
- **放宽 AI 投研搜索召回条件**（`ai_researcher/prompts.go`, `ai_researcher/config.go`, `ai_researcher/researcher.go`）
  - 移除 Tavily `include_domains` 白名单，改为全网搜索 + 本地质量过滤，提升中文财经新闻召回率。
  - 简化搜索查询语句，避免关键词堆砌降低 Tavily 召回效果。
  - 增加行业/产业链/券商研报通用查询，改善中小市值股票素材覆盖。
  - `MaxResults` 默认从 10 提升到 20；质量分数阈值从 0.35 放宽到 0.20。
  - `isRelevant` 相关性过滤放宽：未命中股票名/代码时，允许查询主题关键词或高置信度行业词命中。
- **优化 AI 投研提示词**（`ai_researcher/prompts.go`）
  - 明确允许 LLM 结合行业常识与产业链逻辑进行推断，减少每段都写“搜索结果未提及”的情况。
  - 要求直接输出分析观点，不要以“搜索未...”开头。
  - 在 SystemPrompt 中强制要求 JSON 字符串值内禁止真实换行。

### 测试 (Tests)
- 新增 `TestExtractJSONObject`、`TestParseLLMOutputWithBackslashNewline`、`TestParseLLMOutputWithTabInString`。
- 扩展 `TestSourceFilterRelevance`、`TestBuildQueries` 覆盖放宽后的逻辑。

## [v1.8.8] - 2026-07-04

### 修复 (Fixes)
- **修复 AI 投研偶发解析失败**（`ai_researcher/researcher.go`）
  - 某些模型返回的 JSON 字符串值内部包含未转义的真实换行，导致 `invalid character '\n' in string literal`。
  - 新增 `escapeNewlinesInJSONStrings`，在字符串值内部自动把裸换行/回车转义为 `\n` 后再解析。
- **修复港股财报脚本 macOS Resources 路径回退缺失**（`downloader/hk_financials.go`）
  - 打包后 `.app/Contents/Resources` 路径未被检查，导致部分用户无法下载港股财报。
- **修复删除报告时 RIM 缓存未清理**（`storage.go`, `rim.go`）
  - 删除个股报告后同步清理对应 RIM 估值缓存，避免旧估值被复用。

### 新增 (Features)
- **AI 投研参考来源支持一键复制链接**（`frontend/src/AIResearchPanel.tsx`）
  - 每条参考来源前增加 📎 回形针图标，点击即可复制 URL 到剪贴板。
  - 优先调用 Go 后端 `ClipboardSetText`，兼容 `navigator.clipboard` 与 `execCommand` 兜底。
  - 提示条跟随鼠标下方显示，1.5 秒后自动消失。
- **AI 投研报告导出增加「拷贝到剪贴板」选项**（`frontend/src/App.tsx`）
  - 导出菜单新增 TXT / Markdown 复制，无需先保存文件。

### 优化 (Improvements)
- **选中港股时隐藏首页资金流向卡片**（`frontend/src/App.tsx`）
  - 港股无 A 股式资金流向数据，避免展示空/错误状态。
- **README 增加 AI 投研助手截图**（`README.md`, `docs/screenshots/ai-research.png`）

## [v1.8.6] - 2026-07-03

### 修复 (Fixes)
- **修复 RIM EPS 预测数据源错误导致估值失真**
  - v1.8.5 修改后东财 `stock_profit_forecast_em` 返回子集不再包含部分股票（如中科三环 000970），导致 fallback 到低质量外推 EPS。
  - 改为优先从同花顺 `stock_profit_forecast_ths(indicator="业绩预测详表-详细指标预测")` 获取 BPS 与 ROE，推导 EPS = BPS × ROE。
  - 东财与同花顺双源获取，选择预测年数更多的结果；同时兼容代码列带/不带市场后缀。

## [v1.8.5] - 2026-07-02

### 修复 (Fixes)
- **修复 macOS 打包后 Python 脚本路径硬编码问题**
  - 多个 `scripts/` 下脚本路径解析函数（港股财报、港股资料、行业更新、政策更新、行业数据补全）未检查 macOS `.app` bundle 的 `Contents/Resources` 目录，导致打包后在用户机器上报 `can't open file /Users/lobster/...`。
  - 统一补充 `Contents/Resources` 路径回退。
- **修复港股财报下载超时**
  - 港股被显式排除在 StockFinLens Pro 数据源之外，只能走 akshare Python 脚本，且脚本无超时控制，导致前端长时间挂死后报"网络超时"。
  - 解除港股 SFL 数据源排除，让港股财报优先走 StockFinLens Pro；同时给 `fetch_hk_financials.py` / `fetch_hk_profile.py` 调用增加 20 秒超时，给 `DownloadReports` 整体增加 25 秒超时。
- **修复港股资金流向错误提示**
  - tushare `moneyflow`/`moneyflow_dc` 与东方财富资金流向接口均不支持港股，三个源失败后把东财 EOF 技术错误暴露给用户。
  - 改为 `DataRouter.FetchMoneyflow` 对港股直接返回空数据，`GetStockMoneyflow` 返回"港股暂无资金流向数据"，首页近 3 日资金流向卡片不再显示技术错误。
- **增强 RIM EPS 预测数据健壮性**
  - `fetch_rim_data.py` 改进列名匹配（兼容 `2024预测每股收益` / `2024年预测每股收益`），增加同花顺盈利预测 fallback，输出 `eps_forecast_source` / `eps_forecast_count` / `fetch_time` 元数据。
  - RIM 缓存校验更严格：EPS 预测年数少于 3 年时视为无效缓存，强制重新拉取。

### 文档 (Docs)
- **拆分压缩 `AGENTS.md`**
  - `AGENTS.md` 从 573 行压缩到约 90 行，仅保留 AI 助手核心约束。
  - 详细内容拆分到 `docs/ARCHITECTURE.md`、`docs/TECH_STACK.md`、`docs/TESTING.md`、`docs/RELEASE.md`。
  - `docs/ML_PREDICTION_DESIGN.md` 追加当前运行时模型清单附录。

## [v1.6.8] - 2026-06-17

### 修复 (Fixes)
- **季度滚动预警始终显示最近一期**（`analyzer/quarterly.go`, `analyzer/report_modules.go`）
  - 此前改为单季口径后，若最近一期变化未触发 warning/danger 阈值，模块 3.3 会直接消失。
  - 改为始终生成营业收入、净利润、毛利率、经营现金流的环比/同比条目，未触发阈值时标记为 `normal`。
  - 横表继续展示当前值、环比（vs 上一单季）、同比（vs 去年同期），对比期间一目了然。

## [v1.6.7] - 2026-06-17

### 优化 (Improvements)
- **季度滚动预警改为横表，只展示最近一期**（`analyzer/quarterly.go`, `analyzer/report_modules.go`, `analyzer/quarterly_test.go`）
  - `BuildQuarterlyAlert` 只取最近一个季度生成环比/同比，避免历史多期堆叠。
  - `QuarterlyAlertItem` 新增 `PreviousPeriod` 字段，明确记录对比期间。
  - 报告渲染改为横表：行=指标，列=当前值 / 环比（vs 上一单季） / 同比（vs 去年同期），对比期间一目了然。

## [v1.6.6] - 2026-06-16

### 修复 (Fixes)
- **季度滚动预警改为单季口径**（`analyzer/quarterly.go`, `analyzer/quarterly_test.go`）
  - 原实现把年报（12-31）排除在季度列表外，导致 Q1 的环比基准变成上一年 Q3，出现如道通科技 2026Q1 营收“环比下滑 62.8%”的误导结果。
  - 改为先把累计 YTD 数据还原为单季度值：Q1 直接使用；Q2/Q3/Q4 分别减去上一季度累计值。
  - 环比/同比均基于单季值计算，与雪球等主流平台口径一致。
  - 新增 `analyzer/quarterly_test.go` 覆盖单季还原、相邻季度推算、环比/同比基准选取。

## [v1.6.5] - 2026-06-16

### 新增 (Features)
- **K 线图交互增强**（`frontend/src/UnifiedChart.tsx`）
  - 周期选项（分时/日线/周线/月线）从下拉框改为平铺按钮，放到刷新按钮右侧。
  - K 线图内部上方新增均线数值标签（MA5/MA10/MA30/MA180 等），颜色与图表均线保持一致。
- **日线/周线/月线合并当日实时行情**（`frontend/src/UnifiedChart.tsx`）
  - 当行情接口能获取到当日数据时，自动把当日 open/high/low/close/volume/amount/turnoverRate 合并进日线序列。
  - 周线/月线由日线聚合，因此会自动包含当日数据，保持与雪球等平台一致。

### 优化 (Improvements)
- **默认窗口大小调整**（`main.go`）
  - 应用启动默认窗口从 `1280x800` 调整为 `1600x900`。
- **K 线右侧留白**（`frontend/src/UnifiedChart.tsx`）
  - 当交易日期数不足占满图表时，坐标轴保持固定刻度位置，K 线条保持默认宽度，右侧自然留白，不再自动拉宽。

## [v1.6.4] - 2026-06-08

### 新增 (Features)
- **中栏 UI 重构 + 资金流向卡片化**（`frontend/src/App.tsx`, `frontend/src/App.css`）
  - 股票名称栏改为固定顶部 topbar（与右栏 `.report-tabs` 对齐），"K线"/"财务趋势"两个动作按钮从底部 footer 上移到 topbar 右侧，主要操作触手可及。
  - `.info-panel` 拆出 `.info-panel-scroll` 子滚动容器，topbar 在外、内容可滚动；`scrollbar-gutter: stable` 永远预留 6px 滚动条空间避免显隐切换抖动。
  - 资金流向独立成卡片，「近 N 个交易日资金流向」与「当日流向」分别可折叠（默认全部展开）；当日流向展开后显示 `更新于 HH:MM:SS` 时间戳。
  - 当日流向**自动刷新**：已选股票 + 卡片展开 + 页面可见 + 本地处于 A 股交易时段（09:30–11:30 / 13:00–15:00，周末跳过）时每 60s 拉一次，展开瞬间立刻拉一次；非交易时段不发请求。
- **本地反查表重建工具**（`cmd/rebuild_membership/main.go`）
  - 新增 `go run ./cmd/rebuild_membership/` 命令，本地一键重建 `~/.config/stock-analyzer/_concept_membership.json`，配合 hot_concept 翻页修复使用。

### 修复 (Fixes)
- **TTM（滚动 12 个月）口径重写**（`analyzer/ttm.go`, `analyzer/report_modules.go`, `analyzer/ttm_test.go`）
  - A 股财报为「累计 YTD」口径——2025-09-30 行的"营业收入"已经是 1-9 月累计，旧版 `accumulateQuarters` 把最近 4 个季报直接相加导致重复计算。
  - 重写为标准公式：最新期是年报 → 直接取年报全年；最新期是季报 → `TTM = 上一年报 + 最新季报 − 去年同期`；公式所需数据缺失 → 降级为最近一份年报。
  - `TTMMetrics` 新增 `Mode`（annual / quarterly / annual-fallback）、`EndPeriod`、`Notes` 字段，每份 TTM 都能追溯到具体口径与依赖期间。
- **概念成分股 API 翻页修复**（`downloader/hot_concept.go`）
  - 原 `FetchAllConceptConstituents` 单次 `pz=500` 被东财 API 隐式 clamp 到 100 条，导致"智能穿戴"等真实 100+ 家的热门板块在反查表里只剩 100 家，IDF 算出来全是 ~3.76 分不出区分度。
  - 改为 `pn=1,2,3…` 翻页拉取（pageSize=100，单板块最多 30 页 ~3000 家），终止条件为本页不足 pageSize 或累计 ≥ API 返回 total。中途单页失败时返回已拿到部分，不整体失败。
- **季报权益数据修复扩展**（`analyzer/data.go`）
  - 此前 `fixMissingData` 只遍历 `Years`（年报）做"资产−负债 → 权益"兜底，季报期的权益字段为 0 时修复不到，导致基于季报的 ROE 计算异常。
  - 改为遍历 `Quarters`（覆盖年报 + 季报全部期间）。利润表的"归母净利润 = 净利润 − 少数股东损益"推导仍仅限年报，避免季报 YTD 与单季混淆。
- **数据保留：季报从 3 个提到 6 个**（`downloader/data_router.go`）
  - TTM 标准公式需要"去年同期"季报，最少要覆盖最近 2 年的 Q1/Q2/Q3，原 3 个不够；改为 `keepRecentNonAnnualQuarters = 6`，保证无论最新季报是 Q1/Q2/Q3 都能拿到去年同期，同时给季度环比/同比预警留 1 年缓冲。

### 优化 (Improvements)
- **可比公司推荐：概念匹配引入 IDF 加权 + 强概念叠加奖励**（`analyzer/recommend.go`, `analyzer/recommend_test.go`）
  - 概念命中按 `IDF = log(总股票数 / 该概念命中数)` 加权：稀有概念（"3D玻璃" 15 家 IDF≈5.66）权重显著高于泛主题（"智能穿戴" 100+ 家 IDF≈3.76），解决"长信(光学光电子) vs 蓝思(消费电子)"场景下面板厂(京东方/深天马)共享 5 个泛主题概念分高于玻璃厂(凯盛/宜安)共享 2 个业务概念的错配。
  - 共享 ≥3 个高 IDF 概念时叠加奖励（每个 4 分，封顶 20）。
  - 等价行业映射扩展到消费电子链（光学光电子 ↔ 消费电子 ↔ 元器件 ↔ 电子元件）和半导体材料链（电子化学品 ↔ 半导体材料 ↔ 化工原料），等价行业匹配分从 25 提到 30。
- **报告模块 7 移除冗余 K 线图占位**（`analyzer/report_modules.go`）
  - 删除"技术指标联动分析图（K线+成交量+MACD+RSI+布林带）"标题与 `chart-unified` 占位符；K 线现已通过中栏 topbar 的"K线"按钮全屏查看，避免一份图重复出现在报告与中栏。
  - TTM 段落说明文案同步更新为新口径。

### 测试 (Tests)
- **TTM 单元测试**（`analyzer/ttm_test.go`）：覆盖最新期为年报 / 季报 / 缺去年同期 / 缺上一年报四种分支，验证 Mode 与 EndPeriod 正确。
- **概念探针测试**（`analyzer/probe_test.go`）：辅助验证反查表加载与 DocFreq 计算。
- **推荐测试扩展**（`analyzer/recommend_test.go`）：新增 IDF 加权与强概念叠加奖励的回归用例。
- **冗余清理**（`app_test.go`）：删除已废弃的 13 行 debug 用例。

## [v1.6.3] - 2026-06-03

### 修复 (Fixes)
- **Python 子进程添加超时防止分析流程永久挂死**（`analyzer/ml_inference.go`, `downloader/rim_data.go`, `downloader/risk_crawler.go`, `downloader/auditor.go`, `downloader/exec_changes.go`, `downloader/litigation.go`）
  - 6 个 Python 子进程调用点全部改用 `exec.CommandContext` + `context.WithTimeout`：ML 推理 60s、RIM 数据 60s、风险爬虫 90s、审计/高管/诉讼各 60s。
  - 此前 akshare 底层 requests 无默认超时，网络异常时 `cmd.Output()` 永久阻塞，导致分析流程卡在 92%（生成报告中）永不返回。
- **可比公司综合得分改用固定档位法**（`analyzer/report_helpers.go`）
  - 各指标按 A 股通用固定档位映射到 0-100 分（ROE≥15%、毛利率≥40%、营收增长≥10%、负债率≤40% 视为优秀），替代原来的 Min-Max 池内标准化，避免加减可比公司导致相对排名翻转。

## [v1.6.2] - 2026-06-01

### 优化 (Improvements)
- **macOS 静默自更新**（`updater/updater_darwin.go`, `frontend/src/UpdateModal.tsx`）
  - 升级体验向 Windows 端对齐：点击「立即重启并安装」后，应用自动下载 → 退出 → 挂载 DMG → 替换 `.app` → 重启，全程 ~7s，无需手动拖拽到 Applications。
  - 替换流程：`hdiutil attach -nobrowse` 挂载 → `ditto` 拷贝到旁路 `.new` → 原子三步 `mv`（旧 → `.old` / `.new` → 正式 / 删 `.old`）→ `xattr -dr com.apple.quarantine` 清隔离 → `hdiutil detach` 卸载 → `open` 重启。失败时把 `.old` 回滚到原位，保证应用不会损坏。
  - helper 脚本通过 `setsid` 脱离主进程独立运行，等待主进程 PID 退出后再开始替换；日志写入 `~/.config/stock-analyzer/update/apply_update.log`，便于排错。
  - 防御：若应用运行自 `/Volumes/*`（DMG 内未拖入 Applications），拒绝替换并报错退出。
- **更新弹窗 UI 收敛**（`frontend/src/UpdateModal.tsx`）
  - 移除 macOS 专属的「已打开 DMG，请拖拽到 Applications」分支，按钮文案统一为「立即重启并安装」。
  - 安装中显示蓝色提示「正在安装新版本，应用即将自动重启…」。

## [v1.6.1] - 2026-05-30

### 新增 (Features)
- **互动平台问答展示**（`downloader/interact.go`, `analyzer/types.go`, `analyzer/report_modules.go`, `frontend/src/App.tsx`）
  - 模块 13.4 新增互动平台近期问答展示，支持深市互动易、东财问董秘、上证e互动三个数据源。
  - 自动去重、过滤 90 天内有回答的问答，按回答日期降序排列。
  - 前端默认显示 5 条，点击"查看更多..."每次展开 5 条，"收起"按钮（双箭头图标）回到 5 条。
  - 标题旁新增"刷新"按钮，支持实时获取最新问答数据，带加载状态和成功/失败通知。
  - 切换股票或刷新数据后自动重置显示数量为 5 条。

### 优化 (Improvements)
- **互动问答状态管理优化**（`frontend/src/App.tsx`）
  - 将显示数量状态提升到 App 组件，避免组件重新渲染时状态丢失。
  - 刷新按钮增加加载状态：按钮变灰、图标旋转、文字变为"刷新中..."，防止重复点击。

### 测试 (Tests)
- **互动问答测试覆盖**（`downloader/interact_test.go`）
  - 新增去重和过滤逻辑的单元测试，验证 90 天过滤、问题去重、空答案过滤等功能。

## [v1.6.0] - 2026-05-28

### 新增 (Features)
- **K 线新增「分时」周期**（`frontend/src/IntradayChart.tsx`, `frontend/src/UnifiedChart.tsx`）
  - K 线周期下拉新增「分时」选项，与日线/周线/月线并列；展示当日（或最近交易日）分时走势 + 均价线 + 成交量柱。
  - 完整 240 个时间 slot（09:30-11:30 + 13:00-15:00），午休断点用虚线分隔；价格轴以昨收为中心对称，右轴显示涨跌幅 %。
  - 成交量柱按「该分钟价 vs 上一根有数据的分钟价」着色（涨红跌绿），与雪球/同花顺一致。
  - 盘中自动每 60s 刷新（仅当数据为实时、页面可见、本地处于交易时段），切回前台立即刷一次。
- **分时数据后端双数据源**（`app.go`, `downloader/eastmoney_intraday.go`, `downloader/tencent_intraday.go`）
  - 首选东方财富 trends2，失败退回腾讯 minute/query；`GetIntradayMinutes` 经 `singleflight` 去重并发请求。
  - 内存缓存（`downloader/intraday_cache.go`）：盘中 60s TTL，盘后缓存到下一交易日盘前 9:25，避免重复请求。
  - 交易时段判断（`downloader/market_session.go`）：上海时区 + 周末跳过。

### 优化 (Improvements)
- **分时切换提速：东财 trends2 改为快速失败**（`downloader/eastmoney_intraday.go`）
  - trends2 的 EOF 是 path-specific 反爬（同 IP 下 K 线接口正常、trends2 被单方面 close），重试无意义。
  - 由默认 3 次指数退避（1s→2s→4s）改为 1 次尝试，失败立即退回腾讯。东财失败时的切换耗时从最坏 ~9s 降到 ~0.6s。
- **K 线 / 分时容器共存**（`frontend/src/UnifiedChart.tsx`）
  - K 线容器始终挂载、切到分时时用 `visibility: hidden` 隐藏（不 dispose echarts 实例，避免脱离 DOM）；分时图绝对定位覆盖，切回 K 线时整体 unmount 自动 dispose。

### 修复 (Fixes)
- **启动更新检查移除每日节流**（`app.go` `checkUpdateOnStartup`）
  - 此前用 `LastCheckDate == today` 做持久化节流，导致发布日已启动过的用户当天收不到新版本提示。
  - `checkUpdateOnStartup` 仅在 `startup()` 中调用一次，进程级「每启动一次」已足够避免过度请求；`LastCheckDate` 字段保留仅作展示/调试，不再参与节流判断。

## [v1.5.1] - 2026-05-27

### 新增 (Features)
- **审计机构「正常轮换」识别**（`scripts/fetch_auditor_history.py`, `analyzer/risk_alert.go`, `analyzer/types.go`, `frontend/src/components/RiskAlertBanner.tsx`）
  - 新增 `NORMAL_ROTATION_KEYWORDS`（招标/选聘/连续服务/不存在分歧/年限届满等），通过公告标题识别正常轮换。
  - 风险判定升级为四层：正常轮换 / 政策合规 / 被动更换 → `info` 信息提示（不进入风险警示）；年报披露期内更换或异常辞任 → 高风险一票否决；其他模糊原因 → 中风险。
  - 新增半年内（≤180 天）多次更换审计机构的频繁更换检测，触发一票否决。
  - 前端 `RiskAlertBanner` 支持蓝色信息样式与 hover tooltip，详细解释「为什么这次更换看似异常实则正常」。
  - 报告模块 1.0 审计意见段落追加正常轮换提示引用块。
- **模块 9.x ML 模型标题信息图标**（`frontend/src/App.tsx`）
  - 9.1 Engine-B / 9.2 Engine-A / 9.3 Engine-D 标题右侧添加 ℹ️ 图标，hover 弹出模型原理、如何理解结果、适用场景与局限。
- **可比公司「全添加」按钮**（`frontend/src/App.tsx`）
  - 推荐结果右侧新增「全添加 +」按钮，一键将所有推荐可比公司加入清单（受 7 个上限和已添加状态约束）。

### 优化 (Improvements)
- **Engine-D 风险预警模型修复**（`analyzer/ml_features.go`, `ml_models/inference.py`, `ml_models/engine_d_risk/train.py`）
  - 用真实 Beneish M-Score 与综合 A-Score 替换原伪 mscore（`-accruals/totalAssets*5`，几乎永远触发 `>-2.22` 阈值）与 6 规则代理 ascore，复用 `step8RiskAnalysis` 输出。
  - `gm_risk` 从对称的 `|gm-0.30|`（同时惩罚高/低毛利）改为单边的 YoY 跌幅小数，高毛利公司（军工/科技）不再被误判。
  - 训练侧 `gm_risk` 合成分布同步调整（健康 `max(0, N(0, 0.02))`，风险 `max(0, N(0.08, 0.06))`），模型重训。
  - `top_factors` 从「全局 `feature_importances_` top3」（对每只股票输出相同结果）改为 per-sample 归因：`(value-healthy_mean)/healthy_std × importance` 排序取 top3，附带 `(偏高)/(偏低)` 方向标签。
  - 训练时新增 `model/feature_stats.json` sidecar 保存健康均值/方差/方向/importance，供推理时归因使用。
  - 验证：振华科技-like 输入风险概率从 **88.9% 高风险** → **37.7% 低风险**；真造假股-like 输入仍为 **100% 高风险**。
- **UpdateModal 改 Portal 渲染**（`frontend/src/UpdateModal.tsx`）
  - 弹窗通过 `createPortal(..., document.body)` 挂到 body，`zIndex: 99999`，避免被 ECharts 等高层级 DOM 遮挡。
- **资金流向说明位置调整**（`frontend/src/App.tsx`）
  - 「主力 = 超大单 + 大单」从表格底部小字改为标题旁 ⓘ hover 提示，节省纵向空间。

### 文档 (Docs)
- **K-line / Chart Conventions**（`CLAUDE.md`）
  - 沉淀三类高频复发约定：日期格式统一 `YYYY-MM-DD` + 必须经 `normalizeTime`；禁用 `toISOString()` 解析日 K 时间戳；全屏覆盖层走 React Portal + `z-index: 99999`；多分支组件（loading/empty/main）必须统一更新。

## [v1.5.0] - 2026-05-24

### 新增 (Features)
- **K线图周期切换** (`frontend/src/UnifiedChart.tsx`)
  - 主图周期一键切换：**日线 / 周线 / 月线**，右上角下拉选择
  - 始终拉取日线数据，前端按自然周（周一~周日）/ 自然月聚合 OHLC、成交量、成交额；切换周期无需重新请求后端
  - 周线聚合用 `split('-')` 安全解析 `YYYY-MM-DD`，避开 `toISOString()` 时区偏移与 `Invalid Date` 风险
- **K线均线自定义** (`frontend/src/UnifiedChart.tsx`)
  - 1~6 条均线全自由配置，每条周期范围 1~250 日
  - 右上角齿轮按钮打开设置弹窗：选择条数 + 逐条调周期 + 一键恢复默认（MA5/10/30/60）
  - 配置写入 `localStorage.unifiedChart_maConfig`，跨会话保留

### 优化 (Improvements)
- **K 线刷新流程**（消除"两条长条"闪烁）
  - chart 实例改为**只在 mount/unmount 创建一次**，数据/配置变化只走 `chart.setOption(option, true)` 热更新，避免每次 dispose+init 之间露出深色背景 + 默认 axisPointer 占位帧
  - 刷新期间用 React Portal 提到 `document.body` 层、`zIndex: 99999` 的不透明遮罩盖住整个重绘过程；解除时机由 echarts `finished` 事件触发（含 5s 兜底超时），避免 React 18 自动 batching 让 `setRefreshing(false)` 与 `setRawData()` 同帧 commit、`setOption` 中间帧外露
  - 用 `onCloseRef` 稳定回调，防止 mount-effect 因 prop 变化重建
- **K 线 UI 微调**
  - legend 与右上角控件留出间距、控件尺寸统一
  - 状态覆盖层文案区分加载中 / 暂无数据

### 修复 (Fixes)
- **刷新后切到周线显示"暂无K线数据"** (`frontend/src/UnifiedChart.tsx` `handleRefresh`)
  - `RefreshStockKlines` 返回的列表此前未走 `normalizeTime`，腾讯 `YYYYMMDD` / 网易 `YYYY/MM/DD` 格式不符合 `aggregateToWeekly` 的 `YYYY-MM-DD` 严格解析（`parts.length !== 3` 直接跳过），导致刷新后周线全部样本被丢弃。现在与 `GetStockKlines` 初次加载路径保持一致：刷新返回的 `time` 字段统一归一化为 `YYYY-MM-DD`。
- **跨数据源日期格式不一致** (`frontend/src/UnifiedChart.tsx` `normalizeTime`)
  - 统一处理腾讯 `YYYYMMDD` / 东财 `YYYY-MM-DD` / 网易 `YYYY/MM/DD`，下游聚合/标签/坐标轴一律按 `YYYY-MM-DD` 走

## [v1.4.0] - 2026-05-24

### 新增 (Features)
- **TTM 累加期间显示** (`analyzer/ttm.go`, `frontend/wailsjs/go/models.ts`)
  - `TTMMetrics` 新增 `Periods []string` 字段，记录实际累加的报告期。
  - 模块 3.4 TTM 报告顶部展示累加期间清单（如 `2025-09-30 + 2025-06-30 + 2025-03-31 + 2024-12-31`），不足 4 期时标注「TTM 口径不完整」，无季报数据则退化为年报口径并提示。
  - `BuildTTMMetrics` 把传入累加函数的报告期由降序改为升序，避免最新一期 BPS/equity 取错。
- **季度报告期保留与导入提示** (`downloader/data_router.go`, `app.go`, `frontend/src/App.tsx`)
  - SFL 数据源在 `ConvertToFinancialReportData` 中除全部年报外，额外保留最近 3 个非年报季度（用于 TTM 3 季 + 1 年报累加成 12 个月口径）。
  - `DownloadResult` 新增 `Quarters []string`，前端「导入年限」改为 `N 年报: ... ; M 季报: ...` 两段展示。
  - `FetchFinancialData(ctx, market, code, maxYears)` 新增 `maxYears` 参数，按调用方传入的年数控制 SFL 接口起始时间窗口（缺省 5 年兜底）。

### 优化 (Improvements)
- **模块 5.2 重点政策方向布局** (`analyzer/report_modules.go`)
  - 政策匹配度按 Level 1-5 分组，同档位合并到一行用「、」连接，由高到低排列。
  - 信号条移到行首，名称放后面，整列对齐更整洁。
- **macOS 应用更新流程** (`updater/updater_darwin.go`)
  - 打开 DMG 后延迟 1.5s 主动退出当前进程，避免用户拖动 `.app` 到 Applications 时被提示「目标程序正在运行」。

### 修复 (Fixes)
- **中栏在窗口最大化/恢复后被压缩** (`frontend/src/App.css`)
  - `.info-panel` 增加 `min-width: 300px` 与 `flex-shrink: 0`，防止 macOS 双击标题栏在最大化与原始尺寸切换时，flex 容器把中栏挤压成 0 宽度。

## [v1.3.40] - 2026-05-23

### 新增 (Features)
- **K线"刷新K线"按钮** (`app.go`, `frontend/src/UnifiedChart.tsx`)
  - 新增 Wails 方法 `RefreshStockKlines(symbol)`：绕过本地 `klines.json` 缓存，从远程拉取全量历史（最多 2500 条，SFL 启用时拉全量）后写回缓存。
  - K线图左上角加按钮，解决早期版本写入的旧缓存（条数偏少）导致 dataZoom 拖不到上市初期的问题（典型例子：山东威达 002026.SZ，旧缓存只有 375 条，最早 2024-11-04；刷新后 5142 条，最早 2004-07-27）。
- **"K线"快捷入口** (`frontend/src/App.tsx`)：股票卡片新增红色"K线"按钮，一键进入全窗口 K 线 + 技术指标联动视图。
- **"刷新"基本资料按钮** (`frontend/src/App.tsx`)：股票卡片可手动刷新行业 / PE / PB 等基础信息。
- **HTTP 客户端封装** (`downloader/http.go`)：统一 timeout / Referer / context 取消。

### 优化 (Improvements)
- **可比公司推荐 UI**：去掉常驻显示的推荐理由行，改为鼠标 hover 容器时浏览器原生 tooltip 弹出，列表更紧凑。
- **context 全链路传递**：`DataRouter` / `eastmoney` / `tencent` / `yahoo` 等所有下载器方法签名增加 `ctx context.Context` 参数，配合 HTTP 超时和 App 退出时优雅取消请求。
- **并发分析合并**：`App` 用 `singleflight.Group` 替换原先的 `analysisMu + analysisLocks`，同股票同 flag 的并发分析自动合并为一次。
- **前端 API 分桶重构** (`frontend/src/api/`)：把 wails 生成的所有 Go 方法按领域拆为 `analysis / data / profile / quotes / report / settings / watchlist / admin` 等模块，统一通过 `wrap.ts` 包装错误归一化，调用方仍可从 `./api` 一次性 import。

### 修复 (Fixes)
- **K线缓存陈旧**：早期版本写入的本地 `klines.json` 一旦命中就不再刷新，导致用户拖动 K 线无法回溯到上市初期。现在提供主动刷新入口（不改变默认命中策略，避免破坏港股 / 新股场景）。

## [v1.3.39] - 2026-05-22

### 新增 (Features)
- **全市场缓存系统** (`downloader/market_cache.go`)
  - 后台自动采集全市场股票基础资料、财务指标、概念映射
  - 7 天有效期，启动时自动检测并后台更新，不阻塞前台
  - 支持手动刷新（Wails 绑定 `RefreshMarketCache`）
  - 为可比公司推荐提供全市场候选池，避免推荐结果局限于自选股
- **SFL 数据源批量接口** (`downloader/sfl_datasource.go`)
  - `FetchAllStockBasic`：全市场股票基础资料
  - `FetchAllLatestFinaIndicator`：全市场最新财务指标（自动尝试最近 4 个报告期合并兜底）
  - `FetchAllConceptMappings`：全市场概念映射（股票→概念列表）
- **可比公司推荐算法升级** (`analyzer/recommend.go`)
  - 接入全市场缓存，候选池从「本地数据+全市场代码」升级为「全市场实时缓存」
  - 新增交易活跃度维度（换手率/量比/金额得分）纳入相似度计算
  - 增加调试日志，便于排查推荐结果偏差
- **设置面板新增「功能」页**
  - 市场热点开关（默认关闭）
  - 分析完成提示（从「数据」页迁移）
  - 风险警示敏感度（从「数据」页迁移）

### 修复 (Fixes)
- **切换股票后历史数据残留**：切换自选股时同步清空 `dataHistory` / `dataMissing`
- **Windows 版本菜单栏**：Windows 下不再显示「显示 / 窗口 / 关于」菜单栏（macOS 保留）
- **macOS 菜单栏**：移除「窗口」菜单，仅保留「显示」和「关于」
- **市场热点默认关闭**：左栏入口按设置条件渲染，关闭时自动收起热点面板

### 优化 (Improvements)
- **关于页面文案**：改为三行显示
  - 穿透财报看真相
  - 揭示风险防踩雷
  - 重要指标可溯源

## [v1.3.38] - 2026-05-16

### 修复 (Fixes)
- **可比公司推荐算法严重偏差**
  - 候选池从「仅本地数据目录」扩大到「全市场同市场股票」（A股只推荐A股，港股只推荐港股）
  - 新增行业同义词映射（半导体↔芯片/集成电路、化学制品↔化工/材料等），解决分词匹配不足
  - 新增跨行业惩罚：二级行业不匹配扣 15–20 分，防止扬杰科技推荐出化工/材料股
  - 权重重构：行业 65%、市值 10%、ROE 15%、毛利率 5%、数据质量 5%
  - 「有本地数据」加分从 20 分降至 5 分，避免「有数据就靠前」
- **切换股票后推荐结果残留**
  - 切换自选股时同步清空 `comparables` / `appliedComparables` / `compRecommendations`
  - 解决中栏仍显示上一只股票推荐结果的问题
- **推荐的可比公司无法下载财报**
  - 从「自动推荐」列表点击添加时，前端只改了状态未调用后端 `AddComparable`
  - 修复为异步保存到 storage 后再刷新前端状态，确保财报下载能正确读取列表
- **自动推荐时大量跨市场网络请求**
  - `RecommendComparables` 现在根据代码后缀过滤候选池（`.SH/.SZ/.BJ` vs `.HK`）
  - 批量拉取候选 profile 时不再请求跨市场数据，减少终端刷屏

### 优化 (Improvements)
- **候选资料批量缓存**：`batchFetchCandidateProfiles` 并发（限制10）补充缺失的 `profile.json`
- **Profile 缓存机制**：`GetStockProfile` 本地缓存 7 天，网络失败时自动回退到过期缓存

## [v1.3.37] - 2026-05-15

### 新增 (Features)
- **季度/TTM 滚动数据分析**
  - 财报下载器现在同时下载季报数据（年报+季报），TTM 计算包含最新季度，时效性更强
  - 新增「季度滚动预警」模块（3.3）：检测营收/净利润/毛利率的环比与同比变化
  - 新增「TTM（滚动12个月）数据」模块（3.4）：拆分为经营规模、盈利能力、现金流质量三个子表格
- **与上次分析对比（Diff）**
  - 同一只股票第二次分析时自动显示「模块1.3: 与上次分析对比」
  - 展示评分变化、风险新增/解除/持续、关键指标变动
- **审计意见自动解析**
  - Python 脚本从巨潮资讯网公告标题推断审计意见类型
  - 非标意见自动触发一票否决
- **可比公司自动推荐**
  - 基于行业、市值、关键词、ROE、毛利率五维度相似度评分
- **ML 预测置信度**
  - 新增高/中/低置信度标识，报告中以 🟢🟡🔴 徽章展示

### 修复 (Fixes)
- **风险爬虫 key 不匹配**：`app.go` 与 `risk_alert.go` 统一为 snake_case，确保股权质押/问询函/减持数据正确显示
- **nil 指针 panic**：`GetWatchlistActivity` 中 `quote == nil` 时安全跳过日志打印
- **goroutine 嵌套 race**：`moneyflowData` 从 `sentimentData` 内部提升到顶层，避免 `wgNet.Wait()` 提前返回
- **审计意见误触发一票否决**：`isStandard` fallback 从字符串 `"待确认"` 改为 `true`（bool）
- **XSS 向量**：移除 `rehype-raw`，HTML 标签改为 Markdown 代码块或 Unicode 字符
- **SVG 信号条显示为代码**：改为前端自定义渲染的绿色信号格图标
- **配置/Token 文件权限**：从 `0644` 收紧为 `0600`

### 优化 (Improvements)
- **报告结构重组**
  - 季度/TTM 从模块1.5/1.6 移至模块3（与年度数据放在一起）
  - 模块11 标题简化为「逆向思维检查」，模块12 简化为「投资检查清单」
  - 移除模块1.1 中重复的 A-Score 行
- **文件拆分**：`report.go` 拆为 3 个文件，`app.go` 拆出 `app_analysis.go`
- **前端测试**：新增 `RiskBadge` / `RiskAlertBanner` 组件测试，总测试数 4→14
- **回归脚本**：`scripts/run-regression.sh` 支持 `quick`/`full` 模式

### macOS
- 应用显示名称改为「财报透镜」（`CFBundleDisplayName` + `CFBundleName`）
- `.app` 包文件名保持 `stockfinlens.app` 不变


## [v1.3.36] - 2026-05-13

### 新增 (Features)
- **macOS 菜单栏 tray 滚动字幕全面优化**
  - 滚动字体增大到 14pt，速度减慢至 35px/s，可读性提升
  - 文字颜色统一为白色，深色菜单栏上更清晰
  - 滚动顺序严格跟随自选列表，完整轮播后从头开始（不再因 30s 刷新而中断）
  - 关闭滚动字幕后 tray 图标自动收窄，节省菜单栏空间
  - tray 右键菜单新增「关闭/显示滚动字幕」「隐藏/显示菜单图标」两个 toggle 选项
- **macOS 顶部应用菜单「显示」**
  - 新增「显示/关闭 滚动字幕」「显示/隐藏 菜单图标」菜单项
  - tray 图标隐藏时「显示/关闭 滚动字幕」自动置灰
  - 菜单项标题跟随 tray 状态同步更新
- **关于对话框**
  - 新增「关于 → 关于 StockFinLens」菜单项
  - 修复版本号显示为 `unknown` 的问题（通过 `//go:embed` 嵌入 wails.json）

### 修复 (Fixes)
- tray 滚动字幕因 30s 价格刷新强制重置索引，导致只能看到前 5 只股票的问题
- 启动时 tray 图标尺寸异常（先大后小）的问题

## [v1.3.35] - 2026-05-12

### 优化 (Improvements)
- **macOS Dock 图标调整**
  - 重写图标生成脚本 `scripts/generate_icons.py`，简化渲染逻辑
  - 使用原始 W logo（`assets/icons/source/red-w.png`），去除白色背景
  - 添加 15% padding，使图标在 macOS Dock 圆角中居中且不被裁切
  - 移除过度发光和阴影效果，提升小尺寸下的清晰度

## [v1.3.33] - 2026-05-08

### 新增 (Features)
- **自动更新功能**
  - 启动时自动检测 GitHub 最新 Release，发现新版本弹窗提示（可配置关闭，位于「设置 → 关于」）
  - 支持手动检查更新（「设置 → 关于 → 检查更新」）
  - 多源下载策略：`gh-proxy.com` 加速镜像优先（实测 ~2.5s/9MB），fallback 到 GitHub 直连
  - **Windows**：下载 zip → 解压 → 替换文件 → 自动重启
  - **macOS**：下载 dmg → `open` 打开 → 提示用户拖拽到 Applications
  - 下载失败时提供「去 GitHub 下载」按钮，打开浏览器到 Release 页面
  - 支持「跳过此版本」，不再提示该版本
  - 新增 `updater/` 包（GitHub API 查询、semver 比对、平台 asset 匹配、多源下载）
- **资金流向当日数据展开**
  - 右栏资金流向区域新增「当日流向」展开/收起面板
  - 交易日盘中显示当日实时主力/大单/中单/小单净流入
  - 非交易日提示「当日数据暂未更新」
- **资金流向多源合并 + 扰乱**
  - 新增东财 `push2his.eastmoney.com` 资金流向接口（多 CDN fallback）
  - `DataRouter.FetchMoneyflow` 改为多源合并：SFL 优先历史数据，东财补当日
  - `shuffleSources` Fisher-Yates 扰乱，降低单一源反爬风险
- **可比公司数量上限提升**
  - 可比公司输入上限从 5 只提升到 7 只

### 优化 (Improvements)
- **tooltip 默认可见性修复**
  - `.inline-tooltip .inline-tooltip-body` 基础样式增加 `visibility:hidden` / `opacity:0`
  - 修复 hover 前 tooltip 直接显示的问题

### 修复 (Fixes)
- **报告模块 4.2 信息按钮丢失**
  - `writeModule4` 中 `infoIcon` 返回空字符串导致 ℹ️ 按钮和「获取真实活跃度」链接丢失
  - 恢复 HTML 输出，重新生成报告后生效

## [v1.3.32] - 2026-05-07

### 新增 (Features)
- **个股资金流向可配置交易日数**
  - 设置 → 数据 → 数据源 → "个股资金流向"开关后新增数字输入框（1~20，默认 3 个交易日）
  - 仅 StockFinLens 数据源启用时可用
  - 中栏资金流向表格和 Summary 同步按配置值展示
- **资金流向 Summary 增强散户视角**
  - 新增散户（小单）累计净流入统计
  - 当主力流出、散户流入时，文案显示"**散户接盘**"，形象表达主力出货、散户接手的含义
  - 例如：`主力持续流出 1.74 亿元；散户接盘 2.30 亿元`
- **主力资金统计口径说明**
  - 资金流向模块底部新增灰色小字说明：
    `主力 = 超大单（>100万）+ 大单（20~100万），按单笔成交金额分档统计，机构可通过拆单规避`

### 优化 (Improvements)
- **资金流向字体整体放大**
  - 标题、表头、数据行、Summary 从 10px 提升到 11px
  - 统计口径说明从 9px 提升到 10px
  - 同步微调 grid 列宽防止溢出

### 修复 (Fixes)
- **资金流向前后端口径不一致**
  - 后端查询 9/15 个自然日可能返回超过配置值的数据条数
  - 现后端截断为只返回最近 N 个交易日，Summary 统计口径与表格展示完全一致

## [v1.3.31] - 2026-05-06

### 修复 (Fixes)
- **中栏资金流向不显示**
  - `loadMoneyflow(s.code)` 传入纯代码（如 `300236`），但 `GetStockMoneyflow` 要求 `code.market` 格式
  - 改为 `loadMoneyflow(s.code + '.' + s.market)`
- **Windows Store Python 安装依赖失败 (exit status 9009)**
  - `findPythonExecutable` 优先查找 python.org 官方安装路径，降级 Windows Store redirector/shim
  - README 新增 Windows Python 安装警告
- **模块7 A-Score 标题编号错误**
  - `ascore_module.go` 标题写成了 `# 模块8: A-Score...`，导致 TOC 跳转锚点不匹配
  - 修正为 `# 模块7: A-Score 综合风险画像`

## [v1.3.30] - 2026-05-05

### 品牌隐藏
- **Tushare 全面替换为 StockFinLens**
  - 代码层面所有类型/变量/函数/注释中的 `Tushare` 替换为 `SFL`/`StockFinLens`
  - `downloader/tushare.go` → `downloader/sfl_datasource.go`
  - Wails 绑定方法同步：`GetSFLConfig`、`SaveSFLConfig`、`VerifySFLToken`
  - 前端 Settings.tsx 变量名和显示文本同步替换
  - 实际 API URL (`https://api.tushare.pro`) 和本地配置路径 (`tushare_config.json`) 保持不变

### 修复 (Fixes)
- **可比公司 ROE 全为 0.00%**
  - `analyzer/comparable.go` 的 `loadComparableFinancialData` 漏了 `fixMissingData()`
  - 可比公司归母权益缺失时无法自动推导修复，导致 ROE 分母为 0
  - 修复后添加 `fixMissingData()` + `validate()`，与主分析路径保持一致
- **季报白下载**
  - StockFinLens 数据源返回年报+季报共约 20 条，但分析引擎只使用年报
  - `ConvertToFinancialReportData` 新增 `isAnnualReport()` 过滤，只存储年报
  - 前端显示的年份数与实际分析数据对齐

## [v1.3.29] - 2026-05-04

### 新增 (Features)
- **分红数据自动补充**
  - StockFinLens 数据源普遍缺失 `c_div_profits_or_int_oop`（分配股利、利润或偿付利息支付的现金）字段
  - 新增 `downloader/FetchCashFlowDividendFromEastMoney`：从东财 API 单独抓取分红数据
  - 下载财报时自动检测：分红字段全为 0 时自动从东财补充
  - 财报透镜分析时自动检测：加载本地数据后再次兜底补充
- **数据源自动切换**
  - 资产负债表不平衡时自动尝试备用源（StockFinLens ↔ 东方财富）
  - 前端展示切换建议，用户感知数据源变更
- **热点板块增强**
  - 字段映射修复：`f15`(最高价)→`f6`(成交额)，`f10`(换手率)→`f5`(成交量)
  - 罗马数字后缀去重：去掉"体育II/III"、"军工电子II/III"等重复项
  - 成分股增强：新增市值(`f20`)、近半年涨幅(`f130`)，点击触发快速分析
  - 板块/成分股数据一致：中栏"主力"优先使用成分股加总替代板块指数 f62
  - 快速分析缓存：按 conceptCode→stockCode 缓存，切换热点自动恢复
- **低风险筛选**
  - `WatchlistFilterItem` 新增 `RiskLevel` 字段
  - 低风险筛选排除 `riskAlert.level !== 'low'` 的股票

### 修复 (Fixes)
- **ROE 为 0.00%（StockFinLens 数据源）**
  - StockFinLens(StockFinLens) 遗漏 `n_income_attr_p`（归母净利润）字段映射
  - `downloader/sfl_datasource.go` 添加 `ParentNetIncome` 映射，`data_router.go` 写入利润表
- **热点板块成交额为 0**
  - 东财 API 字段映射错误导致成交额/成交量为 0
- **分红率为 0.00%**
  - 指标重命名为"分红占经营现金流比"，避免与股息率混淆
  - 阈值修正为 `20%~70%`，与 step18 可持续区间一致
  - 数据缺失时提示"分红数据缺失或未实施现金分红"

### UI/UX 优化
- **删除模块6（实时行情数据）**
  - 实时行情对财报分析价值有限，移除后模块编号前移（共 15 个模块）
  - 同步更新 TOC 导航、模块跳转、前端模块列表
- **中栏按钮重命名**
  - "下载财报" → "财报下载"
  - "财报分析" → "财报透镜"
- **进度条重新分配**
  - 分阶段增长：5-25%（初始化）/ 25-50%（网络数据）/ 50-75%（ML推理+风险扫描）/ 75-90%（财报透视分析）
  - 阶段提示文字更具体，用户知道当前在做什么
- **移除"18步"字样**
  - 代码中所有"18步"/"十八步"替换为"财报透视"
  - 指标名称、注释、提示文案全面更新
- **设置面板调整**
  - "Python 环境"从「数据」tab 移至「关于」tab 下方，改名"运行环境"
  - 风险警示敏感度选项简化：去掉 A-Score 限定说明文字
- **主题适配**
  - 热点表头、快速分析卡片、表头单位等使用 CSS 变量适配深浅主题

## [v1.3.28] - 2026-04-29

### 新增 (Features)
- **外部风险数据三爬虫**
  - 审计机构变更（`downloader/auditor.go` + `scripts/fetch_auditor_history.py`）
    - 巨潮资讯网公告查询，结构化提取变更日期、更换前后事务所、对应年报截止日、变更原因
    - 近3年变更检测，年报披露期内（1-4月）变更一票否决
  - 高管变动（`downloader/exec_changes.go` + `scripts/fetch_exec_changes.py`）
    - 聚焦财务负责人（CFO/审计部），近1年>=3次触发高风险
  - 诉讼/担保/资金占用（`downloader/litigation.go` + `scripts/fetch_litigation.py`）
    - 担保分级：违规担保/资金占用一票否决，普通对外担保降级为累积项
  - 巨潮资讯网公告查询公共工具（`scripts/cninfo_utils.py`）
  - 分析时并发获取外部风险数据，融入风险警示与 A-Score 评分
- **A-Score 风险评分增强**
  - 非财务信号融入：股权质押、监管问询、大股东减持、审计机构变更、CFO变动
  - Z-Score 从一票否决降级为参考信息
  - 排雷阈值放宽：ROE低 10%→5%、毛利率下降 5→10 百分点等
  - 累积触发阈值 2→3 条
  - 新增 `analyzer/risk_alert.go`：12 项一票否决 + 二级指标累积逻辑
- **数据质量自动修复与提示**
  - 东方财富 API `PARENT_EQUITY_BALANCE` 返回 0 时自动修复
    - 总权益 = 资产 - 负债
    - 归母权益 = 总权益 - 少数股东权益
    - 归母净利润 = 净利润 - 少数股东损益
  - `analyzer/data.go` 新增 `fixMissingData()` 运行时兜底修复 + `data_test.go` 验证
  - 资产负债表平衡校验修复：使用总权益（所有者权益合计）替代归母权益
  - 数据质量提示区分严重问题与数据源精度问题，文案更友好
- **财报缺失状态检测**
  - `CheckAnalysisCache` 新增 `DataMissing` 字段
  - 无财报时"财报分析"按钮 disabled + tooltip 提示
  - 下载/导入成功后自动刷新缺失状态

### 修复 (Fixes)
- **ROE 0.00% 异常**
  - 东方财富 API 归母权益字段经常返回 0，导致 ROE 计算为 0
  - `downloader/mapping.go` 字段修正 + `analyzer/data.go` 运行时兜底
- **下载后分析按钮不刷新**
  - `handleDownload`/`handleImport` 成功后调用 `CheckAnalysisCache` 刷新 `dataMissing` 状态
- **高风险项详情补齐**
  - 全部 12 个一票否决项添加 `Details` 数据，数值说明格显示 ℹ️ 图标

### UI/UX 优化
- **风险警示面板重构**
  - 中栏/右栏简化：只显示一行 `3项中高风险 ›`，点击展开
  - 风险警示横幅新增表格展示（警示/风险指标/数值说明/等级）
- **Tooltip 统一**
  - 全部 ℹ️ 图标统一为 mouse hover 悬停弹出
  - tooltip body 改为 `position: fixed` + `z-index: 9999`，脱离容器限制无遮挡
  - 固定弹出方向（右下为主，右边界不足则左下），不再动态切换四向
- **导入/导出按钮**
  - 文案"导入本地csv/excel财报"→"导入csv/excel财报"
  - 位置与下方"下载财报"/"财报分析"按钮对齐，单行显示
- **StockFinLens 品牌隐藏**
  - 全项目替换为"StockFinLens 数据源"/"授权码"
  - `app.go` 新增 `decodeToken()` base64 解码

## [v1.3.27] - 2026-04-30

### 新增 (Features)
- **市场热点板块**
  - 新增热门概念板块实时排行（东方财富 API + 综合打分算法）
  - 左栏置顶「🔥 市场热点」入口，点击直达中栏热点面板
  - 中栏展示 Top 20 热点概念，支持按 Score 降序排列
  - 每个热点显示：编号、名称、🔥火焰分级、成交额、主力净流入、涨幅、综合得分
  - 点击热点展开右栏成分股列表（Top 20），显示代码/名称/最新价/涨跌幅/主力净流入
  - 成分股支持「快速分析」：基本面/流动性/舆情/风口关联四宫格卡片
  - 市场热点与自选股完全分离：打开热点清空股票报告，选中股票关闭热点面板
  - 热点数据自动缓存（15 分钟），历史归档保留 30 天

- **StockFinLens Pro 数据源接入**
  - 新增 `downloader/sfl_datasource.go`：StockFinLens Pro API 客户端（日线/行情/财报/资金流向/每日指标）
  - 新增 `downloader/data_router.go`：数据源路由层，支持 K线/行情/财报/资金流向的优先级路由和 fallback
  - 设置面板「数据」Tab 新增 StockFinLens Pro 配置：Token 输入、连通性验证、保存、启用范围勾选（财报/K线/每日指标/资金流向）
  - 当 StockFinLens 不可用时自动降级到腾讯/东财/Yahoo 等备用源

- **个股资金流向**
  - 自选股信息卡片内新增「近3日资金流向」表格（日期/超大/大/中/小，万元单位）
  - `analyzer/report.go` 新增模块 9.6「资金流向分析」，含近5日资金流向表格 + 主力/散户行为判断
  - `app.go` 新增 `GetStockMoneyflow` Wails 绑定，分析时并发获取资金流向数据

- **快速分析功能**
  - 右栏成分股面板支持点击个股触发快速分析
  - 四宫格展示：基本面（行业/市值/PE/PB/EPS）、流动性（最新价/涨跌/换手/量比/资金流向）、舆情（情绪得分/热度/关键词）、风口关联（概念标签/热点匹配）

- **多源数据扩展**
  - 新增 `downloader/yahoo.go`：Yahoo Finance 日线数据 fallback
  - `downloader/concept.go` 扩展：股票概念与风口静态映射、热门概念板块实时排行

### 修复 (Fixes)
- **StockFinLens K线数据单位错误导致活跃度分数暴跌**
  - `daily` 接口 `amount` 字段单位为「千元」，代码直接当作「元」使用，导致成交额/换手率计算值小 1000 倍
  - 修复：`sfl_datasource.go FetchDaily` 中 `Amount` 赋值时乘以 1000 转换为元
- **StockFinLens K线数据时间顺序错误**
  - StockFinLens 返回的数据为时间倒序（最新在前），与腾讯/东财正序不一致
  - 修复：返回前对数组做反转，统一为正序（最新在后）
- **设置面板数据 Tab 内容溢出**
  - 内容过多时下方被截断，无法滚动查看
  - 修复：`.settings-dropdown` 加 `max-height: calc(100vh - 60px)` + flex 列布局，`.settings-section` 加 `overflow-y: auto`

### UI/UX 优化
- 市场热点中栏顶部操作栏改为左侧 ← 返回箭头、右侧「刷新 🔄」按钮，与左栏 → 箭头垂直对齐
- 中栏热点列表改为两行布局：第一行编号+名称+🔥，第二行成交额/主力/涨幅/得分
- 右栏热点模式下隐藏报告 topbar（跳转章节/搜索/删除/下载）和「未选择股票」占位提示
- 自选股卡片资金流向表格去掉「主力」列（Summary 已含主力统计），只保留日期+超大+大+中+小

## [v1.3.26] - 2026-04-27

### 修复 (Fixes)
- **macOS 构建兼容性**
  - `syscall.SysProcAttr{HideWindow: true}` 是 Windows 特有字段，macOS 编译直接失败
  - 为 `main`、`analyzer`、`downloader` 三个包分别创建 `sysproc_windows.go` / `sysproc_other.go`（build tag 隔离）
  - 8 处 `if runtime.GOOS == "windows" { cmd.SysProcAttr = ... }` 统一替换为 `setHideWindow(cmd)`

- **K 线数据解析错位导致联动图表消失**
  - 根因：`parseEastMoneyKlines` 固定 `offset=1`，但 `push2his.eastmoney.com` 在 `fields2` 生效时返回标准格式（无偏移）
  - 更深层问题：标准格式字段顺序为 `开盘,收盘,最高,最低`，偏移格式为 `开盘,收盘,最低,最高`，两者 `low/high` 相对位置相反
  - 修复：新增 `detectEastMoneyOffset` 自动检测；`offset=0` 时 `high=parts[3], low=parts[4]`，`offset=1` 时 `low=parts[4], high=parts[5]`
  - 新增单元测试 `downloader/eastmoney_kline_test.go`，用真实 API 数据验证两种格式

- **技术指标联动图 Y 轴刻度重叠**
  - 5 个子图（K线/换手/MACD/RSI/BOLL）Y 轴均设在左侧同一位置，刻度标签互相覆盖
  - 修复：非主图（换手/MACD/RSI/BOLL）的 `axisLabel.show` 设为 `false`

- **东方财富 API EOF 错误优化**
  - `EOF` 根因：HTTP Keep-Alive 连接被服务端单方面关闭后，客户端复用陈旧连接
  - `httpGetWithRetry` 增加 `DisableKeepAlives: true`，每次请求新建 TCP 连接
  - 退避策略从固定 `1s, 2s` 改为指数退避 + 随机抖动（`1s+100~600ms`, `2s+...`, `4s+...`）

---

## [v1.3.25] - 2026-04-25

### 新增 (Features)
- **Python 依赖自动检测与一键安装**
  - 首次启动时自动检测 Python 环境及 7 个核心依赖包（onnxruntime、scikit-learn、numpy、pandas、akshare、requests、openpyxl）
  - 缺失包可一键自动 `pip install` 安装，实时输出安装日志到前端弹窗
  - 支持 Windows/macOS 跨平台 Python 查找（含 .venv、常见安装路径、PATH 等）
  - 设置面板「数据」页新增「🔍 检测 Python 环境」手动触发入口
  - 检测通过后写入标记文件，避免每次启动重复弹窗

### 修复 (Fixes)
- **K线开盘价显示异常**
  - 修复所有股票 K 线开盘价显示为递减序列（119、118、117...）的问题
  - 根因：WebView2/Vite 缓存了旧前端页面；同时 ECharts `candle.data` 解析与预期不一致
  - tooltip 改为使用 `params.dataIndex` 直接从 `displayData` 数组取值，彻底绕过 ECharts 内部数据解析层

- **换手柱图不显示**
  - 根因：`App.tsx` 传递的 `quote` 为 `null`，`UnifiedChart` 无法补算历史换手率
  - `UnifiedChart` 改为独立调用 `GetStockQuote` 获取行情，不再依赖父组件的 `quote` prop
  - `GetStockQuote` 缓存校验增加 `CirculatingMarketCap > 0` 条件，防止无效缓存持续返回

- **涨跌额/涨跌幅计算错误**
  - 从 `当天收盘 - 当天开盘` 修正为 `当天收盘 - 昨天收盘`
  - 涨跌幅基准同步修正为昨天收盘价

- **MACD 柱状图数值偏差**
  - 公式从 `DIF - DEA` 修正为 `2 × (DIF - DEA)`，与雪球、同花顺等主流软件一致

- **RSI 指标偏差与单周期问题**
  - 计算方法从简单平均（SMA）改为 Wilder's smoothing（指数平滑），与雪球一致
  - 从单条 RSI(14) 扩展为 RSI6、RSI12、RSI24 三条线，与雪球 RSI(6,12,24) 对齐

### 数据层 (Data)
- **东方财富 K 线接口适配**
  - 域名切换为 `push2his.eastmoney.com`
  - 新增参数：`fqt=1`（前复权）、`rtntype=6`、`beg=0`、`end=20500101`
  - 扩展 `fields2` 字段：`f51-f61,f116`
  - `parseEastMoneyKlines` 增加 `offset=1` 适配新接口默认返回格式
  - 增加数据校验：过滤价格≤0、`high<low` 等异常行

---

## [v1.3.24] - 2026-04-23

### 新增 (Features)
- **A-Score 指标说明折叠块**
  - 在模块1.1（执行摘要）和模块8（A-Score 画像）的 A-Score 评分旁增加 ℹ️ 可折叠说明
  - 说明涵盖：指标定义、六维打分逻辑、A股/港股覆盖差异、评判标准、核心价值

### 优化 (Improvements)
- **About 页面文案重写**
  - 去掉"18维"等专业术语和"黑箱"等负面表述
  - 最终定稿："穿透财报看真相，自动扫描财务风险，重要指标可溯源"
  - 同步更新 GitHub 仓库描述和软件内 About 描述

### 重构 (Refactor)
- **仓库与模块名称统一**
  - Go module 路径从 `github.com/stock-analyzer` 迁移至 `github.com/liusaipu/stockfinlens`
  - 所有 import 路径同步替换
  - wails.json 应用名同步更新为 `stockfinlens`

---

## [v1.3.23] - 2026-04-23

### 新增 (Features)
- **RIM 估值参数基准提示**
  - RIM 参数弹窗中增加参数基准日期说明（"默认参数基准：2025年4月市场数据"），提醒用户根据当前市场环境调整

- **现金流风险分状态标签**
  - 将"现金流偏离度"改为"现金流风险分"，消除负值误解
  - 新增 🟢/🟡/🔴 状态标签（优秀/健康/关注/偏高/高风险），直观展示现金流质量
  - 风险说明中增加备注"负值=现金流优于利润"

### 优化 (Improvements)
- **弱化"十八步"术语，强化产品品牌**
  - 报告中所有"十八步财务分析法"统一替换为"财报透视分析框架"
  - "基本面（18步综合）"改为"财务健康度综合评分"
  - 前端按钮、提示、About 页面等所有用户-facing 文案同步替换

- **亮点/风险摘要逻辑统一移至后端**
  - 消除前端硬编码与后端报告生成之间的判断不一致
  - 前端直接展示后端统一生成的 highlights/risks，确保中栏摘要与报告正文完全一致

### 修复 (Fixes)
- **A-Score 风险等级阈值前后端统一**
  - 修复前端与后端 A-Score 风险等级判断不一致的问题
  - 统一标准：<40 安全、<60 低风险、<70 中风险、≥70 高风险

- **智能选股条件数量修正**
  - 前端目录跳转中"智能选股6大条件"修正为"智能选股7大条件"，与实际检查项数量一致

- **隐藏无数据的审计意见步骤**
  - 审计意见（Step 1）目前没有数据源，全部显示"请查询年报确认"
  - 短期在报告中隐藏此空步骤，避免给用户负面第一印象

### 构建与文档
- 补齐缺失文档：`docs/BUILD.md`、`CONTRIBUTING.md`、`LICENSE`
- 整理文件位置：`scripts/validate_activity.go` → `cmd/validate-activity/main.go`
- 清理 `build/` 目录历史构建残留

---

## [v1.3.22] - 2026-04-20

### 新增 (Features)
- **联动图表双击全屏**
  - 双击图表区域进入全屏模式，全屏后显示约 250 个交易日（约 1 年）
  - 全屏状态下再次双击或按 `Esc` 键返回原尺寸
  - 图表左上角常驻提示文字：普通状态显示「双击能扩展到全窗口」，全屏状态显示「双击 / Esc 回到原来的样式」

- **联动图表 5 区域独立布局**
  - 换手率从 K线图右侧隐藏 Y 轴拆出，独立为第 2 个 grid，解决换手率柱体被压缩到不可见的问题
  - 新布局：K线（占 1/2 高度）→ 换手率 → MACD → RSI → BOLL（4 个指标均分剩余 1/2 高度）
  - 5 个 X 轴通过 `axisPointer.link` 严格联动，滚轮缩放/拖拽时日期完全对齐

- **十字线左侧数值显示**
  - 鼠标在图表内移动时，每个区域左侧 Y 轴旁实时显示当前横虚线对应数值
  - K线区域显示价格、换手率区域显示换手率%、MACD 区域显示 MACD 值、RSI 区域显示 RSI 值、BOLL 区域显示布林带值

### 优化 (Improvements)
- **光标统一为普通箭头**
  - 所有 13 个 ECharts series 设置 `cursor: 'default'`
  - CSS 强制 `.unified-chart-container canvas { cursor: default !important; }`，覆盖 ECharts 内部动态设置的 pointer/grab 等光标

- **K线数据缓存扩容**
  - 网络获取 K线从 250 条增加到 375 条（约 1.5 年），支撑全屏模式下 250 条显示 + 指标计算所需历史数据
  - 本地缓存命中条件同步调整为 `>= 300` 条

- **分析引擎并发安全**
  - 为 `AnalyzeStock` 添加按股票维度的互斥锁，防止同一股票被并发分析导致数据竞争
  - 行情/K线/舆情/ML 引擎等并发 goroutine 统一添加 `recover()`，避免单点 panic 导致整个应用崩溃

---

## [v1.3.21] - 2026-04-18

### 修复 (Fixes)
- **腾讯K线/行情成交量单位智能处理**
  - 腾讯接口对科创板(SH 688)返回"股"，对主板/创业板/北交所返回"手"
  - 修复 `fetchKlinesFromTencent` 和 `fetchQuoteFromTencent` 中未按市场区分导致换手率异常的问题
  - 科创板股票不再被放大100倍，主板/创业板股票不再被缩小100倍

### 修复 (Fixes)
- **并发网络请求 panic 恢复保护**
  - 为 `AnalyzeStock` 中行情/K线/舆情三个并发 goroutine 添加 `recover()`
  - 避免单点异常导致整个 Wails 应用崩溃、窗口变黑
  - 补充舆情获取阶段 debug 日志，便于后续定位问题
- **移除 activity-hint DOM 操作，防止 React 渲染崩溃白屏**
  - 将直接插入 DOM 节点的 useEffect 改为 `data-status` 属性 + CSS 伪元素
  - 添加 React Error Boundary，渲染异常时显示错误堆栈而非白屏
- **修复 ECharts turnoverData null 值崩溃**
  - `UnifiedChart` 中 pad 元素由 `null` 改为 `undefined`，避免 ECharts 读取 `null.value` 抛出 TypeError
- **彻底修复 ECharts 数据不足股票白屏（南网数字等）**
  - 将所有 series 的 pad 占位符统一为 `'-'`（ECharts 官方空数据标记），涵盖 K线、换手率、MACD bar 等全部数据序列
  - `padArray` 函数同步改为 `'-'` 填充，避免任何 `null` 对象进入 ECharts `getInitialData`
- **修复【删除报告】按钮点击无效**
  - Windows 系统对话框在部分环境下返回英文按钮文本 `"Yes"`，导致 `ConfirmDialog` 误判为取消
  - Go 后端兼容 `"确定"` / `"Yes"` / `"OK"` 三种返回值，并增加 `debugLog` 便于排查
  - 删除报告后同步清理 `snapshots` state，左下角"亮点与风险"面板不再残留旧数据
  - 前端去掉不必要的 `loadReportHistory` 调用，简化删除流程

### 新增 (Features)
- **可比公司活跃度就地获取**
  - 报告模块4.2中，缺失活跃度的可比公司旁显示 `[获取真实活跃度]` 链接
  - 点击后后台并发获取真实活跃度并缓存，就地更新报告（保存滚动位置，无页面跳动）
  - 状态提示实时显示在报告正文内，获取完成后自动重新生成模块4
- **中栏布局优化**
  - 移除"数据管理"面板，释放中栏空间
  - 导出本地财报按钮上移至顶部操作区，与导入按钮同一行居中放置
  - 下载财报 / 18步分析按钮下移一行，与导入/导出保持间距避免误点

---

## [v1.3.20] - 2026-04-18

### 新增 (Features)
- **全新品牌图标**
  - 统一替换软件 Logo 为 `png-raw.png`（768×768）
  - 支持 Windows（`icon.ico` 多尺寸）、macOS（`AppIcon.png`）、iOS（1024×1024）
  - 前端 About 页面、浏览器 favicon、README 文档 Logo 同步更新

- **K线图表增强**
  - 获取1年数据（约250个交易日），展示区保持6个月（120条），确保 MA60、RSI、BOLL 指标完整
  - 换手率支持自动计算：当数据源未提供时，用 `成交量(手)×100 / 流通股本 ×100%` 自动补全
  - 均线调整为 MA5 / MA10 / MA30 / MA60
  - 优化左侧纵轴名称位置，避免 "K线"/"MACD"/"RSI"/"布林带" 与纵轴数字重叠

### 修复 (Fixes)
- **换手率计算 bug 修复**
  - 东方财富行情接口 `f21`（流通市值）返回单位为"亿元"，之前漏乘 `1e8` 导致换手率被放大数亿倍
  - 修正后换手率与东方财富官方数据一致（正常范围 0%~30%）

- **UI 精简**
  - 移除 Settings 面板中的"图表"标签页（K线时间范围、均线显示设置已不适用当前统一图表设计）

### 优化 (Improvements)
- 清理项目根目录无用文件（旧压缩包、临时图片、日志文件等）

## [v1.3.19] - 2026-04-17

### 新增 (Features)
- **后台全市场行业数据采集（fallback 数据源）**
  - 新增 `scripts/fetch_all_industry_data.py`，使用 `akshare.stock_yjbb_em` 获取 A 股业绩快报数据
  - 自动提取 ROE、毛利率、营收增长率，按行业聚合计算均值
  - 数据写入 `industry_database_fallback.json`，作为本地数据的补充
  - 任务进度实时写入 `industry_task.json`（状态/进度/总数/时间戳）

- **行业数据库自动合并逻辑**
  - `analyzer/industry.go` 加载时同时读取本地数据库和 fallback 数据库
  - 当本地某行业样本数 `< 3` 时，自动用 fallback 数据补充 ROE/毛利率/营收增长
  - 保证行业对比雷达中的核心指标始终有可对比的行业基准

- **Settings 面板状态显示增强**
  - 前端每 3 秒轮询后台任务状态
  - 按钮文案动态变化：`后台采集中 24%...` / `✅ 更新完成` / `🔄 更新行业数据库`
  - 显示实时进度消息和完成统计（采集日期、股票总数、覆盖行业数）

### 优化 (Improvements)
- **行业对比雷达重构：只保留真正可比、有意义的指标**
  - 新增 **毛利率**、**营收增长率** 两项行业对比指标（均有 fallback 全市场数据支撑）
  - 移除 **应收账款占比**、**存货周转率**：本地样本不足 3 家时行业均值失真，对比意义有限
  - 优化 **净利润现金含量** 与 **资产负债率**：当本地样本不足 3 家时，行业均值显示为 `-`，不再用当前值重复填充误导用户
  - 彻底移除雷达中的 A-Score 项：A-Score（0~100 风险分）与 M-Score（Beneish 模型，约 -2.22）量纲与含义完全不同，不能互相作为行业均值对比
- 应用启动时自动检测是否需要启动后台采集（距上次完成超过 7 天或从未执行）
- 后台采集完成后自动热重载行业数据库，无需手动刷新

## [v1.3.18] - 2026-04-17

### 新增 (Features)
- **报告导出体验优化**
  - PDF 导出与长图片导出从纯前端下载改为通过后端系统保存对话框，支持用户自定义保存路径
  - 后端新增 `ExportReportPDF(symbol, base64Data)` 和 `ExportReportImage(symbol, dataURL)` 绑定方法
  - 用户取消保存时不再弹出错误提示

- **行业数据库指标扩展**
  - 行业均值数据库新增 `inventoryTurnover`（存货周转率）、`receivableRatio`（应收账款占比）、`mScore` 三个字段
  - `scripts/update_industry_database.py` 重写指标提取逻辑，支持基于本地财务数据直接计算 Beneish M-Score、存货周转率及应收账款占比

### 优化 (Improvements)
- **行业对比雷达增强**
  - 应收账款占比指标增加与行业均值的定量对比：超过行业均值 1.5 倍时标黄，超过 20% 时标红
  - 存货周转率指标增加与行业均值的对比：低于行业均值 80% 时标黄
  - 去除对同比变化的过度依赖，改为以行业基准为核心的异常判定逻辑
  - 改为表格化展示（状态、指标、当前值、行业均值），信息更规整
  - 鼠标 hover 每行时显示固定指标科普说明（如"利润中真金白银的比例，低于100%需警惕"）
  - 底部提示简化为单行：`基于本地数据计算 · 设置中可更新`

### 修复 (Fixes)
- **行业对比雷达数据读取修复**
  - 修复 `IndustryMetrics` JSON tag 与 Python 脚本输出字段名不匹配（camelCase vs snake_case）的问题，导致除 ROE 外所有行业均值读取为 0
  - 修复 step5/15 的 key 名错误：`receivableRatio` → `ratio`，`cashContent` → `cashRatio`
  - 从雷达中移除 A-Score 项：A-Score（0~100 风险分）与 M-Score（Beneish 操纵模型，正常值 <-2.22）是不同维度指标，不可互相作为行业均值对比

### 其它
- **全局滚动条样式统一为现代细线隐形风格**
  - 宽度仅 6px，轨道透明，滑块半隐，hover 时轻微显色
  - 适配深色/浅色双主题

## [v1.3.17] - 2026-04-17

### 新增 (Features)
- **报告导出 PDF 格式**
  - 将报告下载下拉菜单中的「Excel 格式」替换为「PDF 格式」
  - 使用 `html2pdf.js` 将 Markdown 报告渲染为 A4 尺寸 PDF 并触发下载
  - 保留后端 `ExportFinancialDataToExcel` 方法供后续使用

### 优化 (Improvements)
- **自选列表空状态提示**
  - 当筛选条件无匹配股票时，显示「没有符合条件的股票」提示
  - 当自选列表为空时，显示「自选列表为空」引导提示
- **亮点与风险面板样式简化**
  - 移除「✅ 亮点」和「⚠️ 风险」分组标题，使内容更紧凑
- **财务趋势弹窗交互优化**
  - 移除抽屉头部关闭按钮，统一通过点击遮罩或外部区域关闭

## [v1.3.16] - 2026-04-16

### 新增 (Features)
- **行业对比雷达**
  - 在股票信息卡下方新增异常雷达卡片，用 🔴🟡🟢 色块展示最近一年的 7 项关键风险信号
  - 后端新增 `GetRiskRadar(symbol)` API，优先读取分析快照避免重复计算
  - 升级为「行业对比雷达」：增加行业均值对比，调用 `GetIndustryMetrics` 获取行业基准数据
  - 每个指标消息改为 `当前值 (行业均值 xx)` 格式，异常判定结合行业均值（如 ROE 低于行业均值 30% 时标黄）
  - 位置移至「💡 亮点与风险」下方，中栏新顺序：概念 & 风口 → 亮点与风险 → 行业对比雷达 → 可比公司 → 实时行情
- **一键导出财务数据 Excel**
  - 在报告下载按钮旁新增「导出Excel」按钮
  - 生成包含 4 个 Sheet 的 `.xlsx` 文件：资产负债表、利润表、现金流量表、18步分析汇总
  - 后端使用 `excelize/v2` 库直接从本地财务数据和分析快照生成 Excel
  - 新增 `ExportFinancialDataToExcel(symbol)` 绑定方法

### 优化 (Improvements)
- **报告下载交互优化**
  - 将「下载报告」和「导出Excel」两个按钮合并为下拉菜单
  - 下拉选项：📝 Markdown 格式、📊 Excel 格式、🖼️ 长图片
  - 长图片使用 `html-to-image` 将报告渲染为 PNG 并触发下载
- **左栏布局优化**
  - 筛选器改为 `Collapsible` 可展开/收起（默认收起），标题显示 `🔍 筛选器`（有筛选时附加 `x/y只`）
  - 左栏默认宽度从 220px 扩宽至 260px，最小/最大拖动范围同步调整
  - 中栏 `info-panel` 宽度从 300px 扩宽至 340px，内容显示更舒展
- **行业对比雷达样式优化**
  - 改为 `Collapsible` 组件，支持展开/收起
  - 展开后每项改为单行显示（图标 | 名称 | 状态 | 详情），消息过长时自动省略

### 修复 (Fixes)
- **行业对比雷达空状态提示修复**
  - 修复未执行 18 步分析时错误显示「请先下载财报」的问题
  - 空状态提示改为「暂无对比数据（请先执行18步分析）」，避免误导用户
- **BuildRiskRadar nil pointer dereference 修复**
  - 修复当 `stepResults` 中某些步骤缺失时可能触发的 nil pointer panic
- **行业均值显示修复**
  - 修复行业对比雷达在某些情况下无法正确显示行业均值的问题

## [v1.3.15] - 2026-04-16

### 新增 (Features)
- **自选股多维筛选器**
  - 在自选列表头部新增筛选栏，支持 6 种快捷过滤条件：高回报（股东回报率 >10%）、低风险（A-Score <60）、有财报、未下载、已分析、未分析
  - 新增按行业下拉过滤，自动从已有自选股中提取行业列表
  - 筛选时自动禁用拖拽排序，避免交互冲突
  - 后端新增 `GetWatchlistFilterData` API，批量读取本地快照、财报数据与股票资料，计算所有维度，无网络请求
- **财务指标趋势图**
  - 中栏股票信息卡 footer 新增「财务趋势」按钮
  - 弹出 ECharts 折线图弹窗，展示近 5 年 5 项核心财务指标：ROE、毛利率、营收增长率、现金含量、资产负债率
  - 支持多指标同时切换显示（默认展示 ROE + 毛利率）
  - 数据全部来自本地财报，缺失项不绘制；无数据时提示"请先下载财报"
  - 后端新增 `GetFinancialTrends(symbol)` API

### 修复 (Fixes)
- **"交投清淡"判断逻辑修正**
  - 将 Module 9.2 的量价关系判断从"单日行情数据"改为"近 5 日平均换手率 + 近 5 日平均振幅"
  - 避免单日低换手行情对高活跃股票（如御银股份）的误判

## [v1.3.14] - 2026-04-15

### 新增 (Features)
- **股票信息卡新增股东回报率速算**
  - 在「市盈率 (PE)」与「市净率 (PB)」下方新增「股东回报率」指标
  - 计算公式：股东回报率 = ROE / PB + 股息率（≈ 盈利收益率 + 股息率）
  - 颜色提示：>10% 绿色（显著吸引力）、6%~10% 黄色（一般）、<6% 灰色（偏低）
  - 鼠标悬停显示公式拆解与说明
  - 数据后端统一计算：优先读取本地最新年度财报数据计算 ROE，结合东方财富/腾讯行情接口获取的股息率实时得出

### 修复 (Fixes)
- **股东回报率单位换算修正**
  - 修复 `app.go` 中 `fillShareholderReturnRate` 的单位错误：后端计算的 ROE 是百分比数值（如 20 表示 20%），需先除以 100 转为小数后再参与股东回报率计算。之前未除 100 导致结果被放大了 100 倍（如显示 1000% 而非 10%）
- **Release 构建包完整性修复**
  - 修复 `build-release.sh` 和 `build-windows.ps1` 未打包 `scripts/` 目录的问题，确保发布的 zip 包包含所有 Python 脚本
  - 修复 Go 后端脚本路径查找逻辑：`industry_updater.go`、`policy_updater.go`、`eastmoney.go` 中的脚本路径函数在打包后的 Windows 应用中优先在 `exe` 同级目录查找 `scripts/`，解决用户端更新政策库/行业数据库/港股数据时"脚本不存在"的报错

### 文档
- **新增 AGENTS.md 发布检查清单**
  - 规范版本发布流程，强制要求 `wails.json` 与 `frontend/src/Settings.tsx` 版本号同步
  - 明确"发布后 bump 到下一个版本号"的规则，避免临时构建包显示旧版本号

## [v1.3.13] - 2026-04-14

### 修复 (Fixes)
- **Settings 数据管理提示状态解耦**
  - 将 `settingsActionStatus` 拆分为 `policyActionStatus` 与 `industryActionStatus`，更新政策库/行业数据库的成功或失败提示仅显示在对应按钮下方，不再互相串位
- **行业数据库更新提示时长调整**
  - 更新成功后的轻量提示条保留时间从 3 秒延长至 5 秒，与失败提示保持一致
- **彻底移除 Settings 中的 alert 弹窗**
  - 政策库与行业数据库更新的反馈全部改用按钮下方轻量提示条，不再调用 `alert()`

### 优化 (Improvements)
- **模块 4.2 信息弹窗交互优化**
  - 可比公司明细标题右侧的 ℹ️ 说明弹窗，现支持点击弹窗外部任意区域自动关闭，无需再次点击图标

## [v1.3.12] - 2026-04-14

### 新增 (Features)
- **舆情数据源扩展**
  - 在原有东财研报 + 新浪新闻 fallback 基础上，新增**东财公司公告**作为中间 fallback 数据源
  - 公告数据来自 `np-anotice-stock.eastmoney.com/api/security/ann`，覆盖最近 6 个月的公司公告/重大事项
  - 抓取公告标题后进行情感分析，输出情绪得分、热度指数、利好/风险关键词及舆情摘要
  - 优先级：东财研报 → 东财公告 → 新浪财经新闻
- **港股财报网络下载支持**
  - 新增 `scripts/fetch_hk_financials.py`，通过 akshare 获取港股资产负债表、利润表、现金流量表年报数据
  - 对港股财务科目做中文映射，兼容现有 A 股 18 步分析引擎
  - 港股下载取消原来的"暂不支持"限制，分析流程与 A 股一致
- **港股 K 线支持**
  - 修复腾讯财经 K 线接口对港股的参数兼容性问题（港股接口末尾不加 `qfq`，读取 `day` 键）
  - `FetchStockKlines` 对港股可正常返回日 K 数据，支持技术面分析与前端 ECharts 展示

### 优化 (Improvements)
- **设置开关全面打通**
  - **财报下载年限**：`reportYears` 设置项从前端真正传递到后端下载逻辑，A 股/港股均支持下载 3~10 年财报（默认 5 年）
  - **自动更新行业库**：应用启动时自动检测，若开启 `autoUpdateIndustryDB` 且行业库超过 7 天未更新（或从未更新），自动在后台静默更新
  - **分析完成通知**：若开启 `analysisNotification`，十八步分析完成后自动发送 Windows Toast 系统通知
- **报告样式修复**：模块 4.2 的"综合评分排名"蓝色亮条改为 blockquote 形式，与"解读"条保持一致，深色/浅色模式下均能正确显示
- **可比公司下载提示优化**：下载可比公司财报后，成功/失败结果改为按钮下方轻量提示条（成功 3 秒隐去，失败 5 秒隐去），不再弹出 alert

### 文档
- 修正 `功能列表.md` 中 K 线图表库描述：`lightweight-charts` → `Apache ECharts`
- `README.md` / `plan.md` 更新：将 LLM 智能财报问答标记为"暂不实现"；iOS App 与 Engine-C 列为长期规划

### 平台安装注意
- **Windows**：需要预装 Python 3.10+，并执行 `pip install onnxruntime scikit-learn numpy akshare`；需安装 WebView2 Runtime（Win10/11 通常已预装）
- **macOS**：需要预装 Python 3.10+，并执行 `pip3 install onnxruntime scikit-learn numpy akshare`；首次运行若提示安全拦截，需在"系统设置 > 隐私与安全性"中允许

## [v1.3.11] - 2026-04-14

### 新增 (Features)
- **可比公司分析增强（模块4.2）**
  - 将 M-Score 替换为更具 A 股意义的 **A-Score** 综合风险评分
  - 新增**活跃度**列，复用自选列表的本地缓存数据（缺失时以样本中位数替代，标记为 `*`）
  - 引入 7 维度加权综合评分：ROE(25%)、毛利率(20%)、营收增长(15%)、现金含量(10%)、负债率(10%)、A-Score(10%)、活跃度(10%)
  - 表格按综合得分**降序排序**，当前公司以粗体高亮
  - 增加**蓝色亮条**：显示当前公司在可比池中的排名、综合得分及投资建议（第1名建议重点关注，后40%建议谨慎）
  - 标题右侧添加 ℹ️ 信息图标，点击可查看综合得分计算方式说明

### 修复 (Fixes)
- **亮点与风险面板持久化**：将 `AnalysisReport` 分析快照以 JSON 形式保存到磁盘，重启应用后自动恢复，确保「亮点与风险」摘要不丢失
- **trace-trigger 计算溯源修复**：恢复快照后，点击报告中的 ❓ 图标可正常打开计算过程抽屉（之前因 `report` 状态为空导致点击无反应）

## [v1.3.10] - 2026-04-13

### 优化 (Improvements)
- **亮点与风险面板**: 改用 A-Score 评判（替换 M-Score），更适合 A 股市场
  - A-Score < 40: 低风险，财务质量良好
  - A-Score 40-60: 中等风险，需关注
  - A-Score 60-80: 偏高，存在财务操纵或偿债风险
  - A-Score ≥ 80: 高风险，建议谨慎
- **界面简化**: 将"产业政策库"和"行业均值数据库"从主界面移至设置面板
  - 中栏界面更简洁
  - 数据管理功能统一在设置-数据标签页
  - 保留手动更新入口，方便用户在需要时更新

---

## [v1.3.9] - 2026-04-13

### 新增 (Features)
- **设置面板**: 右上角设置按钮，整合主题切换和配置选项
  - 外观：主题（深色/浅色/跟随系统）
  - 图表：K线默认时间范围、均线显示开关
  - 数据：财报下载年限、自动更新行业库、分析完成提示
  - 关于：版本信息、检查更新链接
- **模块复制功能**: 分析报告中每个模块标题旁添加复制按钮
  - 支持复制 Markdown 格式
  - 支持复制纯文本格式
  - 支持复制为图片（带表格线样式）

### 优化 (Improvements)
- **可比公司明细表**: 模块4.2中当前公司数据放第一行并加粗显示

---

## [v1.3.8] - 2026-04-13

### 新增 (Features)
- **K线图表重构**: 从 lightweight-charts 迁移到 Apache ECharts
  - 4个图表完美对齐（K线+成交量、MACD、RSI、布林带）
  - 添加 5日/30日/180日/250日(年线) 均线
  - 默认显示最近6个月数据（约120个交易日）
  - 成交量柱压缩，不遮挡K线
  - 支持统一缩放和拖拽

---

## [Unreleased]

### 新增 (Features)
- **工作进度文档**: 新增 `progress.md`，建立标准化的会话管理工作流程

### 修复 (Fixes)
- **舆情数据获取失败**: 修复东财研报接口超时导致分析流程受阻的问题
  - 添加重试机制（2次尝试 + 500ms间隔）
  - 调整时间范围为最近6个月
  - 所有数据源失败时返回空数据结构（带友好提示）而非错误
- **行业数据库更新失败**: 修复更新行业数据时脚本执行失败的问题
  - 改为从本地股票财务数据计算行业均值，不再依赖不稳定的外部API
  - 修复 Windows 控制台编码问题
  - 适配正确的财务数据格式 {科目: {日期: 值}}
- **财报下载失败**: 增强财报下载健壮性
  - HTTP请求添加指数退避重试（最多3次，间隔1s/2s/4s）
  - 增加超时时间至30秒
  - 东方财富失败时自动尝试腾讯财经作为备用数据源
  - 改进错误信息，提供故障排查建议

### 开发工具
- **会话管理流程**: 定义标准化的开发流程：启动前查看进度 → 环境检查 → 基础测试 → 修bug/开发新功能 → 验证 → 提交代码 → 更新进度

---

## [v1.2.0] - 2026-04-11

### 新增 (Features)
- **模块4增量更新**: 可比公司"更新到报告"按钮改为只更新模块4（行业横向对比分析），不再重新下载财报或执行完整分析流程，大幅提升更新速度。
- **ML模型数据缺失提示**: 模块10（ML机器学习预测）各子章节（10.1/10.2/10.3）现在始终显示，当模型数据缺失时展示具体原因（ONNX模型文件未加载/历史数据不足等）。

### 优化 (Improvements)
- **主题切换按钮样式**: 调整主题切换按钮高度为30px，与报告栏其他按钮（搜索、删除、下载）保持一致，上下边沿对齐。
- **18步分析按钮禁用逻辑**: 当没有财务数据时（dataHistory.length === 0），"18步分析"按钮自动禁用并提示用户先下载财报。
- **股票名称字体大小**: 中间面板股票名称字体从22px调整为18px，更加协调。
- **删除/下载报告按钮禁用**: 当没有报告内容时，删除和下载按钮自动禁用。

### 修复 (Fixes)
- **止损位计算逻辑**: 修复止损位可能高于入场区间低位的问题。现在止损位始终低于入场区间低位（预留至少5%缓冲），避免"该买还是该卖"的矛盾。
- **模块10子章节编号**: 修复模块10子章节编号不连续问题（如直接从10.2开始）。现在10.1/10.2/10.3始终按顺序显示，数据缺失时显示说明而非跳过。
- **Engine-B模型输入维度**: 修复`BuildMLEngineBInput`返回序列长度不固定的问题。现在始终返回8个时间步（数据不足时用零值填充），匹配ONNX模型期望的`[batch, 8, 8]`输入形状。

---

## [v1.1.2] - 2026-04-07

### 修复 (Fixes)
- **RIM 生产构建路径修复**：Go 后端从二进制位置向上递归搜索项目根目录，解决 Wails 打包后 `scripts/fetch_rim_data.py` 和 `ml_models/inference.py` 路径找不到的问题。
- **RIM 回退逻辑强化**：当外部 EPS 预测获取失败时，使用行情数据（总市值/股价）计算总股本，不再依赖可能为 nil 的外部数据对象。
- **财报字段兼容性**：回退计算中的净利润、股东权益通过 `GetValueOrZero` + 别名归一化读取，兼容网络下载与本地 CSV 导入的不同科目名。
- **Python 子进程环境变量**：注入 `TQDM_DISABLE=1` 和 `PYTHONUNBUFFERED=1`，避免进度条和缓冲污染 JSON 输出。

---

## [v1.1.1] - 2026-04-07

### 新增 (Features)
- **多期 RIM 剩余收益模型估值**
  - 将单阶段简化公式升级为严格 8 步法多期 RIM：逐年滚存 BPS、计算 RE、折现求和、计算持续价值 CV。
  - 自动通过 `akshare` 获取机构一致预期 EPS、中国 10 年期国债收益率、个股实时行情。
  - 资本成本 kE 通过 CAPM 自动计算（Rf + Beta × Rm-Rf），不再使用固定 7% 假设。
  - 报告新增 7.3 多期计算明细表，展示 6 年预测期的 EPS、BPS、RE、折现率、RE 现值。
  - 悲观/基准/乐观三情景输出具体内在价值（元）及相对当前股价的涨跌幅与评级。
- **RIM 参数调整面板**
  - 前端报告区新增「调整 RIM」按钮，支持用户手动修改 Beta、Rf、Rm-Rf、永续增长率 g、BPS0、当前股价和未来 6 年 EPS。
  - 后端新增 `AnalyzeStockWithRIM` 绑定方法，支持从前端传入自定义参数重新分析。

### 修复 (Fixes)
- **Python 虚拟环境调用修复**：Go 后端调用 Python 脚本（`fetch_rim_data.py`、`inference.py`）时，优先使用项目 `.venv/bin/python3`，解决系统 `python3` 缺少 `akshare`/`onnxruntime` 导致数据获取失败的问题。
- **RIM 数据兜底逻辑**：当外部 EPS 预测获取失败时，自动用财报净利润/股本推算 trailing EPS，并按默认增长率外推 6 年，确保只要有 PB 就能出多期估值结果。

---

## [v1.1.0] - 2026-04-07

### 新增 (Features)
- **ML 双引擎预测集成**
  - **Engine A (Sentiment+Price Fusion)**: 基于 Cross-Attention 的 TextCNN+Price Encoder 模型，预测次日走势方向（上涨/持平/下跌）及异动概率。
  - **Engine B (Financial LSTM)**: 基于 BiLSTM+Self-Attention 的财务序列模型，预测 ROE、营收、M-Score 趋势方向及综合财务健康分。
  - 统一 Python ONNX 推理入口 (`ml_models/inference.py`)，Go 侧通过 `analyzer/ml_inference.go` 子进程调用并解析 JSON 结果。
  - 新增 `analyzer/ml_features.go` 负责从财报和 K 线提取模型输入特征。
  - 报告模块 9 自动渲染 Engine A/B 预测表格；模型不可用时优雅回退到财务因子简易推断。
- **RIM 估值接入实时行情**
  - 模块 7 剩余收益模型现在可直接利用实时股价、总市值、PB 计算每股净资产(BPS)、EPS 及内在价值。
  -  pessimistic / baseline / optimistic 三情景输出具体内在价值(元)及相对当前股价的涨跌幅与评级。
  - 动态解读文本根据估算上行空间提示低估/高估/中性判断。
- **十五五政策匹配度 UI 升级**
  - 政策标签改为行内 flex chip 形式，带 5 级 SVG 信号强度条，颜色与透明度随匹配等级变化。
- **主操作按钮优化**
  - `下载财报` / `18步分析` 改为同一行内联布局，增加图标，减少垂直空间占用。
- **自选列表搜索高亮与自动加载**
  - 搜索命中股票后自动滚动定位并添加金色闪烁反馈。
  - 选中自选股后自动加载最新历史报告。

### 优化 (Improvements)
- **报告子章节编号对齐**: 修正模块 6~15 内部子章节编号与模块编号不一致的问题（如模块 6 内从 `5.1/5.2` 改为 `6.1/6.2` 等）。
- **章节跳转滚动位置修正**: `handleTocJump` 改用 `getBoundingClientRect` 精确计算相对滚动容器的位置，确保跳转后模块标题始终位于可视区域最上方。
- **可比公司变更检测**: 新增 `appliedComparables` 状态与橙色脉冲动画，当已生成报告的可比公司与当前配置不一致时提醒用户重新分析。

### 修复 (Fixes)
- **ONNX 输出节点解析修复**: `inference.py` 明确指定输出节点名称，避免 `softmax` 中间节点导致返回值数量不匹配的错误。
- **导入循环修复**: 使用 `analyzer.MLKlineData` 避免 `analyzer` 包直接引用 `downloader.KlineData` 造成的测试导入循环。

---

## [v1.0.2] - 2026-04-07

### 修复 (Fixes)
- **自选列表活跃度修复**: 修复 `GetWatchlistActivity` 中股票代码格式错误（`002584.SZ` 未拆分导致所有数据源返回空），自选列表现在能正常显示 25 只股票的活跃度分值。
- **CSV 解析修复**: 给网易财经 K 线 CSV 解析器增加 `LazyQuotes = true`，避免遇到不规范引号时直接崩溃。

### 优化 (Improvements)
- **自选活跃度显示**: 自选股列表的“活跃度”由星级（⭐）改为显示具体分值（0-100 整数）。
- **表头样式优化**: 未排序时“活跃度 ⇅”保持单行显示；“股票名称”表头左对齐，与下方列表项对齐。

### 新增 (Features) - 来自 v1.0.1 累积
- **交易活跃度评分**: 基于换手率、成交额、持续性、波动性、时间结构 5 维度计算 0-100 分活跃度得分，带行业基准校正和 1-5 星评级。
- **技术形态分析**: 集成 MA/MACD/RSI/Bollinger/形态识别到分析报告。
- **蓝绿属性自动推断**: 针对台湾籍高管，通过百度百科共现匹配推断政治属性。

---

## [v1.0.1] - 2026-04-06

### 修复 (Fixes)
- **构建修复**: 修复 `app.go:334` 的 `non-constant format string` 编译错误。
- **国籍识别修复**: 增加“台湾”字样兜底判定，解决台湾籍董事长被误判为中国大陆的问题。

### 构建 (Build)
- macOS Release: `build/bin/stock-analyzer_20260406_220821.zip`
- Windows Release: `build/bin/stock-analyzer_windows_20260406_220821.zip`

---

## 2026-04-06 18:12
- 前置版本构建（上一次发布基线）。
# Changelog