import { useCallback, useEffect, useState } from 'react'
import { getRemindersConfig, setRemindersDebugClock, setRemindersDisabled } from '../api'
import { useToast } from '../context/ToastContext'

export function Reminders() {
  const { success, error: showError } = useToast()
  const [disabled, setDisabled] = useState(false)
  const [simulatedAt, setSimulatedAt] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [isoInput, setIsoInput] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const c = await getRemindersConfig()
      setDisabled(!!c.disabled)
      setSimulatedAt(c.simulated_at ?? null)
    } catch {
      showError('Не удалось загрузить настройки напоминаний')
    } finally {
      setLoading(false)
    }
  }, [showError])

  useEffect(() => {
    load()
  }, [load])

  const toggle = async () => {
    try {
      const next = !disabled
      await setRemindersDisabled(next)
      setDisabled(next)
      success(next ? 'Напоминания отключены глобально' : 'Напоминания включены')
    } catch {
      showError('Ошибка сохранения')
    }
  }

  const applySim = async () => {
    const normalized = isoInput.trim().replace('T', ' ')
    if (!normalized) {
      showError('Укажите дату/время в формате YYYY-MM-DD HH:MM, например 2026-03-30 15:09')
      return
    }
    if (!/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}(:\d{2})?$/.test(normalized)) {
      showError('Неверный формат. Используйте YYYY-MM-DD HH:MM')
      return
    }
    try {
      await setRemindersDebugClock({ simulated_iso: normalized })
      success('Симуляция времени установлена')
      load()
    } catch {
      showError('Ошибка установки времени')
    }
  }

  const clearSim = async () => {
    try {
      await setRemindersDebugClock({ clear: true })
      success('Симуляция сброшена')
      load()
    } catch {
      showError('Ошибка')
    }
  }

  if (loading) {
    return <p className="muted">Загрузка…</p>
  }

  return (
    <div className="page-layout reminders-page">
      <div className="page-header">
        <h1 className="page-title">Напоминания</h1>
        <p className="text-muted reminders-lead">
          Глобальный выключатель и симуляция времени в Postgres. Бот читает{' '}
          <code>chat.reminder_global_config</code> и <code>chat.reminder_debug_clock</code>.
        </p>
      </div>
      <div className="content-panel reminders-grid">
        <section className="content-card reminders-card">
          <h2 className="reminders-card-title">Статус</h2>
          <p className="reminders-status-line">
            Напоминания сейчас:{' '}
            <strong className={disabled ? 'reminders-state reminders-state--off' : 'reminders-state reminders-state--on'}>
              {disabled ? 'выключены' : 'включены'}
            </strong>
          </p>
          <button type="button" className="btn-primary" onClick={toggle}>
            {disabled ? 'Включить напоминания' : 'Отключить напоминания'}
          </button>
        </section>

        <section className="content-card reminders-card">
          <h2 className="reminders-card-title">Симуляция времени</h2>
          <p className="reminders-sim-line">
            Текущее значение в БД: <code>{simulatedAt ?? '— не задано'}</code>
          </p>
          <label htmlFor="rem-iso">Формат: YYYY-MM-DD HH:MM (МСК)</label>
          <input
            id="rem-iso"
            className="reminders-input"
            type="text"
            value={isoInput}
            onChange={e => setIsoInput(e.target.value)}
            placeholder="2026-03-30 15:09"
          />
          <div className="reminders-actions">
            <button type="button" className="btn-primary" onClick={applySim}>
              Применить
            </button>
            <button type="button" className="btn-secondary" onClick={clearSim}>
              Сбросить
            </button>
          </div>
        </section>

        <section className="content-card reminders-card reminders-card--full">
          <h2 className="reminders-card-title">Как это работает</h2>
          <ul className="reminders-hints">
            <li>В чате используйте формат: <code>[напоминание] HH:MM</code> (МСК).</li>
            <li>Уведомление отправляется один раз в день, когда текущее время достигает заданного.</li>
            <li>Если напоминание не приходит — проверьте глобальный выключатель и доступность сервиса бота.</li>
          </ul>
        </section>
      </div>
    </div>
  )
}
