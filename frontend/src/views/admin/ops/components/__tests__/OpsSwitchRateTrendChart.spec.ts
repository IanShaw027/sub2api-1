import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import OpsSwitchRateTrendChart from '../OpsSwitchRateTrendChart.vue'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Line: {
    name: 'Line',
    props: ['data', 'options'],
    template: '<div data-test="line-chart" />',
  },
}))

vi.mock('@/components/common/HelpTooltip.vue', () => ({
  default: {
    name: 'HelpTooltip',
    template: '<span />',
  },
}))

vi.mock('@/components/common/EmptyState.vue', () => ({
  default: {
    name: 'EmptyState',
    template: '<div data-test="empty-state" />',
  },
}))

describe('OpsSwitchRateTrendChart', () => {
  it('renders separate upstream failover and sticky original unavailable rate datasets', () => {
    const wrapper = mount(OpsSwitchRateTrendChart, {
      props: {
        loading: false,
        timeRange: '1h',
        points: [
          {
            bucket_start: '2026-07-08T10:00:00Z',
            request_count: 10,
            token_consumed: 0,
            switch_count: 2,
            qps: 0,
            tps: 0,
            sticky_original_bound_count: 4,
            sticky_original_unavailable_count: 1,
          },
        ],
      },
    })

    const line = wrapper.getComponent({ name: 'Line' })
    const data = line.props('data') as any
    expect(data.datasets).toHaveLength(2)
    expect(data.datasets[0].label).toBe('admin.ops.upstreamFailoverRate')
    expect(data.datasets[0].data).toEqual([0.2])
    expect(data.datasets[1].label).toBe('admin.ops.stickyOriginalUnavailableRate')
    expect(data.datasets[1].data).toEqual([0.25])
  })
})
