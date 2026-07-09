import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'
import { formatPaymentAmount } from '@/components/payment/currency'

const apiMock = vi.hoisted(() => ({
  getInvoice: vi.fn(),
  cancelInvoice: vi.fn(),
  getInvoiceDownloadURL: vi.fn(async () => ({ data: { url: 'https://x' } })),
}))

vi.mock('@/api/payment', () => ({ paymentAPI: apiMock }))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  const dict: Record<string, string> = {
    'common.close': '关闭',
    'common.back': '返回',
    'common.cancel': '取消',
    'common.success': '成功',
    'common.error': '错误',
    'common.processing': '处理中',
    'payment.invoice.detailPage.relatedOrders': '关联订单',
    'payment.invoice.detailPage.fileWaiting': '等待开具',
    'payment.invoice.list.colOrderCount': '订单数',
    'payment.invoice.applied': '申请中',
    'payment.invoice.issued': '已开具',
    'payment.invoice.cancelled': '已取消',
    'payment.invoice.title': '抬头',
    'payment.invoice.amount': '金额',
    'payment.invoice.status': '状态',
    'payment.invoice.email': '邮箱',
    'payment.invoice.taxNumber': '税号',
    'payment.invoice.contactName': '联系人',
    'payment.invoice.contactPhone': '电话',
    'payment.invoice.fileName': '文件',
    'payment.invoice.cancel': '取消申请',
    'payment.invoice.download': '下载发票',
    'payment.orders.orderNo': '订单号',
    'payment.orders.payAmount': '金额',
    'payment.orders.paymentMethod': '支付方式',
    'payment.orders.createdAt': '创建时间',
  }
  return {
    ...actual,
    useI18n: () => ({ t: (key: string, _fallback?: unknown) => dict[key] ?? key }),
  }
})

import UserInvoiceDetailView from '../UserInvoiceDetailView.vue'

const fakeInvoice = (overrides: Record<string, unknown> = {}) => ({
  id: 42, user_id: 1, user_email: 'u@x.com',
  status: 'APPLIED', invoice_amount: 200, order_count: 2,
  title: 'ACME', tax_number: 'TX', email: 'b@a.com',
  contact_name: '', contact_phone: '',
  created_at: '2026-06-08T00:00:00Z', updated_at: '2026-06-08T00:00:00Z',
  orders: [
    { order_id: 1, out_trade_no: 'A001', pay_amount_snapshot: 100, payment_type: 'wxpay', created_at: '2026-06-08T00:00:00Z' },
    { order_id: 2, out_trade_no: 'A002', pay_amount_snapshot: 100, payment_type: 'wxpay', created_at: '2026-06-08T00:00:00Z' },
  ],
  ...overrides,
})

const i18n = createI18n({ legacy: false, locale: 'zh', messages: { zh: {} } })

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/orders/invoices', name: 'MyInvoices', component: { template: '<div />' } },
      { path: '/orders/invoices/:id', name: 'MyInvoiceDetail', component: UserInvoiceDetailView },
    ],
  })
  router.push('/orders/invoices/42')
  await router.isReady()
  return mount(UserInvoiceDetailView, {
    global: {
      plugins: [router, i18n, createPinia()],
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        OrdersTabBar: true,
      },
    },
  })
}

describe('UserInvoiceDetailView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('shows cancel button when APPLIED, hides download', async () => {
    apiMock.getInvoice.mockResolvedValueOnce({ data: fakeInvoice({ status: 'APPLIED' }) })
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('取消申请')
    expect(wrapper.text()).not.toContain('下载发票')
  })

  it('shows download when ISSUED, hides cancel', async () => {
    apiMock.getInvoice.mockResolvedValueOnce({ data: fakeInvoice({ status: 'ISSUED', file_name: 'i.pdf' }) })
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('下载发票')
    expect(wrapper.text()).not.toContain('取消申请')
  })

  it('renders related orders table', async () => {
    apiMock.getInvoice.mockResolvedValueOnce({ data: fakeInvoice() })
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('A001')
    expect(wrapper.text()).toContain('A002')
  })

  it('formats invoice and order amounts with invoice currency instead of hardcoded cny symbol', async () => {
    apiMock.getInvoice.mockResolvedValueOnce({
      data: fakeInvoice({
        invoice_amount: 200,
        currency: 'USD',
        orders: [
          { order_id: 1, out_trade_no: 'A001', pay_amount_snapshot: 123.45, payment_type: 'stripe', created_at: '2026-06-08T00:00:00Z' },
        ],
      }),
    })
    const wrapper = await mountView()
    await flushPromises()

    expect(wrapper.text()).toContain(formatPaymentAmount(200, 'USD'))
    expect(wrapper.text()).toContain(formatPaymentAmount(123.45, 'USD'))
    expect(wrapper.text()).not.toContain('¥200.00')
    expect(wrapper.text()).not.toContain('¥123.45')
  })
})
