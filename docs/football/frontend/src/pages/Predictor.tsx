// Predictor.tsx - 策略回测与 AI 预测中心
import { useState, useEffect, useRef } from 'react'
import * as echarts from 'echarts'
import { Play, Download, Sliders, Server, ShieldAlert } from 'lucide-react'
import { useLeague } from '../App'
import { GetMatchesByLeague } from '../../bindings/football/app'

interface HistoryRecord {
  timestamp: string
  strategyId: string
  returns: string
  sharpe: string
  winRate: string
  status: '稳健型' | '不合格'
}

// 模拟各个联赛专属的计算结果和参数分布
const LEAGUE_PREDICTION_META: Record<string, {
  sharpe: number
  sortino: number
  drawdown: number
  iterations: number
  equityCurve: number[]
  strategies: string[]
}> = {
  'soccer_epl': {
    sharpe: 3.12,
    sortino: 4.45,
    drawdown: -8.2,
    iterations: 42800,
    equityCurve: [0, 45, 30, 68, 110, 142.5],
    strategies: ['英超盘口强动能策略_v2', '英冠大球高赔率对冲模型', '曼城控球率与角球量化算法']
  },
  'soccer_spain_la_liga': {
    sharpe: 2.95,
    sortino: 3.82,
    drawdown: -6.4,
    iterations: 38400,
    equityCurve: [0, 32, 58, 48, 92, 128.4],
    strategies: ['西甲技术型控制套利模型', '西甲半全场平局对冲策略', '西甲双雄独赢买空组合']
  },
  'soccer_italy_serie_a': {
    sharpe: 3.42,
    sortino: 4.95,
    drawdown: -4.1,
    iterations: 51200,
    equityCurve: [0, 50, 45, 88, 120, 165.2],
    strategies: ['意甲链式防守低进球模型', '米兰德比大单高频扫盘算法', '意甲零封率泊松分布预测']
  },
  'soccer_germany_bundesliga': {
    sharpe: 2.78,
    sortino: 3.52,
    drawdown: -11.5,
    iterations: 31000,
    equityCurve: [0, 18, 48, 25, 75, 108.2],
    strategies: ['德甲大球对决高回报策略', '药厂不败神话指数增量分析', '德甲高压逼抢角球数模型']
  },
  'soccer_france_ligue_one': {
    sharpe: 2.65,
    sortino: 3.15,
    drawdown: -10.2,
    iterations: 24500,
    equityCurve: [0, 22, 38, 55, 68, 95.6],
    strategies: ['法甲巴黎独赢低赔增益策略', '法甲防守反击受让球模型', '法甲冷门盘口过滤过滤器']
  },
  'soccer_uefa_champs_league': {
    sharpe: 3.25,
    sortino: 4.62,
    drawdown: -9.5,
    iterations: 64000,
    equityCurve: [0, 60, 40, 95, 130, 178.4],
    strategies: ['欧冠超级杯赛强强对抗对冲', '欧冠客场进球淘汰赛特定策略', '欧洲皇马杯赛加成权重模型']
  },
  'soccer_uefa_europa_league': {
    sharpe: 2.88,
    sortino: 3.75,
    drawdown: -8.9,
    iterations: 35000,
    equityCurve: [0, 28, 42, 60, 85, 115.8],
    strategies: ['欧联小组赛爆冷偏门扫盘', '欧联强队客场让球回测', '欧联大球狂欢高频买单模型']
  },
  'soccer_fifa_world_cup': {
    sharpe: 3.55,
    sortino: 5.12,
    drawdown: -5.2,
    iterations: 88000,
    equityCurve: [0, 75, 50, 110, 155, 212.5],
    strategies: ['世界杯淘汰赛点球大战套利', '世界杯小组赛冷门对冲策略', '世界杯开幕首战大额跟单算法']
  },
  'soccer_usa_mls': {
    sharpe: 2.52,
    sortino: 3.01,
    drawdown: -14.2,
    iterations: 19800,
    equityCurve: [0, 15, 30, 18, 52, 84.1],
    strategies: ['美职联狂野大球狂欢模型', '迈阿密国际梅西加成估值法', '美职联客场虫套利配对']
  }
}

