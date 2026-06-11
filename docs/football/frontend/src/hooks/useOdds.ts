// useOdds.ts - 赔率状态管理 Hook
// 订阅赔率更新事件，维护本地状态，并追踪赔率变化方向（用于闪烁特效）
import { useState, useCallback } from 'react'
import { useWailsEvent } from './useWailsEvents'

// ─── TypeScript 类型定义（与 Go models 对应）───────────────────

export interface OddsOutcome {
  name: string
  price: number
}

export interface BookmakerOdds {
  bookmaker: string
  market: 'h2h' | 'spreads' | 'totals'
  outcomes: OddsOutcome[]
  updatedAt: string
}

export interface Match {
  id: string
  homeTeam: string
  awayTeam: string
  league: string
  country: string
  scheduledAt: string
  status: string
  homeScore: number
  awayScore: number
  minute: number
}

export interface OddsSnapshot {
  matchId: string
  match: Match
  bookmakers: BookmakerOdds[]
  capturedAt: string
}

export interface ArbitrageOpportunity {
  matchId: string
  match: Match
  market: string
  lValue: number
  roi: number
  legs: Array<{
    bookmaker: string
    outcome: string
    odds: number
    stakePct: number
    stakeAmt: number
  }>
  detectedAt: string
}

// 赔率变化方向（用于单元格闪烁特效）
export type OddsDirection = 'up' | 'down' | null

// ─── Hook 实现 ──────────────────────────────────────────────────

/**
 * useOdds - 赔率状态管理
 * 
 * 订阅 "odds:update" 事件，按 matchId 维护赔率快照 Map
 * 同时记录每个赔率的变化方向（up/down），供闪烁特效组件使用
 */
export function useOdds() {
  // 赔率快照 Map: matchId → OddsSnapshot
  const [oddsMap, setOddsMap] = useState<Record<string, OddsSnapshot>>({})
  
  // 赔率变化方向 Map: `${matchId}:${bookmaker}:${outcome}` → direction
  const [directionMap, setDirectionMap] = useState<Record<string, OddsDirection>>({})

  const handleOddsUpdate = useCallback((snapshot: OddsSnapshot) => {
    if (!snapshot || !snapshot.matchId) return
    setOddsMap(prev => {
      const prevSnapshot = prev[snapshot.matchId]
      
      // 计算赔率变化方向
      if (prevSnapshot) {
        const newDirections: Record<string, OddsDirection> = {}
        
        for (const bk of snapshot.bookmakers) {
          const prevBk = prevSnapshot.bookmakers.find(b => b.bookmaker === bk.bookmaker)
          if (!prevBk) continue
          
          for (const outcome of bk.outcomes) {
            const prevOutcome = prevBk.outcomes.find(o => o.name === outcome.name)
            if (!prevOutcome) continue
            
            const key = `${snapshot.matchId}:${bk.bookmaker}:${outcome.name}`
            if (outcome.price > prevOutcome.price) {
              newDirections[key] = 'up'
            } else if (outcome.price < prevOutcome.price) {
              newDirections[key] = 'down'
            }
          }
        }
        
        // 更新变化方向，600ms 后自动清除（配合 CSS 动画时长）
        if (Object.keys(newDirections).length > 0) {
          setDirectionMap(prev => ({ ...prev, ...newDirections }))
          setTimeout(() => {
            setDirectionMap(prev => {
              const updated = { ...prev }
              for (const key of Object.keys(newDirections)) {
                delete updated[key]
              }
              return updated
            })
          }, 600)
        }
      }
      
      return { ...prev, [snapshot.matchId]: snapshot }
    })
  }, [])

  useWailsEvent<OddsSnapshot>('odds:update', handleOddsUpdate)

  return {
    oddsMap,
    directionMap,
    // 获取某场比赛的赔率快照
    getOddsForMatch: (matchId: string) => oddsMap[matchId] ?? null,
    // 获取某条赔率的变化方向
    getDirection: (matchId: string, bookmaker: string, outcome: string): OddsDirection => {
      return directionMap[`${matchId}:${bookmaker}:${outcome}`] ?? null
    },
  }
}

/**
 * useArbitrageAlerts - 套利警报状态管理
 * 订阅 "arbitrage:alert" 事件，维护最近 50 条套利机会列表
 */
export function useArbitrageAlerts() {
  const [alerts, setAlerts] = useState<ArbitrageOpportunity[]>([])

  const handleAlert = useCallback((opp: ArbitrageOpportunity) => {
    setAlerts(prev => {
      // 保持最多 50 条，最新在前
      const next = [opp, ...prev]
      return next.slice(0, 50)
    })
  }, [])

  useWailsEvent<ArbitrageOpportunity>('arbitrage:alert', handleAlert)

  return { alerts }
}
