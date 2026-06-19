<!-- AGENTS.md for stockfinlens / StockFinLens -->
<!-- 本文件面向 AI 编码助手。修改项目结构或构建流程后，请同步更新此文件。 -->

# Agent Guidelines for StockFinLens

> 本文件面向 AI 编码助手。项目主要使用**中文**进行注释和文档编写。

## 项目概述

**StockFinLens（股票财报透镜）** 是一款基于 Wails v2 的跨平台桌面股票财报透视分析工具，支持 A 股与港股。它通过多层分析引擎（财报透镜财务透视、A-Score 风险评分、可比公司横向对比、ML 三引擎预测、RIM 估值、技术形态与交易活跃度分析等）生成深度 Markdown 分析报告。

- **模块名**: `github.com/liusaipu/stockfinlens`
- **当前版本**: `1.4.0`（唯一来源：`wails.json` 的 `info.productVersion`；前端 `Settings.tsx` 通过 Vite `define` 在构建期注入 `__APP_VERSION__`，**不再硬编码**）
- **本地数据目录**: `~/.config/stock-analyzer/`

## 技术栈

| 层级 | 技术 | 版本/说明 |
|------|------|-----------|
| 桌面框架 | Wails v2 | `v2.12.0` |
| 后端 | Go | `1.25.0`（`go.mod` 硬性要求，不可降级） |
| 前端 | React + TypeScript + Vite | React `^18.2.0`、TypeScript `^5.0.0`、Vite `^5.0.0` |
| 图表 | Apache ECharts、lightweight-charts、recharts | `^6.0.0`、`^5.1.0`、`^2.10.0` |
| Markdown 渲染 | react-markdown + rehype/rehype 插件 | `^10.1.0` |
| ML 推理 | Python 3 + ONNX Runtime + scikit-learn + numpy | 运行时通过 `deps_manager.go` 自动检测 |
| 数据获取 | 东方财富 API、腾讯财经接口、StockFinLens Pro、Yahoo、akshare（港股）、同花顺 CSV/Excel 导入 |
| Excel/CSV | `github.com/xuri/excelize/v2`、标准库 `encoding/csv` | excelize `v2.10.1` |
| 通知 | `git.sr.ht/~jackmordaunt/go-toast/v2`（Windows Toast）| `v2.0.3` |

## 关键配置文件

| 文件 | 用途 |
|------|------|
| `wails.json` | Wails 应用配置：版本号、前端目录、构建命令、dev server URL (`http://localhost:5173`) |
| `go.mod` / `go.sum` | Go 依赖管理。核心依赖：`wails/v2 v2.12.0`、`excelize/v2 v2.10.1`、`go-toast/v2 v2.0.3`、`golang.org/x/text v0.35.0`、`golang.org/x/sync v0.20.0` |
| `frontend/package.json` | 前端依赖（注意：`version` 字段为 `0.1.0`，**不参与**应用版本同步） |
| `frontend/tsconfig.json` | TypeScript 主配置：`target: ES2020`、`jsx: react-jsx`、严格模式、`noUnusedLocals: true`、`noUnusedParameters: true`、`noFallthroughCasesInSwitch: true` |
| `frontend/tsconfig.node.json` | Vite 配置文件专用 TS 配置（`composite: true`），解决 Node 与浏览器环境类型冲突 |
| `frontend/vite.config.ts` | Vite 构建配置：`outDir: 'dist'`、`emptyOutDir: true`；构建期读取 `../wails.json` 注入 `__APP_VERSION__` |
| `requirements.txt` | Python 运行时依赖：`pandas>=2.0.0`、`numpy>=1.24.0`、`akshare>=1.12.0`、`requests>=2.31.0`、`openpyxl>=3.1.0`、`tqdm>=4.65.0` |
| `build-windows.ps1` | Windows 构建/打包/清理脚本（命令：`setup`、`dev`、`build`、`package`、`clean`），含环境检查 |
| `build-release.sh` | macOS/Windows 发布构建脚本（Bash，参数：`mac`、`windows`、`all`） |
| `.vscode/launch.json` | VS Code 调试配置：Wails Dev Mode、Debug Windows、Attach |
| `.vscode/tasks.json` | VS Code 任务：wails build/debug、frontend install/build、python venv、clean |
| `docs/BUILD.md` | 更详细的构建指南（供人类阅读） |

## 项目结构

