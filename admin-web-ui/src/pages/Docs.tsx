import { useState, useEffect, useMemo } from 'react'
import { listDocs, uploadFile, deleteDoc } from '../api'
import { useToast } from '../context/ToastContext'

type SortDir = 'asc' | 'desc' | null

function cycleSort(current: SortDir): SortDir {
  if (current === null) return 'asc'
  if (current === 'asc') return 'desc'
  return null
}

const PAGE_SIZE = 20

export function Docs() {
  const [docs, setDocs] = useState<Array<{ id: string; name: string; created_at: string; versions: unknown }>>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [page, setPage] = useState(1)
  const [fileLabel, setFileLabel] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
  const [nameSort, setNameSort] = useState<SortDir>(null)
  const [createdSort, setCreatedSort] = useState<SortDir>(null)
  const toast = useToast()

  const load = async () => {
    try {
      const { docs: d } = await listDocs()
      setDocs(Array.isArray(d) ? d : [])
    } catch (e) {
      toast.error(String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const sortedDocs = useMemo(() => {
    const list = [...docs]
    if (list.length === 0) return list
    const createdVal = (d: { created_at: string }) => new Date(d.created_at).getTime()
    return list.sort((a, b) => {
      if (nameSort) {
        const c = String(a.name).localeCompare(String(b.name), undefined, { sensitivity: 'base' }) * (nameSort === 'asc' ? 1 : -1)
        if (c !== 0) return c
      }
      if (createdSort) {
        return Math.sign(createdVal(a) - createdVal(b)) * (createdSort === 'asc' ? 1 : -1)
      }
      return 0
    })
  }, [docs, nameSort, createdSort])

  const total = sortedDocs.length
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))
  const paginatedDocs = useMemo(() => {
    const start = (page - 1) * PAGE_SIZE
    return sortedDocs.slice(start, start + PAGE_SIZE)
  }, [sortedDocs, page])

  const hasSelection = selectedIds.size > 0
  const allSelected = sortedDocs.length > 0 && selectedIds.size === sortedDocs.length

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
    else setSelectedIds(new Set(sortedDocs.map(d => d.id)))
  }

  const clearSelection = () => setSelectedIds(new Set())

  const onFiles = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files
    if (!files?.length) return
    const list = Array.from(files)
    setFileLabel(list.length === 1 ? list[0].name : `Файлов: ${list.length}`)
    setUploading(true)
    let ok = 0
    let fail = 0
    try {
      for (const file of list) {
        try {
          await uploadFile(file, file.name)
          ok++
        } catch {
          fail++
        }
      }
      await load()
      if (fail === 0) toast.success(ok === 1 ? 'Файл загружен.' : `Загружено файлов: ${ok}.`)
      else toast.error(`Загружено: ${ok}, ошибок: ${fail}.`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setUploading(false)
      setFileLabel('')
      e.target.value = ''
    }
  }

  const onDeleteSelected = async () => {
    const ids = Array.from(selectedIds)
    if (ids.length === 0) return
    if (!window.confirm(`Удалить выбранные документы (${ids.length})? Чанки будут удалены из поиска.`)) return
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
    if (fail === 0) toast.success(`Удалено документов: ${ok}.`)
    else toast.error(`Удалено: ${ok}, ошибок: ${fail}.`)
  }

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Documents</h1>
        <div className="input-line">
          <label className="file-input-wrap">
            <span className="file-input-btn">Обзор…</span>
            <span className="file-input-label">{fileLabel || 'Файл(ы) не выбраны.'}</span>
            <input
              type="file"
              className="file-input-hidden"
              multiple
              onChange={onFiles}
              disabled={uploading}
            />
          </label>
          {uploading && <span className="text-muted">Загрузка…</span>}
        </div>
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
                <button type="button" className="btn-monitor-inactive" disabled={docs.length === 0} onClick={toggleSelectAll} title={allSelected ? 'Снять выделение со всех' : 'Выбрать все документы'}>
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
                      <th style={{ width: '30%' }}>
                        <button type="button" className="th-sort-btn" onClick={() => setNameSort(cycleSort(nameSort))} title="Сортировка по имени">
                          Name {nameSort === 'asc' && '↑'} {nameSort === 'desc' && '↓'}
                        </button>
                      </th>
                      <th style={{ width: '42%' }}>ID</th>
                      <th style={{ width: '23%' }}>
                        <button type="button" className="th-sort-btn" onClick={() => setCreatedSort(cycleSort(createdSort))} title="Сортировка по дате">
                          Created {createdSort === 'asc' && '↑'} {createdSort === 'desc' && '↓'}
                        </button>
                      </th>
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
