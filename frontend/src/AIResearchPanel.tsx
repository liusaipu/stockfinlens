import { useState, useEffect, useRef } from 'react'
import './AIResearchPanel.css'
import type { ai_researcher } from '../wailsjs/go/models'
import { GetAIConfig } from './api'
import { ClipboardSetText } from '../wailsjs/go/main/App'

interface AIResearchPanelProps {
  symbol: string
  name: string
  report: ai_researcher.AIResearchReport | null
  loading: boolean
  error: string | null
  progress: { stage: string; message: string } | null
  reportRef?: React.RefObject<HTMLDivElement>
}

const sentimentMap: Record<string, { label: string; color: string }> = {
  positive: { label: '乐观', color: '#22c55e' },
  neutral: { label: '中性', color: '#94a3b8' },
  negative: { label: '谨慎', color: '#ef4444' },
}

function isConfigured(cfg: ai_researcher.AIConfig | null): boolean {
  if (!cfg) return false
  if (!cfg.enabled) return false
  if (!cfg.llm_api_key) return false
  if (!cfg.search_api_key) return false
  return true
}

async function writeClipboard(text: string): Promise<void> {
  try {
    await ClipboardSetText(text)
    return
  } catch (e) {
    console.warn('[AIResearchPanel] Go ClipboardSetText failed:', e)
  }
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return
    }
  } catch (e) {
    console.warn('[AIResearchPanel] navigator.clipboard.writeText failed:', e)
  }
  try {
    const ta = document.createElement('textarea')
    ta.value = text
    ta.style.position = 'fixed'
    ta.style.opacity = '0'
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    document.body.removeChild(ta)
    if (ok) return
  } catch (e) {
    console.warn('[AIResearchPanel] execCommand copy failed:', e)
  }
  throw new Error('无法写入剪贴板')
}

