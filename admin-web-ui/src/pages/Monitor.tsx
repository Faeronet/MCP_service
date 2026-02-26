import { useState, useEffect, useMemo } from 'react'
import { getMonitorMetrics, type MonitorMetricsResponse } from '../api'
import { SpeedometerGauge } from '../components/SpeedometerGauge'

type MonitorType = 'system' | 'gpu'

const POLL_MS = 3000
const MAX_HISTORY = 60

export function Monitor() {
  const [mode, setMode] = useState<MonitorType>('system')
  const [data, setData] = useState<MonitorMetricsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    const fetchData = async () => {
      try {
        const res = await getMonitorMetrics()
        if (cancelled) return
        setData(res)
        setError(null)
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : 'Failed to load')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    fetchData()
    const t = setInterval(fetchData, POLL_MS)
    return () => { cancelled = true; clearInterval(t) }
  }, [])

  const chartData = useMemo(() => (data?.history ?? []).slice(-MAX_HISTORY), [data?.history])

  if (loading && !data) {
    return (
      <div className="page-layout">
        <div className="page-header">
          <h1 className="page-title">Monitor</h1>
        </div>
        <div className="content-panel">
          <p className="text-muted">Loading…</p>
        </div>
      </div>
    )
  }

  return (
    <div className="page-layout">
      <div className="page-header">
        <h1 className="page-title">Monitor</h1>
        <p className="text-muted">Нагрузка с контейнеров с ИИ-моделями.</p>
        <div className="monitor-type-toggle">
          <button
            type="button"
            className={mode === 'system' ? 'active' : ''}
            onClick={() => setMode('system')}
          >
            CPU, RAM, Disk I/O
          </button>
          <button
            type="button"
            className={mode === 'gpu' ? 'active' : ''}
            onClick={() => setMode('gpu')}
          >
            GPU, VRAM
          </button>
        </div>
      </div>
      <div className="content-panel">
        {error && <p className="text-error">{error}</p>}
        {mode === 'system' && data && (
          <>
            <div className="monitor-gauges">
              <SpeedometerGauge value={data.system.cpu_pct} label="CPU" unit="%" />
              <SpeedometerGauge value={data.system.ram_pct} label="RAM" unit="%" />
              <SpeedometerGauge value={data.system.disk_io_k} max={8000} label="Disk I/O" unit="k" />
            </div>
            <div className="monitor-chart-panel">
              <h3 className="monitor-chart-title">System</h3>
              <MonitorTimeChart data={chartData} type="system" />
            </div>
          </>
        )}
        {mode === 'gpu' && data && (
          <>
            <div className="monitor-gauges monitor-gauges-single">
              <DualBandGauge
                innerValue={data.gpu.gpu_pct}
                outerValue={data.gpu.vram_pct}
                innerLabel="GPU"
                outerLabel="VRAM"
              />
            </div>
            <div className="monitor-chart-panel">
              <h3 className="monitor-chart-title">GPU / VRAM</h3>
              <MonitorTimeChart data={chartData} type="gpu" />
            </div>
          </>
        )}
      </div>
    </div>
  )
}

const CHART_COLORS = { cpu: '#22c55e', ram: '#3b82f6', disk_io: '#3b82f6', gpu: '#06b6d4', vram: '#eab308' }

