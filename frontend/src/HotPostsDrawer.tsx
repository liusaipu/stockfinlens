import { useEffect, useMemo, useState } from 'react'
import { BrowserOpenURL } from '../wailsjs/runtime'
import { GetStockHotPosts, GetStockHotPostContent } from './api'
import type { downloader } from '../wailsjs/go/models'

interface Props {
  code: string
  name?: string
  onClose: () => void
}

type SortMode = 'time' | 'hot'

const DEFAULT_COUNT = 10

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

function useEscClose(onClose: () => void) {
  useEffect(() => {
    const handle = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handle)
    return () => document.removeEventListener('keydown', handle)
  }, [onClose])
}

// "2026-07-19 12:24:22" -> "07-19 12:24"
function fmtTime(t: string): string {
  if (t.length >= 16) return t.slice(5, 16)
  return t
}

export function HotPostsDrawer({ code, name, onClose }: Props) {
  const isLight = useIsLightTheme()
  useEscClose(onClose)

  const [posts, setPosts] = useState<downloader.HotPost[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [sortMode, setSortMode] = useState<SortMode>('hot')
  const [showAll, setShowAll] = useState(false)

  const [expandedIds, setExpandedIds] = useState<Set<number>>(new Set())
  const [contentMap, setContentMap] = useState<Record<number, downloader.HotPostContent>>({})
  const [contentLoading, setContentLoading] = useState<Set<number>>(new Set())
  const [contentError, setContentError] = useState<Record<number, string>>({})

  useEffect(() => {
    setLoading(true)
    setError('')
    GetStockHotPosts(code)
      .then((list) => setPosts(list || []))
      .catch((e: any) => setError(e?.message || '加载失败'))
      .finally(() => setLoading(false))
  }, [code])

  const sorted = useMemo(() => {
    const arr = [...posts]
    if (sortMode === 'time') {
      arr.sort((a, b) => (a.publishTime < b.publishTime ? 1 : -1))
    } else {
      // 最热：评论数优先，其次阅读数
      arr.sort((a, b) => (b.commentCount - a.commentCount) || (b.clickCount - a.clickCount))
    }
    return arr
  }, [posts, sortMode])

  const visible = showAll ? sorted : sorted.slice(0, DEFAULT_COUNT)

  const toggleExpand = (post: downloader.HotPost) => {
    const id = Number(post.id)
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
    if (!contentMap[id] && !contentLoading.has(id)) {
      setContentLoading((prev) => new Set(prev).add(id))
      setContentError((prev) => ({ ...prev, [id]: '' }))
      GetStockHotPostContent(code, id)
        .then((c) => {
          setContentMap((prev) => ({ ...prev, [id]: c }))
        })
        .catch((e: any) => {
          setContentError((prev) => ({ ...prev, [id]: e?.message || '获取正文失败' }))
        })
        .finally(() => {
          setContentLoading((prev) => {
            const next = new Set(prev)
            next.delete(id)
            return next
          })
        })
    }
  }

  const textPrimary = isLight ? '#1f2937' : '#e2e8f0'
  const textSecondary = isLight ? '#64748b' : '#94a3b8'
  const borderColor = isLight ? 'rgba(148,163,184,0.15)' : 'rgba(148,163,184,0.1)'

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 100,
        background: isLight ? 'rgba(0,0,0,0.35)' : 'rgba(0,0,0,0.5)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 12,
      }}
      onClick={onClose}
    >
      <div
        style={{
          width: 'min(680px, 100%)',
          height: 'min(600px, 88vh)',
          background: isLight ? '#ffffff' : '#0f1419',
          border: isLight ? '1px solid rgba(148,163,184,0.25)' : '1px solid rgba(148,163,184,0.15)',
          borderRadius: 12,
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          boxShadow: isLight
            ? '0 24px 48px rgba(0,0,0,0.15)'
            : '0 24px 48px rgba(0,0,0,0.4)',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '8px 12px',
            borderBottom: `1px solid ${borderColor}`,
          }}
        >
          <div style={{ fontSize: 15, fontWeight: 600, color: textPrimary }}>
            股吧热帖{name ? `-${name}` : ''}
          </div>
          <div style={{ display: 'flex', gap: 6 }}>
            {(['hot', 'time'] as SortMode[]).map((m) => (
              <button
                key={m}
                className="btn-text"
                onClick={() => setSortMode(m)}
                style={{
                  fontSize: 11,
                  padding: '2px 10px',
                  borderRadius: 3,
                  border: sortMode === m ? '1px solid rgba(168,85,247,0.5)' : '1px solid rgba(148,163,184,0.3)',
                  background: sortMode === m ? 'rgba(168,85,247,0.12)' : 'transparent',
                  color: sortMode === m ? '#a855f7' : textSecondary,
                  cursor: 'pointer',
                }}
              >
                {m === 'hot' ? '最热' : '最新'}
              </button>
            ))}
          </div>
        </div>

        <div style={{ flex: 1, overflowY: 'auto', padding: '4px 12px' }}>
          {loading && (
            <div style={{ padding: 24, textAlign: 'center', color: textSecondary, fontSize: 13 }}>加载中...</div>
          )}
          {!loading && error && (
            <div style={{ padding: 24, textAlign: 'center', color: '#ef4444', fontSize: 13 }}>{error}</div>
          )}
          {!loading && !error && visible.length === 0 && (
            <div style={{ padding: 24, textAlign: 'center', color: textSecondary, fontSize: 13 }}>暂无帖子</div>
          )}
          {!loading && !error && visible.map((p, idx) => {
            const id = Number(p.id)
            const expanded = expandedIds.has(id)
            return (
              <div key={p.id}>
                <div
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '7px 2px',
                    borderBottom: `1px solid ${borderColor}`,
                    cursor: 'pointer',
                  }}
                  onClick={() => toggleExpand(p)}
                  title={expanded ? '点击收起正文' : '点击展开正文'}
                >
                  <button
                    onClick={(e) => { e.stopPropagation(); BrowserOpenURL(p.url) }}
                    title="在浏览器打开原帖"
                    style={{
                      flexShrink: 0,
                      width: 22,
                      height: 22,
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      border: 'none',
                      background: 'transparent',
                      color: textSecondary,
                      cursor: 'pointer',
                      fontSize: 13,
                      padding: 0,
                    }}
                  >
                    📎
                  </button>
                  <span style={{ fontSize: 11, color: textSecondary, width: 18, textAlign: 'right', flexShrink: 0 }}>
                    {idx + 1}
                  </span>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div
                      style={{
                        fontSize: 13,
                        color: textPrimary,
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {p.title}
                    </div>
                    <div style={{ fontSize: 11, color: textSecondary, marginTop: 2, display: 'flex', gap: 10 }}>
                      <span>{fmtTime(p.publishTime)}</span>
                      {p.author && <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: 120 }}>{p.author}</span>}
                    </div>
                  </div>
                  <div style={{ fontSize: 11, color: textSecondary, flexShrink: 0, textAlign: 'right', whiteSpace: 'nowrap', minWidth: 72 }}>
                    <span style={{ marginRight: 8 }}>阅 {p.clickCount}</span>
                    <span>评 {p.commentCount}</span>
                  </div>
                </div>
                {expanded && (
                  <div
                    style={{
                      padding: '8px 12px 12px 28px',
                      borderBottom: `1px solid ${borderColor}`,
                      background: isLight ? 'rgba(168,85,247,0.03)' : 'rgba(168,85,247,0.05)',
                    }}
                    onClick={(e) => e.stopPropagation()}
                  >
                    {contentLoading.has(id) && (
                      <div style={{ fontSize: 12, color: textSecondary }}>加载正文...</div>
                    )}
                    {contentError[id] && !contentLoading.has(id) && (
                      <div style={{ fontSize: 12, color: '#ef4444' }}>{contentError[id]}</div>
                    )}
                    {!contentLoading.has(id) && !contentError[id] && contentMap[id] && (
                      <>
                        {contentMap[id].content && (
                          <div
                            style={{
                              fontSize: 13,
                              color: textPrimary,
                              lineHeight: 1.7,
                              whiteSpace: 'pre-wrap',
                              wordBreak: 'break-word',
                            }}
                          >
                            {contentMap[id].content}
                          </div>
                        )}
                        {contentMap[id].images && contentMap[id].images.length > 0 && (
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8, marginTop: 10 }}>
                            {contentMap[id].images.map((url, i) => (
                              <img
                                key={i}
                                src={url}
                                alt=""
                                onClick={() => BrowserOpenURL(url)}
                                style={{
                                  maxWidth: '100%',
                                  maxHeight: 160,
                                  borderRadius: 6,
                                  cursor: 'pointer',
                                  border: `1px solid ${borderColor}`,
                                }}
                              />
                            ))}
                          </div>
                        )}
                      </>
                    )}
                  </div>
                )}
              </div>
            )
          })}
        </div>

        {!loading && !error && sorted.length > DEFAULT_COUNT && (
          <div
            onClick={() => setShowAll(v => !v)}
            style={{
              padding: '8px 12px',
              borderTop: `1px solid ${borderColor}`,
              textAlign: 'center',
              fontSize: 12,
              color: '#a855f7',
              cursor: 'pointer',
              userSelect: 'none',
            }}
          >
            {showAll ? '收起' : `展开全部 ${sorted.length} 条`}
          </div>
        )}
      </div>
    </div>
  )
}
