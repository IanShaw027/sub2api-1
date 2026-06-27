import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { PaymentOrder } from '@/types/payment'
import AdminOrdersView from '../AdminOrdersView.vue'

const { getOrders, getOrder, getRefundPreview, showError, showSuccess, adminPaymentAPI } = vi.hoisted(() => {
  const getOrders = vi.fn()
  const getOrder = vi.fn()
  const getRefundPreview = vi.fn()
  const showError = vi.fn()
  const showSuccess = vi.fn()
  return {
    getOrders,
    getOrder,
    getRefundPreview,
    showError,
    showSuccess,
    adminPaymentAPI: {
      getOrders,
      getOrder,
      getRefundPreview,
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
    showError,
    showSuccess,
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
const PaginationStub = {
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: `
    <div data-test="pagination">
      <button type="button" class="page-2" @click="$emit('update:page', 2)">page 2</button>
      <button type="button" class="page-3" @click="$emit('update:page', 3)">page 3</button>
      <button type="button" class="page-size-50" @click="$emit('update:pageSize', 50)">size 50</button>
    </div>
  `,
}
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
const AdminRefundDialogStub = {
  props: ['show', 'order', 'submitting', 'requireForce', 'warning', 'refundPreview'],
  emits: ['confirm', 'cancel'],
  template: `
    <div
      v-if="show"
      data-test="refund-dialog"
      :data-order-id="String(order?.id ?? '')"
      :data-submitting="String(Boolean(submitting))"
      :data-require-force="String(Boolean(requireForce))"
      :data-warning="warning || ''"
      :data-max-refund="String(refundPreview?.max_refund_amount ?? '')"
    >
      <button type="button" class="refund-confirm" :disabled="submitting" @click="$emit('confirm', { amount: 1, reason: 'refund reason', deduct_balance: true, force: false })">confirm</button>
      <button type="button" class="refund-confirm-force" :disabled="submitting" @click="$emit('confirm', { amount: 1, reason: 'refund reason', deduct_balance: true, force: true })">confirm force</button>
      <button type="button" class="refund-close" @click="$emit('cancel')">close</button>
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
    getRefundPreview.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    adminPaymentAPI.cancelOrder.mockReset()
    adminPaymentAPI.retryRecharge.mockReset()
    adminPaymentAPI.refundOrder.mockReset()
    getRefundPreview.mockResolvedValue({
      data: {
        order_id: 1,
        order_type: 'balance',
        order_amount: 99.5,
        already_refunded: 0,
        max_refund_amount: 99.5,
        refund_enabled: true,
        auto_refund: false,
      },
    })
  })

  it('cancels a pending search debounce before manual pagination loads a new page', async () => {
    vi.useFakeTimers()
    try {
      getOrders.mockResolvedValue({
        data: {
          items: [createOrder({ id: 1, out_trade_no: 'order-1', status: 'PAID' })],
          total: 2,
        },
      })

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
      expect(getOrders).toHaveBeenCalledTimes(1)
      expect(getOrders).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }))

      await wrapper.get('.page-2').trigger('click')
      await flushPromises()
      expect(getOrders).toHaveBeenCalledTimes(2)
      expect(getOrders).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))

      await wrapper.get('input[type="text"]').setValue('stale')
      await wrapper.get('.page-3').trigger('click')
      await flushPromises()
      expect(getOrders).toHaveBeenCalledTimes(3)
      expect(getOrders).toHaveBeenLastCalledWith(expect.objectContaining({ page: 3 }))

      await vi.advanceTimersByTimeAsync(300)
      await flushPromises()

      expect(getOrders).toHaveBeenCalledTimes(3)
      expect(getOrders.mock.calls.map(([params]) => params.page)).toEqual([1, 2, 3])
    } finally {
      vi.useRealTimers()
    }
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

  it('shows requested refund amount for refund requests and exposes all refund status filters', async () => {
    getOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({
            id: 1,
            out_trade_no: 'order-refund-requested',
            status: 'REFUND_REQUESTED',
            refund_amount: 12.34,
            refund_requested_amount: 12.34,
          }),
        ],
        total: 1,
      },
    })

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

    expect(wrapper.text()).toContain('¥12.34')
    expect(wrapper.text().split('¥12.34')).toHaveLength(2)
    expect(wrapper.text()).toContain('payment.status.refunding')
    expect(wrapper.text()).toContain('payment.status.partially_refunded')
  })

  it('clears a pending order search debounce when unmounted', async () => {
    vi.useFakeTimers()
    try {
      getOrders.mockResolvedValue({
        data: {
          items: [],
          total: 0,
        },
      })

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
      expect(getOrders).toHaveBeenCalledTimes(1)

      await wrapper.get('input[type="text"]').setValue('stale')
      wrapper.unmount()

      await vi.advanceTimersByTimeAsync(300)
      await flushPromises()

      expect(getOrders).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('invalidates list, detail, and refund requests on unmount', async () => {
    getOrders.mockResolvedValueOnce({
      data: {
        items: [
          createOrder({ id: 1, out_trade_no: 'order-1', status: 'REFUND_REQUESTED', refund_amount: 3.14, refund_requested_amount: 3.14 }),
        ],
        total: 1,
      },
    })

    const staleListResponse = createDeferred<{ data: { items: PaymentOrder[]; total: number } }>()
    getOrders.mockImplementationOnce(() => staleListResponse.promise)

    const detailResponse = createDeferred<{ data: { order: PaymentOrder; auditLogs: Array<{ id: number; action: string; detail: string | null; operator: string | null; created_at: string }> } }>()
    getOrder.mockReturnValueOnce(detailResponse.promise)

    const refundResponse = createDeferred<{ data: unknown }>()
    adminPaymentAPI.refundOrder.mockReturnValueOnce(refundResponse.promise)

    const wrapper = mount(AdminOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          AdminRefundDialog: AdminRefundDialogStub,
          OrderStatusBadge: true,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const viewButton = wrapper.findAll('button').find((button) => button.text().includes('common.view'))
    const refundButton = wrapper.findAll('button').find((button) => button.text().includes('payment.admin.approveRefund'))
    expect(viewButton).toBeTruthy()
    expect(refundButton).toBeTruthy()

    await viewButton!.trigger('click')
    await refundButton!.trigger('click')
    await wrapper.get('[data-test="refund-dialog"] .refund-confirm').trigger('click')
    await wrapper.get('button[title="common.refresh"]').trigger('click')
    await flushPromises()

    const vm = wrapper.vm as unknown as {
      selectedOrder: PaymentOrder | null
      orderAuditLogs: Array<{ id: number }>
    }
    expect(vm.selectedOrder?.out_trade_no).toBe('order-1')
    expect(vm.orderAuditLogs).toHaveLength(0)

    wrapper.unmount()

    detailResponse.resolve({
      data: {
        order: createOrder({ id: 1, out_trade_no: 'order-1-updated', status: 'COMPLETED' }),
        auditLogs: [{ id: 1, action: 'updated', detail: null, operator: null, created_at: '2026-05-22T00:00:00Z' }],
      },
    })
    refundResponse.resolve({ data: {} })
    staleListResponse.reject(new Error('stale list failed'))
    await flushPromises()

    expect(vm.selectedOrder?.out_trade_no).toBe('order-1')
    expect(vm.orderAuditLogs).toHaveLength(0)
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).not.toHaveBeenCalled()
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

  it('formats order detail amounts with the order currency', async () => {
    getOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({
            id: 3,
            out_trade_no: 'order-eur-detail',
            status: 'COMPLETED',
            currency: 'EUR',
            amount: 12.34,
            pay_amount: 12.34,
          }),
        ],
        total: 1,
      },
    })
    getOrder.mockResolvedValueOnce({
      data: {
        order: createOrder({
          id: 3,
          out_trade_no: 'order-eur-detail',
          status: 'COMPLETED',
          currency: 'EUR',
          amount: 12.34,
          pay_amount: 12.34,
          refund_amount: 1.23,
          refund_requested_amount: 4.56,
        }),
        auditLogs: [],
      },
    })

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

    const viewButton = wrapper.findAll('button').find((button) => button.text().includes('common.view'))
    expect(viewButton).toBeTruthy()

    await viewButton!.trigger('click')
    await flushPromises()

    const dialogText = wrapper.get('[data-test="dialog"]').text()
    expect(dialogText).toContain('€12.34')
    expect(dialogText).toContain('€1.23')
    expect(dialogText).toContain('€4.56')
    expect(dialogText).not.toContain('$12.34')
    expect(dialogText).not.toContain('¥12.34')
  })

  it('does not keep a reopened refund dialog disabled or overwrite it with a stale refund response', async () => {
    getOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({ id: 1, out_trade_no: 'order-old', status: 'REFUND_REQUESTED', refund_amount: 3.14, refund_requested_amount: 3.14 }),
          createOrder({ id: 2, out_trade_no: 'order-new', status: 'REFUND_REQUESTED', refund_amount: 6.28, refund_requested_amount: 6.28 }),
        ],
        total: 2,
      },
    })

    const refundRequest = createDeferred<{ data: unknown }>()
    adminPaymentAPI.refundOrder.mockReturnValueOnce(refundRequest.promise)

    const wrapper = mount(AdminOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          AdminRefundDialog: AdminRefundDialogStub,
          OrderStatusBadge: true,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const refundButtons = wrapper.findAll('button').filter((button) => button.text().includes('payment.admin.approveRefund'))
    expect(refundButtons).toHaveLength(2)

    await refundButtons[0].trigger('click')
    await wrapper.get('[data-test="refund-dialog"] .refund-confirm').trigger('click')
    await flushPromises()

    expect(adminPaymentAPI.refundOrder).toHaveBeenCalledWith(1, {
      amount: 1,
      reason: 'refund reason',
      deduct_balance: true,
      force: false,
    })
    expect(wrapper.get('[data-test="refund-dialog"]').attributes('data-order-id')).toBe('1')
    expect(wrapper.get('[data-test="refund-dialog"]').attributes('data-submitting')).toBe('true')

    await wrapper.get('[data-test="refund-dialog"] .refund-close').trigger('click')
    await flushPromises()

    await refundButtons[1].trigger('click')
    await flushPromises()

    const reopenedDialog = wrapper.get('[data-test="refund-dialog"]')
    expect(reopenedDialog.attributes('data-order-id')).toBe('2')
    expect(reopenedDialog.attributes('data-submitting')).toBe('false')

    refundRequest.resolve({ data: {} })
    await flushPromises()

    expect(wrapper.get('[data-test="refund-dialog"]').attributes('data-order-id')).toBe('2')
    expect(wrapper.get('[data-test="refund-dialog"]').attributes('data-submitting')).toBe('false')
  })

  it('keeps the refund dialog open and surfaces warning when refund requires force', async () => {
    getOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({ id: 1, out_trade_no: 'order-1', status: 'COMPLETED' }),
        ],
        total: 1,
      },
    })
    adminPaymentAPI.refundOrder.mockResolvedValueOnce({
      data: {
        success: false,
        warning: 'order has an issued invoice; refund requires a credit note (use force)',
        require_force: true,
      },
    })

    const wrapper = mount(AdminOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          AdminRefundDialog: AdminRefundDialogStub,
          OrderStatusBadge: true,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const refundButton = wrapper.findAll('button').find((button) => button.text().includes('payment.admin.refund'))
    expect(refundButton).toBeTruthy()

    await refundButton!.trigger('click')
    await wrapper.get('[data-test="refund-dialog"] .refund-confirm').trigger('click')
    await flushPromises()

    const dialog = wrapper.get('[data-test="refund-dialog"]')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(showError).toHaveBeenCalledWith('order has an issued invoice; refund requires a credit note (use force)')
    expect(dialog.attributes('data-require-force')).toBe('true')
    expect(dialog.attributes('data-warning')).toBe('order has an issued invoice; refund requires a credit note (use force)')
  })

  it('loads refund preview and passes backend max refundable amount to the refund dialog', async () => {
    getOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({
            id: 1,
            out_trade_no: 'order-subscription',
            status: 'COMPLETED',
            order_type: 'subscription',
            amount: 100,
            refund_amount: 0,
          }),
        ],
        total: 1,
      },
    })
    getRefundPreview.mockResolvedValueOnce({
      data: {
        order_id: 1,
        order_type: 'subscription',
        order_amount: 100,
        already_refunded: 0,
        max_refund_amount: 37.5,
        refund_enabled: true,
        auto_refund: false,
      },
    })

    const wrapper = mount(AdminOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          AdminRefundDialog: AdminRefundDialogStub,
          OrderStatusBadge: true,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const refundButton = wrapper.findAll('button').find((button) => button.text().includes('payment.admin.refund'))
    expect(refundButton).toBeTruthy()

    await refundButton!.trigger('click')
    await flushPromises()

    expect(getRefundPreview).toHaveBeenCalledWith(1)
    expect(wrapper.get('[data-test="refund-dialog"]').attributes('data-max-refund')).toBe('37.5')
  })

  it('requires admin confirmation and sends force when refunding an order with an invoice application', async () => {
    getOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({
            id: 1,
            out_trade_no: 'order-invoice',
            status: 'COMPLETED',
            invoice_id: 42,
            invoice_status: 'ISSUED',
          }),
        ],
        total: 1,
      },
    })
    adminPaymentAPI.refundOrder.mockResolvedValueOnce({ data: { success: true } })

    const wrapper = mount(AdminOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          AdminRefundDialog: AdminRefundDialogStub,
          OrderStatusBadge: true,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const refundButton = wrapper.findAll('button').find((button) => button.text().includes('payment.admin.refund'))
    expect(refundButton).toBeTruthy()

    await refundButton!.trigger('click')
    const dialog = wrapper.get('[data-test="refund-dialog"]')
    expect(dialog.attributes('data-require-force')).toBe('true')
    expect(dialog.attributes('data-warning')).toContain('payment.admin.invoiceRefundIssuedWarning')

    await wrapper.get('[data-test="refund-dialog"] .refund-confirm-force').trigger('click')
    await flushPromises()

    expect(adminPaymentAPI.refundOrder).toHaveBeenCalledWith(1, {
      amount: 1,
      reason: 'refund reason',
      deduct_balance: true,
      force: true,
    })
  })
})
