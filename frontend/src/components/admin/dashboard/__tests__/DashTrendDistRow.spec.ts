import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import DashTrendDistRow from '../DashTrendDistRow.vue'

const { dimensions } = vi.hoisted(() => ({ dimensions: { width: undefined as unknown } }))
vi.mock('@vueuse/core', () => ({ useElementSize: () => ({ width: dimensions.width }) }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/api/admin/dashboard', () => ({ getModelStats: vi.fn(), getUserBreakdown: vi.fn() }))

describe('DashTrendDistRow axis labels', () => {
  it('sparsifies narrow labels without removing bars and restores labels on resize', async () => {
    const width = ref(310)
    dimensions.width = width
    const trendBars = Array.from({ length: 14 }, (_, index) => ({
      key: String(index), height: `${30 + index}%`, label: `${index}:00`,
      title: `${index}:00: ${100 + index} requests`, fill: 'var(--accent)'
    }))
    const wrapper = mount(DashTrendDistRow, {
      props: {
        trendBars, trendSubtitle: 'Today', trendMetricOptions: [], trendMetric: 'requests',
        distRows: [], distRowsLoading: false, distView: 'models', startDate: '2026-09-01', endDate: '2026-09-07'
      }
    })

    expect(wrapper.findAll('.dash-bar')).toHaveLength(14)
    expect(wrapper.findAll('.dash-bar-label:not(.is-skipped)').length).toBeLessThanOrEqual(6)
    expect(wrapper.findAll('.dash-bar-label:not(.is-skipped)').map(label => label.text()))
      .toEqual(['0:00', '3:00', '6:00', '9:00', '13:00'])
    expect(wrapper.findAll('.dash-bar').map(bar => bar.attributes('title')))
      .toEqual(trendBars.map(bar => bar.title))

    width.value = 900
    await nextTick()
    expect(wrapper.findAll('.dash-bar-label:not(.is-skipped)')).toHaveLength(14)
    expect(wrapper.findAll('.dash-bar')).toHaveLength(14)
    wrapper.unmount()
  })
})
