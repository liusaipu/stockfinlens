import { useEffect, useMemo, useState } from 'react'
import { GetGroupComparison, FetchMissingCompositeData } from './api'
import type { main } from '../wailsjs/go/models'
import './GroupCompareDrawer.css'

type Row = main.GroupComparisonRow

interface Props {
  groupName: string
  codes: string[]
  onClose: () => void
  onSelectStock: (code: string) => void
}

// Esc 关闭（与 HotPostsDrawer 一致）
function useEscClose(onClose: () => void) {
  useEffect(() => {
    const handle = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handle)
    return () => document.removeEventListener('keydown', handle)
  }, [onClose])
}

// 百分比格式：正数带 +，默认 1 位小数（今日涨幅用 2 位，与 sq-change 一致）
function fmtPct(v: number, digits = 1): string {
  return `${v > 0 ? '+' : ''}${v.toFixed(digits)}%`
}

// 热搜热度值：过万显示为 x.x万
function fmtHotValue(v: number): string {
  if (v >= 1e4) return `${(v / 1e4).toFixed(1)}万`
  return `${Math.round(v)}`
}

// 列定义：value 返回 null 表示该行无效，不参与极值统计
interface ColDef {
  key: string
  title: string
  lowerIsBetter?: boolean // true=数值越小越优（A-Score / 热搜排名 / 负债率）
  value: (r: Row) => number | null
  render: (r: Row) => React.ReactNode
}

const COLUMNS: ColDef[] = [
  {
    key: 'composite',
    title: '综合得分',
    value: (r) => (r.compositeScore > 0 ? r.compositeScore : null),
    render: (r) =>
      r.compositeScore > 0 ? (
        <>
          {r.compositeScore.toFixed(1)}
          <span className="gcd-rank">#{r.compositeRank}</span>
        </>
      ) : (
        <span className="gcd-na">—</span>
      ),
  },
  {
    key: 'score',
    title: '18步评分',
    value: (r) => (r.analyzed ? r.yearScore : null),
    render: (r) =>
      r.analyzed ? (
        <>
          {r.yearScore.toFixed(1)}
          {r.grade && <span className="gcd-grade">{r.grade}</span>}
        </>
      ) : (
        <span className="gcd-na">未分析</span>
      ),
  },
  {
    key: 'ascore',
    title: 'A-Score 风险',
    lowerIsBetter: true,
    value: (r) => (r.analyzed ? r.aScore : null),
    render: (r) => (r.analyzed ? r.aScore.toFixed(1) : <span className="gcd-na">未分析</span>),
  },
  {
    key: 'revenueGrowth',
    title: '营收同比',
    value: (r) => (r.inMarketCache ? r.revenueGrowth : null),
    render: (r) => (r.inMarketCache ? fmtPct(r.revenueGrowth) : <span className="gcd-na">—</span>),
  },
  {
    key: 'netProfitGrowth',
    title: '净利同比',
    value: (r) => (r.inMarketCache ? r.netProfitGrowth : null),
    render: (r) => (r.inMarketCache ? fmtPct(r.netProfitGrowth) : <span className="gcd-na">—</span>),
  },
  {
    key: 'roe',
    title: 'ROE',
    value: (r) => (r.inMarketCache ? r.roe : null),
    render: (r) => (r.inMarketCache ? fmtPct(r.roe) : <span className="gcd-na">—</span>),
  },
  {
    key: 'grossMargin',
    title: '毛利率',
    value: (r) => (r.inMarketCache ? r.grossMargin : null),
    render: (r) => (r.inMarketCache ? fmtPct(r.grossMargin) : <span className="gcd-na">—</span>),
  },
  {
    key: 'debtRatio',
    title: '负债率',
    lowerIsBetter: true,
    value: (r) => (r.hasDebtRatio ? r.debtRatio : null),
    render: (r) => (
      <span className={r.hasDebtRatio ? '' : 'gcd-imputed'}>
        {fmtPct(r.debtRatio)}
        {!r.hasDebtRatio && <span className="gcd-star">*</span>}
      </span>
    ),
  },
  {
    key: 'cashRatio',
    title: '现金含量',
    value: (r) => (r.hasCashRatio ? r.cashRatio : null),
    render: (r) => (
      <span className={r.hasCashRatio ? '' : 'gcd-imputed'}>
        {fmtPct(r.cashRatio)}
        {!r.hasCashRatio && <span className="gcd-star">*</span>}
      </span>
    ),
  },
  {
    key: 'activity',
    title: '活跃度',
    value: (r) => (r.hasActivity ? r.activityScore : null),
    render: (r) => (
      <span className={r.hasActivity ? '' : 'gcd-imputed'}>
        {r.activityScore.toFixed(1)}
        {!r.hasActivity && <span className="gcd-star">*</span>}
      </span>
    ),
  },
  {
    key: 'change',
    title: '今日涨幅',
    value: (r) => r.changePercent,
    render: (r) => (
      <span className={r.changePercent >= 0 ? 'gcd-up' : 'gcd-down'}>{fmtPct(r.changePercent, 2)}</span>
    ),
  },
  {
    key: 'halfYearChange',
    title: '半年涨幅',
    value: (r) => (r.hasHalfYearChange ? r.halfYearChangePercent : null),
    render: (r) =>
      r.hasHalfYearChange ? (
        <span className={r.halfYearChangePercent >= 0 ? 'gcd-up' : 'gcd-down'}>
          {fmtPct(r.halfYearChangePercent, 2)}
        </span>
      ) : (
        <span className="gcd-na">—</span>
      ),
  },
  {
    key: 'hot',
    title: '热搜',
    lowerIsBetter: true,
    value: (r) => (r.thsHotRank > 0 ? r.thsHotRank : null),
    render: (r) =>
      r.thsHotRank > 0 ? (
        <>
          #{r.thsHotRank}
          <span className="gcd-hot-value">{fmtHotValue(r.thsHotValue)}</span>
        </>
      ) : (
        <span className="gcd-na">未上榜</span>
      ),
  },
]

