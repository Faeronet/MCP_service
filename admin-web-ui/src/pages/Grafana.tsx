import { grafanaUrl } from '../api'

export function Grafana() {
  const url = grafanaUrl()
  return (
    <div>
      <h1>Grafana</h1>
      <p style={{ marginBottom: 16, color: '#71717a' }}>Embedded via admin-backend proxy. Log in with Grafana admin credentials if prompted.</p>
      <iframe
        title="Grafana"
        src={url}
        style={{ width: '100%', height: 'calc(100vh - 180px)', border: '1px solid #27272a', borderRadius: 8 }}
      />
    </div>
  )
}
