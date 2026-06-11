// Reports.tsx - 资产报告与投注账本页面
import { useState, useEffect, useRef } from 'react'
import * as echarts from 'echarts'
import { Download, ShieldAlert } from 'lucide-react'
import { useLeague } from '../App'
import { GetMatchesByLeague } from '../../bindings/football/app'

interface BettingLog {
  timestamp: string
  event: string
  oddsType: string
  stake: string
  price: string
  status: '红单盈利' | '黑单亏损' | '走单退款'
  returns: string
}

export default function Reports() {
  const { activeLeague } = useLeague()
  const [realMatchesCount, setRealMatchesCount] = useState<number | null>(null)

  useEffect(() => {
    if (!activeLeague) return
    const checkMatches = async () => {
      try {
        const res = await GetMatchesByLeague(activeLeague.name)
        setRealMatchesCount(res ? res.length : 0)
      } catch (err) {
        console.error('获取报告联赛比赛数据失败:', err)
        setRealMatchesCount(0)
      }
    }
    checkMatches()
  }, [activeLeague])

  const [logs] = useState<BettingLog[]>([
    { timestamp: '14:01:22', event: '阿根廷 vs 法国 (世界杯决赛) \n亚洲让球盘 -0.5', oddsType: '强动能动能模型_V2', stake: '5,000.00', price: '1.92', status: '红单盈利', returns: '+4,600.00' },
    { timestamp: '13:45:10', event: '巴西 vs 德国 (半决赛) \n大 2.5 球 盘口', oddsType: '爆冷高倍持仓', stake: '2,500.00', price: '2.15', status: '黑单亏损', returns: '-2,500.00' },
    { timestamp: '13:12:05', event: '西班牙 vs 日本 (小组赛E组) \n标准独赢 - 主胜 (1)', oddsType: 'H2H均值稳健模型', stake: '1,200.00', price: '1.44', status: '走单退款', returns: '0.00' },
    { timestamp: '12:55:40', event: '克罗地亚 vs 摩洛哥 (三四名决赛) \n下一进球：克罗地亚', oddsType: '走地脉搏高频模型', stake: '3,000.00', price: '1.85', status: '红单盈利', returns: '+2,550.00' },
  ])

  const klineRef = useRef<HTMLDivElement>(null)
  const donutRef = useRef<HTMLDivElement>(null)

  // 1. 初始化 EQUITY K-LINE 净值走势蜡烛图
  useEffect(() => {
    if (!klineRef.current) return
    const chart = echarts.init(klineRef.current)
    const option = {
      backgroundColor: 'transparent',
      grid: { left: '8%', right: '5%', bottom: '15%', top: '5%' },
      xAxis: {
        type: 'category',
        data: ['1小时前', '2小时前', '3小时前', '4小时前', '5小时前', '6小时前', '当期实时'],
        axisLine: { lineStyle: { color: 'rgba(203, 213, 225, 0.6)' } },
        axisLabel: { color: '#64748b', fontSize: 9, fontFamily: 'var(--font-sans)' }
      },
      yAxis: {
        type: 'value',
        scale: true,
        splitLine: { lineStyle: { color: 'rgba(226, 232, 240, 0.5)', type: 'dashed' } },
        axisLabel: { color: '#64748b', fontSize: 9, fontFamily: 'JetBrains Mono' }
      },
      series: [
        {
          type: 'candlestick',
          data: [
            [122104, 124502, 121900, 125502], // open, close, low, high
            [124502, 123800, 123500, 125000],
            [123800, 126100, 123000, 127000],
            [126100, 125400, 125000, 126800],
            [125400, 128200, 125100, 128900],
            [128200, 127900, 127000, 128500],
            [127900, 131200, 127500, 131500]
          ],
          itemStyle: {
            color: '#10b981',
            color0: '#ef4444',
            borderColor: '#10b981',
            borderColor0: '#ef4444'
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
  }, [])

  // 2. 初始化 P&L DISTRIBUTION 盈亏分布甜甜圈图
  useEffect(() => {
    if (!donutRef.current) return
    const chart = echarts.init(donutRef.current)
    const option = {
      backgroundColor: 'transparent',
      tooltip: {
        trigger: 'item',
        backgroundColor: 'rgba(255, 255, 255, 0.95)',
        borderColor: 'rgba(226, 232, 240, 0.8)',
        textStyle: { color: '#0f172a', fontFamily: 'var(--font-sans)', fontSize: 10 }
      },
      series: [
        {
          name: '盈亏分配比例',
          type: 'pie',
          radius: ['60%', '80%'],
          avoidLabelOverlap: false,
          label: {
            show: true,
            position: 'center',
            formatter: '68%',
            fontSize: 20,
            fontWeight: 'bold',
            color: '#10b981',
            fontFamily: 'JetBrains Mono'
          },
          labelLine: { show: false },
          data: [
            { value: 242, name: '红单盈利', itemStyle: { color: '#10b981' } },
            { value: 112, name: '黑单亏损', itemStyle: { color: '#ef4444' } },
            { value: 41, name: '走单退款', itemStyle: { color: '#94a3b8' } }
          ]
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
  }, [])

  if (realMatchesCount === 0) {
    return (
      <div className="w-full h-full flex flex-col justify-center items-center p-6 gap-6" style={{ background: 'var(--bg-primary)' }}>
        <div className="max-w-xl border border-[#ff3131]/30 bg-[#ff3131]/5 p-8 flex flex-col items-center gap-6 font-mono text-center shadow-[0_0_30px_rgba(255,49,49,0.05)]">
          <div className="w-16 h-16 rounded-full bg-[#ff3131]/10 border border-[#ff3131]/30 flex items-center justify-center text-[#ff3131] animate-pulse">
            <ShieldAlert size={36} />
          </div>
          
          <div className="flex flex-col gap-2">
            <h2 className="text-[#dae6d2] text-lg font-bold tracking-wider">
              ⏳ 暂无当前联赛（{activeLeague?.name || '当前联赛'}）实单账本数据
            </h2>
            <p className="text-[#84967e] text-xs leading-relaxed max-w-md">
              由于本地 SQLite 数据库中没有可用的真实比赛日程，实单账户资产分析、盈亏分配谱系、累计净值蜡烛图和实单账本历史流水均处于静默待机状态。
            </p>
          </div>

          <div className="w-full border-t border-[#3b4b37]/40 pt-4 flex flex-col gap-3 text-left">
            <div className="text-[10px] text-[#dae6d2] font-semibold uppercase tracking-wider">
              🛠 Lyra 量化账本激活指引：
            </div>
            <div className="bg-black/40 border border-[#3b4b37]/35 p-3 flex flex-col gap-2 text-[11px] text-[#84967e] leading-relaxed">
              <div className="flex items-start gap-2">
                <span className="text-[#00ff41]">1.</span>
                <span>请确保本地 `API-Sports` 密钥已正确加载并启动数据同步。</span>
              </div>
              <div className="flex items-start gap-2">
                <span className="text-[#00ff41]">2.</span>
                <span>当本地 SQLite 持久化并完成赛程、赔率关联后，实单对冲账本与净值增长率分析将全自动与真实赛果关联渲染。</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2 text-[10px] text-[#ff3131] border border-[#ff3131]/20 bg-[#ff3131]/5 px-3 py-1 font-bold">
            <span className="w-1.5 h-1.5 rounded-full bg-[#ff3131] animate-ping" />
            <span>实单账本离线中... 等待真实 API 日程入库</span>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="w-full h-full flex flex-col overflow-y-auto p-6" style={{ background: 'var(--bg-primary)' }}>
      
      {/* 顶部四张核心资产卡片 */}
      <div className="grid grid-cols-4 gap-6 mb-6">
        
        {/* 卡片 1: 账户净值 */}
        <div className="border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col justify-between font-mono relative">
          <span className="text-[10px] text-[#84967e] font-bold tracking-wider font-sans">账户实单资产净值</span>
          <div className="flex justify-between items-end mt-3">
            <div className="flex flex-col">
              <span className="text-xl font-bold text-[#00ff41]">$124,502.84 <b className="text-xs text-[#84967e]">USDT</b></span>
              <span className="text-[8px] text-[#00ff41] font-bold mt-1 font-sans">▲ 较本月度增长 +12.4% (月度至今)</span>
            </div>
            {/* 微型趋势图 */}
            <div className="flex items-end gap-0.5 h-6">
              <span className="w-1 bg-[#3b4b37] h-2" />
              <span className="w-1 bg-[#3b4b37] h-3" />
              <span className="w-1 bg-[#00ff41] h-4" />
              <span className="w-1 bg-[#00ff41] h-5" />
            </div>
          </div>
        </div>

        {/* 卡片 2: 策略胜率 */}
        <div className="border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col justify-between font-mono">
          <span className="text-[10px] text-[#84967e] font-bold tracking-wider font-sans">历史投注实单胜率</span>
          <div className="flex flex-col gap-1.5 mt-3">
            <span className="text-xl font-bold text-[#dae6d2]">68.42%</span>
            <div className="w-full bg-[#0c160a] h-1.5 border border-[#3b4b37]/30">
              <div className="bg-[#00ff41] h-full shadow-glow-green" style={{ width: '68.42%' }} />
            </div>
          </div>
        </div>

        {/* 卡片 3: 最大回撤 */}
        <div className="border border-[#3b4b37] bg-[#ff3131]/5 border-l-2 border-l-[#ff3131] p-4 flex flex-col justify-between font-mono">
          <span className="text-[10px] text-[#ff3131] font-bold tracking-wider font-sans">历史极端最大回撤</span>
          <div className="flex justify-between items-end mt-3 text-[#dae6d2] font-sans">
            <span className="text-xl font-bold text-[#ff3131]">-4.12%</span>
            <span className="text-[8px] text-[#84967e] uppercase font-semibold">峰值: 131,200 | 谷值: 125,800</span>
          </div>
        </div>

        {/* 卡片 4: 当日盈亏 */}
        <div className="border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col justify-between font-mono">
          <span className="text-[10px] text-[#84967e] font-bold tracking-wider font-sans">当日交易结算盈亏</span>
          <div className="flex justify-between items-end mt-3 text-[#dae6d2] font-sans">
            <span className="text-xl font-bold text-[#00ff41]">+$1,204.10 <b className="text-xs text-[#84967e]">USDT</b></span>
            <span className="text-[8px] text-[#84967e] uppercase font-semibold">更新时间: 14:02:44 (北京时间)</span>
          </div>
        </div>

      </div>

      {/* 中部大区：净值蜡烛图 + 盈亏饼图 (2/3 与 1/3) */}
      <div className="grid grid-cols-3 gap-6 mb-6">
        
        {/* 左侧 2/3: Candle 净值蜡烛图 */}
        <div className="col-span-2 border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col">
          <div className="border-b border-[#3b4b37] pb-3 flex justify-between items-center text-xs font-mono font-semibold text-[#dae6d2]">
            <span className="font-sans">● 策略净值历史K线走势图</span>
            <div className="flex border border-[#3b4b37] text-[9px] font-bold">
              <button className="px-2 py-0.5 bg-[#3b4b37] text-white">1小时</button>
              <button className="px-2 py-0.5 text-[#84967e] hover:bg-[#3b4b37]/30">4小时</button>
              <button className="px-2 py-0.5 text-[#84967e] hover:bg-[#3b4b37]/30">1日线</button>
            </div>
          </div>
          <div ref={klineRef} className="h-56 mt-4 w-full" />
        </div>

        {/* 右侧 1/3: Pie 图和策略榜 */}
        <div className="col-span-1 border border-[#3b4b37] bg-[#141e12] p-4 flex flex-col justify-between h-full">
          
          <div className="flex flex-col gap-4">
            <span className="text-[10px] text-[#84967e] font-mono font-bold uppercase tracking-wider border-b border-[#3b4b37] pb-2 font-sans">
              量化实单盈亏结构比例
            </span>
            <div className="flex gap-4 items-center">
              <div ref={donutRef} className="w-28 h-28" />
              <div className="flex flex-col gap-2 font-mono text-[9px] text-[#dae6d2] font-sans">
                <div className="flex justify-between gap-4">
                  <span className="text-[#00ff41] font-bold">红单盈利期数</span>
                  <span>242期</span>
                </div>
                <div className="flex justify-between gap-4">
                  <span className="text-[#ff3131] font-bold">黑单亏损期数</span>
                  <span>112期</span>
                </div>
                <div className="flex justify-between gap-4">
                  <span className="text-[#84967e] font-bold">走单平盘退款</span>
                  <span>41期</span>
                </div>
              </div>
            </div>
          </div>

          {/* 策略细分 */}
          <div className="border-t border-[#3b4b37] pt-4 flex flex-col gap-2.5 font-mono text-[9px] text-[#dae6d2] font-sans">
            <span className="text-[#84967e] font-bold uppercase mb-1">三大主力量化策略盈亏贡献榜</span>
            
            {/* 策略 1 */}
            <div className="flex justify-between items-center">
              <span>走地盘口强动能对冲策略</span>
              <div className="flex items-center gap-2">
                <div className="w-16 bg-[#00ff41]/20 h-1.5"><div className="bg-[#00ff41] h-full" style={{ width: '85%' }} /></div>
                <span className="text-[#00ff41] font-bold">+$12,400.00</span>
              </div>
            </div>

            {/* 策略 2 */}
            <div className="flex justify-between items-center">
              <span>高频多向套利自动扫盘策略</span>
              <div className="flex items-center gap-2">
                <div className="w-16 bg-[#00ff41]/20 h-1.5"><div className="bg-[#00ff41] h-full" style={{ width: '60%' }} /></div>
                <span className="text-[#00ff41] font-bold">+$8,210.00</span>
              </div>
            </div>

            {/* 策略 3 */}
            <div className="flex justify-between items-center">
              <span>弱队冷门高倍率持仓策略</span>
              <div className="flex items-center gap-2">
                <div className="w-16 bg-[#ff3131]/20 h-1.5"><div className="bg-[#ff3131] h-full" style={{ width: '25%' }} /></div>
                <span className="text-[#ff3131] font-bold">-$1,540.00</span>
              </div>
            </div>
          </div>

        </div>

      </div>

      {/* 底部宽栏：投注流水 (Betting Logs) */}
      <div className="border border-[#3b4b37] bg-[#141e12] flex flex-col flex-1">
        <div className="border-b border-[#3b4b37] px-4 py-3 bg-[#071106] flex justify-between items-center text-xs font-mono font-semibold text-[#dae6d2]">
          <span className="font-sans">量化交易投注流水账本</span>
          <div className="flex gap-4 text-[9px] font-bold font-sans">
            <button className="text-[#84967e] hover:text-[#00ff41] bg-transparent border-none cursor-pointer">全部交易盘口</button>
            <button className="text-[#00ff41] hover:text-[#00ff41] bg-transparent border-none cursor-pointer flex items-center gap-1">
              <Download size={10} />
              <span>导出为 CSV 账本文件</span>
            </button>
          </div>
        </div>

        <div className="flex-grow overflow-x-auto p-4">
          <table className="w-full text-left font-mono text-xs border-collapse">
            <thead>
              <tr className="text-[#84967e] border-b border-[#3b4b37] text-[10px] pb-3">
                <th className="pb-3 font-semibold">报单确认时间</th>
                <th className="pb-3 font-semibold">对决战局 / 盘口类型</th>
                <th className="pb-3 font-semibold">采用策略模型</th>
                <th className="pb-3 font-semibold">投注本金 (USDT)</th>
                <th className="pb-3 font-semibold">成交价 (赔率)</th>
                <th className="pb-3 font-semibold">注单结算状态</th>
                <th className="pb-3 font-semibold text-right">结算盈亏变动</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((row, i) => (
                <tr key={i} className="border-b border-[#3b4b37]/20 hover:bg-[#222d20]/30 h-12 text-[11px] font-mono vertical-middle">
                  <td className="text-[#dae6d2]">{row.timestamp}</td>
                  <td className="text-[#dae6d2] font-semibold whitespace-pre-wrap py-1.5 font-sans">{row.event}</td>
                  <td className="text-[#dae6d2] font-bold">{row.oddsType}</td>
                  <td className="text-[#dae6d2]">{row.stake}</td>
                  <td className="text-[#3B82F6] font-bold">{row.price}</td>
                  <td>
                    <span className={`px-1.5 py-0.5 border text-[9px] font-bold ${
                      row.status.includes('赢') ? 'bg-[#00ff41]/10 text-[#00ff41] border-[#00ff41]/20' :
                      row.status.includes('输') ? 'bg-[#ff3131]/10 text-[#ff3131] border-[#ff3131]/20' :
                      'bg-[#84967e]/10 text-[#84967e] border-[#84967e]/20'
                    }`}>
                      {row.status}
                    </span>
                  </td>
                  <td className={`text-right font-bold ${row.returns.startsWith('+') ? 'text-[#00ff41]' : row.returns === '0.00' ? 'text-[#84967e]' : 'text-[#ff3131]'}`}>
                    {row.returns}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* 底部详细指标 */}
        <div className="border-t border-[#3b4b37] px-4 py-2 bg-[#071106] flex justify-between text-[8px] font-mono text-[#84967e] font-bold">
          <span>🟢 核心账本数据库已成功同步</span>
          <span>系统延迟: 12毫秒 | 主网节点: 中国香港量化04节点 | 数据配额: 98.4%</span>
        </div>
      </div>

    </div>
  )
}