// 每列最优/最差高亮集合（key=股票代码；并列同值都高亮；最差需至少 2 个有效值且不与最优同值）
function computeHighlights(rows: Row[]): Record<string, { best: Set<string>; worst: Set<string> }> {
  const result: Record<string, { best: Set<string>; worst: Set<string> }> = {}
  for (const col of COLUMNS) {
    const valid: { code: string; v: number }[] = []
    for (const r of rows) {
      const v = col.value(r)
      if (v !== null) valid.push({ code: r.code, v })
    }
    const best = new Set<string>()
    const worst = new Set<string>()
    if (valid.length > 0) {
      const values = valid.map((x) => x.v)
      const bestV = col.lowerIsBetter ? Math.min(...values) : Math.max(...values)
      valid.filter((x) => x.v === bestV).forEach((x) => best.add(x.code))
      if (valid.length > 1) {
        const worstV = col.lowerIsBetter ? Math.max(...values) : Math.min(...values)
        if (worstV !== bestV) {
          valid.filter((x) => x.v === worstV).forEach((x) => worst.add(x.code))
        }
      }
    }
    result[col.key] = { best, worst }
  }
  return result
}

function hasMissingData(rows: Row[]): boolean {
  return rows.some(
    (r) =>
      !r.inMarketCache ||
      !r.hasCashRatio ||
      !r.hasDebtRatio ||
      !r.hasActivity ||
      !r.hasHalfYearChange
  )
}

function missingSummary(rows: Row[]): string {
  const noMarket = rows.filter((r) => !r.inMarketCache).length
  const noCash = rows.filter((r) => !r.hasCashRatio).length
  const noDebt = rows.filter((r) => !r.hasDebtRatio).length
  const noAct = rows.filter((r) => !r.hasActivity).length
  const noHalfYear = rows.filter((r) => !r.hasHalfYearChange).length
  const parts: string[] = []
  if (noMarket) parts.push(`${noMarket} 只市场缓存缺失`)
  if (noCash) parts.push(`${noCash} 只现金含量缺失`)
  if (noDebt) parts.push(`${noDebt} 只负债率缺失`)
  if (noAct) parts.push(`${noAct} 只活跃度缺失`)
  if (noHalfYear) parts.push(`${noHalfYear} 只半年涨幅缺失`)
  return parts.join('，')
}

type SortDir = 'asc' | 'desc' | null