export function AIResearchPanel({ symbol, name, report, loading, error, progress, reportRef }: AIResearchPanelProps) {
  const [expandedSources, setExpandedSources] = useState(false)
  const [config, setConfig] = useState<ai_researcher.AIConfig | null>(null)
  const [configLoading, setConfigLoading] = useState(true)
  const [elapsed, setElapsed] = useState(0)
  const [copyToast, setCopyToast] = useState<string | null>(null)
  const localReportRef = useRef<HTMLDivElement>(null)
  const contentRef = reportRef || localReportRef
  const toastTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  const showCopyToast = (message: string) => {
    setCopyToast(message)
    if (toastTimer.current) {
      clearTimeout(toastTimer.current)
    }
    toastTimer.current = setTimeout(() => {
      setCopyToast(null)
    }, 2000)
  }

  const handleCopyLink = async (url: string) => {
    try {
      await writeClipboard(url)
      showCopyToast('链接已复制到剪贴板')
    } catch (e) {
      console.warn('[AIResearchPanel] 复制链接失败:', e)
      showCopyToast('复制失败，请手动复制')
    }
  }

  useEffect(() => {
    setConfigLoading(true)
    GetAIConfig()
      .then((cfg) => setConfig(cfg))
      .catch(() => setConfig(null))
      .finally(() => setConfigLoading(false))
    return () => {
      if (toastTimer.current) {
        clearTimeout(toastTimer.current)
      }
    }
  }, [])

  useEffect(() => {
    if (!loading) {
      setElapsed(0)
      return
    }
    const start = Date.now()
    const timer = setInterval(() => {
      setElapsed(Math.floor((Date.now() - start) / 1000))
    }, 1000)
    return () => clearInterval(timer)
  }, [loading])

  const configured = isConfigured(config)
  const showConfigurePrompt = !configLoading && !configured && !report && !loading

  const formatElapsed = (sec: number) => {
    if (sec < 60) return `${sec} 秒`
    const m = Math.floor(sec / 60)
    const s = sec % 60
    return `${m} 分 ${s} 秒`
  }

  return (
    <div className="ai-research-panel">
      <div className="ai-research-header">
        <div>
          <h2 className="ai-research-title">🤖 AI 投研：{name || symbol}</h2>
          <div className="ai-research-subtitle">
            {report?.model_used && (
              <span>模型：{report.model_used}</span>
            )}
            {report?.from_cache && (
              <span className="ai-research-cache">· 来自缓存</span>
            )}
            {report?.generated_at && (
              <span>· {new Date(report.generated_at).toLocaleString('zh-CN')}</span>
            )}
          </div>
        </div>
      </div>

      {error && (
        <div className="ai-research-error">
          ⚠️ {error}
        </div>
      )}

      {loading && (
        <div className="ai-research-loading">
          <div className="ai-research-spinner" />
          <div>{progress?.message || '正在搜索并分析相关信息，请稍候...'}</div>
          <div className="ai-research-loading-hint">
            已耗时 {formatElapsed(elapsed)} · 首次分析约需 30-120 秒
          </div>
          {progress && progress.stage !== 'init' && (
            <div className="ai-research-progress-stage">
              当前阶段：{progress.stage === 'search' ? '联网搜索' : progress.stage === 'llm' ? '大模型分析' : progress.stage === 'parse' ? '解析报告' : progress.stage === 'cache' ? '保存结果' : progress.message}
            </div>
          )}
        </div>
      )}

      {showConfigurePrompt && (
        <div className="ai-research-empty">
          <div className="ai-research-empty-icon">⚙️</div>
          <div style={{ fontWeight: 600, color: '#e2e8f0', marginBottom: 6 }}>
            AI 投研尚未配置
          </div>
          <div className="ai-research-empty-hint">
            请在右上角设置 →「AI 投研」中启用功能，<br />
            并填写 LLM（Kimi / DeepSeek）和 Tavily 搜索 API Key
          </div>
          <div className="ai-research-empty-hint" style={{ marginTop: 8 }}>
            Tavily 免费额度通常足够个人使用，注册地址：<br />
            <a href="https://tavily.com" target="_blank" rel="noopener noreferrer">tavily.com</a>
          </div>
        </div>
      )}

      {!loading && !error && !report && !showConfigurePrompt && (
        <div className="ai-research-empty">
          <div className="ai-research-empty-icon">🤖</div>
          <div>点击顶部工具栏「开始分析」启动 AI 投研</div>
          <div className="ai-research-empty-hint">
            将搜索产品催化剂、政策影响、全球产业映射、国际对标和社交情绪
          </div>
        </div>
      )}

      {report && !loading && (
        <div ref={contentRef} className="ai-research-content">
          <div className="ai-research-disclaimer">
            ⚠️ AI 分析仅供参考，请以上市公司公告和官方数据为准。
          </div>

          <div className="ai-research-sections">
            {report.sections?.map((section, idx) => {
              const sentiment = sentimentMap[section.sentiment] || sentimentMap.neutral
              return (
                <div key={idx} className="ai-research-section">
                  <div className="ai-research-section-header">
                    <h3>{section.title}</h3>
                    <span className="ai-research-sentiment" style={{ color: sentiment.color }}>
                      {sentiment.label}
                    </span>
                  </div>
                  <p className="ai-research-summary">{section.summary}</p>
                  {section.key_points && section.key_points.length > 0 && (
                    <ul className="ai-research-points">
                      {section.key_points.map((point, pidx) => (
                        <li key={pidx}>{point}</li>
                      ))}
                    </ul>
                  )}
                </div>
              )
            })}
          </div>

          {report.sources && report.sources.length > 0 && (
            <div className="ai-research-sources">
              <div
                className="ai-research-sources-header"
                onClick={() => setExpandedSources(!expandedSources)}
              >
                <span>📎 参考来源（{report.sources.length}）</span>
                <span className={`ai-research-sources-toggle ${expandedSources ? 'expanded' : ''}`}>›</span>
              </div>
              {expandedSources && (
                <ul className="ai-research-sources-list">
                  {report.sources.map((source, idx) => (
                    <li key={idx}>
                      <div className="ai-research-source-main">
                        <button
                          type="button"
                          className="ai-research-source-copy"
                          onClick={() => handleCopyLink(source.url)}
                          title="复制链接"
                          aria-label="复制链接"
                        >
                          📎
                        </button>
                        <a
                          href={source.url}
                          target="_blank"
                          rel="noopener noreferrer"
                          title={source.url}
                        >
                          {source.title || source.url}
                        </a>
                      </div>
                      {source.date && <span className="ai-research-source-date">{source.date}</span>}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
          {copyToast && (
            <div className="ai-research-toast ai-research-copy-toast">
              {copyToast}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
