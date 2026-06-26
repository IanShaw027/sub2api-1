import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const {
  refreshUserMock,
  getDashboardStatsMock,
  getDashboardTrendMock,
  getDashboardModelsMock,
  getByDateRangeMock,
  getMyPlatformQuotasMock,
  authStoreState,
} = vi.hoisted(() => ({
  refreshUserMock: vi.fn(),
  getDashboardStatsMock: vi.fn(),
  getDashboardTrendMock: vi.fn(),
  getDashboardModelsMock: vi.fn(),
  getByDateRangeMock: vi.fn(),
  getMyPlatformQuotasMock: vi.fn(),
  authStoreState: {
    user: {
      balance: 12.5,
      email: 'user@example.com',
    },
    isSimpleMode: false,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: authStoreState.user,
    isSimpleMode: authStoreState.isSimpleMode,
    refreshUser: refreshUserMock,
  }),
}))

vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats: (...args: any[]) => getDashboardStatsMock(...args),
    getDashboardTrend: (...args: any[]) => getDashboardTrendMock(...args),
    getDashboardModels: (...args: any[]) => getDashboardModelsMock(...args),
    getByDateRange: (...args: any[]) => getByDateRangeMock(...args),
  },
}))

vi.mock('@/api/user', () => ({
  getMyPlatformQuotas: (...args: any[]) => getMyPlatformQuotasMock(...args),
}))

import DashboardView from '../DashboardView.vue'

function mountView() {
  return mount(DashboardView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        LoadingSpinner: true,
        UserDashboardStats: true,
        UserDashboardCharts: true,
        UserDashboardRecentUsage: true,
        UserDashboardQuickActions: true,
        UserBalanceHistoryModal: true,
      },
    },
  })
}

describe('DashboardView request orchestration', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    refreshUserMock.mockResolvedValue(authStoreState.user)
    getDashboardStatsMock.mockResolvedValue({
      balance: 12.5,
      api_key_count: 2,
      active_api_key_count: 2,
      today_requests: 8,
      today_cost: 0.12,
      today_tokens: 1200,
      total_tokens: 8800,
    })
    getDashboardTrendMock.mockResolvedValue({ trend: [] })
    getDashboardModelsMock.mockResolvedValue({ models: [] })
    getByDateRangeMock.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 5, pages: 0 })
    getMyPlatformQuotasMock.mockResolvedValue({ platform_quotas: [] })
  })

  it('starts dashboard stats loading before refreshUser resolves', async () => {
    let resolveRefreshUser: ((value: unknown) => void) | null = null
    refreshUserMock.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveRefreshUser = resolve
        })
    )

    mountView()

    await Promise.resolve()

    expect(getDashboardStatsMock).toHaveBeenCalledTimes(1)

    resolveRefreshUser?.(authStoreState.user)
  })

  it('requests only five recent usage rows on mount', async () => {
    mountView()

    await Promise.resolve()

    expect(getByDateRangeMock).toHaveBeenCalledWith(
      expect.any(String),
      expect.any(String),
      undefined,
      5
    )
  })
})
