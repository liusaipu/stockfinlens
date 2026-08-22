import { useState, useEffect, useRef, useCallback } from 'react'
import './Settings.css'
import { GetSFLConfig, SaveSFLConfig, VerifySFLToken, CheckForUpdate, SetAutoCheckUpdate, GetAIConfig, SaveAIConfig, TestAIConnection, GetProxyConfig, SetProxyConfig } from './api'
import type { main, ai_researcher } from '../wailsjs/go/models'
import { ClipboardGetText, ClipboardSetText } from '../wailsjs/go/main/App'
import { UpdateModal } from './UpdateModal'

export interface AppSettings {
  theme: 'dark' | 'light' | 'system'
  klineDefaultRange: '1m' | '3m' | '6m' | '1y' | 'all'
  showMA5: boolean
  showMA30: boolean
  showMA180: boolean
  showMA250: boolean
  reportYears: number
  autoUpdateIndustryDB: boolean
  analysisNotification: boolean
  riskSensitivity: 'strict' | 'standard' | 'loose'
  autoCheckUpdate: boolean
  enableHotConcepts: boolean
}

export const DEFAULT_SETTINGS: AppSettings = {
  theme: 'dark',
  klineDefaultRange: '6m',
  showMA5: true,
  showMA30: true,
  showMA180: true,
  showMA250: true,
  reportYears: 5,
  autoUpdateIndustryDB: true,
  analysisNotification: true,
  riskSensitivity: 'standard',
  autoCheckUpdate: true,
  enableHotConcepts: false,
}

const SETTINGS_KEY = 'stockfinlens-settings-v1'

export function loadSettings(): AppSettings {
  try {
    const saved = localStorage.getItem(SETTINGS_KEY)
    if (saved) {
      return { ...DEFAULT_SETTINGS, ...JSON.parse(saved) }
    }
  } catch {
    // ignore
  }
  return DEFAULT_SETTINGS
}

export function saveSettings(settings: AppSettings) {
  try {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
  } catch {
    // ignore
  }
}

// 数据管理相关类型
interface PolicyLibMeta {
  version: string
  updatedAt: string
}

interface IndustryDBMeta {
  version: string
  updatedAt: string
  count: number
}

interface SettingsProps {
  settings: AppSettings
  onSettingsChange: (settings: AppSettings) => void
  // 数据管理相关
  policyLibMeta?: PolicyLibMeta | null
  industryDBMeta?: IndustryDBMeta | null
  policyUpdating?: boolean
  industryUpdating?: boolean
  onUpdatePolicyLibrary?: () => void
  onUpdateIndustryDB?: () => void
  policyActionStatus?: { type: 'success' | 'error' | null; message: string }
  industryActionStatus?: { type: 'success' | 'error' | null; message: string }
  industryTask?: any
  // Python 依赖检测
  onCheckPythonDeps?: () => void
}

// 获取输入框选中的文本
function getSelectedText(el: HTMLInputElement | HTMLTextAreaElement): string {
  const start = el.selectionStart ?? 0
  const end = el.selectionEnd ?? 0
  return el.value.slice(start, end)
}

// 使用原生 value setter 设置值，确保 React controlled 组件能感知变更
function setNativeValue(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = el instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype
  const descriptor = Object.getOwnPropertyDescriptor(proto, 'value')
  if (descriptor && descriptor.set) {
    descriptor.set.call(el, value)
  } else {
    el.value = value
  }
}

// 在光标位置插入文本，替换当前选区
function insertTextAtCursor(el: HTMLInputElement | HTMLTextAreaElement, text: string) {
  const start = el.selectionStart ?? 0
  const end = el.selectionEnd ?? 0
  const newValue = el.value.slice(0, start) + text + el.value.slice(end)
  setNativeValue(el, newValue)
  el.selectionStart = el.selectionEnd = start + text.length
  el.dispatchEvent(new Event('input', { bubbles: true }))
}

// 删除当前选区文本
function deleteSelectedText(el: HTMLInputElement | HTMLTextAreaElement) {
  const start = el.selectionStart ?? 0
  const end = el.selectionEnd ?? 0
  if (start === end) return
  const newValue = el.value.slice(0, start) + el.value.slice(end)
  setNativeValue(el, newValue)
  el.selectionStart = el.selectionEnd = start
  el.dispatchEvent(new Event('input', { bubbles: true }))
}

// 写入剪贴板：Go 后端 -> navigator.clipboard -> execCommand
async function writeClipboard(text: string): Promise<void> {
  // 1. Go 后端（跨平台最可靠）
  try {
    await ClipboardSetText(text)
    return
  } catch (e) {
    console.warn('[Settings] Go ClipboardSetText failed:', e)
  }

  // 2. Web Clipboard API
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(text)
      return
    }
  } catch (e) {
    console.warn('[Settings] navigator.clipboard.writeText failed:', e)
  }

  // 3. execCommand 兜底
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
    console.warn('[Settings] execCommand copy failed:', e)
  }

  throw new Error('无法写入剪贴板')
}

// 读取剪贴板：Go 后端 -> navigator.clipboard
async function readClipboard(): Promise<string> {
  // 1. Go 后端（跨平台最可靠）
  try {
    return await ClipboardGetText()
  } catch (e) {
    console.warn('[Settings] Go ClipboardGetText failed:', e)
  }

  // 2. Web Clipboard API
  try {
    if (navigator.clipboard?.readText) {
      return await navigator.clipboard.readText()
    }
  } catch (e) {
    console.warn('[Settings] navigator.clipboard.readText failed:', e)
  }

  throw new Error('无法读取剪贴板')
}