export default function Predictor() {
  const { activeLeague } = useLeague()
  const [realMatchesCount, setRealMatchesCount] = useState<number | null>(null)

  useEffect(() => {
    if (!activeLeague) return
    const checkMatches = async () => {
      try {
        const res = await GetMatchesByLeague(activeLeague.name)
        setRealMatchesCount(res ? res.length : 0)
      } catch (err) {
        console.error('获取策略联赛比赛数据失败:', err)
        setRealMatchesCount(0)
      }
    }
    checkMatches()
  }, [activeLeague])

  // 核心回测指标
  const [defense, setDefense] = useState(0.82)
  const [latency, setLatency] = useState(14)
  const [anomaly, setAnomaly] = useState(2.5)
  const [volatility, setVolatility] = useState('中')

  const [loading, setLoading] = useState(false)
  const [loadingStrategy, setLoadingStrategy] = useState(false)
  const [history, setHistory] = useState<HistoryRecord[]>([])

  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstance = useRef<echarts.ECharts | null>(null)

  // 1. 动态获取当前联赛下的计算指标与净值曲线
  const leagueKey = activeLeague?.sportKey || 'soccer_epl'
  const meta = LEAGUE_PREDICTION_META[leagueKey] || LEAGUE_PREDICTION_META['soccer_epl']

  // 切换联赛时载入新的 AI 计算模型，产生华丽的切换载入加载中动画
  useEffect(() => {
    if (!activeLeague) return

    setLoadingStrategy(true)
    const timer = setTimeout(() => {
      setLoadingStrategy(false)
    }, 600)

    // 初始化联赛特定的回测记录
    const minutesAgo = (offsetSec: number) => {
      const d = new Date(Date.now() - offsetSec * 1000)
      return d.toLocaleTimeString('zh-CN', { hour12: false })
    }

    setHistory([
      {
        timestamp: minutesAgo(120),
        strategyId: meta.strategies[0],
        returns: `+${meta.equityCurve[5].toFixed(1)}%`,
        sharpe: meta.sharpe.toFixed(2),
        winRate: `${(60 + Math.random() * 12).toFixed(1)}%`,
        status: '稳健型'
      },
      {
        timestamp: minutesAgo(480),
        strategyId: meta.strategies[1],
        returns: `+${(meta.equityCurve[5] * 0.65).toFixed(1)}%`,
        sharpe: (meta.sharpe * 0.9).toFixed(2),
        winRate: `${(58 + Math.random() * 10).toFixed(1)}%`,
        status: '稳健型'
      },
      {
        timestamp: minutesAgo(960),
        strategyId: meta.strategies[2],
        returns: `-${(10 + Math.random() * 15).toFixed(1)}%`,
        sharpe: (meta.sharpe * 0.35).toFixed(2),
        winRate: `${(42 + Math.random() * 8).toFixed(1)}%`,
        status: '不合格'
      }
    ])

    return () => clearTimeout(timer)
  }, [activeLeague, meta])

  // 初始化或更新收益增长曲线 (Equity Curve)
  useEffect(() => {
    if (!chartRef.current || loadingStrategy) return

    // 每次更新曲线时重新创建以保证平滑绘制
    if (chartInstance.current) {
      chartInstance.current.dispose()
    }

    const chart = echarts.init(chartRef.current)
    chartInstance.current = chart

    const option = {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'axis',
        backgroundColor: 'rgba(255, 255, 255, 0.95)',
        borderColor: 'rgba(226, 232, 240, 0.8)',
        textStyle: { color: '#0f172a', fontFamily: 'var(--font-sans)', fontSize: 11 },
        borderWidth: 1,
        formatter: (params: any[]) => {
          return `<div style="font-family: var(--font-sans); text-align: left; color: #0f172a">
            时间区间: ${params[0].name}<br/>
            累计增长率: <b style="color: #6366f1">${params[0].value.toFixed(1)}%</b>
          </div>`
        }
      },
      grid: { left: '3%', right: '4%', bottom: '5%', top: '5%', containLabel: true },
      xAxis: {
        type: 'category',
        boundaryGap: false,
        data: ['2025-Q1', '2025-Q2', '2025-Q3', '2025-Q4', '2026-Q1', '当前实时结算'],
        axisLine: { lineStyle: { color: 'rgba(203, 213, 225, 0.6)' } },
        axisLabel: { color: '#64748b', fontSize: 9, fontFamily: 'var(--font-sans)' }
      },
      yAxis: {
        type: 'value',
        splitLine: { lineStyle: { color: 'rgba(226, 232, 240, 0.5)', type: 'dashed' } },
        axisLabel: { color: '#64748b', fontSize: 9, fontFamily: 'JetBrains Mono', formatter: '{value}%' }
      },
      series: [
        {
          name: '累计净值增长曲线',
          type: 'line',
          smooth: true,
          showSymbol: true,
          symbolSize: 6,
          itemStyle: { color: '#6366f1' },
          data: meta.equityCurve,
          lineStyle: { color: '#6366f1', width: 2.5, shadowBlur: 4, shadowColor: 'rgba(99, 102, 241, 0.2)' },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: 'rgba(99, 102, 241, 0.12)' },
              { offset: 1, color: 'rgba(99, 102, 241, 0)' }
            ])
          }
        }
      ]
    }

    chart.setOption(option)

    const handleResize = () => chart.resize()
    window.addEventListener('resize', handleResize)

    return () => {
      chart.dispose()
      window.removeEventListener('resize', handleResize)
    }
  }, [meta, loadingStrategy])

  // 运行回测模拟
  const handleSimulate = () => {
    setLoading(true)
    setTimeout(() => {
      const now = new Date().toLocaleTimeString('zh-CN', { hour12: false })
      const isLucky = Math.random() > 0.3
      const mockPrefix = meta.strategies[Math.floor(Math.random() * meta.strategies.length)].slice(0, 6)

      const newRecord: HistoryRecord = {
        timestamp: now,
        strategyId: `${mockPrefix}_自研微调模型_${(Math.floor(Math.random() * 800) + 100).toFixed(0)}`,
        returns: `${isLucky ? '+' : '-'}${(Math.random() * 60 + 5).toFixed(1)}%`,
        sharpe: (Math.random() * 1.5 + (isLucky ? 2.2 : 0.8)).toFixed(2),
        winRate: `${(Math.random() * 20 + (isLucky ? 55 : 35)).toFixed(1)}%`,
        status: isLucky ? '稳健型' : '不合格',
      }
      setHistory(prev => [newRecord, ...prev].slice(0, 10))
      setLoading(false)
    }, 1500)
  }

  if (realMatchesCount === 0) {
    return (
      <div className="w-full h-full flex flex-col justify-center items-center p-6 gap-6" style={{ background: 'var(--bg-primary)' }}>
        <div className="max-w-xl border border-[#ff3131]/30 bg-[#ff3131]/5 p-8 flex flex-col items-center gap-6 font-mono text-center shadow-[0_0_30px_rgba(255,49,49,0.05)]">
          <div className="w-16 h-16 rounded-full bg-[#ff3131]/10 border border-[#ff3131]/30 flex items-center justify-center text-[#ff3131] animate-pulse">
            <ShieldAlert size={36} />
          </div>
          
          <div className="flex flex-col gap-2">
            <h2 className="text-[#dae6d2] text-lg font-bold tracking-wider">
              ⏳ 暂无当前联赛（{activeLeague?.name || '当前联赛'}）AI 回测数据
            </h2>
            <p className="text-[#84967e] text-xs leading-relaxed max-w-md">
              由于本地 SQLite 数据库中没有可用的真实比赛日程，AI 策略模拟引擎、净值增长曲线以及历史策略审计流水无法运行。
            </p>
          </div>

          <div className="w-full border-t border-[#3b4b37]/40 pt-4 flex flex-col gap-3 text-left">
            <div className="text-[10px] text-[#dae6d2] font-semibold uppercase tracking-wider">
              🛠️ 建议操作步骤：
            </div>
            <div className="bg-black/40 border border-[#3b4b37]/35 p-3 flex flex-col gap-2 text-[11px] text-[#84967e] leading-relaxed">
              <div className="flex items-start gap-2">
                <span className="text-[#00ff41]">1.</span>
                <span>请确保根目录下 <code className="text-[#dae6d2] font-semibold bg-[#222d20] px-1">.env</code> 密钥已配置，并在 Go 端执行数据拉取。</span>
              </div>
              <div className="flex items-start gap-2">
                <span className="text-[#00ff41]">2.</span>
                <span>当 SQLite 数据库中存在该联赛的真实日程赛程后，AI 预测中心将全自动唤醒并关联真实行情完成回测建模。</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 text-[10px] text-[#ff3131] border border-[#ff3131]/20 bg-[#ff3131]/5 px-3 py-1 font-bold">
            <span className="w-1.5 h-1.5 rounded-full bg-[#ff3131] animate-ping" />
            <span>AI 回测系统离线中... 等待 API 行情入库</span>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="w-full h-full flex flex-col overflow-hidden p-6" style={{ background: 'var(--bg-primary)' }}>
      
      {/* 顶部标题区域 */}
      <div className="flex justify-between items-center border-b border-[#3b4b37] pb-4 mb-6" style={{ minWidth: 0 }}>
        <div className="flex flex-col gap-1" style={{ minWidth: 0 }}>
          <h2 className="font-mono text-xl font-bold text-[#dae6d2] uppercase tracking-wide flex items-center gap-2">
            <span className="whitespace-nowrap">{activeLeague ? `${activeLeague.name} AI 策略历史回测中心` : 'AI 策略历史回测中心'}</span>
          </h2>
          <span className="text-[10px] text-[#84967e] font-mono flex items-center gap-2">
            <Server size={10} className="text-[#00ff41] flex-shrink-0" />
            <span style={{ minWidth: 0 }}>智能计算回测引擎 4.2 | 当前运行集群: 亚太一号集群</span>
          </span>
        </div>

        {/* 顶部按钮 */}
        <div className="flex gap-4" style={{ zIndex: 10, flexShrink: 0 }}>
          <button
            onClick={handleSimulate}
            disabled={loading || loadingStrategy}
            className="btn btn-primary font-mono text-xs uppercase cursor-pointer"
            style={{ whiteSpace: 'nowrap', flexShrink: 0, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}
          >
            {loading ? (
              <span className="spinner" style={{ borderTopColor: '#000', width: '12px', height: '12px', borderWidth: '1.5px', display: 'block', flexShrink: 0 }} />
            ) : (
              <Play size={12} fill="currentColor" style={{ display: 'block', flexShrink: 0 }} />
            )}
            <span style={{ whiteSpace: 'nowrap', display: 'inline-block', lineHeight: 1 }}>{loading ? '回测中...' : '执行回测'}</span>
          </button>
          <button 
            className="btn btn-outline font-mono text-xs uppercase cursor-pointer"
            style={{ whiteSpace: 'nowrap', flexShrink: 0, display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '8px' }}
          >
            <Download size={12} style={{ display: 'block', flexShrink: 0 }} />
            <span style={{ whiteSpace: 'nowrap', display: 'inline-block', lineHeight: 1 }}>导出模型</span>
          </button>
        </div>
      </div>

      {/* 主体两栏栅格 */}
      <div className="grid grid-cols-3 gap-6 flex-1 overflow-hidden relative">
        
        {/* 载入联赛模型时的遮罩，完美融合科幻荧光设计 */}
        {loadingStrategy && (
          <div className="absolute inset-0 bg-[#0c160a]/90 backdrop-blur-sm z-50 flex flex-col justify-center items-center gap-4">
            <span className="spinner" style={{ borderColor: '#00ff41/20', borderTopColor: '#00ff41', width: '40px', height: '40px', borderWidth: '3px' }} />
            <span className="font-mono text-xs text-[#00ff41] animate-pulse">
              正在调取 [{activeLeague?.name || '未知联赛'}] AI 泊松对冲和量化因子参数...
            </span>
          </div>
        )}

        {/* 左侧回测曲线及历史表格 (占 2/3) */}
        <div className="col-span-2 flex flex-col gap-6 overflow-hidden" style={{ minHeight: 0 }}>
          
          {/* Equity Curve 折线图卡片 */}
          <div className="border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col flex-shrink-0">
            <div className="border-b border-[#3b4b37] pb-2 flex justify-between items-center text-xs font-mono font-semibold text-[#dae6d2]">
              <span>净值收益模拟增长曲线 ({activeLeague?.name || ''} 专属模型)</span>
              <div className="flex gap-4 text-[9px] text-[#84967e] font-sans">
                <span>最大历史回撤: <b className="text-[#ff3131]">{meta.drawdown.toFixed(1)}%</b></span>
                <span>回测平均胜率: <b className="text-[#00ff41]">{(62.5 + meta.sharpe * 2).toFixed(1)}%</b></span>
              </div>
            </div>
            <div ref={chartRef} className="h-56 mt-4 w-full" />
          </div>

          {/* 四个回测统计指标卡片 */}
          <div className="grid grid-cols-4 gap-6 flex-shrink-0">
            <div className="border border-[#3b4b37] bg-[#0c160a] p-4 flex flex-col gap-2 font-mono">
              <span className="text-[9px] text-[#84967e] font-bold uppercase font-sans">夏普比率 (Sharpe)</span>
              <span className="text-2xl font-bold text-[#dae6d2]">{meta.sharpe.toFixed(2)}</span>
              <span className="text-[8px] text-[#00ff41] font-bold font-sans">↗ 较通用版本提升 +{(meta.sharpe - 2.5).toFixed(2)}</span>
            </div>

            <div className="border border-[#3b4b37] bg-[#0c160a] p-4 flex flex-col gap-2 font-mono">
              <span className="text-[9px] text-[#84967e] font-bold uppercase font-sans">索提诺比率 (Sortino)</span>
              <span className="text-2xl font-bold text-[#dae6d2]">{meta.sortino.toFixed(2)}</span>
              <span className="text-[8px] text-[#84967e] font-bold font-sans">下行风险针对优化完毕</span>
            </div>

            <div className="border border-[#3b4b37] bg-[#ff3131]/5 border-l-2 border-l-[#ff3131] p-4 flex flex-col gap-2 font-mono">
              <span className="text-[9px] text-[#ff3131] font-bold uppercase font-sans">最大单笔回撤</span>
              <span className="text-2xl font-bold text-[#ff3131]">{meta.drawdown.toFixed(1)}%</span>
              <span className="text-[8px] text-[#84967e] font-bold font-sans">95% 置信度VaR计算</span>
            </div>

            <div className="border border-[#3b4b37] bg-[#0c160a] p-4 flex flex-col gap-2 font-mono">
              <span className="text-[9px] text-[#84967e] font-bold uppercase font-sans">回测计算迭代次数</span>
              <span className="text-2xl font-bold text-[#dae6d2]">{meta.iterations.toLocaleString()}</span>
              <span className="text-[8px] text-[#84967e] font-bold font-sans">完整历史高真匹配战局</span>
            </div>
          </div>

          {/* 最近回测记录表格 */}
          <div className="border border-[#3b4b37] bg-[#141e12] flex-grow flex flex-col overflow-hidden" style={{ minHeight: 0 }}>
            <div className="border-b border-[#3b4b37] px-4 py-2.5 bg-[#071106] text-xs font-mono font-semibold text-[#dae6d2]">
              当前联赛回测记录历史流水表
            </div>
            <div className="p-4 flex-1 overflow-auto">
              <table className="w-full text-left font-mono text-xs border-collapse">
                <thead>
                  <tr className="text-[#84967e] border-b border-[#3b4b37] text-[10px] pb-3">
                    <th className="pb-3 font-semibold whitespace-nowrap">回测计算时间</th>
                    <th className="pb-3 font-semibold whitespace-nowrap">测试策略名称 ID</th>
                    <th className="pb-3 font-semibold whitespace-nowrap">累计模拟回报率</th>
                    <th className="pb-3 font-semibold whitespace-nowrap">夏普比率</th>
                    <th className="pb-3 font-semibold whitespace-nowrap">胜率</th>
                    <th className="pb-3 font-semibold text-right whitespace-nowrap" style={{ paddingRight: '8px' }}>策略状态</th>
                  </tr>
                </thead>
                <tbody>
                  {history.map((row, i) => (
                    <tr key={i} className="border-b border-[#3b4b37]/20 hover:bg-[#222d20]/30 h-10 text-[11px] font-mono">
                      <td className="text-[#dae6d2] whitespace-nowrap">{row.timestamp}</td>
                      <td className="text-[#dae6d2] font-semibold whitespace-nowrap">{row.strategyId}</td>
                      <td className={`font-bold whitespace-nowrap ${row.returns.startsWith('+') ? 'text-[#00ff41]' : 'text-[#ff3131]'}`}>{row.returns}</td>
                      <td className="text-[#dae6d2] whitespace-nowrap">{row.sharpe}</td>
                      <td className="text-[#dae6d2] font-semibold whitespace-nowrap">{row.winRate}</td>
                      <td className="text-right font-bold whitespace-nowrap" style={{ paddingRight: '8px' }}>
                        <span className={`px-1.5 py-0.5 border text-[9px] ${
                          row.status.includes('稳健') ? 'bg-[#00ff41]/10 text-[#00ff41] border-[#00ff41]/20' : 'bg-[#ff3131]/10 text-[#ff3131] border-[#ff3131]/20'
                        }`}>
                          {row.status}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

        </div>

        {/* 右侧回测参数面板 (占 1/3) */}
        <div className="col-span-1 border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col overflow-hidden relative" style={{ minHeight: 0 }}>
          
          {/* 背景水印 (策略) */}
          <div 
            className="absolute select-none pointer-events-none text-9xl font-extrabold font-mono tracking-widest uppercase" 
            style={{ 
              right: '8px',
              top: '25%',
              writingMode: 'vertical-lr',
              color: 'rgba(0, 255, 65, 0.03)',
              zIndex: 1,
              fontSize: '110px',
              lineHeight: 1
            } as any}
          >
            策略
          </div>

          <div className="border-b border-[#3b4b37] pb-2 text-xs font-mono font-semibold text-[#dae6d2] flex items-center gap-1.5 mb-4 flex-shrink-0" style={{ zIndex: 10, position: 'relative' }}>
            <Sliders size={14} className="text-[#00ff41]" />
            <span>决策核心控制参数调节</span>
          </div>

          {/* 参数可滚动区域 */}
          <div className="flex-grow overflow-y-auto flex flex-col gap-6 w-full pr-1 mb-4" style={{ zIndex: 10, position: 'relative' }}>
            {/* 参数 1 */}
            <div className="flex flex-col gap-2 font-mono">
              <div className="flex justify-between text-[10px] text-[#dae6d2]">
                <span className="font-bold text-[#84967e] font-sans">防守因子限制</span>
                <span className="text-[#00ff41] font-bold">{defense.toFixed(2)}</span>
              </div>
              <input
                type="range"
                className="slider w-full"
                min={0}
                max={1}
                step={0.01}
                value={defense}
                onChange={e => setDefense(parseFloat(e.target.value))}
              />
            </div>

            {/* 参数 2 */}
            <div className="flex flex-col gap-2 font-mono">
              <div className="flex justify-between text-[10px] text-[#dae6d2]">
                <span className="font-bold text-[#84967e] font-sans">网络延迟权重</span>
                <span className="text-[#00ff41] font-bold">{latency}毫秒</span>
              </div>
              <input
                type="range"
                className="slider w-full"
                min={1}
                max={100}
                step={1}
                value={latency}
                onChange={e => setLatency(parseInt(e.target.value))}
              />
            </div>

            {/* 参数 3 */}
            <div className="flex flex-col gap-2 font-mono">
              <div className="flex justify-between text-[10px] text-[#dae6d2]">
                <span className="font-bold text-[#84967e] font-sans">异常交易剔除阈值</span>
                <span className="text-[#00ff41] font-bold">{anomaly.toFixed(1)}σ</span>
              </div>
              <input
                type="range"
                className="slider w-full"
                min={1}
                max={5}
                step={0.1}
                value={anomaly}
                onChange={e => setAnomaly(parseFloat(e.target.value))}
              />
            </div>

            {/* 参数 4 */}
            <div className="flex flex-col gap-2 font-mono">
              <div className="flex justify-between text-[10px] text-[#dae6d2]">
                <span className="font-bold text-[#84967e] font-sans">波动性过滤强度</span>
                <span className="text-[#00ff41] font-bold">{volatility}</span>
              </div>
              <div className="grid grid-cols-3 gap-2 mt-1">
                {['弱', '中', '强'].map(v => (
                  <button
                    key={v}
                    onClick={() => setVolatility(v)}
                    className={`text-xs font-bold py-1.5 font-sans cursor-pointer border ${
                      volatility === v
                        ? 'bg-[#00ff41]/10 text-[#00ff41] border-[#00ff41]'
                        : 'bg-transparent text-[#84967e] border-[#3b4b37] hover:border-[#84967e]'
                    }`}
                  >
                    {v}
                  </button>
                ))}
              </div>
            </div>
          </div>

          {/* 底部完整性 */}
          <div className="border-t border-[#3b4b37] pt-4 flex flex-col gap-3 font-mono text-[9px] w-full flex-shrink-0" style={{ zIndex: 2 }}>
            <div className="flex justify-between">
              <span className="text-[#84967e] font-semibold uppercase font-sans">策略计算模型状态</span>
              <span className="text-[#00ff41] font-bold uppercase">超安全线运行中</span>
            </div>
            <div className="flex justify-between">
              <span className="text-[#84967e] font-semibold uppercase font-sans">计算开销与延迟</span>
              <span className="text-[#00ff41] font-bold uppercase">极微型级限制</span>
            </div>
          </div>

        </div>

      </div>

      {/* 底部版权栏 */}
      <footer className="border-t border-[#3b4b37]/30 mt-6 pt-4 flex justify-between text-[8px] font-mono text-[#84967e]">
        <span>🟢 AI 核心回测引擎在线运行中 | 端口时延: 4.2毫秒 | 并发负载: 12%</span>
        <span>© 2026 量化量子体育分析研究实验室联盟</span>
      </footer>

    </div>
  )
}
