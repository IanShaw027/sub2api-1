import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, shallowMount } from '@vue/test-utils'
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
  total_users: 0,
  today_new_users: 0,
  active_users: 0,
  hourly_active_users: 0,
  stats_updated_at: '',
  stats_stale: false,
  total_api_keys: 0,
  active_api_keys: 0,
  total_accounts: 0,
  normal_accounts: 0,
  error_accounts: 0,
  ratelimit_accounts: 0,
  overload_accounts: 0,
  total_requests: 0,
  total_input_tokens: 0,
  total_output_tokens: 0,
  total_cache_creation_tokens: 0,
  total_cache_read_tokens: 0,
  total_tokens: 0,
  total_cost: 0,
  total_actual_cost: 0,
  total_account_cost: 0,
  today_requests: 0,
  today_input_tokens: 0,
  today_output_tokens: 0,
  today_cache_creation_tokens: 0,
  today_cache_read_tokens: 0,
  today_tokens: 0,
  today_cost: 0,
  today_actual_cost: 0,
  today_account_cost: 0,
  total_balance_actual_cost: 0,
  today_balance_actual_cost: 0,
  total_subscription_actual_cost: 0,
  today_subscription_actual_cost: 0,
  total_recharge_amount: 0,
  today_recharge_amount: 0,
  total_refund_amount: 0,
  today_refund_amount: 0,
  average_duration_ms: 0,
  uptime: 0,
  rpm: 0,
  tpm: 0
})

describe('admin DashboardView', () => {
  it('compares range totals, not today fields, and keeps Hero bars on requests', async () => {
    setActivePinia(createPinia())
    getSnapshotV2.mockImplementation(params => Promise.resolve({
      stats: { ...createDashboardStats(), total_requests: params.include_trend ? 120 : 60, today_requests: 999 },
      trend: [{ date: '2026-08-01', requests: 1, total_tokens: 9 }, { date: '2026-08-02', requests: 9, total_tokens: 1 }],
      models: []
    }))
    const wrapper = shallowMount(DashboardView, { global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } } })
    await flushPromises()
    const hero = wrapper.findComponent({ name: 'DashHeroSection' })
    expect(hero.props('requestsDelta').text).toContain('100%')
    const originalBars = hero.props('trendBars')
    const chart = wrapper.findComponent({ name: 'DashTrendDistRow' })
    chart.vm.$emit('update:trendMetric', 'tokens')
    await flushPromises()
    expect(hero.props('trendBars')).toEqual(originalBars)
    expect(chart.props('trendBars')[0].height).toBe('100%')
    expect(hero.props('trendBars')[1].height).toBe('100%')
    wrapper.unmount()
  })
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

  it('shows a skeleton only before the first snapshot and retains data while refreshing', async () => {
    let resolveSnapshot!: (value: unknown) => void
    getSnapshotV2.mockImplementationOnce(() => new Promise(resolve => { resolveSnapshot = resolve }))
    const wrapper = shallowMount(DashboardView, {
      global: { stubs: { AppLayout: { template: '<div><slot /></div>' } } }
    })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'DashboardSkeleton' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'DashHeroSection' }).exists()).toBe(false)
    resolveSnapshot({ stats: createDashboardStats(), trend: [], models: [] })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'DashboardSkeleton' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'DashHeroSection' }).exists()).toBe(true)
    getSnapshotV2.mockImplementationOnce(() => new Promise(resolve => { resolveSnapshot = resolve }))
    wrapper.findComponent({ name: 'DashHeroSection' }).vm.$emit('loadDashboardStats')
    await flushPromises()
    expect(getSnapshotV2).toHaveBeenCalledTimes(4)
    expect(wrapper.findComponent({ name: 'DashboardSkeleton' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'DashHeroSection' }).exists()).toBe(true)
    resolveSnapshot({ stats: createDashboardStats(), trend: [], models: [] })
    await flushPromises()
    wrapper.unmount()
  })

  it('uses last 24 hours as default dashboard range', async () => {
    mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          HelpTooltip: { template: '<div><slot name="trigger" /><slot /></div>' },
          DateRangePicker: true,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()

    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)

    expect(getSnapshotV2).toHaveBeenCalledTimes(2)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))
  })

  it('reloads range-scoped summary stats when the date range changes', async () => {
    const dateRangePicker = {
      template: '<button data-test="date-range" @click="$emit(\'update:startDate\', \'2026-08-01\'); $emit(\'update:endDate\', \'2026-08-07\'); $emit(\'change\', { startDate: \'2026-08-01\', endDate: \'2026-08-07\', preset: null })" />'
    }
    const wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          LoadingSpinner: true,
          Icon: true,
          HelpTooltip: { template: '<div><slot name="trigger" /><slot /></div>' },
          DateRangePicker: dateRangePicker,
          Select: true,
          ModelDistributionChart: true,
          TokenUsageTrend: true,
          Line: true
        }
      }
    })

    await flushPromises()
    expect(getSnapshotV2).toHaveBeenCalledTimes(2)

    await wrapper.get('[data-test="date-range"]').trigger('click')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(4)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-08-01',
      end_date: '2026-08-07',
      include_stats: true
    }))
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: '2026-07-25', end_date: '2026-07-31', include_trend: false
    }))
    expect(getUserUsageTrend).toHaveBeenLastCalledWith(expect.objectContaining({ start_date: '2026-08-01', end_date: '2026-08-07' }))
    expect(getUserSpendingRanking).toHaveBeenLastCalledWith(expect.objectContaining({ start_date: '2026-08-01', end_date: '2026-08-07' }))
  })
})