export function Settings({ 
  settings, 
  onSettingsChange,
  policyLibMeta,
  industryDBMeta,
  policyUpdating = false,
  industryUpdating = false,
  onUpdatePolicyLibrary,
  onUpdateIndustryDB,
  policyActionStatus,
  industryActionStatus,
  industryTask,
  onCheckPythonDeps,
}: SettingsProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [activeTab, setActiveTab] = useState<'appearance' | 'features' | 'data' | 'ai' | 'about'>('appearance')
  const dropdownRef = useRef<HTMLDivElement>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  // StockFinLens 数据源配置状态
  const [sflCfg, setSflCfg] = useState<main.SFLConfig | null>(null)
  const [sflLoading, setSflLoading] = useState(false)
  const [sflVerifyStatus, setSflVerifyStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [sflSaving, setSflSaving] = useState(false)

  // AI 投研配置状态
  const [aiCfg, setAiCfg] = useState<ai_researcher.AIConfig | null>(null)
  const [searchKeyInput, setSearchKeyInput] = useState('')
  const [aiLoading, setAiLoading] = useState(false)
  const [aiTestStatus, setAiTestStatus] = useState<{type: 'success' | 'error' | null, message: string, keyStatuses?: ai_researcher.TavilyKeyStatus[]}>({type: null, message: ''})
  const [aiSaving, setAiSaving] = useState(false)
  const [showLLMKey, setShowLLMKey] = useState(false)
  const [showSearchKey, setShowSearchKey] = useState(false)
  const [clipboardError, setClipboardError] = useState<string | null>(null)

  // 解析 Tavily Key 输入字符串为数组
  const parseSearchKeys = (input: string): string[] => {
    return input
      .split(/[\n,，\s]+/)
      .map((k) => k.trim())
      .filter((k) => k.length > 0)
      .slice(0, 5)
  }

  // 检查更新状态
  const [updateChecking, setUpdateChecking] = useState(false)
  const [updateCheckResult, setUpdateCheckResult] = useState<{type: 'success' | 'info' | 'error' | null, message: string}>({type: null, message: ''})
  const [showUpdateModal, setShowUpdateModal] = useState(false)
  const [foundUpdateInfo, setFoundUpdateInfo] = useState<any>(null)

  // 代理配置状态
  const [proxyCfg, setProxyCfg] = useState<{ enabled: boolean; url: string; username: string; password: string }>({
    enabled: false,
    url: '',
    username: '',
    password: '',
  })
  const [proxySaving, setProxySaving] = useState(false)
  const [proxySaveStatus, setProxySaveStatus] = useState<{type: 'success' | 'error' | null, message: string}>({type: null, message: ''})
  const [proxyExpanded, setProxyExpanded] = useState(false)

  // 加载数据源配置
  useEffect(() => {
    GetProxyConfig().then((cfg) => {
      setProxyCfg({
        enabled: cfg.enabled || false,
        url: cfg.url || '',
        username: cfg.username || '',
        password: cfg.password || '',
      })
    }).catch(() => {})
    GetSFLConfig().then((cfg) => {
      setSflCfg(cfg)
    }).catch(() => {
      setSflCfg({
        enabled: false, token: '', verified: false, verified_at: '',
        use_for_financial: true, use_for_kline: true, use_for_quote: true, use_for_moneyflow: true,
        moneyflow_days: 3
      } as main.SFLConfig)
    })
    GetAIConfig().then((cfg) => {
      setAiCfg(cfg)
      setSearchKeyInput((cfg.search_api_keys || [cfg.search_api_key]).filter(Boolean).join('\n'))
    }).catch(() => {
      setAiCfg({
        enabled: false, llm_provider: 'deepseek', llm_api_key: '', llm_base_url: 'https://api.deepseek.com/v1',
        llm_model: 'deepseek-v4-pro', llm_timeout: 90, temperature: 0.2, max_tokens: 8192, top_p: 1.0,
        search_provider: 'tavily', search_api_key: '', search_api_keys: [], search_depth: 'advanced', search_timeout: 180, max_results: 10,
        search_recency_days: 90, exhausted_search_keys: {}, focus_regions: ['us', 'jp'], output_language: 'zh-CN', enable_social: true,
        cache_ttl_hours: 6
      } as ai_researcher.AIConfig)
    })
  }, [isOpen])

  const handleVerifySFL = useCallback(async () => {
    if (!sflCfg?.token) return
    setSflVerifyStatus({type: null, message: ''})
    setSflLoading(true)
    try {
      const result = await VerifySFLToken(sflCfg.token)
      setSflVerifyStatus({type: result.success ? 'success' : 'error', message: result.message || '验证失败'})
      if (result.success) {
        setSflCfg(prev => prev ? {...prev, verified: true, verified_at: new Date().toISOString()} : prev)
      }
    } catch (err: any) {
      setSflVerifyStatus({type: 'error', message: err?.message || '验证失败'})
    } finally {
      setSflLoading(false)
    }
  }, [sflCfg?.token])

  const handleSaveSFL = useCallback(async () => {
    if (!sflCfg) return
    setSflSaving(true)
    try {
      await SaveSFLConfig(sflCfg)
      setSflVerifyStatus({type: 'success', message: '配置已保存'})
    } catch (err: any) {
      setSflVerifyStatus({type: 'error', message: err?.message || '保存失败'})
    } finally {
      setSflSaving(false)
    }
  }, [sflCfg])

  const handleTestAI = useCallback(async () => {
    if (!aiCfg) return
    // 即使未触发 onBlur，也先根据当前输入框内容解析 Key
    const keys = parseSearchKeys(searchKeyInput)
    const updatedCfg = {...aiCfg, search_api_keys: keys, search_api_key: keys[0] || ''}
    setAiCfg(updatedCfg)
    setSearchKeyInput(keys.join('\n'))

    setAiTestStatus({type: null, message: '', keyStatuses: []})
    setAiLoading(true)
    try {
      // 测试前自动保存当前表单配置
      await SaveAIConfig(updatedCfg)
      const result = await TestAIConnection(updatedCfg)
      setAiTestStatus({
        type: result.success ? 'success' : 'error',
        message: result.message || (result.success ? '连接成功' : '连接失败'),
        keyStatuses: result.search_key_statuses || [],
      })
    } catch (err: any) {
      setAiTestStatus({type: 'error', message: err?.message || '连接测试失败'})
    } finally {
      setAiLoading(false)
    }
  }, [aiCfg, searchKeyInput])

  const handleSaveAI = useCallback(async () => {
    if (!aiCfg) return
    // 即使未触发 onBlur，也先根据当前输入框内容解析 Key
    const keys = parseSearchKeys(searchKeyInput)
    const updatedCfg = {...aiCfg, search_api_keys: keys, search_api_key: keys[0] || ''}
    setAiSaving(true)
    try {
      await SaveAIConfig(updatedCfg)
      setAiCfg(updatedCfg)
      setSearchKeyInput(keys.join('\n'))
      setAiTestStatus({type: 'success', message: '配置已保存'})
    } catch (err: any) {
      setAiTestStatus({type: 'error', message: err?.message || '保存失败'})
    } finally {
      setAiSaving(false)
    }
  }, [aiCfg, searchKeyInput])

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(e.target as Node) &&
        buttonRef.current &&
        !buttonRef.current.contains(e.target as Node)
      ) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // 设置面板打开期间，为所有 input/textarea 提供 Cmd/Ctrl + C/V/X 键盘剪贴板支持
  useEffect(() => {
    if (!isOpen) return

    const isInputElement = (el: Element | null): el is HTMLInputElement | HTMLTextAreaElement => {
      return el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement
    }

    const handleKeyDown = async (e: KeyboardEvent) => {
      const target = document.activeElement
      if (!isInputElement(target)) return

      const isMod = e.metaKey || e.ctrlKey
      if (!isMod) return

      const key = e.key.toLowerCase()

      if (key === 'a') {
        e.preventDefault()
        target.selectionStart = 0
        target.selectionEnd = target.value.length
        return
      }

      if (key === 'v') {
        e.preventDefault()
        try {
          const text = await readClipboard()
          if (text) {
            insertTextAtCursor(target, text)
          }
          setClipboardError(null)
        } catch (e: any) {
          setClipboardError(`粘贴失败: ${e?.message || '未知错误'}`)
        }
        return
      }

      if (key === 'c') {
        const selected = getSelectedText(target)
        if (selected) {
          e.preventDefault()
          try {
            await writeClipboard(selected)
            setClipboardError(null)
          } catch (e: any) {
            setClipboardError(`复制失败: ${e?.message || '未知错误'}`)
          }
        }
        return
      }

      if (key === 'x') {
        const selected = getSelectedText(target)
        if (selected) {
          e.preventDefault()
          try {
            await writeClipboard(selected)
            deleteSelectedText(target)
            setClipboardError(null)
          } catch (e: any) {
            setClipboardError(`剪切失败: ${e?.message || '未知错误'}`)
          }
        }
        return
      }

      if (key === 'z') {
        e.preventDefault()
        try {
          const cmd = e.shiftKey ? 'redo' : 'undo'
          document.execCommand(cmd)
        } catch (e: any) {
          setClipboardError(`撤销失败: ${e?.message || '未知错误'}`)
        }
        return
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen])

  const updateSetting = <K extends keyof AppSettings>(key: K, value: AppSettings[K]) => {
    const newSettings = { ...settings, [key]: value }
    onSettingsChange(newSettings)
    saveSettings(newSettings)
  }

  const version = __APP_VERSION__

  const handleCheckUpdate = useCallback(async () => {
    setUpdateChecking(true)
    setUpdateCheckResult({type: null, message: ''})
    try {
      const info = await CheckForUpdate()
      if (info.hasUpdate) {
        setUpdateCheckResult({type: 'info', message: `发现新版本 ${info.latestVer}`})
        setFoundUpdateInfo(info)
        setShowUpdateModal(true)
      } else {
        setUpdateCheckResult({type: 'success', message: '当前已是最新版本'})
      }
    } catch (err: any) {
      setUpdateCheckResult({type: 'error', message: err?.message || '检查失败'})
    } finally {
      setUpdateChecking(false)
    }
  }, [])

  const handleSaveProxy = useCallback(async () => {
    setProxySaving(true)
    setProxySaveStatus({type: null, message: ''})
    try {
      await SetProxyConfig({
        enabled: proxyCfg.enabled,
        url: proxyCfg.url.trim(),
        username: proxyCfg.username.trim(),
        password: proxyCfg.password,
      } as main.ProxyConfig)
      setProxySaveStatus({type: 'success', message: '代理配置已保存'})
    } catch (err: any) {
      setProxySaveStatus({type: 'error', message: err?.message || '保存失败'})
    } finally {
      setProxySaving(false)
    }
  }, [proxyCfg])

  return (
    <>
      <button
        ref={buttonRef}
        className="settings-toggle"
        title="设置"
        onClick={() => setIsOpen(!isOpen)}
      >
        ⚙️
      </button>

      <UpdateModal
        isOpen={showUpdateModal}
        info={foundUpdateInfo}
        onClose={() => setShowUpdateModal(false)}
      />

      {isOpen && (
        <div ref={dropdownRef} className="settings-dropdown">
          <div className="settings-tabs">
            <button className={activeTab === 'appearance' ? 'active' : ''} onClick={() => setActiveTab('appearance')}>外观</button>
            <button className={activeTab === 'features' ? 'active' : ''} onClick={() => setActiveTab('features')}>功能</button>
            <button className={activeTab === 'data' ? 'active' : ''} onClick={() => setActiveTab('data')}>数据</button>
            <button className={activeTab === 'ai' ? 'active' : ''} onClick={() => setActiveTab('ai')}>AI 投研</button>
            <button className={activeTab === 'about' ? 'active' : ''} onClick={() => setActiveTab('about')}>关于</button>
          </div>

          {clipboardError && (
            <div className="settings-action-status" style={{ padding: '8px 16px', background: 'rgba(239,68,68,0.1)', borderBottom: '1px solid rgba(239,68,68,0.2)' }}>
              <span className="status-error">{clipboardError}</span>
            </div>
          )}

          {activeTab === 'appearance' && (
            <div className="settings-section">
              <div className="settings-item">
                <label>主题</label>
                <div className="settings-options">
                  <label className="settings-radio"><input type="radio" name="theme" checked={settings.theme === 'dark'} onChange={() => updateSetting('theme', 'dark')} /><span>深色</span></label>
                  <label className="settings-radio"><input type="radio" name="theme" checked={settings.theme === 'light'} onChange={() => updateSetting('theme', 'light')} /><span>浅色</span></label>
                  <label className="settings-radio"><input type="radio" name="theme" checked={settings.theme === 'system'} onChange={() => updateSetting('theme', 'system')} /><span>跟随系统</span></label>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'features' && (
            <div className="settings-section">
              <div className="settings-item settings-item-inline">
                <label>市场热点</label>
                <div className="settings-toggle-switch">
                  <label className="switch"><input type="checkbox" checked={settings.enableHotConcepts} onChange={(e) => updateSetting('enableHotConcepts', e.target.checked)} /><span className="slider"></span></label>
                </div>
              </div>
              <div className="settings-item settings-item-inline">
                <label>分析完成提示</label>
                <div className="settings-toggle-switch">
                  <label className="switch"><input type="checkbox" checked={settings.analysisNotification} onChange={(e) => updateSetting('analysisNotification', e.target.checked)} /><span className="slider"></span></label>
                </div>
              </div>
              <div className="settings-item settings-item-inline">
                <label>风险警示敏感度</label>
                <div className="settings-input-group">
                  <select
                    value={settings.riskSensitivity}
                    onChange={(e) => updateSetting('riskSensitivity', e.target.value as 'strict' | 'standard' | 'loose')}
                    style={{ padding: '4px 8px', borderRadius: 4, border: '1px solid rgba(148,163,184,0.3)', background: 'rgba(15,23,42,0.6)', color: '#e2e8f0', fontSize: 13 }}
                  >
                    <option value="strict">严格</option>
                    <option value="standard">标准（默认）</option>
                    <option value="loose">宽松</option>
                  </select>
                </div>
              </div>
            </div>
          )}

          {activeTab === 'data' && (
            <div className="settings-section">
              {/* 基础数据设置 */}
              <div className="settings-item settings-item-inline">
                <label>财报下载年限</label>
                <div className="settings-input-group">
                  <input type="number" min={3} max={10} value={settings.reportYears} onChange={(e) => updateSetting('reportYears', parseInt(e.target.value) || 5)} />
                  <span>年</span>
                </div>
              </div>
              <div className="settings-item settings-item-inline">
                <label>自动更新行业库</label>
                <div className="settings-toggle-switch">
                  <label className="switch"><input type="checkbox" checked={settings.autoUpdateIndustryDB} onChange={(e) => updateSetting('autoUpdateIndustryDB', e.target.checked)} /><span className="slider"></span></label>
                </div>
              </div>

              {/* 数据管理分割线 */}
              <div className="settings-divider" />

              {/* 产业政策库 */}
              <div className="settings-data-section">
                <div className="settings-data-title">📚 产业政策库</div>
                <div className="settings-data-info">
                  <div>版本: <span>{policyLibMeta?.version || 'builtin'}</span></div>
                  <div>更新于: <span>{policyLibMeta?.updatedAt || '内置默认'}</span></div>
                </div>
                <div className="settings-data-desc">
                  为报告模块5（政策匹配度评估）提供政策关键词数据
                </div>
                {onUpdatePolicyLibrary && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onUpdatePolicyLibrary}
                    disabled={policyUpdating}
                  >
                    {policyUpdating ? '更新中...' : '🔄 更新政策库'}
                  </button>
                )}
                {policyActionStatus?.type && !policyUpdating && (
                  <div className="settings-action-status">
                    {policyActionStatus.type === 'success' ? (
                      <span className="status-success">{policyActionStatus.message}</span>
                    ) : (
                      <span className="status-error">{policyActionStatus.message.length > 40 ? policyActionStatus.message.slice(0, 40) + '...' : policyActionStatus.message}</span>
                    )}
                  </div>
                )}
              </div>

              {/* 行业均值数据库 */}
              <div className="settings-data-section">
                <div className="settings-data-title">🏭 行业均值数据库</div>
                <div className="settings-data-info">
                  <div>行业数: <span>{industryDBMeta?.count || 0}</span></div>
                  <div>更新于: <span>{industryDBMeta?.updatedAt || '未更新'}</span></div>
                </div>
                <div className="settings-data-desc">
                  为报告模块4（行业横向对比）提供行业基准数据
                </div>
                {onUpdateIndustryDB && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onUpdateIndustryDB}
                    disabled={industryUpdating}
                  >
                    {industryUpdating
                      ? (industryTask?.status === 'running' && industryTask?.total
                          ? `后台采集中 ${Math.round((industryTask.progress || 0) / industryTask.total * 100)}%...`
                          : '后台采集中...')
                      : '🔄 更新行业数据库'}
                  </button>
                )}
                {industryTask?.status === 'running' && (
                  <div className="settings-action-status">
                    <span style={{ color: '#94a3b8' }}>{industryTask.message || '正在采集全市场数据...'}</span>
                  </div>
                )}
                {industryTask?.status === 'completed' && !industryUpdating && (
                  <div className="settings-action-status">
                    <span className="status-success">{industryTask.message || '后台采集完成'}</span>
                  </div>
                )}
                {industryTask?.status === 'error' && !industryUpdating && (
                  <div className="settings-action-status">
                    <span className="status-error">{industryTask.message || '后台采集失败'}</span>
                  </div>
                )}
                {industryActionStatus?.type && !industryUpdating && industryTask?.status !== 'running' && (
                  <div className="settings-action-status">
                    {industryActionStatus.type === 'success' ? (
                      <span className="status-success">{industryActionStatus.message}</span>
                    ) : (
                      <span className="status-error">{industryActionStatus.message.length > 40 ? industryActionStatus.message.slice(0, 40) + '...' : industryActionStatus.message}</span>
                    )}
                  </div>
                )}
              </div>

              {/* StockFinLens 数据源 */}
              <div className="settings-data-section">
                <div className="settings-data-title">📊 StockFinLens 数据源</div>
                <div className="settings-data-desc">
                  启用 StockFinLens 数据源，提升数据稳定性
                </div>

                {sflCfg && (
                  <>
                    <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                      <label>启用 StockFinLens</label>
                      <div className="settings-toggle-switch">
                        <label className="switch">
                          <input
                            type="checkbox"
                            checked={sflCfg.enabled}
                            onChange={(e) => setSflCfg({ ...sflCfg, enabled: e.target.checked })}
                          />
                          <span className="slider"></span>
                        </label>
                      </div>
                    </div>

                    {sflCfg.enabled && (
                      <>
                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>授权码</label>
                          <input
                            type="password"
                            value={sflCfg.token}
                            onChange={(e) => setSflCfg({ ...sflCfg, token: e.target.value, verified: false })}
                            placeholder="请输入授权码"
                            style={{ width: '100%', marginTop: 4 }}
                          />
                          {sflCfg.verified && (
                            <div style={{ fontSize: 11, color: '#22c55e', marginTop: 2 }}>
                              ✅ 已验证{sflCfg.verified_at ? ` · ${sflCfg.verified_at.slice(0, 10)}` : ''}
                            </div>
                          )}
                        </div>

                        {sflVerifyStatus.type && (
                          <div className="settings-action-status" style={{ marginTop: 8 }}>
                            {sflVerifyStatus.type === 'success' ? (
                              <span className="status-success">{sflVerifyStatus.message}</span>
                            ) : (
                              <span className="status-error">{sflVerifyStatus.message}</span>
                            )}
                          </div>
                        )}

                        <div className="settings-item" style={{ marginTop: 8, fontSize: 12 }}>
                          <label style={{ marginBottom: 4 }}>启用范围</label>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            <label><input type="checkbox" checked={sflCfg.use_for_financial} onChange={(e) => setSflCfg({ ...sflCfg, use_for_financial: e.target.checked })} /> 财报数据</label>
                            <label><input type="checkbox" checked={sflCfg.use_for_kline} onChange={(e) => setSflCfg({ ...sflCfg, use_for_kline: e.target.checked })} /> 历史K线</label>
                            <label><input type="checkbox" checked={sflCfg.use_for_quote} onChange={(e) => setSflCfg({ ...sflCfg, use_for_quote: e.target.checked })} /> 每日指标（PE/PB/市值）</label>
                            <label style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                              <input type="checkbox" checked={sflCfg.use_for_moneyflow} onChange={(e) => setSflCfg({ ...sflCfg, use_for_moneyflow: e.target.checked })} />
                              <span>个股资金流向</span>
                              {sflCfg.enabled && sflCfg.use_for_moneyflow && (
                                <span style={{ display: 'flex', alignItems: 'center', gap: 4, marginLeft: 4 }}>
                                  <input
                                    type="number"
                                    min={3}
                                    max={10}
                                    value={sflCfg.moneyflow_days || 3}
                                    onChange={(e) => {
                                      const val = parseInt(e.target.value, 10)
                                      if (!isNaN(val) && val >= 3 && val <= 10) {
                                        setSflCfg({ ...sflCfg, moneyflow_days: val })
                                      }
                                    }}
                                    style={{
                                      width: 36,
                                      height: 20,
                                      fontSize: 11,
                                      textAlign: 'center',
                                      borderRadius: 3,
                                      border: '1px solid #cbd5e1',
                                      background: '#f8fafc',
                                      color: '#334155',
                                    }}
                                  />
                                  <span style={{ fontSize: 11, color: '#64748b' }}>个交易日</span>
                                </span>
                              )}
                            </label>
                          </div>
                        </div>

                        {/* 操作按钮放在启用范围下方 */}
                        <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
                          <button
                            className="settings-data-btn"
                            onClick={handleVerifySFL}
                            disabled={sflLoading || !sflCfg.token}
                            style={{ whiteSpace: 'nowrap' }}
                          >
                            {sflLoading ? '验证中...' : '🔍 验证连通性'}
                          </button>
                          <button
                            className="settings-data-btn"
                            onClick={handleSaveSFL}
                            disabled={sflSaving}
                            style={{ whiteSpace: 'nowrap' }}
                          >
                            {sflSaving ? '保存中...' : '💾 保存配置'}
                          </button>
                        </div>
                      </>
                    )}
                  </>
                )}
              </div>

            </div>
          )}

          {activeTab === 'ai' && (
            <div className="settings-section">
              <div className="settings-data-section">
                <div className="settings-data-title">🤖 AI 投研</div>
                <div className="settings-data-desc">
                  使用大模型搜索并分析股票的催化剂、政策影响、国际对标与社交情绪
                </div>

                {aiCfg && (
                  <>
                    <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                      <label>启用 AI 投研</label>
                      <div className="settings-toggle-switch">
                        <label className="switch">
                          <input
                            type="checkbox"
                            checked={aiCfg.enabled}
                            onChange={(e) => setAiCfg({ ...aiCfg, enabled: e.target.checked })}
                          />
                          <span className="slider"></span>
                        </label>
                      </div>
                    </div>

                    {!aiCfg.enabled && (
                      <div className="settings-action-status" style={{ marginTop: 8 }}>
                        <span className="status-error">
                          ⚠️ 功能未启用：请先打开上方开关，再填写 API Key 并测试连接
                        </span>
                      </div>
                    )}

                    {aiCfg.enabled && (
                      <>
                        <div style={{ marginTop: 12, fontSize: 12, color: '#94a3b8' }}>
                          大模型配置（第一层：连接信息）
                        </div>

                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>LLM 供应商</label>
                          <select
                            value={aiCfg.llm_provider}
                            onChange={(e) => {
                              const provider = e.target.value
                              const defaults: Record<string, { base_url: string; model: string }> = {
                                kimi: { base_url: 'https://api.moonshot.cn/v1', model: 'kimi-k2.6' },
                                'kimi-code': { base_url: 'https://api.kimi.com/coding/v1', model: 'kimi-k2.6' },
                                deepseek: { base_url: 'https://api.deepseek.com/v1', model: 'deepseek-v4-pro' },
                              }
                              setAiCfg({
                                ...aiCfg,
                                llm_provider: provider,
                                llm_base_url: defaults[provider]?.base_url || aiCfg.llm_base_url,
                                llm_model: defaults[provider]?.model || aiCfg.llm_model,
                              })
                            }}
                            style={{ padding: '4px 8px', borderRadius: 4, border: '1px solid rgba(148,163,184,0.3)', background: 'rgba(15,23,42,0.6)', color: '#e2e8f0', fontSize: 13 }}
                          >
                            <option value="deepseek">DeepSeek</option>
                            <option value="kimi">Kimi（Moonshot 开放平台）</option>
                            <option value="kimi-code">Kimi Code（kimi.com/code）</option>
                          </select>
                        </div>

                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>API Key</label>
                          <div style={{ position: 'relative', marginTop: 4 }}>
                            <input
                              type={showLLMKey ? 'text' : 'password'}
                              value={aiCfg.llm_api_key}
                              onChange={(e) => setAiCfg({ ...aiCfg, llm_api_key: e.target.value })}
                              placeholder="sk-..."
                              style={{ width: '100%', paddingRight: 32 }}
                            />
                            <button
                              type="button"
                              onClick={() => setShowLLMKey((v) => !v)}
                              style={{
                                position: 'absolute',
                                right: 6,
                                top: '50%',
                                transform: 'translateY(-50%)',
                                background: 'transparent',
                                border: 'none',
                                cursor: 'pointer',
                                fontSize: 14,
                                color: '#94a3b8',
                                padding: 2,
                              }}
                              title={showLLMKey ? '隐藏' : '显示'}
                            >
                              {showLLMKey ? '🙈' : '👁️'}
                            </button>
                          </div>
                        </div>

                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>Base URL</label>
                          <input
                            type="text"
                            value={aiCfg.llm_base_url}
                            onChange={(e) => setAiCfg({ ...aiCfg, llm_base_url: e.target.value })}
                            placeholder="https://api.deepseek.com/v1"
                            style={{ width: '100%', marginTop: 4 }}
                          />
                        </div>

                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>模型名称</label>
                          {aiCfg.llm_provider === 'deepseek' ? (
                            <select
                              value={aiCfg.llm_model}
                              onChange={(e) => setAiCfg({ ...aiCfg, llm_model: e.target.value })}
                              style={{ width: '100%', marginTop: 4, padding: '4px 8px', borderRadius: 4, border: '1px solid rgba(148,163,184,0.3)', background: 'rgba(15,23,42,0.6)', color: '#e2e8f0', fontSize: 13 }}
                            >
                              <option value="deepseek-v4-pro">deepseek-v4-pro（默认推荐）</option>
                              <option value="deepseek-v4-flash">deepseek-v4-flash（更快更便宜）</option>
                              <option value="deepseek-chat">deepseek-chat（旧版，2026-07-24 停用）</option>
                              <option value="deepseek-reasoner">deepseek-reasoner（旧版推理，2026-07-24 停用）</option>
                            </select>
                          ) : aiCfg.llm_provider === 'kimi' || aiCfg.llm_provider === 'kimi-code' ? (
                            <select
                              value={aiCfg.llm_model}
                              onChange={(e) => setAiCfg({ ...aiCfg, llm_model: e.target.value })}
                              style={{ width: '100%', marginTop: 4, padding: '4px 8px', borderRadius: 4, border: '1px solid rgba(148,163,184,0.3)', background: 'rgba(15,23,42,0.6)', color: '#e2e8f0', fontSize: 13 }}
                            >
                              <option value="kimi-k2.6">kimi-k2.6（默认推荐）</option>
                              <option value="kimi-k2.5">kimi-k2.5</option>
                              <option value="kimi-k2.7-code">kimi-k2.7-code（编程专用）</option>
                              <option value="kimi-k2.7-code-highspeed">kimi-k2.7-code-highspeed（高速版）</option>
                            </select>
                          ) : (
                            <input
                              type="text"
                              value={aiCfg.llm_model}
                              onChange={(e) => setAiCfg({ ...aiCfg, llm_model: e.target.value })}
                              placeholder="模型名称"
                              style={{ width: '100%', marginTop: 4 }}
                            />
                          )}
                          <div style={{ fontSize: 11, color: '#64748b', marginTop: 2 }}>
                            {aiCfg.llm_provider === 'deepseek'
                              ? 'DeepSeek V4 新模型：pro 质量更高，flash 速度更快、成本更低'
                              : aiCfg.llm_provider === 'kimi' || aiCfg.llm_provider === 'kimi-code'
                                ? 'Kimi K2 系列新模型：k2.6 综合能力最强，k2.7-code 更适合代码场景；Kimi Code 端点会自动处理 temperature'
                                : '可手动输入模型名'}
                          </div>
                        </div>

                        <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                          <label>请求超时</label>
                          <div className="settings-input-group">
                            <input
                              type="number"
                              min={10}
                              max={300}
                              value={aiCfg.llm_timeout}
                              onChange={(e) => setAiCfg({ ...aiCfg, llm_timeout: parseInt(e.target.value, 10) || 90 })}
                            />
                            <span>秒</span>
                          </div>
                        </div>

                        <div style={{ marginTop: 16, fontSize: 12, color: '#94a3b8' }}>
                          搜索引擎配置（首期支持 Tavily）
                        </div>

                        <div className="settings-item" style={{ marginTop: 8 }}>
                          <label>Tavily API Key</label>
                          <div style={{ position: 'relative', marginTop: 4 }}>
                            <textarea
                              value={searchKeyInput}
                              onChange={(e) => setSearchKeyInput(e.target.value)}
                              onBlur={(e) => {
                                const keys = parseSearchKeys(e.target.value)
                                setSearchKeyInput(keys.join('\n'))
                                setAiCfg({
                                  ...aiCfg,
                                  search_api_keys: keys,
                                  search_api_key: keys[0] || '',
                                })
                              }}
                              placeholder={`tvly-...\n支持最多5个key，换行或者逗号分隔`}
                              rows={4}
                              spellCheck={false}
                              style={{
                                width: '100%',
                                padding: '8px 32px 8px 8px',
                                resize: 'vertical',
                                fontFamily: 'monospace',
                                fontSize: 13,
                              }}
                            />
                            <button
                              type="button"
                              onClick={() => setShowSearchKey((v) => !v)}
                              style={{
                                position: 'absolute',
                                right: 6,
                                top: 10,
                                background: 'transparent',
                                border: 'none',
                                cursor: 'pointer',
                                fontSize: 14,
                                color: '#94a3b8',
                                padding: 2,
                              }}
                              title={showSearchKey ? '隐藏' : '显示'}
                            >
                              {showSearchKey ? '🙈' : '👁️'}
                            </button>
                          </div>
                          <div style={{ fontSize: 11, color: '#64748b', marginTop: 4, display: 'flex', justifyContent: 'space-between' }}>
                            <span>支持最多5个key，换行或者逗号分隔，自动轮换使用</span>
                            <span style={{ color: (aiCfg.search_api_keys?.length || 0) > 0 ? '#22c55e' : '#94a3b8' }}>
                              已配置 {(aiCfg.search_api_keys?.length || 0)} 个 Key
                            </span>
                          </div>
                        </div>

                        <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                          <label>搜索深度</label>
                          <select
                            value={aiCfg.search_depth}
                            onChange={(e) => setAiCfg({ ...aiCfg, search_depth: e.target.value as 'basic' | 'advanced' })}
                            style={{ padding: '4px 8px', borderRadius: 4, border: '1px solid rgba(148,163,184,0.3)', background: 'rgba(15,23,42,0.6)', color: '#e2e8f0', fontSize: 13 }}
                          >
                            <option value="advanced">advanced（质量高，费用约 basic 10 倍）</option>
                            <option value="basic">basic（快且便宜）</option>
                          </select>
                        </div>

                        <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                          <label>搜索超时</label>
                          <div className="settings-input-group">
                            <input
                              type="number"
                              min={30}
                              max={600}
                              value={aiCfg.search_timeout}
                              onChange={(e) => setAiCfg({ ...aiCfg, search_timeout: parseInt(e.target.value, 10) || 180 })}
                            />
                            <span>秒</span>
                          </div>
                        </div>

                        <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                          <label>返回条数</label>
                          <div className="settings-input-group">
                            <input
                              type="number"
                              min={1}
                              max={20}
                              value={aiCfg.max_results}
                              onChange={(e) => setAiCfg({ ...aiCfg, max_results: parseInt(e.target.value, 10) || 10 })}
                            />
                            <span>条/查询</span>
                          </div>
                        </div>

                        <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                          <label>时间范围</label>
                          <div className="settings-input-group">
                            <input
                              type="number"
                              min={7}
                              value={aiCfg.search_recency_days}
                              onChange={(e) => setAiCfg({ ...aiCfg, search_recency_days: parseInt(e.target.value, 10) || 90 })}
                            />
                            <span>天内</span>
                          </div>
                        </div>

                        <div className="settings-item" style={{ marginTop: 8, fontSize: 12 }}>
                          <label style={{ marginBottom: 4 }}>国际市场关注</label>
                          <div style={{ display: 'flex', flexWrap: 'wrap', gap: 8 }}>
                            {[
                              { key: 'us', label: '美国' },
                              { key: 'jp', label: '日本' },
                              { key: 'eu', label: '欧洲' },
                              { key: 'hk', label: '香港' },
                              { key: 'kr', label: '韩国' },
                              { key: 'tw', label: '台湾' },
                            ].map((r) => (
                              <label key={r.key} style={{ display: 'flex', alignItems: 'center', gap: 4 }}>
                                <input
                                  type="checkbox"
                                  checked={aiCfg.focus_regions?.includes(r.key) || false}
                                  onChange={(e) => {
                                    const regions = new Set(aiCfg.focus_regions || [])
                                    if (e.target.checked) {
                                      regions.add(r.key)
                                    } else {
                                      regions.delete(r.key)
                                    }
                                    setAiCfg({ ...aiCfg, focus_regions: Array.from(regions) })
                                  }}
                                />
                                <span>{r.label}</span>
                              </label>
                            ))}
                          </div>
                        </div>

                        <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                          <label>抓取社交情绪</label>
                          <div className="settings-toggle-switch">
                            <label className="switch">
                              <input
                                type="checkbox"
                                checked={aiCfg.enable_social}
                                onChange={(e) => setAiCfg({ ...aiCfg, enable_social: e.target.checked })}
                              />
                              <span className="slider"></span>
                            </label>
                          </div>
                        </div>

                        <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                          <label>缓存时间</label>
                          <div className="settings-input-group">
                            <input
                              type="number"
                              min={1}
                              max={720}
                              value={aiCfg.cache_ttl_hours}
                              onChange={(e) => setAiCfg({ ...aiCfg, cache_ttl_hours: parseInt(e.target.value, 10) || 6 })}
                            />
                            <span>小时</span>
                          </div>
                        </div>

                        {aiTestStatus.type && (
                          <div className="settings-action-status" style={{ marginTop: 12 }}>
                            {aiTestStatus.type === 'success' ? (
                              <span className="status-success">{aiTestStatus.message}</span>
                            ) : (
                              <span className="status-error">{aiTestStatus.message}</span>
                            )}
                          </div>
                        )}

                        {/* 逐个 Tavily Key 验证结果 */}
                        {aiTestStatus.keyStatuses && aiTestStatus.keyStatuses.length > 0 && (
                          <div style={{ marginTop: 12, fontSize: 12, lineHeight: 1.6 }}>
                            <div style={{ color: '#94a3b8', marginBottom: 4 }}>Tavily Key 验证结果：</div>
                            {aiTestStatus.keyStatuses.map((s, idx) => (
                              <div key={idx} style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                                <span style={{ fontFamily: 'monospace', color: '#e2e8f0' }}>{s.key}</span>
                                <span style={{
                                  color: s.status === 'ok' ? '#22c55e' : s.status === 'exhausted' ? '#f59e0b' : '#ef4444',
                                  fontWeight: 500,
                                }}>
                                  {s.status === 'ok' ? '✅ 可用' : s.status === 'exhausted' ? '⚠️ 额度用完' : s.status === 'invalid' ? '❌ 无效' : '❌ 错误'}
                                </span>
                                <span style={{ color: '#64748b' }}>{s.message}</span>
                              </div>
                            ))}
                          </div>
                        )}

                        <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
                          <button
                            className="settings-data-btn"
                            onClick={handleTestAI}
                            disabled={aiLoading || !aiCfg.enabled || !aiCfg.llm_api_key || (aiCfg.search_api_keys?.length || 0) === 0}
                            title={!aiCfg.enabled ? '请先启用 AI 投研' : !aiCfg.llm_api_key || (aiCfg.search_api_keys?.length || 0) === 0 ? '请填写 LLM 和 Tavily API Key' : '测试连接'}
                            style={{ whiteSpace: 'nowrap' }}
                          >
                            {aiLoading ? '测试中...' : '🔍 测试连接'}
                          </button>
                          <button
                            className="settings-data-btn"
                            onClick={handleSaveAI}
                            disabled={aiSaving}
                            style={{ whiteSpace: 'nowrap' }}
                          >
                            {aiSaving ? '保存中...' : '💾 保存配置'}
                          </button>
                        </div>

                        <div style={{ marginTop: 12, fontSize: 11, color: '#64748b', lineHeight: 1.5 }}>
                          💡 提示：DeepSeek 推荐 deepseek-v4-pro / deepseek-v4-flash（旧名称 2026-07-24 停用）；Kimi / Kimi Code 推荐 kimi-k2.6 / kimi-k2.5。Kimi Code 请从 kimi.com/code 申请 Key。<br />
                          ⚠️ AI 分析仅供参考，所有结论请以上市公司公告和官方数据为准。
                        </div>
                      </>
                    )}
                  </>
                )}
              </div>
            </div>
          )}

          {activeTab === 'about' && (
            <div className="settings-section about-section">
              <img src="/logo.png" className="about-logo" alt="StockFinLens Logo" />
              <div className="about-title">股票财报透镜</div>
              <div className="about-version">版本 {version}</div>
              <div className="about-desc">
                穿透财报看真相<br />
                揭示风险防踩雷<br />
                重要指标可溯源
              </div>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 16, marginTop: 8 }}>
                <button
                  className="about-link"
                  onClick={handleCheckUpdate}
                  disabled={updateChecking}
                  style={{
                    background: 'none',
                    border: 'none',
                    color: '#60a5fa',
                    cursor: updateChecking ? 'not-allowed' : 'pointer',
                    textDecoration: 'underline',
                    fontSize: 13,
                    padding: 0,
                  }}
                >
                  {updateChecking ? '检查中...' : '检查更新'}
                </button>
                <label style={{ display: 'flex', alignItems: 'center', gap: 6, fontSize: 12, color: '#94a3b8', cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={settings.autoCheckUpdate}
                    onChange={async (e) => {
                      const checked = e.target.checked
                      updateSetting('autoCheckUpdate', checked)
                      try { await SetAutoCheckUpdate(checked) } catch { /* ignore */ }
                    }}
                    style={{ cursor: 'pointer' }}
                  />
                  启动时自动检查
                </label>
              </div>
              {updateCheckResult.type && (
                <div style={{
                  fontSize: 12,
                  marginTop: 6,
                  color: updateCheckResult.type === 'success' ? '#4ade80' : updateCheckResult.type === 'error' ? '#f87171' : '#fbbf24',
                }}>
                  {updateCheckResult.type === 'success' ? '✓ ' : updateCheckResult.type === 'error' ? '✗ ' : 'ℹ️ '}
                  {updateCheckResult.message}
                </div>
              )}

              {/* 网络代理 */}
              <div className="settings-divider" style={{ margin: '16px 0' }} />
              <div className="settings-data-section" style={{ textAlign: 'left' }}>
                <div
                  className="settings-data-title"
                  onClick={() => setProxyExpanded((v) => !v)}
                  style={{ cursor: 'pointer', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}
                >
                  <span>🌐 网络代理</span>
                  <span style={{ fontSize: 12, color: '#94a3b8' }}>{proxyExpanded ? '▾' : '▸'}</span>
                </div>
                <div className="settings-data-desc">
                  为自动更新下载配置代理，支持 http / https / socks5
                </div>

                {proxyExpanded && (
                  <>
                    <div className="settings-item settings-item-inline" style={{ marginTop: 8 }}>
                      <label>启用代理</label>
                      <div className="settings-toggle-switch">
                        <label className="switch">
                          <input
                            type="checkbox"
                            checked={proxyCfg.enabled}
                            onChange={(e) => setProxyCfg({ ...proxyCfg, enabled: e.target.checked })}
                          />
                          <span className="slider"></span>
                        </label>
                      </div>
                    </div>

                    <div className="settings-item" style={{ marginTop: 8 }}>
                      <label>代理地址</label>
                      <input
                        type="text"
                        value={proxyCfg.url}
                        onChange={(e) => setProxyCfg({ ...proxyCfg, url: e.target.value })}
                        placeholder="http://127.0.0.1:7890"
                        disabled={!proxyCfg.enabled}
                        style={{ width: '100%', marginTop: 4 }}
                      />
                    </div>

                    <div className="settings-item" style={{ marginTop: 8 }}>
                      <label>用户名（可选）</label>
                      <input
                        type="text"
                        value={proxyCfg.username}
                        onChange={(e) => setProxyCfg({ ...proxyCfg, username: e.target.value })}
                        placeholder="留空表示无需认证"
                        disabled={!proxyCfg.enabled}
                        style={{ width: '100%', marginTop: 4 }}
                      />
                    </div>

                    <div className="settings-item" style={{ marginTop: 8 }}>
                      <label>密码（可选）</label>
                      <input
                        type="password"
                        value={proxyCfg.password}
                        onChange={(e) => setProxyCfg({ ...proxyCfg, password: e.target.value })}
                        placeholder="留空表示无需认证"
                        disabled={!proxyCfg.enabled}
                        style={{ width: '100%', marginTop: 4 }}
                      />
                    </div>

                    {proxySaveStatus.type && (
                      <div className="settings-action-status" style={{ marginTop: 8 }}>
                        {proxySaveStatus.type === 'success' ? (
                          <span className="status-success">{proxySaveStatus.message}</span>
                        ) : (
                          <span className="status-error">{proxySaveStatus.message}</span>
                        )}
                      </div>
                    )}

                    <div style={{ display: 'flex', gap: 8, marginTop: 12 }}>
                      <button
                        className="settings-data-btn"
                        onClick={handleSaveProxy}
                        disabled={proxySaving}
                        style={{ whiteSpace: 'nowrap' }}
                      >
                        {proxySaving ? '保存中...' : '💾 保存代理配置'}
                      </button>
                    </div>
                  </>
                )}
              </div>

              {/* 运行环境 */}
              <div className="settings-divider" style={{ margin: '16px 0' }} />
              <div className="settings-data-section" style={{ textAlign: 'left' }}>
                <div className="settings-data-title">🖥️ 运行环境</div>
                <div className="settings-data-desc">
                  检测 ML 推理和数据更新所需的 Python 依赖包
                </div>
                {onCheckPythonDeps && (
                  <button 
                    className="settings-data-btn" 
                    onClick={onCheckPythonDeps}
                  >
                    🔍 检测运行环境
                  </button>
                )}
              </div>
            </div>
          )}
        </div>
      )}
    </>
  )
}
