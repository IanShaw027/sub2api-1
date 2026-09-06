import { afterEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'
import { initTheme, setAccent, setTheme } from '@/composables/useTheme'
import { alpha, baseChartOptions, chartTheme, cssVar, pieChartOptions, useChartTheme } from '../chartTheme'

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

  it('pieChartOptions drops the cartesian scales but keeps legend/tooltip', () => {
    const t = chartTheme()
    const o = pieChartOptions(t)
    expect('scales' in o).toBe(false)
    expect(o.plugins.legend.labels.color).toBe(t.text)
    expect(o.plugins.tooltip.backgroundColor).toBe(t.surface)
  })
})

describe('useChartTheme', () => {
  let style: HTMLStyleElement | undefined

  afterEach(() => {
    style?.remove()
    style = undefined
    localStorage.clear()
    initTheme()
  })

  it('updates the palette for accent-only changes as well as light/dark changes', async () => {
    style = document.createElement('style')
    style.textContent = `
      :root[data-accent="blue"] { --accent: #3b82f6; }
      :root[data-accent="teal"] { --accent: #0d9488; }
      :root[data-theme="glass-light"] { --foreground: #111111; }
      :root[data-theme="glass-dark"] { --foreground: #eeeeee; }
    `
    document.head.appendChild(style)
    initTheme()
    setTheme('light')
    setAccent('blue')
    const theme = useChartTheme()
    expect(theme.value.accent).toBe('#3b82f6')
    expect(theme.value.foreground).toBe('#111111')

    setAccent('teal')
    await nextTick()
    expect(document.documentElement.dataset.theme).toBe('glass-light')
    expect(chartTheme().accent).toBe('#0d9488')
    expect(theme.value.accent).toBe('#0d9488')
    expect(theme.value.series[0]).toBe('#0d9488')

    setTheme('dark')
    await nextTick()
    expect(theme.value.accent).toBe('#0d9488')
    expect(theme.value.foreground).toBe('#eeeeee')
    expect(theme.value.tooltip.titleColor).toBe('#eeeeee')
  })
})
