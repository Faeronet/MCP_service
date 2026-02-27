import { useState, useEffect, useMemo } from 'react'
import { getMonitorMetrics, type MonitorMetricsResponse, type GPUMetrics } from '../api'
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
  const gpus = useMemo((): GPUMetrics[] => {
    if (!data) return []
    if (data.gpus?.length) return data.gpus
    if (data.gpu) return [{ name: 'GPU', gpu_pct: data.gpu.gpu_pct, vram_pct: data.gpu.vram_pct }]
    return []
  }, [data])

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
            className={mode === 'system' ? 'btn-primary' : 'btn-monitor-inactive'}
            onClick={() => setMode('system')}
          >
            CPU, RAM, Disk I/O
          </button>
          <button
            type="button"
            className={mode === 'gpu' ? 'btn-primary' : 'btn-monitor-inactive'}
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
              <SpeedometerGauge
                value={data.system.ram_pct}
                label="RAM"
                unit="%"
                valueLabel={
                  data.system.ram_used_gb != null
                    ? `${data.system.ram_used_gb.toFixed(1)} GB`
                    : undefined
                }
              />
              <SpeedometerGauge value={data.system.disk_io_k} max={8000} label="Disk I/O" unit="k" />
            </div>
            <div className="monitor-chart-panel monitor-chart-panel--normal">
              <h3 className="monitor-chart-title">CPU</h3>
              <MonitorTimeChart data={chartData} type="cpu" />
            </div>
            <div className="monitor-chart-panel monitor-chart-panel--normal">
              <h3 className="monitor-chart-title">RAM</h3>
              <MonitorTimeChart data={chartData} type="ram" />
            </div>
            <div className="monitor-chart-panel monitor-chart-panel--normal">
              <h3 className="monitor-chart-title">Disk I/O</h3>
              <MonitorTimeChart data={chartData} type="disk_io" />
            </div>
          </>
        )}
        {mode === 'gpu' && data && gpus.length > 0 && (
          <>
            {gpus.map((card, idx) => (
              <div key={idx} className="gpu-card-frame">
                <h3 className="gpu-card-title">{card.name}</h3>
                <div className="monitor-gauges">
                  <SpeedometerGauge value={card.gpu_pct} label="GPU" unit="%" />
                  <SpeedometerGauge
                    value={card.vram_pct}
                    label="VRAM"
                    unit="%"
                    fillColor="#eab308"
                    valueLabel={
                      card.vram_used_gb != null
                        ? `${card.vram_used_gb.toFixed(2)} GB`
                        : undefined
                    }
                  />
                </div>
                <div className="monitor-chart-panel monitor-chart-panel--normal">
                  <h3 className="monitor-chart-title">GPU / VRAM</h3>
                  <MonitorTimeChart data={chartData} type="gpu" gpuIndex={idx} />
                </div>
              </div>
            ))}
          </>
        )}
      </div>
    </div>
  )
}

const CHART_COLORS = { cpu: '#22c55e', ram: '#3b82f6', disk_io: '#6366f1', gpu: '#06b6d4', vram: '#eab308' }

