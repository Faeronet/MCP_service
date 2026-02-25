import { Component, type ReactNode } from 'react'

type Props = { children: ReactNode }
type State = { hasError: boolean }

export class ErrorBoundary extends Component<Props, State> {
  state: State = { hasError: false }

  static getDerivedStateFromError(): State {
    return { hasError: true }
  }

  render() {
    if (this.state.hasError) {
      return (
        <div style={{
          minHeight: '100vh', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center',
          background: '#18181b', color: '#fafafa', padding: 24, fontFamily: 'system-ui, sans-serif'
        }}>
          <p style={{ fontSize: 18, marginBottom: 16 }}>Произошла ошибка</p>
          <a href="/login" style={{ color: '#818cf8' }}>Перейти на страницу входа</a>
        </div>
      )
    }
    return this.props.children
  }
}
