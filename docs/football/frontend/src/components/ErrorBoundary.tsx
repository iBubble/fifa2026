// ErrorBoundary.tsx - React 错误边界，防止子组件崩溃导致全屏黑屏
import { Component, type ReactNode } from 'react'

interface Props {
  children: ReactNode
  fallbackLabel?: string
}

interface State {
  hasError: boolean
  error: Error | null
  errorInfo: string
}

export default class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null, errorInfo: '' }
  }

  static getDerivedStateFromError(error: Error): Partial<State> {
    return { hasError: true, error }
  }

  componentDidCatch(error: Error, errorInfo: React.ErrorInfo) {
    console.error('[ErrorBoundary] 捕获渲染崩溃:', error, errorInfo)
    this.setState({
      errorInfo: errorInfo.componentStack || ''
    })
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{
          background: '#0c160a',
          color: '#ff3131',
          fontFamily: 'JetBrains Mono, monospace',
          fontSize: '12px',
          padding: '24px',
          overflow: 'auto',
          height: '100%',
          width: '100%',
        }}>
          <div style={{
            border: '1px solid #ff3131',
            padding: '16px',
            marginBottom: '16px',
            background: 'rgba(255, 49, 49, 0.05)'
          }}>
            <div style={{ fontSize: '14px', fontWeight: 'bold', marginBottom: '8px' }}>
              ⚠️ 渲染崩溃已捕获 ({this.props.fallbackLabel || '未知模块'})
            </div>
            <div style={{ color: '#dae6d2', marginBottom: '12px' }}>
              {this.state.error?.message || '未知错误'}
            </div>
            <button
              onClick={() => this.setState({ hasError: false, error: null, errorInfo: '' })}
              style={{
                background: '#00ff41',
                color: '#000',
                border: 'none',
                padding: '6px 16px',
                fontFamily: 'inherit',
                fontWeight: 'bold',
                cursor: 'pointer',
                fontSize: '11px',
              }}
            >
              尝试重新加载组件
            </button>
          </div>
          {this.state.errorInfo && (
            <pre style={{
              color: '#84967e',
              fontSize: '10px',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-all',
              maxHeight: '300px',
              overflow: 'auto',
              border: '1px solid #3b4b37',
              padding: '12px',
              background: '#071106'
            }}>
              {this.state.error?.stack}
              {'\n\n--- 组件堆栈 ---\n'}
              {this.state.errorInfo}
            </pre>
          )}
        </div>
      )
    }
    return this.props.children
  }
}