```
stockfinlens/
├── main.go                       # Wails 入口：embed 前端 dist、股票库、ML 模型文件
├── app.go                        # App 结构体与 Wails 绑定方法（~3500 行）
├── app_analysis.go               # 分析流程编排：AnalyzeStock / QuickAnalyzeStock（~900 行）
├── storage.go                    # 本地文件存储管理器（JSON 持久化，~1150 行）
├── csvparser.go                  # 同花顺 CSV/Excel 财报导入解析器（100+ 科目别名标准化）
├── deps_manager.go               # Python 依赖检测与一键安装管理器
├── sysproc_windows.go            # Windows 构建标签：exec.Cmd 隐藏窗口
├── sysproc_other.go              # 非 Windows 构建标签：exec.Cmd 空操作
├── integration_test.go           # 端到端集成测试（使用真实 CSV 数据 603501）
├── storage_test.go               # 存储层单元测试（归档、清理、历史列表）
├── regression_test.go            # 端到端回归测试（603501 报告模块完整性、评分一致性）
├── app_test.go                   # AppConfig 持久化、版本号、自选股排序测试
├── wails.json                    # Wails 配置（版本号唯一来源）
├── go.mod / go.sum               # Go 依赖
├── requirements.txt              # Python 依赖
├── build-windows.ps1             # Windows 构建/打包/清理脚本
├── build-release.sh              # macOS/Windows 发布构建脚本
│
├── analyzer/                     # 核心分析引擎（纯逻辑，无网络 I/O，32 个文件）
│   ├── engine.go                 # 财报透镜分析主入口（RunAnalysisWithAll 等）
│   ├── steps.go                  # 财报透镜财务透视具体实现
│   ├── evaluator.go              # 评分与评级逻辑（100 分制，A-F 等级）
│   ├── report.go                 # Markdown 报告生成器（16 模块）
│   ├── ascore_module.go          # A-Score 风险评分报告渲染（模块 8）
│   ├── risk_analysis.go          # A-Score 六维风险计算（M-Score/Z-Score/现金偏离/应收异常/毛利率/爬虫）
│   ├── risk_alert.go             # 风险预警汇总（一票否决 + 中风险标记 + 敏感度分级）
│   ├── risk_radar.go             # 风险雷达图数据构建
│   ├── comparable.go             # 可比公司聚焦分析（Min-Max 百分位排名）
│   ├── industry.go               # 行业均值数据库与横向对比
│   ├── policy.go                 # 十五五政策匹配度评估
│   ├── rim.go                    # RIM 剩余收益模型估值（多期悲观/基准/乐观情景）
│   ├── technical.go              # 技术形态分析（MA/MACD/RSI/布林带/W 底/M 顶）
│   ├── activity.go               # 交易活跃度分析（换手率/量比/金额得分，行业相对调整）
│   ├── ml_features.go            # ML 特征工程（构建 Engine A/B/D 输入）
│   ├── ml_inference.go           # 调用 Python ONNX 推理脚本（stdin/stdout JSON）
│   ├── data.go                   # 财务数据加载与清洗（自动修复缺失权益/归母净利润）
│   ├── types.go                  # 核心类型定义（CalcTrace、StepResult、AnalysisReport 等）
│   ├── sysproc_windows.go        # Windows 构建标签：隐藏 Python 子进程窗口
│   ├── sysproc_other.go          # 非 Windows 构建标签：空操作
│   ├── activity_test.go          # 活跃度计算测试
│   ├── ascore_validation_test.go # A-Score 验证测试（10 只真实股票网络 smoke test）
│   ├── data_test.go              # 数据修复逻辑测试（fixMissingData）
│   ├── policy_test.go            # 政策匹配测试
│   └── report_test.go            # 报告生成测试
│
├── ai_researcher/                # AI 投研搜索（大模型 + Tavily 联网搜索）
│   ├── config.go                 # AI 配置结构（LLM + 搜索引擎 + 业务偏好）
│   ├── types.go                  # AI 投研输出结构（报告、模块、来源）
│   ├── tavily.go                 # Tavily 搜索客户端（多查询并发、域名过滤）
│   ├── llm.go                    # Kimi/DeepSeek OpenAI-compatible LLM 客户端
│   ├── prompts.go                # 系统提示词与 4 维度搜索查询模板
│   ├── cache.go                  # 按 symbol 缓存搜索结果与分析结论
│   ├── researcher.go             # 搜索 → LLM → 结构化报告编排
│   └── ai_researcher_test.go     # 配置、查询构建、LLM 输出解析测试
│
├── downloader/                   # 数据下载与爬取层（所有网络 I/O，28 个文件）
│   ├── eastmoney_moneyflow.go    # 东财资金流向接口（多 CDN fallback）
│   ├── data_router.go            # 数据源路由：StockFinLens Pro vs 备用源，按类别切换
│   ├── eastmoney.go              # 东方财富 API（财报、资料、行情、K线、概念，~1360 行）
│   ├── sfl_datasource.go         # StockFinLens Pro HTTP JSON API 封装（~800 行）
│   ├── tencent.go                # 腾讯财经 f10 财报与实时行情备用源
│   ├── yahoo.go                  # Yahoo Finance K线与行情备用源（HK/A 股映射）
│   ├── sentiment.go              # 舆情情绪数据抓取（东财研报/公告/新浪新闻三层回退）
│   ├── risk_crawler.go           # A-Score 非财务风险爬虫（股权质押、问询函、减持）
│   ├── rim_data.go               # RIM 外部参数获取（无风险利率、Beta、EPS 预测）
│   ├── hot_concept.go            # 热门概念板块实时排行（东财 API + 综合打分 + 4h 缓存）
│   ├── concept.go                # 股票概念与风口数据（行业/经营范围双映射 + Wind 标签计算）
│   ├── auditor.go                # 审计师变更历史（cninfo 公告查询）
│   ├── exec_changes.go           # 高管/财务负责人变更（cninfo 公告查询）
│   ├── litigation.go             # 诉讼仲裁违规担保（cninfo 公告查询）
│   ├── industry_updater.go       # 行业均值数据库更新（调用 Python 脚本）
│   ├── policy_updater.go         # 政策库更新（调用 Python 脚本）
│   ├── mapping.go                # HSF10 英文科目名映射与标准化
│   ├── validator.go              # 多源数据校验（datacenter-web 交叉验证）
│   ├── storage.go                # 下载器侧简单存储辅助
│   ├── sysproc_windows.go        # Windows 构建标签：隐藏 Python 子进程窗口
│   ├── sysproc_other.go          # 非 Windows 构建标签：空操作
│   ├── downloader_test.go        # 下载器单元测试（603501 真实数据）
│   ├── hot_concept_test.go       # 热门概念 API 响应解析与综合打分测试
│   ├── analyzer_integration_test.go # 下载器与分析器集成测试
│   └── eastmoney_kline_test.go   # K线数据解析测试（验证偏移格式与标准格式）
│
├── updater/                      # 自动更新（GitHub Release 检测、多源下载、跨平台安装）
│   ├── updater.go                # 更新核心逻辑
│   ├── updater_darwin.go         # macOS 安装实现
│   ├── updater_linux.go          # Linux 安装实现
│   ├── updater_windows.go        # Windows 安装实现
│   └── updater_test.go           # 自动更新纯逻辑测试
│
├── tray/                         # 系统托盘（macOS/Windows 滚动行情）
│   ├── tray.go
│   ├── tray_darwin.go
│   ├── tray_darwin.m
│   ├── tray_other.go
│   └── trayicon.png
│
├── frontend/                     # React + TypeScript 前端
│   ├── package.json              # 前端依赖
│   ├── tsconfig.json             # TS 主配置
│   ├── tsconfig.node.json        # Vite 配置专用 TS 配置
│   ├── vite.config.ts            # Vite 配置
│   ├── index.html                # 入口 HTML（lang="zh-CN"，深色背景）
│   ├── src/
│   │   ├── App.tsx               # 主界面组件（~4140 行，三栏布局，所有全局状态）
│   │   ├── Settings.tsx          # 设置面板（版本号通过 vite define 注入 `__APP_VERSION__`，~566 行）
│   │   ├── UnifiedChart.tsx      # K线统一图表（ECharts，5 格子图：K线+成交量+MACD+RSI+布林带）
│   │   ├── KlineChart.tsx        # K线迷你图表（lightweight-charts）
│   │   ├── indicatorCharts.tsx   # 技术指标子图（MACD/RSI/布林带，lightweight-charts）
│   │   ├── FinancialTrendDrawer.tsx # 财务趋势抽屉（5 年 ROE/毛利率/营收增速/现金含量/负债率）
│   │   ├── AIResearchPanel.tsx   # AI 投研报告展示面板（4 模块 + 来源链接）
│   │   ├── ModuleCopyButton.tsx  # 报告模块复制/导出按钮（Markdown/纯文本/PNG）
│   │   ├── PythonDepsModal.tsx   # Python 依赖安装弹窗（监听 `python:install:progress`）
│   │   ├── ErrorBoundary.tsx     # 错误边界
│   │   ├── stocks.ts             # 内置股票代码库（~8250 行，~600KB）
│   │   ├── main.tsx              # 前端入口
│   │   ├── api/                  # 前端 API 封装层（wrap.ts/errors.ts + 领域 facade）
│   │   │   ├── wrap.ts           # Proxy 包装所有 Wails 绑定方法，统一错误处理
│   │   │   ├── errors.ts         # 错误标准化（AppError、retryable 标记）
│   │   │   ├── index.ts          # Barrel export
│   │   │   ├── analysis.ts       # 分析相关 API
│   │   │   ├── watchlist.ts      # 自选股 API
│   │   │   ├── data.ts           # 数据下载 API
│   │   │   ├── quotes.ts         # 行情 API
│   │   │   ├── profile.ts        # 股票资料 API
│   │   │   ├── report.ts         # 报告 API
│   │   │   ├── settings.ts       # 设置 API
│   │   │   └── admin.ts          # 管理 API
│   │   └── components/           # 通用组件
│   │       ├── RiskBadge.tsx     # 风险等级徽章
│   │       └── RiskAlertBanner.tsx # 可展开风险预警横幅
│   └── wailsjs/                  # Wails 生成的 Go 绑定代码
│       ├── go/main/App.d.ts      # Go 方法 TS 声明（~160 行，60+ 函数）
│       ├── go/main/App.js        # Go 方法 JS 桥接
│       ├── go/models.ts          # Go 结构体 TS 类（~1300 行）
│       └── runtime/              # Wails 运行时类型与实现
│
├── ml_models/                    # ML 模型与推理脚本（打包时必须包含）
│   ├── inference.py              # 统一推理入口（Engine A/B/D），Go 通过 stdin/stdout JSON 调用
│   ├── check_env.py              # Python 环境检测脚本
│   ├── risk_crawler.py           # 风险爬虫 Python 辅助（akshare/cninfo）
│   ├── engine_a_sentiment/       # 情绪+量价融合 ONNX 模型（sentiment_price_fusion.onnx）
│   │   ├── model.py              # PyTorch Transformer 双编码器+交叉注意力架构
│   │   ├── train.py              # 训练脚本（focal loss + BCE，支持合成数据）
│   │   └── export_onnx.py        # 导出 ONNX（opset 11）
│   ├── engine_b_financial/       # 财务 LSTM ONNX 模型（financial_lstm.onnx）+ scaler.pkl
│   │   ├── model.py              # PyTorch BiLSTM + Self-Attention + 4 任务头
│   │   ├── features.py           # 8 季度窗口 8 维财务特征提取
│   │   ├── train.py              # 训练脚本（支持本地真实数据或合成数据）
│   │   └── export_onnx.py        # 导出 ONNX（opset 11）
│   └── engine_d_risk/            # GradientBoosting 风险预警模型（engine_d_model.pkl）
│       ├── feature_engineering.py # 26 维风险特征工程（14 财务+6 市场+6 非财务）
│       ├── train.py              # scikit-learn 训练脚本
│       └── model/
│           ├── engine_d_model.pkl # 运行时 pickled 模型
│           ├── config.json        # 模型元数据
│           └── sample_predictions.json # 样例预测
│
├── scripts/                      # Python 数据脚本（打包时必须包含，19+ 个文件）
│   ├── cninfo_utils.py           # 巨潮资讯网共享工具（get_org_id、query_announcements）
│   ├── fetch_all_industry_data.py       # 全市场行业数据采集（akshare fallback）
│   ├── fetch_hk_financials.py           # 港股财报获取（akshare，映射为 A 股科目名）
│   ├── fetch_hk_profile.py              # 港股基本资料获取
│   ├── fetch_rim_data.py                # RIM 外部数据获取（EPS 预测/十年国债/Beta）
│   ├── fetch_auditor_history.py         # 审计师变更历史（cninfo）
│   ├── fetch_exec_changes.py            # 高管变更（cninfo）
│   ├── fetch_litigation.py              # 诉讼/违规/担保（cninfo）
│   ├── update_industry_database.py      # 行业数据库更新（扫描本地数据算行业均值）
│   ├── update_policy_library.py         # 政策库更新（硬编码十五五+实时新闻 enrich）
│   ├── collect_fraud_cases.py           # 欺诈案例收集（ST/*ST/退市/手工案例）
│   ├── prepare_negative_samples.py      # 负样本准备（3:1 配比）
│   ├── test_rim_dinglong.py             # RIM 测试脚本（鼎龙股份 300054）
│   ├── generate_icons.py                # 图标生成（PIL，Windows ICO + macOS Iconset）
│   ├── generate_logo_set.py             # Logo 生成
│   ├── generate_readme_assets.py        # README 素材生成
│   ├── create-release.ps1               # Windows 发布脚本
│   └── run-regression.sh                # 统一回归测试入口（quick/full）
│
├── cmd/
│   └── validate-activity/        # 活跃度验证 CLI 工具（50 只样本股批量验证）
│
├── data/
│   └── stocks.json               # 内置 A 股/港股代码库（被 embed，~41000 行）
│
├── financial-analysis-18steps/   # 独立 Python 参考实现（18 步分析 + RIM，Go 不调用）
│   └── scripts/analyzer_18steps.py
│
└── docs/                         # 文档与截图
    ├── BUILD.md                  # 详细构建指南
    ├── ML_PREDICTION_DESIGN.md   # ML 预测设计文档
    ├── ml_architecture.png       # 架构图
    └── screenshots/              # 界面截图
```

