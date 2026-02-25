import { useState, useEffect } from 'react'
import { listDocs, uploadFile } from '../api'

export function Docs() {
  const [docs, setDocs] = useState<Array<{ id: string; name: string; created_at: string; versions: unknown }>>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')

  const load = async () => {
    try {
      const { docs: d } = await listDocs()
      setDocs(Array.isArray(d) ? d : [])
    } catch (e) {
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const onFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setUploading(true)
    setError('')
    try {
      await uploadFile(file, file.name)
      await load()
    } catch (err) {
      setError(String(err))
    } finally {
      setUploading(false)
    }
  }

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Documents</h1>
        <div className="input-line">
          <input type="file" onChange={onFile} disabled={uploading} />
          {uploading && <span className="text-muted">Uploading…</span>}
        </div>
        {error && <p className="text-error" style={{ marginBottom: 0 }}>{error}</p>}
      </div>
      <div className="content-panel">
        {loading ? (
          <p className="text-muted">Loading…</p>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>ID</th>
                  <th>Created</th>
                </tr>
              </thead>
              <tbody>
                {(docs ?? []).map(d => (
                  <tr key={d.id}>
                    <td>{d.name}</td>
                    <td className="mono">{d.id}</td>
                    <td>{new Date(d.created_at).toLocaleString()}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
