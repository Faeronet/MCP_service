import { useState, useEffect, useMemo } from 'react'
import { searchLogs } from '../api'

type SortDir = 'asc' | 'desc' | null

function levelClass(level: string): string {
  const l = String(level).toLowerCase()
  if (l === 'error') return 'badge-error'
  if (l === 'warn' || l === 'warning') return 'badge-warning'
  return 'badge-info'
}

function levelWeight(level: string): number {
  const l = String(level).toLowerCase()
  if (l === 'error') return 0
  if (l === 'warn' || l === 'warning') return 1
  if (l === 'info') return 2
  return 3
}

function cycleSort(current: SortDir): SortDir {
  if (current === null) return 'asc'
  if (current === 'asc') return 'desc'
  return null
}

export function Logs() {
  const [logs, setLogs] = useState<Array<Record<string, unknown>>>([])
  const [loading, setLoading] = useState(true)
  const [service, setService] = useState('')
  const [requestId, setRequestId] = useState('')
  const [level, setLevel] = useState('')
  const [timeSort, setTimeSort] = useState<SortDir>(null)
  const [levelSort, setLevelSort] = useState<SortDir>(null)

  const onTimeSortClick = () => setTimeSort(cycleSort(timeSort))
  const onLevelSortClick = () => setLevelSort(cycleSort(levelSort))

  const sortedLogs = useMemo(() => {
    const list = [...(logs ?? [])]
    if (list.length === 0) return list
    const timeVal = (x: unknown) => (x ? new Date(String(x)).getTime() : 0)
    return list.sort((a, b) => {
      if (levelSort) {
        const c = Math.sign(levelWeight(String(a.level)) - levelWeight(String(b.level))) * (levelSort === 'asc' ? 1 : -1)
        if (c !== 0) return c
      }
      if (timeSort) {
        return Math.sign(timeVal(a.ts) - timeVal(b.ts)) * (timeSort === 'asc' ? 1 : -1)
      }
      return 0
    })
  }, [logs, timeSort, levelSort])

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
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Logs</h1>
        <div className="input-line">
          <input placeholder="Level" value={level} onChange={e => setLevel(e.target.value)} />
          <input placeholder="Service" value={service} onChange={e => setService(e.target.value)} />
          <input placeholder="Request ID" value={requestId} onChange={e => setRequestId(e.target.value)} />
          <button type="button" className="btn-primary" onClick={load}>
            Search
          </button>
        </div>
        <p className="text-muted">Индекс из Postgres (таблица obs.logs_index). Для сырых запросов к Loki — Grafana.</p>
      </div>
      <div className="content-panel">
        {loading ? (
          <p className="text-muted">Loading…</p>
        ) : (
          <div className="table-wrap">
            <div className="table-header-wrap">
              <table className="data-table data-table-header">
                <thead>
                  <tr>
                    <th style={{ width: '14%' }}>
                      <button type="button" className="th-sort-btn" onClick={onTimeSortClick} title="Сортировка по времени">
                        Time {timeSort === 'asc' && '↑'} {timeSort === 'desc' && '↓'}
                      </button>
                    </th>
                    <th style={{ width: '8%' }}>
                      <button type="button" className="th-sort-btn" onClick={onLevelSortClick} title="Сортировка по уровню">
                        Level {levelSort === 'asc' && '↑'} {levelSort === 'desc' && '↓'}
                      </button>
                    </th>
                    <th style={{ width: '12%' }}>Service</th>
                    <th style={{ width: '12%' }}>Request ID</th>
                    <th style={{ width: '54%' }}>Message</th>
                  </tr>
                </thead>
              </table>
            </div>
            <div className="table-body-wrap">
              {!(sortedLogs ?? []).length ? (
                <div className="table-empty">
                  <p className="table-empty-msg">Пока данных нет</p>
                  <p className="text-muted table-empty-hint">
                    Запустите <code>docker compose up -d log-indexer promtail</code>. Проверьте: <code>docker compose ps log-indexer</code>, логи: <code>docker compose logs log-indexer --tail 30</code>.
                  </p>
                </div>
              ) : (
                <table className="data-table data-table-body">
                  <tbody>
                    {sortedLogs.map((l: Record<string, unknown>, i: number) => (
                      <tr key={i}>
                        <td style={{ width: '14%' }}>{l.ts ? new Date(String(l.ts)).toISOString() : ''}</td>
                        <td style={{ width: '8%' }}>
                          <span className={`badge ${levelClass(String(l.level || ''))}`}>
                            {String(l.level || 'info')}
                          </span>
                        </td>
                        <td style={{ width: '12%' }} className="mono">{String(l.service || '')}</td>
                        <td style={{ width: '12%' }} className="mono">{String(l.request_id || '')}</td>
                        <td style={{ width: '54%', maxWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>{String(l.message || '')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