function MonitorTimeChart({
  data,
  type,
}: {
  data: MonitorMetricsResponse['history']
  type: 'system' | 'gpu'
}) {
  const width = 800
  const height = 220
  const padding = { top: 20, right: 50, bottom: 32, left: 44 }
  const innerW = width - padding.left - padding.right
  const innerH = height - padding.top - padding.bottom

  const series = useMemo(() => {
    if (type === 'system') {
      const cpu = data.map(d => d.cpu)
      const ram = data.map(d => d.ram)
      const disk = data.map(d => d.disk_io)
      const maxVal = Math.max(100, ...cpu, ...ram, Math.max(...disk) / 1000)
      return {
        cpu: { values: cpu, color: CHART_COLORS.cpu, scale: maxVal },
        ram: { values: ram, color: CHART_COLORS.ram, scale: maxVal },
        disk_io: { values: disk.map(d => d / 1000), color: CHART_COLORS.disk_io, scale: Math.max(maxVal, 8) },
      }
    }
    const gpu = data.map(d => d.gpu)
    const vram = data.map(d => d.vram)
    const maxVal = Math.max(100, ...gpu, ...vram)
    return {
      gpu: { values: gpu, color: CHART_COLORS.gpu, scale: maxVal },
      vram: { values: vram, color: CHART_COLORS.vram, scale: maxVal },
    }
  }, [data, type])

  const keys = type === 'system' ? (['cpu', 'ram', 'disk_io'] as const) : (['gpu', 'vram'] as const)
  const n = data.length
  const xScale = (i: number) => (n <= 1 ? padding.left : padding.left + (i / Math.max(1, n - 1)) * innerW)
  const yScale = (val: number, maxV: number) =>
    padding.top + innerH - (maxV ? (val / maxV) * innerH : 0)

  const paths = useMemo(() => {
    const out: Record<string, string> = {}
    keys.forEach(key => {
      const s = series[key as keyof typeof series]
      if (!s || !s.values.length) return
      const maxV = s.scale
      let path = `M ${xScale(0)} ${yScale(s.values[0], maxV)}`
      for (let i = 1; i < s.values.length; i++) {
        path += ` L ${xScale(i)} ${yScale(s.values[i], maxV)}`
      }
      path += ` L ${xScale(s.values.length - 1)} ${height - padding.bottom} L ${padding.left} ${height - padding.bottom} Z`
      out[key] = path
    })
    return out
  }, [series, keys, width, height])

  const labels = type === 'system'
    ? { cpu: 'CPU', ram: 'RAM', disk_io: 'Disk I/O (k)' }
    : { gpu: 'GPU', vram: 'VRAM' }
  const lastTs = data.length ? data[data.length - 1]?.ts : ''
  const timeLabel = lastTs ? new Date(lastTs).toLocaleString() : ''

  return (
    <div className="monitor-chart-wrap">
      <div className="monitor-chart-legend">
        {keys.map(k => (
          <span key={k} className="monitor-legend-item">
            <span className="monitor-legend-dot" style={{ background: series[k]?.color }} />
            {labels[k]}
          </span>
        ))}
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="monitor-chart-svg">
        <defs>
          {keys.map(k => (
            <linearGradient key={k} id={`grad-${k}`} x1="0" y1="1" x2="0" y2="0">
              <stop offset="0%" stopColor={series[k]?.color} stopOpacity={0.4} />
              <stop offset="100%" stopColor={series[k]?.color} stopOpacity={0.05} />
            </linearGradient>
          ))}
        </defs>
        {/* grid */}
        {[1, 2, 3, 4, 5].map(i => (
          <line
            key={i}
            x1={padding.left}
            y1={padding.top + (innerH * i) / 5}
            x2={width - padding.right}
            y2={padding.top + (innerH * i) / 5}
            className="monitor-grid-line"
          />
        ))}
        {[1, 2, 3, 4].map(i => (
          <line
            key={i}
            x1={padding.left + (innerW * i) / 4}
            y1={padding.top}
            x2={padding.left + (innerW * i) / 4}
            y2={height - padding.bottom}
            className="monitor-grid-line"
          />
        ))}
        {keys.map(k => (
          <path key={`area-${k}`} d={areaPaths[k]} fill={`url(#grad-${k})`} className="monitor-area" />
        ))}
        {keys.map(k => (
          <path key={`line-${k}`} d={linePaths[k]} fill="none" stroke={series[k]?.color} strokeWidth={2} className="monitor-line" />
        ))}
        <text x={width / 2} y={height - 6} textAnchor="middle" className="monitor-axis-label">
          {timeLabel}
        </text>
      </svg>
    </div>
  )
}
