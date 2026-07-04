import { useState, useEffect, useMemo, useRef, useCallback, Children, cloneElement } from 'react'
import './App.css'
import { STOCKS } from './stocks'
import { UnifiedChart } from './UnifiedChart'
import { FinancialTrendChart } from './FinancialTrendChart'
import { FinancialTrendDrawer } from './FinancialTrendDrawer'
import { AIResearchPanel } from './AIResearchPanel'
import { MarginDrawer } from './MarginDrawer'
import { Settings, loadSettings, AppSettings } from './Settings'
import { ModuleCopyButton, setGlobalMarkdownContent } from './ModuleCopyButton'
import { PythonDepsModal } from './PythonDepsModal'
import { UpdateModal, UpdateInfo } from './UpdateModal'

import { AnalyzeStockWithAI, LoadAIResearchReport, CancelAIResearch, ExportAIResearchTxt, ExportAIResearchMd, ExportAIResearchPdf } from './api'
import html2pdf from 'html2pdf.js'
import { EventsOn, WindowGetSize } from '../wailsjs/runtime'
import { RiskBadge } from './components/RiskBadge'
import { RiskAlertBanner } from './components/RiskAlertBanner'
import ReactMarkdown from 'react-markdown'
import type { ai_researcher } from '../wailsjs/go/models'

interface AIProgressEvent {
  symbol: string
  stage: string
  message: string
}
import remarkGfm from 'remark-gfm'
import rehypeSlug from 'rehype-slug'

import { pinyin } from 'pinyin-pro'

function formatAmount(val: number, unit: string): string {
  if (!val || val <= 0) return '-'
  const abs = Math.abs(val)
  if (abs >= 1e8) return `${(val / 1e8).toFixed(2)} 亿${unit}`
  if (abs >= 1e4) return `${(val / 1e4).toFixed(2)} 万${unit}`
  return `${val.toFixed(0)} ${unit}`
}

function DetailsComponent({ children, ...props }: any) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDetailsElement>(null)

  useEffect(() => {
    const details = ref.current
    if (!details) return
    if (!open) {
      details.classList.remove('tooltip-left', 'tooltip-top', 'tooltip-center')
      return
    }
    const handleClickOutside = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)

    // 智能定位：检测 inline-tooltip body 是否超出视口
    const rafId = requestAnimationFrame(() => {
      if (!details.classList.contains('inline-tooltip')) return
      const body = details.querySelector('.inline-tooltip-body') as HTMLElement | null
      if (!body) return
      const rect = body.getBoundingClientRect()
      const vw = window.innerWidth
      const vh = window.innerHeight
      details.classList.remove('tooltip-left', 'tooltip-top', 'tooltip-center')
      const fitsRight = rect.right <= vw - 8
      const fitsBottom = rect.bottom <= vh - 8
      const fitsLeft = rect.left >= 8
      const fitsTop = rect.top >= 8
      if (fitsRight && fitsBottom) {
        // 默认右下，无需调整
      } else if (fitsLeft && fitsBottom) {
        details.classList.add('tooltip-left')
      } else if (fitsRight && fitsTop) {
        details.classList.add('tooltip-top')
      } else if (fitsLeft && fitsTop) {
        details.classList.add('tooltip-left', 'tooltip-top')
      } else {
        details.classList.add('tooltip-center')
      }
    })

    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
      cancelAnimationFrame(rafId)
    }
  }, [open])

  const wrappedChildren = Children.map(children, (child: any) => {
    if (!child) return child
    if (child.type === 'summary') {
      return cloneElement(child, {
        onClick: (e: React.MouseEvent) => {
          e.preventDefault()
          setOpen((prev) => !prev)
        }
      })
    }
    return child
  })

  return (
    <details ref={ref} open={open} {...props}>
      {wrappedChildren}
    </details>
  )
}

function InlineTooltip({ title, body }: { title: string; body: string }) {
  const formattedBody = body.split('\n').map((line, i) => (
    <span key={i}>{line}{i < body.split('\n').length - 1 && <br/>}</span>
  ))
  return (
    <span className="inline-tooltip">
      <span className="inline-tooltip-trigger">i</span>
      <span className="inline-tooltip-body">
        <strong>{title}</strong><br/>
        {formattedBody}
      </span>
    </span>
  )
}

function Collapsible({ title, children, defaultExpanded = false }: { title: React.ReactNode; children: React.ReactNode; defaultExpanded?: boolean }) {
  const [expanded, setExpanded] = useState(defaultExpanded)
  return (
    <div className="collapsible-section">
      <div className="collapsible-header" onClick={() => setExpanded(!expanded)}>
        <span className="collapsible-title">{title}</span>
        <span className={`collapsible-toggle ${expanded ? 'expanded' : ''}`}>›</span>
      </div>
      {expanded && <div className="collapsible-body">{children}</div>}
    </div>
  )
}

function InteractQAPanel({ qas, visibleCount, setVisibleCount }: { qas: any[], visibleCount: number, setVisibleCount: (count: number | ((prev: number) => number)) => void }) {
  const containerRef = useRef<HTMLDivElement>(null)

  if (!qas || qas.length === 0) {
    return <div style={{ fontSize: 13, color: 'var(--text-muted)', padding: '8px 0' }}>暂无互动平台问答数据。</div>
  }
  const visible = qas.slice(0, visibleCount)
  const hasMore = visibleCount < qas.length
  const canCollapse = visibleCount > 5

  const handleCollapse = () => {
    setVisibleCount(5)
  }

  return (
    <div ref={containerRef} style={{ marginTop: 8, marginBottom: 8 }}>
      {visible.map((qa, idx) => (
        <div key={idx} style={{ marginBottom: 12, padding: '10px 12px', background: 'rgba(148,163,184,0.06)', borderRadius: 6, borderLeft: '3px solid #3b82f6' }}>
          <div style={{ fontSize: 12, color: 'var(--text-muted)', marginBottom: 6, display: 'flex', justifyContent: 'space-between', flexWrap: 'wrap', gap: 4 }}>
            <span>{qa.questioner || '投资者'}</span>
            <span>{qa.date || ''}{qa.answerDate && qa.answerDate !== qa.date ? ` → ${qa.answerDate}` : ''}</span>
          </div>
          <div style={{ fontSize: 13, color: 'var(--text-primary)', marginBottom: 6, fontWeight: 500, lineHeight: 1.5 }}>Q: {qa.question}</div>
          <div style={{ fontSize: 13, color: 'var(--text-primary)', lineHeight: 1.6 }}>A: {qa.answer}</div>
        </div>
      ))}
      {(hasMore || canCollapse) && (
        <div style={{ marginTop: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8 }}>
          {hasMore && (
            <button
              onClick={() => setVisibleCount(c => c + 5)}
              style={{ padding: '6px 12px', fontSize: 12, color: '#3b82f6', background: 'transparent', border: '1px solid rgba(59,130,246,0.3)', borderRadius: 4, cursor: 'pointer' }}
            >
              查看更多...（还剩 {qas.length - visibleCount} 条）
            </button>
          )}
          {canCollapse && (
            <button
              onClick={handleCollapse}
              style={{ padding: '6px 12px', fontSize: 12, color: '#64748b', background: 'transparent', border: '1px solid rgba(100,116,139,0.3)', borderRadius: 4, cursor: 'pointer', marginLeft: hasMore ? 0 : 'auto', display: 'flex', alignItems: 'center', gap: 6 }}
              title="收起到前5条"
            >
              <span>收起</span>
              <svg width="12" height="12" viewBox="0 0 12 12" fill="currentColor" style={{ opacity: 0.7 }}>
                <path d="M6 2 L10 6 L8.5 6 L6 3.5 L3.5 6 L2 6 Z" />
                <path d="M6 5 L10 9 L8.5 9 L6 6.5 L3.5 9 L2 9 Z" />
              </svg>
            </button>
          )}
        </div>
      )}
    </div>
  )
}
import { toPng } from 'html-to-image'
import {
  GetWatchlist,
  GetWatchlistActivity,
  GetWatchlistFilterData,
  AddToWatchlist,
  RemoveFromWatchlist,
  ReorderWatchlist,
  ImportFinancialReports,
  DownloadReports,
  AnalyzeStock,
  AnalyzeStockWithRIM,
  CheckAnalysisCache,
  DownloadReport,
  ExportReportPDF,
  ExportReportImage,
  DeleteReport,
  ConfirmDialog,
  GetReport,
  GetStockDataHistory,
  GetStockProfile,
  RefreshStockProfile,
  GetComparables,
  AddComparable,
  RemoveComparable,
  DownloadComparableReports,
  FetchMissingActivity,
  GetStockQuote,
  // GetStockKlines,
  GetStockConcepts,
  FetchHotConcepts,
  FetchHotConceptHistory,
  FetchHotConceptConstituents,
  QuickAnalyzeStock,
  GetStockMoneyflow,
  ExportCurrentFinancialData,
  GetRiskRadar,
  GetLatestMargin,
  UpdatePolicyLibrary,
  UpdateIndustryDatabase,
  GetIndustryDBMeta,
  GetIndustryTaskStatus,
  UpdateModule4Only,
  LoadAnalysisSnapshot,
  SendNotification,
  HasPythonDepsChecked,
  GetSFLConfig,
  RecommendComparables,
  RefreshInteractQA,
} from './api'
import type { main, analyzer, downloader } from '../wailsjs/go/models'

type Stock = main.StockInfo
type WatchlistItem = main.WatchlistItem
type ImportResult = main.ImportResult
type QuickAnalysis = main.QuickAnalysis
type AnalysisReport = analyzer.AnalysisReport
type StepResult = analyzer.StepResult
type DownloadResult = main.DownloadResult
type HistoryMeta = main.HistoryMeta
type StockProfile = main.StockProfile
type StockQuote = downloader.StockQuote
type WatchlistFilterItem = main.WatchlistFilterItem
type RiskRadarItem = analyzer.RiskRadarItem
// type KlineData = downloader.KlineData

function getStepValue(steps: StepResult[], stepNum: number, year: string, key: string): number {
  const step = steps.find((s) => s.stepNum === stepNum)
  if (!step || !step.yearlyData || !step.yearlyData[year]) return 0
  return Number(step.yearlyData[year][key] || 0)
}

function extractHighlightsAndRisks(report: AnalysisReport) {
  const latest = report.years[0]
  if (!latest) return { highlights: [], risks: [] }
  const steps = report.stepResults || []

  const roe = getStepValue(steps, 16, latest, 'roe')
  const gm = getStepValue(steps, 10, latest, 'grossMargin')
  const growth = getStepValue(steps, 9, latest, 'growthRate')
  const pg = getStepValue(steps, 16, latest, 'profitGrowth')
  // A-Score 为 0-100 分，越高风险越大，<60 为安全
  const ascore = getStepValue(steps, 8, latest, 'AScore')
  const dr = getStepValue(steps, 3, latest, 'debtRatio')
  const cr = getStepValue(steps, 15, latest, 'cashRatio')

  const highlights: string[] = []
  const risks: string[] = []

  if (roe >= 15) highlights.push('ROE 优秀，资本回报能力强')
  else risks.push('ROE 低于 15%，资本回报能力有待提升')

  if (gm >= 40) highlights.push('高毛利率，定价权稳固')
  else risks.push('毛利率未达 40%，产品竞争力一般')

  if (dr <= 40) highlights.push('低负债率，财务结构稳健')
  else if (dr > 60) risks.push('负债率超过 60%，偿债压力偏大')

  // A-Score 综合风险评分（A股适配）
  if (ascore < 40) highlights.push('A-Score 安全，财务质量良好')
  else if (ascore < 60) highlights.push('A-Score 低风险，财务质量可控')
  else if (ascore < 70) risks.push('A-Score 中风险，需关注财务健康度')
  else risks.push('A-Score 高风险，建议谨慎')

  if (growth >= 10) highlights.push('营收稳健增长')
  else if (growth < 0) risks.push('营收负增长，成长性承压')

  if (pg >= 10) highlights.push('净利润持续增长')
  else if (pg < 0) risks.push('净利润下滑，盈利能力减弱')

  if (cr >= 100) highlights.push('经营现金流充沛，盈利质量高')
  else if (cr > 0) risks.push('现金流含金量不足')

  return { highlights, risks }
}

// 客户端本地时间是否处于 A 股交易时段（与 IntradayChart 的同名函数对齐）
function isLocalTradingHours(): boolean {
  const now = new Date()
  const day = now.getDay()
  if (day === 0 || day === 6) return false
  const mins = now.getHours() * 60 + now.getMinutes()
  return (mins >= 9 * 60 + 30 && mins < 11 * 60 + 30) || (mins >= 13 * 60 && mins < 15 * 60)
}

