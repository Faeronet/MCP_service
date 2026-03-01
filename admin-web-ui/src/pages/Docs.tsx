import { useState, useEffect, useMemo } from 'react'
import { listDocs, uploadFile, deleteDoc } from '../api'

const PAGE_SIZE = 20

export function Docs() {
  const [docs, setDocs] = useState<Array<{ id: string; name: string; created_at: string; versions: unknown }>>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')
  const [page, setPage] = useState(1)
  const [fileName, setFileName] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

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
  useEffect(() => {
    if (!success) return
    const t = setTimeout(() => setSuccess(''), 4000)
    return () => clearTimeout(t)
  }, [success])

  const total = docs.length
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const paginatedDocs = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE
    return docs.slice(start, start + PAGE_SIZE)
  }, [docs, page])

  const hasSelection = selectedIds.size > 0
  const allSelected = docs.length > 0 && selectedIds.size === docs.length
  const selectAllRef = useRef<HTMLInputElement>(null)
  useEffect(() => {
    const el = selectAllRef.current
    if (!el) return
    el.indeterminate = hasSelection && selectedIds.size < docs.length
  }, [hasSelection, selectedIds.size, docs.length])

  const toggleSelect = (id: string) => {
    setSelectedIds((prev: Set<string>) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (allSelected) setSelectedIds(new Set())
    else setSelectedIds(new Set(docs.map(d => d.id)))
  }

  const clearSelection = () => setSelectedIds(new Set())

  const onFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    setFileName(file.name)
    setUploading(true)
    setError('')
    setSuccess('')
    try {
      await uploadFile(file, file.name)
      setSuccess('Файл загружен.')
      setFileName('')
      await load()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setUploading(false)
    }
    e.target.value = ''
  }

  const onDeleteSelected = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return
    if (!window.confirm(`Удалить выбранные документы (${ids.length})? Чанки будут удалены из поиска.`)) return
    setError('')
    setSuccess('')
    let ok = 0
    let fail = 0
    for (const id of ids) {
      try {
        await deleteDoc(id)
        ok++
      } catch {
        fail++
      }
    }
    setSelectedIds(new Set())
    await load()
    if (fail === 0) setSuccess(`Удалено документов: ${ok}.`)
    else setError(`Удалено: ${ok}, ошибок: ${fail}.`)
  }

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Documents</h1>
        <div className="input-line">
          <label className="file-input-wrap">
            <span className="file-input-btn">Обзор…</span>
            <span className="file-input-label">{fileName || 'Файл не выбран.'}</span>
            <input
              type="file"
              className="file-input-hidden"
              onChange={onFile}
              disabled={uploading}
            />
          </label>
          {uploading && <span className="text-muted">Загрузка…</span>}
        </div>
        {error && <p className="text-error" style={{ marginBottom: 0 }}>{error}</p>}
        {success && <p className="text-muted" style={{ marginBottom: 0, color: 'var(--color-success, #0a0)' }}>{success}</p>}
      </div>
      <div className="content-panel">
        {loading ? (
          <p className="text-muted">Loading…</p>
        ) : (
          <>
            <div className="table-toolbar">
              <span className="table-toolbar-info">Выбрано: {selectedIds.size}</span>
              <div className="table-toolbar-actions">
                <button type="button" className="btn-monitor-inactive" disabled={!hasSelection} onClick={clearSelection} title="Снять все галочки">
                  Отменить
                </button>
                <button type="button" className="btn-monitor-inactive" disabled={!hasSelection} onClick={toggleSelectAll} title={allSelected ? 'Снять выделение со всех' : 'Выбрать все документы'}>
                  {allSelected ? 'Снять выделение' : 'Выбрать все'}
                </button>
                <button type="button" className="btn-primary" disabled={!hasSelection} onClick={onDeleteSelected} title="Удалить выбранные документы">
                  Удалить
                </button>
              </div>
            </div>
            <div className="table-wrap table-wrap-docs">
              <div className="table-header-wrap">
                <table className="data-table data-table-header">
                  <thead>
                    <tr>
                      <th style={{ width: '30%' }}>Name</th>
                      <th style={{ width: '42%' }}>ID</th>
                      <th style={{ width: '23%' }}>Created</th>
                      <th style={{ width: 40 }}></th>
                    </tr>
                  </thead>
                </table>
              </div>
              <div className="table-body-wrap">
                {!(paginatedDocs ?? []).length ? (
                  <div className="table-empty">
                    <p className="table-empty-msg">Документов пока нет</p>
                    <p className="text-muted table-empty-hint">Загрузите файл через кнопку «Обзор…» выше.</p>
                  </div>
                ) : (
                  <table className="data-table data-table-body">
                    <tbody>
                      {(paginatedDocs ?? []).map(d => (
                        <tr key={d.id}>
                          <td style={{ width: '30%', maxWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }} title={d.name}>{d.name}</td>
                          <td style={{ width: '42%' }} className="mono">{d.id}</td>
                          <td style={{ width: '23%' }}>{new Date(d.created_at).toLocaleString()}</td>
                          <td style={{ width: 40 }}>
                            <input
                              type="checkbox"
                              checked={selectedIds.has(d.id)}
                              onChange={() => toggleSelect(d.id)}
                              title="Выбрать документ"
                            />
                          </td>
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
          </>
        )}
      </div>
    </div>
  )
}
