/**
 * chartTheme() — single source of chart.js colours for the Glass UI.
 *
 * Every chart instance must take its colours from here instead of literal
 * hex values so that light/dark themes and the accent switcher apply to
 * canvases too (design.md: "chart.js 图表只对其做令牌化样式约束").
 *
 * Canvas cannot resolve `var(--x)` or `color-mix()` on its own, so the
 * helper reads the computed token values off `<html>` and returns concrete
 * strings. Use `alpha()` for fills; it emits `color-mix()` which modern
 * Chart.js/Canvas accepts, and falls back to hex + alpha when the token
 * value is a hex literal.
 */
import { computed, type ComputedRef } from 'vue'
import { useTheme } from '@/composables/useTheme'

export interface ChartTheme {
  /** axis tick / legend text */
  text: string
  /** strong text (titles, tooltip title) */
  foreground: string
  /** grid lines */
  grid: string
  /** card / tooltip background */
  surface: string
  border: string
  accent: string
  success: string
  warning: string
  danger: string
  info: string
  /** ordered series palette (accent first) */
  series: string[]
  font: { family: string; mono: string; size: number }
  tooltip: {
    backgroundColor: string
    borderColor: string
    borderWidth: number
    titleColor: string
    bodyColor: string
    cornerRadius: number
    padding: number
  }
}

const FALLBACK: Record<string, string> = {
  '--foreground': '#1f2430',
  '--muted': '#6b7280',
  '--border': '#e5e7eb',
  '--surface': '#ffffff',
  '--accent': '#3b82f6',
  '--success': '#10b981',
  '--warning': '#f59e0b',
  '--danger': '#ef4444',
  '--info': '#3b82f6',
  '--font-body': 'Inter, system-ui, sans-serif',
  '--font-mono': '"JetBrains Mono", ui-monospace, monospace'
}

export function cssVar(name: string, fallback = FALLBACK[name] ?? ''): string {
  if (typeof window === 'undefined' || typeof getComputedStyle !== 'function') return fallback
  const value = getComputedStyle(document.documentElement).getPropertyValue(name).trim()
  return value || fallback
}

/** Translucent version of a token colour usable by canvas. */
export function alpha(color: string, pct: number): string {
  const p = Math.max(0, Math.min(100, pct))
  if (/^#[0-9a-f]{6}$/i.test(color)) {
    return color + Math.round((p / 100) * 255).toString(16).padStart(2, '0')
  }
  if (/^#[0-9a-f]{3}$/i.test(color)) {
    const [r, g, b] = color.slice(1).split('')
    return `#${r}${r}${g}${g}${b}${b}` + Math.round((p / 100) * 255).toString(16).padStart(2, '0')
  }
  return `color-mix(in oklch, ${color} ${p}%, transparent)`
}

/** Hue-shifted companions of the accent so multi-series charts stay on-brand. */
function seriesPalette(accent: string, success: string, warning: string, danger: string): string[] {
  const shifted = (deg: number) => {
    const m = accent.match(/^oklch\(\s*([\d.]+%?)\s+([\d.]+)\s+([\d.]+)/i)
    if (!m) return accent
    const hue = (parseFloat(m[3]) + deg + 360) % 360
    return `oklch(${m[1]} ${m[2]} ${hue.toFixed(2)})`
  }
  return [accent, success, warning, shifted(-60), danger, shifted(120), shifted(60), shifted(180)]
}

/** Snapshot of the current theme tokens (non-reactive). */
export function chartTheme(): ChartTheme {
  const text = cssVar('--muted')
  const foreground = cssVar('--foreground')
  const grid = cssVar('--border')
  const surface = cssVar('--surface')
  const accent = cssVar('--accent')
  const success = cssVar('--success')
  const warning = cssVar('--warning')
  const danger = cssVar('--danger')
  const info = cssVar('--info') === 'var(--accent)' ? accent : cssVar('--info')
  return {
    text,
    foreground,
    grid: alpha(grid, 70),
    surface,
    border: grid,
    accent,
    success,
    warning,
    danger,
    info,
    series: seriesPalette(accent, success, warning, danger),
    font: { family: cssVar('--font-body'), mono: cssVar('--font-mono'), size: 11 },
    tooltip: {
      backgroundColor: surface,
      borderColor: grid,
      borderWidth: 1,
      titleColor: foreground,
      bodyColor: text,
      cornerRadius: 10,
      padding: 10
    }
  }
}

/**
 * Reactive variant: recomputes when the theme toggles. Use inside components:
 *   const theme = useChartTheme()
 *   const options = computed(() => ({ scales: { x: { ticks: { color: theme.value.text } } } }))
 */
export function useChartTheme(): ComputedRef<ChartTheme> {
  const { isDark, accent } = useTheme()
  return computed(() => {
    void isDark.value
    void accent.value
    return chartTheme()
  })
}

/** Common scale + legend + tooltip options; spread into a chart's `options`. */
export function baseChartOptions(theme: ChartTheme) {
  const ticks = { color: theme.text, font: { family: theme.font.family, size: theme.font.size } }
  return {
    responsive: true,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        labels: { color: theme.text, boxWidth: 10, boxHeight: 10, usePointStyle: true, font: ticks.font }
      },
      tooltip: { ...theme.tooltip, titleFont: { family: theme.font.family }, bodyFont: { family: theme.font.mono } }
    },
    scales: {
      x: { grid: { display: false }, border: { color: theme.border }, ticks },
      y: { grid: { color: theme.grid }, border: { display: false }, ticks }
    }
  }
}

/** Legend + tooltip options for doughnut / pie charts (no cartesian scales). */
export function pieChartOptions(theme: ChartTheme) {
  const { scales: _scales, ...rest } = baseChartOptions(theme)
  return rest
}
