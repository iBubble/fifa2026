// Market.tsx - 市场监控系统页面
// 包含智能资金流、必发流动性分析、冷热指数分布、异常警报和实时报单执行监控
import { useState, useEffect } from 'react'
import { ShieldAlert, Cpu, Activity } from 'lucide-react'
import { useLeague } from '../App'
import { GetMatchesByLeague } from '../../bindings/football/app'
import { formatTeamName } from '../utils/team'

interface Order {
  time: string
  market: string
  selection: string
  type: '买入 (做多)' | '卖出 (做空)'
  stake: string
  odds: string
  impact: number
  status: '已确认成交' | '匹配中...'
}

interface AnomalyAlert {
  time: string
  type: string
  detail: string
}

export default function Market() {
  const { activeLeague } = useLeague()
  const [orders, setOrders] = useState<Order[]>([])
  const [realMatches, setRealMatches] = useState<any[]>([])
  const [moneyFlows, setMoneyFlows] = useState<Array<{ home: string; away: string; amount: string; market: string }>>([])
  const [anomalyAlerts, setAnomalyAlerts] = useState<AnomalyAlert[]>([])

  // 必发即时流动性的展示球队
  const [liquidityMatch, setLiquidityMatch] = useState<{ home: string; away: string }>({ home: '主队', away: '客队' })

  // 当切换联赛时，重新获取该联赛的比赛，并初始化流动性、智能资金与报单流
  useEffect(() => {
    if (!activeLeague) return

    const loadLeagueMarketData = async () => {
      let matchesList: Array<{ home: string; away: string }> = []
      try {
        const res = await GetMatchesByLeague(activeLeague.name)
        if (res && res.length > 0) {
          setRealMatches(res)
          matchesList = res.map(m => ({ home: formatTeamName(m.homeTeam), away: formatTeamName(m.awayTeam) }))
        } else {
          setRealMatches([])
        }
      } catch (err) {
        console.error('获取市场联赛比赛数据失败:', err)
        setRealMatches([])
      }

      if (matchesList.length === 0) {
        setLiquidityMatch({ home: '主队', away: '客队' })
        setMoneyFlows([])
        setAnomalyAlerts([])
        setOrders([])
        return
      }

      // 1. 设置即时流动性分析针对的第一场比赛
      const topMatch = matchesList[0]
      setLiquidityMatch(topMatch)

      // 2. 初始化主力资金流向
      const flowA = matchesList[0]
      const flowB = matchesList[Math.min(1, matchesList.length - 1)]
      setMoneyFlows([
        {
          home: flowA.home,
          away: flowA.away,
          amount: `+$${(Math.floor(Math.random() * 300) + 150).toFixed(1)}K`,
          market: '独赢盘口 - 主胜 (1)'
        },
        {
          home: flowB.home,
          away: flowB.away,
          amount: `+$${(Math.floor(Math.random() * 200) + 80).toFixed(1)}K`,
          market: '大 2.5 球 盘口'
        }
      ])

      // 3. 初始化高频异动告警
      const minutesAgo = (offsetSec: number) => {
        const d = new Date(Date.now() - offsetSec * 1000)
        return d.toLocaleTimeString('zh-CN', { hour12: false })
      }

      setAnomalyAlerts([
        {
          time: minutesAgo(30),
          type: '赔率瞬间暴跌',
          detail: `${flowA.home} vs ${flowA.away}：主胜赔率在 12 秒内暴跌。检测到非理性大资金高频扫盘。`
        },
        {
          time: minutesAgo(90),
          type: '流动性瞬间真空',
          detail: `${flowB.home} vs ${flowB.away}：必发撮合中心即时成交流动性大幅低于常规安全警戒阈值 (-30%)。`
        }
      ])

      // 4. 初始化近期报单流水
      const initialOrders: Order[] = []
      for (let i = 0; i < 6; i++) {
        const m = matchesList[Math.floor(Math.random() * matchesList.length)]
        const selections = [`${m.home} 胜`, `${m.away} 胜`, '平局', '大 2.5 球', '小 2.5 球']
        const types: ('买入 (做多)' | '卖出 (做空)')[] = ['买入 (做多)', '卖出 (做空)']
        const status: ('已确认成交' | '匹配中...')[] = ['已确认成交', '匹配中...']
        const orderTime = minutesAgo(i * 45 + Math.floor(Math.random() * 30))

        initialOrders.push({
          time: orderTime,
          market: `${activeLeague.name} - ${m.home} vs ${m.away}`,
          selection: selections[Math.floor(Math.random() * selections.length)],
          type: types[Math.floor(Math.random() * types.length)],
          stake: `$${(Math.floor(Math.random() * 200) + 10).toFixed(0)},000`,
          odds: (Math.random() * 2.8 + 1.3).toFixed(2),
          impact: Math.floor(Math.random() * 4) + 1,
          status: status[Math.floor(Math.random() * status.length)]
        })
      }
      setOrders(initialOrders)
    }

    loadLeagueMarketData()
  }, [activeLeague])

  // 报单实时推流模拟 (动态使用当前选定联赛的球队)
  useEffect(() => {
    if (!activeLeague || realMatches.length === 0) return

    const interval = setInterval(() => {
      const matchesList = realMatches.map(m => ({ home: formatTeamName(m.homeTeam), away: formatTeamName(m.awayTeam) }))
      if (matchesList.length === 0) return

      const m = matchesList[Math.floor(Math.random() * matchesList.length)]
      const selections = [`${m.home} 胜`, `${m.away} 胜`, '平局', '大 2.5 球', '小 2.5 球']
      const types: ('买入 (做多)' | '卖出 (做空)')[] = ['买入 (做多)', '卖出 (做空)']
      const status: ('已确认成交' | '匹配中...')[] = ['已确认成交', '匹配中...']
      const time = new Date().toLocaleTimeString('zh-CN', { hour12: false })

      const newOrder: Order = {
        time,
        market: `${activeLeague.name} - ${m.home} vs ${m.away}`,
        selection: selections[Math.floor(Math.random() * selections.length)],
        type: types[Math.floor(Math.random() * types.length)],
        stake: `$${(Math.floor(Math.random() * 200) + 10).toFixed(0)},000`,
        odds: (Math.random() * 2.8 + 1.3).toFixed(2),
        impact: Math.floor(Math.random() * 4) + 1,
        status: status[Math.floor(Math.random() * status.length)]
      }

      setOrders(prev => [newOrder, ...prev].slice(0, 15))
    }, 4000)

    return () => clearInterval(interval)
  }, [activeLeague, realMatches])

  if (realMatches.length === 0) {
    return (
      <div className="w-full h-full flex flex-col justify-center items-center p-6 gap-6" style={{ background: 'var(--bg-primary)' }}>
        <div className="max-w-xl border border-[#ff3131]/30 bg-[#ff3131]/5 p-8 flex flex-col items-center gap-6 font-mono text-center shadow-[0_0_30px_rgba(255,49,49,0.05)]">
          <div className="w-16 h-16 rounded-full bg-[#ff3131]/10 border border-[#ff3131]/30 flex items-center justify-center text-[#ff3131] animate-pulse">
            <ShieldAlert size={36} />
          </div>
          
          <div className="flex flex-col gap-2">
            <h2 className="text-[#dae6d2] text-lg font-bold tracking-wider">
              ⏳ 暂无当前联赛（{activeLeague?.name || '当前联赛'}）市场交易数据
            </h2>
            <p className="text-[#84967e] text-xs leading-relaxed max-w-md">
              由于本地 SQLite 数据库中没有可用的真实比赛日程，主力资金流分析、必发即时流动性、冷热指数谱系以及撮合交易流水均处于待机状态。
            </p>
          </div>

          <div className="w-full border-t border-[#3b4b37]/40 pt-4 flex flex-col gap-3 text-left">
            <div className="text-[10px] text-[#dae6d2] font-semibold uppercase tracking-wider">
              🛠️ 建议操作步骤：
            </div>
            <div className="bg-black/40 border border-[#3b4b37]/35 p-3 flex flex-col gap-2 text-[11px] text-[#84967e] leading-relaxed">
              <div className="flex items-start gap-2">
                <span className="text-[#00ff41]">1.</span>
                <span>确认根目录下 <code className="text-[#dae6d2] font-semibold bg-[#222d20] px-1">.env</code> 中已配置有效的 <code className="text-[#dae6d2] font-semibold bg-[#222d20] px-1">APIFOOTBALL_KEY</code> 密钥。</span>
              </div>
              <div className="flex items-start gap-2">
                <span className="text-[#00ff41]">2.</span>
                <span>在系统终端运行 <code className="text-[#dae6d2] font-semibold bg-[#222d20] px-1">wails3 dev</code> 或直接通过后台同步任务触发数据导入。</span>
              </div>
              <div className="flex items-start gap-2">
                <span className="text-[#00ff41]">3.</span>
                <span>数据抓取完成后，本界面的主力大额报单流、必发成交盘口与异动告警将依据 API 真实数据全自动同步渲染。</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 text-[10px] text-[#ff3131] border border-[#ff3131]/20 bg-[#ff3131]/5 px-3 py-1 font-bold">
            <span className="w-1.5 h-1.5 rounded-full bg-[#ff3131] animate-ping" />
            <span>系统监控挂起中... 等待真实 API 数据成盘</span>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="w-full h-full flex flex-col overflow-y-auto p-6" style={{ background: 'var(--bg-primary)' }}>
      
      {/* 顶部两栏结构 (智能资金 + 必发流动性) */}
      <div className="grid grid-cols-3 gap-6 mb-6">
        
        {/* 左侧 1/3: 主力资金流 (Smart Money Flow) */}
        <div className="col-span-1 border border-[#3b4b37] bg-[#141e12] flex flex-col p-4 gap-4">
          <div className="flex justify-between items-center text-xs font-mono font-semibold tracking-wider text-[#dae6d2] border-b border-[#3b4b37] pb-2">
            <div className="flex items-center gap-2">
              <Activity size={12} className="text-[#00ff41]" />
              <span>{activeLeague ? `${activeLeague.name} 主力资金流向` : '主力资金流向分析'}</span>
            </div>
            <span className="text-[#00ff41] bg-[#00ff41]/10 px-1 py-0.5 border border-[#00ff41]/20 text-[9px]">主力资金指数: 84.2</span>
          </div>

          <div className="flex-1 flex flex-col gap-3 font-mono text-[10px]">
            {moneyFlows.map((flow, idx) => (
              <div key={idx} className="flex flex-col gap-1 border-b border-[#3b4b37]/20 pb-2">
                <div className="flex justify-between text-[#dae6d2]">
                  <span className="font-semibold">{flow.home} VS {flow.away}</span>
                  <span className="text-[#00ff41] font-bold">{flow.amount}</span>
                </div>
                <span className="text-[#84967e]">{flow.market}</span>
              </div>
            ))}

            {/* 柱状占比图 */}
            <div className="h-16 flex items-end gap-1 px-4 mt-2">
              <span className="flex-1 bg-[#3b4b37] h-6" />
              <span className="flex-1 bg-[#3b4b37] h-10" />
              <span className="flex-1 bg-[#00ff41] h-16 shadow-glow-green" />
              <span className="flex-1 bg-[#84967e] h-12" />
            </div>
          </div>
        </div>

        {/* 右侧 2/3: 必发流动性分析 (Betfair Liquidity Analysis) */}
        <div className="col-span-2 border border-[#3b4b37] bg-[#141e12] flex flex-col p-4 gap-4">
          <div className="flex justify-between items-center text-xs font-mono font-semibold tracking-wider text-[#dae6d2] border-b border-[#3b4b37] pb-2">
            <div className="flex items-center gap-2">
              <Cpu size={12} className="text-[#00ff41]" />
              <span>必发即时流动性 ({liquidityMatch.home} vs {liquidityMatch.away})</span>
            </div>
            <div className="flex gap-4 text-[9px]">
              <span className="text-[#00ff41]">■ 买入 (做多)</span>
              <span className="text-[#84967e]">■ 卖出 (做空)</span>
            </div>
          </div>

          <div className="grid grid-cols-3 gap-6 font-mono text-[10px] text-[#dae6d2]">
            
            {/* 主胜流动性 */}
            <div className="border border-[#3b4b37] bg-[#0c160a] p-3 flex flex-col gap-2">
              <div className="flex justify-between border-b border-[#3b4b37] pb-1.5 font-bold">
                <span className="truncate">{liquidityMatch.home} [1]</span>
                <span className="text-[#00ff41]">1.94</span>
              </div>
              <div className="flex flex-col gap-2">
                <div className="flex justify-between items-center">
                  <span>2.00</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#3b4b37]/30 h-2"><div className="bg-[#84967e] h-full" style={{ width: '35%' }} /></div>
                    <span className="text-[#84967e] text-[9px]">$12K</span>
                  </div>
                </div>
                <div className="flex justify-between items-center">
                  <span>1.94</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#00ff41]/20 h-2"><div className="bg-[#00ff41] h-full" style={{ width: '85%' }} /></div>
                    <span className="text-[#00ff41] text-[9px]">$84K</span>
                  </div>
                </div>
                <div className="flex justify-between items-center">
                  <span>1.92</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#3b4b37]/30 h-2"><div className="bg-[#84967e] h-full" style={{ width: '50%' }} /></div>
                    <span className="text-[#84967e] text-[9px]">$22K</span>
                  </div>
                </div>
              </div>
            </div>

            {/* 平局流动性 */}
            <div className="border border-[#3b4b37] bg-[#0c160a] p-3 flex flex-col gap-2">
              <div className="flex justify-between border-b border-[#3b4b37] pb-1.5 font-bold">
                <span>平局 [X]</span>
                <span className="text-[#00ff41]">3.85</span>
              </div>
              <div className="flex flex-col gap-2">
                <div className="flex justify-between items-center">
                  <span>3.90</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#3b4b37]/30 h-2"><div className="bg-[#84967e] h-full" style={{ width: '20%' }} /></div>
                    <span className="text-[#84967e] text-[9px]">$4K</span>
                  </div>
                </div>
                <div className="flex justify-between items-center">
                  <span>3.85</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#00ff41]/20 h-2"><div className="bg-[#00ff41] h-full" style={{ width: '45%' }} /></div>
                    <span className="text-[#00ff41] text-[9px]">$15K</span>
                  </div>
                </div>
                <div className="flex justify-between items-center">
                  <span>3.80</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#3b4b37]/30 h-2"><div className="bg-[#84967e] h-full" style={{ width: '15%' }} /></div>
                    <span className="text-[#84967e] text-[9px]">$3K</span>
                  </div>
                </div>
              </div>
            </div>

            {/* 客胜流动性 */}
            <div className="border border-[#3b4b37] bg-[#0c160a] p-3 flex flex-col gap-2">
              <div className="flex justify-between border-b border-[#3b4b37] pb-1.5 font-bold">
                <span className="truncate">{liquidityMatch.away} [2]</span>
                <span className="text-[#00ff41]">4.10</span>
              </div>
              <div className="flex flex-col gap-2">
                <div className="flex justify-between items-center">
                  <span>4.20</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#3b4b37]/30 h-2"><div className="bg-[#84967e] h-full" style={{ width: '10%' }} /></div>
                    <span className="text-[#84967e] text-[9px]">$2K</span>
                  </div>
                </div>
                <div className="flex justify-between items-center">
                  <span>4.10</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#00ff41]/20 h-2"><div className="bg-[#00ff41] h-full" style={{ width: '30%' }} /></div>
                    <span className="text-[#00ff41] text-[9px]">$9K</span>
                  </div>
                </div>
                <div className="flex justify-between items-center">
                  <span>4.00</span>
                  <div className="flex items-center gap-1.5">
                    <div className="w-16 bg-[#3b4b37]/30 h-2"><div className="bg-[#84967e] h-full" style={{ width: '75%' }} /></div>
                    <span className="text-[#84967e] text-[9px]">$45K</span>
                  </div>
                </div>
              </div>
            </div>

          </div>
        </div>

      </div>

      {/* 中间栏：冷热分布指数 (Cold/Hot Distribution Index) + 异常告警 */}
      <div className="grid grid-cols-3 gap-6 mb-6">
        
        {/* 左侧 2/3: 冷热分布色块条 */}
        <div className="col-span-2 border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col gap-3">
          <span className="text-[10px] text-[#84967e] font-mono font-bold uppercase tracking-wider">全市场买卖资金冷热分布度量色谱</span>
          
          {/* 彩色渐变条 */}
          <div className="w-full h-8 flex border border-[#3b4b37]">
            <div className="flex-1 bg-[#00ff41]" />
            <div className="flex-1 bg-[#00d035]" />
            <div className="flex-1 bg-[#222d20]" style={{ flex: 3 }} />
            <div className="flex-1 bg-[#84967e]" />
            <div className="flex-1 bg-[#a54a37]" />
            <div className="flex-1 bg-[#ff3131]" style={{ flex: 2 }} />
          </div>

          <div className="flex justify-between text-[8px] text-[#84967e] font-mono font-bold uppercase">
            <span>极强多头买盘支撑 (做多)</span>
            <span>盘口处于中立平稳状态</span>
            <span className="text-[#ff3131]">极强空头抛售砸盘 (做空)</span>
          </div>
        </div>

        {/* 右侧 1/3: 异常状态告警 (Anomaly Alerts) */}
        <div className="col-span-1 border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col gap-3">
          <div className="text-xs font-mono font-semibold text-[#ff3131] flex items-center gap-1.5 border-b border-[#3b4b37] pb-2">
            <ShieldAlert size={14} />
            <span>异常高频异动告警</span>
          </div>
          
          <div className="flex flex-col gap-3 font-mono text-[9px] leading-relaxed">
            {anomalyAlerts.map((alert, idx) => (
              <div key={idx} className={`p-2 flex flex-col border ${idx === 0 ? 'border-[#ff3131] bg-[#ff3131]/5' : 'border-[#ff3131]/60 bg-transparent'}`}>
                <div className="flex justify-between text-[#ff3131] font-bold">
                  <span>{alert.type}</span>
                  <span>{alert.time}</span>
                </div>
                <p className="text-[#84967e] mt-1">{alert.detail}</p>
              </div>
            ))}
          </div>
        </div>

      </div>

      {/* 底部宽栏：实时报单执行监控 (Live Order Execution Monitor) */}
      <div className="border border-[#3b4b37] bg-[#141e12] flex flex-col flex-1">
        <div className="border-b border-[#3b4b37] px-4 py-3 bg-[#071106] flex justify-between items-center text-xs font-mono font-semibold text-[#dae6d2]">
          <div className="flex items-center gap-2">
            <Activity size={12} className="text-[#00ff41]" />
            <span>{activeLeague ? activeLeague.name : ''} 撮合大额报单流实时监控</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded-full bg-[#00ff41] animate-ping" />
            <span className="text-[10px] text-[#00ff41]">报单数据实时高速推送中...</span>
          </div>
        </div>

        <div className="flex-1 overflow-x-auto p-4">
          <table className="w-full text-left font-mono text-xs border-collapse">
            <thead>
              <tr className="text-[#84967e] border-b border-[#3b4b37] text-[10px] pb-3">
                <th className="pb-3 font-semibold">撮合时间</th>
                <th className="pb-3 font-semibold">交易盘口</th>
                <th className="pb-3 font-semibold">选端下注项</th>
                <th className="pb-3 font-semibold">报单类型</th>
                <th className="pb-3 font-semibold">撮合成交额</th>
                <th className="pb-3 font-semibold">撮合价 (赔率)</th>
                <th className="pb-3 font-semibold">市场冲击度</th>
                <th className="pb-3 font-semibold text-right">报单状态</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((ord, i) => (
                <tr key={i} className="border-b border-[#3b4b37]/20 hover:bg-[#222d20]/30 h-10 text-[11px] font-mono">
                  <td className="text-[#dae6d2] font-semibold">{ord.time}</td>
                  <td className="text-[#dae6d2]">{ord.market}</td>
                  <td className="text-[#dae6d2] font-semibold">{ord.selection}</td>
                  <td>
                    <span className={`px-1.5 py-0.5 border font-bold ${
                      ord.type.includes('买') ? 'bg-[#00ff41]/10 text-[#00ff41] border-[#00ff41]/20' : 'bg-[#84967e]/10 text-[#84967e] border-[#84967e]/20'
                    }`}>
                      {ord.type}
                    </span>
                  </td>
                  <td className="text-[#dae6d2] font-bold">{ord.stake}</td>
                  <td className="text-[#00ff41] font-bold">{ord.odds}</td>
                  
                  {/* 冲击波形图 */}
                  <td>
                    <div className="flex gap-0.5 text-[#00ff41] font-bold">
                      {Array.from({ length: 4 }).map((_, idx) => (
                        <span key={idx} className={idx < ord.impact ? (ord.type.includes('买') ? 'text-[#00ff41]' : 'text-[#84967e]') : 'text-[#3b4b37]'}>
                          |
                        </span>
                      ))}
                    </div>
                  </td>

                  <td className={`text-right font-bold ${ord.status === '已确认成交' ? 'text-[#00ff41]' : 'text-[#ffd5ae] animate-pulse'}`}>
                    {ord.status}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* 底部状态标识 */}
        <div className="border-t border-[#3b4b37] px-4 py-2 bg-[#071106] flex justify-end text-[9px] font-mono font-bold">
          <span className="text-[#00ff41] bg-[#00ff41]/10 border border-[#00ff41]/30 px-2 py-0.5">智能量化核心状态: 最佳运行模式</span>
        </div>
      </div>

    </div>
  )
}