## 构建与运行

### 环境要求

- **Go** >= `1.25.0`（`go.mod` 硬性指定，不可降级）
- **Node.js** >= 18
- **Python** 3.10+（运行时必需，用于 ML 推理与数据脚本）
- **Wails CLI** >= v2.12：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **GCC / MinGW-w64**（Windows 构建必需）

### 安装依赖

```bash
# Go 依赖
go mod tidy

# 前端依赖
cd frontend && npm install

# Python 依赖（运行时必需）
pip install -r requirements.txt
# 核心运行时额外依赖：onnxruntime, scikit-learn, numpy
```

### 常用命令

```bash
# 开发模式（热重载前端 + Go）
wails dev

# 构建生产版本（Windows，推荐用脚本）
.\build-windows.ps1 build

# 打包为 ZIP（包含 ml_models 和 scripts）
.\build-windows.ps1 package

# macOS 发布构建
./build-release.sh mac

# 运行全部 Go 测试
go test ./...

# 运行特定包测试
go test ./analyzer/...
go test ./downloader/...
go test ./updater/...

# 运行前端测试
cd frontend && npm test

# 统一回归测试（quick / full）
./scripts/run-regression.sh quick
./scripts/run-regression.sh full
```

### 构建注意事项

1. **版本号唯一来源**: `wails.json` 中的 `info.productVersion` 是应用版本的**唯一来源**。前端通过 `frontend/vite.config.ts` 在构建期读取并以 `define` 注入全局常量 `__APP_VERSION__`，`Settings.tsx` 直接引用该常量，**禁止重新硬编码**。构建脚本不再做一致性校验（版本自然一致）。当前版本为 `1.6.9`。
2. **前端 dist 重建**: 如果前端代码有变更，构建前必须确保 `frontend/dist` 是最新的。Wails `build` 在 `dist` 已存在时可能跳过前端构建，导致打包旧代码。`build-release.sh` 会强制先执行 `cd frontend && npm run build`。Windows 构建建议同样手动前置该步骤。
3. **打包产物必须包含**: `ml_models/` 和 `scripts/` 目录。Go 后端在运行时会从可执行文件同级目录查找这些路径。
4. **开发模式 vs 生产模式**: `main.go` 中 `readStockJSON()` 优先读取本地 `data/stocks.json`，打包后 fallback 到 `embed.FS`。
5. **跨平台构建标签**: `main`、`analyzer`、`downloader` 三个包均包含 `sysproc_windows.go`（`//go:build windows`）和 `sysproc_other.go`（`//go:build !windows`），用于隔离 Windows 特有的 `syscall.SysProcAttr{HideWindow: true}`，避免 macOS/Linux 编译失败。新增包若需调用 Python 子进程，应遵循同样模式。
6. **构建脚本硬编码路径**: `build-release.sh` 中 Wails CLI 路径硬编码为 `/Users/lobster/go/bin/wails`，在他人机器上构建时需确保路径存在或创建软链接。

