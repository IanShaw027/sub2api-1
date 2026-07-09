import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { PaymentOrder } from '@/types/payment'
import UserOrdersView from '../UserOrdersView.vue'

const { paymentAPI, showError, showSuccess, routerPush } = vi.hoisted(() => ({
  paymentAPI: {
    getMyOrders: vi.fn(),
    getRefundEligibleProviders: vi.fn(),
    getInvoiceEligibleProviders: vi.fn(),
    cancelOrder: vi.fn(),
    getRefundPreview: vi.fn(),
    requestRefund: vi.fn(),
    createInvoice: vi.fn(),
  },
  showError: vi.fn(),
  showSuccess: vi.fn(),
  routerPush: vi.fn(),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: routerPush,
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
const OrdersTabBarStub = { template: '<div />' }
const BaseDialogStub = { props: ['show'], template: '<div v-if="show"><slot /><slot name="footer" /></div>' }
const IconStub = { template: '<span />' }
const PaginationStub = { template: '<div />' }
const SelectStub = {
  props: ['modelValue', 'options'],
  emits: ['update:modelValue', 'change'],
  template: `<select @change="$emit('change')"></select>`,
}
const OrderTableStub = {
  props: ['orders', 'selectable', 'isSelected', 'isRowSelectable'],
  emits: ['toggle-row'],
  template: `
    <div>
      <div v-for="row in orders" :key="row.id" :data-test="'row-actions-' + row.out_trade_no">
        <input
          v-if="selectable"
          :data-test="'select-' + row.out_trade_no"
          type="checkbox"
          :checked="isSelected?.(row) ?? false"
          :disabled="!(isRowSelectable?.(row) ?? true)"
          @change="$emit('toggle-row', row)"
        />
        <slot name="actions" :row="row" />
      </div>
    </div>
  `,
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
    provider_instance_id: 'provider-1',
    ...overrides,
  }
}

describe('UserOrdersView refund visibility', () => {
  beforeEach(() => {
    paymentAPI.getMyOrders.mockReset()
    paymentAPI.getRefundEligibleProviders.mockReset()
    paymentAPI.getInvoiceEligibleProviders.mockReset()
    paymentAPI.cancelOrder.mockReset()
    paymentAPI.getRefundPreview.mockReset()
    paymentAPI.requestRefund.mockReset()
    paymentAPI.createInvoice.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    routerPush.mockReset()

    paymentAPI.getRefundEligibleProviders.mockResolvedValue({
      data: { provider_instance_ids: ['provider-1'] },
    })
    paymentAPI.getInvoiceEligibleProviders.mockResolvedValue({
      data: { provider_instance_ids: ['provider-1'] },
    })
  })

  it('hides the refund request action for orders with active invoice applications', async () => {
    paymentAPI.getMyOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({ id: 1, out_trade_no: 'order-applied', invoice_id: 41, invoice_status: 'APPLIED' }),
          createOrder({ id: 2, out_trade_no: 'order-issued', invoice_id: 42, invoice_status: 'ISSUED' }),
          createOrder({ id: 3, out_trade_no: 'order-open' }),
        ],
        total: 3,
      },
    })

    const wrapper = mount(UserOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          OrdersTabBar: OrdersTabBarStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-test="row-actions-order-applied"]').text()).not.toContain('payment.orders.requestRefund')
    expect(wrapper.get('[data-test="row-actions-order-issued"]').text()).not.toContain('payment.orders.requestRefund')
    expect(wrapper.get('[data-test="row-actions-order-open"]').text()).toContain('payment.orders.requestRefund')
  })

  it('formats the refund dialog amounts with the order currency', async () => {
    paymentAPI.getMyOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({
            id: 4,
            out_trade_no: 'order-eur',
            amount: 12.34,
            pay_amount: 12.34,
            currency: 'EUR',
          }),
        ],
        total: 1,
      },
    })
    paymentAPI.getRefundPreview.mockResolvedValue({
      data: {
        order_id: 4,
        order_type: 'balance',
        order_amount: 12.34,
        already_refunded: 0,
        max_refund_amount: 12.34,
        refund_enabled: true,
        auto_refund: false,
      },
    })

    const wrapper = mount(UserOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          OrdersTabBar: OrdersTabBarStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const refundButton = wrapper
      .findAll('[data-test="row-actions-order-eur"] button')
      .find((button) => button.text().includes('payment.orders.requestRefund'))
    expect(refundButton).toBeTruthy()
    await refundButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('€12.34')
    expect(wrapper.text()).not.toContain('$12.34')
  })

  it('uses preview max_refund_amount as the default refund amount instead of credited order amount', async () => {
    paymentAPI.getMyOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({
            id: 7,
            out_trade_no: 'order-mixed-currency',
            amount: 8,
            pay_amount: 12.34,
            currency: 'EUR',
          }),
        ],
        total: 1,
      },
    })
    paymentAPI.getRefundPreview.mockResolvedValue({
      data: {
        order_id: 7,
        order_type: 'balance',
        order_amount: 12.34,
        already_refunded: 0,
        max_refund_amount: 12.34,
        refund_enabled: true,
        auto_refund: false,
      },
    })

    const wrapper = mount(UserOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          OrdersTabBar: OrdersTabBarStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()

    const refundButton = wrapper
      .findAll('[data-test="row-actions-order-mixed-currency"] button')
      .find((button) => button.text().includes('payment.orders.requestRefund'))
    expect(refundButton).toBeTruthy()
    await refundButton!.trigger('click')
    await flushPromises()

    expect((wrapper.get('input[type="number"]').element as HTMLInputElement).value).toBe('12.34')
  })

  it('formats selected invoice totals by the selected orders currencies', async () => {
    paymentAPI.getMyOrders.mockResolvedValue({
      data: {
        items: [
          createOrder({
            id: 5,
            out_trade_no: 'order-usd',
            amount: 12.34,
            pay_amount: 12.34,
            currency: 'USD',
          }),
          createOrder({
            id: 6,
            out_trade_no: 'order-eur',
            amount: 5,
            pay_amount: 5,
            currency: 'EUR',
          }),
        ],
        total: 2,
      },
    })

    const wrapper = mount(UserOrdersView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          OrdersTabBar: OrdersTabBarStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
          OrderTable: OrderTableStub,
        },
      },
    })

    await flushPromises()
    await wrapper.get('[data-test="select-order-usd"]').trigger('change')
    await wrapper.get('[data-test="select-order-eur"]').trigger('change')
    await flushPromises()

    expect(wrapper.text()).toContain('$12.34')
    expect(wrapper.text()).toContain('€5.00')
    expect(wrapper.text()).not.toContain('¥17.34')

    const createInvoiceButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('payment.invoice.create.action'))
    expect(createInvoiceButton).toBeTruthy()
    await createInvoiceButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('$12.34')
    expect(wrapper.text()).toContain('€5.00')
    expect(wrapper.text()).not.toContain('¥17.34')
  })
})
