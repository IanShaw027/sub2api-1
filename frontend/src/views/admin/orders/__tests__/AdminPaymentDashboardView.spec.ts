import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import AdminPaymentDashboardView from '../AdminPaymentDashboardView.vue'

const { getDashboard, adminPaymentAPI } = vi.hoisted(() => {
  const getDashboard = vi.fn()
  return {
    getDashboard,
    adminPaymentAPI: {
      getDashboard,
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
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => fallback || key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const LoadingSpinnerStub = { template: '<div />' }
const IconStub = { template: '<span />' }
const OrderStatsCardsStub = { template: '<div data-test="cards" />' }
const DailyRevenueChartStub = { template: '<div data-test="chart" />' }

function mountView() {
  return mount(AdminPaymentDashboardView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        LoadingSpinner: LoadingSpinnerStub,
        Icon: IconStub,
        OrderStatsCards: OrderStatsCardsStub,
        DailyRevenueChart: DailyRevenueChartStub,
      },
    },
  })
}

describe('AdminPaymentDashboardView', () => {
  beforeEach(() => {
    getDashboard.mockReset()
  })

  it('renders method and top-user totals without a hardcoded yuan symbol', async () => {
    getDashboard.mockResolvedValueOnce({
      data: {
        today_amount: 10,
        total_amount: 25.5,
        today_count: 2,
        total_count: 5,
        avg_amount: 5.1,
        pending_orders: 0,
        daily_series: [],
        payment_methods: [
          { type: 'stripe', amount: 103, count: 1 },
        ],
        top_users: [
          { user_id: 8, email: 'user@example.com', amount: 88 },
        ],
      },
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('103.00')
    expect(wrapper.text()).toContain('88.00')
    expect(wrapper.text()).not.toContain('¥103.00')
    expect(wrapper.text()).not.toContain('¥88.00')
  })
})
