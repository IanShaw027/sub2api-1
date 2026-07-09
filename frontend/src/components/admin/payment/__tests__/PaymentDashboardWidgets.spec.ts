import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import PaymentMethodChart from '../PaymentMethodChart.vue'
import TopUsersLeaderboard from '../TopUsersLeaderboard.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => fallback || key,
    }),
  }
})

describe('payment dashboard widgets', () => {
  it('renders method totals without a hardcoded dollar prefix', () => {
    const wrapper = mount(PaymentMethodChart, {
      props: {
        methods: [
          { type: 'stripe', amount: 103, count: 1 },
        ],
      },
    })

    expect(wrapper.text()).toContain('103.00')
    expect(wrapper.text()).not.toContain('$103.00')
  })

  it('renders top-user totals without a hardcoded dollar prefix', () => {
    const wrapper = mount(TopUsersLeaderboard, {
      props: {
        users: [
          { user_id: 8, email: 'user@example.com', amount: 88 },
        ],
      },
    })

    expect(wrapper.text()).toContain('88.00')
    expect(wrapper.text()).not.toContain('$88.00')
  })
})
