import { useEffect, useState } from 'react'
import { X } from 'lucide-react'
import type { ToastItem as ToastItemType } from '../context/ToastContext'

const TOAST_DURATION_MS = 4000

interface ToastItemProps {
  item: ToastItemType
  onRemove: (id: string) => void
}

export function ToastItem({ item, onRemove }: ToastItemProps) {
  const [exiting, setExiting] = useState(false)
  const [progress, setProgress] = useState(100)

  useEffect(() => {
    const start = Date.now()
    const end = start + TOAST_DURATION_MS
    const tick = () => {
      const now = Date.now()
      if (now >= end) {
        setExiting(true)
        return
      }
      setProgress(((end - now) / TOAST_DURATION_MS) * 100)
      requestAnimationFrame(tick)
    }
    const raf = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(raf)
  }, [])

  useEffect(() => {
    if (!exiting) return
    const t = setTimeout(() => onRemove(item.id), 300)
    return () => clearTimeout(t)
  }, [exiting, item.id, onRemove])

  const isError = item.type === 'error'

  return (
    <div
      className={`toast toast--${item.type} ${exiting ? 'toast--exiting' : ''}`}
      role="alert"
    >
      <div className="toast__content">
        {isError && <span className="toast__label toast__label--error">Ошибка:</span>}
        <span className="toast__message">{item.message}</span>
      </div>
      <button
        type="button"
        className="toast__close"
        onClick={() => setExiting(true)}
        aria-label="Закрыть"
      >
        <X size={16} />
      </button>
      <div
        className="toast__progress"
        style={{ width: `${progress}%` }}
        aria-hidden
      />
    </div>
  )
}

interface ToastContainerProps {
  toasts: ToastItemType[]
  onRemove: (id: string) => void
}

export function ToastContainer({ toasts, onRemove }: ToastContainerProps) {
  return (
    <div className="toast-container" aria-live="polite">
      {toasts.map((item) => (
        <ToastItem key={item.id} item={item} onRemove={onRemove} />
      ))}
    </div>
  )
}
