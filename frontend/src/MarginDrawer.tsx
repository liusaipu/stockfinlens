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

function useEscClose(onClose: () => void) {
  useEffect(() => {
    const handle = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handle)
    return () => document.removeEventListener('keydown', handle)
  }, [onClose])
}

export function MarginDrawer({ code, name, onClose }: Props) {
  const isLight = useIsLightTheme()
  useEscClose(onClose)

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
            padding: '8px 12px',
            borderBottom: isLight ? '1px solid rgba(148,163,184,0.15)' : '1px solid rgba(148,163,184,0.1)',
          }}
        >
          <div style={{ fontSize: 15, fontWeight: 600, color: isLight ? '#1f2937' : '#e2e8f0' }}>
            融资融券{name ? `-${name}` : ''}
          </div>
        </div>

        <div style={{ flex: 1, padding: 8, overflow: 'hidden' }}>
          <MarginChart code={code} style={{ width: '100%', height: '100%', minHeight: 480 }} />
        </div>


      </div>
    </div>
  )
}
