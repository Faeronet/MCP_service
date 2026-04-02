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

const PAGE_SIZE = 50

/** Дата и время по Москве: ДД.ММ.ГГГГ, чч:мм */
const fmtMSKDateTime = new Intl.DateTimeFormat('ru-RU', {
  timeZone: 'Europe/Moscow',
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
  hour12: false,
})

function formatLogDateTimeMSK(ts: unknown): string {
  if (ts == null || ts === '') return ''
  const d = new Date(String(ts))
  if (Number.isNaN(d.getTime())) return String(ts)
  return fmtMSKDateTime.format(d)
}

export function Logs() {
  const [logs, setLogs] = useState<Array<Record<string, unknown>>>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(true)
  const [service, setService] = useState('')
  const [requestId, setRequestId] = useState('')
  const [level, setLevel] = useState('')
  const [page, setPage] = useState(1)
  const [timeSort, setTimeSort] = useState<SortDir>(null)
  const [levelSort, setLevelSort] = useState<SortDir>(null)

  const onTimeSortClick = () => setTimeSort(cycleSort(timeSort))
  const onLevelSortClick = () => setLevelSort(cycleSort(levelSort))
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

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

  const load = async (pageNum: number = page) => {
    setLoading(true)
    try {
      const { logs: l, total: t } = await searchLogs({
        service: service || undefined,
        request_id: requestId || undefined,
        level: level || undefined,
        limit: PAGE_SIZE,
        offset: (pageNum - 1) * PAGE_SIZE,
      })
      setLogs(Array.isArray(l) ? l : [])
      setTotal(typeof t === 'number' ? t : 0)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load(page) }, [page])
  const onSearch = () => { setPage(1); load(1) }

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Logs</h1>
        <div className="input-line">
          <input placeholder="Level" value={level} onChange={e => setLevel(e.target.value)} />
          <input placeholder="Service" value={service} onChange={e => setService(e.target.value)} />
          <input placeholder="Request ID" value={requestId} onChange={e => setRequestId(e.target.value)} />
          <button type="button" className="btn-primary" onClick={onSearch}>
            Поиск
          </button>
        </div>
        <p className="text-muted">Индекс из Postgres (таблица obs.logs_index).</p>
      </div>
      <div className="content-panel">
        {loading ? (
          <p className="text-muted">Loading…</p>
        ) : (
          <div className="table-wrap table-wrap-logs">
            <div className="table-header-wrap">
              <table className="data-table data-table-header">
                <thead>
                  <tr>
                    <th style={{ width: '18%' }}>
                      <button type="button" className="th-sort-btn" onClick={onTimeSortClick} title="Сортировка по времени (МСК)">
                        Time (МСК) {timeSort === 'asc' && '↑'} {timeSort === 'desc' && '↓'}
                      </button>
                    </th>
                    <th style={{ width: '8%' }}>
                      <button type="button" className="th-sort-btn" onClick={onLevelSortClick} title="Сортировка по уровню">
                        Level {levelSort === 'asc' && '↑'} {levelSort === 'desc' && '↓'}
                      </button>
                    </th>
                    <th style={{ width: '12%' }}>Service</th>
                    <th style={{ width: '12%' }}>Request ID</th>
                    <th style={{ width: '50%' }}>Message</th>
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
                        <td style={{ width: '18%' }} title={l.ts ? new Date(String(l.ts)).toISOString() : ''}>
                          {formatLogDateTimeMSK(l.ts)}
                        </td>
                        <td style={{ width: '8%' }}>
                          <span className={`badge ${levelClass(String(l.level || ''))}`}>
                            {String(l.level || 'info')}
                          </span>
                        </td>
                        <td style={{ width: '12%' }} className="mono">{String(l.service || '')}</td>
                        <td style={{ width: '12%' }} className="mono">{String(l.request_id || '')}</td>
                        <td style={{ width: '50%', maxWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>{String(l.message || '')}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        )}
        {!loading && total > 0 && (
          <div className="logs-pagination">
            <span className="logs-pagination-total">Всего: {total} записей</span>
            <div className="logs-pagination-buttons">
              <button type="button" className="btn-monitor-inactive" disabled={page <= 1} onClick={() => setPage(1)} title="Первая">«</button>
              <button type="button" className="btn-monitor-inactive" disabled={page <= 1} onClick={() => setPage(p => Math.max(1, p - 1))} title="Назад">‹</button>
              <span className="logs-pagination-pages">
                {Array.from({ length: Math.min(7, totalPages) }, (_, i) => {
                  let p: number
                  if (totalPages <= 7) p = i + 1
                  else if (page <= 4) p = i + 1
                  else if (page >= totalPages - 3) p = totalPages - 6 + i
                  else p = page - 3 + i
                  return (
                    <button key={p} type="button" className={p === page ? 'btn-primary' : 'btn-monitor-inactive'} onClick={() => setPage(p)}>{p}</button>
                  )
                })}
              </span>
              <button type="button" className="btn-monitor-inactive" disabled={page >= totalPages} onClick={() => setPage(p => Math.min(totalPages, p + 1))} title="Вперёд">›</button>
              <button type="button" className="btn-monitor-inactive" disabled={page >= totalPages} onClick={() => setPage(totalPages)} title="Последняя">»</button>
            </div>
            <span className="logs-pagination-info">Страница {page} из {totalPages}</span>
          </div>
        )}
      </div>
    </div>
  )
}