## 代码组织规范

### 包划分

| 包 | 职责 |
|----|------|
| `main` | Wails 应用生命周期、App 绑定方法、存储管理、CSV/Excel 解析、Python 依赖管理、系统托盘 |
| `analyzer` | 纯分析逻辑，不依赖网络，输入为本地财务数据 + 外部传入的行情/舆情/可比公司数据 |
| `ai_researcher` | AI 投研搜索：调用 Tavily 搜索互联网公开信息，通过 Kimi/DeepSeek 生成结构化投研报告 |
| `downloader` | 所有网络 I/O：财报下载、行情、K线、舆情、风险爬虫、外部数据获取、数据源路由 |
| `updater` | 自动更新：GitHub Release 检测、多源下载、跨平台安装 |
| `tray` | 系统托盘集成（macOS/Windows） |

### 核心数据流

1. 用户在 `App.tsx` 选择股票 -> 调用 `AddToWatchlist`
2. 下载财报：`DownloadReports` -> `downloader.DataRouter` -> 多源获取 -> 保存到 `~/.config/stock-analyzer/data/{symbol}/`
3. 自动更新：`startup` -> 后台检查 GitHub API -> `update:available` Event -> `UpdateModal` -> `DownloadUpdate`（gh-proxy.com 加速镜像优先）-> `ApplyUpdate`（Windows: bat 替换+重启 / macOS: open dmg）
4. 执行分析：`AnalyzeStock` -> `analyzer.RunAnalysisWithAll` -> 生成 `AnalysisReport` -> 保存 Markdown 报告与 JSON 快照
5. 前端读取快照恢复亮点/风险面板，读取 Markdown 渲染报告
6. AI 投研：用户点击"AI 投研" -> `AnalyzeStockWithAI` -> `ai_researcher.Research` -> Tavily 多维度搜索 -> LLM 生成结构化报告 -> 缓存到 `data/{symbol}/ai_research_cache.json` -> 前端 `AIResearchPanel` 渲染
7. 市场热点：用户点击"刷新" -> `FetchHotConcepts` -> `downloader.FetchHotConceptBoard` -> 东财 API -> 综合打分排序 -> 缓存到 `data/hot_concepts/latest.json` + 归档历史 -> 前端展示 Top 20 热门概念及成分股