function MonitorTimeChart({
  data,
  type,
  gpuIndex = 0,
}: {
  data: MonitorMetricsResponse['history']
  type: 'cpu' | 'ram' | 'disk_io' | 'gpu'
  gpuIndex?: number
}) {
  const width = 800
  const height = 220
  const padding = { top: 20, right: 44, bottom: 32, left: 48 }
  const innerW = width - padding.left - padding.right
  const innerH = height - padding.top - padding.bottom

  const { series, yTicks, yAxisLabel } = useMemo(() => {
    if (type === 'cpu') {
      const values = data.map(d => d.cpu)
      const maxVal = Math.max(100, ...values)
      return {
        series: [{ values, color: CHART_COLORS.cpu, scale: maxVal }],
        yTicks: [0, 25, 50, 75, 100],
        yAxisLabel: '%',
      }
    }
    if (type === 'ram') {
      const values = data.map(d => d.ram)
      const maxVal = Math.max(100, ...values)
      return {
        series: [{ values, color: CHART_COLORS.ram, scale: maxVal }],
        yTicks: [0, 25, 50, 75, 100],
        yAxisLabel: '%',
      }
    }
    if (type === 'disk_io') {
      const values = data.map(d => d.disk_io)
      const maxVal = Math.max(1, ...values)
      const step = maxVal <= 4 ? 1 : Math.ceil(maxVal / 4)
      const ticks = [0, step, step * 2, step * 3, Math.ceil(maxVal)]
      return {
        series: [{ values, color: CHART_COLORS.disk_io, scale: maxVal }],
        yTicks: ticks,
        yAxisLabel: 'k',
      }
    }
    const gpu = data.map(d => (d.gpus?.[gpuIndex] ? d.gpus[gpuIndex].gpu_pct : d.gpu))
    const vram = data.map(d => (d.gpus?.[gpuIndex] ? d.gpus[gpuIndex].vram_pct : d.vram))
    const maxVal = Math.max(100, ...gpu, ...vram)
    return {
      series: [
        { values: gpu, color: CHART_COLORS.gpu, scale: maxVal },
        { values: vram, color: CHART_COLORS.vram, scale: maxVal },
      ],
      yTicks: [0, 25, 50, 75, 100],
      yAxisLabel: '%',
    }
  }, [data, type, gpuIndex])

  const n = data.length
  const xScale = (i: number) => (n <= 1 ? padding.left : padding.left + (i / Math.max(1, n - 1)) * innerW)
  const yScale = (val: number, maxV: number) =>
    padding.top + innerH - (maxV ? (val / maxV) * innerH : 0)

  const { areaPaths, linePaths } = useMemo(() => {
    const area: string[] = []
    const line: string[] = []
    series.forEach((s) => {
      if (!s.values.length) return
      const maxV = s.scale
      let linePath = `M ${xScale(0)} ${yScale(s.values[0], maxV)}`
      for (let i = 1; i < s.values.length; i++) {
        linePath += ` L ${xScale(i)} ${yScale(s.values[i], maxV)}`
      }
      line.push(linePath)
      area.push(linePath + ` L ${xScale(s.values.length - 1)} ${height - padding.bottom} L ${padding.left} ${height - padding.bottom} Z`)
    })
    return { areaPaths: area, linePaths: line }
  }, [series])

  const lastTs = data.length ? data[data.length - 1]?.ts : ''
  const timeLabel = lastTs ? new Date(lastTs).toLocaleString() : ''
  const keys = type === 'gpu' ? ['GPU', 'VRAM'] : [type === 'cpu' ? 'CPU' : type === 'ram' ? 'RAM' : 'Disk I/O']

  return (
    <div className="monitor-chart-wrap monitor-chart-wrap--normal">
      <div className="monitor-chart-legend">
        {series.map((s, k) => (
          <span key={k} className="monitor-legend-item">
            <span className="monitor-legend-dot" style={{ background: s.color }} />
            {keys[k]}
          </span>
        ))}
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="monitor-chart-svg monitor-chart-svg--normal">
        <defs>
          {series.map((s, k) => (
            <linearGradient key={k} id={`grad-${type}-${gpuIndex}-${k}`} x1="0" y1="1" x2="0" y2="0">
              <stop offset="0%" stopColor={s.color} stopOpacity={0.25} />
              <stop offset="100%" stopColor={s.color} stopOpacity={0.02} />
            </linearGradient>
          ))}
        </defs>
        {/* Y-axis labels */}
        {yTicks.map((tick, i) => (
          <text
            key={i}
            x={padding.left - 6}
            y={padding.top + innerH - (tick / (yTicks[yTicks.length - 1] || 1)) * innerH}
            textAnchor="end"
            dominantBaseline="middle"
            className="monitor-axis-y-label"
          >
            {tick}{yAxisLabel}
          </text>
        ))}
        {/* Grid lines */}
        {yTicks.slice(1, -1).map((_, i) => (
          <line
            key={i}
            x1={padding.left}
            y1={padding.top + (innerH * (i + 1)) / yTicks.length}
            x2={width - padding.right}
            y2={padding.top + (innerH * (i + 1)) / yTicks.length}
            className="monitor-grid-line monitor-grid-line--normal"
          />
        ))}
        {[1, 2, 3].map(i => (
          <line
            key={i}
            x1={padding.left + (innerW * i) / 4}
            y1={padding.top}
            x2={padding.left + (innerW * i) / 4}
            y2={height - padding.bottom}
            className="monitor-grid-line monitor-grid-line--normal"
          />
        ))}
        {areaPaths.map((d, k) => (
          <path key={`area-${k}`} d={d} fill={`url(#grad-${type}-${gpuIndex}-${k}-${gpuSeries ?? ''})`} className="monitor-area" />
        ))}
        {linePaths.map((d, k) => (
          <path key={`line-${k}`} d={d} fill="none" stroke={series[k].color} strokeWidth={2} className="monitor-line" />
        ))}
        <text x={width / 2} y={height - 6} textAnchor="middle" className="monitor-axis-label">
          {timeLabel}
        </text>
      </svg>
    </div>
  )
}
