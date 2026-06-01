import { useState, useEffect, useCallback } from 'react'
import { createPortal } from 'react-dom'
import './App.css'
import {
  DownloadUpdate,
  ApplyUpdate,
  SkipVersion,
} from './api'
import { EventsOn, EventsOff } from '../wailsjs/runtime'
import { BrowserOpenURL } from '../wailsjs/runtime'

export interface UpdateInfo {
  hasUpdate: boolean
  currentVer: string
  latestVer: string
  releaseName: string
  releaseNote: string
  publishedAt: string
  assetURL: string
  htmlURL: string
}

interface UpdateModalProps {
  isOpen: boolean
  info: UpdateInfo | null
  onClose: () => void
}

export function UpdateModal({ isOpen, info, onClose }: UpdateModalProps) {
  const [downloading, setDownloading] = useState(false)
  const [downloadProgress, setDownloadProgress] = useState(0)
  const [downloadError, setDownloadError] = useState('')
  const [downloadedPath, setDownloadedPath] = useState('')
  const [applying, setApplying] = useState(false)

  useEffect(() => {
    if (!downloading) return
    const handler = (percent: number) => {
      setDownloadProgress(percent)
    }
    EventsOn('update:progress', handler)
    return () => {
      EventsOff('update:progress')
    }
  }, [downloading])

  const resetState = useCallback(() => {
    setDownloading(false)
    setDownloadProgress(0)
    setDownloadError('')
    setDownloadedPath('')
    setApplying(false)
  }, [])

  useEffect(() => {
    if (isOpen) {
      resetState()
    }
  }, [isOpen, resetState])

  if (!isOpen || !info) return null

  const handleDownload = async () => {
    setDownloading(true)
    setDownloadProgress(0)
    setDownloadError('')
    try {
      const path = await DownloadUpdate(info.assetURL, `v${info.latestVer}`)
      setDownloadedPath(path)
      setDownloadProgress(100)
    } catch (e: any) {
      setDownloadError('下载失败: ' + (e?.message || String(e)))
    } finally {
      setDownloading(false)
    }
  }

  const handleApply = async () => {
    if (!downloadedPath) return
    setApplying(true)
    try {
      await ApplyUpdate(downloadedPath)
      // Windows / macOS 都会在 ~1.5s 后 os.Exit(0)；helper 脚本接管替换与重启。
      // 若 ApplyUpdate 在启动脚本前就报错，会走到 catch；否则进程很快消失，无需手动改状态。
    } catch (e: any) {
      setDownloadError('安装失败: ' + (e?.message || String(e)))
      setApplying(false)
    }
  }

  const handleSkip = async () => {
    try {
      await SkipVersion(info.latestVer)
    } catch {
      // ignore
    }
    onClose()
  }

  const handleOpenBrowser = () => {
    if (info.htmlURL) {
      BrowserOpenURL(info.htmlURL)
    }
  }

  return createPortal(
    <div className="modal-overlay" style={{ zIndex: 99999 }}>
      <div className="modal-content" style={{ maxWidth: 520, width: '90%' }}>
        <h4 style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <span>🚀</span>
          发现新版本
        </h4>

        <div style={{ marginBottom: 16 }}>
          <div style={{ fontSize: 14, color: '#94a3b8', marginBottom: 8 }}>
            当前版本 <strong style={{ color: '#cbd5e1' }}>{info.currentVer}</strong>
            {' → '}
            新版本 <strong style={{ color: '#4ade80' }}>{info.latestVer}</strong>
          </div>
          <div style={{ fontSize: 12, color: '#64748b' }}>
            发布时间: {info.publishedAt}
          </div>
        </div>

        {info.releaseNote && (
          <div style={{
            background: 'rgba(30,41,59,0.6)',
            borderRadius: 8,
            padding: 12,
            marginBottom: 16,
            maxHeight: 200,
            overflowY: 'auto',
            fontSize: 13,
            lineHeight: 1.6,
            whiteSpace: 'pre-wrap',
          }}>
            {info.releaseNote}
          </div>
        )}

        {downloading && (
          <div style={{ marginBottom: 16 }}>
            <div style={{ fontSize: 13, color: '#94a3b8', marginBottom: 6 }}>
              正在下载更新包... {downloadProgress}%
            </div>
            <div style={{
              height: 6,
              background: 'rgba(30,41,59,0.8)',
              borderRadius: 3,
              overflow: 'hidden',
            }}>
              <div style={{
                height: '100%',
                width: `${downloadProgress}%`,
                background: '#3b82f6',
                borderRadius: 3,
                transition: 'width 0.3s ease',
              }} />
            </div>
          </div>
        )}

        {downloadError && (
          <div style={{
            background: 'rgba(248,113,113,0.1)',
            borderRadius: 8,
            padding: 12,
            marginBottom: 16,
            fontSize: 13,
            color: '#f87171',
          }}>
            {downloadError}
            <div style={{ marginTop: 8 }}>
              <button
                className="btn btn-secondary"
                onClick={handleOpenBrowser}
                style={{ fontSize: 12, padding: '4px 12px' }}
              >
                去 GitHub 下载
              </button>
            </div>
          </div>
        )}

        {downloadedPath && !downloadError && !applying && (
          <div style={{
            background: 'rgba(74,222,128,0.1)',
            borderRadius: 8,
            padding: 12,
            marginBottom: 16,
            fontSize: 13,
            color: '#4ade80',
          }}>
            ✅ 下载完成
          </div>
        )}

        {applying && (
          <div style={{
            background: 'rgba(59,130,246,0.1)',
            borderRadius: 8,
            padding: 12,
            marginBottom: 16,
            fontSize: 13,
            color: '#93c5fd',
          }}>
            正在安装新版本，应用即将自动重启…
          </div>
        )}

        <div className="modal-actions">
          {!downloading && !applying && !downloadedPath && (
            <>
              <button className="btn btn-secondary" onClick={onClose}>
                稍后提醒
              </button>
              <button className="btn btn-secondary" onClick={handleSkip}>
                跳过此版本
              </button>
              <button className="btn btn-primary" onClick={handleDownload}>
                立即更新
              </button>
            </>
          )}

          {downloadedPath && !downloadError && !applying && (
            <>
              <button className="btn btn-secondary" onClick={onClose}>
                稍后重启
              </button>
              <button className="btn btn-primary" onClick={handleApply}>
                立即重启并安装
              </button>
            </>
          )}

          {(downloading || applying) && (
            <button className="btn btn-secondary" disabled>
              {downloading ? '下载中...' : '安装中...'}
            </button>
          )}
        </div>
      </div>
    </div>,
    document.body
  )
}