### 并发模型

- **单次分析互斥**: `app.analysisLocks[symbol]` 防止同一只股票重复分析
- **分析内部并行**: `analyzeStockInternal` 使用 `sync.WaitGroup` 并发拉取 quote/klines/sentiment/moneyflow（4 个网络 goroutine）以及 ML/RIM/risk crawler/external risk（3–4 个数据 goroutine）
- **快速分析**: `QuickAnalyzeStock` 采用类似的并行 goroutine 模式
- **自选股活跃度**: 批量并发获取，带缓存
- **请求去重**: `singleflight.Group` 合并同一股票的并发分析请求，避免重复计算

### 股票代码格式

- A 股上海：`603501.SH`
- A 股深圳：`000001.SZ`
- 港股：`00700.HK`
- 内部存储和 UI 传递均使用上述带点格式

### 缓存策略

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

### AI 投研配置参数

配置文件：`~/.config/stock-analyzer/ai_config.json`

| 层级 | 参数 | 说明 | 默认值 |
|------|------|------|--------|
| 连接层 | `llm_provider` | LLM 供应商：`kimi` / `deepseek` | `deepseek` |
| 连接层 | `llm_api_key` | LLM API Key | `''` |
| 连接层 | `llm_base_url` | OpenAI-compatible 端点 | `https://api.deepseek.com/v1` |
| 连接层 | `llm_model` | 模型名，支持手动输入未来版本 | `deepseek-chat` |
| 连接层 | `llm_timeout` | 请求超时（秒） | `90` |
| 生成层 | `temperature` | 创造性/稳定性控制 | `0.2` |
| 生成层 | `max_tokens` | 单次输出上限 | `4096` |
| 生成层 | `top_p` | 采样多样性 | `1.0` |
| 搜索层 | `search_provider` | 搜索引擎：`tavily` | `tavily` |
| 搜索层 | `search_api_key` | Tavily API Key | `''` |
| 搜索层 | `search_depth` | `basic` / `advanced` | `advanced` |
| 搜索层 | `max_results` | 每次查询返回条数 | `10` |
| 搜索层 | `search_recency_days` | 只搜最近 N 天 | `90` |
| 业务层 | `focus_regions` | 国际市场关注区域 | `['us', 'jp']` |
| 业务层 | `output_language` | 输出语言 | `zh-CN` |
| 业务层 | `enable_social` | 是否抓取社交情绪 | `true` |
| 业务层 | `cache_ttl_hours` | 缓存有效期 | `6` |

