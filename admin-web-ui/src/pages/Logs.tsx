import { useState, useEffect } from 'react'
import { searchLogs } from '../api'

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
      <h1>Logs</h1>
      <div style={{ marginBottom: 16, display: 'flex', gap: 16, flexWrap: 'wrap' }}>
        <input placeholder="Service" value={service} onChange={e => setService(e.target.value)} style={{ padding: 8 }} />
        <input placeholder="Request ID" value={requestId} onChange={e => setRequestId(e.target.value)} style={{ padding: 8 }} />
        <input placeholder="Level" value={level} onChange={e => setLevel(e.target.value)} style={{ padding: 8 }} />
        <button type="button" onClick={load} style={{ padding: '8px 16px', cursor: 'pointer' }}>Search</button>
      </div>
      <p style={{ fontSize: 12, color: '#71717a' }}>Индекс из Postgres (таблица obs.logs_index). Для сырых запросов к Loki — Grafana.</p>
      {loading ? <p>Loading…</p> : (
        <>
          {!(logs ?? []).length && (
            <p style={{ color: '#a1a1aa', marginTop: 16 }}>
              Записей нет. Запустите <code style={{ background: '#27272a', padding: '2px 6px', borderRadius: 4 }}>docker compose up -d log-indexer promtail</code>. Проверьте: <code style={{ background: '#27272a', padding: '2px 6px', borderRadius: 4 }}>docker compose ps log-indexer</code>, логи: <code style={{ background: '#27272a', padding: '2px 6px', borderRadius: 4 }}>docker compose logs log-indexer --tail 30</code>.
            </p>
          )}
          <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 13 }}>
            <thead>
              <tr style={{ borderBottom: '1px solid #27272a' }}>
                <th style={{ textAlign: 'left', padding: 8 }}>Time</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Level</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Service</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Request ID</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Message</th>
              </tr>
            </thead>
            <tbody>
              {(logs ?? []).map((l, i) => (
              <tr key={i} style={{ borderBottom: '1px solid #27272a' }}>
                <td style={{ padding: 8 }}>{l.ts ? new Date(String(l.ts)).toISOString() : ''}</td>
                <td style={{ padding: 8 }}>{String(l.level || '')}</td>
                <td style={{ padding: 8 }}>{String(l.service || '')}</td>
                <td style={{ padding: 8, fontFamily: 'monospace' }}>{String(l.request_id || '')}</td>
                <td style={{ padding: 8, maxWidth: 400, overflow: 'hidden', textOverflow: 'ellipsis' }}>{String(l.message || '')}</td>
              </tr>
            ))}
          </tbody>
        </table>
        </>
      )}
    </div>
  )
}
