import React from 'react'

const DEG = Math.PI / 180
const START_ANGLE = 225 * DEG   // bottom-left
const TOTAL_ANGLE = 270 * DEG  // 3/4 circle

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
  value: number       // 0..100 (or 0..max for disk_io scaled)
  max?: number       // for disk_io use e.g. 10000
  label: string
  unit?: string      // '%' or 'k'
  className?: string
}

export function SpeedometerGauge({ value, max = 100, label, unit = '%', className }: SpeedometerGaugeProps) {
  const r = 48
  const cx = 56
  const cy = 56
  const stroke = 10
  const normalized = Math.min(1, Math.max(0, value / max))
  const endAngle = START_ANGLE - normalized * TOTAL_ANGLE
  const bgPath = describeArc(cx, cy, r, START_ANGLE, START_ANGLE - TOTAL_ANGLE)
  const valuePath = describeArc(cx, cy, r, START_ANGLE, endAngle)
  const displayVal = max === 100 ? Math.round(value) : value

  return (
    <div className={`speedometer-gauge ${className ?? ''}`}>
      <svg viewBox="0 0 112 80" className="speedometer-svg">
        <path d={bgPath} fill="none" stroke="var(--gauge-bg)" strokeWidth={stroke} strokeLinecap="round" />
        <path d={valuePath} fill="none" stroke="var(--gauge-fill)" strokeWidth={stroke} strokeLinecap="round" />
        <text x={cx} y={cy + 4} textAnchor="middle" className="speedometer-value">
          {displayVal}{unit}
        </text>
      </svg>
      <div className="speedometer-label">{label}</div>
    </div>
  )
}

interface DualBandGaugeProps {
  innerValue: number  // 0..100 (e.g. GPU)
  outerValue: number  // 0..100 (e.g. VRAM)
  innerLabel: string
  outerLabel: string
}

export function DualBandGauge({ innerValue, outerValue, innerLabel, outerLabel }: DualBandGaugeProps) {
  const r1 = 36
  const r2 = 48
  const cx = 56
  const cy = 56
  const stroke = 8
  const n1 = Math.min(1, Math.max(0, innerValue / 100))
  const n2 = Math.min(1, Math.max(0, outerValue / 100))
  const end1 = START_ANGLE - n1 * TOTAL_ANGLE
  const end2 = START_ANGLE - n2 * TOTAL_ANGLE
  const bg1 = describeArc(cx, cy, r1, START_ANGLE, START_ANGLE - TOTAL_ANGLE)
  const bg2 = describeArc(cx, cy, r2, START_ANGLE, START_ANGLE - TOTAL_ANGLE)
  const path1 = describeArc(cx, cy, r1, START_ANGLE, end1)
  const path2 = describeArc(cx, cy, r2, START_ANGLE, end2)

  return (
    <div className="speedometer-gauge speedometer-dual">
      <svg viewBox="0 0 112 80" className="speedometer-svg">
        <path d={bg2} fill="none" stroke="var(--gauge-bg)" strokeWidth={stroke} strokeLinecap="round" />
        <path d={path2} fill="none" stroke="var(--gauge-fill-outer)" strokeWidth={stroke} strokeLinecap="round" />
        <path d={bg1} fill="none" stroke="var(--gauge-bg)" strokeWidth={stroke} strokeLinecap="round" />
        <path d={path1} fill="none" stroke="var(--gauge-fill)" strokeWidth={stroke} strokeLinecap="round" />
        <text x={cx} y={cy - 8} textAnchor="middle" className="speedometer-value small">{Math.round(innerValue)}%</text>
        <text x={cx} y={cy + 10} textAnchor="middle" className="speedometer-value small">{Math.round(outerValue)}%</text>
      </svg>
      <div className="speedometer-labels">
        <span className="speedometer-legend-inner">{innerLabel}</span>
        <span className="speedometer-legend-outer">{outerLabel}</span>
      </div>
    </div>
  )
}
