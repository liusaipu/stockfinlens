import { useEffect, useMemo, useRef, useState, type CSSProperties } from 'react'
import { createPortal } from 'react-dom'
import * as echarts from 'echarts'
import type { downloader } from '../wailsjs/go/models'
import { GetStockKlines, GetStockQuote, RefreshStockKlines } from './api'
import { IntradayChart } from './IntradayChart'

type KlineData = downloader.KlineData
type StockQuote = downloader.StockQuote

interface Props {
  code: string
  name?: string
  quote?: StockQuote
  // 挂载时即为全屏展开状态（用于"技术图"按钮触发的独立实例）
  initialExpanded?: boolean
  // 用户退出全屏时回调（dblclick / Esc / 右上角按钮三个出口都会触发）
  onClose?: () => void
}

// 均线配置
interface MAConfig {
  count: number
  periods: number[]
}

const defaultMAConfig: MAConfig = {
  count: 4,
  periods: [5, 10, 30, 60],
}

const maColors = ['#fbbf24', '#60a5fa', '#a78bfa', '#f87171', '#34d399', '#fb923c']

const colors = {
  up: '#ef4444',
  down: '#22c55e',
  macd: '#f59e0b',
  signal: '#3b82f6',
  histPositive: '#ef4444',
  histNegative: '#22c55e',
  rsi6: '#f97316',
  rsi12: '#a78bfa',
  rsi24: '#94a3b8',
  bbUpper: '#ef4444',
  bbMid: '#f59e0b',
  bbLower: '#10b981',
  shIndex: '#8b5cf6',
  szIndex: '#06b6d4',
  cyIndex: '#ec4899',
}

// 大盘指数配置
const INDEX_CONFIGS = [
  { code: '000001.SH', label: '上证综指', color: colors.shIndex },
  { code: '399001.SZ', label: '深圳成指', color: colors.szIndex },
  { code: '399006.SZ', label: '创业板指', color: colors.cyIndex },
]

function calcEMA(arr: number[], period: number): (number | null)[] {
  const k = 2 / (period + 1)
  const ema: (number | null)[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i === 0) ema.push(arr[0])
    else ema.push(arr[i] * k + (ema[i - 1] as number) * (1 - k))
  }
  return ema
}

function calcMA(arr: number[], period: number): (number | null)[] {
  const ma: (number | null)[] = []
  for (let i = 0; i < arr.length; i++) {
    if (i < period - 1) { ma.push(null); continue }
    let sum = 0
    for (let j = i - period + 1; j <= i; j++) sum += arr[j]
    ma.push(sum / period)
  }
  return ma
}

function calculateIndicators(data: KlineData[], maConfig: MAConfig) {
  const closes = data.map(d => d.close)

  // 动态计算均线
  const mas: { name: string; data: (number | null)[]; color: string }[] = []
  for (let i = 0; i < maConfig.count; i++) {
    const period = maConfig.periods[i] || 5
    mas.push({
      name: `MA${period}`,
      data: calcMA(closes, period),
      color: maColors[i % maColors.length],
    })
  }

  const ema12 = calcEMA(closes, 12)
  const ema26 = calcEMA(closes, 26)
  const dif: (number | null)[] = ema12.map((v, i) => (v == null || ema26[i] == null) ? null : v - ema26[i]!)
  const validDif = dif.filter((v): v is number => v != null)
  const validDea = calcEMA(validDif, 9)
  const dea: (number | null)[] = []
  let deaIdx = 0
  for (let i = 0; i < dif.length; i++) {
    if (dif[i] == null) dea.push(null)
    else dea.push(validDea[deaIdx++] ?? null)
  }
  const hist: (number | null)[] = dif.map((v, i) => (v == null || dea[i] == null) ? null : 2 * (v - dea[i]!))

  function calcRSI(period: number): (number | null)[] {
    const result: (number | null)[] = []
    let avgGain = 0
    let avgLoss = 0
    for (let i = 0; i < closes.length; i++) {
      if (i === 0) { result.push(null); continue }
      const diff = closes[i] - closes[i - 1]
      const gain = diff > 0 ? diff : 0
      const loss = diff < 0 ? -diff : 0
      if (i < period) {
        avgGain += gain
        avgLoss += loss
        result.push(null)
      } else if (i === period) {
        avgGain += gain
        avgLoss += loss
        avgGain /= period
        avgLoss /= period
        result.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
      } else {
        avgGain = (avgGain * (period - 1) + gain) / period
        avgLoss = (avgLoss * (period - 1) + loss) / period
        result.push(avgLoss === 0 ? 100 : 100 - 100 / (1 + avgGain / avgLoss))
      }
    }
    return result
  }
  const rsi6 = calcRSI(6)
  const rsi12 = calcRSI(12)
  const rsi24 = calcRSI(24)

  const bbUpper: (number | null)[] = [], bbMid: (number | null)[] = [], bbLower: (number | null)[] = []
  for (let i = 0; i < closes.length; i++) {
    if (i < 19) { bbUpper.push(null); bbMid.push(null); bbLower.push(null); continue }
    const slice = closes.slice(i - 19, i + 1)
    const mean = slice.reduce((a, b) => a + b, 0) / 20
    const std = Math.sqrt(slice.reduce((sq, n) => sq + Math.pow(n - mean, 2), 0) / 20)
    bbMid.push(mean)
    bbUpper.push(mean + 2 * std)
    bbLower.push(mean - 2 * std)
  }

  return { dif, dea, hist, rsi6, rsi12, rsi24, bbUpper, bbMid, bbLower, mas }
}

function fmt2(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(2)
}
function fmt3(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(3)
}
function fmt1(v: any): string {
  if (v == null) return '-'
  const n = Number(v)
  if (isNaN(n)) return '-'
  return n.toFixed(1)
}

