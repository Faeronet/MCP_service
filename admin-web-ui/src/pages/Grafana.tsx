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
      <h1 className="page-title">Grafana</h1>
      <p className="text-muted" style={{ marginBottom: 16 }}>
        Через прокси админки (анонимный просмотр). Если видите «failed to load» или 404 — задайте <code style={{ background: 'var(--card-bg)', padding: '2px 6px', borderRadius: 4 }}>GRAFANA_ROOT_URL</code> равным URL этой страницы (например <code style={{ background: 'var(--card-bg)', padding: '2px 6px', borderRadius: 4 }}>http://ВАШ_IP:5173/api/grafana</code>) и перезапустите <code style={{ background: 'var(--card-bg)', padding: '2px 6px', borderRadius: 4 }}>grafana</code>.
      </p>
      {!authorized ? (
        <div className="content-card" style={{ color: 'var(--text-muted)' }}>
          Откройте Grafana после входа в админку: страница «Grafana» в меню слева подставляет токен в iframe. При прямом переходе по адресу <code style={{ background: 'var(--input-bg)', padding: '2px 6px', borderRadius: 4 }}>/api/grafana/</code> токена нет — поэтому появляется «missing authorization». Войдите в админку и откройте раздел Grafana из меню.
        </div>
      ) : (
        <div className="grafana-iframe-wrap">
          <iframe title="Grafana" src={url} />
        </div>
      )}
    </div>
  )
}
