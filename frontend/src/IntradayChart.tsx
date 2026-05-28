import { useEffect, useMemo, useRef, useState } from 'react'
import * as echarts from 'echarts'
import { GetIntradayMinutes } from './api'
import type { downloader } from '../wailsjs/go/models'

type IntradayData = downloader.IntradayData

interface Props {
  code: string
  isLightTheme?: boolean
  onDoubleClick?: () => void
}

// 生成 A 股交易日完整 240 个时间 slot（09:30-11:30 + 13:00-15:00）
function buildFullSlots(): string[] {
  const slots: string[] = []
  // 上午 09:30 - 11:30 共 121 分钟，但 09:30 是开盘点也算一根，共 121 个点中常见接口给 121 根
  // 实测东财从 09:30 算起到 11:30 含两端共 121 个点；下午 13:00-15:00 含两端 121 个点；总共 242
  // 这里按 9:30-11:30 含两端 + 13:00-15:00 含两端 = 242 个 slot
  const push = (h: number, m: number) => slots.push(`${String(h).padStart(2, '0')}:${String(m).padStart(2, '0')}`)
  for (let m = 0; m <= 120; m++) {
    const h = 9 + Math.floor((30 + m) / 60)
    const mm = (30 + m) % 60
    push(h, mm)
  }
  for (let m = 0; m <= 120; m++) {
    const h = 13 + Math.floor(m / 60)
    const mm = m % 60
    push(h, mm)
  }
  return slots
}

// 简易判断：客户端时间是否处于 A 股交易时段
function isLocalTradingHours(): boolean {
  const now = new Date()
  const day = now.getDay()
  if (day === 0 || day === 6) return false
  const mins = now.getHours() * 60 + now.getMinutes()
  return (mins >= 9 * 60 + 30 && mins < 11 * 60 + 30) || (mins >= 13 * 60 && mins < 15 * 60)
}

