import { useEffect, useState } from 'react'
import { MarginChart } from './MarginChart'

interface Props {
  code: string
  name?: string
  onClose: () => void
}

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

export function MarginDrawer({ code, name, onClose }: Props) {
  const isLight = useIsLightTheme()

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
        padding: 20,
      }}
      onClick={onClose}
    >
      <div
        style={{
          width: 'min(1100px, 100%)',
          height: 'min(720px, 90vh)',
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
            padding: '12px 16px',
            borderBottom: isLight ? '1px solid rgba(148,163,184,0.15)' : '1px solid rgba(148,163,184,0.1)',
          }}
        >
          <div style={{ fontSize: 15, fontWeight: 600, color: isLight ? '#1f2937' : '#e2e8f0' }}>
            融资融券{name ? `-${name}` : ''}
          </div>
          <button
            onClick={onClose}
            style={{
              background: 'transparent',
              border: 'none',
              color: isLight ? '#64748b' : '#94a3b8',
              fontSize: 20,
              cursor: 'pointer',
              lineHeight: 1,
              padding: '0 4px',
            }}
            title="关闭"
          >
            ×
          </button>
        </div>

        <div style={{ flex: 1, padding: 12, overflow: 'hidden' }}>
          <MarginChart code={code} style={{ width: '100%', height: '100%', minHeight: 480 }} />
        </div>


      </div>
    </div>
  )
}
