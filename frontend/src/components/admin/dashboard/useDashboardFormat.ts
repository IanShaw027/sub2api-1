/** Pure formatting + sparkline geometry helpers shared by the admin dashboard view and its panels. */
import type { SparkPath } from './types'

const SPARK_W = 100
const SPARK_H = 28

/** Build the prototype's 100×28 area + line sparkline geometry for a series. */
export const buildSpark = (values: number[]): SparkPath => {
  const points = values.length >= 2 ? values : [0, 0]
  const min = Math.min(...points)
  const max = Math.max(...points)
  const span = max - min
  const coords = points.map((value, index) => {
    const ratio = span > 0 ? (value - min) / span : 0.5
    const norm = 0.18 + 0.82 * ratio
    return [(index * SPARK_W) / (points.length - 1), SPARK_H - norm * (SPARK_H - 4)] as const
  })
  const line = `M${coords.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' L')}`
  return { line, area: `${line} L${SPARK_W},${SPARK_H} L0,${SPARK_H} Z` }
}

export const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

export const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

export const formatNumber = (value: number | null | undefined): string => {
  return toFiniteNumber(value).toLocaleString()
}

export const formatCost = (value: number | null | undefined): string => {
  const safeValue = toFiniteNumber(value)
  if (safeValue >= 1000) {
    return (safeValue / 1000).toFixed(2) + 'K'
  } else if (safeValue >= 1) {
    return safeValue.toFixed(2)
  } else if (safeValue >= 0.01) {
    return safeValue.toFixed(3)
  }
  return safeValue.toFixed(4)
}

export const asNumber = (value: number | null | undefined): number => {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0
}

export const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

export const formatEventTime = (value: string): string => {
  if (!value) return '--:--'
  const parsed = new Date(value)
  if (Number.isNaN(parsed.getTime())) return '--:--'
  return `${String(parsed.getHours()).padStart(2, '0')}:${String(parsed.getMinutes()).padStart(2, '0')}`
}