export function IntradayChart({ code, isLightTheme = false, onDoubleClick }: Props) {
  const chartDomRef = useRef<HTMLDivElement | null>(null)
  const chartRef = useRef<echarts.ECharts | null>(null)
  const [data, setData] = useState<IntradayData | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [lastFetchedAt, setLastFetchedAt] = useState<Date | null>(null)

  const slots = useMemo(() => buildFullSlots(), [])

  // 数据加载（初次 + code 变化）
  useEffect(() => {
    if (!code) return
    let cancelled = false
    setLoading(true)
    setError(null)
    GetIntradayMinutes(code)
      .then((d: IntradayData) => {
        if (cancelled) return
        setData(d)
        setLastFetchedAt(new Date())
      })
      .catch((e: any) => {
        if (cancelled) return
        setError(String(e?.message || e))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [code])

  // 自动刷新：仅当数据为实时且页面可见且本地仍在交易时段
  useEffect(() => {
    if (!code || !data?.isRealtime) return
    const fetch = () => {
      if (document.visibilityState !== 'visible') return
      if (!isLocalTradingHours()) return
      GetIntradayMinutes(code)
        .then((d: IntradayData) => {
          setData(d)
          setLastFetchedAt(new Date())
        })
        .catch(() => {
          // 静默失败：保持上一次数据展示，避免一次抖动就清屏
        })
    }
    const handle = window.setInterval(fetch, 60_000)
    const onVis = () => {
      // 页面切回前台时立刻刷一次
      if (document.visibilityState === 'visible' && isLocalTradingHours()) {
        fetch()
      }
    }
    document.addEventListener('visibilitychange', onVis)
    return () => {
      window.clearInterval(handle)
      document.removeEventListener('visibilitychange', onVis)
    }
  }, [code, data?.isRealtime])

  // 用 ref 稳定 onDoubleClick，避免 chart init useEffect 因 prop 变化而重建实例
  const onDoubleClickRef = useRef(onDoubleClick)
  useEffect(() => { onDoubleClickRef.current = onDoubleClick }, [onDoubleClick])

  // ECharts 实例初始化（只 mount 一次）
  useEffect(() => {
    if (!chartDomRef.current) return
    const chart = echarts.init(chartDomRef.current)
    chartRef.current = chart
    chart.getZr().on('dblclick', () => {
      onDoubleClickRef.current?.()
    })
    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)
    return () => {
      window.removeEventListener('resize', handleResize)
      chart.dispose()
      chartRef.current = null
    }
  }, [])

  // 数据变化时 setOption（热更新，不重建实例）
  useEffect(() => {
    if (!chartRef.current || !data) return
    const prevClose = data.prevClose
    const pointsByTime: Record<string, { price: number; avg: number; vol: number }> = {}
    for (const p of data.points) {
      pointsByTime[p.time] = { price: p.price, avg: p.avgPx, vol: p.volume }
    }
    const priceArr: (number | null)[] = []
    const avgArr: (number | null)[] = []
    const volArr: (number | null)[] = []
    let lastSeenPrice: number | null = null
    for (const slot of slots) {
      const p = pointsByTime[slot]
      if (p) {
        priceArr.push(p.price)
        avgArr.push(p.avg)
        volArr.push(p.vol)
        lastSeenPrice = p.price
      } else {
        // 已经开过盘但当前 slot 没数据：可能是午休/盘后空白 → 给 null（不连线）
        // 还没开盘到的 slot 也给 null
        priceArr.push(null)
        avgArr.push(null)
        volArr.push(null)
      }
    }
    void lastSeenPrice

    const textColor = isLightTheme ? '#1f2937' : '#e2e8f0'
    const subTextColor = isLightTheme ? '#64748b' : '#94a3b8'
    const gridColor = isLightTheme ? '#e5e7eb' : '#334155'
    const refLineColor = isLightTheme ? '#94a3b8' : '#64748b'

    // 颜色：当前价 vs 昨收
    const lastPrice = data.points.length > 0 ? data.points[data.points.length - 1].price : prevClose
    const upColor = '#ef4444' // 涨红
    const downColor = '#10b981' // 跌绿
    const priceLineColor = lastPrice >= prevClose ? upColor : downColor

    // 价格轴范围：取 |max-prevClose| 与 |min-prevClose| 中较大者，对称
    const validPrices = priceArr.filter((v): v is number => v != null)
    let yMin = prevClose * 0.99
    let yMax = prevClose * 1.01
    if (validPrices.length > 0) {
      const minP = Math.min(...validPrices, prevClose)
      const maxP = Math.max(...validPrices, prevClose)
      const range = Math.max(prevClose - minP, maxP - prevClose) * 1.1 || prevClose * 0.005
      yMin = prevClose - range
      yMax = prevClose + range
    }

    const option: echarts.EChartsOption = {
      animation: false,
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: isLightTheme ? '#fff' : '#1e293b',
        borderColor: gridColor,
        textStyle: { color: textColor },
        formatter: (params: any) => {
          if (!Array.isArray(params) || params.length === 0) return ''
          const idx = params[0].dataIndex
          const slot = slots[idx]
          const price = priceArr[idx]
          const avg = avgArr[idx]
          const vol = volArr[idx]
          if (price == null) return `${slot}<br/>暂无数据`
          const pct = (((price as number) - prevClose) / prevClose) * 100
          const pctColor = price >= prevClose ? upColor : downColor
          return [
            `<b>${slot}</b>`,
            `价格: <span style="color:${pctColor}">${(price as number).toFixed(2)} (${pct >= 0 ? '+' : ''}${pct.toFixed(2)}%)</span>`,
            avg != null ? `均价: ${(avg as number).toFixed(2)}` : '',
            vol != null ? `成交量: ${((vol as number) / 10000).toFixed(2)} 万股` : '',
          ].filter(Boolean).join('<br/>')
        },
      },
      axisPointer: { link: [{ xAxisIndex: 'all' }] },
      grid: [
        { left: 56, right: 56, top: 28, height: '62%' },
        { left: 56, right: 56, top: '74%', height: '20%' },
      ],
      xAxis: [
        {
          type: 'category',
          data: slots,
          gridIndex: 0,
          axisLine: { lineStyle: { color: gridColor } },
          axisLabel: { show: false },
          axisTick: { show: false },
          splitLine: {
            show: true,
            interval: (i: number) => i === 120, // 午休分隔
            lineStyle: { color: gridColor, type: 'dashed' },
          },
        },
        {
          type: 'category',
          data: slots,
          gridIndex: 1,
          axisLine: { lineStyle: { color: gridColor } },
          axisLabel: {
            color: subTextColor,
            fontSize: 10,
            interval: 30, // 每 30 分钟一个标签
          },
          axisTick: { show: false },
          splitLine: {
            show: true,
            interval: (i: number) => i === 120,
            lineStyle: { color: gridColor, type: 'dashed' },
          },
        },
      ],
      yAxis: [
        {
          type: 'value',
          gridIndex: 0,
          position: 'left',
          min: yMin,
          max: yMax,
          axisLine: { lineStyle: { color: gridColor } },
          axisLabel: {
            color: subTextColor,
            fontSize: 10,
            formatter: (v: number) => v.toFixed(2),
          },
          splitLine: { lineStyle: { color: gridColor, type: 'dashed' } },
        },
        {
          type: 'value',
          gridIndex: 0,
          position: 'right',
          min: ((yMin - prevClose) / prevClose) * 100,
          max: ((yMax - prevClose) / prevClose) * 100,
          axisLine: { lineStyle: { color: gridColor } },
          axisLabel: {
            color: subTextColor,
            fontSize: 10,
            formatter: (v: number) => `${v >= 0 ? '+' : ''}${v.toFixed(2)}%`,
          },
          splitLine: { show: false },
        },
        {
          type: 'value',
          gridIndex: 1,
          axisLine: { lineStyle: { color: gridColor } },
          axisLabel: {
            color: subTextColor,
            fontSize: 10,
            formatter: (v: number) => `${(v / 10000).toFixed(0)}万`,
          },
          splitLine: { show: false },
        },
      ],
      series: [
        {
          name: '价格',
          type: 'line',
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: priceArr,
          showSymbol: false,
          smooth: false,
          lineStyle: { color: priceLineColor, width: 1.5 },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: priceLineColor + '44' },
              { offset: 1, color: priceLineColor + '00' },
            ]),
          },
          markLine: {
            silent: true,
            symbol: 'none',
            lineStyle: { color: refLineColor, type: 'dashed', width: 1 },
            label: {
              formatter: `昨收 ${prevClose.toFixed(2)}`,
              position: 'insideEndTop',
              color: subTextColor,
              fontSize: 10,
            },
            data: [{ yAxis: prevClose }],
          },
          connectNulls: false,
        },
        {
          name: '均价',
          type: 'line',
          xAxisIndex: 0,
          yAxisIndex: 0,
          data: avgArr,
          showSymbol: false,
          smooth: false,
          lineStyle: { color: '#f59e0b', width: 1, type: 'solid' },
          connectNulls: false,
        },
        {
          name: '成交量',
          type: 'bar',
          xAxisIndex: 1,
          yAxisIndex: 2,
          data: volArr.map((v, i) => {
            if (v == null) return null
            const price = priceArr[i]
            if (price == null) return null
            // 雪球/同花顺方式：柱体颜色按"该分钟价 vs 上一根有数据的分钟价"
            // 首根无前值，退化为 vs 昨收
            let prevPrice: number | null = null
            for (let j = i - 1; j >= 0; j--) {
              if (priceArr[j] != null) { prevPrice = priceArr[j] as number; break }
            }
            const baseline = prevPrice ?? prevClose
            const color = (price as number) >= baseline ? upColor : downColor
            return { value: v, itemStyle: { color } }
          }) as any,
          barWidth: '70%',
        },
      ],
    }
    chartRef.current.setOption(option, true)
  }, [data, slots, isLightTheme])

  const last = data?.points.length ? data.points[data.points.length - 1] : null
  const currentPrice = last?.price ?? data?.prevClose ?? 0
  const pct = data ? ((currentPrice - data.prevClose) / data.prevClose) * 100 : 0
  const priceColor = pct >= 0 ? '#ef4444' : '#10b981'

  const cardBg = isLightTheme ? '#ffffff' : '#0f172a'
  const headerTextColor = isLightTheme ? '#1f2937' : '#e2e8f0'
  const subText = isLightTheme ? '#64748b' : '#94a3b8'

  return (
    <div style={{
      position: 'relative',
      width: '100%',
      height: '100%',
      background: cardBg,
      borderRadius: 8,
      padding: 8,
      boxSizing: 'border-box',
    }}>
      {/* 顶部信息条 */}
      <div style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        marginBottom: 6, padding: '2px 8px', fontSize: 12, color: headerTextColor,
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
          <span style={{ color: subText }}>{code}</span>
          {data && (
            <>
              <span style={{ color: priceColor, fontWeight: 600, fontSize: 14 }}>
                {currentPrice.toFixed(2)}
              </span>
              <span style={{ color: priceColor }}>
                {pct >= 0 ? '+' : ''}{pct.toFixed(2)}%
              </span>
              <span style={{ color: subText, fontSize: 11 }}>
                昨收 {data.prevClose.toFixed(2)}
              </span>
            </>
          )}
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 11, color: subText }}>
          {data && (
            <>
              <span>{data.date}</span>
              {data.isRealtime ? (
                <span style={{
                  color: '#10b981', padding: '1px 6px', borderRadius: 3,
                  background: 'rgba(16,185,129,0.12)',
                }}>● 实时</span>
              ) : (
                <span style={{ color: subText }}>历史分时</span>
              )}
              {lastFetchedAt && (
                <span>更新 {lastFetchedAt.toLocaleTimeString('zh-CN', { hour12: false })}</span>
              )}
            </>
          )}
        </div>
      </div>

      {/* 图表容器 */}
      <div ref={chartDomRef} style={{ width: '100%', height: 'calc(100% - 30px)' }} />

      {/* 加载/错误覆盖层 */}
      {(loading && !data) && (
        <div style={{
          position: 'absolute', inset: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
          color: subText, fontSize: 13, background: cardBg + 'cc',
        }}>
          加载分时数据...
        </div>
      )}
      {error && (
        <div style={{
          position: 'absolute', top: 40, left: '50%', transform: 'translateX(-50%)',
          padding: '6px 12px', borderRadius: 4,
          background: 'rgba(239,68,68,0.12)', color: '#ef4444',
          fontSize: 12, border: '1px solid rgba(239,68,68,0.3)',
        }}>
          数据源不可达：{error}
        </div>
      )}
    </div>
  )
}
