import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { login } from '../api'

type LoginProps = { onLogin?: () => void }

export function Login({ onLogin }: LoginProps) {
  const [user, setUser] = useState('')
  const [pass, setPass] = useState('')
  const [err, setErr] = useState('')
  const navigate = useNavigate()

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setErr('')
    try {
      const data = await login(user, pass)
      const token = data?.token
      if (!token) {
        setErr('Invalid response from server')
        return
      }
      localStorage.setItem('token', token)
      window.dispatchEvent(new Event('auth-change'))
      onLogin?.()
      navigate('/docs', { replace: true })
    } catch (e) {
      const msg = e instanceof Error ? e.message : ''
      if (msg === 'INVALID_CREDENTIALS') setErr('Неверный логин или пароль')
      else if (msg === 'CONNECTION_ERROR') setErr('Ошибка соединения с сервером. Проверьте, что backend запущен (docker compose ps).')
      else if (msg.startsWith('SERVER_ERROR:')) setErr(`Ошибка сервера ${msg.replace('SERVER_ERROR:', '')}. Проверьте логи admin-backend.`)
      else if (msg === 'INVALID_RESPONSE') setErr('Сервер вернул неверный ответ.')
      else setErr(msg || 'Ошибка входа')
    }
  }

  return (
    <div className="login-page">
      <div className="login-center">
        <div className="login-card">
          <h1>Admin Login</h1>
          <form onSubmit={submit}>
            <label htmlFor="login-user">Username</label>
            <input
              id="login-user"
              type="text"
              value={user}
              onChange={e => setUser(e.target.value)}
              required
              autoComplete="username"
            />
            <label htmlFor="login-pass">Password</label>
            <input
              id="login-pass"
              type="password"
              value={pass}
              onChange={e => setPass(e.target.value)}
              required
              autoComplete="current-password"
            />
            {err && <p className="text-error">{err}</p>}
            <button type="submit" className="btn-primary" disabled={!!err}>
              Login
            </button>
          </form>
        </div>
      </div>
    </div>
  )
}
