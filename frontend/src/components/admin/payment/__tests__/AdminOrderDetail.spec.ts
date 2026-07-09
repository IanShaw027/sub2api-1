import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AdminOrderDetail from '../AdminOrderDetail.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => fallback || key,
    }),
  }
})

describe('AdminOrderDetail', () => {
  it('formats order-currency amounts without forced decimals for zero-decimal currencies', () => {
    const wrapper = mount(AdminOrderDetail, {
      props: {
        show: true,
        order: {
          id: 42,
          user_id: 9,
          amount: 100,
          pay_amount: 108,
          currency: 'JPY',
          fee_rate: 0,
          payment_type: 'stripe',
          out_trade_no: 'sub2_jpy_42',
          status: 'COMPLETED',
          order_type: 'balance',
          created_at: '2026-07-01T10:00:00Z',
          expires_at: '2026-07-01T10:30:00Z',
          refund_amount: 0,
          refund_requested_amount: 0,
        },
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show'],
            template: '<div v-if="show"><slot /></div>',
          },
        },
      },
    })

    expect(wrapper.text()).toContain('¥108')
    expect(wrapper.text()).not.toContain('¥108.00')
  })
})
