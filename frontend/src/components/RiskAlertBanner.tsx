import { useState } from 'react'
import { analyzer } from '../../wailsjs/go/models'

interface RiskAlertBannerProps {
  alert?: analyzer.RiskAlertSummary
}

function InfoTooltip({ note }: { note: string }) {
  const [show, setShow] = useState(false)
  return (
    <span
      className="risk-info-tooltip"
      onMouseEnter={() => setShow(true)}
      onMouseLeave={() => setShow(false)}
      style={{ position: 'relative', display: 'inline-flex', alignItems: 'center', marginLeft: 4, cursor: 'help' }}
    >
      <span
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: 14,
          height: 14,
          borderRadius: '50%',
          background: '#3b82f6',
          color: '#fff',
          fontSize: 10,
          fontWeight: 700,
          fontStyle: 'italic',
        }}
      >
        i
      </span>
      {show && (
        <span
          style={{
            position: 'absolute',
            left: '100%',
            top: '50%',
            transform: 'translateY(-50%)',
            marginLeft: 6,
            background: '#1e293b',
            color: '#e2e8f0',
            padding: '6px 10px',
            borderRadius: 6,
            fontSize: 11,
            lineHeight: 1.4,
            whiteSpace: 'normal',
            width: 260,
            zIndex: 100,
            boxShadow: '0 4px 12px rgba(0,0,0,0.3)',
            border: '1px solid #334155',
          }}
        >
          {note}
        </span>
      )}
    </span>
  )
}

export function RiskAlertBanner({ alert }: RiskAlertBannerProps) {
  const [expanded, setExpanded] = useState(false)
  if (!alert) return null

  const infoFlags = alert.flags?.filter((f: any) => f.level === 'info') || []
  const riskFlags = alert.flags?.filter((f: any) => f.level !== 'info') || []
  const hasInfoOnly = infoFlags.length > 0 && riskFlags.length === 0

  if (alert.level === 'low' && !hasInfoOnly) return null

  const isHigh = alert.level === 'high'
  const isMedium = alert.level === 'medium'

  // 提取数字：如 "该股票存在 3 项中高风险信号" → "3项中高风险"
  const shortMsg = alert.primaryMsg
    .replace(/^.*存在\s+(\d+)\s+项(高风险|中风险|中高风险)信号.*$/, '$1项$2')
    .replace(/^.*🟢\s*/, '')

  let headerText = shortMsg
  let bannerClass = 'risk-alert-banner'
  if (hasInfoOnly) {
    headerText = 'ℹ️ 信息提示'
    bannerClass = 'risk-alert-banner risk-alert-info'
  } else if (isHigh) {
    bannerClass = 'risk-alert-banner risk-alert-high'
  } else if (isMedium) {
    bannerClass = 'risk-alert-banner risk-alert-medium'
  }

  const displayFlags = hasInfoOnly ? infoFlags : alert.flags

  return (
    <div className={bannerClass}>
      <div
        className="risk-alert-header"
        onClick={() => setExpanded(!expanded)}
        style={{ cursor: displayFlags && displayFlags.length > 0 ? 'pointer' : 'default' }}
      >
        <span>{headerText}</span>
        {displayFlags && displayFlags.length > 0 && (
          <span className={`risk-alert-toggle ${expanded ? 'expanded' : ''}`}>›</span>
        )}
      </div>
      {expanded && displayFlags && displayFlags.length > 0 && (
        <div className="risk-alert-body">
          {displayFlags.map((f: any, i: number) => (
            <div key={i} className={`risk-alert-flag risk-alert-flag-${f.level}`}>
              <span>{f.level === 'high' ? '🔴' : f.level === 'info' ? 'ℹ️' : '🟡'}</span>
              <span>
                {f.name}{f.format ? `：${f.format}` : ''}
                {f.note && <InfoTooltip note={f.note} />}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