// 日线聚合为周线（按自然周：周一~周日）
function aggregateToWeekly(data: KlineData[]): KlineData[] {
  if (data.length === 0) return []
  const weeks: Record<string, KlineData[]> = {}
  data.forEach((d) => {
    // 安全解析 YYYY-MM-DD，避免 toISOString() 时区/Invalid Date 问题
    const parts = d.time.split('-')
    if (parts.length !== 3) return
    const y = parseInt(parts[0], 10)
    const m = parseInt(parts[1], 10) - 1
    const day = parseInt(parts[2], 10)
    if (isNaN(y) || isNaN(m) || isNaN(day)) return

    const date = new Date(y, m, day)
    const dayOfWeek = date.getDay() // 0=周日, 1=周一...
    const mondayOffset = dayOfWeek === 0 ? -6 : 1 - dayOfWeek
    const monday = new Date(y, m, day + mondayOffset)

    const ky = monday.getFullYear()
    const km = String(monday.getMonth() + 1).padStart(2, '0')
    const kd = String(monday.getDate()).padStart(2, '0')
    const key = `${ky}-${km}-${kd}`

    if (!weeks[key]) weeks[key] = []
    weeks[key].push(d)
  })
  return Object.entries(weeks).map(([, bars]) => ({
    time: bars[bars.length - 1].time,
    open: bars[0].open,
    close: bars[bars.length - 1].close,
    high: Math.max(...bars.map((b) => b.high)),
    low: Math.min(...bars.map((b) => b.low)),
    volume: bars.reduce((sum, b) => sum + b.volume, 0),
    amount: bars.reduce((sum, b) => sum + b.amount, 0),
    turnoverRate: bars[bars.length - 1].turnoverRate,
  }))
}

// 日线聚合为月线
function aggregateToMonthly(data: KlineData[]): KlineData[] {
  if (data.length === 0) return []
  const months: Record<string, KlineData[]> = {}
  data.forEach((d) => {
    const key = d.time.slice(0, 7)
    if (!months[key]) months[key] = []
    months[key].push(d)
  })
  return Object.entries(months).map(([, bars]) => ({
    time: bars[bars.length - 1].time,
    open: bars[0].open,
    close: bars[bars.length - 1].close,
    high: Math.max(...bars.map((b) => b.high)),
    low: Math.min(...bars.map((b) => b.low)),
    volume: bars.reduce((sum, b) => sum + b.volume, 0),
    amount: bars.reduce((sum, b) => sum + b.amount, 0),
    turnoverRate: bars[bars.length - 1].turnoverRate,
  }))
}