### 分析引擎的财报透镜流程

在 `analyzer/engine.go` 中按顺序执行：
1. 审计意见 → 2. 资产规模 → 3. 偿债能力 → 4. 竞争地位 → 5. 应收账款 → 6. 固定资产 → 7. 投资资产 → 8. **风险分析（A-Score）** → 9. 营收增长 → 10. 毛利率 → 11. 运营效率 → 12. 成本控制 → 13. 费用率 → 14. 核心利润 → 15. 现金流质量 → 16. ROE → 17. 资本支出 → 18. 分红政策

## 测试策略

### 分层测试体系（L1/L2/L3）

| 层级 | 类型 | 命令 | 用途 |
|------|------|------|------|
| **L1** | Go 后端快速回归 | `go test -short ./...` | CI/CD 入口，无网络，~1s |
| **L1** | Go 后端完整 | `go test ./...` | 发布前验证，含端到端 |
| **L2** | 前端组件 | `cd frontend && npm test` | Vitest + React Testing Library |
| **L3** | E2E（手动） | Playwright（待配置） | 关键用户旅程验证 |

统一回归入口：`./scripts/run-regression.sh [quick|full]`

### Go 后端测试文件分布（共 15 个 `*_test.go`）

**ai_researcher 包（1 个）**：
- `ai_researcher/ai_researcher_test.go` — 配置默认值、查询构建、LLM 输出解析、来源去重测试

**analyzer 包（5 个）**：
- `analyzer/activity_test.go` — 活跃度计算测试（大/中/小市值模拟）
- `analyzer/ascore_validation_test.go` — A-Score 验证测试（10 只真实股票网络 smoke test）⚠️ `-short` 跳过
- `analyzer/data_test.go` — 数据修复逻辑测试（fixMissingData）
- `analyzer/policy_test.go` — 政策匹配测试
- `analyzer/report_test.go` — 报告生成测试（验证 Markdown 结构、废弃行为）

**downloader 包（4 个）**：
- `downloader/downloader_test.go` — 下载器单元测试（603501 真实数据 + 数据校验）⚠️ `-short` 跳过
- `downloader/hot_concept_test.go` — 热门概念 API 响应解析与综合打分测试
- `downloader/analyzer_integration_test.go` — 下载器与分析器集成测试 ⚠️ `-short` 跳过
- `downloader/eastmoney_kline_test.go` — K线数据解析测试（验证偏移格式与标准格式）

**main/updater 包（5 个）**：
- `app_test.go` — Wails 绑定方法测试（Config 持久化、版本号、自选股排序）
- `regression_test.go` — 端到端回归测试（603501 报告模块完整性、存储 CRUD、评分一致性）⚠️ `-short` 跳过
- `storage_test.go` — 存储层测试（归档、清理、历史列表）
- `integration_test.go` — 端到端集成测试（使用 603501 真实 CSV 数据）⚠️ `-short` 跳过
- `updater/updater_test.go` — 自动更新纯逻辑测试（版本比对、时间格式化、asset 匹配、多源下载）

### 前端测试（L2）

框架：**Vitest** + **React Testing Library** + **jsdom** + `@testing-library/jest-dom`

```bash
cd frontend
npm test          # 运行所有测试
npm run test:ui   # 打开 UI 界面
```

测试文件（3 个）：
- `frontend/src/Settings.test.ts` — Settings 工具函数测试（loadSettings/saveSettings、默认值、非法 JSON 回退）
- `frontend/src/components/RiskBadge.test.tsx` — 风险徽章组件渲染测试（low/medium/high、尺寸）
- `frontend/src/components/RiskAlertBanner.test.tsx` — 风险横幅交互测试（展开/收起、flags 渲染）

> `App.tsx` 等复杂组件的测试需要先 mock Wails 运行时绑定，目前暂无自动化测试。

### 已知测试覆盖缺口

