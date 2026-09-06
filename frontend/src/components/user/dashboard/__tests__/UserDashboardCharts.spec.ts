import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { ModelStat } from '@/types'
import UserDashboardCharts from '../UserDashboardCharts.vue'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function mountCharts(models: ModelStat[], error = false) {
  return mount(UserDashboardCharts, {
    props: { models, trend: [], granularity: 'day', loading: false, error, rangeLabel: '2026-08-01 - 2026-08-31' },
    global: { stubs: { TokenUsageTrend: true } },
  })
}

describe('dashboard distribution and failure states', () => {
  it('uses the total cost as denominator and labels the selected range', () => {
    const wrapper = mountCharts([
      { model: 'model-b', actual_cost: 20 },
      { model: 'model-a', actual_cost: 80 },
    ] as ModelStat[])
    expect(wrapper.findAll('.dash-dist-meta').map(row => row.text())).toEqual(['$80.0000 · 80.0%', '$20.0000 · 20.0%'])
    expect(wrapper.findAll('.progress-bar').map(row => row.attributes('style'))).toEqual(['width: 80%;', 'width: 20%;'])
    expect(wrapper.findAll('.dash-chart-sub').every(row => row.text() === '2026-08-01 - 2026-08-31')).toBe(true)
    wrapper.unmount()
  })

  it('shows zero percentages for a zero-cost range rather than NaN', () => {
    const wrapper = mountCharts([{ model: 'free-model', actual_cost: 0 }] as ModelStat[])
    expect(wrapper.get('.dash-dist-meta').text()).toContain('0.0%')
    expect(wrapper.get('.progress-bar').attributes('style')).toBe('width: 0%;')
    wrapper.unmount()
  })

  it('offers retry on failure rather than displaying a no-data message', async () => {
    const wrapper = mountCharts([], true)
    expect(wrapper.find('.dash-dist-empty').exists()).toBe(false)
    expect(wrapper.findAll('[role="alert"]')).toHaveLength(2)
    await wrapper.get('[role="alert"] button').trigger('click')
    expect(wrapper.emitted('retry')).toHaveLength(1)
    wrapper.unmount()
  })
})
