import { useEffect, useMemo, useRef, useState } from 'react'
import * as echarts from 'echarts'
import type { main } from '../wailsjs/go/models'
import { GetFinancialTrends } from './api'

interface Props {
  code: string
  name?: string
  onClose: () => void
}

function useEscClose(onClose: () => void) {
  useEffect(() => {
    const handle = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handle)
    return () => document.removeEventListener('keydown', handle)
  }, [onClose])
}

type MetricKey = 'roe' | 'grossMargin' | 'revenueGrowth' | 'cashContent' | 'debtRatio'

interface MetricConfig {
  key: MetricKey
  label: string
  color: string
}

const METRICS: MetricConfig[] = [
  { key: 'roe', label: 'ROE', color: '#3b82f6' },
  { key: 'grossMargin', label: '毛利率', color: '#10b981' },
  { key: 'revenueGrowth', label: '营收增长', color: '#f59e0b' },
  { key: 'cashContent', label: '现金含量', color: '#8b5cf6' },
  { key: 'debtRatio', label: '负债率', color: '#ef4444' },
]

export function FinancialTrendDrawer({ code, name, onClose }: Props) {
  useEscClose(onClose)
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)
  const [data, setData] = useState<main.FinancialTrendsData | null>(null)
  const [loading, setLoading] = useState(false)
  const [activeKeys, setActiveKeys] = useState<MetricKey[]>(['roe', 'grossMargin'])

  // 加载数据
  useEffect(() => {
    if (!code) return
    setLoading(true)
    GetFinancialTrends(code)
      .then((res) => setData(res || null))
      .catch(() => setData(null))
      .finally(() => setLoading(false))
  }, [code])

  // 初始化 ECharts（只执行一次）
  useEffect(() => {
    const el = chartRef.current
    if (!el) return
    const instance = echarts.init(el, undefined, { renderer: 'canvas' })
    chartInstanceRef.current = instance
    const timers = [60, 200].map((ms) => setTimeout(() => instance.resize(), ms))
    const handleResize = () => instance.resize()
    window.addEventListener('resize', handleResize)
    return () => {
      timers.forEach(clearTimeout)
      window.removeEventListener('resize', handleResize)
      instance.dispose()
      chartInstanceRef.current = null
    }
  }, [])

  // 更新图表
  useEffect(() => {
    const instance = chartInstanceRef.current
    if (!instance || !data?.items?.length) return

    const items = [...data.items].reverse()
    const years = items.map((i) => i.year)

    const cashActive = activeKeys.includes('cashContent')
    const series = METRICS.filter((m) => activeKeys.includes(m.key)).map((m) => ({
      name: m.label,
      type: 'line' as const,
      smooth: true,
      symbol: 'circle',
      symbolSize: 6,
      yAxisIndex: m.key === 'cashContent' ? 1 : 0,
      lineStyle: { width: 3, color: m.color },
      itemStyle: { color: m.color },
      data: items.map((i) => {
        const v = (i as any)[m.key]
        return v != null ? Number(v) : null
      }),
    }))

    const option: echarts.EChartsOption = {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(15,23,42,0.95)',
        borderColor: 'rgba(148,163,184,0.2)',
        textStyle: { color: '#e2e8f0' },
        formatter: (params: any) => {
          let html = `<div style="font-weight:600;margin-bottom:4px;">${params[0]?.axisValue}年</div>`
          params.forEach((p: any) => {
            const val = p.value != null ? `${p.value.toFixed(2)}%` : '-'
            html += `<div style="display:flex;align-items:center;gap:6px;margin:2px 0;">
              <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${p.color};"></span>
              <span style="flex:1;">${p.seriesName}</span>
              <span style="font-weight:600;">${val}</span>
            </div>`
          })
          return html
        },
      },
      legend: { show: false },
      grid: { left: 48, right: cashActive ? 64 : 24, top: 24, bottom: 32 },
      xAxis: {
        type: 'category',
        data: years,
        axisLine: { lineStyle: { color: 'rgba(148,163,184,0.3)' } },
        axisLabel: { color: '#94a3b8' },
        axisTick: { show: false },
      },
      yAxis: [
        {
          type: 'value',
          axisLine: { show: false },
          axisLabel: { color: '#94a3b8', formatter: '{value}%' },
          splitLine: { lineStyle: { color: 'rgba(148,163,184,0.1)' } },
        },
        {
          type: 'value',
          position: 'right',
          show: cashActive,
          axisLine: { show: false },
          axisLabel: { color: '#8b5cf6', formatter: '{value}%' },
          splitLine: { show: false },
        },
      ],
      series,
    }

    instance.setOption(option, true)
    requestAnimationFrame(() => instance.resize())
  }, [data, activeKeys])

  const toggleMetric = (key: MetricKey) => {
    setActiveKeys((prev) => {
      if (prev.includes(key)) return prev.filter((k) => k !== key)
      return [...prev, key]
    })
  }

  const hasData = useMemo(() => (data?.items?.length || 0) > 0, [data])

  return (
    <div
      className="modal-overlay"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
      style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', padding: 16 }}
    >
      <div
        className="modal-content"
        onClick={(e) => e.stopPropagation()}
        style={{
          width: 'min(720px, 96vw)',
          maxWidth: 'none',
          maxHeight: 'min(560px, 90vh)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '14px 16px',
            borderBottom: '1px solid rgba(148,163,184,0.15)',
          }}
        >
          <h4 style={{ margin: 0, fontSize: 16, fontWeight: 600 }}>
            {name || code} 财务指标趋势
          </h4>
        </div>

        <div style={{ padding: '12px 16px 4px' }}>
          <div style={{ display: 'flex', flexDirection: 'row', flexWrap: 'nowrap', gap: 10 }}>
            {METRICS.map((m) => {
              const active = activeKeys.includes(m.key)
              return (
                <button
                  key={m.key}
                  onClick={() => toggleMetric(m.key)}
                  style={{
                    minWidth: 90,
                    padding: '6px 10px',
                    whiteSpace: 'nowrap',
                    borderRadius: 6,
                    border: '1px solid',
                    borderColor: active ? m.color : 'rgba(148,163,184,0.25)',
                    background: active ? `${m.color}20` : 'transparent',
                    color: active ? m.color : '#94a3b8',
                    fontSize: 12,
                    fontWeight: 500,
                    cursor: 'pointer',
                    transition: 'all .15s ease',
                  }}
                >
                  {m.label}
                </button>
              )
            })}
          </div>
        </div>

        <div style={{ position: 'relative', flex: 1, minHeight: 280, padding: '8px 8px 16px' }}>
          {/* chart 容器始终渲染，保证 ECharts ref 始终有效 */}
          <div ref={chartRef} style={{ width: '100%', height: 280 }} />

          {(loading || !hasData) && (
            <div
              style={{
                position: 'absolute',
                inset: 0,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: '#94a3b8',
                fontSize: 14,
                textAlign: 'center',
                background: 'rgba(20,27,36,0.6)',
                backdropFilter: 'blur(2px)',
                borderRadius: 8,
                margin: '8px 8px 16px',
              }}
            >
              {loading ? '加载中...' : (
                <span>
                  暂无财务数据
                  <br />
                  请先下载财报
                </span>
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
