import { useState, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { login, fetchKeycloakConfig, type KeycloakConfig } from '../api'
import { REDIRECT_KEY, VERIFIER_KEY } from './KeycloakCallback'

type LoginProps = { onLogin?: () => void }

function b64url(buf: Uint8Array): string {
  let s = ''
  for (let i = 0; i < buf.length; i++) s += String.fromCharCode(buf[i])
  return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/u, '')
}

function randomVerifier(): string {
  const a = new Uint8Array(32)
  crypto.getRandomValues(a)
  return b64url(a)
}

async function sha256Challenge(verifier: string): Promise<string> {
  const enc = new TextEncoder().encode(verifier)
  const hash = await crypto.subtle.digest('SHA-256', enc)
  return b64url(new Uint8Array(hash))
}

export function Login({ onLogin }: LoginProps) {
  const [user, setUser] = useState('')
  const [pass, setPass] = useState('')
  const [err, setErr] = useState('')
  const [kc, setKc] = useState<KeycloakConfig | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    let cancelled = false
    void fetchKeycloakConfig().then((c) => {
      if (!cancelled) setKc(c)
    })
    return () => {
      cancelled = true
    }
  }, [])

  const startKeycloak = useCallback(async () => {
    setErr('')
    if (!kc?.enabled || !kc.authorization_endpoint || !kc.client_id) {
      setErr('Keycloak не настроен')
      return
    }
    const redirectUri = `${window.location.origin}/auth/callback`
    const verifier = randomVerifier()
    const challenge = await sha256Challenge(verifier)
    sessionStorage.setItem(VERIFIER_KEY, verifier)
    sessionStorage.setItem(REDIRECT_KEY, redirectUri)
    const u = new URL(kc.authorization_endpoint)
    u.searchParams.set('client_id', kc.client_id)
    u.searchParams.set('redirect_uri', redirectUri)
    u.searchParams.set('response_type', 'code')
    u.searchParams.set('scope', 'openid')
    u.searchParams.set('code_challenge_method', 'S256')
    u.searchParams.set('code_challenge', challenge)
    window.location.href = u.toString()
  }, [kc])

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

  const kcEnabled = kc?.enabled === true && !!kc.authorization_endpoint && !!kc.client_id
  const hidePasswordForm = kc?.password_login_disabled === true

  return (
    <div className="login-page">
      <div className="login-center">
        <div className="login-card">
          <h1>Admin Login</h1>
          {kcEnabled && (
            <>
              <button type="button" className="btn-primary" style={{ width: '100%', marginBottom: '1rem' }} onClick={() => void startKeycloak()}>
                Войти через Keycloak
              </button>
              {!hidePasswordForm && <p className="text-muted" style={{ textAlign: 'center', marginBottom: '0.75rem' }}>или локально</p>}
            </>
          )}
          {!hidePasswordForm && (
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
              <button type="submit" className="btn-primary">
                Login
              </button>
            </form>
          )}
          {hidePasswordForm && err && <p className="text-error">{err}</p>}
        </div>
      </div>
    </div>
  )
}