export function GroupCompareDrawer({ groupName, codes, onClose, onSelectStock }: Props) {
  useEscClose(onClose)

  const [rows, setRows] = useState<Row[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [fetching, setFetching] = useState(false)
  const [fetchMessage, setFetchMessage] = useState('')
  const [sortKey, setSortKey] = useState<string | null>('composite')
  const [sortDir, setSortDir] = useState<SortDir>('desc')

  const handleSort = (key: string) => {
    if (sortKey === key) {
      if (sortDir === 'asc') setSortDir('desc')
      else if (sortDir === 'desc') {
        setSortKey(null)
        setSortDir(null)
      } else {
        setSortDir('asc')
      }
    } else {
      setSortKey(key)
      setSortDir('asc')
    }
  }

  const sortedRows = useMemo(() => {
    if (!sortKey || !sortDir) return rows
    const col = COLUMNS.find((c) => c.key === sortKey)
    if (!col) return rows
    return [...rows].sort((a, b) => {
      const va = col.value(a)
      const vb = col.value(b)
      if (va === null && vb === null) return 0
      if (va === null) return 1
      if (vb === null) return -1
      return sortDir === 'asc' ? va - vb : vb - va
    })
  }, [rows, sortKey, sortDir])

  const codesKey = codes.join(',')
  const load = () => {
    setLoading(true)
    setError('')
    setFetchMessage('')
    GetGroupComparison(codesKey ? codesKey.split(',') : [])
      .then((list) => setRows(list || []))
      .catch((e: any) => setError(e?.message || '加载失败'))
      .finally(() => setLoading(false))
  }
  useEffect(load, [codesKey])

  const highlights = useMemo(() => computeHighlights(rows), [rows])
  const missing = useMemo(() => hasMissingData(rows), [rows])

  const handleFetchMissing = async () => {
    setFetching(true)
    setFetchMessage('')
    try {
      const res = await FetchMissingCompositeData(codesKey ? codesKey.split(',') : [])
      setFetchMessage(res?.message || '补齐完成')
      load()
    } catch (e: any) {
      setFetchMessage(e?.message || '补齐失败')
    } finally {
      setFetching(false)
    }
  }

  return (
    <div className="gcd-overlay" onClick={onClose}>
      <div className="gcd-drawer" onClick={(e) => e.stopPropagation()}>
        <div className="gcd-header">
          <div className="gcd-header-text">
            <div className="gcd-title">组内对比 · {groupName}</div>
            <div className="gcd-subtitle">共 {codes.length} 只股票</div>
          </div>
          <button className="gcd-close" onClick={onClose} title="关闭 (Esc)">
            ✕
          </button>
        </div>

        <div className="gcd-body">
          {loading && <div className="gcd-state">加载中...</div>}
          {!loading && error && <div className="gcd-state gcd-state-error">{error}</div>}
          {!loading && !error && rows.length === 0 && <div className="gcd-state">暂无对比数据</div>}
          {!loading && !error && rows.length > 0 && (
            <>
              {missing && (
                <div className="gcd-missing-bar">
                  <span className="gcd-missing-text">{missingSummary(rows)}，已用组内中位数临时替代（标 *）</span>
                  <button
                    className="gcd-fetch-btn"
                    onClick={handleFetchMissing}
                    disabled={fetching}
                    title="只补齐缺失的股票数据"
                  >
                    {fetching ? '补齐中...' : '补齐缺失数据'}
                  </button>
                </div>
              )}
              {fetchMessage && <div className="gcd-fetch-msg">{fetchMessage}</div>}
              <div className="gcd-table-wrap">
                <table className="gcd-table">
                  <thead>
                    <tr>
                      <th className="gcd-th-stock">股票</th>
                      {COLUMNS.map((col) => {
                        const sortable = col.key !== 'hot'
                        const active = sortKey === col.key
                        const cls = [
                          sortable ? 'gcd-sortable' : '',
                          active && sortDir ? `gcd-sorted-${sortDir}` : '',
                        ]
                          .filter(Boolean)
                          .join(' ')
                        return (
                          <th
                            key={col.key}
                            className={cls}
                            onClick={sortable ? () => handleSort(col.key) : undefined}
                            title={sortable ? '点击排序' : undefined}
                          >
                            {col.title}
                          </th>
                        )
                      })}
                    </tr>
                  </thead>
                  <tbody>
                    {sortedRows.map((r) => (
                      <tr key={r.code}>
                        <td className="gcd-td-stock">
                          <button
                            className="gcd-stock-btn"
                            title={`查看 ${r.name} 详情`}
                            onClick={() => {
                              onSelectStock(r.code)
                              onClose()
                            }}
                          >
                            <span className="gcd-stock-name">{r.name}</span>
                            <span className="gcd-stock-code">{r.code}</span>
                          </button>
                        </td>
                        {COLUMNS.map((col) => {
                          const hl = highlights[col.key]
                          const cls = [
                            'gcd-td',
                            hl?.best.has(r.code) ? 'cell-best' : '',
                            hl?.worst.has(r.code) ? 'cell-worst' : '',
                          ]
                            .filter(Boolean)
                            .join(' ')
                          return (
                            <td key={col.key} className={cls}>
                              {col.render(r)}
                            </td>
                          )
                        })}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="gcd-footer">
                绿底=组内最优，红底=组内最弱；仅统计有数据的股票。带 * 的指标使用组内中位数替代。
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  )
}
