import { useState, useEffect } from 'react'
import { listJobs, getJob } from '../api'

export function Jobs() {
  const [jobs, setJobs] = useState<Array<Record<string, unknown>>>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<Record<string, unknown> | null>(null)

  const load = async () => {
    try {
      const { jobs: j } = await listJobs(100)
      setJobs(Array.isArray(j) ? j : [])
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const openJob = async (id: string) => {
    try {
      const j = await getJob(id)
      setSelected(j)
    } catch {
      setSelected(null)
    }
  }

  return (
    <div>
      <h1 className="page-title">Jobs</h1>
      {loading ? (
        <p className="text-muted">Loading…</p>
      ) : (
        <>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Type</th>
                  <th>Status</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {(jobs ?? []).map((j: Record<string, unknown>) => (
                  <tr key={String(j.id)}>
                    <td>
                      <button
                        type="button"
                        onClick={() => openJob(String(j.id))}
                        style={{ background: 'none', border: 'none', color: 'var(--link)', cursor: 'pointer', textDecoration: 'underline', padding: 0 }}
                      >
                        {String(j.id).slice(0, 8)}…
                      </button>
                    </td>
                    <td>{String(j.type)}</td>
                    <td>{String(j.status)}</td>
                    <td>{j.created_at ? new Date(String(j.created_at)).toLocaleString() : ''}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {selected && (
            <div className="content-card" style={{ marginTop: 24 }}>
              <h3 style={{ margin: '0 0 12px 0', fontSize: '1rem' }}>Job detail</h3>
              <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12, margin: 0 }}>{JSON.stringify(selected, null, 2)}</pre>
            </div>
          )}
        </>
      )}
    </div>
  )
}
