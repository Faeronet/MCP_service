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
import { ZonesPickerTable } from '../components/ZonesPickerTable'
import { RefreshCw, Sparkles } from 'lucide-react'

export function PromptsPage() {
  const [zones, setZones] = useState<ZoneListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [modalZone, setModalZone] = useState<ZoneListItem | null>(null)
  const [promptRows, setPromptRows] = useState<McpPromptRow[]>([])
  const [listErr, setListErr] = useState<string | null>(null)
  const [listLoading, setListLoading] = useState(false)
  const [textById, setTextById] = useState<Record<string, string>>({})
  const [dirtyById, setDirtyById] = useState<Record<string, boolean>>({})
  const [savingId, setSavingId] = useState<string | null>(null)
  const { success: toastSuccess, error: toastError } = useToast()

  const loadZones = useCallback(
    async (opts?: { silent?: boolean }) => {
      const silent = opts?.silent === true
      if (!silent) setLoading(true)
      try {
        const { zones: z } = await listZones()
        setZones(Array.isArray(z) ? z : [])
      } catch (e) {
        toastError(e instanceof Error ? e.message : 'Failed to load list')
        setZones([])
      } finally {
        if (!silent) setLoading(false)
      }
    },
    [toastError]
  )

  useEffect(() => {
    void loadZones()
  }, [loadZones])

  const loadModalData = useCallback(
    async (z: ZoneListItem) => {
      setListLoading(true)
      setListErr(null)
      setPromptRows([])
      setTextById({})
      setDirtyById({})
      try {
        const meta = await listZoneMcpPrompts(z.id)
        const rows = Array.isArray(meta.prompts) ? meta.prompts : []
        setPromptRows(rows)
        if (meta.error) setListErr(meta.error)
        const texts: Record<string, string> = {}
        await Promise.all(
          rows.map(async (row) => {
            try {
              texts[row.id] = await getZoneMcpPrompt(z.id, row.id)
            } catch {
              texts[row.id] = ''
            }
          })
        )
        setTextById(texts)
        setDirtyById({})
      } catch (e) {
        toastError(e instanceof Error ? e.message : 'Failed to load prompts')
        setListErr(e instanceof Error ? e.message : 'Error')
      } finally {
        setListLoading(false)
      }
    },
    [toastError]
  )

  useEffect(() => {
    if (!modalZone) {
      setPromptRows([])
      setListErr(null)
      setTextById({})
      setDirtyById({})
      return
    }
    void loadModalData(modalZone)
  }, [modalZone, loadModalData])

  const closeModal = () => setModalZone(null)

  const handleBackdrop = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) closeModal()
  }

  const savePrompt = async (promptId: string) => {
    if (!modalZone) return
    setSavingId(promptId)
    try {
      await putZoneMcpPrompt(modalZone.id, promptId, textById[promptId] ?? '')
      setDirtyById((d) => ({ ...d, [promptId]: false }))
      toastSuccess(`Prompt ${promptId} saved on host`)
    } catch (e) {
      toastError(e instanceof Error ? e.message : 'Save failed')
    } finally {
      setSavingId(null)
    }
  }

  return (
    <div className="page-layout zones-list-page">
      <div className="page-header">
        <h1 className="page-title">Prompts</h1>
        <p className="text-muted">
          Edit <span className="mono-inline">mcp-proxy</span> prompt files on the host (
          <span className="mono-inline">mcp-proxy/prompts/*.txt</span>) via{' '}
          <span className="mono-inline">zone-agent</span>. Restart <span className="mono-inline">mcp-proxy</span> after
          changes to apply.
        </p>
      </div>
      <div className="content-panel">
        <div className="zones-toolbar">
          <button
            type="button"
            className="btn-secondary zones-refresh"
            onClick={() => void loadZones({ silent: zones.length > 0 })}
            disabled={loading}
          >
            <RefreshCw className="zones-icon" aria-hidden size={16} />
            Refresh list
          </button>
        </div>
        <ZonesPickerTable
          zones={zones}
          loading={loading}
          emptyTitle="Configurations are not set"
          emptyHint={
            <>
              Add <span className="mono-inline">ZONE_AGENTS</span> in admin-backend (same as the Configurations page).
            </>
          }
          onSelectRow={setModalZone}
        />
      </div>

      {modalZone && (
        <div className="chat-modal-overlay zones-modal-overlay" onClick={handleBackdrop} role="presentation">
          <div className="chat-modal zones-modal zones-modal--wide" onClick={(e) => e.stopPropagation()}>
            <div className="chat-modal-header zones-modal-header">
              <div className="zones-modal-title-block">
                <h2 className="chat-modal-title">
                  <span className="zones-name">
                    <Sparkles className="zones-row-icon" size={20} aria-hidden />
                    {modalZone.name}
                  </span>
                </h2>
                <p className="zones-modal-sub mono">{modalZone.id}</p>
              </div>
              <div className="zones-modal-header-actions">
                <span className={modalZone.agent_ok ? 'zones-pill zones-pill--ok' : 'zones-pill zones-pill--bad'}>
                  {modalZone.agent_ok ? 'agent OK' : 'agent down'}
                </span>
                <button
                  type="button"
                  className="btn-secondary btn-sm"
                  onClick={() => modalZone && void loadModalData(modalZone)}
                  disabled={listLoading || !modalZone.agent_ok}
                >
                  <RefreshCw size={14} aria-hidden />
                  Reload
                </button>
                <button type="button" className="chat-modal-close" onClick={closeModal} aria-label="Close">
                  ×
                </button>
              </div>
            </div>
            <div className="chat-modal-body zones-modal-body">
              <p className="text-muted zones-prompts-hint">
                Files: <span className="mono-inline">mcp-proxy/prompts/</span> — only zones with that path (e.g.{' '}
                <span className="mono-inline">chat-orchestrator</span>) have prompts. Restart <span className="mono-inline">mcp-proxy</span>{' '}
                to load edits.
              </p>
              {listLoading && promptRows.length === 0 ? (
                <p className="text-muted">Loading…</p>
              ) : listErr && promptRows.length === 0 ? (
                <p className="zones-services-err">{listErr}</p>
              ) : (
                <div className="zones-prompts-list">
                  {promptRows.map((row) => (
                    <section key={row.id} className="zones-section zones-prompt-block">
                      <div className="zones-env-head">
                        <h3 className="zones-section-title">{row.title}</h3>
                        <div className="zones-env-actions">
                          <span className="text-muted mono-inline zones-prompt-meta">
                            {row.file}
                            {!row.exists ? ' · missing on disk' : ` · ${row.size} B`}
                          </span>
                          <button
                            type="button"
                            className="btn-primary btn-sm"
                            disabled={
                              !!savingId || !modalZone.agent_ok || !dirtyById[row.id]
                            }
                            onClick={() => void savePrompt(row.id)}
                          >
                            {savingId === row.id ? '…' : 'Save'}
                          </button>
                        </div>
                      </div>
                      <textarea
                        className="zones-env-textarea zones-prompt-textarea"
                        value={textById[row.id] ?? ''}
                        onChange={(e) => {
                          const v = e.target.value
                          setTextById((t) => ({ ...t, [row.id]: v }))
                          setDirtyById((d) => ({ ...d, [row.id]: true }))
                        }}
                        spellCheck={false}
                        autoComplete="off"
                        rows={14}
                        disabled={!modalZone.agent_ok}
                      />
                    </section>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
