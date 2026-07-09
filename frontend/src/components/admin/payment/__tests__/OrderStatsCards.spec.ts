import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import OrderStatsCards from '../OrderStatsCards.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('OrderStatsCards', () => {
  it('renders summary amounts without a hardcoded currency symbol for mixed-currency totals', () => {
    const wrapper = mount(OrderStatsCards, {
      props: {
        stats: {
          today_amount: 10,
          total_amount: 25.5,
          today_count: 2,
          total_count: 5,
          avg_amount: 5.1,
          pending_orders: 0,
          daily_series: [],
          payment_methods: [],
          top_users: [],
        },
      },
      global: {
        stubs: {
          Icon: { template: '<span />' },
        },
      },
    })

    expect(wrapper.text()).toContain('10.00')
    expect(wrapper.text()).toContain('25.50')
    expect(wrapper.text()).toContain('5.10')
    expect(wrapper.text()).not.toContain('$10.00')
    expect(wrapper.text()).not.toContain('$25.50')
    expect(wrapper.text()).not.toContain('$5.10')
  })
})