| 领域 | 说明 |
|------|------|
| `App.tsx` 主组件 | ~4140 行单文件大组件，无自动化测试 |
| `AIResearchPanel.tsx` | AI 投研报告面板无自动化测试 |
| ML 推理引擎 | `ml_models/inference.py` 及 Engine A/B/D 无 Python 单元测试 |
| RIM 估值模型 | `analyzer/rim.go` 无独立单元测试 |
| 技术形态分析 | `analyzer/technical.go` 无测试 |
| 可比公司横向对比 | `analyzer/comparable.go` 无测试 |
| CSV/Excel 导入解析 | `csvparser.go` 无测试 |
| 数据源路由多源 fallback | 异常场景（CDN 切换、超时、限流）缺乏故障注入测试 |
| 舆情情绪抓取 | `downloader/sentiment.go` 三层回退逻辑无测试 |
| Python 依赖管理器 | `deps_manager.go` 无测试 |
| 前端图表组件 | `UnifiedChart.tsx`、`KlineChart.tsx` 等无测试 |
| E2E | Playwright 待配置 |

## ML 模型架构

| 引擎 | 模型 | 输入维度 | 输出 | 运行时格式 |
|------|------|----------|------|------------|
| Engine A | SentimentPriceFusion Transformer | text_seq[16,32] + price_seq[16,24] | 方向(down/flat/up) + 异常概率 | ONNX (`sentiment_price_fusion.onnx`) |
| Engine B | FinancialLSTM (BiLSTM+Self-Attention) | financial_seq[8,N_features] | ROE方向/营收方向/M-Score方向 + 健康分 | ONNX (`financial_lstm.onnx`) + scaler.pkl |
| Engine D | GradientBoostingClassifier | 25 维风险向量 | 风险标签/概率/等级 + top-3 因子 | pickle (`engine_d_model.pkl`) |

- **Engine A/B**: 运行时通过 `onnxruntime` 的 `CPUExecutionProvider` 推理。
- **Engine D**: 通过 `pickle.load()` 加载模型；若加载失败则优雅降级为基于规则的风险评估。
- **特征维度注意**: `engine_d_risk/feature_engineering.py` 定义了 26 个特征（含 `cfo_change_count_2y`），但训练脚本与推理入口实际使用 25 维，`cfo_change` 相关特征被静默丢弃。新增特征时需同步训练与推理代码。

## Python ↔ Go 集成规范

### 通信协议

所有 Python 脚本被 Go 调用时均使用 **stdin/stdout JSON**，不使用 HTTP、文件或 socket：
1. Go 将请求 JSON 写入 `cmd.StdinPipe()`
2. Python 通过 `json.load(sys.stdin)` 读取，处理后 `print(json.dumps(result))`
3. Go 读取 `cmd.Output()` 并反序列化为类型化结构体

### 环境变量

调用 Python 子进程时，Go 会注入：
- `TQDM_DISABLE=1` — 防止进度条污染 stdout
- `PYTHONUNBUFFERED=1` — 禁用输出缓冲

### 路径解析策略（开发与打包双模式）

Python 脚本与模型文件的路径解析采用 **4 级回退**：

| 优先级 | 脚本路径 | Python 可执行文件 |
|--------|----------|-------------------|
| 1 | `os.Executable()` 所在目录（打包后的 Windows/macOS） | 同目录下的 `.venv/bin/python3` 或 `.venv/Scripts/python.exe` |
| 2 | macOS `.app` bundle 的 `Contents/Resources/` | macOS `.app` bundle 内的 `.venv` |
| 3 | `runtime.Caller(0)` 向上查找标记文件 | 从 `runtime.Caller(0)` 向上查找 |
| 4 | 硬编码相对路径 fallback | `python`（Win）/ `python3`（Unix） |

**标记文件**: `ml_models/inference.py`、`ml_models/risk_crawler.py`、`scripts/fetch_rim_data.py` 等用于根目录定位。

### Windows 编码与输出

- 新增通过 stdout 与 Go 通信的 Python 脚本时，应设置 `PYTHONIOENCODING=utf-8` 并视情况 monkey-patch `tqdm`，防止进度条破坏 JSON 输出。
- 当前 `fetch_hk_financials.py`、`fetch_hk_profile.py`、`update_policy_library.py` 均已做此处理。

## 代码风格指南

### Go
- 遵循标准 Go 代码风格，提交前运行 `go fmt ./...`
- 优先使用项目已有的错误处理方式（返回 `fmt.Errorf("...: %w", err)`）
- 涉及并发操作时，务必添加 `recover()` 防止 panic 扩散
- 所有代码注释保持中文
- 新增包若需在 Windows 隐藏 Python 子进程窗口，必须创建 `sysproc_windows.go`（`//go:build windows`）和 `sysproc_other.go`（`//go:build !windows`），并提供统一的 `setHideWindow(cmd *exec.Cmd)` 函数

