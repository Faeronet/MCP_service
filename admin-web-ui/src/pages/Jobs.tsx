import { useState, useEffect, useMemo } from 'react'
import { listJobs, getJob } from '../api'

const PAGE_SIZE = 50

export function Jobs() {
  const [jobs, setJobs] = useState<Array<Record<string, unknown>>>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<Record<string, unknown> | null>(null)
  const [page, setPage] = useState(1)

  const load = async () => {
    try {
      const { jobs: j } = await listJobs(500)
      setJobs(Array.isArray(j) ? j : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const total = jobs.length
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const paginatedJobs = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE
    return jobs.slice(start, start + PAGE_SIZE)
  }, [jobs, page])

  const openJob = async (id: string) => {
    try {
      const j = await getJob(id)
      setSelected(j)
    } catch {
      setSelected(null)
    }
  }

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Jobs</h1>
      </div>
      <div className="content-panel">
        {loading ? (
          <p className="text-muted">Loading…</p>
        ) : (
          <>
            <div className="table-wrap">
              <div className="table-header-wrap">
                <table className="data-table data-table-header">
                  <thead>
                    <tr>
                      <th style={{ width: '22%' }}>ID</th>
                      <th style={{ width: '15%' }}>Type</th>
                      <th style={{ width: '18%' }}>Status</th>
                      <th style={{ width: '45%' }}>Created</th>
                    </tr>
                  </thead>
                </table>
              </div>
              <div className="table-body-wrap">
                {!(paginatedJobs ?? []).length ? (
                  <div className="table-empty">
                    <p className="table-empty-msg">Джобов пока нет</p>
                  </div>
                ) : (
                  <table className="data-table data-table-body">
                    <tbody>
                      {(paginatedJobs ?? []).map((j: Record<string, unknown>) => (
                        <tr key={String(j.id)}>
                          <td style={{ width: '22%' }}>
                            <button
                              type="button"
                              onClick={() => openJob(String(j.id))}
                              style={{ background: 'none', border: 'none', color: 'var(--link)', cursor: 'pointer', textDecoration: 'underline', padding: 0 }}
                            >
                              {String(j.id).slice(0, 8)}…
                            </button>
                          </td>
                          <td style={{ width: '15%' }}>{String(j.type)}</td>
                          <td style={{ width: '18%' }}>{String(j.status)}</td>
                          <td style={{ width: '45%' }}>{j.created_at ? new Date(String(j.created_at)).toLocaleString() : ''}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
            {total > 0 && (
              <div className="logs-pagination">
                <span className="logs-pagination-total">Всего: {total}</span>
                <div className="logs-pagination-buttons">
                  <button type="button" className="btn-monitor-inactive" disabled={page <= 1} onClick={() => setPage(1)} title="Первая">«</button>
                  <button type="button" className="btn-monitor-inactive" disabled={page <= 1} onClick={() => setPage(p => Math.max(1, p - 1))}>‹</button>
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
                  <button type="button" className="btn-monitor-inactive" disabled={page >= totalPages} onClick={() => setPage(p => Math.min(totalPages, p + 1))}>›</button>
                  <button type="button" className="btn-monitor-inactive" disabled={page >= totalPages} onClick={() => setPage(totalPages)} title="Последняя">»</button>
                </div>
                <span className="logs-pagination-info">Страница {page} из {totalPages}</span>
              </div>
            )}
            {selected && (
              <div className="content-card" style={{ marginTop: 24 }}>
                <h3 style={{ margin: '0 0 12px 0', fontSize: '1rem' }}>Job detail</h3>
                <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12, margin: 0 }}>{JSON.stringify(selected, null, 2)}</pre>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  )
}
