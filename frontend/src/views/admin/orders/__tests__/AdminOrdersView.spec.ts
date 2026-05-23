import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { PaymentOrder } from '@/types/payment'
import AdminOrdersView from '../AdminOrdersView.vue'

const { getOrders, getOrder, adminPaymentAPI } = vi.hoisted(() => {
  const getOrders = vi.fn()
  const getOrder = vi.fn()
  return {
    getOrders,
    getOrder,
    adminPaymentAPI: {
      getOrders,
      getOrder,
      cancelOrder: vi.fn(),
      retryRecharge: vi.fn(),
      refundOrder: vi.fn(),
    },
  }
})

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI,
  default: adminPaymentAPI,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
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

const AppLayoutStub = { template: '<div><slot /></div>' }
const BaseDialogStub = { props: ['show'], template: '<div v-if="show" data-test="dialog"><slot /><slot name="footer" /></div>' }
const PaginationStub = { template: '<div data-test="pagination" />' }
const IconStub = { template: '<span />' }
const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: `
    <select :value="modelValue ?? ''" @change="$emit('update:modelValue', $event.target.value); $emit('change')">
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
    </select>
  `,
}
const OrderTableStub = {
  props: ['orders', 'loading'],
  template: `
    <div>
      <div data-test="orders">{{ orders.map((row) => row.out_trade_no).join(',') }}</div>
      <div v-for="row in orders" :key="row.id">
        <slot name="actions" :row="row" />
      </div>
    </div>
  `,
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function createOrder(overrides: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 1,
    user_id: 1001,
    amount: 99.5,
    pay_amount: 99.5,
    fee_rate: 0.03,
    payment_type: 'alipay',
    out_trade_no: 'order-1',
    status: 'COMPLETED',
    order_type: 'balance',
    created_at: '2026-05-22T00:00:00Z',
    expires_at: '2026-05-22T01:00:00Z',
    refund_amount: 0,
    refund_requested_amount: 0,
    ...overrides,
  }
}

describe('AdminOrdersView request races', () => {
  beforeEach(() => {
    getOrders.mockReset()
    getOrder.mockReset()
  })

  it('keeps the newest list response when filters change before the first request returns', async () => {
    const firstResponse = createDeferred<{ data: { items: PaymentOrder[]; total: number } }>()
    const secondResponse = createDeferred<{ data: { items: PaymentOrder[]; total: number } }>()
    getOrders.mockImplementationOnce(() => firstResponse.promise)
    getOrders.mockImplementationOnce(() => secondResponse.promise)

    const wrapper = mount(AdminOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          AdminRefundDialog: true,
          OrderStatusBadge: true,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    await wrapper.get('select').setValue('PAID')
    expect(getOrders).toHaveBeenCalledTimes(2)

    secondResponse.resolve({
      data: {
        items: [createOrder({ id: 2, out_trade_no: 'order-new', status: 'PAID' })],
        total: 1,
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-test="orders"]').text()).toBe('order-new')

    firstResponse.resolve({
      data: {
        items: [createOrder({ id: 1, out_trade_no: 'order-old' })],
        total: 1,
      },
    })
    await flushPromises()

    expect(wrapper.get('[data-test="orders"]').text()).toBe('order-new')
  })

  it('keeps the newest order detail when two rows are opened back to back', async () => {
    getOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({ id: 1, out_trade_no: 'order-old', status: 'FAILED' }),
          createOrder({ id: 2, out_trade_no: 'order-new', status: 'PAID' }),
        ],
        total: 2,
      },
    })

    const firstDetail = createDeferred<{ data: { order: PaymentOrder; auditLogs: Array<{ id: number; action: string; detail: string | null; operator: string | null; created_at: string }> } }>()
    const secondDetail = createDeferred<{ data: { order: PaymentOrder; auditLogs: Array<{ id: number; action: string; detail: string | null; operator: string | null; created_at: string }> } }>()
    getOrder.mockImplementationOnce(() => firstDetail.promise)
    getOrder.mockImplementationOnce(() => secondDetail.promise)

    const wrapper = mount(AdminOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          AdminRefundDialog: true,
          OrderStatusBadge: true,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const viewButtons = wrapper.findAll('button').filter((button) => button.text().includes('common.view'))
    expect(viewButtons).toHaveLength(2)

    await viewButtons[0].trigger('click')
    await viewButtons[1].trigger('click')

    expect(getOrder).toHaveBeenNthCalledWith(1, 1)
    expect(getOrder).toHaveBeenNthCalledWith(2, 2)

    secondDetail.resolve({
      data: {
        order: createOrder({ id: 2, out_trade_no: 'order-new', status: 'PAID' }),
        auditLogs: [],
      },
    })
    await flushPromises()

    const dialog = wrapper.get('[data-test="dialog"]')
    expect(dialog.text()).toContain('order-new')
    expect(dialog.text()).not.toContain('order-old')

    firstDetail.resolve({
      data: {
        order: createOrder({ id: 1, out_trade_no: 'order-old', status: 'FAILED' }),
        auditLogs: [],
      },
    })
    await flushPromises()

    expect(dialog.text()).toContain('order-new')
    expect(dialog.text()).not.toContain('order-old')
  })
})
