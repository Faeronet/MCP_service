import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { keycloakCallback } from '../api'

const VERIFIER_KEY = 'oidc_code_verifier'
const REDIRECT_KEY = 'oidc_redirect_uri'

export function KeycloakCallback() {
  const [err, setErr] = useState('')
  const navigate = useNavigate()

  useEffect(() => {
    const run = async () => {
      const params = new URLSearchParams(window.location.search)
      const code = params.get('code')
      const error = params.get('error')
      const errDesc = params.get('error_description')
      if (error) {
        setErr(errDesc || error || 'OAuth error')
        return
      }
      if (!code) {
        setErr('Нет параметра code')
        return
      }
      const verifier = sessionStorage.getItem(VERIFIER_KEY) || ''
      const redirectUri = sessionStorage.getItem(REDIRECT_KEY) || `${window.location.origin}/auth/callback`
      if (!verifier) {
        setErr('Сессия входа устарела. Откройте вход снова.')
        return
      }
      try {
        await keycloakCallback(code, redirectUri, verifier)
        sessionStorage.removeItem(VERIFIER_KEY)
        sessionStorage.removeItem(REDIRECT_KEY)
        window.dispatchEvent(new Event('auth-change'))
        navigate('/docs', { replace: true })
      } catch (e) {
        const msg = e instanceof Error ? e.message : String(e)
        setErr(msg || 'Ошибка обмена кода')
      }
    }
    void run()
  }, [navigate])

  if (err) {
    return (
      <div className="login-page">
        <div className="login-center">
          <div className="login-card">
            <h1>Keycloak</h1>
            <p className="text-error">{err}</p>
            <button type="button" className="btn-primary" onClick={() => navigate('/login', { replace: true })}>
              На страницу входа
            </button>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="login-page">
      <div className="login-center">
        <p className="text-muted">Завершение входа…</p>
      </div>
    </div>
  )
}

export { VERIFIER_KEY, REDIRECT_KEY }
