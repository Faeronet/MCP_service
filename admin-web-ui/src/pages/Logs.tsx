import { useState, useEffect } from 'react'
import { searchLogs } from '../api'

function levelClass(level: string): string {
  const l = String(level).toLowerCase()
  if (l === 'error') return 'badge-error'
  if (l === 'warn' || l === 'warning') return 'badge-warning'
  return 'badge-info'
}

export function Logs() {
  const [logs, setLogs] = useState<Array<Record<string, unknown>>>([])
  const [loading, setLoading] = useState(true)
  const [service, setService] = useState('')
  const [requestId, setRequestId] = useState('')
  const [level, setLevel] = useState('')

  const load = async () => {
    setLoading(true)
    try {
      const { logs: l } = await searchLogs({
        service: service || undefined,
        request_id: requestId || undefined,
        level: level || undefined,
        limit: 100,
      })
      setLogs(Array.isArray(l) ? l : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  return (
    <div>
      <h1 className="page-title">Logs</h1>
      <div className="content-card">
        <div className="input-line">
          <input placeholder="Service" value={service} onChange={e => setService(e.target.value)} />
          <input placeholder="Request ID" value={requestId} onChange={e => setRequestId(e.target.value)} />
          <input placeholder="Level" value={level} onChange={e => setLevel(e.target.value)} />
          <button type="button" className="btn-primary" onClick={load}>
            Search
          </button>
        </div>
        <p className="text-muted">Индекс из Postgres (таблица obs.logs_index). Для сырых запросов к Loki — Grafana.</p>
      </div>
      {loading ? (
        <p className="text-muted">Loading…</p>
      ) : (
        <>
          {!(logs ?? []).length && (
            <p className="text-muted" style={{ marginTop: 16 }}>
              Записей нет. Запустите <code style={{ background: 'var(--card-bg)', padding: '2px 6px', borderRadius: 4 }}>docker compose up -d log-indexer promtail</code>. Проверьте: <code style={{ background: 'var(--card-bg)', padding: '2px 6px', borderRadius: 4 }}>docker compose ps log-indexer</code>, логи: <code style={{ background: 'var(--card-bg)', padding: '2px 6px', borderRadius: 4 }}>docker compose logs log-indexer --tail 30</code>.
            </p>
          )}
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Time</th>
                  <th>Level</th>
                  <th>Service</th>
                  <th>Request ID</th>
                  <th>Message</th>
                </tr>
              </thead>
              <tbody>
                {(logs ?? []).map((l, i) => (
                  <tr key={i}>
                    <td>{l.ts ? new Date(String(l.ts)).toISOString() : ''}</td>
                    <td>
                      <span className={`badge ${levelClass(String(l.level || ''))}`}>
                        {String(l.level || 'info')}
                      </span>
                    </td>
                    <td className="mono">{String(l.service || '')}</td>
                    <td className="mono">{String(l.request_id || '')}</td>
                    <td style={{ maxWidth: 400, overflow: 'hidden', textOverflow: 'ellipsis' }}>{String(l.message || '')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </div>
  )
}
