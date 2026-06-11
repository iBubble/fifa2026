import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'

// 全局错误捕获 - 将错误信息直接渲染到页面上，防止黑屏无法定位
window.onerror = (message, source, lineno, colno, error) => {
  console.error('[全局异常]', message, source, lineno, colno, error)
  showGlobalError(`[window.onerror] ${message}\n来源: ${source}:${lineno}:${colno}\n${error?.stack || ''}`)
  return true // 阻止默认处理
}

window.onunhandledrejection = (event) => {
  console.error('[未处理Promise异常]', event.reason)
  const msg = event.reason?.message || event.reason || '未知 Promise 异常'
  const stack = event.reason?.stack || ''
  showGlobalError(`[unhandledrejection] ${msg}\n${stack}`)
}

function showGlobalError(text: string) {
  // 如果页面已经有错误展示容器，则追加
  let container = document.getElementById('global-error-overlay')
  if (!container) {
    container = document.createElement('div')
    container.id = 'global-error-overlay'
    container.style.cssText = `
      position: fixed; top: 0; left: 0; right: 0; bottom: 0; z-index: 99999;
      background: rgba(12, 22, 10, 0.97); color: #ff3131;
      font-family: 'JetBrains Mono', monospace; font-size: 12px;
      padding: 24px; overflow: auto;
    `
    document.body.appendChild(container)
  }
  const entry = document.createElement('pre')
  entry.style.cssText = `
    border: 1px solid #ff3131; padding: 12px; margin-bottom: 12px;
    background: rgba(255,49,49,0.05); white-space: pre-wrap; word-break: break-all;
  `
  entry.textContent = `[${new Date().toLocaleTimeString('zh-CN', { hour12: false })}] ${text}`
  container.appendChild(entry)

  // 添加"刷新"按钮
  if (!document.getElementById('global-error-reload-btn')) {
    const btn = document.createElement('button')
    btn.id = 'global-error-reload-btn'
    btn.textContent = '🔄 刷新页面'
    btn.style.cssText = `
      background: #00ff41; color: #000; border: none; padding: 8px 24px;
      font-family: inherit; font-weight: bold; cursor: pointer; font-size: 13px;
      margin-top: 12px;
    `
    btn.onclick = () => window.location.reload()
    container.appendChild(btn)
  }
}

ReactDOM.createRoot(document.getElementById('root') as HTMLElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
