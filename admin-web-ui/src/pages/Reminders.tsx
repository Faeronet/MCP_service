import { useCallback, useEffect, useState } from 'react'
import { getRemindersConfig, setRemindersDebugClock, setRemindersDisabled } from '../api'
import { useToast } from '../context/ToastContext'

function reminderDebugFromStorage(): boolean {
  return localStorage.getItem('reminder_debug') === '1'
}

export function Reminders() {
  const { success, error: showError } = useToast()
  const [disabled, setDisabled] = useState(false)
  const [simulatedAt, setSimulatedAt] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [isoInput, setIsoInput] = useState('')
  const canDebug = reminderDebugFromStorage()

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
      show(next ? 'Напоминания отключены глобально' : 'Напоминания включены')
    } catch {
      show('Ошибка сохранения')
    }
  }

  const applySim = async () => {
    if (!isoInput.trim()) {
      error('Укажите дату/время в RFC3339, например 2026-03-29T09:30:00+03:00')
      return
    }
    try {
      await setRemindersDebugClock({ simulated_iso: isoInput.trim() })
      success('Симуляция времени установлена (бот увидит при BOT_DEBUG=1)')
      load()
    } catch (e) {
      if (e instanceof Error && e.message === 'FORBIDDEN') showError('Доступ только для супер-админа (REMINDER_SUPERADMIN_SUB)')
      else showError('Ошибка установки времени')
    }
  }

  const clearSim = async () => {
    try {
      await setRemindersDebugClock({ clear: true })
      success('Симуляция сброшена')
      load()
    } catch (e) {
      if (e instanceof Error && e.message === 'FORBIDDEN') error('Доступ только для супер-админа')
      else error('Ошибка')
    }
  }

  if (loading) {
    return <p className="muted">Загрузка…</p>
  }

  return (
    <div className="reminders-page">
      <h1>Напоминания (бот)</h1>
      <p className="muted">
        Глобальный выключатель и симуляция времени в Postgres. Бот читает <code>chat.reminder_global_config</code> и{' '}
        <code>reminder_debug_clock</code>; симулированное время учитывается только при <code>BOT_DEBUG=1</code>.
      </p>

      <section className="card-block">
        <h2>Статус</h2>
        <p>
          Напоминания сейчас: <strong>{disabled ? 'выключены' : 'включены'}</strong>
        </p>
        <button type="button" className="btn-primary" onClick={toggle}>
          {disabled ? 'Включить напоминания' : 'Отключить напоминания'}
        </button>
      </section>

      <section className="card-block">
        <h2>Симуляция времени (админ)</h2>
        {!canDebug ? (
          <p className="muted">Доступно только при входе под логином из <code>REMINDER_SUPERADMIN_SUB</code> (по умолчанию <code>admin</code>).</p>
        ) : (
          <>
            <p>
              Текущее значение в БД:{' '}
              <code>{simulatedAt ?? '— не задано'}</code>
            </p>
            <label htmlFor="rem-iso">RFC3339 (МСК можно через +03:00)</label>
            <input
              id="rem-iso"
              type="text"
              value={isoInput}
              onChange={e => setIsoInput(e.target.value)}
              placeholder="2026-03-29T09:30:00+03:00"
              style={{ width: '100%', maxWidth: '28rem', display: 'block', marginBottom: '0.5rem' }}
            />
            <div style={{ display: 'flex', gap: '0.5rem', flexWrap: 'wrap' }}>
              <button type="button" className="btn-primary" onClick={applySim}>
                Применить
              </button>
              <button type="button" className="btn-secondary" onClick={clearSim}>
                Сбросить
              </button>
            </div>
          </>
        )}
      </section>
    </div>
  )
}
