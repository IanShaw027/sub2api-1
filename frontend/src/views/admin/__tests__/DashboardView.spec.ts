import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

import type { DashboardStats } from '@/types'
import DashboardView from '../DashboardView.vue'

const { getSnapshotV2, getUserUsageTrend, getUserSpendingRanking } = vi.hoisted(() => ({
  getSnapshotV2: vi.fn(),
  getUserUsageTrend: vi.fn(),
  getUserSpendingRanking: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    dashboard: {
      getSnapshotV2,
      getUserUsageTrend,
      getUserSpendingRanking
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const createDashboardStats = (): DashboardStats => ({
  total_users: 4567,
  today_new_users: 12,
  active_users: 34,
  hourly_active_users: 5,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 100,
  active_api_keys: 80,
  total_accounts: 20,
  normal_accounts: 18,
  error_accounts: 2,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 1000,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 2500000,
  total_cost: 123.45,
  total_actual_cost: 120.12,
  total_account_cost: 118.88,
  total_balance_actual_cost: 70.1,
  total_subscription_actual_cost: 50.02,
  total_recharge_amount: 500.5,
  total_refund_amount: 10.2,
  today_requests: 200,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 125000,
  today_cost: 12.34,
  today_actual_cost: 11.11,
  today_account_cost: 10.01,
  today_balance_actual_cost: 6.6,
  today_subscription_actual_cost: 4.4,
  today_recharge_amount: 30.3,
  today_refund_amount: 1.2,
  average_duration_ms: 245,
  uptime: 0,
  rpm: 22,
  tpm: 3333
})

describe('admin DashboardView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())

    getSnapshotV2.mockReset()
    getUserUsageTrend.mockReset()
    getUserSpendingRanking.mockReset()

    getSnapshotV2.mockResolvedValue({
      stats: createDashboardStats(),
      trend: [],
      models: []
    })
    getUserUsageTrend.mockResolvedValue({
      trend: [],
      start_date: '',
      end_date: '',
      granularity: 'hour'
    })
    getUserSpendingRanking.mockResolvedValue({
      ranking: [],
      total_actual_cost: 0,
      total_requests: 0,
      total_tokens: 0,
      start_date: '',
      end_date: ''
    })
  })

  it('uses last 24 hours as default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          HelpTooltip: { template: '<div class="help-tooltip-stub"><slot name="trigger" /><slot /></div>' },
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
  })

  it('shows dashboard breakdowns as value-only tooltip triggers', async () => {
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: true,
          Select: true,
          HelpTooltip: { template: '<div class="help-tooltip-stub"><slot name="trigger" /><slot /></div>' },
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const normalized = wrapper.text().replace(/\s+/g, ' ')

    expect(normalized).toContain('$11.11/$10.01/$12.34')
    expect(normalized).toContain('$6.60/$4.40/$30.30/$1.20')
    expect(normalized).toContain('admin.dashboard.actual')
    expect(normalized).toContain('admin.dashboard.accountCost')
    expect(normalized).toContain('admin.dashboard.standard')
    expect(normalized).toContain('admin.dashboard.balanceConsumption')
    expect(normalized).toContain('admin.dashboard.subscriptionConsumption')
    expect(normalized).toContain('admin.dashboard.rechargeAmount')
    expect(normalized).toContain('admin.dashboard.refundAmount')
  })

  it('refreshes summary cards when the date range changes', async () => {
    const refreshedStats: DashboardStats = {
      ...createDashboardStats(),
      total_users: 9999,
      today_new_users: 88,
      total_requests: 4321
    }

    getSnapshotV2
      .mockResolvedValueOnce({
        stats: createDashboardStats(),
        trend: [],
        models: []
      })
      .mockResolvedValueOnce({
        stats: refreshedStats,
        trend: [],
        models: []
      })

    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          DateRangePicker: {
            name: 'DateRangePicker',
            emits: ['change'],
            template: '<div class="date-range-picker-stub" />'
          },
          Select: true,
          HelpTooltip: { template: '<div class="help-tooltip-stub"><slot name="trigger" /><slot /></div>' },
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const picker = wrapper.findComponent({ name: 'DateRangePicker' })
    picker.vm.$emit('change', {
      startDate: '2026-03-01',
      endDate: '2026-03-03',
      preset: null
    })

    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(2)
    expect(getSnapshotV2).toHaveBeenLastCalledWith(expect.objectContaining({
      start_date: '2026-03-01',
      end_date: '2026-03-03',
      granularity: 'day',
      include_stats: true
    }))

    const normalized = wrapper.text().replace(/\s+/g, ' ')
    expect(normalized).toContain('+88')
    expect(normalized).toContain('9,999')
  })
})
