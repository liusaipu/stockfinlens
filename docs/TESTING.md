# 测试策略

> 测试分层、命令与已知覆盖缺口。新增或调整测试时同步更新本文件。

## 分层测试体系（L1/L2/L3）

| 层级 | 类型 | 命令 | 用途 |
|------|------|------|------|
| **L1** | Go 后端快速回归 | `go test -short ./...` | CI/CD 入口，无网络，~1s |
| **L1** | Go 后端完整 | `go test ./...` | 发布前验证，含端到端 |
| **L2** | 前端组件 | `cd frontend && npm test` | Vitest + React Testing Library |
| **L3** | E2E（手动） | Playwright（待配置） | 关键用户旅程验证 |

统一回归入口：`./scripts/run-regression.sh [quick|full]`

## Go 后端测试文件分布

**ai_researcher 包（1 个）**:
- `ai_researcher/ai_researcher_test.go` — 配置默认值、查询构建、LLM 输出解析、来源去重测试

**analyzer 包（5 个）**:
- `analyzer/activity_test.go` — 活跃度计算测试（大/中/小市值模拟）
- `analyzer/ascore_validation_test.go` — A-Score 验证测试（10 只真实股票网络 smoke test）⚠️ `-short` 跳过
- `analyzer/data_test.go` — 数据修复逻辑测试（fixMissingData）
- `analyzer/policy_test.go` — 政策匹配测试
- `analyzer/report_test.go` — 报告生成测试（验证 Markdown 结构、废弃行为）

**downloader 包（4 个）**:
- `downloader/downloader_test.go` — 下载器单元测试（603501 真实数据 + 数据校验）⚠️ `-short` 跳过
- `downloader/hot_concept_test.go` — 热门概念 API 响应解析与综合打分测试
- `downloader/analyzer_integration_test.go` — 下载器与分析器集成测试 ⚠️ `-short` 跳过
- `downloader/eastmoney_kline_test.go` — K线数据解析测试（验证偏移格式与标准格式）

**main/updater 包（5 个）**:
- `app_test.go` — Wails 绑定方法测试（Config 持久化、版本号、自选股排序）
- `regression_test.go` — 端到端回归测试（603501 报告模块完整性、存储 CRUD、评分一致性）⚠️ `-short` 跳过
- `storage_test.go` — 存储层测试（归档、清理、历史列表）
- `integration_test.go` — 端到端集成测试（使用 603501 真实 CSV 数据）⚠️ `-short` 跳过
- `updater/updater_test.go` — 自动更新纯逻辑测试（版本比对、时间格式化、asset 匹配、多源下载）

## 前端测试（L2）

框架：**Vitest** + **React Testing Library** + **jsdom** + `@testing-library/jest-dom`

```bash
cd frontend
npm test          # 运行所有测试
npm run test:ui   # 打开 UI 界面
```

测试文件（3 个）：
- `frontend/src/Settings.test.ts` — Settings 工具函数测试
- `frontend/src/components/RiskBadge.test.tsx` — 风险徽章组件渲染测试
- `frontend/src/components/RiskAlertBanner.test.tsx` — 风险横幅交互测试

> `App.tsx` 等复杂组件的测试需要先 mock Wails 运行时绑定，目前暂无自动化测试。

## 已知测试覆盖缺口

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
