# 项目架构与数据流

> 详细的项目结构、数据流、并发模型与运行期约定。若修改架构，请同步更新本文件。

## 包划分

| 包 | 职责 |
|----|------|
| `main` | Wails 应用生命周期、App 绑定方法、存储管理、CSV/Excel 解析、Python 依赖管理、系统托盘 |
| `analyzer` | 纯分析逻辑，不依赖网络，输入为本地财务数据 + 外部传入的行情/舆情/可比公司数据 |
| `ai_researcher` | AI 投研搜索：调用 Tavily 搜索互联网公开信息，通过 Kimi/DeepSeek 生成结构化投研报告 |
| `downloader` | 所有网络 I/O：财报下载、行情、K线、舆情、风险爬虫、外部数据获取、数据源路由 |
| `updater` | 自动更新：GitHub Release 检测、多源下载、跨平台安装 |
| `tray` | 系统托盘集成（macOS/Windows） |

## 项目结构

```
stockfinlens/
├── main.go                       # Wails 入口：embed 前端 dist、股票库、ML 模型文件
├── app.go                        # App 结构体与 Wails 绑定方法
├── app_analysis.go               # 分析流程编排：AnalyzeStock / QuickAnalyzeStock
├── storage.go                    # 本地文件存储管理器（JSON 持久化）
├── csvparser.go                  # 同花顺 CSV/Excel 财报导入解析器
├── deps_manager.go               # Python 依赖检测与一键安装管理器
├── sysproc_windows.go            # Windows 构建标签：exec.Cmd 隐藏窗口
├── sysproc_other.go              # 非 Windows 构建标签：exec.Cmd 空操作
├── integration_test.go           # 端到端集成测试（真实 CSV 数据 603501）
├── storage_test.go               # 存储层单元测试
├── regression_test.go            # 端到端回归测试
├── app_test.go                   # AppConfig 持久化、版本号、自选股排序测试
├── wails.json                    # Wails 配置（版本号唯一来源）
├── go.mod / go.sum               # Go 依赖
├── requirements.txt              # Python 依赖
├── build-windows.ps1             # Windows 构建/打包/清理脚本
├── build-release.sh              # macOS/Windows 发布构建脚本
│
├── analyzer/                     # 核心分析引擎（纯逻辑，无网络 I/O）
├── ai_researcher/                # AI 投研搜索（大模型 + Tavily）
├── downloader/                   # 数据下载与爬取层（所有网络 I/O）
├── updater/                      # 自动更新
├── tray/                         # 系统托盘
├── frontend/                     # React + TypeScript 前端
├── ml_models/                    # ML 模型与推理脚本（打包时必须包含）
├── scripts/                      # Python 数据脚本（打包时必须包含）
├── cmd/                          # CLI 工具
├── data/                         # 内置股票代码库等静态数据
└── docs/                         # 文档
```

## 核心数据流

1. 用户在 `App.tsx` 选择股票 -> 调用 `AddToWatchlist`
2. 下载财报：`DownloadReports` -> `downloader.DataRouter` -> 多源获取 -> 保存到 `~/.config/stock-analyzer/data/{symbol}/`
3. 自动更新：`startup` -> 后台检查 GitHub API -> `update:available` Event -> `UpdateModal` -> `DownloadUpdate`（gh-proxy.com 加速镜像优先）-> `ApplyUpdate`（Windows: bat 替换+重启 / macOS: open dmg）
4. 执行分析：`AnalyzeStock` -> `analyzer.RunAnalysisWithAll` -> 生成 `AnalysisReport` -> 保存 Markdown 报告与 JSON 快照
5. 前端读取快照恢复亮点/风险面板，读取 Markdown 渲染报告
6. AI 投研：用户点击"AI 投研" -> `AnalyzeStockWithAI` -> `ai_researcher.Research` -> Tavily 多维度搜索 -> LLM 生成结构化报告 -> 缓存到 `data/{symbol}/ai_research_cache.json` -> 前端 `AIResearchPanel` 渲染
7. 市场热点：用户点击"刷新" -> `FetchHotConcepts` -> `downloader.FetchHotConceptBoard` -> 东财 API -> 综合打分排序 -> 缓存到 `data/hot_concepts/latest.json` + 归档历史 -> 前端展示 Top 20 热门概念及成分股

## 并发模型

- **单次分析互斥**: `app.analysisLocks[symbol]` 防止同一只股票重复分析
- **分析内部并行**: `analyzeStockInternal` 使用 `sync.WaitGroup` 并发拉取 quote/klines/sentiment/moneyflow（4 个网络 goroutine）以及 ML/RIM/risk crawler/external risk（3–4 个数据 goroutine）
- **快速分析**: `QuickAnalyzeStock` 采用类似的并行 goroutine 模式
- **自选股活跃度**: 批量并发获取，带缓存
- **请求去重**: `singleflight.Group` 合并同一股票的并发分析请求，避免重复计算

## 股票代码格式

- A 股上海：`603501.SH`
- A 股深圳：`000001.SZ`
- 港股：`00700.HK`
- 内部存储和 UI 传递均使用上述带点格式

