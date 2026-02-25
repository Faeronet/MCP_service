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
    <div>
      <h1>Documents</h1>
      <div style={{ marginBottom: 16 }}>
        <input type="file" onChange={onFile} disabled={uploading} />
        {uploading && <span style={{ marginLeft: 8 }}>Uploading…</span>}
      </div>
      {error && <p style={{ color: '#f87171' }}>{error}</p>}
      {loading ? <p>Loading…</p> : (
        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr style={{ borderBottom: '1px solid #27272a' }}>
              <th style={{ textAlign: 'left', padding: 8 }}>Name</th>
              <th style={{ textAlign: 'left', padding: 8 }}>ID</th>
              <th style={{ textAlign: 'left', padding: 8 }}>Created</th>
            </tr>
          </thead>
          <tbody>
            {(docs ?? []).map(d => (
              <tr key={d.id} style={{ borderBottom: '1px solid #27272a' }}>
                <td style={{ padding: 8 }}>{d.name}</td>
                <td style={{ padding: 8, fontFamily: 'monospace', fontSize: 12 }}>{d.id}</td>
                <td style={{ padding: 8 }}>{new Date(d.created_at).toLocaleString()}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
