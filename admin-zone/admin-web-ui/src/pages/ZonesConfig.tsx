import { useCallback, useEffect, useState } from 'react'
import {
  getZoneEnv,
  getZoneServices,
  listZones,
  putZoneEnv,
  zoneRebuild,
  type ZoneListItem,
  type ZoneServiceRow,
} from '../api'
import { useToast } from '../context/ToastContext'
import { RefreshCw, Server, Layers } from 'lucide-react'

export function ZonesConfig() {
  const [zones, setZones] = useState<ZoneListItem[]>([])
  const [loading, setLoading] = useState(true)
  const [modalZone, setModalZone] = useState<ZoneListItem | null>(null)
  const [services, setServices] = useState<ZoneServiceRow[]>([])
  const [servicesErr, setServicesErr] = useState<string | null>(null)
  const [servicesLoading, setServicesLoading] = useState(false)
  const [envText, setEnvText] = useState('')
  const [envLoading, setEnvLoading] = useState(false)
  const [envDirty, setEnvDirty] = useState(false)
  const [rebuildLog, setRebuildLog] = useState<string | null>(null)
  const [rebuilding, setRebuilding] = useState<string | null>(null)
  const { success: toastSuccess, error: toastError } = useToast()

  const loadZones = useCallback(
    async (opts?: { silent?: boolean }) => {
      const silent = opts?.silent === true
      if (!silent) setLoading(true)
      try {
        const { zones: z } = await listZones()
        setZones(Array.isArray(z) ? z : [])
      } catch (e) {
        toastError(e instanceof Error ? e.message : 'Не удалось загрузить зоны')
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
      setServicesLoading(true)
      setServicesErr(null)
      setEnvLoading(true)
      setRebuildLog(null)
      try {
        const [svc, env] = await Promise.all([getZoneServices(z.id), getZoneEnv(z.id)])
        setServices(Array.isArray(svc.services) ? svc.services : [])
        setServicesErr(svc.error || null)
        setEnvText(env)
        setEnvDirty(false)
      } catch (e) {
        toastError(e instanceof Error ? e.message : 'Ошибка загрузки зоны')
        setServices([])
        setEnvText('')
      } finally {
        setServicesLoading(false)
        setEnvLoading(false)
      }
    },
    [toastError]
  )

  useEffect(() => {
    if (!modalZone) {
      setServices([])
      setEnvText('')
      setEnvDirty(false)
      setRebuildLog(null)
      return
    }
    void loadModalData(modalZone)
  }, [modalZone, loadModalData])

  const closeModal = () => setModalZone(null)

  const handleBackdrop = (e: React.MouseEvent) => {
    if (e.target === e.currentTarget) closeModal()
  }

  const saveEnv = async () => {
    if (!modalZone) return
    try {
      await putZoneEnv(modalZone.id, envText)
      setEnvDirty(false)
      toastSuccess('.env сохранён на хосте')
    } catch (e) {
      toastError(e instanceof Error ? e.message : 'Ошибка сохранения')
    }
  }

  const refreshServices = async () => {
    if (!modalZone) return
    setServicesLoading(true)
    setServicesErr(null)
    try {
      const svc = await getZoneServices(modalZone.id)
      setServices(Array.isArray(svc.services) ? svc.services : [])
      setServicesErr(svc.error || null)
    } catch (e) {
      toastError(e instanceof Error ? e.message : 'Ошибка списка сервисов')
    } finally {
      setServicesLoading(false)
    }
  }

  const runRebuild = async (service?: string, all?: boolean) => {
    if (!modalZone) return
    const key = all ? '__all__' : service || ''
    setRebuilding(key)
    setRebuildLog(null)
    try {
      const out = await zoneRebuild(modalZone.id, all ? { all: true } : { service: service! })
      setRebuildLog(out.log || (out.ok ? 'Готово' : 'Ошибка'))
      if (!out.ok) toastError('Сборка завершилась с ошибкой')
      else toastSuccess(all ? 'Все сервисы обработаны' : `Сервис ${service} обновлён`)
      await refreshServices()
    } catch (e) {
      toastError(e instanceof Error ? e.message : 'Ошибка пересборки')
    } finally {
      setRebuilding(null)
    }
  }

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Конфигурация зон</h1>
        <p className="text-muted">
          Управление .env и docker compose на хосте через{' '}
          <span className="mono-inline">zone-agent</span> (см. каталог <span className="mono-inline">zone-agent</span> в репозитории).
          В <span className="mono-inline">admin-backend</span> задайте <span className="mono-inline">ZONE_AGENTS</span> (JSON).
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
            Обновить список
          </button>
        </div>
        {loading ? (
          <p className="text-muted">Загрузка…</p>
        ) : zones.length === 0 ? (
          <div className="table-empty">
            <p className="table-empty-msg">Зоны не настроены</p>
            <p className="text-muted table-empty-hint">
              Добавьте JSON в ZONE_AGENTS у admin-backend и поднимите сервис zone-agent в compose каждой зоны.
            </p>
          </div>
        ) : (
          <div className="table-wrap table-wrap-logs">
            <div className="table-header-wrap">
              <table className="data-table data-table-header">
                <thead>
                  <tr>
                    <th style={{ width: '40%' }}>Зона</th>
                    <th style={{ width: '35%' }}>ID</th>
                    <th style={{ width: '25%' }}>Агент</th>
                  </tr>
                </thead>
              </table>
            </div>
            <div className="table-body-wrap">
              <table className="data-table data-table-body">
                <tbody>
                  {zones.map((z) => (
                    <tr
                      key={z.id}
                      className="chat-log-row zones-row"
                      onClick={() => setModalZone(z)}
                      role="button"
                      tabIndex={0}
                      onKeyDown={(e) => e.key === 'Enter' && setModalZone(z)}
                    >
                      <td style={{ width: '40%' }}>
                        <span className="zones-name">
                          <Layers className="zones-row-icon" size={18} aria-hidden />
                          {z.name}
                        </span>
                      </td>
                      <td style={{ width: '35%' }} className="mono">
                        {z.id}
                      </td>
                      <td style={{ width: '25%' }}>
                        <span className={z.agent_ok ? 'zones-pill zones-pill--ok' : 'zones-pill zones-pill--bad'}>
                          {z.agent_ok ? 'доступен' : 'нет связи'}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>

      {modalZone && (
        <div className="chat-modal-overlay zones-modal-overlay" onClick={handleBackdrop} role="presentation">
          <div className="chat-modal zones-modal" onClick={(e) => e.stopPropagation()}>
            <div className="chat-modal-header zones-modal-header">
              <div className="zones-modal-title-block">
                <h2 className="chat-modal-title">{modalZone.name}</h2>
                <p className="zones-modal-sub mono">{modalZone.id}</p>
              </div>
              <div className="zones-modal-header-actions">
                <span className={modalZone.agent_ok ? 'zones-pill zones-pill--ok' : 'zones-pill zones-pill--bad'}>
                  {modalZone.agent_ok ? 'агент OK' : 'агент недоступен'}
                </span>
                <button type="button" className="btn-secondary btn-sm" onClick={() => void refreshServices()} disabled={servicesLoading}>
                  <RefreshCw size={14} aria-hidden />
                  Статусы
                </button>
                <button
                  type="button"
                  className="btn-primary btn-sm"
                  disabled={!!rebuilding || !modalZone.agent_ok}
                  onClick={() => void runRebuild(undefined, true)}
                >
                  {rebuilding === '__all__' ? '…' : 'Пересобрать все'}
                </button>
                <button type="button" className="chat-modal-close" onClick={closeModal} aria-label="Закрыть">
                  ×
                </button>
              </div>
            </div>
            <div className="chat-modal-body zones-modal-body">
              <section className="zones-section">
                <h3 className="zones-section-title">
                  <Server size={18} aria-hidden />
                  Сервисы docker compose
                </h3>
                {servicesLoading && services.length === 0 ? (
                  <p className="text-muted">Загрузка…</p>
                ) : servicesErr ? (
                  <p className="zones-services-err">{servicesErr}</p>
                ) : services.length === 0 ? (
                  <p className="text-muted">Нет сервисов или compose не найден.</p>
                ) : (
                  <ul className="zones-service-list">
                    {services.map((s) => (
                      <li key={s.name} className="zones-service-item">
                        <div className="zones-service-info">
                          <span className="mono">{s.name}</span>
                          <span className={s.running ? 'zones-pill zones-pill--ok' : 'zones-pill zones-pill--idle'}>
                            {s.running ? 'running' : s.state || 'stopped'}
                          </span>
                        </div>
                        <button
                          type="button"
                          className="btn-secondary btn-sm"
                          disabled={!!rebuilding || !modalZone.agent_ok}
                          onClick={() => void runRebuild(s.name)}
                        >
                          {rebuilding === s.name ? '…' : 'Пересобрать'}
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </section>

              {rebuildLog != null && (
                <section className="zones-section">
                  <h3 className="zones-section-title">Лог пересборки</h3>
                  <pre className="zones-rebuild-log">{rebuildLog}</pre>
                </section>
              )}

              <section className="zones-section">
                <div className="zones-env-head">
                  <h3 className="zones-section-title">Файл .env на хосте</h3>
                  <div className="zones-env-actions">
                    <button
                      type="button"
                      className="btn-secondary btn-sm"
                      disabled={envLoading}
                      onClick={() => modalZone && void loadModalData(modalZone)}
                    >
                      Перечитать
                    </button>
                    <button type="button" className="btn-primary btn-sm" disabled={envLoading || !envDirty} onClick={() => void saveEnv()}>
                      Сохранить
                    </button>
                  </div>
                </div>
                {envLoading ? (
                  <p className="text-muted">Загрузка .env…</p>
                ) : (
                  <textarea
                    className="zones-env-textarea"
                    value={envText}
                    onChange={(e) => {
                      setEnvText(e.target.value)
                      setEnvDirty(true)
                    }}
                    spellCheck={false}
                    autoComplete="off"
                    rows={18}
                  />
                )}
              </section>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
