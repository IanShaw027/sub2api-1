/**
 * Structure contracts: channel-monitor-v2 + studio shells must use project
 * design-system utility classes rather than isolated flat RGB skins.
 */
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '../../..')

function read(rel: string) {
  return readFileSync(resolve(root, rel), 'utf8')
}

describe('channel-monitor-v2 design system structure', () => {
  it('user ChannelStatus V2 shell inlines a DashboardView-style hero + StatCard KPI row and delegates the rest to composable-driven subcomponents', () => {
    // Route wrapper may switch V1/V2; design chrome lives on the V2 implementation.
    // Per glass-ui-redesign coordination: DashboardPageLayout has zero page
    // consumers, so this view inlines the same hero recipe already shipped in
    // views/admin/DashboardView.vue and views/user/DashboardView.vue instead of
    // introducing that shared layout component.
    const src = read('views/user/ChannelStatusV2View.vue')
    expect(src).toContain('class="glass-card dash-hero"')
    expect(src).toContain('dash-hero-title')
    expect(src).toContain('dash-refresh-btn')
    // StatCard-based KPI row (top of page, ahead of filters/trend/tables)
    expect(src).toContain('<StatCard')
    expect(src).toContain('monitor-stat-grid')
    // Business logic lives in the composable; the view only wires it up
    expect(src).toContain('useChannelMonitorV2()')
    expect(src).toContain('trendView')
    // Filters, trend viz, and tab tables are extracted feature subcomponents
    expect(src).toContain('<MonitorToolbar')
    expect(src).toContain('<MonitorTrendChart')
    expect(src).toContain('<RelayPulseMatrix')
    expect(src).toContain('<MonitorDataTabs')
    // Overview-first KPI strip before primary viz
    expect(src.indexOf('summaryAria')).toBeLessThan(src.indexOf('MonitorTrendChart'))
    // No page-level fixed min-width that forces viewport horizontal scroll
    expect(src).not.toMatch(/min-width:\s*980px/)
    expect(src).not.toMatch(/min-w-\[980px\]/)
  })

  it('MonitorToolbar owns the compact single-row filter chrome', () => {
    const src = read('features/channel-monitor-v2/MonitorToolbar.vue')
    expect(src).toContain('filter-row')
    expect(src).toContain('clearFilters')
    expect(src).toContain('badge badge-warning')
  })

  it('MonitorDataTabs owns the models/errors/users tab tables', () => {
    const src = read('features/channel-monitor-v2/MonitorDataTabs.vue')
    expect(src).toContain('SegmentedControl')
    expect(src).toMatch(/max-h-\[min\(52vh/)
    expect(src).toContain('overflow-auto')
    expect(src).toContain('tabular-nums')
  })

  it('RelayPulseMatrix uses card chrome, matrix scroll, and hover tooltips (no click modal)', () => {
    const src = read('features/channel-monitor-v2/RelayPulseMatrix.vue')
    expect(src).toContain('class="glass-card')
    expect(src).toContain('glass-card-header')
    expect(src).toContain('glass-card-body')
    expect(src).toContain('matrix-scroll')
    expect(src).toMatch(/max-h-\[min\(42vh/)
    expect(src).toContain('overflow-auto')
    expect(src).toContain('pulse-tooltip')
    expect(src).toContain('min-h-[360px]')
    expect(src).not.toContain('modal-overlay')
    expect(src).not.toContain('modal-content')
  })

  it('MetricCell uses summary-chip utility', () => {
    const src = read('features/channel-monitor-v2/MetricCell.vue')
    expect(src).toContain('summary-chip')
    expect(src).toContain('summary-chip-label')
    expect(src).toContain('summary-chip-value')
    expect(src).toContain('summary-chip-dot')
  })

  it('MonitorTrendChart uses Ops chart shell tokens', () => {
    const src = read('features/channel-monitor-v2/MonitorTrendChart.vue')
    expect(src).toContain('class="glass-card')
    expect(src).toContain('glass-card-header')
    expect(src).toContain('EmptyState')
    expect(src).toContain('min-h-[360px]')
  })

  it('FilterMultiSelect uses rounded-xl input chrome and dropdown utility', () => {
    const src = read('features/channel-monitor-v2/FilterMultiSelect.vue')
    expect(src).toContain('rounded-xl')
    expect(src).toContain('dropdown')
    expect(src).toContain('dropdown-item')
  })

  it('MonitorSettingsPanel uses page-header, card, btn-primary, tabs', () => {
    const src = read('features/channel-monitor-v2/MonitorSettingsPanel.vue')
    expect(src).toContain('page-header')
    expect(src).toContain('btn btn-primary')
    expect(src).toContain('class="glass-card')
    // Enable + refresh-interval rows use the shared SettingRow/SegmentedControl
    // recipe instead of a bespoke .tabs/.tab-active button loop.
    expect(src).toContain('SettingRow')
    expect(src).toContain('SegmentedControl')
    expect(src).toMatch(/max-h-\[min\(40vh/)
  })

  it('admin ChannelMonitorView V2 tab chrome uses project tabs', () => {
    const src = read('views/admin/ChannelMonitorView.vue')
    expect(src).toContain('page-header')
    expect(src).toContain('page-title')
    expect(src).toContain('class="tabs')
    expect(src).toContain('tab-active')
    expect(src).toContain('MonitorSettingsPanel')
  })
})
