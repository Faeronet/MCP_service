import { useState, useEffect } from 'react'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from './components/Layout'
import { ToastContainer } from './components/Toast'
import { MonitorProvider } from './context/MonitorContext'
import { useToast } from './context/ToastContext'
import { Login } from './pages/Login'
import { Docs } from './pages/Docs'
import { Jobs } from './pages/Jobs'
import { Logs } from './pages/Logs'
import { ChatLog } from './pages/ChatLog'
import { Chat } from './pages/Chat'
import { Monitor } from './pages/Monitor'
import { Grafana } from './pages/Grafana'

export default function App() {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('token'))
  const { toasts, remove } = useToast()

  useEffect(() => {
    const onAuthChange = () => setToken(localStorage.getItem('token'))
    window.addEventListener('auth-change', onAuthChange)
    return () => window.removeEventListener('auth-change', onAuthChange)
  }, [])

  return (
    <BrowserRouter>
      <ToastContainer toasts={toasts} onRemove={remove} />
      <Routes>
        <Route path="/login" element={<Login onLogin={() => setToken(localStorage.getItem('token'))} />} />
        <Route path="/" element={token ? <MonitorProvider><Layout /></MonitorProvider> : <Navigate to="/login" replace />}>
          <Route index element={<Navigate to="/docs" replace />} />
          <Route path="docs" element={<Docs />} />
          <Route path="jobs" element={<Jobs />} />
          <Route path="logs" element={<Logs />} />
          <Route path="chat-log" element={<ChatLog />} />
          <Route path="chat" element={<Chat />} />
          <Route path="monitor" element={<Monitor />} />
          <Route path="grafana" element={<Grafana />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

