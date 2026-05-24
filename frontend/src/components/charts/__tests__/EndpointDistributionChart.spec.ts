import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import EndpointDistributionChart from '../EndpointDistributionChart.vue'

const { getUserBreakdown } = vi.hoisted(() => ({
  getUserBreakdown: vi.fn(),
}))

vi.mock('@/api/admin/dashboard', () => ({
  getUserBreakdown,
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('vue-chartjs', () => ({
  Doughnut: {
    props: ['data'],
    template: '<div class="chart-data">{{ JSON.stringify(data) }}</div>',
  },
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

describe('EndpointDistributionChart drilldown races', () => {
  it('keeps the latest endpoint breakdown response', async () => {
    const first = deferred<any>()
    const second = deferred<any>()
    getUserBreakdown.mockImplementationOnce(() => first.promise)
    getUserBreakdown.mockImplementationOnce(() => second.promise)

    const wrapper = mount(EndpointDistributionChart, {
      props: {
        endpointStats: [
          { endpoint: '/v1/old', requests: 1, total_tokens: 100, cost: 1, actual_cost: 1 },
          { endpoint: '/v1/new', requests: 2, total_tokens: 200, cost: 2, actual_cost: 2 },
        ],
      },
      global: {
        stubs: {
          LoadingSpinner: true,
          UserBreakdownSubTable: {
            props: ['items', 'loading'],
            template: '<div data-test="breakdown">{{ items.map((item) => item.email).join(",") }}</div>',
          },
        },
      },
    })

    const rows = wrapper.findAll('tbody tr').filter((row) => row.text().includes('/v1/'))
    await rows[0].trigger('click')
    await rows[1].trigger('click')

    second.resolve({ users: [{ user_id: 2, email: 'new@example.com' }] })
    await flushPromises()
    expect(wrapper.get('[data-test="breakdown"]').text()).toBe('new@example.com')

    first.resolve({ users: [{ user_id: 1, email: 'old@example.com' }] })
    await flushPromises()
    expect(wrapper.get('[data-test="breakdown"]').text()).toBe('new@example.com')
  })
})
