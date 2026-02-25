import { grafanaUrl } from '../api'

function hasToken(): boolean {
  if (typeof window === 'undefined') return false
  return !!localStorage.getItem('token')
}

export function Grafana() {
  const url = grafanaUrl()
  const authorized = hasToken()

  return (
    <div>
      <h1>Grafana</h1>
      <p style={{ marginBottom: 16, color: '#71717a' }}>
        Через прокси админки (анонимный просмотр). Если видите «failed to load» или 404 — задайте <code style={{ background: '#27272a', padding: '2px 6px', borderRadius: 4 }}>GRAFANA_ROOT_URL</code> равным URL этой страницы (например <code style={{ background: '#27272a', padding: '2px 6px', borderRadius: 4 }}>http://ВАШ_IP:5173/api/grafana</code>) и перезапустите <code style={{ background: '#27272a', padding: '2px 6px', borderRadius: 4 }}>grafana</code>.
      </p>
      {!authorized ? (
        <div style={{ padding: 24, background: '#27272a', borderRadius: 8, color: '#a1a1aa' }}>
          Откройте Grafana после входа в админку: страница «Grafana» в меню слева подставляет токен в iframe. При прямом переходе по адресу <code style={{ background: '#18181b', padding: '2px 6px', borderRadius: 4 }}>/api/grafana/</code> токена нет — поэтому появляется «missing authorization». Войдите в админку и откройте раздел Grafana из меню.
        </div>
      ) : (
        <iframe
          title="Grafana"
          src={url}
          style={{ width: '100%', height: 'calc(100vh - 180px)', border: '1px solid #27272a', borderRadius: 8 }}
        />
      )}
    </div>
  )
}
