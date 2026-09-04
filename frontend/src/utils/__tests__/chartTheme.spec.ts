import { describe, expect, it, vi } from 'vitest'

vi.mock('@/composables/useTheme', () => ({ useTheme: () => ({ isDark: { value: false } }) }))

import { alpha, baseChartOptions, chartTheme, cssVar } from '../chartTheme'

describe('chartTheme', () => {
  it('alpha handles hex and oklch inputs', () => {
    expect(alpha('#3b82f6', 50)).toBe('#3b82f680')
    expect(alpha('#abc', 100)).toBe('#aabbccff')
    expect(alpha('oklch(62% 0.18 260)', 16)).toBe('color-mix(in oklch, oklch(62% 0.18 260) 16%, transparent)')
  })

  it('falls back to defaults when tokens are not defined', () => {
    expect(cssVar('--definitely-missing', '#123456')).toBe('#123456')
    const t = chartTheme()
    expect(t.accent).toBeTruthy()
    expect(t.series[0]).toBe(t.accent)
    expect(t.series.length).toBeGreaterThanOrEqual(6)
    expect(t.tooltip.titleColor).toBe(t.foreground)
  })

  it('baseChartOptions wires token colours into scales and legend', () => {
    const t = chartTheme()
    const o = baseChartOptions(t)
    expect(o.scales.y.grid.color).toBe(t.grid)
    expect(o.scales.x.ticks.color).toBe(t.text)
    expect(o.plugins.legend.labels.color).toBe(t.text)
    expect(o.plugins.tooltip.backgroundColor).toBe(t.surface)
  })
})
