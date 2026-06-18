import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import OrderTable from '@/components/payment/OrderTable.vue'
import type { PaymentOrder } from '@/types/payment'

const messages: Record<string, string> = {
  'payment.orders.orderId': 'Order ID',
  'payment.orders.orderNo': 'Order No',
  'payment.orders.payAmount': 'Pay Amount',
  'payment.orders.paymentMethod': 'Payment Method',
  'payment.orders.status': 'Status',
  'payment.orders.createdAt': 'Created At',
  'payment.orders.fee': 'Fee',
  'payment.orders.creditedAmount': 'Credited Amount',
  'common.actions': 'Actions',
  'payment.methods.alipay': 'Alipay',
  'payment.methods.wxpay': 'WeChat Pay',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key,
  }),
}))

vi.mock('@/components/common/DataTable.vue', () => ({
  default: defineComponent({
    name: 'DataTableStub',
    props: {
      columns: { type: Array, required: true },
      data: { type: Array, required: true },
    },
    setup(props, { slots }) {
      return () => h('div', [
        h('div', { 'data-testid': 'column-keys' }, (props.columns as Array<{ key: string }>).map(col => col.key).join(',')),
        ...(props.data as Array<Record<string, unknown>>).map((row, rowIndex) =>
          h('div', { key: rowIndex, class: 'row' }, (props.columns as Array<{ key: string }>).map((col) => {
            const slotName = `cell-${col.key}`
            const slot = slots[slotName]
            const value = row[col.key]
            return h('div', { key: col.key, 'data-testid': `cell-${String(col.key)}` }, slot
              ? slot({ value, row })
              : String(value ?? ''),
            )
          })),
        ),
      ])
    },
  }),
}))

vi.mock('@/components/payment/OrderStatusBadge.vue', () => ({
  default: defineComponent({
    name: 'OrderStatusBadgeStub',
    props: ['status'],
    template: '<span>{{ status }}</span>',
  }),
}))

function createOrder(paymentType: string): PaymentOrder {
  return {
    id: 1,
    user_id: 2,
    amount: 10,
    pay_amount: 10,
    fee_rate: 0,
    payment_type: paymentType,
    out_trade_no: 'trade_1',
    status: 'PENDING',
    order_type: 'subscription',
    created_at: '2026-05-07T12:00:00.000Z',
    expires_at: '2026-05-07T12:10:00.000Z',
    refund_amount: 0,
    refund_requested_amount: 0,
  }
}

describe('OrderTable', () => {
  it('normalizes aliased payment methods in the payment type column', () => {
    const wrapper = mount(OrderTable, {
      props: {
        orders: [createOrder('alipay_direct'), createOrder('wechat_pay')],
        loading: false,
      },
    })

    const paymentCells = wrapper.findAll('[data-testid="cell-payment_type"]')

    expect(paymentCells).toHaveLength(2)
    expect(paymentCells[0]?.text()).toContain('Alipay')
    expect(paymentCells[1]?.text()).toContain('WeChat Pay')
  })
})
