import { beforeEach, describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: true,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: {
    channel_monitor_enabled: false,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({ customMenuItems: [] }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))


vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: (key: string) => ({
        'nav.channelStatus': 'Channel Monitor',
        'admin.dashboard.title': 'Admin Dashboard',
      }[key] ?? key),
    },
  },
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

describe('channel monitor routes', () => {
  beforeEach(() => {
    authStore.checkAuth.mockClear()
    authStore.isAuthenticated = true
    authStore.isAdmin = true
    authStore.isSimpleMode = false
    appStore.backendModeEnabled = false
    appStore.cachedPublicSettings = { channel_monitor_enabled: false }
    window.scrollTo = vi.fn()
  })

  it('marks user and admin monitor routes as requiring the channel monitor feature', async () => {
    const { default: router } = await import('@/router')

    expect(router.getRoutes().find((route) => route.path === '/monitor')?.meta.requiresChannelMonitor).toBe(true)
    expect(router.getRoutes().find((route) => route.path === '/admin/channels/monitor')?.meta.requiresChannelMonitor).toBe(true)
  })

  it('redirects direct monitor navigation when the feature is disabled', async () => {
    const { default: router } = await import('@/router')

    await router.push('/monitor')

    expect(router.currentRoute.value.path).toBe('/admin/dashboard')
  })
})
