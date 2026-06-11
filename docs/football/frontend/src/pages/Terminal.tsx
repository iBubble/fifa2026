// Terminal.tsx - 后端日志与系统监控终端
import { useState, useEffect, useRef } from 'react'
import { useWailsEvent } from '../hooks/useWailsEvents'
import { Cpu } from 'lucide-react'
import { useLeague } from '../App'

interface LogEntry {
  level: 'DEBUG' | 'INFO' | 'WARN' | 'ERROR' | 'SUCCESS'
  message: string
  source: string
  timestamp: string
}

export default function Terminal() {
  const { activeLeague } = useLeague()
  const [logs, setLogs] = useState<string[]>([
    '00:22:19 [系统警告] [通道] 检测到网络代理专线延迟丢包 (2.1%)',
    '00:22:20 [信息流] [抓取] 正在向全球数据中心抓取世界杯2026战局数据... [已成功匹配]',
    '00:22:22 [信息流] [连接] 与伦敦赔率推送服务器成功建立 142 条全新 TCP 数据通道',
    '00:22:23 [系统警告] [通道] 检测到网络代理专线延迟丢包 (2.1%)',
    '00:22:24 [系统警告] [通道] 检测到网络代理专线延迟丢包 (2.1%)',
    '00:22:25 [计算引擎] [算法] 策略计算引擎核心：基于泊松分布模型对大小球市场进行对冲回归校验中',
    '00:22:26 [信息流] [分析] 高阶战绩形态形态模型：主胜理论成功率回测结果为 68.2%',
    '00:22:28 [计算引擎] [算法] 策略计算引擎核心：基于泊松分布模型对大小球市场进行对冲回归校验中',
    '00:22:29 [安全鉴权] [系统] 系统身份权限鉴权通过。后台会话周期已自动延长。',
    '00:22:31 [系统警告] [算法] 策略计算引擎核心：基于泊松分布模型对大小球市场进行对冲回归校验中',
    '00:22:32 [网络同步] [节点] 盘口缓存差异增量包已成功推送到区域备用计算节点02',
    '00:22:34 [安全鉴权] [系统] 系统身份权限鉴权通过。后台会话周期已自动延长。',
    '00:22:35 [套利撮合] [自动] 【量化自动下单】套利交易撮合引擎成功在 365 自动购入对冲注单 (成交价: 2.14, 成交额: 1.5万 USDT)',
    '00:22:36 [网络同步] [节点] 盘口缓存差异增量包已成功推送到区域备用计算节点02',
    '00:22:37 [套利撮合] [自动] 【量化自动下单】套利交易撮合引擎成功在 365 自动购入对冲注单 (成交价: 2.14, 成交额: 1.5万 USDT)',
    '00:22:38 [系统警告] [通道] 检测到网络代理专线延迟丢包 (2.1%)',
  ])

  // 资源监控状态
  const [cpuLoad, setCpuLoad] = useState(32.4)
  const [heapMem, setHeapMem] = useState(1.24)
  const [goroutines, setGoroutines] = useState(12042)
  const [gcPause, setGcPause] = useState(0.82)

  const terminalEndRef = useRef<HTMLDivElement>(null)

  // 当切换联赛时，在终端内生成显式的切换引导日志
  useEffect(() => {
    if (!activeLeague) return
    const time = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    setLogs(prev => [
      ...prev,
      `${time} [SUCCESS] [System] 🖥️ 量化终端正在部署 [${activeLeague.name}] (${activeLeague.fullName}) 专用高频撮合环境...`,
      `${time} [INFO] [System] 📡 成功加载 ${activeLeague.name} 数据管道 (Season: ${activeLeague.season || 2025})，合并订阅 The Odds API 推送流`
    ].slice(-500))
  }, [activeLeague])

  // 订阅 Wails 后端真实推送日志
  useWailsEvent<LogEntry>('log:entry', (entry) => {
    if (!entry || !entry.timestamp) return
    const formatted = `${new Date(entry.timestamp).toLocaleTimeString('zh-CN', { hour12: false })} [${entry.level}] [${entry.source}] ${entry.message}`
    setLogs(prev => [...prev, formatted].slice(-500))
  })

  // 仿真资源波动
  useEffect(() => {
    const timer = setInterval(() => {
      setCpuLoad(prev => parseFloat((prev + (Math.random() - 0.5) * 4).toFixed(1)))
      setHeapMem(prev => parseFloat((prev + (Math.random() - 0.5) * 0.05).toFixed(2)))
      setGoroutines(prev => prev + Math.floor((Math.random() - 0.5) * 10))
      setGcPause(prev => parseFloat((prev + (Math.random() - 0.5) * 0.02).toFixed(2)))
    }, 2500)
    return () => clearInterval(timer)
  }, [])

  // 日志滚动到底部
  useEffect(() => {
    terminalEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [logs])

  // 辅助渲染日志行染色
  const getLogLineColorClass = (line: string) => {
    if (line.includes('[系统警告]') || line.includes('[WARN]') || line.includes('[LogWarning]')) return 'text-[#ffd5ae]' // Warning yellow-sand
    if (line.includes('[错误]') || line.includes('[ERROR]') || line.includes('[LogError]')) return 'text-[#ff3131]' // Error red
    if (line.includes('[已成功]') || line.includes('[SUCCESS]') || line.includes('[LogSuccess]') || line.includes('[自动]') || line.includes('[OK]')) return 'text-[#00ff41]' // Neon green
    return 'text-[#84967e]' // Dim green
  };

  return (
    <div className="w-full h-full flex overflow-hidden p-6 gap-6" style={{ background: 'var(--bg-primary)' }}>
      
      {/* ── 左侧日志流终端 (占 2/3) ── */}
      <div className="flex-1 border border-[#3b4b37] bg-[#141e12] flex flex-col overflow-hidden h-full">
        {/* 终端头部状态 */}
        <div className="border-b border-[#3b4b37] px-4 py-3 bg-[#071106] flex justify-between items-center text-xs font-mono font-semibold">
          <div className="flex items-center gap-2 text-[#dae6d2] font-sans">
            <span className="w-2.5 h-2.5 bg-[#00ff41] rounded-full inline-block shadow-glow-green animate-pulse" />
            <span>{activeLeague ? `[${activeLeague.name}] 计算集群主系统日志 / 原生底层进程监控` : '计算集群主系统日志 / 原生底层进程监控'}</span>
          </div>
          <span className="text-[#84967e] text-[9px] uppercase font-mono">进程 PID: 98221 | 已运行时间: 412小时12分04秒</span>
        </div>

        {/* 终端控制台核心内容 (黑色代码瀑布) */}
        <div className="flex-1 overflow-y-auto p-4 font-mono text-[10px] leading-relaxed flex flex-col gap-1.5 scrollbar-thin" style={{ background: '#0b0f19' }}>
          {logs.map((line, i) => (
            <div key={i} className={`whitespace-pre-wrap select-text selection:bg-[#00ff41] selection:text-black ${getLogLineColorClass(line)}`}>
              {line}
            </div>
          ))}
          {/* 光标闪烁 */}
          <div className="flex items-center gap-1 text-[#00ff41] font-bold">
            <span>{new Date().toLocaleTimeString('zh-CN', { hour12: false })} [计算内核活动正常] [当前所选联赛: {activeLeague?.name || ''}] [等待报单触发]</span>
            <span className="w-1.5 h-3.5 bg-[#00ff41] animate-pulse" />
          </div>
          <div ref={terminalEndRef} />
        </div>
      </div>

      {/* ── 右侧系统资源面板 (占 1/3) ── */}
      <div className="w-80 border border-[#3b4b37] bg-[#141e12] flex flex-col p-4 justify-between h-full">
        
        <div className="flex flex-col gap-6">
          <div className="border-b border-[#3b4b37] pb-2 text-xs font-mono font-semibold text-[#dae6d2] flex items-center gap-1.5 font-sans">
            <Cpu size={14} className="text-[#00ff41]" />
            <span>内核资源实时负荷 (Go Core)</span>
          </div>

          {/* CPU 负荷 */}
          <div className="flex flex-col gap-1.5 font-mono text-[10px]">
            <div className="flex justify-between text-[#dae6d2]">
              <span className="font-semibold uppercase text-[#84967e] font-sans">处理器核心负载率 (CPU CORE)</span>
              <span className="text-[#00ff41] font-bold">{cpuLoad.toFixed(1)}%</span>
            </div>
            <div className="w-full bg-[#0c160a] h-2 border border-[#3b4b37]/40">
              <div className="bg-[#00ff41] h-full shadow-glow-green" style={{ width: `${cpuLoad}%` }} />
            </div>
          </div>

          {/* 堆内存分配 */}
          <div className="flex flex-col gap-1.5 font-mono text-[10px]">
            <div className="flex justify-between text-[#dae6d2]">
              <span className="font-semibold uppercase text-[#84967e] font-sans">运行堆内存空间分配 (HEAP MEM)</span>
              <span className="text-[#00ff41] font-bold">{heapMem.toFixed(2)} GB</span>
            </div>
            <div className="w-full bg-[#0c160a] h-2 border border-[#3b4b37]/40">
              <div className="bg-[#00ff41] h-full shadow-glow-green" style={{ width: `${(heapMem / 4) * 100}%` }} />
            </div>
          </div>

          {/* 并发 Goroutines 计数 */}
          <div className="grid grid-cols-2 gap-4 mt-2">
            <div className="border border-[#3b4b37] bg-[#0c160a] p-3 text-center font-mono flex flex-col gap-1">
              <span className="text-[8px] text-[#84967e] block font-bold uppercase font-sans">活跃协程数 (GOROUTINES)</span>
              <span className="text-base font-bold text-[#00ff41]">{goroutines.toLocaleString()}</span>
            </div>
            <div className="border border-[#3b4b37] bg-[#0c160a] p-3 text-center font-mono flex flex-col gap-1">
              <span className="text-[8px] text-[#84967e] block font-bold uppercase font-sans">垃圾回收暂停时延 (GC PAUSE)</span>
              <span className="text-base font-bold text-[#00ff41]">{gcPause.toFixed(2)}ms</span>
            </div>
          </div>
        </div>

        {/* 底部延迟趋势柱状图 */}
        <div className="border-t border-[#3b4b37] pt-4 font-mono text-[10px] w-full">
          <span className="text-[#84967e] font-semibold uppercase block mb-3 font-sans">撮合网络往返延迟 (过去5分钟)</span>
          <div className="h-16 flex items-end justify-between gap-1.5 px-2">
            <div className="w-full bg-[#84967e]/30 h-6" />
            <div className="w-full bg-[#84967e]/30 h-10" />
            <div className="w-full bg-[#84967e]/30 h-8" />
            <div className="w-full bg-[#84967e]/30 h-12" />
            <div className="w-full bg-[#00ff41]/50 h-14" />
            <div className="w-full bg-[#00ff41] h-16 shadow-glow-green" />
          </div>
        </div>

      </div>

    </div>
  )
}