// 统一日期格式为 YYYY-MM-DD，兼容 YYYYMMDD 和 YYYY/MM/DD
function normalizeTime(time: string): string {
  if (/^\d{8}$/.test(time)) {
    return `${time.slice(0, 4)}-${time.slice(4, 6)}-${time.slice(6, 8)}`
  }
  if (/^\d{4}\/\d{2}\/\d{2}/.test(time)) {
    return time.replace(/\//g, '-')
  }
  return time
}

// 按当前周期聚合指数日线数据，使其与股票 data 的日期对齐
function aggregateIndexForPeriod(indexDaily: KlineData[], period: 'daily' | 'weekly' | 'monthly'): KlineData[] {
  if (period === 'daily') return indexDaily
  if (indexDaily.length === 0) return []
  return period === 'weekly' ? aggregateToWeekly(indexDaily) : aggregateToMonthly(indexDaily)
}

// 把指数收盘价按股票交易日对齐，使用独立右侧纵轴显示真实指数点位
function buildIndexSeries(
  data: KlineData[],
  indexData: Record<string, KlineData[]>,
  selectedIndices: Record<string, boolean>,
  period: 'daily' | 'weekly' | 'monthly'
): { name: string; color: string; data: (number | null)[] }[] {
  if (data.length === 0) return []
  const dateSet = new Set(data.map(d => d.time))

  return INDEX_CONFIGS.filter(cfg => selectedIndices[cfg.code]).map(cfg => {
    const aggregated = aggregateIndexForPeriod(indexData[cfg.code] || [], period)
    const closeMap = new Map<string, number>()
    aggregated.forEach(d => {
      if (dateSet.has(d.time) && d.close > 0) closeMap.set(d.time, d.close)
    })

    const seriesData: (number | null)[] = []
    data.forEach(d => {
      const ic = closeMap.get(d.time)
      seriesData.push(ic != null ? ic : null)
    })
    return { name: cfg.label, color: cfg.color, data: seriesData }
  })
}

// 把当日实时行情合并进日线序列：若历史已有当日则覆盖，否则追加
function mergeTodayQuote(data: KlineData[], quote: StockQuote | undefined): KlineData[] {
  if (!quote || quote.currentPrice <= 0 || !quote.quoteTime) return data
  const today = normalizeTime(quote.quoteTime.slice(0, 10))
  if (!today || today.length !== 10) return data

  const todayBar: KlineData = {
    time: today,
    open: quote.open > 0 ? quote.open : quote.currentPrice,
    close: quote.currentPrice,
    high: quote.high > 0 ? quote.high : quote.currentPrice,
    low: quote.low > 0 ? quote.low : quote.currentPrice,
    volume: quote.volume,
    amount: quote.turnoverAmount,
    turnoverRate: quote.turnoverRate,
  }

  const last = data[data.length - 1]
  if (last && last.time === today) {
    return [...data.slice(0, -1), todayBar]
  }
  if (!last || last.time < today) {
    return [...data, todayBar]
  }
  return data
}

function loadMAConfig(): MAConfig {
  try {
    const saved = localStorage.getItem('unifiedChart_maConfig')
    if (saved) {
      const parsed = JSON.parse(saved)
      if (typeof parsed.count === 'number' && Array.isArray(parsed.periods)) {
        return {
          count: Math.min(6, Math.max(1, parsed.count)),
          periods: parsed.periods.map((p: number) => Math.min(250, Math.max(1, p))),
        }
      }
    }
  } catch {}
  return { ...defaultMAConfig }
}

function saveMAConfig(config: MAConfig) {
  localStorage.setItem('unifiedChart_maConfig', JSON.stringify(config))
}

export function UnifiedChart({ code, name, quote: propQuote, initialExpanded, onClose }: Props) {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)
  const [rawData, setRawData] = useState<KlineData[]>([])
  const [localQuote, setLocalQuote] = useState<StockQuote | undefined>(propQuote)
  const [loading, setLoading] = useState(false)
  const [refreshing, setRefreshing] = useState(false)
  const [isExpanded, setIsExpanded] = useState(!!initialExpanded)
  const [period, setPeriod] = useState<'intraday' | 'daily' | 'weekly' | 'monthly'>('daily')
  const [maConfig, setMAConfig] = useState<MAConfig>(loadMAConfig)
  const [showSettings, setShowSettings] = useState(false)
  const [indexData, setIndexData] = useState<Record<string, KlineData[]>>({})
  const [selectedIndices, setSelectedIndices] = useState<Record<string, boolean>>({})
  const intradayBtnRef = useRef<HTMLButtonElement>(null)
  const maRowRef = useRef<HTMLDivElement>(null)

  // 用 ref 稳定 onClose，避免 chart 创建 useEffect 因 prop 变化而重建
  const onCloseRef = useRef(onClose)
  useEffect(() => { onCloseRef.current = onClose }, [onClose])

  // 标记当前是否在等待"刷新触发的 setOption"完成绘制。
  // setOption(option, true) 内部会异步分批重绘 series / 重新计算 dataZoom / axisPointer，
  // React 18 自动 batching 会让 setRefreshing(false) 与 setRawData() 同帧 commit，
  // 那一瞬间遮罩消失但 zrender 还没画完，残留的旧 axisPointer / 部分 series 露出来变成"两条长条"。
  // 改成等 chart 'finished' 事件（echarts 完整绘制结束）触发后再解除 refreshing。
  const refreshPendingRef = useRef(false)

  // 如果 propQuote 变化，同步更新 localQuote
  useEffect(() => {
    setLocalQuote(propQuote)
  }, [propQuote])

  // K 线数据加载（始终获取日线，前端按周期聚合）
  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetStockKlines(code, 'daily')
      .then((list) => {
        // 统一日期格式（兼容腾讯 YYYYMMDD / 东财 YYYY-MM-DD / 网易 YYYY/MM/DD）
        const normalized = (list || []).map((d) => ({
          ...d,
          time: normalizeTime(d.time),
        }))
        setRawData(normalized)
      })
      .catch(() => setRawData([]))
      .finally(() => setLoading(false))
  }, [code])

  // 加载大盘指数数据（日线）
  useEffect(() => {
    if (!code) return
    const loadIndices = async () => {
      const result: Record<string, KlineData[]> = {}
      await Promise.all(
        INDEX_CONFIGS.map(async (cfg) => {
          try {
            let list = await GetStockKlines(cfg.code, 'daily')
            if (!list || list.length === 0) {
              console.warn(`[UnifiedChart] index ${cfg.code} empty from GetStockKlines, retry with RefreshStockKlines`)
              list = await RefreshStockKlines(cfg.code, 'daily')
            }
            const normalized = (list || []).map((d) => ({
              ...d,
              time: normalizeTime(d.time),
            }))
            result[cfg.code] = normalized
            console.log(`[UnifiedChart] index ${cfg.code} loaded ${normalized.length} bars`)
          } catch (e) {
            console.warn(`[UnifiedChart] index ${cfg.code} load failed`, e)
            result[cfg.code] = []
          }
        })
      )
      setIndexData(result)
    }
    loadIndices()
  }, [code])

  // 自己获取行情（如果 propQuote 为 null/undefined）
  useEffect(() => {
    if (!code || propQuote) return
    GetStockQuote(code)
      .then((q) => {
        if (q && q.currentPrice > 0) {
          setLocalQuote(q)
        }
      })
      .catch(() => {})
  }, [code, propQuote])

  const data = useMemo(() => {
    if (rawData.length === 0) return []
    // 分时图不合并日线；日线/周线/月线先把当日 quote 合并进日线序列，再做聚合
    const dailyData = period === 'intraday' ? rawData : mergeTodayQuote(rawData, localQuote)
    // 根据周期聚合数据
    let aggregated = dailyData
    if (period === 'weekly') {
      aggregated = aggregateToWeekly(dailyData)
    } else if (period === 'monthly') {
      aggregated = aggregateToMonthly(dailyData)
    }
    const quote = localQuote
    const hasTurnover = aggregated.some(d => d.turnoverRate > 0)
    if (hasTurnover || !quote || quote.circulatingMarketCap <= 0 || quote.currentPrice <= 0) {
      return aggregated
    }
    const circulatingShares = quote.circulatingMarketCap / quote.currentPrice
    return aggregated.map(d => ({
      ...d,
      turnoverRate: (d.volume * 100 / circulatingShares) * 100,
    }))
  }, [rawData, localQuote, period])

  // 指标计算：供图表 series 与左上角均线标签共用
  const indicators = useMemo(() => calculateIndicators(data, maConfig), [data, maConfig])

  // 指数序列（周线/月线时先对指数日线做同样聚合），使用右侧独立纵轴显示真实点位
  const indexSeries = useMemo(
    () => buildIndexSeries(data, indexData, selectedIndices, period === 'intraday' ? 'daily' : period),
    [data, indexData, selectedIndices, period]
  )

  // 让 K 线图内部均线标签的 "均线" 二字与顶层 "分时" 按钮水平对齐
  useEffect(() => {
    const update = () => {
      if (intradayBtnRef.current && maRowRef.current) {
        const panel = intradayBtnRef.current.closest('[data-chart-panel]') as HTMLElement | null
        if (!panel) return
        const intradayRect = intradayBtnRef.current.getBoundingClientRect()
        const panelRect = panel.getBoundingClientRect()
        maRowRef.current.style.left = `${intradayRect.left - panelRect.left}px`
      }
    }
    update()
    window.addEventListener('resize', update)
    return () => window.removeEventListener('resize', update)
  }, [period, refreshing, isExpanded])

  // chart 实例创建/销毁：只在 mount/unmount 时执行。
  // 数据/配置变化只通过 setOption 更新内容，避免 dispose→init 之间露出深色背景 + 蓝色 axisPointer 占位帧。
  useEffect(() => {
    if (!chartRef.current) return

    const chart = echarts.init(chartRef.current, 'dark', { renderer: 'canvas' })
    chartInstanceRef.current = chart

    chart.getZr().on('dblclick', () => {
      setIsExpanded(prev => {
        const next = !prev
        if (!next) onCloseRef.current?.()
        return next
      })
    })

    // 等 echarts 完整绘制结束后，才解除"刷新中"遮罩
    chart.on('finished', () => {
      if (refreshPendingRef.current) {
        refreshPendingRef.current = false
        setRefreshing(false)
      }
    })

    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)

    return () => {
      window.removeEventListener('resize', handleResize)
      chart.dispose()
      chartInstanceRef.current = null
    }
  }, [])

  const [isLightTheme, setIsLightTheme] = useState(false)
  useEffect(() => {
    const check = () => setIsLightTheme(document.body.classList.contains('light'))
    check()
    const observer = new MutationObserver(check)
    observer.observe(document.body, { attributes: true, attributeFilter: ['class'] })
    return () => observer.disconnect()
  }, [])

  // 数据/配置变化时只更新 option（notMerge 完整替换 series/grid 结构）
  useEffect(() => {
    const chart = chartInstanceRef.current
    if (!chart || data.length === 0) return

    // 默认可见窗口大小:常态 120 条 / 全屏 250 条;
    // 但 xAxis.data 给的是**全量**,通过 dataZoom 控制可见范围,这样用户能左右拖动浏览历史。
    const visibleSize = isExpanded ? 250 : 120
    const total = data.length
    const zoomStart = total > visibleSize ? ((total - visibleSize) / total) * 100 : 0

    const { dif, dea, hist, rsi6, rsi12, rsi24, bbUpper, bbMid, bbLower, mas } = indicators

    // xAxis 与所有 series 全部使用全量数据;dataZoom 只裁剪可见窗口,不裁剪计算窗口。
    const dates = data.map(d => d.time)
    const candleData = data.map(d => [d.open, d.close, d.low, d.high])
    const turnoverData = data.map((d: KlineData) => ({
      value: d.turnoverRate,
      itemStyle: { color: d.close >= d.open ? 'rgba(239,68,68,0.35)' : 'rgba(34,197,94,0.35)' },
    }))

    const xAxisLabelInterval = isExpanded ? Math.max(1, Math.floor(visibleSize / 6)) : Math.max(1, Math.floor(visibleSize / 6))
    // 数据不足可见窗口时，让坐标轴保持 visibleSize 个刻度位置，右侧自然留白，不拉宽 K 线
    const xAxisMax = Math.max(total, visibleSize) - 1

    // 右侧边距固定 100px，避免选择/取消指数时图表整体跳动
    const gridRight = 100

    const option: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      animation: false,
      tooltip: {
        trigger: 'axis',
        axisPointer: {
          type: 'cross',
          link: [{ xAxisIndex: 'all' }] as any,
          label: { show: false },
        },
        backgroundColor: 'rgba(15, 23, 42, 0.95)',
        borderColor: 'rgba(148, 163, 184, 0.25)',
        borderWidth: 1,
        textStyle: { color: '#e2e8f0', fontSize: 12 },
        padding: 0,
        formatter: (params: any) => {
          if (!params || params.length === 0) return ''
          const date = params[0].axisValue || ''
          if (!date) return ''

          const leftItems: string[] = []
          const candle = params.find((p: any) => p.seriesName === 'K线')
          if (candle) {
            // 全量 data 模式：dataIndex 即 data 数组的索引，不再有 padding
            const idx = candle.dataIndex
            const d = data[idx]
            if (d) {
              const o = d.open, c = d.close, l = d.low, h = d.high
              const prevClose = idx > 0 ? data[idx - 1].close : o
              const change = c - prevClose
              const changePct = prevClose !== 0 ? (change / prevClose) * 100 : 0
              const changeColor = change >= 0 ? '#ef4444' : '#22c55e'
              const changeSign = change >= 0 ? '+' : ''
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">开盘</span><span>${fmt2(o)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">收盘</span><span>${fmt2(c)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">涨跌额</span><span style="color:${changeColor}">${changeSign}${fmt2(change)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">涨跌幅</span><span style="color:${changeColor}">${changeSign}${fmt2(changePct)}%</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">最低</span><span>${fmt2(l)}</span></div>`)
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">最高</span><span>${fmt2(h)}</span></div>`)
            }
          }
          params.filter((p: any) => p.seriesName.startsWith('MA')).forEach((p: any) => {
            const color = p.color || '#94a3b8'
            leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt2(p.value)}</span></div>`)
          })

          // 指数（右侧纵轴真实点位）
          const indexParams = params.filter((p: any) => INDEX_CONFIGS.some(cfg => cfg.label === p.seriesName))
          if (indexParams.length) {
            if (leftItems.length) leftItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            indexParams.forEach((p: any) => {
              const cfg = INDEX_CONFIGS.find(c => c.label === p.seriesName)
              const color = cfg?.color || '#94a3b8'
              leftItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt2(p.value)}</span></div>`)
            })
          }

          const rightItems: string[] = []
          const turnover = params.find((p: any) => p.seriesName === '换手率')
          if (turnover) {
            rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:#94a3b8">换手率</span><span>${turnover.value != null ? fmt2(turnover.value) + '%' : '-'}</span></div>`)
          }

          const macdParams = params.filter((p: any) => ['DIF', 'DEA', 'MACD'].includes(p.seriesName))
          if (macdParams.length) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            macdParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt3(p.value)}</span></div>`)
            })
          }
          const rsiParams = params.filter((p: any) => ['RSI6', 'RSI12', 'RSI24'].includes(p.seriesName))
          if (rsiParams.length) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            rsiParams.forEach((p: any) => {
              const colorMap: Record<string, string> = { RSI6: colors.rsi6, RSI12: colors.rsi12, RSI24: colors.rsi24 }
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${colorMap[p.seriesName] || '#94a3b8'}">● ${p.seriesName}</span><span>${fmt1(p.value)}</span></div>`)
            })
          }
          const bbParams = params.filter((p: any) => ['上轨', '中轨', '下轨'].includes(p.seriesName))
          if (bbParams.length) {
            if (rightItems.length) rightItems.push('<div style="border-top:1px solid rgba(148,163,184,0.12);margin:4px 0"></div>')
            bbParams.forEach((p: any) => {
              const color = p.color || '#94a3b8'
              rightItems.push(`<div style="display:flex;justify-content:space-between;gap:18px"><span style="color:${color}">● ${p.seriesName}</span><span>${fmt2(p.value)}</span></div>`)
            })
          }

          return `
            <div style="line-height:1.65;font-size:12px">
              <div style="font-weight:600;margin-bottom:6px;color:#f0f0f0;padding:10px 14px 0">${date}</div>
              <div style="display:flex;gap:14px;padding:0 14px 10px">
                <div style="min-width:110px">${leftItems.join('')}</div>
                <div style="min-width:110px">${rightItems.join('')}</div>
              </div>
            </div>
          `
        },
      },
      axisPointer: {
        link: [{ xAxisIndex: 'all' }],
        label: { show: false },
      },
      // dataZoom 让用户能左右拖动浏览全部历史，初始窗口落在最新一段。
      // type: 'inside' = 内嵌交互，无底部滑块；5 个子图共享同一个 zoom。
      dataZoom: [
        {
          type: 'inside',
          xAxisIndex: [0, 1, 2, 3, 4],
          start: zoomStart,
          end: 100,
          zoomOnMouseWheel: true,  // 滚轮缩放
          moveOnMouseMove: true,   // 按住鼠标拖动平移
          moveOnMouseWheel: false, // shift+滚轮 平移（关掉，避免与缩放混淆）
        },
      ],
      // 整体上移 18px，避免左上角 topbar 文字与左侧纵轴“股价”名称重叠
      grid: isExpanded ? [
        { left: 75, right: gridRight, top: 56, height: '44%' },
        { left: 75, right: gridRight, top: '50%', height: '11%' },
        { left: 75, right: gridRight, top: '62%', height: '11%' },
        { left: 75, right: gridRight, top: '74%', height: '11%' },
        { left: 75, right: gridRight, top: '86%', height: '14%' },
      ] : [
        { left: 75, right: gridRight, top: 56, height: 240 },
        { left: 75, right: gridRight, top: 322, height: 50 },
        { left: 75, right: gridRight, top: 380, height: 50 },
        { left: 75, right: gridRight, top: 438, height: 50 },
        { left: 75, right: gridRight, top: 496, height: 58 },
      ],
      xAxis: [
        { type: 'category', data: dates, boundaryGap: true, max: xAxisMax, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10, interval: xAxisLabelInterval }, splitLine: { show: false }, gridIndex: 0, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, max: xAxisMax, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 1, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, max: xAxisMax, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 2, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, max: xAxisMax, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { show: false }, splitLine: { show: false }, gridIndex: 3, axisPointer: { label: { show: false } } },
        { type: 'category', data: dates, boundaryGap: true, max: xAxisMax, axisLine: { onZero: false, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisLabel: { color: '#94a3b8', fontSize: 10, interval: xAxisLabelInterval, margin: 6 }, splitLine: { show: false }, gridIndex: 4, axisPointer: { label: { show: true, backgroundColor: '#3b82f6' } } },
      ],
      yAxis: [
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 0, position: 'left', axisLabel: { fontSize: 10, color: '#94a3b8', margin: 10 }, splitNumber: 5, name: '股价', nameLocation: 'end', nameRotate: 0, nameGap: 8, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right', verticalAlign: 'top', lineHeight: 14 }, axisPointer: { label: { show: true, formatter: (params: any) => fmt2(params.value) } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 1, position: 'left', axisLabel: { show: false }, splitNumber: 2, name: '换手', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt2(params.value) + '%' } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 2, position: 'left', axisLabel: { show: false }, splitNumber: 3, name: 'MACD', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt3(params.value) } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, min: 0, max: 100, gridIndex: 3, position: 'left', axisLabel: { show: false }, splitNumber: 2, name: 'RSI', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt1(params.value) } } },
        { scale: true, splitArea: { show: false }, splitLine: { lineStyle: { color: 'rgba(148, 163, 184, 0.08)' } }, gridIndex: 4, position: 'left', axisLabel: { show: false }, splitNumber: 3, name: 'BOLL', nameLocation: 'middle', nameRotate: 0, nameGap: 32, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'right' }, axisPointer: { label: { show: true, formatter: (params: any) => fmt2(params.value) } } },
        { scale: true, splitArea: { show: false }, splitLine: { show: false }, gridIndex: 0, position: 'right', splitNumber: 6, axisLabel: { show: true, inside: false, align: 'left', fontSize: 12, fontWeight: 600, color: indexSeries[0]?.color || '#94a3b8', margin: 8, formatter: (value: any) => fmt2(value) }, axisLine: { show: true, lineStyle: { color: 'rgba(148,163,184,0.2)' } }, axisPointer: { label: { show: true, formatter: (params: any) => fmt2(params.value) } }, name: '指数', nameLocation: 'end', nameRotate: 0, nameGap: 8, nameTextStyle: { color: '#94a3b8', fontSize: 11, align: 'left', verticalAlign: 'top', lineHeight: 14 } },
      ],
      series: [
        {
          name: 'K线',
          type: 'candlestick',
          data: candleData,
          itemStyle: {
            color: colors.up,
            color0: colors.down,
            borderColor: colors.up,
            borderColor0: colors.down,
          },
          xAxisIndex: 0,
          yAxisIndex: 0,
          cursor: 'default',
          markPoint: {
            symbol: 'circle',
            symbolSize: 8,
            label: { fontSize: 10, fontWeight: 600, formatter: (p: any) => fmt2(p.value) },
            data: [
              { type: 'max' as const, valueDim: 'highest', name: '最高', itemStyle: { color: colors.up }, label: { position: 'top', color: colors.up } },
              { type: 'min' as const, valueDim: 'lowest', name: '最低', itemStyle: { color: colors.down }, label: { position: 'bottom', color: colors.down } },
            ],
          },
        },
        {
          name: '换手率',
          type: 'bar',
          data: turnoverData,
          xAxisIndex: 1,
          yAxisIndex: 1,
          cursor: 'default',
        },
        ...mas.map(m => ({
          name: m.name,
          type: 'line' as const,
          data: m.data,
          smooth: false,
          lineStyle: { color: m.color, width: 1.5 },
          symbol: 'none',
          xAxisIndex: 0,
          yAxisIndex: 0,
          cursor: 'default' as const,
        })),
        { name: 'DIF', type: 'line', data: dif, smooth: true, lineStyle: { color: colors.macd }, symbol: 'none', xAxisIndex: 2, yAxisIndex: 2, cursor: 'default' },
        { name: 'DEA', type: 'line', data: dea, smooth: true, lineStyle: { color: colors.signal }, symbol: 'none', xAxisIndex: 2, yAxisIndex: 2, cursor: 'default' },
        {
          name: 'MACD', type: 'bar', data: hist.map(v => typeof v === 'number' ? {
            value: v,
            itemStyle: { color: v >= 0 ? colors.histPositive : colors.histNegative },
          } : '-'),
          xAxisIndex: 2, yAxisIndex: 2, cursor: 'default',
        },
        { name: 'RSI6', type: 'line', data: rsi6, smooth: true, lineStyle: { color: colors.rsi6, width: 1.5 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3, connectNulls: false, cursor: 'default' },
        { name: 'RSI12', type: 'line', data: rsi12, smooth: true, lineStyle: { color: colors.rsi12, width: 1.5 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3, connectNulls: false, cursor: 'default' },
        { name: 'RSI24', type: 'line', data: rsi24, smooth: true, lineStyle: { color: colors.rsi24, width: 1.5 }, symbol: 'none', xAxisIndex: 3, yAxisIndex: 3, connectNulls: false, cursor: 'default' },
        { name: '上轨', type: 'line', data: bbUpper, smooth: true, lineStyle: { color: colors.bbUpper }, symbol: 'none', xAxisIndex: 4, yAxisIndex: 4, connectNulls: false, cursor: 'default' },
        { name: '中轨', type: 'line', data: bbMid, smooth: true, lineStyle: { color: colors.bbMid, width: 2 }, symbol: 'none', xAxisIndex: 4, yAxisIndex: 4, connectNulls: false, cursor: 'default' },
        { name: '下轨', type: 'line', data: bbLower, smooth: true, lineStyle: { color: colors.bbLower }, symbol: 'none', xAxisIndex: 4, yAxisIndex: 4, connectNulls: false, cursor: 'default' },
        ...indexSeries.map(s => ({
          name: s.name,
          type: 'line' as const,
          data: s.data,
          smooth: true,
          lineStyle: { color: s.color, width: 2, type: 'dotted' as const },
          itemStyle: { color: s.color },
          symbol: 'none',
          connectNulls: true,
          z: 10,
          xAxisIndex: 0,
          yAxisIndex: 5,
          cursor: 'default' as const,
          endLabel: {
            show: true,
            formatter: '{a}',
            color: s.color,
            fontSize: 11,
            offset: [12, 0],
            backgroundColor: isLightTheme ? 'rgba(255,255,255,0.7)' : 'rgba(15,23,42,0.7)',
            padding: [2, 4],
            borderRadius: 3,
          },
        })),
      ],
    }

    chart.setOption(option, true)
    // isExpanded 切换时容器尺寸变化（fixed 全屏 vs flex item），需要 resize 触发 echarts 重测画布
    chart.resize()
  }, [data, isExpanded, maConfig, indexSeries, isLightTheme])

  const fullscreenBg = isLightTheme ? '#f8fafc' : '#0f172a'
  const btnBg = isLightTheme ? 'rgba(255,255,255,0.9)' : 'rgba(30,41,59,0.9)'
  const btnText = isLightTheme ? '#1f2937' : '#e2e8f0'
  const hintText = isLightTheme ? '#94a3b8' : '#64748b'

  useEffect(() => {
    if (!isExpanded) return
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsExpanded(false)
        onClose?.()
      }
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [isExpanded, onClose])

  // 用户主动触发：绕过缓存重拉。早期版本写入的缓存可能只有几百条，导致 dataZoom 拖不到上市初期。
  const handleRefresh = async () => {
    if (!code || refreshing) return
    setRefreshing(true)
    try {
      const list = await RefreshStockKlines(code, 'daily')
      // 与 GetStockKlines 路径保持一致：统一日期格式，否则刷新后切到周线会因
      // aggregateToWeekly 严格要求 'YYYY-MM-DD' 而把腾讯 'YYYYMMDD' / 网易 'YYYY/MM/DD' 全部跳过。
      const normalized = (list || []).map((d) => ({
        ...d,
        time: normalizeTime(d.time),
      }))
      // 标记等待 chart 'finished' 事件再解除遮罩，避免 zrender 还没画完就露出残留
      refreshPendingRef.current = true
      setRawData(normalized)
      // 兜底：5s 后强制解除，防止 finished 事件因某种原因未触发导致按钮永久卡住
      setTimeout(() => {
        if (refreshPendingRef.current) {
          refreshPendingRef.current = false
          setRefreshing(false)
        }
      }, 5000)
    } catch (e) {
      console.error('刷新K线失败:', e)
      refreshPendingRef.current = false
      setRefreshing(false)
    }
  }

  // MA 配置更新
  const handleMACountChange = (count: number) => {
    const newConfig = {
      count,
      periods: maConfig.periods.slice(0, count).concat(
        Array(Math.max(0, count - maConfig.periods.length)).fill(5)
          .map((_, i) => defaultMAConfig.periods[i] || 5)
      ),
    }
    setMAConfig(newConfig)
    saveMAConfig(newConfig)
  }

  const handleMAPeriodChange = (index: number, value: number) => {
    const newPeriods = [...maConfig.periods]
    newPeriods[index] = value
    const newConfig = { ...maConfig, periods: newPeriods }
    setMAConfig(newConfig)
    saveMAConfig(newConfig)
  }

  // 外层包装:isExpanded=true 时完全脱离正常布局(0×0 fixed),避免在 App.tsx 顶层挂载时
  // 作为 flex item 挤压中栏宽度;子元素仍以自身 fixed 定位铺满 viewport。
  const outerStyle: CSSProperties = isExpanded
    ? { position: 'fixed', top: 0, left: 0, width: 0, height: 0, overflow: 'visible', zIndex: 9999 }
    : { width: '100%', height: '560px', position: 'relative' }

  const overlayBg = isLightTheme ? 'rgba(243, 244, 246, 0.85)' : 'rgba(15, 23, 42, 0.85)'

  return (
    <div style={outerStyle}>
      <div style={{
        width: isExpanded ? '100vw' : '100%',
        height: isExpanded ? '100vh' : '100%',
        position: isExpanded ? 'fixed' : 'relative',
        top: 0, left: 0,
        zIndex: isExpanded ? 9999 : 1,
        backgroundColor: isExpanded ? fullscreenBg : 'transparent',
      }} data-chart-panel>
        {/* 左上角：提示文字 + 刷新按钮 + 周期选择 + 股票名称/代码 */}
        <div style={{
          position: 'absolute', top: 12, left: 12, zIndex: 10000,
          display: 'flex', alignItems: 'center', gap: 12,
        }}>
          <span style={{ color: hintText, fontSize: 11, pointerEvents: 'none' }}>
            {isExpanded ? '双击 / Esc 关闭' : '双击扩展'}
          </span>
          <button onClick={handleRefresh} disabled={refreshing} title="重新拉取全量历史K线（绕过缓存）" style={{
            padding: '4px 10px', borderRadius: 4,
            border: '1px solid rgba(148,163,184,0.3)',
            background: btnBg, color: btnText,
            fontSize: 12, cursor: refreshing ? 'wait' : 'pointer',
            opacity: refreshing ? 0.6 : 1,
          }}>
            {refreshing ? '刷新中…' : '刷新'}
          </button>

          {/* 周期选择器 */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
            {([
              { value: 'intraday', label: '分时' },
              { value: 'daily', label: '日线' },
              { value: 'weekly', label: '周线' },
              { value: 'monthly', label: '月线' },
            ] as const).map((p) => {
              const active = period === p.value
              return (
                <button
                  key={p.value}
                  ref={p.value === 'intraday' ? intradayBtnRef : undefined}
                  onClick={() => setPeriod(p.value)}
                  style={{
                    padding: '3px 10px',
                    borderRadius: 4,
                    border: '1px solid',
                    borderColor: active ? '#3b82f6' : 'rgba(148,163,184,0.3)',
                    background: active ? '#3b82f6' : btnBg,
                    color: active ? '#fff' : btnText,
                    fontSize: 12,
                    cursor: 'pointer',
                  }}
                >
                  {p.label}
                </button>
              )
            })}
          </div>

          {/* 大盘指数叠加开关 */}
          {period !== 'intraday' && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 8, marginLeft: 12 }}>
              {INDEX_CONFIGS.map((cfg) => {
                const active = !!selectedIndices[cfg.code]
                return (
                  <button
                    key={cfg.code}
                    onClick={() => setSelectedIndices(prev => {
                      // 排他单选：再次点击已选中项则取消，否则只选中当前项
                      if (prev[cfg.code]) return {}
                      return { [cfg.code]: true }
                    })}
                    title={`${cfg.label}（右侧纵轴真实点位，单选）`}
                    style={{
                      display: 'flex', alignItems: 'center', gap: 5,
                      padding: '3px 8px 3px 6px',
                      borderRadius: 999,
                      border: '1px solid',
                      borderColor: active ? cfg.color : 'rgba(148,163,184,0.25)',
                      background: active ? `${cfg.color}22` : btnBg,
                      color: active ? cfg.color : btnText,
                      fontSize: 11,
                      cursor: 'pointer',
                      opacity: active ? 1 : 0.8,
                    }}
                  >
                    <span style={{
                      width: 10, height: 10, borderRadius: '50%',
                      background: cfg.color,
                      boxShadow: active ? `0 0 0 2px ${cfg.color}44` : 'none',
                    }} />
                    <span>{cfg.label}</span>
                  </button>
                )
              })}
            </div>
          )}

          {/* 股票名称/代码 */}
          <span style={{ color: btnText, fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap', pointerEvents: 'none' }}>
            {name ? `${name} (${code})` : code}
          </span>
        </div>

        {/* K线图内部上方：均线数值标签 */}
        {period !== 'intraday' && (
          <div ref={maRowRef} style={{
            position: 'absolute', top: 64, zIndex: 9999,
            display: 'flex', alignItems: 'center', gap: 8,
            pointerEvents: 'none',
          }}>
            <span style={{ color: hintText, fontSize: 11 }}>均线</span>
            {indicators.mas.map((m) => {
              const last = [...m.data].reverse().find(v => v != null)
              return (
                <span key={m.name} style={{ color: m.color, fontSize: 11, whiteSpace: 'nowrap' }}>
                  {m.name}:{fmt2(last)}
                </span>
              )
            })}
          </div>
        )}

        {/* 右上角：设置按钮 */}
        <div style={{
          position: 'absolute', top: 6, right: 8, zIndex: 10001,
          display: 'flex', alignItems: 'center', gap: 6,
        }}>
          <button
            onClick={() => setShowSettings(true)}
            title="均线设置"
            style={{
              padding: '6px 10px', borderRadius: 4,
              border: '1px solid rgba(148,163,184,0.3)',
              background: btnBg, color: btnText,
              fontSize: 18, cursor: 'pointer',
              lineHeight: 1,
            }}
          >
            ⚙
          </button>
        </div>

        {/* 设置弹窗 */}
        {showSettings && (
          <div style={{
            position: 'fixed', inset: 0, zIndex: 10002,
            display: 'flex', justifyContent: 'center', alignItems: 'center',
            backgroundColor: 'rgba(0,0,0,0.5)',
          }} onClick={() => setShowSettings(false)}>
            <div
              style={{
                width: 320,
                padding: 20,
                borderRadius: 8,
                background: isLightTheme ? '#fff' : '#1e293b',
                border: `1px solid ${isLightTheme ? '#e2e8f0' : '#334155'}`,
                color: isLightTheme ? '#1f2937' : '#e2e8f0',
              }}
              onClick={(e) => e.stopPropagation()}
            >
              <div style={{ fontSize: 15, fontWeight: 600, marginBottom: 16 }}>均线设置</div>

              {/* 均线数量 */}
              <div style={{ marginBottom: 16 }}>
                <div style={{ fontSize: 13, marginBottom: 8, color: isLightTheme ? '#64748b' : '#94a3b8' }}>
                  均线数量（1~6）
                </div>
                <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap' }}>
                  {[1, 2, 3, 4, 5, 6].map((n) => (
                    <button
                      key={n}
                      onClick={() => handleMACountChange(n)}
                      style={{
                        padding: '4px 10px',
                        borderRadius: 4,
                        border: '1px solid',
                        borderColor: maConfig.count === n
                          ? '#3b82f6'
                          : isLightTheme ? '#e2e8f0' : '#475569',
                        background: maConfig.count === n ? '#3b82f6' : 'transparent',
                        color: maConfig.count === n ? '#fff' : 'inherit',
                        fontSize: 12,
                        cursor: 'pointer',
                      }}
                    >
                      {n}条
                    </button>
                  ))}
                </div>
              </div>

              {/* 每条均线周期 */}
              <div style={{ marginBottom: 16 }}>
                <div style={{ fontSize: 13, marginBottom: 8, color: isLightTheme ? '#64748b' : '#94a3b8' }}>
                  均线周期（1~250）
                </div>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  {Array.from({ length: maConfig.count }).map((_, i) => (
                    <div key={i} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{
                        width: 10, height: 10, borderRadius: '50%',
                        background: maColors[i % maColors.length],
                        flexShrink: 0,
                      }} />
                      <span style={{ fontSize: 12, width: 40 }}>MA{i + 1}</span>
                      <input
                        type="number"
                        min={1}
                        max={250}
                        value={maConfig.periods[i] || 5}
                        onChange={(e) => {
                          const v = parseInt(e.target.value, 10)
                          if (!isNaN(v)) {
                            handleMAPeriodChange(i, Math.min(250, Math.max(1, v)))
                          }
                        }}
                        style={{
                          width: 70,
                          padding: '4px 8px',
                          borderRadius: 4,
                          border: `1px solid ${isLightTheme ? '#e2e8f0' : '#475569'}`,
                          background: isLightTheme ? '#f8fafc' : '#0f172a',
                          color: 'inherit',
                          fontSize: 12,
                          outline: 'none',
                        }}
                      />
                    </div>
                  ))}
                </div>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
                <button
                  onClick={() => {
                    setMAConfig({ ...defaultMAConfig })
                    saveMAConfig({ ...defaultMAConfig })
                  }}
                  style={{
                    padding: '5px 12px', borderRadius: 4,
                    border: '1px solid rgba(148,163,184,0.3)',
                    background: 'transparent', color: 'inherit',
                    fontSize: 12, cursor: 'pointer',
                  }}
                >
                  恢复默认
                </button>
                <button
                  onClick={() => setShowSettings(false)}
                  style={{
                    padding: '5px 12px', borderRadius: 4,
                    border: '1px solid #3b82f6',
                    background: '#3b82f6', color: '#fff',
                    fontSize: 12, cursor: 'pointer',
                  }}
                >
                  确定
                </button>
              </div>
            </div>
          </div>
        )}

        {/* K 线容器：始终挂载，避免分时切换时 dispose/init 导致 echarts 实例脱离 DOM；
            切到分时时用 visibility 隐藏（visibility 不影响布局也不触发回流，比 display:none 安全）。 */}
        <div ref={chartRef} className="unified-chart-container" style={{
          width: '100%', height: '100%',
          visibility: period === 'intraday' || loading || refreshing || data.length === 0 ? 'hidden' : 'visible',
        }} />

        {/* K 线模式下的状态覆盖层（加载中 / 无数据） */}
        {period !== 'intraday' && (loading || data.length === 0) && (
          <div style={{
            position: 'absolute', inset: 0, zIndex: 100,
            display: 'flex', justifyContent: 'center', alignItems: 'center',
            backgroundColor: isExpanded ? fullscreenBg : overlayBg,
            color: '#64748b', fontSize: 14,
          }}>
            {loading ? '加载图表数据中...' : '暂无K线数据'}
          </div>
        )}

        {/* 分时图：绝对定位覆盖在 K 线之上；切回 K 线时整个卸载，IntradayChart 自己的 echarts 实例随 unmount 自动 dispose。 */}
        {period === 'intraday' && (
          <div style={{
            position: 'absolute', inset: 0, zIndex: 50, paddingTop: 40,
            background: isLightTheme ? '#f8fafc' : '#0f172a',
          }}>
            <IntradayChart
              code={code}
              isLightTheme={isLightTheme}
              onDoubleClick={() => {
                setIsExpanded(prev => {
                  const next = !prev
                  if (!next) onCloseRef.current?.()
                  return next
                })
              }}
            />
          </div>
        )}
      </div>

      {/* 刷新遮罩：通过 Portal 挂到 document.body，z=99999 高于所有元素（包括 echarts 自带的 tooltip/zr 辅助 DOM 与可能的 Wails/macOS 系统级覆盖）。
          由 chart 'finished' 事件触发解除，确保完整盖住整个 setOption 重绘过程。 */}
      {refreshing && createPortal(
        <div style={{
          position: 'fixed', inset: 0, zIndex: 99999,
          display: 'flex', justifyContent: 'center', alignItems: 'center',
          backgroundColor: isLightTheme ? '#f8fafc' : '#0f172a',
          color: '#64748b', fontSize: 14,
        }}>
          刷新中…
        </div>,
        document.body
      )}
    </div>
  )
}
