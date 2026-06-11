// useWailsEvents.ts - 通用 Wails v3 事件监听 Hook
// Wails v3 Events.On 的回调类型是 WailsEventCallback<T>，事件对象是 WailsEvent<T>
import { useEffect, useRef } from 'react'
import { Events } from '@wailsio/runtime'

/**
 * useWailsEvent - 泛型 Wails 事件监听 Hook
 * 
 * @param eventName - 事件名称（与 Go 后端 Event.Emit 的第一个参数对应）
 * @param callback  - 事件处理函数（接收事件数据）
 * 
 * 用法示例:
 * ```tsx
 * useWailsEvent<OddsSnapshot>('odds:update', (data) => {
 *   setOdds(data)
 * })
 * ```
 */
export function useWailsEvent<T>(
  eventName: string,
  callback: (data: T) => void
) {
  // 使用 ref 保存最新 callback，避免 useEffect 频繁重注册
  const callbackRef = useRef(callback)
  callbackRef.current = callback

  useEffect(() => {
    // Wails v3 WailsEvent 类型：事件数据在 ev.data 字段
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const off = Events.On(eventName, (ev: any) => {
      try {
        // Wails v3 alpha 版本中，数据可能在 ev.data 或直接是 ev
        const data: T = ev?.data !== undefined ? ev.data : ev
        callbackRef.current(data)
      } catch (err) {
        console.error(`[useWailsEvent] 事件回调 "${eventName}" 执行异常:`, err)
      }
    })

    return () => {
      off() // 组件卸载时自动取消订阅
    }
  }, [eventName]) // 只在 eventName 变化时重新注册
}

/**
 * useWailsEvents - 批量注册多个事件监听
 */
export function useWailsEvents(handlers: Record<string, (data: unknown) => void>) {
  useEffect(() => {
    const offFns: Array<() => void> = []

    for (const [eventName, callback] of Object.entries(handlers)) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const off = Events.On(eventName, (ev: any) => {
        try {
          const data = ev?.data !== undefined ? ev.data : ev
          callback(data)
        } catch (err) {
          console.error(`[useWailsEvents] 事件回调 "${eventName}" 执行异常:`, err)
        }
      })
      offFns.push(off)
    }

    return () => {
      offFns.forEach(off => off())
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
}
