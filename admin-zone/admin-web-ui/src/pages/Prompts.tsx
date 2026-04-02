import { useCallback, useEffect, useState } from 'react'
import {
  getZoneMcpPrompt,
  listZoneMcpPrompts,
  listZones,
  putZoneMcpPrompt,
  type McpPromptRow,
  type ZoneListItem,
} from '../api'
import { useToast } from '../context/ToastContext'
import { Pencil, RefreshCw } from 'lucide-react'

type PromptTableEntry = {
  key: string
  zoneId: string
  zoneName: string
  agentOk: boolean
  row: McpPromptRow
}

export function PromptsPage() {
  const [entries, setEntries] = useState<PromptTableEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<PromptTableEntry | null>(null)
  const [modalText, setModalText] = useState('')
  const [modalLoading, setModalLoading] = useState(false)
  const [modalDirty, setModalDirty] = useState(false)
  const [saving, setSaving] = useState(false)
  const { success: toastSuccess, error: toastError } = useToast()

  const loadPrompts = useCallback(
    async (opts?: { silent?: boolean }) => {
      const silent = opts?.silent === true
      if (!silent) setLoading(true)
      try {
        const { zones: z } = await listZones()
        const zones: ZoneListItem[] = Array.isArray(z) ? z : []
        const out: PromptTableEntry[] = []
        await Promise.all(
          zones.map(async (zone) => {
            try {
              const meta = await listZoneMcpPrompts(zone.id)
              const rows = Array.isArray(meta.prompts) ? meta.prompts : []
              for (const row of rows) {
                out.push({
                  key: `${zone.id}:${row.id}`,
                  zoneId: zone.id,
                  zoneName: zone.name,
                  agentOk: zone.agent_ok,
                  row,
                })
              }
            } catch {
              /* зона без агента или без mcp-proxy/prompts — пропускаем */
            }
          })
        )
        out.sort((a, b) => {
          const t = a.row.title.localeCompare(b.row.title, undefined, { sensitivity: 'base' })
          if (t !== 0) return t
          return a.zoneName.localeCompare(b.zoneName, undefined, { sensitivity: 'base' })
        })
        setEntries(out)
      } catch (e) {
        toastError(e instanceof Error ? e.message : 'Не удалось загрузить список')
        setEntries([])
      } finally {
        if (!silent) setLoading(false)
      }
    },
    [toastError]
  )

  useEffect(() => {
    void loadPrompts()
  }, [loadPrompts])

  const openModal = async (e: PromptTableEntry) => {
    setSelected(e)
    setModalText('')
    setModalDirty(false)
    setModalLoading(true)
    try {
      const text = await getZoneMcpPrompt(e.zoneId, e.row.id)
      setModalText(text)
    } catch (err) {
      toastError(err instanceof Error ? err.message : 'Не удалось прочитать промпт')
      setModalText('')
    } finally {
      setModalLoading(false)
    }
  }

  const closeModal = () => {
    if (saving) return
    setSelected(null)
    setModalText('')
    setModalDirty(false)
  }

  const handleBackdrop = (ev: React.MouseEvent) => {
    if (ev.target === ev.currentTarget) closeModal()
  }

  const reloadModalText = async () => {
    if (!selected) return
    setModalLoading(true)
    try {
      const text = await getZoneMcpPrompt(selected.zoneId, selected.row.id)
      setModalText(text)
      setModalDirty(false)
    } catch (err) {
      toastError(err instanceof Error ? err.message : 'Ошибка перезагрузки')
    } finally {
      setModalLoading(false)
    }
  }

  const savePrompt = async () => {
    if (!selected || saving) return
    setSaving(true)
    try {
      await putZoneMcpPrompt(selected.zoneId, selected.row.id, modalText)
      setModalDirty(false)
      toastSuccess(`Промпт «${selected.row.title}» сохранён на хосте`)
      await loadPrompts({ silent: true })
    } catch (e) {
      toastError(e instanceof Error ? e.message : 'Сохранение не удалось')
    } finally {
      setSaving(false)
    }
  }

  useEffect(() => {
    setSelected((prev) => {
      if (!prev) return null
      const still = entries.find((x) => x.key === prev.key)
      return still ?? prev
    })
  }, [entries])

  return (
    <div className="page-layout zones-list-page">
      <div className="page-header">
        <h1 className="page-title">Prompts</h1>
        <p className="text-muted">
          Файлы <span className="mono-inline">mcp-proxy/prompts/*.txt</span> на хостах зон (через{' '}
          <span className="mono-inline">zone-agent</span>). Нажмите строку, чтобы открыть редактор. После правок перезапустите{' '}
          <span className="mono-inline">mcp-proxy</span>.
        </p>
      </div>
      <div className="content-panel">
        <div className="zones-toolbar">
          <button
            type="button"
            className="btn-secondary zones-refresh"
            onClick={() => void loadPrompts({ silent: entries.length > 0 })}
            disabled={loading}
          >
            <RefreshCw className="zones-icon" aria-hidden size={16} />
            Обновить список
          </button>
        </div>

        <div className="table-wrap table-wrap-logs zones-picker-table">
          <div className="table-header-wrap">
            <table className="data-table data-table-header">
              <thead>
                <tr>
                  <th style={{ width: '32%' }}>Промпт</th>
                  <th style={{ width: '28%' }}>Файл</th>
                  <th style={{ width: '22%' }}>Зона</th>
                  <th style={{ width: '18%' }}>Статус</th>
                </tr>
              </thead>
            </table>
          </div>
          <div className="table-body-wrap">
            {loading ? (
              <div className="table-empty">
                <p className="text-muted zones-table-loading">Загрузка…</p>
              </div>
            ) : entries.length === 0 ? (
              <div className="table-empty">
                <p className="table-empty-msg">Промпты не найдены</p>
                <p className="text-muted table-empty-hint">
                  Нужны записи в <span className="mono-inline">ZONE_AGENTS</span> и каталог{' '}
                  <span className="mono-inline">mcp-proxy/prompts</span> на зоне (например{' '}
                  <span className="mono-inline">chat-orchestrator</span>).
                </p>
              </div>
            ) : (
              <table className="data-table data-table-body">
                <tbody>
                  {entries.map((e) => (
                    <tr
                      key={e.key}
                      className="chat-log-row zones-row prompts-table-row"
                      onClick={() => void openModal(e)}
                      role="button"
                      tabIndex={0}
                      onKeyDown={(ev) => ev.key === 'Enter' && void openModal(e)}
                    >
                      <td style={{ width: '32%' }}>
                        <span className="zones-name">
                          <Pencil className="zones-row-icon" size={18} aria-hidden />
                          {e.row.title}
                        </span>
                      </td>
                      <td style={{ width: '28%' }} className="mono prompts-table-file">
                        {e.row.file}
                      </td>
                      <td style={{ width: '22%' }}>
                        <span className="mono">{e.zoneName}</span>
                        <span className="text-muted mono prompts-table-zone-id"> · {e.zoneId}</span>
                      </td>
                      <td style={{ width: '18%' }}>
                        <span
                          className={
                            !e.agentOk
                              ? 'zones-pill zones-pill--bad'
                              : e.row.exists
                                ? 'zones-pill zones-pill--ok'
                                : 'zones-pill zones-pill--idle'
                          }
                        >
                          {!e.agentOk ? 'agent' : e.row.exists ? `${e.row.size} B` : 'нет файла'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>

      {selected && (
        <div className="chat-modal-overlay zones-modal-overlay" onClick={handleBackdrop} role="presentation">
          <div className="chat-modal zones-modal zones-modal--wide" onClick={(ev) => ev.stopPropagation()}>
            <div className="chat-modal-header zones-modal-header">
              <div className="zones-modal-title-block">
                <h2 className="chat-modal-title">
                  <span className="zones-name">
                    <Pencil className="zones-row-icon" size={20} aria-hidden />
                    {selected.row.title}
                  </span>
                </h2>
                <p className="zones-modal-sub mono">
                  {selected.row.file} · {selected.zoneName} ({selected.zoneId})
                </p>
              </div>
              <div className="zones-modal-header-actions">
                <span className={selected.agentOk ? 'zones-pill zones-pill--ok' : 'zones-pill zones-pill--bad'}>
                  {selected.agentOk ? 'agent OK' : 'agent down'}
                </span>
                <button
                  type="button"
                  className="btn-secondary btn-sm"
                  onClick={() => void reloadModalText()}
                  disabled={modalLoading || !selected.agentOk}
                >
                  <RefreshCw size={14} aria-hidden />
                  Перечитать
                </button>
                <button
                  type="button"
                  className="btn-primary btn-sm"
                  disabled={saving || !selected.agentOk || !modalDirty}
                  onClick={() => void savePrompt()}
                >
                  {saving ? '…' : 'Сохранить'}
                </button>
                <button type="button" className="chat-modal-close" onClick={closeModal} disabled={saving} aria-label="Close">
                  ×
                </button>
              </div>
            </div>
            <div className="chat-modal-body zones-modal-body">
              {modalLoading && modalText === '' ? (
                <p className="text-muted">Загрузка текста…</p>
              ) : (
                <textarea
                  className="zones-env-textarea zones-prompt-textarea"
                  value={modalText}
                  onChange={(ev) => {
                    setModalText(ev.target.value)
                    setModalDirty(true)
                  }}
                  spellCheck={false}
                  autoComplete="off"
                  rows={20}
                  disabled={!selected.agentOk}
                />
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
