import { useState, useEffect } from 'react'
import { listJobs, getJob } from '../api'

export function Jobs() {
  const [jobs, setJobs] = useState<Array<Record<string, unknown>>>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<Record<string, unknown> | null>(null)

  const load = async () => {
    try {
      const { jobs: j } = await listJobs(100)
      setJobs(j)
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
      <h1>Jobs</h1>
      {loading ? <p>Loading…</p> : (
        <>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ borderBottom: '1px solid #27272a' }}>
                <th style={{ textAlign: 'left', padding: 8 }}>ID</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Type</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Status</th>
                <th style={{ textAlign: 'left', padding: 8 }}>Created</th>
              </tr>
            </thead>
            <tbody>
              {jobs.map((j: Record<string, unknown>) => (
                <tr key={String(j.id)} style={{ borderBottom: '1px solid #27272a' }}>
                  <td style={{ padding: 8 }}>
                    <button type="button" onClick={() => openJob(String(j.id))} style={{ background: 'none', border: 'none', color: '#818cf8', cursor: 'pointer', textDecoration: 'underline' }}>
                      {String(j.id).slice(0, 8)}…
                    </button>
                  </td>
                  <td style={{ padding: 8 }}>{String(j.type)}</td>
                  <td style={{ padding: 8 }}>{String(j.status)}</td>
                  <td style={{ padding: 8 }}>{j.created_at ? new Date(String(j.created_at)).toLocaleString() : ''}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {selected && (
            <div style={{ marginTop: 24, padding: 16, background: '#18181b', borderRadius: 8 }}>
              <h3>Job detail</h3>
              <pre style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{JSON.stringify(selected, null, 2)}</pre>
            </div>
          )}
        </>
      )}
    </div>
  )
}
