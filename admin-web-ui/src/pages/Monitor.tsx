import { useState, useEffect, useMemo, useId } from 'react'
import { getMonitorMetrics, type MonitorMetricsResponse, type GPUMetrics } from '../api'
import { SpeedometerGauge } from '../components/SpeedometerGauge'

type MonitorType = 'system' | 'gpu'
type ChartMode = 'all' | 'separate'

const POLL_MS = 3000
const MAX_HISTORY = 60
const CHART_WIDTH = 800
const CHART_HEIGHT = 220

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
        {mode === 'gpu' && data && gpus.length > 0 && (
          <>
            {gpus.map((card, idx) => (
              <div key={idx} className="monitor-section-frame">
                <h3 className="monitor-section-title">{card.name}</h3>
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
          </>
        )}
      </div>
    </div>
  )
}

const CHART_COLORS = { cpu: '#22c55e', ram: '#3b82f6', gpu: '#06b6d4', vram: '#eab308' }

function MonitorTimeChart({
  data,
  type,
  gpuIndex = 0,
  gpuSeries,
  width = CHART_WIDTH,
  height = CHART_HEIGHT,
}: {
  data: MonitorMetricsResponse['history']
  type: 'cpu' | 'ram' | 'disk_io' | 'system' | 'gpu'
  gpuIndex?: number
  gpuSeries?: 'gpu' | 'vram'
  width?: number
  height?: number
}) {
  const gradPrefix = useId().replace(/:/g, '-')
  const [hoveredPointIndex, setHoveredPointIndex] = useState<number | null>(null)
  const padding = { top: 20, right: 44, bottom: 32, left: 48 }
  const innerW = width - padding.left - padding.right
  const innerH = height - padding.top - padding.bottom

  const { series, yTicks, yAxisLabel } = useMemo(() => {
    if (type === 'system') {
      const cpu = data.map(d => d.cpu)
      const ram = data.map(d => d.ram)
      const maxVal = 100
      return {
        series: [
          { values: cpu, color: CHART_COLORS.cpu, scale: maxVal },
          { values: ram, color: CHART_COLORS.ram, scale: maxVal },
        ],
        yTicks: [0, 25, 50, 75, 100],
        yAxisLabel: '%',
      }
    }
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
    const gpu = data.map(d => (d.gpus?.[gpuIndex] ? d.gpus[gpuIndex].gpu_pct : d.gpu))
    const vram = data.map(d => (d.gpus?.[gpuIndex] ? d.gpus[gpuIndex].vram_pct : d.vram))
    const maxVal = Math.max(100, ...gpu, ...vram)
    if (gpuSeries === 'gpu') {
      return {
        series: [{ values: gpu, color: CHART_COLORS.gpu, scale: maxVal }],
        yTicks: [0, 25, 50, 75, 100],
        yAxisLabel: '%',
      }
    }
    if (gpuSeries === 'vram') {
      return {
        series: [{ values: vram, color: CHART_COLORS.vram, scale: maxVal }],
        yTicks: [0, 25, 50, 75, 100],
        yAxisLabel: '%',
      }
    }
    return {
      series: [
        { values: gpu, color: CHART_COLORS.gpu, scale: maxVal },
        { values: vram, color: CHART_COLORS.vram, scale: maxVal },
      ],
      yTicks: [0, 25, 50, 75, 100],
      yAxisLabel: '%',
    }
  }, [data, type, gpuIndex, gpuSeries])

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
  const keys =
    type === 'system'
      ? ['CPU', 'RAM']
      : type === 'gpu'
        ? gpuSeries === 'gpu'
          ? ['GPU']
          : gpuSeries === 'vram'
            ? ['VRAM']
            : ['GPU', 'VRAM']
        : [type === 'cpu' ? 'CPU' : 'RAM']

  const pointIndices = useMemo(() => {
    const indices: number[] = []
    for (let i = 0; i < n; i += 10) indices.push(i) // точка каждые 30 с при POLL_MS=3000
    if (n > 0 && indices[indices.length - 1] !== n - 1) indices.push(n - 1)
    return indices
  }, [n])

  return (
    <div className="monitor-chart-wrap monitor-chart-wrap--normal monitor-chart-wrap--compact">
      <div className="monitor-chart-legend">
        {series.map((s, k) => (
          <span key={k} className="monitor-legend-item">
            <span className="monitor-legend-dot" style={{ background: s.color }} />
            {keys[k]}
          </span>
        ))}
      </div>
      <svg viewBox={`0 0 ${width} ${height}`} className="monitor-chart-svg monitor-chart-svg--normal" style={{ maxWidth: width }}>
        <defs>
          {series.map((s, k) => (
            <linearGradient key={k} id={`${gradPrefix}-grad-${k}`} x1="0" y1="1" x2="0" y2="0">
              <stop offset="0%" stopColor={s.color} stopOpacity={0.25} />
              <stop offset="100%" stopColor={s.color} stopOpacity={0.02} />
            </linearGradient>
          ))}
        </defs>
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
        {yTicks.slice(1, -1).map((tick, i) => {
          const maxTick = yTicks[yTicks.length - 1] || 1
          const y = padding.top + innerH - (tick / maxTick) * innerH
          return (
            <line
              key={i}
              x1={padding.left}
              y1={y}
              x2={width - padding.right}
              y2={y}
              className="monitor-grid-line monitor-grid-line--normal"
            />
          )
        })}
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
          <path key={`area-${k}`} d={d} fill={`url(#${gradPrefix}-grad-${k})`} className="monitor-area" />
        ))}
        {linePaths.map((d, k) => (
          <path key={`line-${k}`} d={d} fill="none" stroke={series[k].color} strokeWidth={2} className="monitor-line" />
        ))}
        {series.map((s, k) =>
          pointIndices.map((i) => {
            if (i >= s.values.length) return null
            const x = xScale(i)
            const y = yScale(s.values[i], s.scale)
            return (
              <circle
                key={`point-${k}-${i}`}
                cx={x}
                cy={y}
                r={5}
                fill={s.color}
                className="monitor-point"
                onMouseEnter={() => setHoveredPointIndex(i)}
                onMouseLeave={() => setHoveredPointIndex(null)}
              />
            )
          })
        )}
        {hoveredPointIndex != null && data[hoveredPointIndex] && (
          <g
            className="monitor-tooltip"
            onMouseEnter={() => setHoveredPointIndex(hoveredPointIndex)}
            onMouseLeave={() => setHoveredPointIndex(null)}
          >
            <rect
              x={Math.min(xScale(hoveredPointIndex) + 12, width - 280)}
              y={padding.top}
              width={268}
              height={34 + series.length * 16}
              rx={4}
              fill="rgba(30,30,40,0.95)"
              stroke="rgba(255,255,255,0.2)"
              strokeWidth={1}
            />
            {series.map((s, k) => (
              <text
                key={k}
                x={Math.min(xScale(hoveredPointIndex) + 20, width - 272)}
                y={padding.top + 16 + k * 16}
                fill="#fff"
                fontSize={12}
              >
                {type === 'cpu' ? 'CPU usage' : keys[k]}: {s.values[hoveredPointIndex] != null ? s.values[hoveredPointIndex].toFixed(2) : '—'} %
              </text>
            ))}
            <text
              x={Math.min(xScale(hoveredPointIndex) + 20, width - 272)}
              y={padding.top + 16 + series.length * 16}
              fill="rgba(255,255,255,0.8)"
              fontSize={11}
            >
              {new Date(data[hoveredPointIndex].ts).toString()}
            </text>
          </g>
        )}
        <text x={width / 2} y={height - 6} textAnchor="middle" className="monitor-axis-label">
          {timeLabel}
        </text>
      </svg>
    </div>
  )
}
