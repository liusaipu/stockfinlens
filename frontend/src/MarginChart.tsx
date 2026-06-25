import { useEffect, useMemo, useRef, useState } from 'react'
import * as echarts from 'echarts'
import type { downloader } from '../wailsjs/go/models'
import { GetMarginHistory, GetStockKlines } from './api'

type KlineData = downloader.KlineData
type MarginData = downloader.MarginData

interface MarginChartProps {
  code: string
  style?: React.CSSProperties
}

const HALF_YEAR_TRADING_DAYS = 120

function useIsLightTheme(): boolean {
  const [isLight, setIsLight] = useState(() => document.body.classList.contains('light'))
  useEffect(() => {
    const observer = new MutationObserver(() => {
      setIsLight(document.body.classList.contains('light'))
    })
    observer.observe(document.body, { attributes: true, attributeFilter: ['class'] })
    return () => observer.disconnect()
  }, [])
  return isLight
}

export function MarginChart({ code, style }: MarginChartProps) {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstance = useRef<echarts.ECharts | null>(null)
  const [klines, setKlines] = useState<KlineData[]>([])
  const [marginData, setMarginData] = useState<MarginData[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const isLight = useIsLightTheme()

  useEffect(() => {
    if (!code) return
    let cancelled = false
    setLoading(true)
    setError(null)

    const load = async () => {
      let kRes: KlineData[] = []
      let mRes: MarginData[] = []
      let kErr: any = null
      let mErr: any = null

      try {
        kRes = await GetStockKlines(code, 'daily')
      } catch (e) { kErr = e }

      try {
        mRes = await GetMarginHistory(code, false)
      } catch (e) { mErr = e }

      if (cancelled) return
      setKlines(kRes || [])
      setMarginData(mRes || [])

      if (mErr) {
        setError(`融资融券数据加载失败: ${mErr?.message || mErr || '未知错误'}`)
      } else if (kErr) {
        setError(`K线数据加载失败: ${kErr?.message || kErr || '未知错误'}`)
      }

      setLoading(false)
    }

    load()
    return () => { cancelled = true }
  }, [code])

  const normalizeDate = (d: string): string => {
    if (!d) return ''
    const s = d.trim().replace(/\s.*$/, '')
    if (s.length === 8 && /^\d{8}$/.test(s)) {
      return `${s.slice(0, 4)}-${s.slice(4, 6)}-${s.slice(6, 8)}`
    }
    if (s.length === 10 && s[4] === '-' && s[7] === '-') {
      return s
    }
    return s
  }

  const merged = useMemo(() => {
    if (!klines?.length || !marginData?.length) return []
    const marginMap = new Map<string, MarginData>()
    for (const m of marginData) {
      const d = normalizeDate(m.date)
      if (d) marginMap.set(d, m)
    }
    return klines.map(k => ({
      ...k,
      margin: marginMap.get(normalizeDate(k.time)) || null,
    })).filter(k => k.margin !== null)
  }, [klines, marginData])

  useEffect(() => {
    if (!chartRef.current || loading || merged.length === 0) return

    if (!chartInstance.current) {
      chartInstance.current = echarts.init(chartRef.current)
    }
    const chart = chartInstance.current

    const theme = isLight
      ? {
          text: '#1f2937',
          textSecondary: '#4b5563',
          grid: '#e5e7eb',
          border: '#d1d5db',
          bg: '#ffffff',
          tooltipBg: 'rgba(255,255,255,0.95)',
          tooltipBorder: '#e5e7eb',
        }
      : {
          text: '#e2e8f0',
          textSecondary: '#94a3b8',
          grid: '#1e293b',
          border: '#334155',
          bg: '#0f1419',
          tooltipBg: 'rgba(15,23,42,0.95)',
          tooltipBorder: '#334155',
        }

    const dates = merged.map(d => d.time)
    const closeData = merged.map(d => d.close)
    const volumes = merged.map(d => ({
      value: d.volume,
      itemStyle: { color: d.close >= d.open ? '#ef4444' : '#22c55e' },
    }))
    const rzData = merged.map(d => d.margin?.rzye ?? null)
    const rqData = merged.map(d => d.margin?.rqye ?? null)

    // 默认显示最近半年（约 120 个交易日）
    const total = merged.length
    const defaultStart = total <= HALF_YEAR_TRADING_DAYS
      ? 0
      : Math.round((total - HALF_YEAR_TRADING_DAYS) / total * 100)

    const option: echarts.EChartsOption = {
      backgroundColor: theme.bg,
      animation: false,
      tooltip: {
        trigger: 'axis',
        axisPointer: { type: 'cross' },
        backgroundColor: theme.tooltipBg,
        borderColor: theme.tooltipBorder,
        textStyle: { color: theme.text },
        formatter: (params: any) => {
          if (!params?.length) return ''
          const idx = params[0].dataIndex
          const k = merged[idx]
          const m = k.margin
          const borderColor = isLight ? '#e5e7eb' : '#334155'
          const displayDate = normalizeDate(k.time)
          let html = `<div style="font-weight:600;margin-bottom:4px;color:${theme.text}">${displayDate}</div>`
          html += `<div style="color:${theme.textSecondary}">收盘 ${k.close.toFixed(2)}</div>`
          html += `<div style="color:${theme.textSecondary}">成交量 ${(k.volume / 10000).toFixed(2)} 万手</div>`
          if (m) {
            html += `<div style="margin-top:6px;border-top:1px solid ${borderColor};padding-top:4px">`
            html += `<div style="color:#f59e0b">融资余额 ${(m.rzye / 100000000).toFixed(2)} 亿</div>`
            html += `<div style="color:#3b82f6">融券余额 ${(m.rqye / 100000000).toFixed(4)} 亿</div>`
            html += `<div style="color:${theme.textSecondary}">融资买入 ${(m.rzmre / 100000000).toFixed(4)} 亿</div>`
            html += `<div style="color:${theme.textSecondary}">融资偿还 ${(m.rzche / 100000000).toFixed(4)} 亿</div>`
            html += `</div>`
          }
          return html
        },
      },
      axisPointer: {
        link: [{ xAxisIndex: 'all' }],
        label: { backgroundColor: isLight ? '#f3f4f6' : '#1e293b', color: theme.text },
      },
      grid: [
        { left: '6%', right: '8%', top: '6%', height: '60%' },
        { left: '6%', right: '8%', top: '68%', height: '28%' },
      ],
      xAxis: [
        {
          type: 'category',
          data: dates,
          boundaryGap: false,
          axisLine: { lineStyle: { color: theme.border } },
          axisLabel: { color: theme.textSecondary },
          splitLine: { show: false },
        },
        {
          type: 'category',
          gridIndex: 1,
          data: dates,
          boundaryGap: false,
          axisLine: { lineStyle: { color: theme.border } },
          axisLabel: { show: false },
          splitLine: { show: false },
        },
      ],
      yAxis: [
        {
          scale: true,
          position: 'left',
          name: '股价',
          nameTextStyle: { color: theme.textSecondary },
          axisLine: { lineStyle: { color: theme.border } },
          axisLabel: { color: theme.textSecondary },
          splitLine: { lineStyle: { color: theme.grid } },
        },
        {
          scale: true,
          position: 'right',
          name: '余额',
          nameTextStyle: { color: theme.textSecondary },
          axisLine: { lineStyle: { color: theme.border } },
          axisLabel: {
            color: theme.textSecondary,
            formatter: (v: number) => {
              if (v >= 1e8) return (v / 1e8).toFixed(0) + '亿'
              if (v >= 1e4) return (v / 1e4).toFixed(0) + '万'
              return v.toString()
            },
          },
          splitLine: { show: false },
        },
        {
          scale: true,
          gridIndex: 1,
          position: 'left',
          axisLine: { lineStyle: { color: theme.border } },
          axisLabel: { color: theme.textSecondary },
          splitLine: { lineStyle: { color: theme.grid } },
        },
      ],
      dataZoom: [
        { type: 'inside', xAxisIndex: [0, 1], start: defaultStart, end: 100 },
      ],
      series: [
        {
          name: '收盘价',
          type: 'line',
          data: closeData,
          smooth: false,
          showSymbol: false,
          lineStyle: { width: 1.5, color: '#22c55e' },
          itemStyle: { color: '#22c55e' },
        },
        {
          name: '融资余额',
          type: 'line',
          yAxisIndex: 1,
          data: rzData,
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: '#f59e0b' },
          itemStyle: { color: '#f59e0b' },
        },
        {
          name: '融券余额',
          type: 'line',
          yAxisIndex: 1,
          data: rqData,
          smooth: true,
          showSymbol: false,
          lineStyle: { width: 2, color: '#3b82f6' },
          itemStyle: { color: '#3b82f6' },
        },
        {
          name: '成交量',
          type: 'bar',
          xAxisIndex: 1,
          yAxisIndex: 2,
          data: volumes,
          itemStyle: { color: '#64748b' },
        },
      ],
      legend: {
        data: ['收盘价', '融资余额', '融券余额', '成交量'],
        textStyle: { color: theme.textSecondary },
        top: 2,
      },
    }

    chart.setOption(option, true)

    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)
    return () => {
      window.removeEventListener('resize', handleResize)
      chart.dispose()
      chartInstance.current = null
    }
  }, [merged, loading, code, isLight])

  const emptyColor = isLight ? '#6b7280' : '#94a3b8'
  const errorColor = '#ef4444'

  if (loading) {
    return (
      <div style={{ ...style, display: 'flex', alignItems: 'center', justifyContent: 'center', color: emptyColor }}>
        正在加载融资融券数据...
      </div>
    )
  }

  if (error) {
    return (
      <div style={{ ...style, display: 'flex', alignItems: 'center', justifyContent: 'center', color: errorColor }}>
        {error}
      </div>
    )
  }

  if (merged.length === 0) {
    return (
      <div style={{ ...style, display: 'flex', alignItems: 'center', justifyContent: 'center', color: emptyColor }}>
        该股票暂无融资融券数据
      </div>
    )
  }

  return (
    <div style={{ position: 'relative', width: '100%', height: '100%', ...style }}>
      <div ref={chartRef} style={{ width: '100%', height: '100%' }} />
      <div
        style={{
          position: 'absolute',
          left: 'calc(6% + 4px)',
          top: 'calc(68% + 4px)',
          fontSize: 11,
          color: emptyColor,
          pointerEvents: 'none',
          userSelect: 'none',
        }}
      >
        成交量
      </div>
    </div>
  )
}
