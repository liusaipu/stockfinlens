import { useEffect, useState, useCallback } from 'react'
import './TitleBar.css'
import {
  Environment,
  WindowMinimise,
  WindowMaximise,
  WindowUnmaximise,
  WindowIsMaximised,
  WindowToggleMaximise,
  Quit,
} from '../../wailsjs/runtime'

export function TitleBar() {
  const [visible, setVisible] = useState(false)
  const [isMaximised, setIsMaximised] = useState(false)

  useEffect(() => {
    Environment().then((env) => {
      const onWindows = env.platform === 'windows'
      setVisible(onWindows)
      if (onWindows) {
        document.body.classList.add('windows-titlebar')
      } else {
        document.body.classList.remove('windows-titlebar')
      }
    })
    return () => {
      document.body.classList.remove('windows-titlebar')
    }
  }, [])

  // 监听窗口最大化状态变化
  const refreshMaximised = useCallback(async () => {
    try {
      setIsMaximised(await WindowIsMaximised())
    } catch {
      // ignore
    }
  }, [])

  useEffect(() => {
    if (!visible) return
    refreshMaximised()
    const handleResize = () => refreshMaximised()
    window.addEventListener('resize', handleResize)
    return () => window.removeEventListener('resize', handleResize)
  }, [visible, refreshMaximised])

  if (!visible) return null

  const handleDoubleClick = (e: React.MouseEvent) => {
    // 仅在标题区域双击时触发，避免按钮区域误触
    if ((e.target as HTMLElement).closest('.titlebar-btn')) return
    WindowToggleMaximise()
  }

  const handleMaximiseClick = () => {
    if (isMaximised) {
      WindowUnmaximise()
    } else {
      WindowMaximise()
    }
  }

  return (
    <div className="titlebar" onDoubleClick={handleDoubleClick}>
      <div className="titlebar-drag">
        <span className="titlebar-icon">W</span>
        <span className="titlebar-text">StockFinLens</span>
      </div>
      <div className="titlebar-controls">
        <button
          className="titlebar-btn titlebar-minimise"
          onClick={WindowMinimise}
          title="最小化"
          aria-label="最小化"
        >
          <svg width="10" height="10" viewBox="0 0 10 10">
            <rect x="0" y="4" width="10" height="1" fill="currentColor" />
          </svg>
        </button>
        <button
          className="titlebar-btn titlebar-maximise"
          onClick={handleMaximiseClick}
          title={isMaximised ? '还原' : '最大化'}
          aria-label={isMaximised ? '还原' : '最大化'}
        >
          {isMaximised ? (
            <svg width="10" height="10" viewBox="0 0 10 10">
              <rect
                x="1.5"
                y="3.5"
                width="5"
                height="5"
                stroke="currentColor"
                strokeWidth="0.9"
                fill="none"
              />
              <path
                d="M3.5 3.5 V1.5 H8.5 V6.5 H6.5"
                stroke="currentColor"
                strokeWidth="0.9"
                fill="none"
              />
            </svg>
          ) : (
            <svg width="10" height="10" viewBox="0 0 10 10">
              <rect
                x="0.5"
                y="0.5"
                width="9"
                height="9"
                stroke="currentColor"
                strokeWidth="0.9"
                fill="none"
              />
            </svg>
          )}
        </button>
        <button
          className="titlebar-btn titlebar-close"
          onClick={Quit}
          title="关闭"
          aria-label="关闭"
        >
          <svg width="10" height="10" viewBox="0 0 10 10">
            <path
              d="M1 1 L9 9 M9 1 L1 9"
              stroke="currentColor"
              strokeWidth="0.9"
              fill="none"
            />
          </svg>
        </button>
      </div>
    </div>
  )
}
