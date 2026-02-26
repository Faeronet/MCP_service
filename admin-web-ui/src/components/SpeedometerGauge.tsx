const DEG = Math.PI / 180
const START_ANGLE = 225 * DEG
const TOTAL_ANGLE = 270 * DEG

function polar(cx: number, cy: number, r: number, angle: number) {
  return { x: cx + r * Math.cos(angle), y: cy - r * Math.sin(angle) }
}

function describeArc(cx: number, cy: number, r: number, startRad: number, endRad: number) {
  const s = polar(cx, cy, r, startRad)
  const e = polar(cx, cy, r, endRad)
  const large = Math.abs(endRad - startRad) > Math.PI ? 1 : 0
  return `M ${s.x} ${s.y} A ${r} ${r} 0 ${large} 1 ${e.x} ${e.y}`
}

interface SpeedometerGaugeProps {
  value: number
  max?: number
  label: string
  unit?: string
  className?: string
  /** Цвет дуги нагрузки (по умолчанию cyan). Для VRAM — жёлтый. */
  fillColor?: string
}

export function SpeedometerGauge({ value, max = 100, label, unit = '%', className, fillColor }: SpeedometerGaugeProps) {
  const r = 28
  const cx = 32
  const cy = 32
  const stroke = 5
  const normalized = Math.min(1, Math.max(0, value / max))
  const endAngle = START_ANGLE - normalized * TOTAL_ANGLE
  const bgPath = describeArc(cx, cy, r, START_ANGLE, START_ANGLE - TOTAL_ANGLE)
  const valuePath = describeArc(cx, cy, r, START_ANGLE, endAngle)
  const displayVal = max === 100 ? Math.round(value) : value
  const strokeColor = fillColor ?? 'var(--gauge-fill)'

  return (
    <div className={`speedometer-gauge ${className ?? ''}`}>
      <svg viewBox="0 0 64 52" className="speedometer-svg">
        <path d={bgPath} fill="none" stroke="var(--gauge-bg)" strokeWidth={stroke} strokeLinecap="round" />
        <path d={valuePath} fill="none" stroke={strokeColor} strokeWidth={stroke} strokeLinecap="round" />
        <text x={cx} y={cy + 2} textAnchor="middle" className="speedometer-value">
          {displayVal}{unit}
        </text>
      </svg>
      <div className="speedometer-label">{label}</div>
    </div>
  )
}
