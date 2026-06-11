// App.tsx - 应用根组件，配置路由和全局布局
import { useState, useEffect, createContext, useContext } from 'react'
import { HashRouter, Routes, Route, NavLink, useLocation } from 'react-router-dom'
import {
  LayoutDashboard, TrendingUp, BarChart2, FileText,
  Terminal as TerminalIcon, Search, Power, ChevronDown
} from 'lucide-react'
import Dashboard from './pages/Dashboard'
import Market from './pages/Market'
import Predictor from './pages/Predictor'
import Reports from './pages/Reports'
import Terminal from './pages/Terminal'
import ErrorBoundary from './components/ErrorBoundary'
import './index.css'
import { GetLeagues, SetActiveLeague, GetActiveLeague } from '../bindings/football/app'
import { League } from '../bindings/football/internal/models/models'

// Re-export so other components can import League from App
export type { League }

import { useSearchParams, useNavigate } from 'react-router-dom'

// ─── 联赛 Context (全局共享当前联赛信息) ─────────────
interface LeagueContextType {
  activeLeague: League | null
  allLeagues: League[]
  switchLeague: (sportKey: string) => Promise<void>
}

const LeagueContext = createContext<LeagueContextType>({
  activeLeague: null,
  allLeagues: [],
  switchLeague: async () => {},
})

export function useLeague() {
  return useContext(LeagueContext)
}

// 导航项配置
const NAV_ITEMS = [
  { path: '/',          icon: LayoutDashboard, label: '仪表盘',    id: 'nav-dashboard' },
  { path: '/market',   icon: TrendingUp,       label: '市场监控',  id: 'nav-market' },
  { path: '/predictor',icon: BarChart2,        label: '策略预测',  id: 'nav-predictor' },
  { path: '/terminal', icon: TerminalIcon,     label: '系统终端',  id: 'nav-terminal' },
  { path: '/reports',  icon: FileText,         label: '资产报告',  id: 'nav-reports' },
]

const SUB_TABS = [
  { key: 'odds',      label: '实时赔率' },
  { key: 'arbitrage', label: '套利扫描' },
  { key: 'momentum',  label: '动能趋势' },
  { key: 'ai_analysis', label: 'AI 智能分析' },
  { key: 'history',   label: '历史数据' },
  { key: 'standings', label: '积分/赛程' },
]

