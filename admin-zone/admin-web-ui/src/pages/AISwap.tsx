import { useCallback, useEffect, useState } from 'react'
import {
  getAISwapCatalog,
  getAISwapStatus,
  streamAISwap,
  type AISwapCatalogModel,
  type AISwapServiceRow,
} from '../api'
import { useToast } from '../context/ToastContext'
import { ArrowLeftRight, RefreshCw } from 'lucide-react'

const AI_ZONE_ID = (import.meta.env.VITE_AI_ZONE_ID as string | undefined)?.trim() || 'ai-zone'

export function AISwap() {
  const [status, setStatus] = useState<AISwapServiceRow[]>([])
  const [hint, setHint] = useState<string | null>(null)
  const [storeOk, setStoreOk] = useState(false)
  const [loading, setLoading] = useState(true)
  const [modalRow, setModalRow] = useState<AISwapServiceRow | null>(null)
  const [catalog, setCatalog] = useState<AISwapCatalogModel[]>([])
  const [catalogLoading, setCatalogLoading] = useState(false)
  const [swapLog, setSwapLog] = useState('')
  const [swapping, setSwapping] = useState(false)
  const toast = useToast()

  const loadStatus = useCallback(async () => {
    setLoading(true)
    try {
      const s = await getAISwapStatus(AI_ZONE_ID)
      setStatus(Array.isArray(s.services) ? s.services : [])
      setStoreOk(!!s.ai_store_configured)
      setHint(s.ai_store_hint || null)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Не удалось загрузить AI swap')
      setStatus([])
      setHint(null)
    } finally {
      setLoading(false)
    }
  }, [toast])

  useEffect(() => {
    void loadStatus()
  }, [loadStatus])

  const openModal = async (row: AISwapServiceRow) => {
    setModalRow(row)
    setSwapLog('')
    setCatalog([])
    setCatalogLoading(true)
    try {
      const c = await getAISwapCatalog(AI_ZONE_ID, row.id)
      setCatalog(Array.isArray(c.models) ? c.models : [])
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Каталог моделей')
      setCatalog([])
    } finally {
      setCatalogLoading(false)
    }
  }

  const closeModal = () => {
    if (swapping) return
    setModalRow(null)
    setCatalog([])
    setSwapLog('')
  }

  const handleBackdrop = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) closeModal()
  }

  const runSwap = async (modelId: string) => {
    if (!modalRow || swapping) return
    setSwapping(true)
    setSwapLog('')
    try {
      await streamAISwap(AI_ZONE_ID, { service: modalRow.id, model_id: modelId }, (chunk) => {
        setSwapLog((prev) => prev + chunk)
      })
      toast.success('Смена модели завершена (проверьте лог при ошибках compose)')
      await loadStatus()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Ошибка swap')
    } finally {
      setSwapping(false)
    }
  }

  return (
    <div className="page-layout zones-list-page">
      <div className="page-header">
        <h1 className="page-title">AI swap</h1>
        <p className="text-muted">
          Смена моделей LLM / Embedding / Rerank на хосте зоны <span className="mono-inline">{AI_ZONE_ID}</span> через{' '}
          <span className="mono-inline">zone-agent</span>. Каталог: файл{' '}
          <span className="mono-inline">ai-swap-catalog.json</span> в корне зоны или встроенный список. AI Store (MinIO) — позже;
          переменные <span className="mono-inline">AI_STORE_*</span> задействуются при появлении сервиса.
        </p>
        {hint && <p className="text-muted ai-swap-hint">{hint}</p>}
        <p className="text-muted">
          AI Store:{' '}
          <span className={storeOk ? 'zones-pill zones-pill--ok' : 'zones-pill zones-pill--idle'}>
            {storeOk ? 'настроен' : 'не настроен'}
          </span>
        </p>
      </div>
      <div className="content-panel">
        <div className="zones-toolbar">
          <button type="button" className="btn-secondary zones-refresh" onClick={() => void loadStatus()} disabled={loading}>
            <RefreshCw className="zones-icon" aria-hidden size={16} />
            Обновить
          </button>
        </div>

        <div className="table-wrap table-wrap-logs zones-picker-table">
          <div className="table-header-wrap">
            <table className="data-table data-table-header">
              <thead>
                <tr>
                  <th style={{ width: '18%' }}>Роль</th>
                  <th style={{ width: '18%' }}>Compose</th>
                  <th style={{ width: '44%' }}>Текущая модель</th>
                  <th style={{ width: '20%' }} />
                </tr>
              </thead>
            </table>
          </div>
          <div className="table-body-wrap">
            {loading ? (
              <div className="table-empty">
                <p className="text-muted zones-table-loading">Загрузка…</p>
              </div>
            ) : status.length === 0 ? (
              <div className="table-empty">
                <p className="table-empty-msg">Нет данных</p>
                <p className="text-muted table-empty-hint">
                  Проверьте <span className="mono-inline">ZONE_AGENTS</span> (зона <span className="mono-inline">{AI_ZONE_ID}</span>) и что на
                  zone-agent включён <span className="mono-inline">ZONE_AGENT_AI_SWAP=1</span>.
                </p>
              </div>
            ) : (
              <table className="data-table data-table-body">
                <tbody>
                  {status.map((row) => (
                    <tr key={row.id} className="chat-log-row zones-row">
                      <td style={{ width: '18%' }}>
                        <span className="zones-name">
                          <ArrowLeftRight className="zones-row-icon" size={18} aria-hidden />
                          {row.label}
                        </span>
                      </td>
                      <td style={{ width: '18%' }} className="mono">
                        {row.compose_service}
                      </td>
                      <td style={{ width: '44%' }} className="mono ai-swap-model-cell">
                        {row.current_model}
                      </td>
                      <td style={{ width: '20%' }}>
                        <button type="button" className="btn-primary btn-sm" onClick={() => void openModal(row)}>
                          Сменить модель
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>

      {modalRow && (
        <div className="chat-modal-overlay zones-modal-overlay" onClick={handleBackdrop} role="presentation">
          <div className="chat-modal zones-modal" onClick={(e) => e.stopPropagation()}>
            <div className="chat-modal-header zones-modal-header">
              <div className="zones-modal-title-block">
                <h2 className="chat-modal-title">
                  {modalRow.label} · <span className="mono">{modalRow.compose_service}</span>
                </h2>
                <p className="zones-modal-sub">
                  Текущая модель: <span className="mono">{modalRow.current_model}</span>
                </p>
              </div>
              <button type="button" className="chat-modal-close" onClick={closeModal} disabled={swapping} aria-label="Close">
                ×
              </button>
            </div>
            <div className="chat-modal-body zones-modal-body">
              <section className="zones-section">
                <h3 className="zones-section-title">Модели из каталога (AI store позже)</h3>
                {catalogLoading ? (
                  <p className="text-muted">Загрузка каталога…</p>
                ) : catalog.length === 0 ? (
                  <p className="text-muted">Нет позиций в каталоге.</p>
                ) : (
                  <div className="ai-swap-model-buttons">
                    {catalog.map((m) => (
                      <button
                        key={m.id}
                        type="button"
                        className="btn-secondary ai-swap-model-btn"
                        disabled={swapping || m.id === modalRow.current_model}
                        onClick={() => void runSwap(m.id)}
                        title={m.id}
                      >
                        <span className="ai-swap-model-btn-label">{m.label}</span>
                        <span className="mono ai-swap-model-btn-id">{m.id}</span>
                      </button>
                    ))}
                  </div>
                )}
              </section>
              {swapLog ? (
                <section className="zones-section">
                  <h3 className="zones-section-title">Лог</h3>
                  <pre className="zones-rebuild-log ai-swap-log">{swapLog}</pre>
                </section>
              ) : null}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
