import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'
import { getMonitorMetrics, type MonitorMetricsResponse } from '../api'

const POLL_MS = 10000

type MonitorContextValue = {
  lastData: MonitorMetricsResponse | null
  setLastData: (data: MonitorMetricsResponse | null) => void
}

const MonitorContext = createContext<MonitorContextValue | null>(null)

export function MonitorProvider({ children }: { children: ReactNode }) {
  const [lastData, setLastData] = useState<MonitorMetricsResponse | null>(null)

  const fetchMetrics = useCallback(async () => {
    try {
      const res = await getMonitorMetrics()
      setLastData(res)
    } catch {
      // Тихо игнорируем ошибки фонового обновления
    }
  }, [])

  useEffect(() => {
    fetchMetrics()
    const t = setInterval(fetchMetrics, POLL_MS)
    return () => clearInterval(t)
  }, [fetchMetrics])

  return (
    <MonitorContext.Provider value={{ lastData, setLastData }}>
      {children}
    </MonitorContext.Provider>
  )
}

export function useMonitorContext(): MonitorContextValue {
  const ctx = useContext(MonitorContext)
  if (!ctx) {
    return {
      lastData: null,
      setLastData: () => {},
    }
  }
  return ctx
}
