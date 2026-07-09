import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { Invoice } from '@/types/payment'
import { formatPaymentAmount } from '@/components/payment/currency'
import AdminInvoiceApplicationsView from '../AdminInvoiceApplicationsView.vue'

const { getInvoices, getInvoice, cancelInvoice, adminPaymentAPI } = vi.hoisted(() => {
  const getInvoices = vi.fn()
  const getInvoice = vi.fn()
  const cancelInvoice = vi.fn()
  return {
    getInvoices,
    getInvoice,
    cancelInvoice,
    adminPaymentAPI: {
      getInvoices,
      getInvoice,
      cancelInvoice,
      uploadInvoiceFile: vi.fn(),
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
    useI18n: () => ({ t: (key: string) => key }),
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

function createInvoice(overrides: Partial<Invoice> = {}): Invoice {
  return {
    id: 42,
    user_id: 1,
    user_email: 'first@example.com',
    status: 'APPLIED',
    invoice_amount: 318,
    order_count: 3,
    title: 'ACME Inc.',
    tax_number: '91110000000000000X',
    email: 'billing@acme.com',
    contact_name: 'Alice',
    contact_phone: '13800000000',
    request_note: undefined,
    file_media_id: undefined,
    file_name: '',
    file_mime_type: '',
    file_size_bytes: 0,
    applied_at: '2026-06-08T00:00:00Z',
    cancelled_at: undefined,
    issued_at: undefined,
    created_at: '2026-06-08T00:00:00Z',
    updated_at: '2026-06-08T00:00:00Z',
    orders: [
      { order_id: 101, out_trade_no: 'ORD-101', pay_amount_snapshot: 100, payment_type: 'wxpay', created_at: '2026-06-08T00:00:00Z' },
      { order_id: 102, out_trade_no: 'ORD-102', pay_amount_snapshot: 100, payment_type: 'wxpay', created_at: '2026-06-08T00:00:00Z' },
      { order_id: 103, out_trade_no: 'ORD-103', pay_amount_snapshot: 118, payment_type: 'alipay', created_at: '2026-06-08T00:00:00Z' },
    ],
    ...overrides,
  }
}

function mountView() {
  return mount(AdminInvoiceApplicationsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        BaseDialog: BaseDialogStub,
        Pagination: PaginationStub,
        Icon: IconStub,
        Select: SelectStub,
      },
    },
  })
}

describe('AdminInvoiceApplicationsView', () => {
  beforeEach(() => {
    getInvoices.mockReset()
    getInvoice.mockReset()
    cancelInvoice.mockReset()
  })

  it('renders list with order_count and user_email columns', async () => {
    getInvoices.mockResolvedValueOnce({
      data: { items: [createInvoice()], total: 1, page: 1, page_size: 20 },
    })
    const wrapper = mountView()
    await flushPromises()
    const text = wrapper.text()
    expect(text).toContain('first@example.com')
    expect(text).toContain('ACME Inc.')
    expect(text).toContain('3')
    expect(text).toContain('318')
  })

  it('shows related orders table in detail dialog', async () => {
    getInvoices.mockResolvedValueOnce({
      data: { items: [createInvoice()], total: 1, page: 1, page_size: 20 },
    })
    getInvoice.mockResolvedValueOnce({ data: createInvoice() })

    const wrapper = mountView()
    await flushPromises()
    const viewBtn = wrapper.findAll('button').find((b) => b.text().includes('common.view'))
    expect(viewBtn).toBeDefined()
    await viewBtn!.trigger('click')
    await flushPromises()

    const dialog = wrapper.find('[data-test="dialog"]')
    expect(dialog.exists()).toBe(true)
    expect(dialog.text()).toContain('ORD-101')
    expect(dialog.text()).toContain('ORD-102')
    expect(dialog.text()).toContain('ORD-103')
  })

  it('formats list and detail amounts with invoice currency instead of hardcoded cny symbol', async () => {
    getInvoices.mockResolvedValueOnce({
      data: { items: [createInvoice({ invoice_amount: 318, currency: 'USD' })], total: 1, page: 1, page_size: 20 },
    })
    getInvoice.mockResolvedValueOnce({
      data: createInvoice({
        invoice_amount: 318,
        currency: 'USD',
        orders: [
          { order_id: 101, out_trade_no: 'ORD-101', pay_amount_snapshot: 100.5, payment_type: 'stripe', created_at: '2026-06-08T00:00:00Z' },
        ],
      }),
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain(formatPaymentAmount(318, 'USD'))
    expect(wrapper.text()).not.toContain('¥318.00')

    await wrapper.findAll('button').find((b) => b.text().includes('common.view'))!.trigger('click')
    await flushPromises()

    const dialog = wrapper.find('[data-test="dialog"]')
    expect(dialog.text()).toContain(formatPaymentAmount(318, 'USD'))
    expect(dialog.text()).toContain(formatPaymentAmount(100.5, 'USD'))
    expect(dialog.text()).not.toContain('¥100.50')
  })

  it('shows cancel button only when status is APPLIED', async () => {
    getInvoices.mockResolvedValueOnce({
      data: { items: [createInvoice()], total: 1, page: 1, page_size: 20 },
    })
    getInvoice.mockResolvedValueOnce({ data: createInvoice({ status: 'APPLIED' }) })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text().includes('common.view'))!.trigger('click')
    await flushPromises()

    const dialog = wrapper.find('[data-test="dialog"]')
    expect(dialog.text()).toContain('payment.invoice.cancel')
  })

  it('hides cancel button when status is ISSUED', async () => {
    getInvoices.mockResolvedValueOnce({
      data: { items: [createInvoice({ status: 'ISSUED' })], total: 1, page: 1, page_size: 20 },
    })
    getInvoice.mockResolvedValueOnce({ data: createInvoice({ status: 'ISSUED' }) })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text().includes('common.view'))!.trigger('click')
    await flushPromises()

    const dialog = wrapper.find('[data-test="dialog"]')
    expect(dialog.text()).not.toContain('payment.invoice.cancel')
  })

  it('calls cancelInvoice when admin confirms cancellation', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    getInvoices.mockResolvedValue({
      data: { items: [createInvoice()], total: 1, page: 1, page_size: 20 },
    })
    getInvoice.mockResolvedValueOnce({ data: createInvoice({ status: 'APPLIED' }) })
    cancelInvoice.mockResolvedValueOnce({ data: createInvoice({ status: 'CANCELLED' }) })

    const wrapper = mountView()
    await flushPromises()
    await wrapper.findAll('button').find((b) => b.text().includes('common.view'))!.trigger('click')
    await flushPromises()

    const cancelBtn = wrapper.findAll('button').find((b) => b.text().includes('payment.invoice.cancel'))
    expect(cancelBtn).toBeDefined()
    await cancelBtn!.trigger('click')
    await flushPromises()

    expect(cancelInvoice).toHaveBeenCalledWith(42)
  })
})