## 缓存策略

| 数据类型 | 缓存位置 | 有效期 |
|----------|----------|--------|
| 实时行情 | `data/{symbol}/quote.json` | 15 分钟 |
| 舆情情绪 | `data/{symbol}/sentiment.json` | 60 分钟 |
| 股票资料 | `data/{symbol}/profile.json` | 7 天 |
| 活跃度 | `data/{symbol}/activity.json` | 1 天 |
| RIM 外部数据 | `data/{symbol}/rim_cache.json` | 12 小时 |
| K线数据 | `data/{symbol}/klines.json` | 持久（分析时写入） |
| 分析报告 | `reports/{symbol}/latest.md` | 每次分析覆盖 |
| 分析快照 | `snapshots/{symbol}.json` | 每次分析覆盖 |
| AI 投研报告 | `data/{symbol}/ai_research_cache.json` | 6 小时（可配置） |
| 热门概念排行 | `data/hot_concepts/latest.json` | 15 分钟 |
| 热门概念历史 | `data/hot_concepts/history/YYYY-MM-DD.json` | 永久（保留 30 天） |

## AI 投研配置参数

配置文件：`~/.config/stock-analyzer/ai_config.json`

> 如何申请 Tavily / DeepSeek / Kimi API Key 并填入设置，请参阅项目首页 [README.md](../README.md) 中的「**AI 投研设置指南**」。

| 层级 | 参数 | 说明 | 默认值 |
|------|------|------|--------|
| 连接层 | `llm_provider` | LLM 供应商：`kimi` / `kimi-code` / `deepseek` | `deepseek` |
| 连接层 | `llm_api_key` | LLM API Key | `''` |
| 连接层 | `llm_base_url` | OpenAI-compatible 端点 | `https://api.deepseek.com/v1` |
| 连接层 | `llm_model` | 模型名称，可通过 UI「更新模型」按钮拉取服务商最新列表 | `deepseek-v4-pro` |
| 连接层 | `llm_timeout` | 请求超时（秒） | `90` |
| 生成层 | `temperature` | 创造性/稳定性控制 | `0.2` |
| 生成层 | `max_tokens` | 单次输出上限 | `8192` |
| 生成层 | `top_p` | 采样多样性 | `1.0` |
| 搜索层 | `search_provider` | 搜索引擎：`tavily` | `tavily` |
| 搜索层 | `search_api_key` | Tavily API Key（兼容旧配置单 Key） | `''` |
| 搜索层 | `search_api_keys` | Tavily API Key 列表，最多 5 个，自动轮询 | `[]` |
| 搜索层 | `search_depth` | `basic` / `advanced` | `advanced` |
| 搜索层 | `search_timeout` | Tavily 请求超时（秒） | `180` |
| 搜索层 | `max_results` | 每次查询返回条数 | `20` |
| 搜索层 | `search_recency_days` | 只搜最近 N 天 | `180` |
| 业务层 | `focus_regions` | 国际市场关注区域 | `['us', 'jp']` |
| 业务层 | `output_language` | 输出语言 | `zh-CN` |
| 业务层 | `enable_social` | 是否抓取社交情绪 | `true` |
| 业务层 | `cache_ttl_hours` | 缓存有效期（小时，1~720） | `6` |

## 财报透镜分析流程

在 `analyzer/engine.go` 中按顺序执行：
1. 审计意见 → 2. 资产规模 → 3. 偿债能力 → 4. 竞争地位 → 5. 应收账款 → 6. 固定资产 → 7. 投资资产 → 8. **风险分析（A-Score）** → 9. 营收增长 → 10. 毛利率 → 11. 运营效率 → 12. 成本控制 → 13. 费用率 → 14. 核心利润 → 15. 现金流质量 → 16. ROE → 17. 资本支出 → 18. 分红政策

## Python ↔ Go 路径解析策略

Python 脚本与模型文件的路径解析采用 **4 级回退**：

| 优先级 | 脚本路径 | Python 可执行文件 |
|--------|----------|-------------------|
| 1 | `os.Executable()` 所在目录（打包后的 Windows/macOS） | 同目录下的 `.venv/bin/python3` 或 `.venv/Scripts/python.exe` |
| 2 | macOS `.app` bundle 的 `Contents/Resources/` | macOS `.app` bundle 内的 `.venv` |
| 3 | `runtime.Caller(0)` 向上查找标记文件 | 从 `runtime.Caller(0)` 向上查找 |
| 4 | 硬编码相对路径 fallback | `python`（Win）/ `python3`（Unix） |

**标记文件**: `ml_models/inference.py`、`ml_models/risk_crawler.py`、`scripts/fetch_rim_data.py` 等用于根目录定位。

## Windows 编码与输出

- 新增通过 stdout 与 Go 通信的 Python 脚本时，应设置 `PYTHONIOENCODING=utf-8` 并视情况 monkey-patch `tqdm`，防止进度条破坏 JSON 输出。
- 当前 `fetch_hk_financials.py`、`fetch_hk_profile.py`、`update_policy_library.py` 均已做此处理。
