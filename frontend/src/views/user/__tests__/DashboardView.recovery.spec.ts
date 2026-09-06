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
})