function AppShell() {
  const location = useLocation()
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()
  const [searchTerm, setSearchTerm] = useState('')
  const { activeLeague, allLeagues, switchLeague } = useLeague()
  const [leagueDropdownOpen, setLeagueDropdownOpen] = useState(false)

  const activeTab = searchParams.get('tab') || 'odds'

  // 处理横向顶部 Tab 点击
  const handleTabClick = (key: string) => {
    if (location.pathname !== '/') {
      navigate(`/?tab=${key}`)
    } else {
      setSearchParams({ tab: key })
    }
  }

  // 获取当前页面标题
  const getHeaderTitle = () => {
    const leagueName = activeLeague ? `${activeLeague.emoji} ${activeLeague.name}` : ''
    return `足球量化分析终端 ${leagueName}`
  }

  return (
    <div className="flex h-full w-full overflow-hidden bg-[var(--bg-primary)]">
      {/* ── 左侧图标 + 文字 宽导航栏（全高） ── */}
      <nav className="nav-sidebar flex-shrink-0" style={{ width: '220px', minWidth: '220px', maxWidth: '220px', flexShrink: 0 }}>
        <div className="flex flex-col w-full">
          {/* 品牌标识 */}
          <div className="brand-section">
            <div className="brand-title-main">
              足球量化<br />
              <span className="brand-title-accent">分析终端</span>
            </div>
            <div className="brand-subtitle">V4.0.0-多联赛版</div>
          </div>

          {/* ── 联赛选择器 ── */}
          <div style={{ padding: '0 12px', marginBottom: '8px' }}>
            <div
               onClick={() => setLeagueDropdownOpen(!leagueDropdownOpen)}
               style={{
                 background: 'rgba(99, 102, 241, 0.05)',
                 border: '1px solid rgba(99, 102, 241, 0.25)',
                 padding: '8px 10px',
                 cursor: 'pointer',
                 display: 'flex',
                 justifyContent: 'space-between',
                 alignItems: 'center',
                 transition: 'all 0.15s ease',
               }}
               onMouseEnter={(e) => {
                 e.currentTarget.style.borderColor = 'rgba(99, 102, 241, 0.5)'
                 e.currentTarget.style.background = 'rgba(99, 102, 241, 0.08)'
               }}
               onMouseLeave={(e) => {
                 e.currentTarget.style.borderColor = 'rgba(99, 102, 241, 0.25)'
                 e.currentTarget.style.background = 'rgba(99, 102, 241, 0.05)'
               }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ fontSize: '16px' }}>{activeLeague?.emoji || '⚽'}</span>
                <div style={{ display: 'flex', flexDirection: 'column' }}>
                  <span style={{
                    fontFamily: 'var(--font-sans)',
                    fontSize: '11px',
                    fontWeight: 700,
                    color: '#818cf8',
                    letterSpacing: '0.02em',
                  }}>
                    {activeLeague?.name || '选择联赛'}
                  </span>
                  <span style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '9px',
                    color: '#64748b',
                  }}>
                    {activeLeague?.fullName || ''}
                  </span>
                </div>
              </div>
              <ChevronDown
                size={12}
                style={{
                  color: '#818cf8',
                  transform: leagueDropdownOpen ? 'rotate(180deg)' : 'rotate(0deg)',
                  transition: 'transform 0.2s ease',
                }}
              />
            </div>

            {/* 联赛下拉列表 */}
            {leagueDropdownOpen && (
              <div style={{
                border: '1px solid rgba(99, 102, 241, 0.2)',
                borderTop: 'none',
                background: '#0f172a',
                maxHeight: '280px',
                overflowY: 'auto',
                zIndex: 100,
                position: 'relative',
              }}>
                {allLeagues.map((league) => {
                  const isActive = activeLeague?.sportKey === league.sportKey
                  return (
                    <div
                      key={league.sportKey}
                      onClick={async () => {
                        await switchLeague(league.sportKey)
                        setLeagueDropdownOpen(false)
                      }}
                      style={{
                        padding: '7px 10px',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '8px',
                        background: isActive ? 'rgba(99, 102, 241, 0.15)' : 'transparent',
                        borderLeft: isActive ? '2px solid #6366f1' : '2px solid transparent',
                        transition: 'all 0.1s ease',
                      }}
                      onMouseEnter={(e) => {
                        if (!isActive) {
                          e.currentTarget.style.background = 'rgba(255,255,255,0.03)'
                        }
                      }}
                      onMouseLeave={(e) => {
                        if (!isActive) {
                          e.currentTarget.style.background = 'transparent'
                        }
                      }}
                    >
                      <span style={{ fontSize: '14px', width: '20px', textAlign: 'center' }}>{league.emoji}</span>
                      <div style={{ display: 'flex', flexDirection: 'column' }}>
                        <span style={{
                          fontFamily: 'var(--font-sans)',
                          fontSize: '11px',
                          fontWeight: isActive ? 700 : 500,
                          color: isActive ? '#818cf8' : '#cbd5e1',
                        }}>
                          {league.name}
                        </span>
                        <span style={{
                          fontFamily: 'var(--font-mono)',
                          fontSize: '8px',
                          color: '#64748b',
                        }}>
                          {league.country} · {league.type === 'cup' ? '杯赛制' : '联赛制'}
                        </span>
                      </div>
                      {isActive && (
                        <span style={{
                          marginLeft: 'auto',
                          fontSize: '8px',
                          color: '#818cf8',
                          fontFamily: 'var(--font-mono)',
                          fontWeight: 700,
                        }}>
                          ● 监控中
                        </span>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* 导航菜单列表 */}
          <div className="nav-links">
            {NAV_ITEMS.map(({ path, icon: Icon, label, id }) => {
              const isActive = path === '/'
                ? location.pathname === '/'
                : location.pathname.startsWith(path)
              return (
                <NavLink
                  key={path}
                  to={path}
                  id={id}
                  className={`nav-item ${isActive ? 'active' : ''}`}
                >
                  <Icon size={16} />
                  <span>{label}</span>
                </NavLink>
              )
            })}
          </div>
        </div>

        {/* 底部操作员状态 */}
        <div className="operator-profile">
          <div className="operator-avatar">首席</div>
          <div className="operator-info">
            <span className="operator-name">分析师_01</span>
            <span className="operator-session">系统运行中</span>
          </div>
        </div>
      </nav>

      {/* ── 右侧主工作区 ── */}
      <div className="flex-1 flex flex-col overflow-hidden" style={{ minWidth: 0 }}>
        {/* 顶部综合控制标题栏 */}
        <header className="titlebar justify-between" style={{ paddingLeft: '24px' }}>
          {/* 主标题与子 Tab */}
          <div className="flex items-center gap-12" style={{ WebkitAppRegion: 'no-drag', height: '100%' } as any}>
            <h1 style={{
              fontFamily: 'var(--font-sans)',
              fontSize: '18px',
              fontWeight: 800,
              color: 'var(--text-primary)',
              letterSpacing: '0.05em',
              margin: 0
            }}>
              {getHeaderTitle()}
            </h1>

            {/* 顶栏横向 Tab 导航：只有在 / (仪表盘) 路由下才渲染 */}
            {location.pathname === '/' && (
              <div className="flex items-center gap-6" style={{ height: '100%', flexShrink: 0 }}>
                {SUB_TABS.map(({ key, label }) => {
                  const isTabActive = activeTab === key
                  return (
                    <button
                      key={key}
                      onClick={() => handleTabClick(key)}
                      style={{
                        background: 'none',
                        border: 'none',
                        outline: 'none',
                        color: isTabActive ? 'var(--accent-green)' : 'var(--text-muted)',
                        fontFamily: 'var(--font-sans)',
                        fontSize: '12px',
                        fontWeight: 600,
                        cursor: 'pointer',
                        padding: '0 12px',
                        height: '100%',
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        position: 'relative',
                        transition: 'all var(--transition-fast)',
                        borderBottom: isTabActive ? '3px solid var(--accent-green)' : '3px solid transparent',
                        textShadow: isTabActive ? '0 0 8px rgba(0, 255, 65, 0.4)' : 'none',
                        whiteSpace: 'nowrap'
                      }}
                    >
                      {label}
                    </button>
                  )
                })}
              </div>
            )}
          </div>

          {/* 右侧搜索与动作控件组 */}
          <div className="flex items-center gap-4" style={{ marginLeft: 'auto', WebkitAppRegion: 'no-drag' } as any}>
            {/* 搜索框 */}
            <div className="relative flex items-center" style={{ width: '200px' }}>
              <input
                type="text"
                placeholder="搜索比赛..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                style={{
                  background: 'rgba(24, 34, 22, 0.6)',
                  border: '1px solid var(--border)',
                  color: 'var(--text-primary)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '11px',
                  padding: '4px 8px 4px 24px',
                  outline: 'none',
                  width: '100%',
                  height: '24px'
                }}
              />
              <Search size={11} style={{ position: 'absolute', left: '8px', color: 'var(--text-muted)' }} />
            </div>

            {/* 关闭电源 */}
            <button title="系统退出" className="flex items-center justify-center p-1 text-secondary hover:text-white" style={{ background: 'none', border: 'none', cursor: 'pointer' }}>
              <Power size={14} style={{ color: 'var(--accent-red)' }} />
            </button>
          </div>
        </header>

        {/* 页面主视图内容 */}
        <main className="flex-1 overflow-hidden relative">
          <Routes>
            <Route path="/"          element={<ErrorBoundary fallbackLabel="仪表盘 Dashboard"><Dashboard /></ErrorBoundary>} />
            <Route path="/market"    element={<ErrorBoundary fallbackLabel="市场监控 Market"><Market /></ErrorBoundary>} />
            <Route path="/predictor" element={<ErrorBoundary fallbackLabel="策略预测 Predictor"><Predictor /></ErrorBoundary>} />
            <Route path="/terminal"  element={<ErrorBoundary fallbackLabel="系统终端 Terminal"><Terminal /></ErrorBoundary>} />
            <Route path="/reports"   element={<ErrorBoundary fallbackLabel="资产报告 Reports"><Reports /></ErrorBoundary>} />
          </Routes>
        </main>
      </div>
    </div>
  )
}

export default function App() {
  const [allLeagues, setAllLeagues] = useState<League[]>([])
  const [activeLeague, setActiveLeague] = useState<League | null>(null)

  // 初始化：加载联赛列表和当前活跃联赛
  useEffect(() => {
    const init = async () => {
      try {
        const leagues = await GetLeagues()
        if (leagues && leagues.length > 0) {
          setAllLeagues(leagues)

          // 获取后端当前活跃联赛
          const activeSportKey = await GetActiveLeague()
          const current = leagues.find(l => l.sportKey === activeSportKey) || leagues[0]
          setActiveLeague(current)
        }
      } catch (err) {
        console.error('初始化联赛数据失败:', err)
      }
    }
    init()
  }, [])

  const switchLeague = async (sportKey: string) => {
    try {
      await SetActiveLeague(sportKey)
      const league = allLeagues.find(l => l.sportKey === sportKey)
      if (league) {
        setActiveLeague(league)
      }
    } catch (err) {
      console.error('切换联赛失败:', err)
    }
  }

  return (
    <ErrorBoundary fallbackLabel="应用根组件">
      <LeagueContext.Provider value={{ activeLeague, allLeagues, switchLeague }}>
        <HashRouter>
          <AppShell />
        </HashRouter>
      </LeagueContext.Provider>
    </ErrorBoundary>
  )
}
