import { createContext, useCallback, useContext, useReducer } from 'react'

export type ToastType = 'success' | 'error'

export interface ToastItem {
  id: string
  message: string
  type: ToastType
  createdAt: number
}

type ToastState = ToastItem[]

type ToastAction =
  | { type: 'add'; payload: Omit<ToastItem, 'id' | 'createdAt'> }
  | { type: 'remove'; payload: string }

function toastReducer(state: ToastState, action: ToastAction): ToastState {
  switch (action.type) {
    case 'add':
      return [
        ...state,
        {
          ...action.payload,
          id: `toast-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`,
          createdAt: Date.now(),
        },
      ]
    case 'remove':
      return state.filter((t) => t.id !== action.payload)
    default:
      return state
  }
}

type ToastContextValue = {
  toasts: ToastItem[]
  success: (message: string) => void
  error: (message: string) => void
  remove: (id: string) => void
}

const ToastContext = createContext<ToastContextValue | null>(null)

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, dispatch] = useReducer(toastReducer, [])

  const success = useCallback((message: string) => {
    dispatch({ type: 'add', payload: { message, type: 'success' } })
  }, [])

  const error = useCallback((message: string) => {
    dispatch({ type: 'add', payload: { message, type: 'error' } })
  }, [])

  const remove = useCallback((id: string) => {
    dispatch({ type: 'remove', payload: id })
  }, [])

  return (
    <ToastContext.Provider value={{ toasts, success, error, remove }}>
      {children}
    </ToastContext.Provider>
  )
}

export function useToast() {
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within ToastProvider')
  return ctx
}
