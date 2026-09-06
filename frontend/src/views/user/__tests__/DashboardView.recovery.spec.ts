import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import DashboardView from '../DashboardView.vue'

const mocks = vi.hoisted(() => ({
  refreshUser: vi.fn(),
  getDashboardStats: vi.fn(),
  getDashboardTrend: vi.fn(),
  getDashboardModels: vi.fn(),
  getByDateRange: vi.fn(),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: false,
    user: { balance: 10, email: 'user@example.com' },
    refreshUser: mocks.refreshUser,
  }),
}))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('@/api/usage', () => ({ usageAPI: mocks }))
vi.mock('@/api/user', () => ({
  getMyRPMStatus: vi.fn().mockResolvedValue({}),
  getMyPlatformQuotas: vi.fn().mockResolvedValue({ platform_quotas: [] }),
}))

const stats = {
  total_api_keys: 2, active_api_keys: 1, today_requests: 3,
  average_duration_ms: 100, rpm: 4,
}

describe('dashboard request failure recovery', () => {
  let wrapper: VueWrapper | undefined

  beforeEach(() => {
    vi.resetAllMocks()
    mocks.refreshUser.mockResolvedValue(undefined)
    mocks.getDashboardStats.mockResolvedValue(stats)
    mocks.getDashboardTrend.mockResolvedValue({ trend: [] })
    mocks.getDashboardModels.mockResolvedValue({ models: [] })
    mocks.getByDateRange.mockResolvedValue({ items: [] })
    vi.spyOn(console, 'error').mockImplementation(() => {})
    vi.spyOn(console, 'warn').mockImplementation(() => {})
  })

  afterEach(() => {
    wrapper?.unmount()
    vi.restoreAllMocks()
  })

  async function mountDashboard() {
    wrapper = mount(DashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          DateRangePicker: true,
          UserDashboardStats: true,
          UserDashboardCharts: true,
          UserDashboardRecentUsage: true,
          UserDashboardQuickActions: true,
          UserBalanceHistoryModal: true,
        },
      },
    })
    await flushPromises()
    return wrapper
  }

  it.each(['stats', 'profile'] as const)('offers retry after initial %s failure', async (failure) => {
    const request = failure === 'stats' ? mocks.getDashboardStats : mocks.refreshUser
    request.mockRejectedValueOnce(new Error('temporary failure'))
    const page = await mountDashboard()
    const alert = page.get('[role="alert"]')
    expect(alert.text()).toContain('dashboard.loadFailed')
    await alert.get('button').trigger('click')
    await flushPromises()
    expect(mocks.getDashboardStats).toHaveBeenCalledTimes(2)
    expect(page.find('[role="alert"]').exists()).toBe(false)
    expect(page.find('.dash-hero').exists()).toBe(true)
  })

  it('keeps loaded statistics visible during and after a failed refresh', async () => {
    const page = await mountDashboard()
    let reject!: (reason: unknown) => void
    mocks.getDashboardStats.mockReturnValueOnce(new Promise((_, rejectPromise) => { reject = rejectPromise }))
    await page.get('.dash-refresh-btn').trigger('click')
    expect(page.find('.dash-hero').exists()).toBe(true)
    reject(new Error('refresh failed'))
    await flushPromises()
    expect(page.find('.dash-hero').exists()).toBe(true)
    expect(page.get('[role="alert"]').text()).toContain('dashboard.loadFailed')
    expect(page.getComponent({ name: 'UserDashboardStats' }).props('stats')).toEqual(stats)
  })

  it('refreshes charts and recent usage with the selected range', async () => {
    const page = await mountDashboard()
    const picker = page.getComponent({ name: 'DateRangePicker' })
    picker.vm.$emit('update:startDate', '2026-08-01')
    picker.vm.$emit('update:endDate', '2026-08-31')
    picker.vm.$emit('change')
    await flushPromises()
    expect(mocks.getDashboardModels).toHaveBeenLastCalledWith({ start_date: '2026-08-01', end_date: '2026-08-31' })
    expect(mocks.getByDateRange).toHaveBeenLastCalledWith('2026-08-01', '2026-08-31')
    expect(page.getComponent({ name: 'UserDashboardCharts' }).props('rangeLabel')).toBe('2026-08-01 - 2026-08-31')
    expect(page.getComponent({ name: 'UserDashboardRecentUsage' }).props('rangeLabel')).toBe('2026-08-01 - 2026-08-31')
  })

  it('does not label failed charts or usage as an empty successful response', async () => {
    mocks.getDashboardTrend.mockRejectedValueOnce(new Error('trend failed'))
    mocks.getByDateRange.mockRejectedValueOnce(new Error('usage failed'))
    const page = await mountDashboard()
    const charts = page.getComponent({ name: 'UserDashboardCharts' })
    const recent = page.getComponent({ name: 'UserDashboardRecentUsage' })
    expect(charts.props('error')).toBe(true)
    expect(recent.props('error')).toBe(true)
    charts.vm.$emit('retry')
    recent.vm.$emit('retry')
    await flushPromises()
    expect(charts.props('error')).toBe(false)
    expect(recent.props('error')).toBe(false)
  })

  it('ignores older chart and recent usage results after a newer range finishes', async () => {
    const page = await mountDashboard()
    let resolveModels!: (value: unknown) => void
    let resolveUsage!: (value: unknown) => void
    mocks.getDashboardModels.mockReturnValueOnce(new Promise(resolve => { resolveModels = resolve }))
    mocks.getByDateRange.mockReturnValueOnce(new Promise(resolve => { resolveUsage = resolve }))
    const picker = page.getComponent({ name: 'DateRangePicker' })
    picker.vm.$emit('update:startDate', '2026-08-01')
    picker.vm.$emit('change')
    await flushPromises()
    mocks.getDashboardModels.mockResolvedValueOnce({ models: [{ model: 'new-model', actual_cost: 25 }] })
    mocks.getByDateRange.mockResolvedValueOnce({ items: [{ id: 200 }] })
    picker.vm.$emit('update:startDate', '2026-08-02')
    picker.vm.$emit('change')
    await flushPromises()
    resolveModels({ models: [{ model: 'old-model', actual_cost: 100 }] })
    resolveUsage({ items: [{ id: 100 }] })
    await flushPromises()
    expect(page.getComponent({ name: 'UserDashboardCharts' }).props('models')).toEqual([{ model: 'new-model', actual_cost: 25 }])
    expect(page.getComponent({ name: 'UserDashboardRecentUsage' }).props('data')).toEqual([{ id: 200 }])
  })
})
