import { useState, useEffect, useMemo } from 'react'
import { getMonitorMetrics, type MonitorMetricsResponse, type GPUMetrics, type ContainerMetrics } from '../api'
import { SpeedometerGauge } from '../components/SpeedometerGauge'

type MonitorType = 'system' | 'gpu'
type ChartMode = 'all' | 'separate'

const POLL_MS = 3000
const MAX_HISTORY = 60
const CHART_WIDTH = 700
const CHART_HEIGHT = 220
/** Интервал между точками на графике (секунды). */
const POINT_INTERVAL_MS = 30000

function formatTooltipTime(ts: string): string {
  const s = new Date(ts).toString()
  return s.replace(/\s*\([^)]*\)$/, '').trim()
}

type ChartSeriesEntry = { values: number[]; color: string; scale: number }
type ChartSeries = Record<string, ChartSeriesEntry | undefined>

function formatUptime(sec: number): string {
  if (!sec || sec <= 0) return '—'
  const d = Math.floor(sec / 86400)
  const h = Math.floor((sec % 86400) / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const parts: string[] = []
  if (d > 0) parts.push(`${d}д`)
  if (h > 0) parts.push(`${h}ч`)
  parts.push(`${m}мин`)
  return parts.join(' ')
}

export function Monitor() {
  const [mode, setMode] = useState<MonitorType>('system')
  const [data, setData] = useState<MonitorMetricsResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [chartModeSystem, setChartModeSystem] = useState<ChartMode>('all')
  const [chartModeGpu, setChartModeGpu] = useState<ChartMode>('all')

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
        <div className="page-header-row">
          <div className="monitor-type-toggle">
            <button
              type="button"
              className={mode === 'system' ? 'btn-primary' : 'btn-monitor-inactive'}
              onClick={() => setMode('system')}
            >
              CPU, RAM
            </button>
            <button
              type="button"
              className={mode === 'gpu' ? 'btn-primary' : 'btn-monitor-inactive'}
              onClick={() => setMode('gpu')}
            >
              GPU, VRAM
            </button>
          </div>
          <div className="monitor-uptime">Uptime: {formatUptime(data?.uptime_sec ?? 0)}</div>
        </div>
      </div>
      <div className="content-panel">
        {error && <p className="text-error">{error}</p>}
        <div className="monitor-cards-row">
          {mode === 'system' && data && (
            <div className="monitor-section-frame">
              <h3 className="monitor-section-title">Система</h3>
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
              </div>
              <div className="monitor-chart-mode-toggle">
                <button
                  type="button"
                  className={chartModeSystem === 'all' ? 'btn-primary' : 'btn-monitor-inactive'}
                  onClick={() => setChartModeSystem('all')}
                >
                  Всё на одном
                </button>
                <button
                  type="button"
                  className={chartModeSystem === 'separate' ? 'btn-primary' : 'btn-monitor-inactive'}
                  onClick={() => setChartModeSystem('separate')}
                >
                  По отдельности
                </button>
              </div>
              <div className="monitor-chart-block">
                {chartModeSystem === 'all' ? (
                  <MonitorTimeChart data={chartData} type="system" width={CHART_WIDTH} height={CHART_HEIGHT} />
                ) : (
                  <>
                    <div className="monitor-chart-panel monitor-chart-panel--compact">
                      <h4 className="monitor-chart-title">CPU</h4>
                      <MonitorTimeChart data={chartData} type="cpu" width={CHART_WIDTH} height={CHART_HEIGHT} />
                    </div>
                    <div className="monitor-chart-panel monitor-chart-panel--compact">
                      <h4 className="monitor-chart-title">RAM</h4>
                      <MonitorTimeChart data={chartData} type="ram" width={CHART_WIDTH} height={CHART_HEIGHT} />
                    </div>
                  </>
                )}
              </div>
            </div>
          )}
          {mode === 'system' && data?.containers && data.containers.length > 0 && data.containers.map((c: ContainerMetrics, idx: number) => (
            <div key={`container-${idx}-${c.name}`} className="monitor-section-frame monitor-section-frame--container">
              <h3 className="monitor-section-title">{c.name}</h3>
              <div className="monitor-gauges">
                <SpeedometerGauge value={c.cpu_pct} label="CPU" unit="%" />
                <SpeedometerGauge
                  value={c.ram_pct}
                  label="RAM"
                  unit="%"
                  valueLabel={
                    c.ram_used_gb != null && c.ram_limit_gb != null && c.ram_limit_gb > 0
                      ? `${c.ram_used_gb.toFixed(2)} / ${c.ram_limit_gb.toFixed(1)} GB`
                      : c.ram_used_gb != null
                        ? `${c.ram_used_gb.toFixed(2)} GB`
                        : undefined
                  }
                />
              </div>
            </div>
          ))}
          {mode === 'gpu' && data && gpus.length > 0 && gpus.map((card: GPUMetrics, idx: number) => (
            <div key={idx} className="monitor-section-frame">
              <h3 className="monitor-section-title">{card.name?.trim() || `GPU ${idx}`}</h3>
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
              <div className="monitor-chart-mode-toggle">
                <button
                  type="button"
                  className={chartModeGpu === 'all' ? 'btn-primary' : 'btn-monitor-inactive'}
                  onClick={() => setChartModeGpu('all')}
                >
                  Всё на одном
                </button>
                <button
                  type="button"
                  className={chartModeGpu === 'separate' ? 'btn-primary' : 'btn-monitor-inactive'}
                  onClick={() => setChartModeGpu('separate')}
                >
                  По отдельности
                </button>
              </div>
              <div className="monitor-chart-block">
                {chartModeGpu === 'all' ? (
                  <MonitorTimeChart data={chartData} type="gpu" gpuIndex={idx} width={CHART_WIDTH} height={CHART_HEIGHT} />
                ) : (
                  <>
                    <div className="monitor-chart-panel monitor-chart-panel--compact">
                      <h4 className="monitor-chart-title">GPU</h4>
                      <MonitorTimeChart data={chartData} type="gpu" gpuIndex={idx} gpuSeries="gpu" width={CHART_WIDTH} height={CHART_HEIGHT} />
                    </div>
                    <div className="monitor-chart-panel monitor-chart-panel--compact">
                      <h4 className="monitor-chart-title">VRAM</h4>
                      <MonitorTimeChart data={chartData} type="gpu" gpuIndex={idx} gpuSeries="vram" width={CHART_WIDTH} height={CHART_HEIGHT} />
                    </div>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

const CHART_COLORS = { cpu: '#22c55e', ram: '#3b82f6', disk_io: '#3b82f6', gpu: '#06b6d4', vram: '#eab308' }

type TooltipState = { key: string; index: number; value: number; ts: string; x: number; y: number } | null

function MonitorTimeChart({
  data,
  type,
  gpuIndex = 0,
  gpuSeries,
  width = 800,
  height = 220,
}: {
  data: MonitorMetricsResponse['history']
  type: 'system' | 'cpu' | 'ram' | 'gpu'
  gpuIndex?: number
  gpuSeries?: 'gpu' | 'vram'
  width?: number
  height?: number
}) {
  const [tooltip, setTooltip] = useState<TooltipState>(null)
  const padding = { top: 20, right: 50, bottom: 32, left: 44 }
  const innerW = width - padding.left - padding.right
  const innerH = height - padding.top - padding.bottom

  const pointIndices = useMemo(() => {
    if (!data.length) return []
    const out: number[] = [0]
    let lastT = new Date(data[0].ts).getTime()
    data.forEach((d, i) => {
      const t = new Date(d.ts).getTime()
      if (i > 0 && t - lastT >= POINT_INTERVAL_MS) {
        out.push(i)
        lastT = t
      }
    })
    return out
  }, [data])

  const series = useMemo(() => {
    if (type === 'system') {
      const cpu = data.map(d => d.cpu)
      const ram = data.map(d => d.ram)
      const maxVal = Math.max(100, ...cpu, ...ram)
      return {
        cpu: { values: cpu, color: CHART_COLORS.cpu, scale: maxVal },
        ram: { values: ram, color: CHART_COLORS.ram, scale: maxVal },
      }
    }
    if (type === 'cpu') {
      const cpu = data.map(d => d.cpu)
      return { cpu: { values: cpu, color: CHART_COLORS.cpu, scale: Math.max(100, ...cpu) } }
    }
    if (type === 'ram') {
      const ram = data.map(d => d.ram)
      return { ram: { values: ram, color: CHART_COLORS.ram, scale: Math.max(100, ...ram) } }
    }
    const gpu = data.map(d => (d.gpus?.[gpuIndex] ? d.gpus[gpuIndex].gpu_pct : d.gpu))
    const vram = data.map(d => (d.gpus?.[gpuIndex] ? d.gpus[gpuIndex].vram_pct : d.vram))
    const maxVal = Math.max(100, ...gpu, ...vram)
    if (gpuSeries === 'gpu') {
      return { gpu: { values: gpu, color: CHART_COLORS.gpu, scale: maxVal } }
    }
    if (gpuSeries === 'vram') {
      return { vram: { values: vram, color: CHART_COLORS.vram, scale: maxVal } }
    }
    return {
      gpu: { values: gpu, color: CHART_COLORS.gpu, scale: maxVal },
      vram: { values: vram, color: CHART_COLORS.vram, scale: maxVal },
    }
  }, [data, type, gpuIndex, gpuSeries])

  const keys: string[] = type === 'system' ? ['cpu', 'ram'] : type === 'cpu' ? ['cpu'] : type === 'ram' ? ['ram'] : gpuSeries ? [gpuSeries] : ['gpu', 'vram']
  const seriesMap = series as ChartSeries
  const n = data.length
  const xScale = (i: number) => (n <= 1 ? padding.left : padding.left + (i / Math.max(1, n - 1)) * innerW)
  const yScale = (val: number, maxV: number) =>
    padding.top + innerH - (maxV ? (val / maxV) * innerH : 0)

  const paths = useMemo(() => {
    const out: Record<string, string> = {}
    keys.forEach(key => {
      const s = seriesMap[key]
      if (!s || !s.values.length) return
      const maxV = s.scale
      let linePath = `M ${xScale(0)} ${yScale(s.values[0], maxV)}`
      for (let i = 1; i < s.values.length; i++) {
        linePath += ` L ${xScale(i)} ${yScale(s.values[i], maxV)}`
      }
      out[key] = linePath
    })
    return out
  }, [series, keys, n])

  const labels: Record<string, string> = { cpu: 'CPU', ram: 'RAM', disk_io: 'Disk I/O', gpu: 'GPU', vram: 'VRAM' }
  const lastTs = data.length ? data[data.length - 1]?.ts : ''
  const timeLabel = lastTs ? new Date(lastTs).toLocaleString() : ''
  const gradPrefix = `monitor-grad-${type}-${gpuIndex}`

  return (
    <div className="monitor-chart-wrap monitor-chart-wrap--normal monitor-chart-wrap--compact">
      {tooltip && (
        <div
          className="monitor-chart-tooltip"
          style={{ left: tooltip.x + 12, top: tooltip.y - 8 }}
          role="tooltip"
        >
          <div className="monitor-chart-tooltip-metric">
            {labels[tooltip.key]} usage: {tooltip.value.toFixed(2)} %
          </div>
          <div className="monitor-chart-tooltip-time">
            {formatTooltipTime(tooltip.ts)}
          </div>
        </div>
      )}
      <div className="monitor-chart-legend">
        {keys.map(k => (
          <span key={k} className="monitor-legend-item">
            <span className="monitor-legend-dot" style={{ background: seriesMap[k]?.color }} />
            {labels[k]}
          </span>
        ))}
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="monitor-chart-svg monitor-chart-svg--normal" style={{ width: '100%', height: 'auto', minHeight: 180 }} preserveAspectRatio="xMidYMid meet">
        <defs>
          {keys.map(k => (
            <linearGradient key={k} id={`${gradPrefix}-grad-${k}`} x1="0" y1="1" x2="0" y2="0">
              <stop offset="0%" stopColor={seriesMap[k]?.color} stopOpacity={0.4} />
              <stop offset="100%" stopColor={seriesMap[k]?.color} stopOpacity={0.05} />
            </linearGradient>
          ))}
        </defs>
        <line x1={padding.left} y1={padding.top} x2={padding.left} y2={height - padding.bottom} className="monitor-grid-line monitor-grid-line--normal" />
        <line x1={padding.left} y1={height - padding.bottom} x2={width - padding.right} y2={height - padding.bottom} className="monitor-grid-line monitor-grid-line--normal" />
        {[1, 2, 3, 4].map(i => (
          <line key={i} x1={padding.left} y1={padding.top + (innerH * i) / 4} x2={width - padding.right} y2={padding.top + (innerH * i) / 4} className="monitor-grid-line monitor-grid-line--normal" />
        ))}
        {[1, 2, 3].map(i => (
          <line key={i} x1={padding.left + (innerW * i) / 3} y1={padding.top} x2={padding.left + (innerW * i) / 3} y2={height - padding.bottom} className="monitor-grid-line monitor-grid-line--normal" />
        ))}
        {/* Сначала все заливки и линии, чтобы порядок серий не перекрывал точки */}
        {keys.map(k => {
          const d = paths[k]
          const ent = seriesMap[k]
          if (!d || !ent?.values.length) return null
          const areaPath = d + ` L ${xScale(ent.values.length - 1)} ${height - padding.bottom} L ${padding.left} ${height - padding.bottom} Z`
          return (
            <g key={`area-line-${k}`}>
              <path d={areaPath} fill={`url(#${gradPrefix}-grad-${k})`} className="monitor-area" />
              <path d={d} fill="none" stroke={ent.color} strokeWidth={2} className="monitor-line" />
            </g>
          )
        })}
        {/* Точки поверх всех линий — все серии наводятся */}
        {keys.map(k => {
          const ent = seriesMap[k]
          if (!ent?.values.length) return null
          const maxV = ent.scale
          return pointIndices.filter(i => i < ent.values.length).map(i => {
            const val = ent.values[i]
            const ts = data[i]?.ts ?? ''
            return (
              <circle
                key={`point-${k}-${i}`}
                cx={xScale(i)}
                cy={yScale(val, maxV)}
                r={4}
                fill={ent.color}
                className="monitor-point"
                onMouseEnter={e => setTooltip({ key: k, index: i, value: val, ts, x: e.clientX, y: e.clientY })}
                onMouseLeave={() => setTooltip(null)}
              />
            )
          })
        })}
        <text x={width / 2} y={height - 6} textAnchor="middle" className="monitor-axis-label">
          {timeLabel}
        </text>
      </svg>
    </div>
  )
}