function App() {
  const [watchlist, setWatchlist] = useState<WatchlistItem[]>([])
  const [selectedCode, setSelectedCode] = useState<string | null>(null)
  const selectedCodeRef = useRef<string | null>(null)
  useEffect(() => {
    selectedCodeRef.current = selectedCode
  }, [selectedCode])
  const [query, setQuery] = useState('')
  const [showDropdown, setShowDropdown] = useState(false)
  const [loading, setLoading] = useState(false)
  const [settings, setSettings] = useState<AppSettings>(() => loadSettings())
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [downloadResult, setDownloadResult] = useState<DownloadResult | null>(null)
  const [downloading, setDownloading] = useState(false)
  const [downloadStatus, setDownloadStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [downloadSuggestion, setDownloadSuggestion] = useState<string>('')
  const [report, setReport] = useState<AnalysisReport | null>(null)
  const [snapshots, setSnapshots] = useState<Record<string, AnalysisReport>>({})

  // AI 投研状态
  const [activeReportTab, setActiveReportTab] = useState<'report' | 'ai'>('report')
  const [aiReport, setAiReport] = useState<ai_researcher.AIResearchReport | null>(null)
  const [aiReportLoading, setAiReportLoading] = useState(false)
  const [aiReportError, setAiReportError] = useState<string | null>(null)
  const [aiAnalyzingCode, setAiAnalyzingCode] = useState<string | null>(null)
  const [aiProgress, setAiProgress] = useState<{ stage: string; message: string } | null>(null)

  // 监听 AI 投研进度事件
  useEffect(() => {
    const cleanup = EventsOn('ai:progress', (data: AIProgressEvent) => {
      if (data.symbol === selectedCodeRef.current) {
        setAiProgress({ stage: data.stage, message: data.message })
      }
    })
    return () => {
      cleanup()
    }
  }, [])

  // 切换股票时自动加载该股票的 AI 投研缓存；若该股票正在后台分析，则保持加载状态
  useEffect(() => {
    setAiReport(null)
    setAiReportError(null)
    setAiProgress(null)
    setAiReportLoading(aiAnalyzingCode === selectedCode)
    if (!selectedCode) return
    LoadAIResearchReport(selectedCode)
      .then((report) => {
        if (report) {
          setAiReport(report)
          setAiReportLoading(false)
        }
      })
      .catch((err: any) => {
        console.error('加载 AI 投研缓存失败:', err)
      })
  }, [selectedCode])

  const [analyzing, setAnalyzing] = useState(false)
  const [analyzeProgress, setAnalyzeProgress] = useState(0)
  const [viewingHistory, setViewingHistory] = useState<string | null>(null)
  const [historyContent, setHistoryContent] = useState<string>('')
  const [dataHistory, setDataHistory] = useState<HistoryMeta[]>([])
  const [dataMissing, setDataMissing] = useState(false)
  const [profile, setProfile] = useState<StockProfile | null>(null)
  const [comparables, setComparables] = useState<string[]>([])
  const [appliedComparables, setAppliedComparables] = useState<string[]>([])
  const [compQuery, setCompQuery] = useState('')
  const [showCompDropdown, setShowCompDropdown] = useState(false)
  const [compRecommendations, setCompRecommendations] = useState<analyzer.ComparableRecommendation[]>([])
  const [compRecommending, setCompRecommending] = useState(false)
  const [compDownloading, setCompDownloading] = useState(false)
  const [refreshingInteractQA, setRefreshingInteractQA] = useState(false)
  const [interactQAVisibleCount, setInteractQAVisibleCount] = useState(5)
  const [compReportsDownloaded, setCompReportsDownloaded] = useState(false)
  const [compDownloadStatus, setCompDownloadStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [fetchingActivity, setFetchingActivity] = useState(false)
  const [fetchActivityStatus, setFetchActivityStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})

  const [concepts, setConcepts] = useState<downloader.StockConcepts | null>(null)
  const [moneyflow, setMoneyflow] = useState<main.StockMoneyflowResult | null>(null)
  const [todayMoneyflowExpanded, setTodayMoneyflowExpanded] = useState(true)
  const [recentMoneyflowExpanded, setRecentMoneyflowExpanded] = useState(true)

  const [moneyflowRefreshing, setMoneyflowRefreshing] = useState(false)
  const [sflConfig, setSflConfig] = useState<main.SFLConfig | null>(null)

  // 加载 SFL 配置
  useEffect(() => {
    GetSFLConfig().then((cfg) => {
      setSflConfig(cfg)
    }).catch(() => {
      setSflConfig(null)
    })
  }, [])

  // 监听自动更新事件
  useEffect(() => {
    const handler = (info: UpdateInfo) => {
      setUpdateInfo(info)
      setShowUpdateModal(true)
    }
    EventsOn('update:available', handler)
    return () => {
      // Wails runtime 没有 EventsOff 针对单个 handler 的便捷方式
      // 但 EventsOn 返回的 cleanup 在组件卸载时会自动清理
    }
  }, [])

  // 市场热点/风口
  const [hotConcepts, setHotConcepts] = useState<downloader.HotConcept[]>([])
  const [hotConceptDate, setHotConceptDate] = useState<string>('')
  const [hotConceptLoading, setHotConceptLoading] = useState(false)
  const [hotConceptHistory, setHotConceptHistory] = useState<Record<string, string[]>>({})
  const [conceptConstituents, setConceptConstituents] = useState<Record<string, downloader.ConceptConstituent[]>>({})
  const [hotConceptError, setHotConceptError] = useState<string>('')
  const [hotPanelOpen, setHotPanelOpen] = useState(false)
  const [selectedHotConceptCode, setSelectedHotConceptCode] = useState<string | null>(null)

  // 快速分析
  const [quickAnalysisCode, setQuickAnalysisCode] = useState<string | null>(null)
  const [quickAnalysisData, setQuickAnalysisData] = useState<QuickAnalysis | null>(null)
  const [quickAnalysisLoading, setQuickAnalysisLoading] = useState(false)
  // 快速分析结果缓存: conceptCode -> stockCode -> QuickAnalysis
  const [quickAnalysisCache, setQuickAnalysisCache] = useState<Record<string, Record<string, QuickAnalysis>>>({})
  // 缓存日期标记，用于每日首次清理
  const [quickAnalysisCacheDate, setQuickAnalysisCacheDate] = useState<string>('')
  // 成分股主力净流入加总: conceptCode -> sum(main_inflow)，用于替代板块指数f62
  const [conceptMainInflowSum, setConceptMainInflowSum] = useState<Record<string, number>>({})

  const [policyLibMeta, setPolicyLibMeta] = useState<{version: string, updatedAt: string} | null>(null)
  const [policyUpdating, setPolicyUpdating] = useState(false)
  const [industryDBMeta, setIndustryDBMeta] = useState<{version: string, updatedAt: string, count: number} | null>(null)
  const [industryUpdating, setIndustryUpdating] = useState(false)
  const [industryTask, setIndustryTask] = useState<any>(null)
  const [policyActionStatus, setPolicyActionStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [industryActionStatus, setIndustryActionStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [quote, setQuote] = useState<StockQuote | null>(null)
  const [quoteError, setQuoteError] = useState<string>('')
  // K线数据由 UnifiedChart 组件内部管理
  // const [klines, setKlines] = useState<KlineData[]>([])
  // const [klineError, setKlineError] = useState<string>('')
  const [activityMap, setActivityMap] = useState<Record<string, main.WatchlistActivitySummary>>({})
  const [activitySort, setActivitySort] = useState<'none' | 'desc' | 'asc'>('none')
  const [flashCode, setFlashCode] = useState<string | null>(null)
  const [filterData, setFilterData] = useState<Record<string, WatchlistFilterItem>>({})

  const [watchlistFilter, setWatchlistFilter] = useState<
    'none' | 'highReturn' | 'lowRisk' | 'hasData' | 'noData' | 'analyzed' | 'unanalyzed'
  >('none')
  const [watchlistIndustryFilter, setWatchlistIndustryFilter] = useState<string>('全部')
  const flashTimeoutRef = useRef<number | null>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const reportContentRef = useRef<HTMLDivElement>(null)
  const dragIndexRef = useRef<number | null>(null)
  const watchlistRef = useRef<HTMLUListElement>(null)
  const dragOverIndexRef = useRef<number | null>(null)
  const originalOrderRef = useRef<WatchlistItem[]>([])
  const [draggingIndex, setDraggingIndex] = useState<number | null>(null)
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null)
  const ghostElRef = useRef<HTMLDivElement | null>(null)
  const reportSearchRef = useRef<HTMLInputElement>(null)
  const reportMatchesRef = useRef<HTMLElement[]>([])
  const reportSearchIndexRef = useRef(0)
  const reportLastQueryRef = useRef('')
  const downloadMenuRef = useRef<HTMLDivElement>(null)
  const downloadMenuBtnRef = useRef<HTMLButtonElement>(null)
  const tocSelectRef = useRef<HTMLSelectElement>(null)
  const [traceDrawerOpen, setTraceDrawerOpen] = useState(false)
  const [currentTrace, setCurrentTrace] = useState<analyzer.CalcTrace | null>(null)
  const [traceList, setTraceList] = useState<analyzer.CalcTrace[]>([])
  const [forceAnalyzeOpen, setForceAnalyzeOpen] = useState(false)
  const [lastAnalysisAt, setLastAnalysisAt] = useState('')
  const [trendDrawerCode, setTrendDrawerCode] = useState<string | null>(null)
  const [marginDrawerCode, setMarginDrawerCode] = useState<string | null>(null)
  const [marginTargetMap, setMarginTargetMap] = useState<Record<string, boolean>>({})
  const [klineFullscreen, setKlineFullscreen] = useState(false)
  const [riskRadar, setRiskRadar] = useState<RiskRadarItem[] | null>(null)
  const [downloadMenuOpen, setDownloadMenuOpen] = useState(false)
  const aiExportMenuRef = useRef<HTMLDivElement>(null)
  const aiExportMenuBtnRef = useRef<HTMLButtonElement>(null)
  const [aiExportMenuOpen, setAiExportMenuOpen] = useState(false)
  const aiReportContentRef = useRef<HTMLDivElement>(null)
  // Python 依赖检测弹窗
  const [showPythonDepsModal, setShowPythonDepsModal] = useState(false)
  // 自动更新弹窗
  const [updateInfo, setUpdateInfo] = useState<UpdateInfo | null>(null)
  const [showUpdateModal, setShowUpdateModal] = useState(false)

  // RIM 参数弹窗状态
  const [showRIMModal, setShowRIMModal] = useState(false)
  const [rimBeta, setRimBeta] = useState(0.98)
  const [rimRf, setRimRf] = useState(1.83)
  const [rimRfDate, setRimRfDate] = useState('')
  const [rimRmRf, setRimRmRf] = useState(5.17)
  const [rimRmRfDate, setRimRmRfDate] = useState('')
  const [rimBetaDate, setRimBetaDate] = useState('')
  const [rimG, setRimG] = useState(5.0)
  const [rimEPS, setRimEPS] = useState<(number | string)[]>(['0', '0', '0', '0', '0', '0'])
  const [rimBPS0, setRimBPS0] = useState(0)
  const [rimPrice, setRimPrice] = useState(0)
  const [rimLoading, setRimLoading] = useState(false)
  const [rimProgress, setRimProgress] = useState(0)

  const tocSections = [
    { label: '模块1: 执行摘要', id: '模块1-执行摘要' },
    { label: '模块2: 换手率深度分析', id: '模块2-换手率深度分析' },
    { label: '模块3: 公司基本面分析', id: '模块3-公司基本面分析' },
    { label: '模块4: 行业横向对比分析', id: '模块4-行业横向对比分析' },
    { label: '模块5: 十五五政策匹配度评估', id: '模块5-十五五政策匹配度评估' },
    { label: '模块6: 剩余收益模型估值(RIM)', id: '模块6-剩余收益模型估值rim' },
    { label: '模块7: A-Score 综合风险画像', id: '模块7-a-score-综合风险画像' },
    { label: '模块8: 技术面分析', id: '模块8-技术面分析' },
    { label: '模块9: ML机器学习预测', id: '模块9-ml机器学习预测' },
    { label: '模块10: 智能选股7大条件', id: '模块10-智能选股7大条件' },
    { label: '模块11: 逆向思维检查', id: '模块11-逆向思维检查' },
    { label: '模块12: 投资检查清单', id: '模块12-投资检查清单' },
    { label: '模块13: 社交媒体情绪监控', id: '模块13-社交媒体情绪监控' },
    { label: '模块14: 综合投资建议', id: '模块14-综合投资建议' },
    { label: '模块15: 结论与附录', id: '模块15-结论与附录' },
  ]

  const handleTocJump = (id: string) => {
    if (!reportContentRef.current || !id) return
    const el = reportContentRef.current.querySelector(`#${CSS.escape(id)}`) as HTMLElement | null
    if (el) {
      const container = reportContentRef.current
      const top = el.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop - 12
      container.scrollTo({ top, behavior: 'smooth' })
    }
  }

  const clearSearchHighlights = () => {
    if (!reportContentRef.current) return
    const container = reportContentRef.current.querySelector('.markdown-body')
    if (!container) return
    const highlights = container.querySelectorAll('span.search-highlight, span.search-highlight-active')
    highlights.forEach((span) => {
      const parent = span.parentNode
      if (parent) {
        parent.replaceChild(document.createTextNode(span.textContent || ''), span)
        parent.normalize()
      }
    })
    reportMatchesRef.current = []
    reportSearchIndexRef.current = 0
  }

  const buildSearchHighlights = (query: string): number => {
    if (!reportContentRef.current) return 0
    const container = reportContentRef.current.querySelector('.markdown-body')
    if (!container) return 0
    clearSearchHighlights()
    const walker = document.createTreeWalker(container, NodeFilter.SHOW_TEXT)
    const matches: { node: Text; start: number; end: number }[] = []
    const lowerQuery = query.toLowerCase()
    let node: Node | null
    while ((node = walker.nextNode())) {
      const textNode = node as Text
      const text = textNode.textContent || ''
      const lowerText = text.toLowerCase()
      let idx = 0
      while ((idx = lowerText.indexOf(lowerQuery, idx)) !== -1) {
        matches.push({ node: textNode, start: idx, end: idx + query.length })
        idx += query.length
      }
    }
    // Process from end to start so node positions don't shift
    for (let i = matches.length - 1; i >= 0; i--) {
      const { node, start, end } = matches[i]
      const range = document.createRange()
      range.setStart(node, start)
      range.setEnd(node, end)
      const span = document.createElement('span')
      span.className = 'search-highlight'
      try {
        range.surroundContents(span)
        reportMatchesRef.current.unshift(span)
      } catch {
        // ignore ranges that span multiple elements
      }
    }
    return reportMatchesRef.current.length
  }

  const handleReportSearchKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter') return
    const query = e.currentTarget.value.trim()
    if (!query) {
      clearSearchHighlights()
      return
    }
    if (!displayContent) {
      alert('没有可搜索的内容')
      return
    }
    // If query changed, rebuild highlights
    if (reportLastQueryRef.current !== query) {
      reportLastQueryRef.current = query
      reportSearchIndexRef.current = 0
      const count = buildSearchHighlights(query)
      if (count === 0) {
        alert('没有匹配')
        return
      }
    }
    const matches = reportMatchesRef.current
    if (matches.length === 0) {
      const count = buildSearchHighlights(query)
      if (count === 0) {
        alert('没有匹配')
        return
      }
    }
    // Remove active class from previous
    matches.forEach((m) => (m.className = 'search-highlight'))
    // Scroll to current match
    const currentIdx = reportSearchIndexRef.current
    const current = matches[currentIdx]
    current.className = 'search-highlight search-highlight-active'
    current.scrollIntoView({ behavior: 'smooth', block: 'center' })
    if (currentIdx === matches.length - 1) {
      alert('已经达到最后一个匹配的字串')
      reportSearchIndexRef.current = 0
    } else {
      reportSearchIndexRef.current++
    }
  }

  // 左栏宽度可拖动调整
  const [sidebarWidth, setSidebarWidth] = useState(230)
  const [isResizing, setIsResizing] = useState(false)

  useEffect(() => {
    if (!isResizing) return
    const handleMouseMove = (e: MouseEvent) => {
      const newWidth = Math.min(Math.max(e.clientX, 200), 400)
      setSidebarWidth(newWidth)
    }
    const handleMouseUp = () => setIsResizing(false)
    document.addEventListener('mousemove', handleMouseMove)
    document.addEventListener('mouseup', handleMouseUp)
    return () => {
      document.removeEventListener('mousemove', handleMouseMove)
      document.removeEventListener('mouseup', handleMouseUp)
    }
  }, [isResizing])

  // 拼音首字母缓存
  const pinyinMap = useMemo(() => {
    const map = new Map<string, string>()
    STOCKS.forEach((s) => {
      try {
        const py = pinyin(s.name, { pattern: 'first', toneType: 'none', type: 'string' }).toLowerCase().replace(/\s+/g, '')
        map.set(s.code, py)
      } catch {
        map.set(s.code, '')
      }
    })
    return map
  }, [])

  // 初始化加载自选列表及活跃度
  useEffect(() => {
    GetWatchlist().then((list) => {
      setWatchlist(list || [])
    })
    GetWatchlistActivity().then((list) => {
      const map: Record<string, main.WatchlistActivitySummary> = {}
      ;(list || []).forEach((item) => {
        map[item.code] = item
      })
      setActivityMap(map)
    })
    GetWatchlistFilterData().then((list) => {
      const map: Record<string, WatchlistFilterItem> = {}
      ;(list || []).forEach((item) => {
        map[item.code] = item
      })
      setFilterData(map)
    }).catch((err) => {
      console.error('[GetWatchlistFilterData] error', err)
    })
    // 加载政策库元信息
    loadPolicyLibMeta()
    // 加载行业数据库元信息，并根据设置决定是否自动更新
    const autoUpdateIndustry = async () => {
      const meta = await GetIndustryDBMeta()
      const formatted = {
        version: meta.version || '1.0',
        updatedAt: meta.updatedAt || '未更新',
        count: meta.count || 0,
      }
      setIndustryDBMeta(formatted)
      if (!settings.autoUpdateIndustryDB) return
      if (formatted.updatedAt === '未更新') {
        handleUpdateIndustryDB()
        return
      }
      try {
        const last = new Date(formatted.updatedAt.replace(/-/g, '/'))
        const days = (Date.now() - last.getTime()) / (1000 * 60 * 60 * 24)
        if (days >= 7) {
          handleUpdateIndustryDB()
        }
      } catch {
        // ignore
      }
    }
    autoUpdateIndustry()
    // 首次启动检测 Python 依赖
    HasPythonDepsChecked().then(checked => {
      if (!checked) {
        setShowPythonDepsModal(true)
      }
    })
  }, [])

  // 自选股变化时刷新活跃度
  useEffect(() => {
    if (watchlist.length === 0) return
    GetWatchlistActivity().then((list) => {
      console.log('[GetWatchlistActivity] returned', list)
      const map: Record<string, main.WatchlistActivitySummary> = {}
      ;(list || []).forEach((item) => {
        map[item.code] = item
      })
      setActivityMap(map)
    }).catch((err) => {
      console.error('[GetWatchlistActivity] error', err)
    })
  }, [watchlist.length])

  // 切换热点概念时：清理当前分析结果，若有缓存则恢复第一个
  useEffect(() => {
    if (!selectedHotConceptCode) {
      setQuickAnalysisCode(null)
      setQuickAnalysisData(null)
      return
    }
    const cached = quickAnalysisCache[selectedHotConceptCode]
    if (cached && Object.keys(cached).length > 0) {
      const firstCode = Object.keys(cached)[0]
      const firstData = cached[firstCode]
      setQuickAnalysisCode(firstCode)
      setQuickAnalysisData(firstData)
    } else {
      setQuickAnalysisCode(null)
      setQuickAnalysisData(null)
    }
  }, [selectedHotConceptCode, quickAnalysisCache])

  // 搜索输入时，若已加自选中有匹配，则高亮并滚动
  useEffect(() => {
    const q = query.trim()
    if (!q) {
      setFlashCode(null)
      return
    }
    const lower = q.toLowerCase()
    const matched = watchlist.find(
      (s) => s.code.toLowerCase().includes(lower) || s.name.toLowerCase().includes(lower)
    )
    if (matched) {
      setFlashCode(matched.code)
      if (flashTimeoutRef.current) {
        window.clearTimeout(flashTimeoutRef.current)
      }
      flashTimeoutRef.current = window.setTimeout(() => {
        setFlashCode(null)
      }, 1500)
      // 等待 DOM 更新后滚动
      requestAnimationFrame(() => {
        const el = document.querySelector(`.watchlist li[data-code="${matched.code}"]`)
        if (el) {
          el.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
        }
      })
    } else {
      setFlashCode(null)
    }
  }, [query, watchlist])

  // 主题持久化
  useEffect(() => {
    // 应用主题
    const effectiveTheme = settings.theme === 'system' 
      ? (window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark')
      : settings.theme
    
    if (effectiveTheme === 'light') {
      document.body.classList.add('light')
    } else {
      document.body.classList.remove('light')
    }
  }, [settings.theme])



  // 通过 Wails runtime 轮询窗口尺寸，检测到变化后强制触发 report-tabs-right 的浏览器重绘
  useEffect(() => {
    let prevWidth = 0
    let prevHeight = 0
    const tick = async () => {
      try {
        const size = await WindowGetSize()
        if ((size.w !== prevWidth || size.h !== prevHeight) && prevWidth !== 0) {
          const el = document.querySelector('.report-tabs-right') as HTMLElement | null
          if (el) {
            // 通过读取 offsetHeight 强制浏览器重排，触发 WebKit 重新绘制被隐藏的 flex 子项
            void el.offsetHeight
            el.style.transform = 'translateZ(0)'
            requestAnimationFrame(() => {
              el.style.transform = ''
            })
          }
        }
        prevWidth = size.w
        prevHeight = size.h
      } catch {
        // ignore
      }
    }
    const interval = window.setInterval(tick, 300)
    // 立即执行一次初始化
    tick()
    return () => clearInterval(interval)
  }, [])

  // 市场热点开关关闭时自动收起热点面板
  useEffect(() => {
    if (!settings.enableHotConcepts && hotPanelOpen) {
      setHotPanelOpen(false)
      setSelectedHotConceptCode(null)
    }
  }, [settings.enableHotConcepts])

  // 本地搜索过滤：按代码、名称或拼音首字母匹配，最多10条
  const suggestions = useMemo(() => {
    const q = query.trim()
    if (!q) return []
    const lower = q.toLowerCase()
    return STOCKS.filter(
      (s) =>
        s.code.toLowerCase().includes(lower) ||
        s.name.toLowerCase().includes(lower) ||
        (pinyinMap.get(s.code) || '').includes(lower)
    ).slice(0, 10)
  }, [query, pinyinMap])

  const selectedStock = useMemo(
    () => watchlist.find((s) => s.code === selectedCode) || null,
    [selectedCode, watchlist]
  )

  const displayWatchlist = useMemo(() => {
    let list = [...watchlist]

    // 应用筛选条件
    if (watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部') {
      list = list.filter((s) => {
        const fd = filterData[s.code]
        if (!fd) return false

        if (watchlistIndustryFilter !== '全部' && fd.industry !== watchlistIndustryFilter) {
          return false
        }

        switch (watchlistFilter) {
          case 'highReturn':
            return fd.shareholderReturnRate > 0.10
          case 'lowRisk':
            return fd.aScore > 0 && fd.aScore < 60 && fd.riskLevel === 'low'
          case 'hasData':
            return fd.hasFinancialData
          case 'noData':
            return !fd.hasFinancialData
          case 'analyzed':
            return fd.hasSnapshot
          case 'unanalyzed':
            return !fd.hasSnapshot
          default:
            return true
        }
      })
    }

    if (activitySort === 'none') return list
    list.sort((a, b) => {
      const scoreA = activityMap[a.code]?.score ?? -1
      const scoreB = activityMap[b.code]?.score ?? -1
      if (activitySort === 'desc') return scoreB - scoreA
      return scoreA - scoreB
    })
    return list
  }, [watchlist, activityMap, activitySort, filterData, watchlistFilter, watchlistIndustryFilter])

  // 通过 data-status 属性控制 activity-hint 状态显示（避免直接操作 DOM）
  useEffect(() => {
    if (!reportContentRef.current) return
    const trigger = reportContentRef.current.querySelector('.fetch-activity-trigger')
    if (!trigger) return
    if (fetchingActivity) {
      trigger.setAttribute('data-status', 'loading')
    } else if (fetchActivityStatus.type === 'success') {
      trigger.setAttribute('data-status', 'success')
    } else if (fetchActivityStatus.type === 'error') {
      trigger.setAttribute('data-status', 'error')
    } else {
      trigger.removeAttribute('data-status')
    }
  }, [fetchingActivity, fetchActivityStatus])

  // 当切换股票时，若内存中没有快照，尝试从磁盘加载
  useEffect(() => {
    if (!selectedStock) return
    if (snapshots[selectedStock.code]) return
    LoadAnalysisSnapshot(selectedStock.code)
      .then((snapshot) => {
        if (snapshot) {
          setSnapshots((prev) => ({ ...prev, [selectedStock.code]: snapshot }))
        }
      })
      .catch(() => {
        // 忽略加载失败的错误
      })
  }, [selectedStock, snapshots])

  // 切换股票时检查是否为融资融券标的
  useEffect(() => {
    if (!selectedStock) return
    if (marginTargetMap[selectedStock.code] !== undefined) return
    GetLatestMargin(selectedStock.code)
      .then(() => {
        setMarginTargetMap((prev) => ({ ...prev, [selectedStock.code]: true }))
      })
      .catch(() => {
        setMarginTargetMap((prev) => ({ ...prev, [selectedStock.code]: false }))
      })
  }, [selectedStock, marginTargetMap])

  // 自选股列表加载完成后，批量加载所有股票的快照（用于左栏警示灯持久显示）
  useEffect(() => {
    if (watchlist.length === 0) return
    watchlist.forEach((s) => {
      if (snapshots[s.code]) return
      LoadAnalysisSnapshot(s.code)
        .then((snapshot) => {
          if (snapshot) {
            setSnapshots((prev) => ({ ...prev, [s.code]: snapshot }))
          }
        })
        .catch(() => {
          // 静默忽略，该股票可能从未分析过
        })
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [watchlist])

  const currentSnapshot = selectedStock ? snapshots[selectedStock.code] : null
  const { highlights, risks } = useMemo(() => {
    if (!currentSnapshot) return { highlights: [], risks: [] }
    // 优先使用后端统一生成的亮点/风险，fallback 到前端本地计算
    if (currentSnapshot.highlights && currentSnapshot.highlights.length > 0) {
      return { highlights: currentSnapshot.highlights, risks: currentSnapshot.risks || [] }
    }
    return extractHighlightsAndRisks(currentSnapshot)
  }, [currentSnapshot])

  const loadReportHistory = useCallback(async (code: string, autoLoadLatest = false) => {
    try {
      // 获取分析缓存时间
      const cache = await CheckAnalysisCache(code)
      setLastAnalysisAt(cache?.lastAnalysisAt || '')
      setDataMissing(!!cache?.dataMissing)
      if (autoLoadLatest) {
        const content = await GetReport(code, 'latest.md')
        if (content) {
          setHistoryContent(content)
          setViewingHistory('latest.md')
        } else {
          setHistoryContent('')
          setViewingHistory(null)
        }
      }
    } catch {
      setLastAnalysisAt('')
    }
  }, [])

  const loadDataHistory = useCallback(async (code: string) => {
    try {
      console.log('[loadDataHistory] Loading for:', code)
      const list = await GetStockDataHistory(code)
      console.log('[loadDataHistory] Result:', list)
      setDataHistory(list || [])
    } catch (err: any) {
      console.error('[loadDataHistory] Error:', err)
      setDataHistory([])
    }
  }, [])

  const loadProfile = useCallback(async (code: string) => {
    try {
      const p = await GetStockProfile(code)
      setProfile(p || null)
      return p || null
    } catch {
      setProfile(null)
      return null
    }
  }, [])

  const handleRefreshProfile = async () => {
    if (!selectedStock) return
    try {
      const p = await RefreshStockProfile(selectedStock.code)
      setProfile(p || null)
    } catch (err: any) {
      alert('刷新基本信息失败: ' + String(err))
    }
  }

  // 注：必须 useCallback 稳定引用，否则会让下面的 markdownComponents useMemo 每次 App 重渲染都失效，
  // 进而让 ReactMarkdown 的 components prop 每次都换新对象，触发 markdown 内嵌图表(FinancialTrendChart/UnifiedChart)被反复卸载-重挂。
  const handleRefreshInteractQA = useCallback(async () => {
    if (!selectedStock || refreshingInteractQA) return
    setRefreshingInteractQA(true)
    try {
      const qas = await RefreshInteractQA(selectedStock.code)
      if (qas && qas.length > 0) {
        setSnapshots((prev) => {
          const snap = prev[selectedStock.code]
          if (!snap) return prev
          return {
            ...prev,
            [selectedStock.code]: {
              ...(snap as any),
              interactQAs: qas,
            } as any,
          }
        })
        // 同时更新当前报告中的 markdown，替换 13.4 的更新时间
        setReport((prev) => {
          if (!prev) return prev
          const updated = prev.markdownContent.replace(
            /> 已获取 \*\*\d+\*\* 条互动平台问答（来源：[^）]+），最近更新时间：\d{4}-\d{2}-\d{2}/,
            `> 已获取 **${qas.length}** 条互动平台问答（来源：${qas[0]?.source === 'sse' ? '上证e互动' : qas[0]?.source === 'guba' ? '东方财富股吧' : '巨潮互动易'}），最近更新时间：${qas[0]?.answerDate || qas[0]?.date || ''}`
          )
          return { ...(prev as any), markdownContent: updated } as any
        })
        // 刷新后重置显示数量为5条
        setInteractQAVisibleCount(5)
        SendNotification('success', `已刷新互动问答，获取 ${qas.length} 条最新数据`)
      } else {
        SendNotification('info', '暂无新的互动问答数据')
      }
    } catch (err: any) {
      const errMsg = String(err)
      SendNotification('error', '刷新互动问答失败: ' + errMsg)
    } finally {
      setRefreshingInteractQA(false)
    }
  }, [selectedStock, refreshingInteractQA])

  const loadConcepts = useCallback(async (code: string) => {
    try {
      const c = await GetStockConcepts(code)
      setConcepts(c || null)
    } catch {
      setConcepts(null)
    }
  }, [])

  const loadMoneyflow = useCallback(async (code: string) => {
    try {
      const cfg = await GetSFLConfig()
      const days = cfg?.moneyflow_days || 3
      const mf = await GetStockMoneyflow(code, days)
      setMoneyflow(mf || null)
    } catch {
      setMoneyflow(null)
    }
  }, [])

  const refreshMoneyflow = useCallback(async () => {
    if (!selectedCode || moneyflowRefreshing) return
    setMoneyflowRefreshing(true)
    try {
      await loadMoneyflow(selectedCode)
    } finally {
      setMoneyflowRefreshing(false)
    }
  }, [selectedCode, moneyflowRefreshing, loadMoneyflow])

  // 当日流向自动刷新：仅当展开 + 已选股票 + 交易时段 + 页面可见时,每 60s 拉一次。
  // 展开瞬间立刻拉一次,展开 → 收起则停掉 interval。
  // 用 ref 解耦 refreshMoneyflow 引用,避免 moneyflowRefreshing 频繁变化重启 interval。
  const refreshMoneyflowRef = useRef(refreshMoneyflow)
  useEffect(() => { refreshMoneyflowRef.current = refreshMoneyflow }, [refreshMoneyflow])
  useEffect(() => {
    if (!todayMoneyflowExpanded || !selectedCode) return
    if (selectedCode.endsWith('.HK')) return // 港股暂无真实资金流向数据，跳过自动刷新
    const tryFetch = () => {
      if (document.visibilityState !== 'visible') return
      if (!isLocalTradingHours()) return
      refreshMoneyflowRef.current()
    }
    tryFetch() // 展开瞬间立刻拉一次（非交易时段则跳过，避免无效请求）
    const handle = window.setInterval(tryFetch, 60_000)
    const onVis = () => {
      if (document.visibilityState === 'visible') tryFetch()
    }
    document.addEventListener('visibilitychange', onVis)
    return () => {
      window.clearInterval(handle)
      document.removeEventListener('visibilitychange', onVis)
    }
  }, [todayMoneyflowExpanded, selectedCode])

  // 加载政策库元信息（从 localStorage 或默认值）
  const loadPolicyLibMeta = useCallback(() => {
    try {
      const saved = localStorage.getItem('policy_library_meta')
      if (saved) {
        setPolicyLibMeta(JSON.parse(saved))
      } else {
        setPolicyLibMeta({ version: 'builtin', updatedAt: '内置默认' })
      }
    } catch {
      setPolicyLibMeta({ version: 'builtin', updatedAt: '内置默认' })
    }
  }, [])

  // 更新政策库
  const handleUpdatePolicyLibrary = useCallback(async () => {
    setPolicyUpdating(true)
    setPolicyActionStatus({type: null, message: ''})
    try {
      const result = await UpdatePolicyLibrary()
      if (result) {
        const meta = { version: result.path ? 'external' : 'builtin', updatedAt: new Date().toLocaleString('zh-CN') }
        setPolicyLibMeta(meta)
        localStorage.setItem('policy_library_meta', JSON.stringify(meta))
        const msg = `政策库更新成功：新增行业关键词 ${result.added_industry_keywords} 个，概念关键词 ${result.added_concept_keywords} 个，行业 ${result.total_industries} 个，概念 ${result.total_concepts} 个`
        setPolicyActionStatus({type: 'success', message: msg})
        setTimeout(() => setPolicyActionStatus({type: null, message: ''}), 3000)
      }
    } catch (err: any) {
      const msg = '政策库更新失败: ' + (err?.message || String(err))
      setPolicyActionStatus({type: 'error', message: msg.length > 100 ? msg.slice(0, 100) + '...' : msg})
      setTimeout(() => setPolicyActionStatus({type: null, message: ''}), 5000)
    } finally {
      setPolicyUpdating(false)
    }
  }, [])

  // 加载行业数据库元信息
  const loadIndustryDBMeta = useCallback(async () => {
    try {
      const meta = await GetIndustryDBMeta()
      setIndustryDBMeta({
        version: meta.version || '1.0',
        updatedAt: meta.updatedAt || '未更新',
        count: meta.count || 0
      })
    } catch {
      setIndustryDBMeta({ version: '1.0', updatedAt: '未更新', count: 0 })
    }
  }, [])

  // 轮询后台行业数据采集任务状态
  useEffect(() => {
    let prevStatus = ''
    const check = async () => {
      try {
        const task = await GetIndustryTaskStatus()
        setIndustryTask(task)
        const status = task?.status || 'idle'
        if (status === 'running') {
          setIndustryUpdating(true)
        } else {
          setIndustryUpdating(false)
        }
        // 如果刚从 running 变为 completed，刷新元信息
        if (prevStatus === 'running' && status === 'completed') {
          loadIndustryDBMeta()
        }
        prevStatus = status
      } catch {
        // ignore
      }
    }
    check()
    const id = setInterval(check, 3000)
    return () => clearInterval(id)
  }, [loadIndustryDBMeta])

  // 更新行业数据库
  const handleUpdateIndustryDB = useCallback(async () => {
    setIndustryUpdating(true)
    setIndustryActionStatus({type: null, message: ''})
    try {
      const result = await UpdateIndustryDatabase()
      if (result) {
        await loadIndustryDBMeta()
        const msg = `行业数据库更新成功：更新行业 ${result.updated_count} 个，跳过 ${result.skipped_count} 个，行业总数 ${result.total_industries} 个`
        setIndustryActionStatus({type: 'success', message: msg})
        setTimeout(() => setIndustryActionStatus({type: null, message: ''}), 5000)
      }
    } catch (err: any) {
      console.error('更新行业数据库失败:', err)
      const msg = '行业数据库更新失败: ' + (err?.message || String(err))
      setIndustryActionStatus({type: 'error', message: msg.length > 100 ? msg.slice(0, 100) + '...' : msg})
      setTimeout(() => setIndustryActionStatus({type: null, message: ''}), 5000)
    } finally {
      setIndustryUpdating(false)
    }
  }, [loadIndustryDBMeta])

  // ========== 市场热点/风口 ==========
  const loadHotConcepts = useCallback(async () => {
    setHotConceptLoading(true)
    setHotConceptError('')
    // 每日首次进入热点时清理过期缓存
    const today = new Date().toISOString().split('T')[0]
    if (quickAnalysisCacheDate !== today) {
      console.log('[HotConcept] new day, clearing quick analysis cache', quickAnalysisCacheDate, '->', today)
      setQuickAnalysisCache({})
      setQuickAnalysisCacheDate(today)
    }
    try {
      const board = await FetchHotConcepts(20)
      console.log('[HotConcepts] loaded', board?.concepts?.length, 'concepts')
      if (board && board.concepts) {
        setHotConcepts(board.concepts)
        setHotConceptDate(board.date)
        // 加载历史用于连续上榜标记
        const history = await FetchHotConceptHistory(7)
        const historyMap: Record<string, string[]> = {}
        history.forEach((h: any) => {
          historyMap[h.date] = h.top_names || []
        })
        setHotConceptHistory(historyMap)
      }
    } catch (err: any) {
      console.error('加载热门概念失败:', err)
      setHotConceptError('获取热门概念失败，请检查网络')
    } finally {
      setHotConceptLoading(false)
    }
  }, [quickAnalysisCacheDate])

  const loadConceptConstituents = useCallback(async (code: string, name: string) => {
    if (!code) return
    try {
      console.log('[Constituents] loading for', code, name)
      const list = await FetchHotConceptConstituents(code, name)
      console.log('[Constituents] loaded', list?.length, 'stocks for', code)
      setConceptConstituents((prev) => ({ ...prev, [code]: list || [] }))
      // 计算成分股主力净流入加总，用于替代板块指数f62
      const sum = (list || []).reduce((acc, s) => acc + (s.main_inflow || 0), 0)
      setConceptMainInflowSum((prev) => ({ ...prev, [code]: sum }))
    } catch (err: any) {
      console.error('加载成分股失败:', err)
      setConceptConstituents((prev) => ({ ...prev, [code]: [] }))
    }
  }, [])

  const loadQuickAnalysis = useCallback(async (code: string, name: string, market: string) => {
    const conceptCode = selectedHotConceptCode || ''
    // 优先读取缓存
    if (conceptCode && quickAnalysisCache[conceptCode]?.[code]) {
      setQuickAnalysisCode(code)
      setQuickAnalysisData(quickAnalysisCache[conceptCode][code])
      setQuickAnalysisLoading(false)
      return
    }
    setQuickAnalysisCode(code)
    setQuickAnalysisLoading(true)
    setQuickAnalysisData(null)
    try {
      const data = await QuickAnalyzeStock(code, name, market, conceptCode)
      setQuickAnalysisData(data)
      // 写入缓存
      if (conceptCode && data) {
        setQuickAnalysisCache((prev) => ({
          ...prev,
          [conceptCode]: {
            ...(prev[conceptCode] || {}),
            [code]: data,
          },
        }))
      }
    } catch (err: any) {
      console.error('快速分析失败:', err)
      setQuickAnalysisData(null)
    } finally {
      setQuickAnalysisLoading(false)
    }
  }, [selectedHotConceptCode, quickAnalysisCache])

  const loadComparables = useCallback(async (code: string) => {
    try {
      const list = await GetComparables(code)
      setComparables(list || [])
      setAppliedComparables(list || [])
    } catch {
      setComparables([])
      setAppliedComparables([])
    }
  }, [])

  const loadQuote = useCallback(async (code: string) => {
    try {
      setQuoteError('')
      const q = await GetStockQuote(code)
      setQuote(q || null)
    } catch (err: any) {
      setQuote(null)
      setQuoteError('行情获取失败，请检查网络')
      console.error('行情加载失败:', err)
    }
  }, [])

  const loadRiskRadar = useCallback(async (code: string, industry: string) => {
    try {
      const items = await GetRiskRadar(code, industry)
      setRiskRadar(items || [])
    } catch (err: any) {
      setRiskRadar([])
      console.error('风险雷达加载失败:', err)
    }
  }, [])


  // K线数据由 UnifiedChart 组件内部管理
  // const loadKlines = useCallback(async (code: string) => {
  //   try {
  //     setKlineError('')
  //     const list = await GetStockKlines(code)
  //     setKlines(list || [])
  //   } catch (err: any) {
  //     setKlines([])
  //     setKlineError('K线数据获取失败')
  //     console.error('K线加载失败:', err)
  //   }
  // }, [])

  const handleSelectSuggestion = async (stock: Stock) => {
    setQuery('')
    setShowDropdown(false)
    setLoading(true)
    try {
      await AddToWatchlist(stock.code)
      const list = await GetWatchlist()
      setWatchlist(list || [])
      // 刷新筛选数据
      GetWatchlistFilterData().then((fd) => {
        const map: Record<string, WatchlistFilterItem> = {}
        ;(fd || []).forEach((item) => { map[item.code] = item })
        setFilterData(map)
      })
      setSelectedCode(stock.code)
      setProfile(null)
      setQuote(null)
      setQuoteError('')
      // setKlines([])
      // setKlineError('')
      setDownloadResult(null)
      setDownloadSuggestion('')
      setReport(null)
      setViewingHistory(null)
      setHistoryContent('')
      setActiveReportTab('report')
      setAiReport(null)
      setAiReportError(null)
      setCompReportsDownloaded(false)
      setComparables([])
      setAppliedComparables([])
      setCompRecommendations([])
      setDataHistory([])
      setDataMissing(false)
      setInteractQAVisibleCount(5) // 切换股票时重置互动问答显示数量
      await loadReportHistory(stock.code, true)
      await loadDataHistory(stock.code)
      const p = await loadProfile(stock.code)
      await loadConcepts(stock.code)
      await loadComparables(stock.code)
      await loadQuote(stock.code)
      await loadRiskRadar(stock.code, p?.industry || '')
      // await loadKlines(stock.code)
    } catch (e) {
      alert(String(e))
    } finally {
      setLoading(false)
    }
  }

  const handleRemove = async (code: string, e: React.MouseEvent) => {
    e.stopPropagation()
    setLoading(true)
    try {
      await RemoveFromWatchlist(code)
      const list = await GetWatchlist()
      setWatchlist(list || [])
      // 刷新筛选数据
      GetWatchlistFilterData().then((fd) => {
        const map: Record<string, WatchlistFilterItem> = {}
        ;(fd || []).forEach((item) => { map[item.code] = item })
        setFilterData(map)
      })
      if (selectedCode === code) {
        setSelectedCode(null)
        setProfile(null)
        setQuote(null)
        setQuoteError('')
        // setKlines([])
        // setKlineError('')
        setImportResult(null)
        setDownloadResult(null)
        setDownloadSuggestion('')
        setReport(null)
        setViewingHistory(null)
        setHistoryContent('')
        setDataHistory([])
        setComparables([])
        setConcepts(null)
      }
    } catch (err) {
      alert(String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleImport = async () => {
    if (!selectedStock) return
    setLoading(true)
    try {
      const result = await ImportFinancialReports(selectedStock.code)
      setImportResult(result)
      if (result && result.success) {
        alert(`导入成功！\n${result.message}\n资产负债表年份: ${result.balanceSheet?.join(', ') || '无'}\n利润表年份: ${result.income?.join(', ') || '无'}\n现金流量表年份: ${result.cashFlow?.join(', ') || '无'}`)
        await loadDataHistory(selectedStock.code)
        const cache = await CheckAnalysisCache(selectedStock.code)
        setDataMissing(!!cache?.dataMissing)
      } else {
        alert('导入失败')
      }
    } catch (err: any) {
      console.error('导入失败:', err)
      alert(String(err))
    } finally {
      setLoading(false)
    }
  }

  const handleDownload = async () => {
    if (!selectedStock) return
    setDownloading(true)
    setDownloadStatus({type: null, message: ''})
    setDownloadSuggestion('')
    try {
      const maxYears = typeof settings.reportYears === 'number' && settings.reportYears > 0
        ? Math.floor(settings.reportYears)
        : 5
      const result = await Promise.race([
        DownloadReports(selectedStock.code, maxYears),
        new Promise<never>((_, reject) =>
          setTimeout(() => reject(new Error('下载超时，请检查网络或刷新页面后重试')), 30000)
        )
      ]) as Awaited<ReturnType<typeof DownloadReports>>
      setDownloadResult(result)
      if (result.success) {
        // 简化消息：年份多时显示范围
        const years = result.years || []
        let yearStr = ''
        if (years.length > 0) {
          if (years.length <= 3) {
            yearStr = years.join(', ')
          } else {
            yearStr = `${years[0]}～${years[years.length - 1]}`
          }
        }
        const msg = `✅ 下载成功${yearStr ? '，包含' + yearStr + '年' : ''}`
        setDownloadStatus({type: 'success', message: msg})
        setDownloadSuggestion(result.sourceSuggestion || '')
        console.log('[handleDownload] Reloading data history for:', selectedStock.code)
        await loadDataHistory(selectedStock.code)
        const cache = await CheckAnalysisCache(selectedStock.code)
        setDataMissing(!!cache?.dataMissing)
        console.log('[handleDownload] Data history reloaded, count:', dataHistory.length)
        // 刷新筛选数据
        GetWatchlistFilterData().then((fd) => {
          const map: Record<string, WatchlistFilterItem> = {}
          ;(fd || []).forEach((item) => { map[item.code] = item })
          setFilterData(map)
        })
        // 3秒后清除成功消息
        setTimeout(() => setDownloadStatus({type: null, message: ''}), 3000)
      } else {
        setDownloadStatus({type: 'error', message: '❌ 下载失败'})
      }
    } catch (err: any) {
      console.error('下载失败:', err)
      const msg = err?.message || String(err)
      if (msg.includes('companyType') || msg.includes('未找到') || msg.includes('无数据') || msg.includes('无法确定')) {
        setDownloadStatus({type: 'error', message: '❌ 该股票财报暂不可从网络获取，建议手动导入CSV'})
      } else if (msg.includes('timeout') || msg.includes('Timeout') || msg.includes('超时')) {
        setDownloadStatus({type: 'error', message: '❌ 网络超时，请稍后重试'})
      } else {
        setDownloadStatus({type: 'error', message: '❌ ' + msg})
      }
    } finally {
      setDownloading(false)
    }
  }

  const handleExportCurrentData = async () => {
    if (!selectedStock) return
    try {
      await ExportCurrentFinancialData(selectedStock.code)
    } catch (err: any) {
      console.error('导出当前数据失败:', err)
      alert('导出失败: ' + String(err))
    }
  }

  // 根据分析进度返回当前阶段描述
  const getAnalyzeStageText = (p: number) => {
    if (p < 25) return '初始化/获取行情...'
    if (p < 50) return '获取舆情/K线/资金流向...'
    if (p < 75) return 'ML三引擎推理/风险扫描...'
    if (p < 90) return '财报透视分析...'
    return '生成报告中...'
  }

  const runAnalyze = async (overwriteLatest = false) => {
    if (!selectedStock) {
      alert('没有选择股票')
      return
    }
    
    setAnalyzing(true)
    setAnalyzeProgress(5)
    const interval = setInterval(() => {
      setAnalyzeProgress((p) => {
        if (p >= 90) return 92    // 90-92%：极慢前进后停住
        if (p >= 75) return p + 1 // 75-90%：慢速（财报透视分析+报告生成）
        if (p >= 50) return p + 2 // 50-75%：中速（对应ML推理+风险扫描）
        if (p >= 25) return p + 2 // 25-50%：中速（对应网络数据获取）
        return p + 3              // 5-25%：快速（初始化阶段）
      })
    }, 500)
    try {
      const result = await AnalyzeStock(selectedStock.code, overwriteLatest)
      setReport(result)
      setViewingHistory(null)
      setHistoryContent('')
      setActiveReportTab('report')
      if (settings.analysisNotification) {
        SendNotification('分析完成', `${selectedStock.name || selectedStock.code} 的财报分析已完成`).catch(() => {})
      }
      if (result) {
        setSnapshots((prev) => ({ ...prev, [selectedStock.code]: result }))
      }
      setAppliedComparables(comparables)
      await loadReportHistory(selectedStock.code)
      // 刷新筛选数据
      GetWatchlistFilterData().then((fd) => {
        const map: Record<string, WatchlistFilterItem> = {}
        ;(fd || []).forEach((item) => { map[item.code] = item })
        setFilterData(map)
      })
    } catch (err: any) {
      console.error('分析失败:', err)
      const errorMsg = err?.message || String(err) || '未知错误'
      alert('分析失败: ' + errorMsg)
    } finally {
      clearInterval(interval)
      setAnalyzeProgress(100)
      setTimeout(() => {
        setAnalyzing(false)
        setAnalyzeProgress(0)
      }, 400)
    }
  }

  const handleAnalyze = async () => {
    if (!selectedStock) {
      alert('请选择一只股票')
      return
    }
    
    // 检查是否有财务数据
    if (dataHistory.length === 0) {
      alert('请先下载或导入财报数据')
      return
    }
    
    let overwriteLatest = false
    
    try {
      const cache = await CheckAnalysisCache(selectedStock.code)
      setDataMissing(!!cache?.dataMissing)

      if (cache?.dataMissing) {
        alert('请先下载或导入财报数据')
        return
      }
      
      if (cache?.unchanged) {
        setLastAnalysisAt(cache.lastAnalysisAt || '')
        setForceAnalyzeOpen(true)
        return
      }
      // 数据没变但可比公司变了：覆盖上次报告
      overwriteLatest = !cache?.dataChanged && !!cache?.comparablesChanged
    } catch (err: any) {
      console.error('检查分析缓存失败:', err)
      // 继续执行分析，不要阻塞用户
    }
    
    await runAnalyze(overwriteLatest)
  }

  const handleAnalyzeAI = useCallback(async (forceRefresh = false) => {
    if (!selectedStock) {
      alert('请选择一只股票')
      return
    }
    setActiveReportTab('ai')
    setAiReport(null)
    setAiReportLoading(true)
    setAiReportError(null)
    setAiProgress({ stage: 'init', message: '正在初始化...' })
    const targetCode = selectedStock.code
    setAiAnalyzingCode(targetCode)
    try {
      const result = await AnalyzeStockWithAI(selectedStock.code, selectedStock.name || '', forceRefresh)
      // 如果分析过程中切换了股票，忽略旧结果
      if (targetCode === selectedCodeRef.current) {
        setAiReport(result)
      }
    } catch (err: any) {
      console.error('[AIResearch] 分析失败:', err)
      if (targetCode === selectedCodeRef.current) {
        setAiReportError(err?.message || 'AI 投研分析失败')
      }
    } finally {
      if (targetCode === selectedCodeRef.current) {
        setAiReportLoading(false)
        setAiProgress(null)
      }
      setAiAnalyzingCode((prev) => prev === targetCode ? null : prev)
    }
  }, [selectedStock])

  const handleCancelAI = useCallback(async () => {
    if (!selectedStock) return
    try {
      await CancelAIResearch(selectedStock.code)
    } catch (err: any) {
      console.error('取消 AI 投研失败:', err)
    }
    setAiReportLoading(false)
    setAiProgress(null)
    setAiAnalyzingCode((prev) => prev === selectedStock.code ? null : prev)
  }, [selectedStock])

  const openRIMModal = () => {
    if (!selectedStock) return
    // 优先用当前报告中的RIM数据预填充，否则用默认值
    const rim = report?.rim
    if (rim && rim.hasData) {
      setRimBeta(rim.beta ?? 0.98)
      setRimRf((rim.rf ?? 0.0183) * 100)
      setRimRfDate(rim.rfDate ?? '')
      setRimRmRf((rim.rmRf ?? 0.0517) * 100)
      setRimRmRfDate(rim.rmRfDate ?? '')
      setRimBetaDate(rim.betaDate ?? '')
      setRimG((rim.params?.GTerminal ?? 0.05) * 100)
      setRimBPS0(rim.params?.BPS0 ?? 0)
      setRimPrice(rim.params?.CurrentPrice ?? 0)
      let eps: (number | string)[] = []
      if (rim.epsRaw && Object.keys(rim.epsRaw).length > 0) {
        const years = Object.keys(rim.epsRaw).sort()
        eps = years.slice(0, 6).map((y) => rim.epsRaw![y].toFixed(2))
      }
      const forecast = rim.params?.Forecast?.EPS?.slice(0, 6) || []
      while (eps.length < 6) {
        eps.push((forecast[eps.length] ?? 0).toFixed(2))
      }
      setRimEPS(eps)
    } else if (quote) {
      // 从行情推算默认值
      setRimBeta(0.98)
      setRimRfDate('')
      setRimRf(1.83)
      setRimRmRfDate('')
      setRimRmRf(5.17)
      setRimBetaDate('')
      setRimG(5.0)
      setRimBPS0(quote.pb > 0 ? quote.currentPrice / quote.pb : 0)
      setRimPrice(quote.currentPrice)
      setRimEPS([0, 0, 0, 0, 0, 0])
    }
    setShowRIMModal(true)
  }

  const handleAnalyzeWithRIM = async () => {
    if (!selectedStock) return
    setRimLoading(true)
    setRimProgress(5)
    const interval = setInterval(() => {
      setRimProgress((p) => (p >= 90 ? 90 : p + 3))
    }, 400)
    try {
      const params = {
        BPS0: rimBPS0,
        KE: rimRf / 100 + rimBeta * (rimRmRf / 100),
        GTerminal: rimG / 100,
        Forecast: { EPS: rimEPS.map(Number).filter((v) => v > 0), DPS: [] },
        CurrentPrice: rimPrice,
      }
      const rimData = {
        hasData: true,
        params,
        result: null as any,
        rf: rimRf / 100,
        beta: rimBeta,
        rmRf: rimRmRf / 100,
      }
      const rimJSON = JSON.stringify(rimData)
      const result = await AnalyzeStockWithRIM(selectedStock.code, false, rimJSON)
      setReport(result)
      setViewingHistory(null)
      setHistoryContent('')
      if (settings.analysisNotification) {
        SendNotification('分析完成', `${selectedStock.name || selectedStock.code} 的财报分析（含RIM估值）已完成`).catch(() => {})
      }
      if (result) {
        setSnapshots((prev) => ({ ...prev, [selectedStock.code]: result }))
      }
      setAppliedComparables(comparables)
      await loadReportHistory(selectedStock.code)
      setShowRIMModal(false)
    } catch (err: any) {
      console.error('RIM分析失败:', err)
      alert(String(err))
    } finally {
      clearInterval(interval)
      setRimProgress(100)
      setTimeout(() => {
        setRimLoading(false)
        setRimProgress(0)
      }, 400)
    }
  }

  const handleReportDownload = async () => {
    if (!selectedStock || !displayContent) {
      return
    }
    const content = viewingHistory ? historyContent : report?.markdownContent
    if (!content) {
      alert('没有可下载的报告内容')
      return
    }
    try {
      await DownloadReport(selectedStock.code, content)
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) {
        return
      }
      alert('下载报告失败: ' + msg)
    }
  }

  const handleExportPDF = async () => {
    if (!selectedStock || !reportContentRef.current) return
    const markdownBody = reportContentRef.current.querySelector('.markdown-body') as HTMLElement | null
    if (!markdownBody) {
      alert('未找到报告内容')
      return
    }
    try {
      const opt: any = {
        margin: [10, 10, 10, 10],
        filename: `${selectedStock.code}_投资分析报告.pdf`,
        image: { type: 'jpeg', quality: 0.98 },
        html2canvas: {
          scale: 2,
          useCORS: true,
          backgroundColor: '#ffffff',
          onclone: (clonedDoc: Document) => {
            // 在克隆的 DOM 中注入亮色样式，不依赖外部 CSS 是否正确复制
            const style = clonedDoc.createElement('style')
            style.textContent = `
              .markdown-body { color: #1f2937 !important; background: #ffffff !important; }
              .markdown-body h1, .markdown-body h2, .markdown-body h3, .markdown-body h4, .markdown-body h5, .markdown-body h6 { color: #111827 !important; border-bottom-color: #e5e7eb !important; }
              .markdown-body p, .markdown-body li, .markdown-body span, .markdown-body div, .markdown-body strong, .markdown-body em, .markdown-body td { color: #1f2937 !important; }
              .markdown-body th { background: #f3f4f6 !important; color: #111827 !important; }
              .markdown-body td, .markdown-body tr { background: #ffffff !important; }
              .markdown-body th, .markdown-body td { border-color: #e5e7eb !important; }
              .markdown-body a { color: #2563eb !important; }
              .markdown-body blockquote { background: rgba(59,130,246,0.06) !important; color: #4b5563 !important; }
              .markdown-body code { background: rgba(0,0,0,0.06) !important; }
              .markdown-body pre { background: #f9fafb !important; border-color: #e5e7eb !important; }
              .markdown-body hr { border-top-color: #e5e7eb !important; }
            `
            clonedDoc.head.appendChild(style)
          },
        },
        jsPDF: { unit: 'mm', format: 'a4', orientation: 'portrait' },
      }
      const pdfDataUrl: string = await html2pdf().set(opt).from(markdownBody).outputPdf('datauristring')
      // 去掉 data:application/pdf;base64, 前缀
      const base64Data = pdfDataUrl.split(',')[1]
      await ExportReportPDF(selectedStock.code, base64Data)
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) {
        return
      }
      alert('导出PDF失败: ' + msg)
    }
  }

  const handleDownloadImage = async () => {
    if (!selectedStock || !reportContentRef.current) return
    const markdownBody = reportContentRef.current.querySelector('.markdown-body') as HTMLElement | null
    if (!markdownBody) {
      alert('没有可下载的报告内容')
      return
    }
    try {
      const dataUrl = await toPng(markdownBody, {
        quality: 0.95,
        backgroundColor: getComputedStyle(document.body).backgroundColor,
        pixelRatio: 2,
      })
      await ExportReportImage(selectedStock.code, dataUrl)
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) {
        return
      }
      alert('生成图片失败: ' + msg)
    }
  }

  // AI 投研报告导出
  const reportToTxt = (report: ai_researcher.AIResearchReport): string => {
    const sentimentMap: Record<string, string> = { positive: '乐观', neutral: '中性', negative: '谨慎' }
    const lines: string[] = []
    lines.push(`AI 投研报告：${report.name || report.symbol}`)
    lines.push(`股票代码：${report.symbol}`)
    lines.push(`生成时间：${new Date(report.generated_at).toLocaleString('zh-CN')}`)
    lines.push(`使用模型：${report.model_used}${report.from_cache ? '（来自缓存）' : ''}`)
    lines.push('')
    lines.push('====================================')
    lines.push('')
    report.sections?.forEach((section) => {
      lines.push(`【${section.title}】`)
      lines.push(`情绪：${sentimentMap[section.sentiment] || section.sentiment}`)
      lines.push('')
      lines.push(section.summary)
      lines.push('')
      if (section.key_points && section.key_points.length > 0) {
        lines.push('要点：')
        section.key_points.forEach((point, idx) => {
          lines.push(`${idx + 1}. ${point}`)
        })
        lines.push('')
      }
      lines.push('------------------------------------')
      lines.push('')
    })
    if (report.sources && report.sources.length > 0) {
      lines.push('参考来源：')
      report.sources.forEach((source, idx) => {
        lines.push(`${idx + 1}. ${source.title}${source.date ? ` (${source.date})` : ''}`)
        lines.push(`   ${source.url}`)
      })
    }
    lines.push('')
    lines.push('免责声明：AI 分析仅供参考，请以上市公司公告和官方数据为准。')
    return lines.join('\n')
  }

  const reportToMd = (report: ai_researcher.AIResearchReport): string => {
    const sentimentMap: Record<string, string> = { positive: '乐观', neutral: '中性', negative: '谨慎' }
    const lines: string[] = []
    lines.push(`# AI 投研报告：${report.name || report.symbol}`)
    lines.push('')
    lines.push(`- **股票代码**：${report.symbol}`)
    lines.push(`- **生成时间**：${new Date(report.generated_at).toLocaleString('zh-CN')}`)
    lines.push(`- **使用模型**：${report.model_used}${report.from_cache ? '（来自缓存）' : ''}`)
    lines.push('')
    lines.push('---')
    lines.push('')
    report.sections?.forEach((section) => {
      lines.push(`## ${section.title}`)
      lines.push('')
      lines.push(`**情绪**：${sentimentMap[section.sentiment] || section.sentiment}`)
      lines.push('')
      lines.push(section.summary)
      lines.push('')
      if (section.key_points && section.key_points.length > 0) {
        lines.push('### 要点')
        lines.push('')
        section.key_points.forEach((point) => {
          lines.push(`- ${point}`)
        })
        lines.push('')
      }
      lines.push('---')
      lines.push('')
    })
    if (report.sources && report.sources.length > 0) {
      lines.push('## 参考来源')
      lines.push('')
      report.sources.forEach((source) => {
        lines.push(`- [${source.title}${source.date ? ` (${source.date})` : ''}](${source.url})`)
      })
      lines.push('')
    }
    lines.push('> ⚠️ **免责声明**：AI 分析仅供参考，请以上市公司公告和官方数据为准。')
    return lines.join('\n')
  }

  const handleExportAIResearchTxt = async () => {
    if (!selectedStock || !aiReport) return
    try {
      await ExportAIResearchTxt(selectedStock.code, reportToTxt(aiReport))
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) return
      alert('导出 TXT 失败: ' + msg)
    }
  }

  const handleExportAIResearchMd = async () => {
    if (!selectedStock || !aiReport) return
    try {
      await ExportAIResearchMd(selectedStock.code, reportToMd(aiReport))
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) return
      alert('导出 Markdown 失败: ' + msg)
    }
  }

  const handleCopyAIResearchTxt = async () => {
    if (!aiReport) return
    try {
      await navigator.clipboard.writeText(reportToTxt(aiReport))
      alert('已复制 TXT 格式到剪贴板')
    } catch (err: any) {
      alert('复制 TXT 失败: ' + String(err))
    }
  }

  const handleCopyAIResearchMd = async () => {
    if (!aiReport) return
    try {
      await navigator.clipboard.writeText(reportToMd(aiReport))
      alert('已复制 Markdown 格式到剪贴板')
    } catch (err: any) {
      alert('复制 Markdown 失败: ' + String(err))
    }
  }

  const handleExportAIResearchPdf = async () => {
    if (!selectedStock || !aiReport || !aiReportContentRef.current) return
    try {
      const opt = {
        margin: [12, 12, 12, 12] as [number, number, number, number],
        filename: `${selectedStock.code}_AI投研报告.pdf`,
        image: { type: 'jpeg' as const, quality: 0.95 },
        html2canvas: {
          scale: 2,
          useCORS: true,
          backgroundColor: '#ffffff',
          onclone: (clonedDoc: Document) => {
            const style = clonedDoc.createElement('style')
            style.textContent = `
              .ai-research-content { color: #1f2937 !important; background: #ffffff !important; }
              .ai-research-content h2 { color: #111827 !important; }
              .ai-research-content p, .ai-research-content li, .ai-research-content span, .ai-research-content div { color: #1f2937 !important; }
              .ai-research-disclaimer { background: rgba(245,158,11,0.08) !important; color: #b45309 !important; }
              .ai-research-section { background: #ffffff !important; border-left-color: #3b82f6 !important; }
              .ai-research-sources { background: #f8fafc !important; }
              .ai-research-sources-list a { color: #2563eb !important; }
            `
            clonedDoc.head.appendChild(style)
          },
        },
        jsPDF: { unit: 'mm' as const, format: 'a4' as const, orientation: 'portrait' as const },
      }
      const pdfDataUrl: string = await html2pdf().set(opt).from(aiReportContentRef.current).outputPdf('datauristring')
      const base64Data = pdfDataUrl.split(',')[1]
      await ExportAIResearchPdf(selectedStock.code, base64Data)
    } catch (err: any) {
      const msg = String(err)
      if (msg.includes('取消保存') || msg.includes('用户取消')) return
      alert('导出 PDF 失败: ' + msg)
    }
  }

  // 下载菜单点击外部关闭
  useEffect(() => {
    if (!downloadMenuOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (
        downloadMenuRef.current &&
        !downloadMenuRef.current.contains(e.target as Node) &&
        downloadMenuBtnRef.current &&
        !downloadMenuBtnRef.current.contains(e.target as Node)
      ) {
        setDownloadMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [downloadMenuOpen])

  // AI 投研导出菜单点击外部关闭
  useEffect(() => {
    if (!aiExportMenuOpen) return
    const handleClickOutside = (e: MouseEvent) => {
      if (
        aiExportMenuRef.current &&
        !aiExportMenuRef.current.contains(e.target as Node) &&
        aiExportMenuBtnRef.current &&
        !aiExportMenuBtnRef.current.contains(e.target as Node)
      ) {
        setAiExportMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [aiExportMenuOpen])

  const handleDeleteReport = async () => {
    if (!selectedStock || !displayContent) {
      return
    }
    const filename = viewingHistory || 'latest.md'
    const confirmed = await ConfirmDialog('确认删除', `确定删除报告 ${filename} 吗？`)
    if (!confirmed) {
      return
    }
    try {
      await DeleteReport(selectedStock.code, filename)
      setViewingHistory(null)
      setHistoryContent('')
      setReport(null)
      setLastAnalysisAt('')
      // 同时清理该股票的快照，避免左下角亮点与风险面板仍显示旧数据
      setSnapshots((prev) => {
        const next = { ...prev }
        delete next[selectedStock.code]
        return next
      })
    } catch (err: any) {
      alert('删除报告失败: ' + String(err))
    }
  }

  const compSuggestions = useMemo(() => {
    const q = compQuery.trim()
    if (!q) return []
    const lower = q.toLowerCase()
    return STOCKS.filter(
      (s) =>
        s.code !== selectedCode &&
        (
          s.code.toLowerCase().includes(lower) ||
          s.name.toLowerCase().includes(lower) ||
          (pinyinMap.get(s.code) || '').includes(lower)
        )
    ).slice(0, 10)
  }, [compQuery, selectedCode, pinyinMap])

  const handleAddComparable = async (stock: Stock) => {
    if (!selectedStock || stock.code === selectedStock.code) return
    try {
      await AddComparable(selectedStock.code, stock.code)
      const list = await GetComparables(selectedStock.code)
      setComparables(list || [])
      setCompReportsDownloaded(false) // 可比公司变化，需要重新下载
      setCompQuery('')
      setShowCompDropdown(false)
    } catch (err: any) {
      alert(String(err))
    }
  }

  const handleRemoveComparable = async (code: string) => {
    if (!selectedStock) return
    try {
      await RemoveComparable(selectedStock.code, code)
      const list = await GetComparables(selectedStock.code)
      setComparables(list || [])
      setCompReportsDownloaded(false) // 可比公司变化，需要重新下载
    } catch (err: any) {
      alert(String(err))
    }
  }

  const handleDownloadComparables = async () => {
    if (!selectedStock || comparables.length === 0) return
    setCompDownloading(true)
    setCompDownloadStatus({type: null, message: ''})
    try {
      const result = await DownloadComparableReports(selectedStock.code)
      if (result) {
        if (result.success) {
          setCompReportsDownloaded(true)
          setCompDownloadStatus({type: 'success', message: result.message})
          setTimeout(() => setCompDownloadStatus({type: null, message: ''}), 3000)
        } else {
          setCompDownloadStatus({type: 'error', message: result.message || '下载失败'})
          setTimeout(() => setCompDownloadStatus({type: null, message: ''}), 5000)
        }
      }
    } catch (err: any) {
      console.error('下载可比公司财报失败:', err)
      const msg = err?.message || String(err)
      setCompDownloadStatus({type: 'error', message: msg.length > 60 ? msg.slice(0, 60) + '...' : msg})
      setTimeout(() => setCompDownloadStatus({type: null, message: ''}), 5000)
    } finally {
      setCompDownloading(false)
    }
  }

  const handleFetchMissingActivity = async () => {
    if (!selectedStock || comparables.length === 0) return
    setFetchingActivity(true)
    setFetchActivityStatus({type: null, message: ''})
    // 保存当前滚动位置，避免更新后跳动
    const scrollContainer = reportContentRef.current
    const scrollTop = scrollContainer?.scrollTop ?? 0
    try {
      const result = await FetchMissingActivity(comparables)
      if (result && result.successCount > 0) {
        setFetchActivityStatus({type: 'success', message: '正在更新模块4...'})
        const module4Result = await UpdateModule4Only(selectedStock.code)
        if (module4Result) {
          // 只更新 markdownContent，保留报告其他字段，避免右栏跳动
          if (report) {
            setReport({ ...report, markdownContent: module4Result.markdownContent } as AnalysisReport)
          } else {
            setReport(module4Result)
          }
          setSnapshots((prev) => ({ ...prev, [selectedStock.code]: module4Result }))
          setFetchActivityStatus({type: 'success', message: `已更新 ${result.successCount} 家公司活跃度`})
          // 恢复滚动位置
          requestAnimationFrame(() => {
            if (reportContentRef.current) {
              reportContentRef.current.scrollTop = scrollTop
            }
          })
        }
      } else if (result && result.failCount > 0) {
        setFetchActivityStatus({type: 'error', message: result.message || '获取失败'})
      } else {
        setFetchActivityStatus({type: 'success', message: '所有公司活跃度已是最新'})
      }
    } catch (err: any) {
      console.error('获取缺失活跃度失败:', err)
      setFetchActivityStatus({type: 'error', message: err?.message || String(err)})
    } finally {
      setFetchingActivity(false)
      setTimeout(() => setFetchActivityStatus({type: null, message: ''}), 4000)
    }
  }

  const handleRecommendComparables = async () => {
    if (!selectedStock) return
    setCompRecommending(true)
    try {
      const result = await RecommendComparables(selectedStock.code)
      setCompRecommendations(result || [])
    } catch (e) {
      console.error('推荐可比公司失败:', e)
    } finally {
      setCompRecommending(false)
    }
  }

  const handleAddRecommendedComparable = async (symbol: string) => {
    if (!selectedStock || comparables.includes(symbol) || comparables.length >= 7) return
    try {
      await AddComparable(selectedStock.code, symbol)
      const list = await GetComparables(selectedStock.code)
      setComparables(list || [])
      setAppliedComparables(list || [])
    } catch (err: any) {
      console.error('添加推荐可比公司失败:', err)
      alert('添加失败: ' + String(err))
      return
    }
    setCompRecommendations((prev) => prev.filter((r) => r.symbol !== symbol))
  }

  const handleAddAllRecommendations = async () => {
    if (!selectedStock) return
    const toAdd = compRecommendations.filter(r => !comparables.includes(r.symbol))
    if (toAdd.length === 0) return
    try {
      for (const r of toAdd) {
        if (comparables.length >= 7) break
        await AddComparable(selectedStock.code, r.symbol)
      }
      const list = await GetComparables(selectedStock.code)
      setComparables(list || [])
      setAppliedComparables(list || [])
      // 清空推荐结果
      setCompRecommendations([])
    } catch (err: any) {
      console.error('批量添加推荐可比公司失败:', err)
      alert('批量添加失败: ' + String(err))
    }
  }

  const handleAnalyzeWithComparables = async () => {
    if (!selectedStock || comparables.length === 0) return
    setAnalyzing(true)
    setAnalyzeProgress(5)
    const interval = setInterval(() => {
      setAnalyzeProgress((p) => (p >= 90 ? 90 : p + 5))
    }, 300)
    try {
      // 只更新模块4，不重新下载财报，不跑完整分析
      const result = await UpdateModule4Only(selectedStock.code)
      setReport(result)
      setViewingHistory(null)
      setHistoryContent('')
      if (result) {
        setSnapshots((prev) => ({ ...prev, [selectedStock.code]: result }))
      }
      setAppliedComparables(comparables)
      await loadReportHistory(selectedStock.code)
      setTimeout(() => {
        handleTocJump('模块4-行业横向对比分析')
      }, 150)
    } catch (err: any) {
      console.error('更新模块4失败:', err)
      alert(String(err))
    } finally {
      clearInterval(interval)
      setAnalyzeProgress(100)
      setTimeout(() => {
        setAnalyzing(false)
        setAnalyzeProgress(0)
      }, 400)
    }
  }

  const displayContent = viewingHistory ? historyContent : report?.markdownContent

  function formatTraceValue(v: number): string {
    const abs = Math.abs(v)
    if (abs >= 1e8) return `${(v / 1e8).toFixed(2)} 亿元`
    if (abs >= 1e4) return `${(v / 1e4).toFixed(2)} 万元`
    return `${v.toFixed(0)} 元`
  }

  function formatTraceResult(v: number, indicator: string): string {
    if (indicator.includes('率') || indicator === 'ROE' || indicator === '毛利率') {
      return `${v.toFixed(2)}%`
    }
    return formatTraceValue(v)
  }

  // 切换报告时清除搜索高亮和 trace，并更新全局 Markdown 内容
  useEffect(() => {
    clearSearchHighlights()
    reportLastQueryRef.current = ''
    if (reportSearchRef.current) {
      reportSearchRef.current.value = ''
    }
    setTraceDrawerOpen(false)
    setCurrentTrace(null)
    setTraceList([])
    // 设置全局 Markdown 内容供模块复制功能使用
    setGlobalMarkdownContent(displayContent || '')
  }, [displayContent])

  // 报告渲染后：为风险警示表格中有 Details 的风险项添加 inline-tooltip
  useEffect(() => {
    if (!displayContent) return
    const timer = setTimeout(() => {
      const reportContainer = reportContentRef.current
      if (!reportContainer) return

      const riskAlert = (report?.riskAlert || currentSnapshot?.riskAlert) as any
      if (!riskAlert?.flags?.length) return

      const tables = reportContainer.querySelectorAll('table')
      tables.forEach((table) => {
        const headers = table.querySelectorAll('th')
        if (headers.length < 4) return
        const headerTexts = Array.from(headers).map(h => h.textContent?.trim() || '')
        if (!headerTexts.includes('风险指标') || !headerTexts.includes('数值说明')) return

        const rows = table.querySelectorAll('tbody tr')
        rows.forEach((row, rowIdx) => {
          const cells = row.querySelectorAll('td')
          if (cells.length < 4) return

          const flag = riskAlert.flags[rowIdx]
          // 高风险和中风险项均显示信息图标 tooltip
          if (!flag || (flag.level !== 'high' && flag.level !== 'medium') || !flag.details?.length) return

          const valueCell = cells[2] // 数值说明列
          // 避免重复添加
          if (valueCell.querySelector('.inline-tooltip')) return

          const tooltipSpan = document.createElement('span')
          tooltipSpan.className = 'inline-tooltip'
          const triggerClass = flag.level === 'high' ? 'inline-tooltip-trigger inline-tooltip-trigger-high' : 'inline-tooltip-trigger inline-tooltip-trigger-medium'
          const bodyHtml = flag.details.map((d: string) => d.replace(/</g, '&lt;').replace(/>/g, '&gt;')).join('<br/>')
          tooltipSpan.innerHTML = '<span class="' + triggerClass + '">i</span><span class="inline-tooltip-body"><strong>' + flag.name + '</strong><br/>' + bodyHtml + '</span>'
          valueCell.appendChild(tooltipSpan)
        })
      })

      // 为所有 inline-tooltip 绑定智能定位（右侧空间不足时显示在左侧）
      document.querySelectorAll('.inline-tooltip').forEach((el) => {
        const tooltip = el as HTMLElement
        const trigger = tooltip.querySelector('.inline-tooltip-trigger') as HTMLElement | null
        if (!trigger) return
        if ((trigger as any)._tooltipSmartBound) return
        ;(trigger as any)._tooltipSmartBound = true

        const handleEnter = () => {
          const rect = trigger.getBoundingClientRect()
          const vw = window.innerWidth
          const bodyWidth = 360
          const offset = 8
          if (rect.right + offset + bodyWidth > vw - 8) {
            tooltip.classList.add('tooltip-left')
          } else {
            tooltip.classList.remove('tooltip-left')
          }
        }
        trigger.addEventListener('mouseenter', handleEnter)
      })
    }, 3000)
    return () => clearTimeout(timer)
  }, [displayContent, report, currentSnapshot])

  // 报告内容滚动时联动更新"跳转章节"下拉框显示
  useEffect(() => {
    const container = reportContentRef.current
    if (!container || !displayContent) return

    let rafId: number | null = null
    const handleScroll = () => {
      if (rafId) return
      rafId = requestAnimationFrame(() => {
        rafId = null
        const headings = container.querySelectorAll('h1')
        if (headings.length === 0 || !tocSelectRef.current) return
        const containerTop = container.getBoundingClientRect().top
        let closest: Element | null = null
        let closestOffset = Infinity
        for (const h of headings) {
          const offset = h.getBoundingClientRect().top - containerTop
          if (offset >= -40 && offset < closestOffset) {
            closest = h
            closestOffset = offset
          }
        }
        // 如果所有标题都在上方，取最后一个
        if (!closest && headings.length > 0) {
          closest = headings[headings.length - 1]
        }
        if (closest) {
          const id = closest.id
          const label = tocSections.find((s) => s.id === id)?.label || '📑 跳转章节'
          const firstOpt = tocSelectRef.current.querySelector('option:first-child') as HTMLOptionElement | null
          if (firstOpt) {
            firstOpt.textContent = '⬅ ' + label
          }
        }
      })
    }
    container.addEventListener('scroll', handleScroll)
    // 初始触发一次
    handleScroll()
    return () => {
      container.removeEventListener('scroll', handleScroll)
      if (rafId) cancelAnimationFrame(rafId)
    }
  }, [displayContent])

  const markdownComponents = useMemo(() => ({
    details: DetailsComponent,
    span({ className, 'data-steps': dataSteps, children, ...props }: any) {
      if (className === 'trace-trigger' && dataSteps) {
        const stepNums = String(dataSteps)
          .split(',')
          .map((s: string) => parseInt(s.trim(), 10))
          .filter((n: number) => !isNaN(n))
        return (
          <button
            className="trace-trigger-btn"
            onClick={() => {
              const sourceReport = report || currentSnapshot
              const matched =
                sourceReport?.stepResults?.flatMap((step) =>
                  stepNums.includes(step.stepNum) && step.traces ? step.traces : []
                ) || []
              if (matched.length > 0) {
                setTraceList(matched)
                setCurrentTrace(matched[0])
                setTraceDrawerOpen(true)
              } else {
                alert('暂无该指标的计算过程数据，请重新执行分析后再试。')
              }
            }}
            title="查看计算过程"
          >
            {children}
          </button>
        )
      }
      return (
        <span className={className} {...props}>
          {children}
        </span>
      )
    },
    code({ className, children, ...props }: any) {
      const text = String(children || '')
      const lang = className?.replace('language-', '') || ''
      const stockCode = selectedStock?.code || ''
      const stockName = selectedStock?.name || ''
      // 图表占位代码块（fenced code block，有 language-xxx className）
      if (lang === 'chart-unified' && stockCode) {
        return <UnifiedChart code={stockCode} name={stockName} quote={quote || undefined} />
      }
      if (lang === 'chart-financial-trend' && stockCode) {
        return <FinancialTrendChart code={stockCode} name={stockName} />
      }
      // 互动平台问答交互面板占位
      if (!className && text.trim() === 'INTERACT_QA_PANEL') {
        const qas = currentSnapshot?.interactQAs || []
        return <InteractQAPanel qas={qas} visibleCount={interactQAVisibleCount} setVisibleCount={setInteractQAVisibleCount} />
      }
      // 政策信号强度 inline code: signal:3（inline code 没有 className）
      if (!className && text.startsWith('signal:')) {
        const level = parseInt(text.replace('signal:', ''), 10)
        if (!isNaN(level) && level >= 1 && level <= 5) {
          return (
            <span style={{ display: 'inline-flex', alignItems: 'flex-end', gap: '2px', height: '14px', marginLeft: '4px' }}>
              {[1, 2, 3, 4, 5].map((i) => (
                <span
                  key={i}
                  style={{
                    width: '3px',
                    height: `${4 + i * 2}px`,
                    borderRadius: '1px',
                    backgroundColor: i <= level ? '#22c55e' : '#e2e8f0',
                    opacity: i <= level ? 1 : 0.4,
                  }}
                />
              ))}
            </span>
          )
        }
      }
      return (
        <code className={className} {...props}>
          {children}
        </code>
      )
    },
    pre({ children }: any) {
      // 检测是否为图表占位代码块，避免被 <pre> 包裹导致样式异常
      const child = children
      if (child && child.props && child.props.className) {
        const lang = String(child.props.className).replace('language-', '')
        if (lang === 'chart-unified' || lang === 'chart-financial-trend') {
          return <>{children}</>
        }
      }
      return <pre>{children}</pre>
    },
    a({ href, children, ...props }: any) {
      if (href === '#fetch-activity') {
        return (
          <a href="#fetch-activity" onClick={(e: React.MouseEvent) => { e.preventDefault(); handleFetchMissingActivity() }} {...props}>
            {children}
          </a>
        )
      }
      return <a href={href} {...props}>{children}</a>
    },
    // 为模块标题添加复制按钮（仅 h1 级别的模块标题）
    h1({ children, id, ...props }: any) {
      const titleText = children?.toString() || ''
      // 匹配模块标题：模块X: 标题
      const isModuleTitle = /^模块\d+/.test(titleText)
      const isModule6 = titleText.includes('模块6')
      const isModule7 = titleText.includes('模块7')
      // 强制修正模块7的 id，确保 TOC 导航匹配
      const headingId = isModule7 ? '模块7-a-score-综合风险画像' : id
      // 过滤掉 children 中的 trace-trigger（旧版后端可能残留）
      const filteredChildren = isModule7
        ? Children.map(children, (child: any) => {
            if (child && typeof child === 'object' && typeof child.props?.className === 'string' && child.props.className.includes('trace-trigger')) {
              return null
            }
            return child
          })
        : children
      
      return (
        <h1 id={headingId} {...props} style={{ position: 'relative', display: 'flex', alignItems: 'center', justifyContent: 'space-between', paddingRight: isModule7 ? '52px' : '32px' }}>
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {filteredChildren}
            {isModule7 && (
              <InlineTooltip
                title="A-Score 综合风险画像"
                body="A-Score（0-100分）综合评估企业财务风险，分数越高，潜在隐患越大。基于公开财务报表与监管信息，从六个维度打分：财务造假风险、偿债能力、现金流质量、应收账款健康度、盈利稳定性，以及股权质押/减持/监管问询等非财务信号。其中财务维度适用于 A 股与港股，非财务信号目前主要覆盖 A 股。评判标准：< 40分安全，40-60分低风险，60-70分中风险（需深入核查），≥ 70分高危（建议回避）。"
              />
            )}
          </span>
          <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            {isModule6 && (
              <button className="rim-adjust-btn" onClick={(e: React.MouseEvent) => { e.preventDefault(); e.stopPropagation(); openRIMModal() }}>调整RIM</button>
            )}
            {isModuleTitle && (
              <ModuleCopyButton moduleId={headingId || ''} moduleTitle={titleText} />
            )}
          </span>
        </h1>
      )
    },
    // 9.1 / 9.2 / 9.3 ML 模型小节标题旁添加信息图标
    h2({ children, ...props }: any) {
      const titleText = children?.toString() || ''
      let tip: { title: string; body: string } | null = null
      if (/^9\.1\s+Engine-B/.test(titleText)) {
        tip = {
          title: '9.1 Engine-B 财务趋势预测',
          body:
            '【模型】BiLSTM + Self-Attention，输入是过去 5 年的财务指标序列（营收、利润、现金流、资产负债、毛利等），输出未来 1–3 年的预测趋势分。\n\n' +
            '【如何理解】关注趋势方向和置信度，不要把绝对预测值当成精准估值。模型擅长捕捉「持续向好/持续恶化」的中长期信号，对突发事件无感。\n\n' +
            '【适用场景】判断公司基本面是处在改善通道还是下行通道，作为长期视角的辅助参考。'
        }
      } else if (/^9\.2\s+Engine-A/.test(titleText)) {
        tip = {
          title: '9.2 Engine-A 市场情绪与价格融合预测',
          body:
            '【模型】Cross-Attention 融合 K 线序列（价格/成交量）与舆情文本（公告、研报、新闻情感），输出未来短期（数日到数周）的方向倾向。\n\n' +
            '【如何理解】这是短期情绪/技术面信号，不代表基本面观点。舆情数据稀缺时（如冷门股）置信度会显著下降，应忽略其结论。\n\n' +
            '【适用场景】辅助择时，不作为持仓决策的主要依据。与 9.1（长期趋势）结合使用更稳妥。'
        }
      } else if (/^9\.3\s+Engine-D/.test(titleText)) {
        tip = {
          title: '9.3 Engine-D 风险预警模型',
          body:
            '【模型】GradientBoosting 二分类，使用 25 维特征（财务14+市场6+非财务5）预测「财务造假/退市」概率。\n\n' +
            '【如何理解】概率 < 40% 低风险，40%–70% 中风险（建议核查），≥ 70% 高风险（建议回避）。\n\n' +
            '【主要风险因子】带「(偏高)/(偏低)」的标签是针对当前这只股票相对健康公司均值的偏离方向，不是模型的全局通用偏好。比如 ar_risk(偏高) 表示该股的应收/营收比明显高于健康基准。\n\n' +
            '【局限】训练数据为 A 股历史造假/退市样本 + 健康对照，对军工、券商、银行等结构性特征异常的行业可能误报，需结合 9.1 / A-Score / 风险警示综合判断。'
        }
      }
      const isInteractQA = /^13\.4\s+互动平台近期问答/.test(titleText)
      return (
        <h2 {...props} style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 8 }}>
          {children}
          {tip && <InlineTooltip title={tip.title} body={tip.body} />}
          {isInteractQA && (
            <button
              onClick={handleRefreshInteractQA}
              disabled={refreshingInteractQA}
              title={refreshingInteractQA ? '正在刷新...' : '刷新互动问答'}
              style={{
                marginLeft: 'auto',
                padding: '2px 8px',
                fontSize: 12,
                color: refreshingInteractQA ? '#94a3b8' : '#3b82f6',
                background: 'transparent',
                border: `1px solid ${refreshingInteractQA ? 'rgba(148,163,184,0.3)' : 'rgba(59,130,246,0.3)'}`,
                borderRadius: 4,
                cursor: refreshingInteractQA ? 'not-allowed' : 'pointer',
                display: 'flex',
                alignItems: 'center',
                gap: 4,
                opacity: refreshingInteractQA ? 0.6 : 1,
              }}
            >
              <span style={{ display: 'inline-block', animation: refreshingInteractQA ? 'spin 1s linear infinite' : 'none' }}>🔄</span>
              {refreshingInteractQA ? '刷新中...' : '刷新'}
            </button>
          )}
        </h2>
      )
    },
    // 风险警示标题旁添加信息图标（hover 弹出说明）
    h3({ children, ...props }: any) {
      const titleText = children?.toString() || ''
      const isRiskAlert = /风险警示/.test(titleText)
      return (
        <h3 {...props} style={{ position: 'relative', display: 'flex', alignItems: 'center', gap: 8 }}>
          {children}
          {isRiskAlert && (
            <InlineTooltip
              title="风险警示功能说明"
              body={'本模块从三大维度检测股票风险。\n\n1. 一票否决（单项即高风险）：审计意见非标、审计机构异常更换、核心财务负责人频繁更换、资金占用/违规担保/诉讼、经营现金流连续2年为负、资产负债率>85%、营收同比<-30%、大股东质押>70%、一年内监管问询≥3次、毛利率为负。\n\n注：国企8年强制轮换期届满等政策合规更换，以及通过招标/选聘方式、连续服务多年后正常轮换的审计机构更换，系统会自动识别为正常轮换，以信息提示展示，不列入风险警示。\n\n2. 二级指标（3项及以上中风险）：A-Score 60–69分、M-Score>-1.78、毛利率下降>10百分点、ROE<5%、净利润现金含量<80%、应收+存货占比>40%、营收同比<0%、负债率70%–85%、商誉占比>50%、DEPI>1.1。\n\n3. 外部数据：审计机构变更、高管变动、诉讼担保、大股东减持、股权质押、监管问询等。'}
            />
          )}
        </h3>
      )
    },
  }), [report, selectedStock, currentSnapshot, handleRefreshInteractQA])

  return (
    <div className="app">
      {/* 设置按钮 */}
      <Settings 
        settings={settings} 
        onSettingsChange={setSettings}
        policyLibMeta={policyLibMeta}
        industryDBMeta={industryDBMeta}
        policyUpdating={policyUpdating}
        industryUpdating={industryUpdating}
        onUpdatePolicyLibrary={handleUpdatePolicyLibrary}
        onUpdateIndustryDB={handleUpdateIndustryDB}
        policyActionStatus={policyActionStatus}
        industryActionStatus={industryActionStatus}
        industryTask={industryTask}
        onCheckPythonDeps={() => setShowPythonDepsModal(true)}
      />

      {/* 左栏：自选列表 */}
      <aside className="sidebar" style={{ width: sidebarWidth, minWidth: sidebarWidth }}>
        {settings.enableHotConcepts && (
          <>
            {/* 市场热点入口：固定行，点击 → 送到中栏 */}
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '8px 0',
              marginTop: 10,
              borderBottom: '1px solid rgba(148,163,184,0.1)',
            }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ fontSize: 16 }}>🔥</span>
                <span style={{ fontWeight: 600, fontSize: 14 }}>市场热点</span>
                {hotConceptDate && (
                  <span style={{ fontSize: 11, color: '#64748b' }}>{hotConceptDate}</span>
                )}
              </div>
              <span
                style={{ fontSize: 18, color: '#3b82f6', cursor: 'pointer', padding: '0 4px' }}
                title="查看热点详情"
                onClick={() => {
                  setHotPanelOpen(true)
                  setSelectedCode(null)
                  setSelectedHotConceptCode(null)
                  setReport(null)
                  setViewingHistory(null)
                  setHistoryContent('')
                  if (!hotConceptLoading) {
                    loadHotConcepts()
                  }
                }}
              >
                →
              </span>
            </div>
            {hotConceptError && (
              <div style={{ fontSize: 12, color: '#ef4444', padding: '4px 0' }}>{hotConceptError}</div>
            )}
          </>
        )}

        <div className="sidebar-header">
          <h2>自选股票</h2>
          <div className="search-box">
            <input
              ref={inputRef}
              type="text"
              placeholder="输入代码或名称..."
              value={query}
              disabled={loading}
              onChange={(e) => {
                setQuery(e.target.value)
                setShowDropdown(true)
              }}
              onFocus={() => setShowDropdown(true)}
              onKeyDown={(e) => {
                if (e.key === 'Escape') {
                  setShowDropdown(false)
                }
              }}
            />
            {showDropdown && suggestions.length > 0 && (
              <ul className="dropdown">
                {suggestions.map((s) => (
                  <li
                    key={s.code}
                    onClick={() => handleSelectSuggestion(s)}
                    className="dropdown-item"
                  >
                    <span className="stock-code">{s.code}</span>
                    <span className="stock-name">{s.name}</span>
                  </li>
                ))}
              </ul>
            )}
          </div>
        </div>

        {(() => {
          const industries = Array.from(
            new Set(Object.values(filterData).map((d) => d.industry).filter(Boolean))
          ).sort()
          const filterButtons: { key: typeof watchlistFilter; label: string }[] = [
            { key: 'none', label: '全部' },
            { key: 'highReturn', label: '高回报' },
            { key: 'lowRisk', label: '低风险' },
            { key: 'hasData', label: '有财报' },
            { key: 'noData', label: '未下载' },
            { key: 'analyzed', label: '已分析' },
            { key: 'unanalyzed', label: '未分析' },
          ]
          const activeFilterLabel = filterButtons.find((b) => b.key === watchlistFilter)?.label
          const hasFilter = watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部'
          let title = '🔍 筛选器'
          if (hasFilter) {
            const parts: string[] = []
            if (watchlistFilter !== 'none') parts.push(activeFilterLabel!)
            if (watchlistIndustryFilter !== '全部') parts.push(watchlistIndustryFilter)
            title += ` · ${parts.join(' · ')} (${displayWatchlist.length}/${watchlist.length}只)`
          }
          return (
            <Collapsible title={title} defaultExpanded={false}>
              <div className="watchlist-filters" style={{ padding: '8px 0 4px' }}>
                <div style={{ display: 'flex', gap: '3px', flexWrap: 'wrap', alignItems: 'center', marginBottom: '6px' }}>
                  {filterButtons.map((btn) => (
                    <button
                      key={btn.key}
                      onClick={() => setWatchlistFilter(btn.key)}
                      style={{
                        padding: '2px 5px',
                        fontSize: '11px',
                        borderRadius: '4px',
                        border: '1px solid ' + (watchlistFilter === btn.key ? '#3b82f6' : 'rgba(148,163,184,0.3)'),
                        background: watchlistFilter === btn.key ? '#3b82f6' : 'transparent',
                        color: watchlistFilter === btn.key ? '#fff' : '#94a3b8',
                        cursor: 'pointer',
                        lineHeight: 1.4,
                      }}
                    >
                      {btn.label}
                    </button>
                  ))}
                  {industries.length > 0 && (
                    <select
                      value={watchlistIndustryFilter}
                      onChange={(e) => setWatchlistIndustryFilter(e.target.value)}
                      style={{
                        padding: '3px 6px',
                        fontSize: '12px',
                        borderRadius: '4px',
                        border: '1px solid rgba(148,163,184,0.3)',
                        background: 'transparent',
                        color: '#94a3b8',
                        marginLeft: '4px',
                      }}
                    >
                      <option value="全部">全部行业</option>
                      {industries.map((ind) => (
                        <option key={ind} value={ind}>{ind}</option>
                      ))}
                    </select>
                  )}
                </div>
                <div style={{ fontSize: '11px', color: '#64748b' }}>
                  显示 {displayWatchlist.length} / {watchlist.length} 只
                  {hasFilter && (
                    <button
                      onClick={() => { setWatchlistFilter('none'); setWatchlistIndustryFilter('全部') }}
                      style={{
                        marginLeft: '8px',
                        fontSize: '11px',
                        color: '#3b82f6',
                        background: 'none',
                        border: 'none',
                        cursor: 'pointer',
                        padding: 0,
                      }}
                    >
                      清除筛选
                    </button>
                  )}
                </div>
              </div>
            </Collapsible>
          )
        })()}

        <div className="watchlist-header">
          <span className="watch-header-name">股票名称</span>
          <span
            className="watch-header-activity"
            title="点击排序"
            onClick={() => {
              setActivitySort((prev) => {
                if (prev === 'none') return 'desc'
                if (prev === 'desc') return 'asc'
                return 'none'
              })
            }}
          >
            热度
            {activitySort === 'desc' && ' ▼'}
            {activitySort === 'asc' && ' ▲'}
            {activitySort === 'none' && ' ⇅'}
          </span>
          <span className="watch-header-action" />
        </div>
        <ul className="watchlist" ref={watchlistRef}>
          {displayWatchlist.map((s, idx) => {
            const act = activityMap[s.code]
            const scoreText = act ? Math.round(act.score).toString() : '-'
            return (
              <li
                key={s.code}
                data-code={s.code}
                className={`${selectedCode === s.code ? 'active' : ''}${flashCode === s.code ? ' flash-match' : ''}${draggingIndex === idx ? ' drag-placeholder' : ''}${draggingIndex !== null && dragOverIndex === idx ? ' drag-indicator-active' : ''}`}
                onClick={() => {
                  setSelectedCode(s.code)
                  setHotPanelOpen(false)
                  setSelectedHotConceptCode(null)
                  setProfile(null)
                  setQuote(null)
                  setQuoteError('')
                  // setKlines([])
                  // setKlineError('')
                  setImportResult(null)
                  setDownloadResult(null)
                  setDownloadSuggestion('')
                  setReport(null)
                  setViewingHistory(null)
                  setHistoryContent('')
                  setComparables([])
                  setAppliedComparables([])
                  setCompRecommendations([])
                  setDataHistory([])
                  setDataMissing(false)
                  loadReportHistory(s.code, true)
                  loadDataHistory(s.code)
                  loadProfile(s.code).then((p) => loadRiskRadar(s.code, p?.industry || ''))
                  loadConcepts(s.code)
                  loadMoneyflow(s.code)
                  loadComparables(s.code)
                  loadQuote(s.code)
                  // loadKlines(s.code)
                }}
              >
                <span
                  className="watch-drag-handle"
                  title={activitySort === 'none' && watchlistFilter === 'none' && watchlistIndustryFilter === '全部' ? '拖动排序' : '排序中禁用拖动'}
                  onMouseDown={(e) => {
                    if (activitySort !== 'none' || watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部') return
                    if (e.button !== 0) return
                    e.preventDefault()
                    e.stopPropagation()

                    const li = (e.target as HTMLElement).closest('li') as HTMLElement
                    if (!li) return
                    const rect = li.getBoundingClientRect()
                    const offsetX = e.clientX - rect.left
                    const offsetY = e.clientY - rect.top

                    dragIndexRef.current = idx
                    originalOrderRef.current = [...displayWatchlist]
                    setDraggingIndex(idx)
                    setDragOverIndex(idx)

                    // 创建幽灵元素
                    const ghost = li.cloneNode(true) as HTMLDivElement
                    ghost.style.position = 'fixed'
                    ghost.style.left = rect.left + 'px'
                    ghost.style.top = rect.top + 'px'
                    ghost.style.width = rect.width + 'px'
                    ghost.style.height = rect.height + 'px'
                    ghost.style.zIndex = '1000'
                    ghost.style.pointerEvents = 'none'
                    ghost.style.opacity = '0.9'
                    ghost.style.boxShadow = '0 4px 12px rgba(0,0,0,0.3)'
                    ghost.style.transform = 'scale(1.02)'
                    ghost.style.transition = 'none'
                    ghost.classList.add('drag-ghost')
                    document.body.appendChild(ghost)
                    ghostElRef.current = ghost

                    const onMouseMove = (moveEvent: MouseEvent) => {
                      // 移动幽灵
                      if (ghostElRef.current) {
                        ghostElRef.current.style.left = (moveEvent.clientX - offsetX) + 'px'
                        ghostElRef.current.style.top = (moveEvent.clientY - offsetY) + 'px'
                      }

                      // 计算插入位置
                      const list = watchlistRef.current
                      if (!list) return
                      const items = Array.from(list.querySelectorAll('li:not(.watchlist-empty)')) as HTMLElement[]
                      let closestIdx = items.length
                      for (let i = 0; i < items.length; i++) {
                        const item = items[i]
                        const r = item.getBoundingClientRect()
                        const midY = r.top + r.height / 2
                        if (moveEvent.clientY < midY) {
                          closestIdx = i
                          break
                        }
                      }
                      if (closestIdx !== dragOverIndexRef.current) {
                        dragOverIndexRef.current = closestIdx
                        setDragOverIndex(closestIdx)
                      }
                    }

                    const onMouseUp = (upEvent: MouseEvent) => {
                      window.removeEventListener('mousemove', onMouseMove)
                      window.removeEventListener('mouseup', onMouseUp)

                      // 移除幽灵
                      if (ghostElRef.current) {
                        document.body.removeChild(ghostElRef.current)
                        ghostElRef.current = null
                      }

                      const list = watchlistRef.current
                      if (list) {
                        const rect = list.getBoundingClientRect()
                        const inside = upEvent.clientX >= rect.left && upEvent.clientX <= rect.right &&
                                       upEvent.clientY >= rect.top && upEvent.clientY <= rect.bottom
                        if (inside) {
                          const fromIdx = dragIndexRef.current!
                          const toIdx = dragOverIndexRef.current ?? fromIdx
                          if (fromIdx !== toIdx) {
                            const newList = [...originalOrderRef.current]
                            const [moved] = newList.splice(fromIdx, 1)
                            // 如果 toIdx > fromIdx，由于 splice 后索引变化，需要调整
                            const insertIdx = toIdx > fromIdx ? toIdx - 1 : toIdx
                            newList.splice(insertIdx, 0, moved)
                            setWatchlist(newList)
                            const codes = newList.map((i) => i.code)
                            ReorderWatchlist(codes).catch((err) => console.error('排序保存失败:', err))
                          }
                        }
                      }
                      setDraggingIndex(null)
                      setDragOverIndex(null)
                      dragIndexRef.current = null
                      dragOverIndexRef.current = null
                    }

                    window.addEventListener('mousemove', onMouseMove)
                    window.addEventListener('mouseup', onMouseUp)
                  }}
                >☰</span>
                <div className="watch-info" title={`${s.name}(${s.code})`}>
                  {s.name}<span className="code-part">({s.code})</span>
                </div>
                {snapshots[s.code]?.riskAlert && snapshots[s.code].riskAlert!.level !== 'low' && (
                  <RiskBadge level={snapshots[s.code].riskAlert!.level} size="small" />
                )}
                <div className="watch-activity" title={act ? `${act.grade} · ${Math.round(act.score)}分` : ''}>
                  {scoreText}
                </div>
                <button
                  className="btn-remove"
                  title="移除"
                  onClick={(e) => handleRemove(s.code, e)}
                  disabled={loading}
                >
                  ×
                </button>
              </li>
            )
          })}
          {displayWatchlist.length === 0 && (
            <li className="watchlist-empty" style={{ padding: '24px 12px', textAlign: 'center', color: '#64748b', fontSize: '13px', listStyle: 'none' }}>
              {watchlist.length === 0 ? (
                <>
                  <div style={{ marginBottom: '8px', fontSize: '16px' }}>🔍</div>
                  <div>自选列表为空</div>
                  <div style={{ marginTop: '4px', fontSize: '12px', opacity: 0.8 }}>在上方搜索框输入代码或名称添加股票</div>
                </>
              ) : (
                <>
                  <div style={{ marginBottom: '8px', fontSize: '16px' }}>🍃</div>
                  <div>没有符合条件的股票</div>
                  <div style={{ marginTop: '4px', fontSize: '12px', opacity: 0.8 }}>尝试调整筛选条件</div>
                </>
              )}
            </li>
          )}
        </ul>

        <div className="watchlist-footer">
          {(watchlistFilter !== 'none' || watchlistIndustryFilter !== '全部')
            ? `显示 ${displayWatchlist.length} 只（全部 ${watchlist.length} / 100）`
            : `共 ${watchlist.length} / 100 只`}
        </div>
        <div
          className="sidebar-resizer"
          onMouseDown={() => setIsResizing(true)}
          title="拖动调整宽度"
        />
      </aside>

      {/* 中栏：股票信息 & 操作 */}
      <section className="info-panel">
        {hotPanelOpen ? (
          <div style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0, overflow: 'hidden', padding: '10px 0 16px' }}>
            {/* 热点详情面板 */}
            <div className="stock-header" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexShrink: 0, padding: '8px 16px' }}>
              <span
                style={{ fontSize: 18, color: '#3b82f6', cursor: 'pointer', padding: '0 4px' }}
                title="返回自选股票"
                onClick={() => {
                  setHotPanelOpen(false)
                  setSelectedHotConceptCode(null)
                }}
              >
                ←
              </span>
              <span style={{ fontSize: 15, fontWeight: 600, color: 'var(--text-primary)' }}>热点概念</span>
              <button
                className="btn-text"
                onClick={loadHotConcepts}
                disabled={hotConceptLoading}
              >
                {hotConceptLoading ? '刷新中...' : '刷新'} 🔄
              </button>
            </div>

            {hotConceptError && (
              <div style={{ padding: '8px 16px', color: '#ef4444', fontSize: 13, flexShrink: 0 }}>{hotConceptError}</div>
            )}

            {hotConcepts.length > 0 ? (
              <div style={{ overflowY: 'auto', flex: 1, minHeight: 0 }}>
                {/* 表头 */}
                <div style={{
                  display: 'grid',
                  gridTemplateColumns: '1fr 70px 70px 50px',
                  gap: 4,
                  padding: '6px 16px',
                  fontSize: 11,
                  color: 'var(--text-muted)',
                  borderBottom: '1px solid rgba(148,163,184,0.15)',
                  position: 'sticky',
                  top: 0,
                  background: 'var(--panel-bg)',
                  zIndex: 1,
                  whiteSpace: 'nowrap',
                }}>
                  <span>成交额(亿)</span>
                  <span style={{ textAlign: 'right' }}>主力(亿)</span>
                  <span style={{ textAlign: 'right' }}>涨幅</span>
                  <span style={{ textAlign: 'right' }}>得分</span>
                </div>
                {[...hotConcepts].sort((a, b) => {
                  return (b.score || 0) - (a.score || 0)
                }).map((c, idx) => {
                  const consecutiveDays = Object.values(hotConceptHistory).filter(
                    (names) => names.includes(c.name)
                  ).length
                  const isActive = selectedHotConceptCode === c.code
                  // 热度🔥分级
                  let fireCount = 1
                  if (c.score >= 85 || consecutiveDays >= 3) fireCount = 4
                  else if (c.score >= 70 || consecutiveDays >= 2) fireCount = 3
                  else if (c.score >= 50) fireCount = 2
                  const fires = '🔥'.repeat(fireCount)
                  return (
                    <div
                      key={c.code}
                      onClick={() => {
                        setSelectedHotConceptCode(c.code)
                        if (!conceptConstituents[c.code]) {
                          loadConceptConstituents(c.code, c.name)
                        }
                      }}
                      style={{
                        padding: '6px 16px',
                        borderBottom: '1px solid rgba(148,163,184,0.06)',
                        background: isActive ? 'rgba(59,130,246,0.08)' : 'transparent',
                        borderRadius: isActive ? 4 : 0,
                        cursor: 'pointer',
                        fontSize: 12,
                      }}
                    >
                      {/* 第一行：排名 + 名称 + 🔥 */}
                      <div style={{ display: 'flex', alignItems: 'center', gap: 5, overflow: 'hidden', marginBottom: 2 }}>
                        <span style={{ color: '#64748b', fontWeight: 600, fontSize: 11, minWidth: 18 }}>{idx + 1}</span>
                        <span style={{ fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{c.name}</span>
                        <span style={{ fontSize: 10 }} title={`热度${fireCount}/4 · 连续${consecutiveDays}天上榜 · 得分${c.score?.toFixed(1)}`}>{fires}</span>
                      </div>
                      {/* 第二行：数据 */}
                      <div style={{
                        display: 'grid',
                        gridTemplateColumns: '1fr 70px 70px 50px',
                        gap: 4,
                        alignItems: 'center',
                        color: '#64748b',
                        fontSize: 11,
                      }}>
                        <span>{(c.turnover / 1e8).toFixed(2)}</span>
                        <span style={{
                          textAlign: 'right',
                          color: (conceptMainInflowSum[c.code] !== undefined ? conceptMainInflowSum[c.code] : c.main_inflow) > 0 ? '#ef4444' : (conceptMainInflowSum[c.code] !== undefined ? conceptMainInflowSum[c.code] : c.main_inflow) < 0 ? '#22c55e' : '#64748b',
                        }}>
                          {((conceptMainInflowSum[c.code] !== undefined ? conceptMainInflowSum[c.code] : c.main_inflow) / 1e8).toFixed(2)}
                        </span>
                        <span style={{
                          textAlign: 'right',
                          fontWeight: 600,
                          color: c.change_pct > 0 ? '#ef4444' : c.change_pct < 0 ? '#22c55e' : '#94a3b8',
                        }}>
                          {c.change_pct > 0 ? '+' : ''}{c.change_pct?.toFixed(2)}%
                        </span>
                        <span style={{ textAlign: 'right', fontWeight: 600, color: '#f59e0b' }}>{c.score?.toFixed(1)}</span>
                      </div>
                    </div>
                  )
                })}
              </div>
            ) : (
              <div style={{ padding: '40px 0', textAlign: 'center', color: '#64748b', fontSize: 13 }}>
                暂无热点数据，点击"刷新"获取
              </div>
            )}
          </div>
        ) : selectedStock ? (
          <>
            <div className="stock-header">
              <h1 style={{ fontSize: 14, gap: 6, flex: 1, minWidth: 0, overflow: 'hidden' }}>
                <span style={{ whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{selectedStock.name}</span>
                <span className="stock-sub" style={{ fontSize: 11, whiteSpace: 'nowrap', flexShrink: 0 }}>{selectedStock.code}</span>
              </h1>

            </div>

            <div className="info-panel-scroll">
            <div className="stock-info-card" style={{ position: 'relative', marginBottom: 4 }}>
              <button
                className="stock-info-refresh"
                onClick={handleRefreshProfile}
                title="刷新股票基本信息（行业/PE/PB 等）"
                style={{ position: 'absolute', top: 4, right: 6, whiteSpace: 'nowrap', zIndex: 1 }}
              >
                刷新
              </button>
              <div className="stock-info-grid">
                <div className="stock-info-item">
                  <span className="stock-info-label">所属行业</span>
                  <span className="stock-info-value">{profile?.industry || '--'}</span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">上市日期</span>
                  <span className="stock-info-value">{profile?.listingDate || '--'}</span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">
                    {profile?.controller ? (
                      <><strong>实控人</strong>/董事长</>
                    ) : profile?.chairman ? (
                      <>实控人/<strong>董事长</strong></>
                    ) : (
                      '实控人/董事长'
                    )}
                  </span>
                  <span className="stock-info-value">
                    {profile?.controller || profile?.chairman || '--'}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">籍属</span>
                  <span className="stock-info-value">
                    {profile?.chairmanNationality ? (
                      profile.chairmanNationality === '中国台湾' && profile?.politicalAffiliation ? (
                        <strong
                          style={{
                            color: profile.politicalAffiliation === 'blue' ? '#3b82f6' : '#22c55e',
                          }}
                        >
                          {profile.chairmanNationality}
                        </strong>
                      ) : (
                        profile.chairmanNationality
                      )
                    ) : (
                      '--'
                    )}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">总市值</span>
                  <span className="stock-info-value">
                    {(profile?.marketCap || quote?.marketCap)
                      ? `${(((profile?.marketCap || 0) > 0 ? profile!.marketCap : quote!.marketCap) / 1e8).toFixed(2)} 亿`
                      : '--'}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">市盈率 (PE)</span>
                  <span className="stock-info-value">
                    {profile?.pe
                      ? profile.pe.toFixed(2)
                      : quote?.pe
                      ? quote.pe.toFixed(2)
                      : '--'}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">市净率 (PB)</span>
                  <span className="stock-info-value">
                    {profile?.pb
                      ? profile.pb.toFixed(2)
                      : quote?.pb
                      ? quote.pb.toFixed(2)
                      : '--'}
                  </span>
                </div>
                <div className="stock-info-item">
                  <span className="stock-info-label">股东回报率</span>
                  {(() => {
                    const rate = quote?.shareholderReturnRate
                    if (rate == null || rate <= 0) {
                      return <span className="stock-info-value">--</span>
                    }
                    let color = '#94a3b8'
                    if (rate > 0.10) color = '#22c55e'
                    else if (rate >= 0.06) color = '#eab308'
                    const dy = quote?.dividendYield || 0
                    const ey = rate - dy
                    const tooltip = `股东回报率 ≈ 盈利收益率(ROE/PB) + 股息率\n当前: ${(ey * 100).toFixed(2)}% + ${(dy * 100).toFixed(2)}% = ${(rate * 100).toFixed(2)}%\n假设公司维持当前盈利能力且估值不变，该数字可视为股东每年的名义总回报。`
                    return (
                      <span
                        className="stock-info-value"
                        style={{ color, cursor: 'help' }}
                        title={tooltip}
                      >
                        {(rate * 100).toFixed(2)}%
                      </span>
                    )
                  })()}
                </div>
              </div>
            </div>
            {/* 近3日资金流向（独立卡片，与基本信息紧凑挨着）；港股暂无真实资金流向数据，直接隐藏 */}
            {!selectedStock?.code?.endsWith('.HK') && ((moneyflow?.has_data && moneyflow.items && moneyflow.items.length > 0) || (moneyflow && !moneyflow.has_data && moneyflow.summary)) ? (
            <div className="stock-info-card">
              {moneyflow?.has_data && moneyflow.items && moneyflow.items.length > 0 ? (
                <div>
                  <div
                    onClick={() => setRecentMoneyflowExpanded(v => !v)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'space-between',
                      cursor: 'pointer',
                      fontSize: 11,
                      fontWeight: 600,
                      color: '#64748b',
                      userSelect: 'none',
                      marginBottom: recentMoneyflowExpanded ? 4 : 0,
                    }}
                  >
                    <span style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                      近{moneyflow.days || sflConfig?.moneyflow_days || 3}个交易日资金流向（亿元）
                      <span
                        title="主力 = 超大单（>100万）+ 大单（20~100万），按单笔成交金额分档统计，机构可通过拆单规避"
                        onClick={(e) => e.stopPropagation()}
                        style={{ cursor: 'help', fontSize: 12, opacity: 0.5 }}
                      >ⓘ</span>
                    </span>
                    <span
                      style={{
                        fontSize: 12,
                        color: '#64748b',
                        display: 'inline-block',
                        transition: 'transform 0.2s ease',
                        transform: recentMoneyflowExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
                      }}
                    >
                      ›
                    </span>
                  </div>
                  {recentMoneyflowExpanded && (
                    <>
                  {/* 表头 */}
                  <div style={{
                    display: 'grid',
                    gridTemplateColumns: '32px 52px 52px 50px 50px',
                    gap: 1,
                    fontSize: 11,
                    color: '#94a3b8',
                    paddingBottom: 3,
                    borderBottom: '1px solid rgba(148,163,184,0.08)',
                    marginBottom: 3,
                  }}>
                    <span style={{ textAlign: 'left' }}>日期</span>
                    <span style={{ textAlign: 'right' }}>超大</span>
                    <span style={{ textAlign: 'right' }}>大</span>
                    <span style={{ textAlign: 'right' }}>中</span>
                    <span style={{ textAlign: 'right' }}>小</span>
                  </div>
                  {/* 数据行 */}
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                    {moneyflow.items.slice(0, moneyflow.days || sflConfig?.moneyflow_days || 3).map((item, idx) => (
                      <div key={idx} style={{
                        display: 'grid',
                        gridTemplateColumns: '32px 52px 52px 50px 50px',
                        gap: 1,
                        fontSize: 11,
                        alignItems: 'center',
                      }}>
                        <span style={{ color: '#64748b', fontFamily: 'monospace', textAlign: 'left', whiteSpace: 'nowrap' }}>
                          {item.date?.slice(4, 6)}/{item.date?.slice(6)}
                        </span>
                        <span style={{
                          color: item.elg_net_amount > 0 ? '#ef4444' : item.elg_net_amount < 0 ? '#22c55e' : '#94a3b8',
                          textAlign: 'right',
                          whiteSpace: 'nowrap',
                        }}>
                          {item.elg_net_amount > 0 ? '+' : ''}{(item.elg_net_amount/1e8).toFixed(2)}
                        </span>
                        <span style={{
                          color: item.lg_net_amount > 0 ? '#ef4444' : item.lg_net_amount < 0 ? '#22c55e' : '#94a3b8',
                          textAlign: 'right',
                          whiteSpace: 'nowrap',
                        }}>
                          {item.lg_net_amount > 0 ? '+' : ''}{(item.lg_net_amount/1e8).toFixed(2)}
                        </span>
                        <span style={{
                          color: item.md_net_amount > 0 ? '#ef4444' : item.md_net_amount < 0 ? '#22c55e' : '#94a3b8',
                          textAlign: 'right',
                          whiteSpace: 'nowrap',
                        }}>
                          {item.md_net_amount > 0 ? '+' : ''}{(item.md_net_amount/1e8).toFixed(2)}
                        </span>
                        <span style={{
                          color: item.sm_net_amount > 0 ? '#ef4444' : item.sm_net_amount < 0 ? '#22c55e' : '#94a3b8',
                          textAlign: 'right',
                          whiteSpace: 'nowrap',
                        }}>
                          {item.sm_net_amount > 0 ? '+' : ''}{(item.sm_net_amount/1e8).toFixed(2)}
                        </span>
                      </div>
                    ))}
                  </div>
                  {moneyflow.summary && (
                    <div style={{ fontSize: 11, color: '#64748b', marginTop: 5, paddingTop: 4, borderTop: '1px solid rgba(148,163,184,0.06)' }}>
                      {moneyflow.summary}
                    </div>
                  )}
                    </>
                  )}

                  {/* 当日流向（展开/收起） */}
                  <div style={{ marginTop: 6, borderTop: '1px solid rgba(148,163,184,0.06)', paddingTop: 6 }}>
                    <div
                      onClick={() => setTodayMoneyflowExpanded(v => !v)}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        cursor: 'pointer',
                        fontSize: 11,
                        fontWeight: 600,
                        color: '#64748b',
                        userSelect: 'none',
                      }}
                    >
                      <span>当日流向</span>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                        <span
                          style={{
                            fontSize: 12,
                            color: '#64748b',
                            display: 'inline-block',
                            transition: 'transform 0.2s ease',
                            transform: todayMoneyflowExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
                          }}
                        >
                          ›
                        </span>
                      </div>
                    </div>
                    {todayMoneyflowExpanded && (
                      <div style={{ marginTop: 5 }}>
                        {moneyflow.today_item ? (
                          <>
                            <div style={{
                              display: 'grid',
                              gridTemplateColumns: '32px 52px 52px 50px 50px',
                              gap: 1,
                              fontSize: 11,
                              color: '#94a3b8',
                              paddingBottom: 3,
                              borderBottom: '1px solid rgba(148,163,184,0.08)',
                              marginBottom: 3,
                            }}>
                              <span style={{ textAlign: 'left' }}>日期</span>
                              <span style={{ textAlign: 'right' }}>超大</span>
                              <span style={{ textAlign: 'right' }}>大</span>
                              <span style={{ textAlign: 'right' }}>中</span>
                              <span style={{ textAlign: 'right' }}>小</span>
                            </div>
                            <div style={{
                              display: 'grid',
                              gridTemplateColumns: '32px 52px 52px 50px 50px',
                              gap: 1,
                              fontSize: 11,
                              alignItems: 'center',
                            }}>
                              <span style={{ color: '#64748b', fontFamily: 'monospace', textAlign: 'left', whiteSpace: 'nowrap' }}>
                                {moneyflow.today_item.date?.slice(4, 6)}/{moneyflow.today_item.date?.slice(6)}
                              </span>
                              <span style={{
                                color: moneyflow.today_item.elg_net_amount > 0 ? '#ef4444' : moneyflow.today_item.elg_net_amount < 0 ? '#22c55e' : '#94a3b8',
                                textAlign: 'right', whiteSpace: 'nowrap',
                              }}>
                                {moneyflow.today_item.elg_net_amount > 0 ? '+' : ''}{(moneyflow.today_item.elg_net_amount/1e8).toFixed(2)}
                              </span>
                              <span style={{
                                color: moneyflow.today_item.lg_net_amount > 0 ? '#ef4444' : moneyflow.today_item.lg_net_amount < 0 ? '#22c55e' : '#94a3b8',
                                textAlign: 'right', whiteSpace: 'nowrap',
                              }}>
                                {moneyflow.today_item.lg_net_amount > 0 ? '+' : ''}{(moneyflow.today_item.lg_net_amount/1e8).toFixed(2)}
                              </span>
                              <span style={{
                                color: moneyflow.today_item.md_net_amount > 0 ? '#ef4444' : moneyflow.today_item.md_net_amount < 0 ? '#22c55e' : '#94a3b8',
                                textAlign: 'right', whiteSpace: 'nowrap',
                              }}>
                                {moneyflow.today_item.md_net_amount > 0 ? '+' : ''}{(moneyflow.today_item.md_net_amount/1e8).toFixed(2)}
                              </span>
                              <span style={{
                                color: moneyflow.today_item.sm_net_amount > 0 ? '#ef4444' : moneyflow.today_item.sm_net_amount < 0 ? '#22c55e' : '#94a3b8',
                                textAlign: 'right', whiteSpace: 'nowrap',
                              }}>
                                {moneyflow.today_item.sm_net_amount > 0 ? '+' : ''}{(moneyflow.today_item.sm_net_amount/1e8).toFixed(2)}
                              </span>
                            </div>
                            {moneyflow.today_item.main_inflow !== 0 && (
                              <div style={{ fontSize: 11, color: '#64748b', marginTop: 4 }}>
                                主力净流入{' '}
                                <span style={{ color: moneyflow.today_item.main_inflow > 0 ? '#ef4444' : '#22c55e' }}>
                                  {moneyflow.today_item.main_inflow > 0 ? '+' : ''}{(moneyflow.today_item.main_inflow/1e8).toFixed(2)} 亿元
                                </span>
                              </div>
                            )}
                          </>
                        ) : (
                          <div style={{ fontSize: 11, color: '#94a3b8', padding: '4px 0' }}>
                            当日数据暂未更新
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              ) : moneyflow && !moneyflow.has_data && moneyflow.summary ? (
                <div>
                  <div style={{ fontSize: 11, fontWeight: 600, color: '#64748b', marginBottom: 4 }}>近{sflConfig?.moneyflow_days || 3}个交易日资金流向（亿元）</div>
                  <div style={{ fontSize: 11, color: '#94a3b8' }}>{moneyflow.summary}</div>
                </div>
              ) : null}
            </div>
            ) : null}

            {/* K线 / 融资融券 / 财务趋势 快捷按钮（独立于资金流向卡片，港股也能使用 K线/财务趋势） */}
            {selectedStock && (
              <div style={{ display: 'flex', gap: 6, marginTop: 8, flexWrap: 'wrap' }}>
                <button
                  className="btn-text"
                  onClick={() => setKlineFullscreen(true)}
                  disabled={!selectedStock}
                  style={{
                    fontSize: 10,
                    color: '#ef4444',
                    padding: '2px 6px',
                    borderRadius: 3,
                    border: '1px solid rgba(239,68,68,0.4)',
                    background: 'rgba(239,68,68,0.08)',
                    cursor: selectedStock ? 'pointer' : 'not-allowed',
                    whiteSpace: 'nowrap',
                  }}
                  title="全窗口查看 K线 + 技术指标联动分析图"
                >
                  K线
                </button>
                <button
                  className="btn-text"
                  onClick={() => selectedStock && setMarginDrawerCode(selectedStock.code)}
                  disabled={!selectedStock || selectedStock.code.endsWith('.HK') || marginTargetMap[selectedStock.code] !== true}
                  style={{
                    fontSize: 10,
                    color: !selectedStock || selectedStock.code.endsWith('.HK') || marginTargetMap[selectedStock.code] !== true ? '#64748b' : '#f59e0b',
                    padding: '2px 6px',
                    borderRadius: 3,
                    border: !selectedStock || selectedStock.code.endsWith('.HK') || marginTargetMap[selectedStock.code] !== true
                      ? '1px solid rgba(100,116,139,0.3)'
                      : '1px solid rgba(245,158,11,0.4)',
                    background: !selectedStock || selectedStock.code.endsWith('.HK') || marginTargetMap[selectedStock.code] !== true
                      ? 'rgba(100,116,139,0.06)'
                      : 'rgba(245,158,11,0.08)',
                    cursor: !selectedStock || selectedStock.code.endsWith('.HK') || marginTargetMap[selectedStock.code] !== true ? 'not-allowed' : 'pointer',
                    whiteSpace: 'nowrap',
                    opacity: !selectedStock || selectedStock.code.endsWith('.HK') || marginTargetMap[selectedStock.code] !== true ? 0.7 : 1,
                  }}
                  title={
                    !selectedStock
                      ? '请先选择股票'
                      : selectedStock.code.endsWith('.HK')
                        ? '港股暂不支持融资融券'
                        : marginTargetMap[selectedStock.code] === false
                          ? '该股票不是融资融券标的'
                          : marginTargetMap[selectedStock.code] === undefined
                            ? '正在检查融资融券标的...'
                            : '查看融资融券与股价叠加图'
                  }
                >
                  融资融券
                </button>
                <button
                  className="btn-text"
                  onClick={() => setTrendDrawerCode(selectedStock!.code)}
                  style={{
                    fontSize: 10,
                    color: '#10b981',
                    padding: '2px 6px',
                    borderRadius: 3,
                    border: '1px solid rgba(16,185,129,0.4)',
                    background: 'rgba(16,185,129,0.08)',
                    cursor: 'pointer',
                    whiteSpace: 'nowrap',
                  }}
                  title="查看近5年财务指标趋势"
                >
                  财务趋势
                </button>
              </div>
            )}

            {/* 导入/导出操作区 */}
            <div className="actions-sub" style={{ display: 'flex', marginBottom: 10, gap: 16, justifyContent: 'center' }}>
              <button className="btn-text" style={{ flex: '1 1 0', textAlign: 'center', whiteSpace: 'nowrap', fontSize: 11 }} onClick={handleImport} disabled={loading}>
                {loading ? '处理中...' : '导入csv/excel财报'}
              </button>
              <button className="btn-text" style={{ flex: '1 1 0', textAlign: 'center', whiteSpace: 'nowrap', fontSize: 11 }} onClick={handleExportCurrentData} disabled={!selectedStock || dataHistory.length === 0} title={dataHistory.length === 0 ? '请先下载或导入财报数据' : '导出当前财务数据到本地'}>
                导出本地财报
              </button>
            </div>

            {/* 主操作按钮 */}
            <div className="actions">
              <button className="btn primary" onClick={handleDownload} disabled={downloading || loading}>
                财报下载
              </button>
              <button className="btn primary" onClick={handleAnalyze} disabled={analyzing || downloading || loading || dataHistory.length === 0 || dataMissing} title={dataHistory.length === 0 || dataMissing ? '请先下载或导入财报数据' : ''}>
                财报透镜
              </button>
            </div>

            {/* 状态显示 - 按钮下方一行 */}
            <div className="action-status-line">
              {downloading && <span className="status-downloading">正在下载...</span>}
              {downloadStatus.type === 'success' && !downloading && (
                <span className="status-success" title={downloadStatus.message}>{downloadStatus.message}</span>
              )}
              {downloadStatus.type === 'error' && !downloading && (
                <span className="status-error" title={downloadStatus.message}>{downloadStatus.message.length > 30 ? downloadStatus.message.slice(0, 30) + '...' : downloadStatus.message}</span>
              )}
              {analyzing && (
                <span className="status-analyzing">
                  分析中 {analyzeProgress}%（{getAnalyzeStageText(analyzeProgress)}）
                </span>
              )}
            </div>

            {downloadSuggestion && (
              <div style={{
                marginTop: 8,
                padding: '8px 12px',
                background: 'rgba(59, 130, 246, 0.12)',
                borderLeft: '3px solid #3b82f6',
                borderRadius: '0 6px 6px 0',
                fontSize: 12,
                color: '#60a5fa',
                lineHeight: 1.5,
              }}>
                💡 {downloadSuggestion}
              </div>
            )}

            {importResult && importResult.success && (
              <Collapsible title="📥 导入结果">
                <div className="import-summary" style={{ marginBottom: 0 }}>
                  <div className="import-row">
                    <span className="import-label">资产负债表</span>
                    <span className="import-years">
                      {importResult.balanceSheet?.length
                        ? `${importResult.balanceSheet.length} 年: ${importResult.balanceSheet.join(', ')}`
                        : '未导入'}
                    </span>
                  </div>
                  <div className="import-row">
                    <span className="import-label">利润表</span>
                    <span className="import-years">
                      {importResult.income?.length
                        ? `${importResult.income.length} 年: ${importResult.income.join(', ')}`
                        : '未导入'}
                    </span>
                  </div>
                  <div className="import-row">
                    <span className="import-label">现金流量表</span>
                    <span className="import-years">
                      {importResult.cashFlow?.length
                        ? `${importResult.cashFlow.length} 年: ${importResult.cashFlow.join(', ')}`
                        : '未导入'}
                    </span>
                  </div>
                </div>
              </Collapsible>
            )}

            {downloadResult && downloadResult.success && (
              <Collapsible title="⬇️ 下载结果">
                <div className="import-summary" style={{ marginBottom: 0 }}>
                  <div className="import-row">
                    <span className="import-label">
                      网络下载
                      {downloadResult.sourceName && (
                        <span style={{ fontSize: 11, color: '#94a3b8', fontWeight: 400, marginLeft: 4 }}>
                          ({downloadResult.sourceName})
                        </span>
                      )}
                    </span>
                    <span className="import-years">
                      {(() => {
                        const ys = downloadResult.years ?? []
                        const qs = downloadResult.quarters ?? []
                        if (ys.length === 0 && qs.length === 0) return '无'
                        const parts: string[] = []
                        if (ys.length > 0) parts.push(`${ys.length} 年报: ${ys.join(', ')}`)
                        if (qs.length > 0) parts.push(`${qs.length} 季报: ${qs.join(', ')}`)
                        return parts.join('；')
                      })()}
                    </span>
                  </div>
                  {downloadResult.validation && downloadResult.validation.length > 0 && (
                    <div style={{ marginTop: 8 }}>
                      <div style={{ fontWeight: 600, marginBottom: 4 }}>数据校验：</div>
                      {downloadResult.validation.map((v, idx) => (
                        <div
                          key={idx}
                          style={{
                            fontSize: 12,
                            color:
                              v.status === 'error'
                                ? '#ef4444'
                                : v.status === 'warning'
                                ? '#f59e0b'
                                : '#22c55e',
                          }}
                        >
                          {v.year} {v.indicator}: 差异 {v.diffPercent.toFixed(2)}%
                        </div>
                      ))}
                    </div>
                  )}
                  <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 10 }}>
                    <button className="btn-text" onClick={handleExportCurrentData}>
                      ⬇️ 下载到本地
                    </button>
                  </div>
                </div>
              </Collapsible>
            )}

            {(report?.riskAlert || currentSnapshot?.riskAlert) && (
              <RiskAlertBanner alert={report?.riskAlert || currentSnapshot?.riskAlert} />
            )}

            <Collapsible title="🚀 概念与风口">
              <div className="concept-panel" style={{ marginTop: 0, marginBottom: 0 }}>
                <div className="concept-wind">{concepts?.wind || '--'}</div>
                {concepts && concepts.concepts.length > 0 ? (
                  <div className="concept-tags">
                    {concepts.concepts.map((c, idx) => (
                      <span key={idx} className="concept-tag">{c}</span>
                    ))}
                  </div>
                ) : (
                  <div style={{ color: '#64748b', fontSize: 12, padding: '4px 0' }}>暂无概念数据</div>
                )}
              </div>
            </Collapsible>

            {currentSnapshot && (
              <Collapsible title="💡 亮点与风险">
                <div className="highlights-risks" style={{ marginTop: 0, marginBottom: 0 }}>
                  {highlights.length > 0 && (
                    <div className="hr-section">
                      {highlights.map((h, idx) => (
                        <div key={`h-${idx}`} className="highlight-item">
                          {h}
                        </div>
                      ))}
                    </div>
                  )}
                  {risks.length > 0 && (
                    <div className="hr-section">
                      {risks.map((r, idx) => (
                        <div key={`r-${idx}`} className="risk-item">
                          {r}
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </Collapsible>
            )}

            {/* 与上次分析对比 */}
            {(() => {
              const diff = (currentSnapshot as any)?.diff
              if (!diff || !diff.hasPrevious) return null
              return (
                <Collapsible title="📈 与上次分析对比">
                  <div style={{ marginTop: 0, marginBottom: 0, fontSize: 12 }}>
                    <div style={{ color: '#94a3b8', marginBottom: 6, fontSize: 11 }}>
                      上次分析：{diff.previousTime}
                    </div>

                    {/* 评分变化 */}
                    {diff.scoreChange !== 0 && (
                      <div style={{
                        marginBottom: 8,
                        padding: '6px 8px',
                        borderRadius: 4,
                        background: diff.scoreChange > 0 ? 'rgba(34,197,94,0.08)' : 'rgba(239,68,68,0.08)',
                        color: diff.scoreChange > 0 ? '#4ade80' : '#f87171',
                        fontWeight: 600,
                      }}>
                        综合评分 {diff.scoreChange > 0 ? '↑' : '↓'} {Math.abs(diff.scoreChange).toFixed(0)} 分
                        {diff.gradeChanged ? `（等级 ${diff.previousGrade} → ${diff.currentGrade}）` : `（等级维持 ${diff.currentGrade}）`}
                      </div>
                    )}

                    {/* 风险变化 */}
                    {(diff.newFlags?.length > 0 || diff.resolvedFlags?.length > 0) && (
                      <div style={{ marginBottom: 8 }}>
                        {diff.newFlags?.length > 0 && (
                          <div style={{ marginBottom: 4 }}>
                            <span style={{ color: '#f87171', fontWeight: 600 }}>🆕 新增风险（{diff.newFlags.length}项）</span>
                            {diff.newFlags.map((f: any, i: number) => (
                              <div key={`nf-${i}`} style={{ paddingLeft: 16, color: '#fca5a5', fontSize: 11, marginTop: 2 }}>
                                {f.level === 'high' ? '🔴' : '🟡'} {f.format}
                              </div>
                            ))}
                          </div>
                        )}
                        {diff.resolvedFlags?.length > 0 && (
                          <div>
                            <span style={{ color: '#4ade80', fontWeight: 600 }}>✅ 解除风险（{diff.resolvedFlags.length}项）</span>
                            {diff.resolvedFlags.map((f: any, i: number) => (
                              <div key={`rf-${i}`} style={{ paddingLeft: 16, color: '#86efac', fontSize: 11, marginTop: 2 }}>
                                {f.format}
                              </div>
                            ))}
                          </div>
                        )}
                      </div>
                    )}

                    {/* 关键指标变化 */}
                    {diff.keyMetricChanges?.length > 0 && (
                      <div>
                        <div style={{ color: '#cbd5e1', fontWeight: 600, marginBottom: 4 }}>关键指标变化</div>
                        <table style={{ width: '100%', fontSize: 11, borderCollapse: 'collapse' }}>
                          <thead>
                            <tr style={{ color: '#94a3b8', borderBottom: '1px solid rgba(148,163,184,0.2)' }}>
                              <th style={{ textAlign: 'left', padding: '2px 4px' }}>指标</th>
                              <th style={{ textAlign: 'right', padding: '2px 4px' }}>上次</th>
                              <th style={{ textAlign: 'right', padding: '2px 4px' }}>本次</th>
                              <th style={{ textAlign: 'right', padding: '2px 4px' }}>变化</th>
                            </tr>
                          </thead>
                          <tbody>
                            {diff.keyMetricChanges.map((m: any, i: number) => (
                              <tr key={`mc-${i}`} style={{ borderBottom: '1px solid rgba(148,163,184,0.05)' }}>
                                <td style={{ padding: '2px 4px', color: '#e2e8f0' }}>{m.name}</td>
                                <td style={{ textAlign: 'right', padding: '2px 4px', color: '#94a3b8' }}>{m.previous.toFixed(2)}</td>
                                <td style={{ textAlign: 'right', padding: '2px 4px', color: '#e2e8f0' }}>{m.current.toFixed(2)}</td>
                                <td style={{
                                  textAlign: 'right', padding: '2px 4px',
                                  color: m.delta > 0 ? '#4ade80' : m.delta < 0 ? '#f87171' : '#94a3b8',
                                  fontWeight: m.significant ? 600 : 400,
                                }}>
                                  {m.delta > 0 ? '+' : ''}{m.delta.toFixed(2)}
                                  {m.significant ? (m.delta > 0 ? ' 🟢' : ' 🔴') : ''}
                                </td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                  </div>
                </Collapsible>
              )
            })()}

            {selectedStock && (
              <Collapsible title="📊 行业对比雷达">
                <div className="risk-radar-collapsible-body" style={{ marginTop: 0, marginBottom: 0 }}>
                  {riskRadar && riskRadar.length > 0 ? (
                    <>
                      <table className="risk-radar-table">
                        <thead>
                          <tr>
                            <th style={{ width: 40, textAlign: 'center' }}>状态</th>
                            <th>指标</th>
                            <th style={{ textAlign: 'right' }}>当前值</th>
                            <th style={{ textAlign: 'right' }}>行业均值</th>
                          </tr>
                        </thead>
                        <tbody>
                          {riskRadar.map((item, idx) => (
                            <tr key={idx} className={`risk-radar-tr risk-radar-${item.level}`} title={item.desc}>
                              <td style={{ textAlign: 'center' }}>{item.icon}</td>
                              <td>{item.name}</td>
                              <td style={{ textAlign: 'right', fontWeight: 500 }}>{item.value}</td>
                              <td style={{ textAlign: 'right', color: '#94a3b8' }}>{item.industry || '-'}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                      <div className="risk-radar-hint">基于本地数据计算 · 设置中可更新</div>
                    </>
                  ) : (
                    <div className="risk-radar-empty">暂无对比数据（请先执行财报分析）</div>
                  )}
                </div>
              </Collapsible>
            )}

            <Collapsible title="🏢 可比公司">
              <div className="comparable-panel" style={{ marginTop: 0, marginBottom: 0 }}>
                <div className="cp-search">
                  <input
                    type="text"
                    placeholder="添加可比公司 (3~7家)..."
                    value={compQuery}
                    disabled={loading || comparables.length >= 7}
                    onChange={(e) => {
                      setCompQuery(e.target.value)
                      setShowCompDropdown(true)
                    }}
                    onFocus={() => setShowCompDropdown(true)}
                  />
                  {showCompDropdown && compSuggestions.length > 0 && (
                    <ul className="dropdown cp-dropdown">
                      {compSuggestions.map((s) => (
                        <li
                          key={s.code}
                          onClick={() => handleAddComparable(s)}
                          className="dropdown-item"
                        >
                          <span className="stock-code">{s.code}</span>
                          <span className="stock-name">{s.name}</span>
                        </li>
                      ))}
                    </ul>
                  )}
                </div>
                {comparables.length > 0 && (
                  <div className="cp-list">
                    {comparables.map((c) => {
                      const info = STOCKS.find((s) => s.code === c)
                      return (
                        <div key={c} className="cp-item">
                          <span className="cp-name">{info?.name || c}</span>
                          <button
                            className="cp-remove"
                            onClick={() => handleRemoveComparable(c)}
                            title="移除"
                          >
                            ×
                          </button>
                        </div>
                      )
                    })}
                  </div>
                )}

                {/* 自动推荐 */}
                <div style={{ marginTop: 8, marginBottom: 4 }}>
                  <button
                    className="btn-text"
                    onClick={handleRecommendComparables}
                    disabled={compRecommending}
                    style={{ fontSize: 11, color: '#60a5fa' }}
                  >
                    {compRecommending ? '推荐中...' : '🔍 自动推荐可比公司'}
                  </button>
                </div>

                {compRecommendations.length > 0 && (
                  <div className="cp-recommendations" style={{ marginTop: 4, marginBottom: 8 }}>
                    <div style={{ fontSize: 11, color: '#94a3b8', marginBottom: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span>推荐结果（点击添加）：</span>
                      <span style={{ display: 'flex', alignItems: 'center', gap: 2, marginRight: 6 }}>
                        <span style={{ fontSize: 11, color: '#94a3b8' }}>全添加</span>
                        <span
                          onClick={handleAddAllRecommendations}
                          style={{
                            color: (comparables.length >= 7 || compRecommendations.every(r => comparables.includes(r.symbol))) ? '#4b5563' : '#60a5fa',
                            fontSize: 16,
                            cursor: (comparables.length >= 7 || compRecommendations.every(r => comparables.includes(r.symbol))) ? 'default' : 'pointer',
                            flexShrink: 0,
                          }}
                        >+</span>
                      </span>
                    </div>
                    {compRecommendations.map((r) => {
                      const info = STOCKS.find((s) => s.code === r.symbol)
                      return (
                        <div
                          key={r.symbol}
                          className="cp-rec-item"
                          onClick={() => handleAddRecommendedComparable(r.symbol)}
                          title={r.reasons && r.reasons.length > 0 ? r.reasons.join(' · ') : undefined}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'space-between',
                            padding: '4px 6px',
                            borderRadius: 4,
                            background: 'rgba(59,130,246,0.08)',
                            marginBottom: 4,
                            cursor: 'pointer',
                            fontSize: 11,
                          }}
                        >
                          <div style={{ flex: 1, minWidth: 0 }}>
                            <div style={{ display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 4 }}>
                              <span style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{info?.name || r.name || r.symbol}</span>
                              <span style={{ color: 'var(--text-secondary, #64748b)' }}>{r.symbol}</span>
                              <span style={{ color: '#fbbf24' }}>相似度 {r.score.toFixed(0)}</span>
                              {r.dataQuality === 'high' ? (
                                <span style={{ color: '#4ade80' }}>✅</span>
                              ) : (
                                <span style={{ color: '#fbbf24' }}>⚠️</span>
                              )}
                            </div>
                          </div>
                          <span style={{ color: '#60a5fa', fontSize: 16, marginLeft: 4, flexShrink: 0 }}>+</span>
                        </div>
                      )
                    })}
                  </div>
                )}
                <div className="cp-actions">
                  <button
                    className="btn-text cp-download"
                    onClick={handleDownloadComparables}
                    disabled={compDownloading || comparables.length === 0}
                  >
                    {compDownloading ? '下载中...' : '财报下载'}
                  </button>
                  {(() => {
                    const compChanged = JSON.stringify([...appliedComparables].sort()) !== JSON.stringify([...comparables].sort())
                    const canUpdate = compReportsDownloaded && comparables.length > 0
                    return (
                      <button
                        className={`btn-icon cp-merge${compChanged ? ' changed' : ''}`}
                        title={canUpdate ? '更新模块4（行业横向对比分析）到报告' : '请先下载可比公司财报'}
                        onClick={handleAnalyzeWithComparables}
                        disabled={analyzing || !canUpdate}
                      >
                        {analyzing ? (
                          '···'
                        ) : (
                          <svg
                            width="16"
                            height="16"
                            viewBox="0 0 24 24"
                            fill="none"
                            stroke="currentColor"
                            strokeWidth="2.3"
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            style={{ display: 'block' }}
                          >
                            <rect x="3" y="6" width="14" height="12" rx="2" ry="2" />
                            <path d="M17 12l4 4-4 4" />
                            <path d="M8 6V4a2 2 0 0 1 2-2h2" />
                            <polyline points="11 3 13 1 15 3" />
                          </svg>
                        )}
                      </button>
                    )
                  })()}
                </div>
                {compDownloadStatus.type && !compDownloading && (
                  <div className="cp-status-line">
                    {compDownloadStatus.type === 'success' ? (
                      <span className="status-success" title={compDownloadStatus.message}>{compDownloadStatus.message}</span>
                    ) : (
                      <span className="status-error" title={compDownloadStatus.message}>{compDownloadStatus.message.length > 40 ? compDownloadStatus.message.slice(0, 40) + '...' : compDownloadStatus.message}</span>
                    )}
                  </div>
                )}

              </div>
            </Collapsible>

            {quote && (
              <Collapsible title="📈 实时行情">
                <div className="stock-quote" style={{ marginTop: 0, marginBottom: 0 }}>
                  <div className="sq-header">
                    <div>
                      <span className={`sq-price ${quote.changePercent >= 0 ? 'up' : 'down'}`}>
                        {quote.currentPrice.toFixed(2)}
                      </span>
                      <span className={`sq-change ${quote.changePercent >= 0 ? 'up' : 'down'}`}>
                        {quote.changePercent >= 0 ? '+' : ''}
                        {quote.changePercent.toFixed(2)}% ({quote.changeAmount >= 0 ? '+' : ''}
                        {quote.changeAmount.toFixed(2)})
                      </span>
                    </div>
                    <div className="sq-time">
                      {quote.quoteTime || ''}
                    </div>
                  </div>
                  <div className="sq-grid">
                    <div className="sq-item">
                      <span className="sq-label">今开</span>
                      <span className="sq-value">{quote.open ? quote.open.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">最高</span>
                      <span className="sq-value">{quote.high ? quote.high.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">最低</span>
                      <span className="sq-value">{quote.low ? quote.low.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">昨收</span>
                      <span className="sq-value">{quote.previousClose ? quote.previousClose.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">换手率</span>
                      <span className="sq-value">{quote.turnoverRate ? `${quote.turnoverRate.toFixed(2)}%` : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">成交量</span>
                      <span className="sq-value">{formatAmount(quote.volume, '手')}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">成交额</span>
                      <span className="sq-value">{formatAmount(quote.turnoverAmount, '元')}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">振幅</span>
                      <span className="sq-value">{quote.amplitude ? `${quote.amplitude.toFixed(2)}%` : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">量比</span>
                      <span className="sq-value">{quote.volumeRatio ? quote.volumeRatio.toFixed(2) : '-'}</span>
                    </div>
                    <div className="sq-item">
                      <span className="sq-label">流通市值</span>
                      <span className="sq-value">
                        {quote.circulatingMarketCap ? `${(quote.circulatingMarketCap / 100000000).toFixed(2)} 亿` : '-'}
                      </span>
                    </div>
                  </div>
                </div>
              </Collapsible>
            )}
            {quoteError && (
              <div className="quote-error">{quoteError}</div>
            )}


            </div>
          </>
        ) : (
          <div className="placeholder" style={{ padding: '10px 16px 16px' }}>
            <p>请从左侧自选列表中选择一只股票</p>
          </div>
        )}
      </section>

      {/* 右栏：报告展示 */}
      <section className="report-panel">
        {/* 热点成分股面板 */}
        {selectedHotConceptCode && (
          <div style={{ borderBottom: '1px solid rgba(148,163,184,0.15)', background: 'rgba(59,130,246,0.03)', display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0 }}>
            <div style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              padding: '8px 12px',
              borderBottom: '1px solid rgba(148,163,184,0.1)',
            }}>
              <span style={{ fontSize: 14, fontWeight: 600 }}>
                {hotConcepts.find((c) => c.code === selectedHotConceptCode)?.name || '概念板块'} 成分股
              </span>
              <button
                className="btn-text"
                style={{ fontSize: 12 }}
                onClick={() => {
                  setSelectedHotConceptCode(null)
                  setQuickAnalysisCode(null)
                  setQuickAnalysisData(null)
                }}
              >
                关闭
              </button>
            </div>
            {(() => {
              const cons = conceptConstituents[selectedHotConceptCode]
              const concept = hotConcepts.find((c) => c.code === selectedHotConceptCode)
              if (!cons) {
                return <div style={{ padding: '16px 12px', textAlign: 'center', color: '#64748b', fontSize: 13 }}>加载成分股中...</div>
              }
              if (cons.length === 0) {
                return (
                  <div style={{ padding: '16px 12px' }}>
                    <div style={{ textAlign: 'center', color: '#64748b', fontSize: 13, marginBottom: 8 }}>暂无成分股数据</div>
                    {concept && (
                      <div style={{ textAlign: 'center', fontSize: 12, color: '#94a3b8' }}>
                        主力净流入: {(concept.main_inflow / 1e8).toFixed(2)}亿 · 龙头: {concept.top_stock || '--'}
                      </div>
                    )}
                  </div>
                )
              }
              return (
                <div style={{ flex: 1, display: 'flex', flexDirection: 'column', minHeight: 0, overflow: 'hidden', padding: '4px 0' }}>
                  <div style={{ flexShrink: 0, maxHeight: '50%', overflowY: 'auto' }}>
                  <table style={{ width: '100%', fontSize: 13, borderCollapse: 'collapse' }}>
                    <thead>
                      <tr style={{ color: '#64748b', textAlign: 'left' }}>
                        <th style={{ padding: '6px 10px' }}>代码</th>
                        <th style={{ padding: '6px 10px' }}>名称</th>
                        <th style={{ padding: '6px 10px', textAlign: 'right' }}>最新价</th>
                        <th style={{ padding: '6px 10px', textAlign: 'right' }}>涨跌幅</th>
                        <th style={{ padding: '6px 10px', textAlign: 'right' }}>市值</th>
                        <th style={{ padding: '6px 10px', textAlign: 'right' }}>半年涨幅</th>
                        <th style={{ padding: '6px 10px', textAlign: 'right' }}>主力净流入</th>
                      </tr>
                    </thead>
                    <tbody>
                      {cons.slice(0, 20).map((s) => {
                        const isSelected = quickAnalysisCode === s.code
                        const inWatchlist = watchlist.some((w) => w.code === s.code)
                        return (
                          <tr
                            key={s.code}
                            onClick={() => {
                              if (isSelected) {
                                setQuickAnalysisCode(null)
                                setQuickAnalysisData(null)
                              } else {
                                loadQuickAnalysis(s.code, s.name, s.market || '')
                              }
                            }}
                            style={{
                              borderBottom: '1px solid rgba(148,163,184,0.06)',
                              cursor: 'pointer',
                              background: isSelected ? 'rgba(59,130,246,0.12)' : 'transparent',
                            }}
                          >
                            <td style={{ padding: '6px 10px', fontFamily: 'monospace', fontSize: 12 }}>
                              {inWatchlist ? (
                                <strong style={{ color: '#3b82f6' }}>{s.code}</strong>
                              ) : (
                                s.code
                              )}
                            </td>
                            <td style={{ padding: '6px 10px' }}>{s.name}</td>
                            <td style={{ padding: '6px 10px', textAlign: 'right' }}>{s.price?.toFixed(2) || '--'}</td>
                            <td style={{
                              padding: '6px 10px',
                              textAlign: 'right',
                              fontWeight: 600,
                              color: s.change_pct > 0 ? '#ef4444' : s.change_pct < 0 ? '#22c55e' : '#94a3b8',
                            }}>
                              {s.change_pct > 0 ? '+' : ''}{s.change_pct?.toFixed(2)}%
                            </td>
                            <td style={{ padding: '6px 10px', textAlign: 'right', fontSize: 12, color: '#64748b' }}>
                              {s.market_cap > 0 ? (s.market_cap / 1e8).toFixed(1) + '亿' : '--'}
                            </td>
                            <td style={{
                              padding: '6px 10px',
                              textAlign: 'right',
                              fontWeight: 600,
                              fontSize: 12,
                              color: s.half_year_change_pct > 0 ? '#ef4444' : s.half_year_change_pct < 0 ? '#22c55e' : '#94a3b8',
                            }}>
                              {s.half_year_change_pct > 0 ? '+' : ''}{s.half_year_change_pct?.toFixed(1)}%
                            </td>
                            <td style={{ padding: '6px 10px', textAlign: 'right', fontSize: 12, color: '#64748b' }}>
                              {Math.abs(s.main_inflow) >= 1e8
                                ? (s.main_inflow / 1e8).toFixed(2) + '亿'
                                : (s.main_inflow / 1e4).toFixed(0) + '万'}
                            </td>
                          </tr>
                        )
                      })}
                    </tbody>
                  </table>
                  </div>

                  {/* 快速分析卡片 */}
                  {quickAnalysisCode && (
                    <div style={{ flex: 1, minHeight: 0, overflowY: 'auto', margin: '8px 10px', borderRadius: 6, border: '1px solid rgba(148,163,184,0.2)', background: 'var(--panel-bg)' }}>
                      {quickAnalysisLoading && (
                        <div style={{ padding: '20px', textAlign: 'center', color: '#64748b', fontSize: 13 }}>
                          正在分析 {quickAnalysisCode}...
                        </div>
                      )}
                      {!quickAnalysisLoading && quickAnalysisData && (
                        <>
                          {/* 顶部标题栏 */}
                          <div style={{
                            display: 'flex',
                            justifyContent: 'space-between',
                            alignItems: 'center',
                            padding: '8px 10px',
                            borderBottom: '1px solid rgba(148,163,184,0.1)',
                            background: 'rgba(59,130,246,0.05)',
                          }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                              <span style={{ fontWeight: 600, fontSize: 13 }}>{quickAnalysisData.name}</span>
                              <span style={{ fontSize: 11, color: '#64748b', fontFamily: 'monospace' }}>{quickAnalysisData.code}</span>
                              {watchlist.some((w) => w.code === quickAnalysisData.code) && (
                                <span style={{ fontSize: 10, color: '#3b82f6', background: 'rgba(59,130,246,0.1)', padding: '1px 5px', borderRadius: 4 }}>自选</span>
                              )}
                              {quickAnalysisData.riskAlert && quickAnalysisData.riskAlert.level !== 'low' && (
                                <RiskBadge level={quickAnalysisData.riskAlert.level} size="small" />
                              )}
                            </div>
                            <button
                              className="btn-text"
                              style={{ fontSize: 12 }}
                              onClick={() => {
                                if (watchlist.some((w) => w.code === quickAnalysisData.code)) {
                                  return
                                }
                                AddToWatchlist(quickAnalysisData.symbol || quickAnalysisData.code + '.' + quickAnalysisData.market)
                                  .then(() => {
                                    GetWatchlist().then((list) => setWatchlist(list || []))
                                    alert(`${quickAnalysisData.name} 已加入自选`)
                                  })
                                  .catch((err: any) => alert('加入自选失败: ' + (err?.message || err)))
                              }}
                              disabled={watchlist.some((w) => w.code === quickAnalysisData.code)}
                            >
                              {watchlist.some((w) => w.code === quickAnalysisData.code) ? '已在自选' : '+ 加入自选'}
                            </button>
                          </div>

                          {/* 四宫格 */}
                          <div style={{
                            display: 'grid',
                            gridTemplateColumns: '1fr 1fr',
                            gap: 1,
                            fontSize: 12,
                          }}>
                            {/* 基本面 */}
                            <div style={{ padding: '8px 10px', background: 'rgba(59,130,246,0.03)' }}>
                              <div style={{ fontWeight: 600, color: '#3b82f6', marginBottom: 4, fontSize: 11 }}>📊 基本面</div>
                              <div style={{ color: 'var(--text-primary)', lineHeight: 1.6 }}>
                                <div>行业: {quickAnalysisData.industry || '--'}</div>
                                <div>市值: {quickAnalysisData.market_cap > 0 ? (quickAnalysisData.market_cap / 1e8).toFixed(1) + '亿' : '--'}</div>
                                <div>PE: {quickAnalysisData.pe > 0 ? quickAnalysisData.pe.toFixed(1) : '--'} · PB: {quickAnalysisData.pb > 0 ? quickAnalysisData.pb.toFixed(1) : '--'}</div>
                                <div>EPS: {quickAnalysisData.eps > 0 ? quickAnalysisData.eps.toFixed(2) : '--'}</div>
                              </div>
                            </div>
                            {/* 流动性 */}
                            <div style={{ padding: '8px 10px', background: 'rgba(34,197,94,0.03)' }}>
                              <div style={{ fontWeight: 600, color: '#22c55e', marginBottom: 4, fontSize: 11 }}>💧 流动性</div>
                              <div style={{ color: 'var(--text-primary)', lineHeight: 1.6 }}>
                                <div>最新价: {quickAnalysisData.current_price > 0 ? quickAnalysisData.current_price.toFixed(2) : '--'}</div>
                                <div>涨跌: <span style={{ color: quickAnalysisData.change_percent > 0 ? '#ef4444' : quickAnalysisData.change_percent < 0 ? '#22c55e' : '#94a3b8' }}>{quickAnalysisData.change_percent > 0 ? '+' : ''}{quickAnalysisData.change_percent?.toFixed(2) || '--'}%</span></div>
                                <div>换手: {quickAnalysisData.turnover_rate > 0 ? quickAnalysisData.turnover_rate.toFixed(2) + '%' : '--'} · 量比: {quickAnalysisData.volume_ratio > 0 ? quickAnalysisData.volume_ratio.toFixed(2) : '--'}</div>
                                {quickAnalysisData.has_moneyflow_data ? (
                                  <>
                                    <div style={{ fontWeight: 600 }}>主力净流入: <span style={{ color: quickAnalysisData.main_inflow > 0 ? '#ef4444' : quickAnalysisData.main_inflow < 0 ? '#22c55e' : '#94a3b8' }}>{quickAnalysisData.main_inflow > 0 ? '+' : ''}{formatTraceValue(quickAnalysisData.main_inflow)}</span></div>
                                    <div style={{ fontSize: 10, color: '#64748b', display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                                      <span>超大单: <span style={{ color: quickAnalysisData.elg_net_amount > 0 ? '#ef4444' : '#22c55e' }}>{quickAnalysisData.elg_net_amount > 0 ? '+' : ''}{formatTraceValue(quickAnalysisData.elg_net_amount)}</span></span>
                                      <span>大单: <span style={{ color: quickAnalysisData.lg_net_amount > 0 ? '#ef4444' : '#22c55e' }}>{quickAnalysisData.lg_net_amount > 0 ? '+' : ''}{formatTraceValue(quickAnalysisData.lg_net_amount)}</span></span>
                                      <span>中单: <span style={{ color: quickAnalysisData.md_net_amount > 0 ? '#ef4444' : '#22c55e' }}>{quickAnalysisData.md_net_amount > 0 ? '+' : ''}{formatTraceValue(quickAnalysisData.md_net_amount)}</span></span>
                                      <span>小单: <span style={{ color: quickAnalysisData.sm_net_amount > 0 ? '#ef4444' : '#22c55e' }}>{quickAnalysisData.sm_net_amount > 0 ? '+' : ''}{formatTraceValue(quickAnalysisData.sm_net_amount)}</span></span>
                                    </div>
                                  </>
                                ) : (
                                  <div>主力净流入: {quickAnalysisData.main_inflow > 0 ? (quickAnalysisData.main_inflow/1e8).toFixed(2) + '亿元' : '--'}<span style={{ fontSize: 10, color: '#94a3b8', marginLeft: 4 }}>(估算)</span></div>
                                )}
                              </div>
                            </div>
                            {/* 舆情 */}
                            <div style={{ padding: '8px 10px', background: 'rgba(168,85,247,0.03)' }}>
                              <div style={{ fontWeight: 600, color: '#a855f7', marginBottom: 4, fontSize: 11 }}>💬 舆情</div>
                              <div style={{ color: 'var(--text-primary)', lineHeight: 1.6 }}>
                                {quickAnalysisData.has_sentiment_data ? (
                                  <>
                                    <div>情绪得分: {quickAnalysisData.sentiment_score > 0 ? '+' : ''}{quickAnalysisData.sentiment_score?.toFixed(1) || '--'}</div>
                                    <div>热度: {quickAnalysisData.sentiment_heat || '--'}</div>
                                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: 3, marginTop: 2 }}>
                                      {quickAnalysisData.sentiment_keywords?.slice(0, 4).map((kw: string, i: number) => (
                                        <span key={i} style={{ fontSize: 10, background: 'rgba(168,85,247,0.1)', color: '#7e22ce', padding: '1px 4px', borderRadius: 3 }}>{kw}</span>
                                      ))}
                                    </div>
                                  </>
                                ) : (
                                  <div style={{ color: '#94a3b8' }}>舆情数据暂不可用</div>
                                )}
                              </div>
                            </div>
                            {/* 风口关联 */}
                            <div style={{ padding: '8px 10px', background: 'rgba(249,115,22,0.03)' }}>
                              <div style={{ fontWeight: 600, color: '#f97316', marginBottom: 4, fontSize: 11 }}>🌪️ 风口关联</div>
                              <div style={{ color: 'var(--text-primary)', lineHeight: 1.6 }}>
                                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 3, marginBottom: 4 }}>
                                  {quickAnalysisData.concepts?.slice(0, 5).map((c: string, i: number) => (
                                    <span key={i} style={{ fontSize: 10, background: 'rgba(249,115,22,0.1)', color: '#c2410c', padding: '1px 4px', borderRadius: 3 }}>{c}</span>
                                  )) || <span style={{ color: '#94a3b8' }}>--</span>}
                                </div>
                                {quickAnalysisData.concept_match && quickAnalysisData.concept_match.length > 0 && (
                                  <div style={{ fontSize: 11, color: '#ef4444' }}>
                                    🔥 当前热点匹配: {quickAnalysisData.concept_match.join('、')}
                                  </div>
                                )}
                              </div>
                            </div>
                          </div>

                          {/* 风险摘要 */}
                          {quickAnalysisData.riskAlert && quickAnalysisData.riskAlert.level !== 'low' && (
                            <div style={{ padding: '6px 10px', fontSize: 11, borderTop: '1px solid rgba(148,163,184,0.1)', background: quickAnalysisData.riskAlert.level === 'high' ? 'rgba(239,68,68,0.05)' : 'rgba(245,158,11,0.05)' }}>
                              <span style={{ color: quickAnalysisData.riskAlert.level === 'high' ? '#fca5a5' : '#fcd34d', fontWeight: 600 }}>
                                {quickAnalysisData.riskAlert.level === 'high' ? '🔴' : '🟡'} {quickAnalysisData.riskAlert.primaryMsg}
                              </span>
                              {quickAnalysisData.riskAlert.flags && quickAnalysisData.riskAlert.flags.length > 0 && (
                                <span style={{ color: '#94a3b8', marginLeft: 6 }}>
                                  {quickAnalysisData.riskAlert.flags.slice(0, 2).map((f: any) => f.name).join(' · ')}
                                  {quickAnalysisData.riskAlert.flags.length > 2 && ` +${quickAnalysisData.riskAlert.flags.length - 2}`}
                                </span>
                              )}
                            </div>
                          )}

                          {/* 错误提示 */}
                          {quickAnalysisData.errors && quickAnalysisData.errors.length > 0 && (
                            <div style={{ padding: '4px 10px', fontSize: 11, color: '#ef4444', borderTop: '1px solid rgba(148,163,184,0.1)' }}>
                              {quickAnalysisData.errors.map((e: string, i: number) => (
                                <div key={i}>⚠️ {e}</div>
                              ))}
                            </div>
                          )}
                        </>
                      )}
                    </div>
                  )}
                </div>
              )
            })()}
          </div>
        )}

        {!hotPanelOpen && (<>
        <div className="report-tabs">
          <div className="report-tabs-left">
            {selectedStock && (
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <button
                  className={`report-tab-btn ${activeReportTab === 'report' ? 'active' : ''}`}
                  onClick={() => setActiveReportTab('report')}
                  disabled={!displayContent}
                  title={!displayContent ? '请先执行财报分析' : '查看财报分析报告'}
                >
                  财报报告
                </button>
                <button
                  className={`report-tab-btn ${activeReportTab === 'ai' ? 'active' : ''}`}
                  onClick={() => setActiveReportTab('ai')}
                >
                  AI 投研
                </button>
                {activeReportTab === 'report' && (
                  <span className="report-timestamp">
                    {lastAnalysisAt
                      ? `上次分析: ${lastAnalysisAt}`
                      : '请先执行财报分析'}
                  </span>
                )}
              </div>
            )}
            {activeReportTab === 'report' && displayContent && (
              <select
                ref={tocSelectRef}
                className="toc-select"
                value=""
                onChange={(e) => {
                  const id = e.target.value
                  if (id) {
                    handleTocJump(id)
                    const select = e.target
                    const label = tocSections.find((s) => s.id === id)?.label || '📑 跳转章节'
                    const firstOpt = select.querySelector('option:first-child') as HTMLOptionElement | null
                    if (firstOpt) {
                      firstOpt.textContent = '⬅ ' + label
                      firstOpt.value = ''
                    }
                    select.value = ''
                  }
                }}
              >
                <option value="" disabled>📑 跳转章节</option>
                {tocSections.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.label}
                  </option>
                ))}
              </select>
            )}

          </div>
          <div className="report-tabs-right">
            {activeReportTab === 'ai' ? (
              <>
                {aiReport && (
                  <div className="download-dropdown" ref={aiExportMenuRef}>
                    <button
                      ref={aiExportMenuBtnRef}
                      className="btn-download"
                      onClick={() => setAiExportMenuOpen(!aiExportMenuOpen)}
                      title="导出 AI 投研报告"
                    >
                      导出报告 ▼
                    </button>
                    {aiExportMenuOpen && (
                      <div className="download-dropdown-menu">
                        <div
                          className="download-dropdown-item"
                          onClick={() => {
                            setAiExportMenuOpen(false)
                            handleExportAIResearchTxt()
                          }}
                        >
                          <span>📝</span> TXT 格式
                        </div>
                        <div
                          className="download-dropdown-item"
                          onClick={() => {
                            setAiExportMenuOpen(false)
                            handleCopyAIResearchTxt()
                          }}
                        >
                          <span>📋</span> 拷贝 TXT 到剪贴板
                        </div>
                        <div
                          className="download-dropdown-item"
                          onClick={() => {
                            setAiExportMenuOpen(false)
                            handleExportAIResearchMd()
                          }}
                        >
                          <span>📄</span> Markdown 格式
                        </div>
                        <div
                          className="download-dropdown-item"
                          onClick={() => {
                            setAiExportMenuOpen(false)
                            handleCopyAIResearchMd()
                          }}
                        >
                          <span>📋</span> 拷贝 Markdown 到剪贴板
                        </div>
                        <div
                          className="download-dropdown-item"
                          onClick={() => {
                            setAiExportMenuOpen(false)
                            handleExportAIResearchPdf()
                          }}
                        >
                          <span>📑</span> PDF 格式
                        </div>
                      </div>
                    )}
                  </div>
                )}
                {aiReportLoading ? (
                  <button
                    className="btn-delete-report"
                    onClick={handleCancelAI}
                    title="取消分析"
                  >
                    取消
                  </button>
                ) : (
                  <button
                    className="btn-download"
                    onClick={() => handleAnalyzeAI(aiReport ? true : false)}
                    title={aiReport ? '重新分析' : '开始分析'}
                  >
                    {aiReport ? '重新分析' : '开始分析'}
                  </button>
                )}
              </>
            ) : (
              <>
                <div className="report-search-wrap">
                  <input
                    ref={reportSearchRef}
                    type="text"
                    className="report-search-input"
                    placeholder="搜索报告内容"
                    onKeyDown={handleReportSearchKeyDown}
                    disabled={!displayContent}
                    title={!displayContent ? '请先执行分析' : '输入关键词，按回车依次跳转匹配项'}
                  />
                </div>
                <button
                  className="btn-delete-report"
                  onClick={handleDeleteReport}
                  disabled={!displayContent}
                  title={!displayContent ? '没有可删除的报告' : '删除当前显示的报告'}
                >
                  删除报告
                </button>
                <div className="download-dropdown" ref={downloadMenuRef}>
                  <button
                    ref={downloadMenuBtnRef}
                    className="btn-download"
                    onClick={() => setDownloadMenuOpen(!downloadMenuOpen)}
                    disabled={!displayContent}
                    title={!displayContent ? '请先执行分析' : '下载当前显示的报告'}
                  >
                    下载报告 ▼
                  </button>
                  {downloadMenuOpen && (
                    <div className="download-dropdown-menu">
                      <div
                        className="download-dropdown-item"
                        onClick={() => {
                          setDownloadMenuOpen(false)
                          handleReportDownload()
                        }}
                      >
                        <span>📝</span> Markdown 格式
                      </div>
                      <div
                        className="download-dropdown-item"
                        onClick={() => {
                          setDownloadMenuOpen(false)
                          handleExportPDF()
                        }}
                      >
                        <span>📄</span> PDF 格式
                      </div>
                      <div
                        className="download-dropdown-item"
                        onClick={() => {
                          setDownloadMenuOpen(false)
                          handleDownloadImage()
                        }}
                      >
                        <span>🖼️</span> 长图片
                      </div>
                    </div>
                  )}
                </div>
              </>
            )}
          </div>
        </div>
        <div className="report-content" ref={reportContentRef}>
          {activeReportTab === 'ai' && selectedStock ? (
            <AIResearchPanel
              symbol={selectedStock.code}
              name={selectedStock.name || ''}
              report={aiReport}
              loading={aiReportLoading}
              error={aiReportError}
              progress={aiProgress}
              reportRef={aiReportContentRef}
            />
          ) : displayContent ? (
            <div className="markdown-body" onClick={(e) => {
              const target = e.target as HTMLElement
              if (target.closest('.rim-adjust-btn')) {
                e.preventDefault()
                openRIMModal()
                return
              }
              if (target.closest('.fetch-activity-trigger')) {
                e.preventDefault()
                handleFetchMissingActivity()
              }
            }}>
              <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeSlug]} components={markdownComponents}>
                {displayContent}
              </ReactMarkdown>
            </div>
          ) : selectedStock ? (
            <div className="placeholder">
              <p>【Markdown 报告展示区】</p>
              <p>选择股票后点击"财报分析"，报告将在此渲染</p>
            </div>
          ) : (
            <div className="placeholder">
              <p>未选择股票</p>
            </div>
          )}
        </div>
        </>)}
        {hotPanelOpen && !selectedHotConceptCode && (
          <div className="placeholder">
            <p>请选择热点概念查看成分股</p>
          </div>
        )}
      </section>

      {/* 点击空白处关闭下拉 */}
      {showDropdown && (
        <div
          className="overlay"
          onClick={() => {
            setShowDropdown(false)
            inputRef.current?.blur()
          }}
        />
      )}

      {/* 计算溯源抽屉 */}
      {traceDrawerOpen && currentTrace && (
        <div className="trace-overlay" onClick={() => setTraceDrawerOpen(false)}>
          <div className="trace-drawer" onClick={(e) => e.stopPropagation()}>
            <div className="trace-header">
              <h3>
                {currentTrace.indicator}（{currentTrace.year}）计算过程
              </h3>
              <button className="trace-close" onClick={() => setTraceDrawerOpen(false)}>
                ×
              </button>
            </div>
            {traceList.length > 1 && (
              <div className="trace-switcher">
                <select
                  value={traceList.indexOf(currentTrace)}
                  onChange={(e) => {
                    const idx = Number(e.target.value)
                    if (traceList[idx]) setCurrentTrace(traceList[idx])
                  }}
                >
                  {(() => {
                    const groups: Record<string, analyzer.CalcTrace[]> = {}
                    traceList.forEach((t) => {
                      if (!groups[t.indicator]) groups[t.indicator] = []
                      groups[t.indicator].push(t)
                    })
                    Object.keys(groups).forEach((indicator) => {
                      groups[indicator].sort((a, b) => b.year.localeCompare(a.year))
                    })
                    return Object.entries(groups).map(([indicator, traces]) => (
                      <optgroup key={indicator} label={indicator}>
                        {traces.map((t) => {
                          const idx = traceList.indexOf(t)
                          return (
                            <option key={idx} value={idx}>
                              {t.year}
                            </option>
                          )
                        })}
                      </optgroup>
                    ))
                  })()}
                </select>
                <span className="trace-count">共 {traceList.length} 个指标</span>
              </div>
            )}
            <div className="trace-body">
              <div className="trace-section">
                <div className="trace-section-title">公式</div>
                <div className="trace-formula">{currentTrace.formula}</div>
              </div>
              <div className="trace-section">
                <div className="trace-section-title">原始数据</div>
                {currentTrace.inputs &&
                  Object.entries(currentTrace.inputs).map(([k, v]) => (
                    <div key={k} className="trace-input-row">
                      <span className="trace-input-name">
                        • {v.item}（{v.source}，{v.year}）
                      </span>
                      <span className="trace-input-value">{formatTraceValue(v.value)}</span>
                      {v.note && <span className="trace-input-note">{v.note}</span>}
                    </div>
                  ))}
              </div>
              <div className="trace-section">
                <div className="trace-section-title">计算步骤</div>
                {currentTrace.steps?.map((s, idx) => (
                  <div key={idx} className="trace-step">
                    <div className="trace-step-desc">
                      {idx + 1}. {s.desc}
                    </div>
                    <div className="trace-step-expr">{s.expr}</div>
                    <div className="trace-step-result">
                      = {formatTraceResult(s.value, currentTrace.indicator)}
                    </div>
                  </div>
                ))}
              </div>
              {currentTrace.note && (
                <div className="trace-section">
                  <div className="trace-section-title">💡 口径说明</div>
                  <div className="trace-note">{currentTrace.note}</div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* 强制重新分析弹窗 */}
      {forceAnalyzeOpen && (
        <div className="modal-overlay" onClick={() => setForceAnalyzeOpen(false)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <h4>数据未发生变化</h4>
            <p>
              上次分析时间：{lastAnalysisAt || '未知'}
              <br />
              当前财务数据与上次分析时一致，是否强制重新生成报告？
            </p>
            <div className="modal-actions">
              <button className="btn" onClick={() => setForceAnalyzeOpen(false)}>
                取消
              </button>
              <button
                className="btn primary"
                onClick={async () => {
                  setForceAnalyzeOpen(false)
                  await runAnalyze(true)
                }}
              >
                强制重新分析
              </button>
            </div>
          </div>
        </div>
      )}

      {/* RIM 参数调整弹窗 */}
      {showRIMModal && (
        <div className="modal-overlay" onClick={() => setShowRIMModal(false)}>
          <div className="modal-content rim-modal" onClick={(e) => e.stopPropagation()}>
            <h4>调整 RIM 估值参数</h4>
            <div className="rim-form">
              <div className="rim-hint" style={{ fontSize: 12, color: 'var(--text-secondary)', marginBottom: 4 }}>
                💡 默认参数来源：Rf（中国10年期国债收益率）、Beta（近1年个股 vs 沪深300）、Rm-Rf（沪深300隐含风险溢价），建议根据当前市场环境调整
              </div>
              {(rimRfDate || rimBetaDate || rimRmRfDate) && (
                <div style={{ fontSize: 11, color: 'var(--text-muted)', marginBottom: 8 }}>
                  数据日期：{[rimRfDate && `Rf ${rimRfDate}`, rimBetaDate && `Beta ${rimBetaDate}`, rimRmRfDate && `Rm-Rf ${rimRmRfDate}`].filter(Boolean).join(' / ')}
                </div>
              )}
              <div className="rim-row">
                <label>Beta</label>
                <input type="number" step={0.01} value={rimBeta} onChange={(e) => setRimBeta(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>无风险利率 Rf (%)</label>
                <input type="number" step={0.01} value={rimRf} onChange={(e) => setRimRf(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>市场风险溢价 Rm-Rf (%)</label>
                <input type="number" step={0.01} value={rimRmRf} onChange={(e) => setRimRmRf(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>永续增长率 g (%)</label>
                <input type="number" step={0.1} value={rimG} onChange={(e) => setRimG(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>每股净资产 BPS0</label>
                <input type="number" step={0.01} value={rimBPS0} onChange={(e) => setRimBPS0(Number(e.target.value))} />
              </div>
              <div className="rim-row">
                <label>当前股价</label>
                <input type="number" step={0.01} value={rimPrice} onChange={(e) => setRimPrice(Number(e.target.value))} />
              </div>
              <div className="rim-eps-title">预测期 EPS（至少填前3年）</div>
              <div className="rim-eps-grid">
                {rimEPS.map((v, i) => (
                  <div className="rim-eps-item" key={i}>
                    <label>第{i + 1}年</label>
                    <input type="text" inputMode="decimal" value={v} onChange={(e) => {
                      const val = e.target.value
                      if (!/^\d*\.?\d*$/.test(val)) return
                      const next = [...rimEPS]
                      next[i] = val === '' ? '0' : val
                      setRimEPS(next)
                    }} />
                  </div>
                ))}
              </div>
            </div>
            <div className="modal-actions">
              <button className="btn" onClick={() => setShowRIMModal(false)} disabled={rimLoading}>
                取消
              </button>
              <button className="btn primary" onClick={handleAnalyzeWithRIM} disabled={rimLoading || rimBPS0 <= 0 || rimEPS.map(Number).filter((v) => v > 0).length < 1}>
                {rimLoading ? (
                  <>
                    <span className="btn-progress" style={{ width: `${rimProgress}%` }} />
                    <span style={{ position: 'relative', zIndex: 1 }}>分析中 {rimProgress}%</span>
                  </>
                ) : '应用并重新分析'}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 财务指标趋势图弹窗 */}
      {trendDrawerCode && (
        <FinancialTrendDrawer
          code={trendDrawerCode}
          name={selectedStock?.name}
          onClose={() => setTrendDrawerCode(null)}
        />
      )}

      {marginDrawerCode && (
        <MarginDrawer
          code={marginDrawerCode}
          name={selectedStock?.name}
          onClose={() => setMarginDrawerCode(null)}
        />
      )}




      {/* 技术图全窗口（K线 + 技术指标联动） */}
      {klineFullscreen && selectedStock && (
        <UnifiedChart
          code={selectedStock.code}
          name={selectedStock.name}
          quote={quote || undefined}
          initialExpanded
          onClose={() => setKlineFullscreen(false)}
        />
      )}

      {/* Python 依赖检测弹窗 */}
      <PythonDepsModal
        isOpen={showPythonDepsModal}
        onClose={() => setShowPythonDepsModal(false)}
      />

      {/* 自动更新弹窗 */}
      <UpdateModal
        isOpen={showUpdateModal}
        info={updateInfo}
        onClose={() => setShowUpdateModal(false)}
      />
    </div>
  )
}

export default App