### TypeScript / React
- 前端使用 React + TypeScript，确保 `npm run build` 无类型错误
- `tsconfig.json` 启用了 `strict: true`、`noUnusedLocals: true`、`noUnusedParameters: true`、`noFallthroughCasesInSwitch: true`，未使用的变量/参数会导致构建失败
- ECharts 数据处理时避免传入 `null`，统一使用 `'-'` 或 `undefined`
- 前端状态管理集中在 `App.tsx`（~4140 行单文件大组件），**不使用 Redux/Zustand 等外部状态管理库**。新增功能时优先在现有 hooks 体系内扩展
- CSS 采用组件级 CSS 文件（`App.css`、`Settings.css` 等），不使用 CSS-in-JS
- 主题系统：默认深色，通过 `document.body.classList.add('light')` 切换亮色模式
- `frontend/src/api/` 是前端与 Go 后端通信的封装层，包含 `wrap.ts`（Proxy 包装 + 错误捕获）、`errors.ts`（错误标准化）及领域 facade 文件。新增 Wails 绑定方法时，同步在对应领域文件中导出

### Python
- ML 脚本位于 `ml_models/` 和 `scripts/`，保持与现有推理接口兼容
- ONNX 模型导出后请验证与 Go 端的推理结果一致
- `inference.py` 通过 stdin/stdout JSON 与 Go 通信，不要改变该接口格式
- Python 脚本路径解析：开发时通过 `runtime.Caller(0)` 或 `__file__` 定位；打包后优先使用 `os.Executable()` 所在目录
- 新增 stdout 通信脚本需处理 UTF-8 编码与 tqdm 进度条屏蔽

### Commit 规范
建议遵循以下前缀：

| 前缀 | 用途 |
|------|------|
| `feat:` | 新功能 |
| `fix:` | 修复 bug |
| `docs:` | 文档更新 |
| `ui:` | 界面/交互优化 |
| `chore:` | 构建/版本/工具链等杂项 |
| `refactor:` | 重构（无功能变化） |

## 安全注意事项

- **不要破坏 Wails 绑定**: `app.go` 中导出的方法（首字母大写）会被前端调用，修改签名必须同步更新 `frontend/src/App.tsx` 中的调用以及 `frontend/src/api/` 下对应的 facade 文件。`frontend/wailsjs/go/main/App.d.ts` 与 `App.js` 通常由 `wails dev` 自动生成，不应手动编辑。
- **Python 脚本路径解析**: `ml_models/inference.py` 和 `scripts/*.py` 在开发模式与打包后的路径解析逻辑不同。开发时通过脚本所在目录向上查找；打包后优先使用 `os.Executable()` 所在目录。新增 Python 脚本时请遵循同样的 4 级回退策略。
- **Windows 隐藏窗口**: 所有通过 `exec.Command` 调用 Python 脚本的地方，在 Windows 上必须设置 `syscall.SysProcAttr{HideWindow: true}`，否则会出现 CMD 黑框。当前项目已将该逻辑封装为 `setHideWindow(cmd)`，通过 build tag 隔离平台差异。
- **新增 Go 依赖**: 本项目不使用 `vendor`，直接通过 `go mod` 管理。新增依赖后运行 `go mod tidy`。
- **ML 模型文件**: `main.go` 通过 `//go:embed` 嵌入了前端 dist、股票库及部分 ML 模型文件。新增模型文件时需同步更新 embed 指令；但运行时仍要求模型文件以物理文件形式存在于可执行文件同级目录（Wails embed 仅辅助静态打包）。
- **本地数据安全**: 所有用户数据（自选、财报、报告）保存在 `~/.config/stock-analyzer/`，不涉及云端传输。
- **Python 依赖检测**: `deps_manager.go` 实现了跨平台 Python 查找与 7 个核心包检测（`onnxruntime`、`scikit-learn`、`numpy`、`pandas`、`akshare`、`requests`、`openpyxl`）。其中前 5 个为 `Required: true`，`requests` 与 `openpyxl` 为 `Required: false`。修改检测逻辑时需同步更新 `requiredPackages` 列表与前端的 `PythonDepsModal.tsx`。
- **前端事件**: 前端目前监听 Wails runtime 事件 `update:available`（自动更新通知）和 `python:install:progress`（Python 包安装进度）。新增事件时需在前端相应组件中补充 `EventsOn`/`EventsOff`。

## 发布流程

1. **版本号更新**: 仅需修改 `wails.json` 的 `info.productVersion`（前端构建时自动注入）。
2. **更新 CHANGELOG.md**: 在顶部追加新版本说明。
3. **完整回归测试（硬性要求）**:
   ```bash
   ./scripts/run-regression.sh full
   ```
   - 必须 **全部通过** 才能继续发布
   - 结果自动保存到 `test-results/regression_full_YYYYMMDD_HHMMSS.log`
   - 失败时脚本返回非零 exit code 并阻止后续步骤
4. **提交并推送**: `git commit`, `git push origin main`
5. **打标签**: `git tag v1.4.0`, `git push origin v1.4.0`
6. **构建发布包**:
   - Windows: ` .\build-windows.ps1 package`
   - macOS: `./build-release.sh mac`
7. **创建 GitHub Release**: 上传 ZIP/DMG，将 `CHANGELOG.md` 对应章节粘贴到 Release Notes，并附上 `test-results/latest_summary.md` 测试报告。
8. **版本号递增**: 发布后立刻将 `wails.json` 版本号 bump 到下一个未发布版本（如 `1.4.1`）。
