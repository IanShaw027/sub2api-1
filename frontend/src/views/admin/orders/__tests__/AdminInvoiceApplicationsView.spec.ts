import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { InvoiceApplication } from '@/types/payment'
import AdminInvoiceApplicationsView from '../AdminInvoiceApplicationsView.vue'

const { getInvoices, getInvoice, adminPaymentAPI } = vi.hoisted(() => {
  const getInvoices = vi.fn()
  const getInvoice = vi.fn()
  return {
    getInvoices,
    getInvoice,
    adminPaymentAPI: {
      getInvoices,
      getInvoice,
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

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function createInvoice(overrides: Partial<InvoiceApplication> = {}): InvoiceApplication {
  return {
    id: 1,
    order_id: 101,
    user_id: 201,
    user_email: 'first@example.com',
    order_out_trade_no: 'invoice-1',
    payment_type: 'alipay',
    provider_instance_id: 'provider-1',
    provider_key: 'alipay',
    status: 'APPLIED',
    invoice_amount: 120,
    title: 'Invoice 1',
    tax_number: '1234567890',
    email: 'first@example.com',
    contact_name: 'First User',
    contact_phone: '1234567890',
    created_at: '2026-05-22T00:00:00Z',
    updated_at: '2026-05-22T00:00:00Z',
    ...overrides,
  }
}

describe('AdminInvoiceApplicationsView request races', () => {
  beforeEach(() => {
    getInvoices.mockReset()
    getInvoice.mockReset()
  })

  it('keeps the newest invoice list response when filters change before the first request returns', async () => {
    const firstResponse = createDeferred<{ data: { items: InvoiceApplication[]; total: number } }>()
    const secondResponse = createDeferred<{ data: { items: InvoiceApplication[]; total: number } }>()
    getInvoices.mockImplementationOnce(() => firstResponse.promise)
    getInvoices.mockImplementationOnce(() => secondResponse.promise)

    const wrapper = mount(AdminInvoiceApplicationsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    await wrapper.get('select').setValue('ISSUED')
    expect(getInvoices).toHaveBeenCalledTimes(2)

    secondResponse.resolve({
      data: {
        items: [createInvoice({ id: 2, order_out_trade_no: 'invoice-new', status: 'ISSUED', email: 'second@example.com', user_email: 'second@example.com' })],
        total: 1,
      },
    })
    await flushPromises()

    expect(wrapper.get('tbody').text()).toContain('invoice-new')

    firstResponse.resolve({
      data: {
        items: [createInvoice({ id: 1, order_out_trade_no: 'invoice-old', email: 'old@example.com', user_email: 'old@example.com' })],
        total: 1,
      },
    })
    await flushPromises()

    expect(wrapper.get('tbody').text()).toContain('invoice-new')
  })

  it('keeps the newest invoice detail when two rows are opened back to back', async () => {
    getInvoices.mockResolvedValue({
      data: {
        items: [
          createInvoice({ id: 1, order_out_trade_no: 'invoice-old', email: 'old@example.com', user_email: 'old@example.com' }),
          createInvoice({ id: 2, order_out_trade_no: 'invoice-new', email: 'second@example.com', user_email: 'second@example.com' }),
        ],
        total: 2,
      },
    })

    const firstDetail = createDeferred<{ data: InvoiceApplication }>()
    const secondDetail = createDeferred<{ data: InvoiceApplication }>()
    getInvoice.mockImplementationOnce(() => firstDetail.promise)
    getInvoice.mockImplementationOnce(() => secondDetail.promise)

    const wrapper = mount(AdminInvoiceApplicationsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    const viewButtons = wrapper.findAll('tbody button')
    expect(viewButtons).toHaveLength(2)

    await viewButtons[0].trigger('click')
    await viewButtons[1].trigger('click')

    expect(getInvoice).toHaveBeenNthCalledWith(1, 1)
    expect(getInvoice).toHaveBeenNthCalledWith(2, 2)

    secondDetail.resolve({
      data: createInvoice({ id: 2, order_out_trade_no: 'invoice-new', email: 'second@example.com', user_email: 'second@example.com' }),
    })
    await flushPromises()

    const dialog = wrapper.get('[data-test="dialog"]')
    expect(dialog.text()).toContain('invoice-new')
    expect(dialog.text()).not.toContain('invoice-old')

    firstDetail.resolve({
      data: createInvoice({ id: 1, order_out_trade_no: 'invoice-old', email: 'old@example.com', user_email: 'old@example.com' }),
    })
    await flushPromises()

    expect(dialog.text()).toContain('invoice-new')
    expect(dialog.text()).not.toContain('invoice-old')
  })

  it('does not keep a reopened invoice dialog submitting or overwrite it with stale detail and upload responses', async () => {
    const firstDetail = createDeferred<{ data: InvoiceApplication }>()
    const secondDetail = createDeferred<{ data: InvoiceApplication }>()
    getInvoices.mockResolvedValue({
      data: {
        items: [
          createInvoice({ id: 1, order_out_trade_no: 'invoice-old', email: 'old@example.com', user_email: 'old@example.com' }),
          createInvoice({ id: 2, order_out_trade_no: 'invoice-new', email: 'new@example.com', user_email: 'new@example.com' }),
        ],
        total: 2,
      },
    })
    getInvoice.mockImplementationOnce(() => firstDetail.promise)
    getInvoice.mockImplementationOnce(() => secondDetail.promise)
    getInvoice.mockResolvedValueOnce({
      data: createInvoice({ id: 2, order_out_trade_no: 'invoice-new', email: 'new@example.com', user_email: 'new@example.com' }),
    })

    const uploadRequest = createDeferred<{ data: InvoiceApplication }>()
    adminPaymentAPI.uploadInvoiceFile.mockReturnValueOnce(uploadRequest.promise)

    const wrapper = mount(AdminInvoiceApplicationsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          BaseDialog: BaseDialogStub,
          Pagination: PaginationStub,
          Select: SelectStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()

    const viewButtons = wrapper.findAll('tbody button')
    await viewButtons[0].trigger('click')
    await viewButtons[1].trigger('click')

    secondDetail.resolve({
      data: createInvoice({ id: 2, order_out_trade_no: 'invoice-new', email: 'new@example.com', user_email: 'new@example.com' }),
    })
    await flushPromises()

    const firstFileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(firstFileInput.element, 'files', {
      value: [new File(['invoice'], 'invoice.pdf', { type: 'application/pdf' })],
      configurable: true,
    })
    await firstFileInput.trigger('change')
    await wrapper.get('[data-test="dialog"] button.btn-primary').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="dialog"] button.btn-primary').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-test="dialog"] button.btn-secondary').trigger('click')
    await flushPromises()

    await viewButtons[1].trigger('click')
    await flushPromises()

    const reopenedDialog = wrapper.get('[data-test="dialog"]')
    expect(reopenedDialog.text()).toContain('invoice-new')
    expect(reopenedDialog.text()).not.toContain('invoice-old')

    const secondFileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(secondFileInput.element, 'files', {
      value: [new File(['invoice'], 'invoice-2.pdf', { type: 'application/pdf' })],
      configurable: true,
    })
    await secondFileInput.trigger('change')
    expect(wrapper.get('[data-test="dialog"] button.btn-primary').attributes('disabled')).toBeUndefined()

    firstDetail.resolve({
      data: createInvoice({ id: 1, order_out_trade_no: 'invoice-old', email: 'old@example.com', user_email: 'old@example.com' }),
    })
    await flushPromises()

    expect(wrapper.get('[data-test="dialog"]').text()).toContain('invoice-new')
    expect(wrapper.get('[data-test="dialog"]').text()).not.toContain('invoice-old')

    uploadRequest.resolve({
      data: createInvoice({ id: 1, order_out_trade_no: 'invoice-old', email: 'old@example.com', user_email: 'old@example.com', status: 'ISSUED' }),
    })
    await flushPromises()

    expect(wrapper.get('[data-test="dialog"]').text()).toContain('invoice-new')
    expect(wrapper.get('[data-test="dialog"]').text()).not.toContain('invoice-old')
  })
})
