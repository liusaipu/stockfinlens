# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

StockFinLens (股票财报透镜) is a Wails v2 cross-platform desktop app for A-share/HK stock financial report analysis. It generates deep Markdown analysis reports via a multi-layer engine: financial statement lens, A-Score risk scoring, comparable company analysis, ML triple-engine prediction, RIM valuation, and technical pattern analysis.

- **Tech stack**: Go 1.25.0 backend + React 18 + TypeScript + Vite frontend, Python 3.10+ for ML inference
- **Version source**: `wails.json` `info.productVersion` is the single source of truth. `frontend/vite.config.ts` reads it at build time and injects it as `__APP_VERSION__`; `Settings.tsx` consumes this constant. Do not hardcode version strings in the frontend.
- **Language**: Comments and docs are primarily in Chinese.

## Common Commands

```bash
# Development (hot-reload for both Go and frontend)
wails dev

# Build production binary
wails build

# Build for specific platform
wails build -platform darwin/universal -clean
wails build -platform windows/amd64 -clean

# Run all Go tests
go test ./...

# Run specific package tests
go test ./analyzer/...
go test ./downloader/...
go test ./updater/...

# Run frontend tests
cd frontend && npm test

# Regression tests (quick or full)
./scripts/run-regression.sh quick
./scripts/run-regression.sh full

# Release builds
./build-release.sh mac       # macOS universal DMG
./build-release.sh windows   # Windows amd64 ZIP
./build-release.sh all       # Both (builds mac first, then windows)
```

## High-Level Architecture

### Three-Layer Separation

The codebase enforces a strict separation of concerns across three layers:

1. **Frontend (`frontend/src/`)** — React + TypeScript. All UI state lives in `App.tsx`. API calls go through `frontend/src/api/wrap.ts`, a Proxy that wraps every Wails-generated Go method with unified error normalization (`AppError` from `errors.ts`). Domain facades (`analysis.ts`, `watchlist.ts`, etc.) re-export methods from this proxy.

2. **Wails Binding Layer (`app.go`, `app_analysis.go`)** — The `App` struct exposes ~60+ methods bound to the frontend via Wails. It holds runtime state (`Storage`, `DataRouter`, `MarketCacheManager`, `singleflight.Group` for deduplicating concurrent analysis requests). `app_analysis.go` orchestrates the analysis pipeline: `AnalyzeStock` / `QuickAnalyzeStock`.

3. **Engine & I/O Layer**
   - `analyzer/` — Pure logic, no network I/O. Contains the financial analysis engine (`engine.go`, `steps.go`), report generation (`report.go`, `report_modules.go`), A-Score risk scoring (`risk_analysis.go`, `ascore_module.go`), comparable company analysis (`comparable.go`), ML feature engineering and inference calling (`ml_features.go`, `ml_inference.go`).
   - `downloader/` — All network I/O lives here. Multi-source data fetchers (East Money, Tencent, Yahoo, StockFinLens Pro), data routing (`data_router.go`), hot concept boards, sentiment crawling, and risk crawlers.

### ML Inference (Go ↔ Python)

Go calls Python ONNX inference via `exec.Command` with stdin/stdout JSON:
- `ml_models/inference.py` — Unified entry point for Engine A (sentiment), B (financial), D (risk).
- Go builds feature vectors in `analyzer/ml_features.go`, pipes them as JSON to Python, and parses the JSON response.
- **Cross-platform subprocess**: `main`, `analyzer`, and `downloader` each have `sysproc_windows.go` (`//go:build windows`) and `sysproc_other.go` (`//go:build !windows`) to isolate `HideWindow` syscalls. Any new package that spawns Python subprocesses should follow the same pattern.

### Local Storage

Data directory: `~/.config/stock-analyzer/`
- `watchlist.json` — Watchlist (max 100 stocks)
- `comparables.json` — Comparable company configs
- `data/{symbol}/` — Current financial report JSON
- `data/{symbol}/history/` — Archived batches (last 3 retained)
- `reports/{symbol}/` — Generated Markdown reports

Managed by `storage.go` (root-level `Storage` struct).

## Key Conventions

- **Commit message prefixes**: `feat:`, `fix:`, `docs:`, `ui:`, `chore:`, `refactor:`
- **Go style**: `go fmt ./...` before commit. Use `recover()` in goroutines to prevent panic propagation.
- **Frontend**: `npm run build` must pass with zero TypeScript errors. ECharts data should avoid `null`; use `'-'` or `undefined` instead.
- **ML models**: After exporting ONNX, verify inference results match between Python and Go.

## K-line / Chart Conventions

These are project invariants. Same bugs (date format, z-index/stacking) have surfaced in 3+ fix cycles; document them once, stop hitting them.

- **Date format is `YYYY-MM-DD` across the frontend, no exceptions.** Backend sources return inconsistent formats: 东财 = `YYYY-MM-DD`, 腾讯 = `YYYYMMDD`, 网易 = `YYYY/MM/DD`. Every code path that calls a Kline-returning API (`GetStockKlines`, `RefreshStockKlines`, anything new) **must** run results through `normalizeTime()` in `UnifiedChart.tsx` before storing or aggregating. `aggregateToWeekly` strict-parses `split('-').length === 3` and silently drops mismatched rows → "暂无K线数据" — that's the symptom to recognize.
- **Never parse dates with `new Date(str).toISOString()` for daily-bar timestamps.** Use `split('-')` + `new Date(y, m, day)` instead. `toISOString()` shifts by local timezone offset and returns `Invalid Date` for some inputs; both have caused regressions in weekly aggregation.
- **Full-screen / global overlays go through React Portal at `z-index: 99999`.** ECharts creates DOM outside the chart container (tooltip, zr helpers) with its own z-index; React 18 auto-batching can also expose mid-`setOption` frames. Two patterns to follow when adding modals or chart loading overlays:
  - `createPortal(<div style={{ zIndex: 99999 }}>...</div>, document.body)` — see `UnifiedChart.tsx` refresh mask and `UpdateModal.tsx`.
  - For chart redraws, reuse the echarts instance (mount-once `useEffect`) + `chart.setOption(option, true)` and trigger overlay dismissal from the `'finished'` event, not from the `setState` that started the refresh (auto-batching collapses them into the same commit).
- **When refactoring a component with multiple return paths (loading / empty / main), update ALL branches.** Loading and empty states share container styling that's easy to miss during a "fix the main render" pass.

## Release Checklist

1. Version bump: update only `wails.json` `info.productVersion`. The frontend picks it up automatically via Vite.
2. Force rebuild `frontend/dist` from scratch before `wails build` to avoid embedding stale code.
3. Distribution must include `ml_models/` and `scripts/` directories (Go looks for them next to the executable).
4. Append new version to `CHANGELOG.md`.
5. Create and push a Git tag (e.g., `v1.4.0`).
