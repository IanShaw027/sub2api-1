import { describe, it, expect, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { createPinia } from 'pinia'

const translations: Record<string, string> = {
  'nav.myOrders': '我的订单',
  'nav.myInvoices': '我的发票',
  'common.all': '全部',
  'common.actions': '操作',
  'common.refresh': '刷新',
  'common.view': '查看',
  'common.cancel': '取消',
  'common.error': '错误',
  'payment.orders.createdAt': '创建时间',
  'payment.invoice.list.title': '我的发票',
  'payment.invoice.list.empty': '暂无发票',
  'payment.invoice.list.colOrderCount': '订单数',
  'payment.invoice.list.searchPlaceholder': '搜索发票…',
  'payment.invoice.applied': '申请中',
  'payment.invoice.issued': '已开具',
  'payment.invoice.cancelled': '已取消',
  'payment.invoice.status': '发票状态',
  'payment.invoice.amount': '金额',
  'payment.invoice.title': '抬头',
  'payment.invoice.download': '下载发票',
  'payment.invoice.cancel': '取消申请',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string | Record<string, unknown>) =>
        translations[key] ?? (typeof fallback === 'string' ? fallback : key),
    }),
  }
})

import UserInvoicesView from '../UserInvoicesView.vue'

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    listMyInvoices: vi.fn(async () => ({
      data: {
        items: [{
          id: 42, user_id: 1, user_email: 'u@x.com',
          status: 'ISSUED', invoice_amount: 318, currency: 'USD', order_count: 3,
          title: 'ACME', tax_number: 'TX', email: 'b@a.com',
          contact_name: '', contact_phone: '',
          file_name: 'inv.pdf',
          created_at: '2026-06-08T00:00:00Z', updated_at: '2026-06-08T00:00:00Z',
        }],
        total: 1, page: 1, page_size: 20,
      },
    })),
    cancelInvoice: vi.fn(),
    getInvoiceDownloadURL: vi.fn(async () => ({ data: { url: 'https://files.example.com/x' } })),
  },
}))

const i18n = createI18n({
  legacy: false, locale: 'zh',
  messages: { zh: {
    nav: { myOrders: '我的订单', myInvoices: '我的发票' },
    common: { all: '全部', actions: '操作', refresh: '刷新', view: '查看', cancel: '取消', error: '错误' },
    payment: {
      orders: { createdAt: '创建时间' },
      invoice: {
        list: { title: '我的发票', empty: '暂无发票', colOrderCount: '订单数' },
        applied: '申请中', issued: '已开具', cancelled: '已取消',
        status: '发票状态', amount: '金额', title: '抬头', download: '下载发票', cancel: '取消申请',
        errors: {},
      },
    },
  } },
})

async function mountView() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/orders', component: { template: '<div />' } },
      { path: '/orders/invoices', component: UserInvoicesView },
      { path: '/orders/invoices/:id', name: 'MyInvoiceDetail', component: { template: '<div />' } },
    ],
  })
  await router.push('/orders/invoices')
  await router.isReady()
  return mount(UserInvoicesView, {
    global: {
      plugins: [router, i18n, createPinia()],
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        OrdersTabBar: true,
        Pagination: true,
        Select: { props: ['modelValue', 'options'], template: '<select />' },
        Icon: true,
      },
    },
  })
}

describe('UserInvoicesView', () => {
  beforeEach(() => vi.clearAllMocks())

  it('renders list with order_count and amount', async () => {
    const wrapper = await mountView()
    await flushPromises()
    expect(wrapper.text()).toContain('ACME')
    expect(wrapper.text()).toContain('3')          // order_count
    expect(wrapper.text()).toContain('$318.00')    // invoice_amount with invoice currency
    expect(wrapper.text()).not.toContain('¥318.00')
    expect(wrapper.text()).toContain('已开具')      // status label
  })
})
