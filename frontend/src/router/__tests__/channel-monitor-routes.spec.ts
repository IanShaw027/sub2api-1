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
  publicSettingsLoaded: true,
  fetchPublicSettings: vi.fn(),
  cachedPublicSettings: {
    channel_monitor_enabled: false,
    available_channels_enabled: false,
  },
}))

const adminComplianceStore = vi.hoisted(() => ({
  initialized: true,
  required: false,
  shouldShow: false,
  fetchStatus: vi.fn(),
  requireAcknowledgement: vi.fn(),
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

vi.mock('@/stores/adminCompliance', () => ({
  useAdminComplianceStore: () => adminComplianceStore,
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
    appStore.publicSettingsLoaded = true
    appStore.fetchPublicSettings.mockReset()
    appStore.cachedPublicSettings = { channel_monitor_enabled: false, available_channels_enabled: false }
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

  it('marks available channels as requiring the available channels feature', async () => {
    const { default: router } = await import('@/router')

    expect(router.getRoutes().find((route) => route.path === '/available-channels')?.meta.requiresAvailableChannels).toBe(true)
  })

  it('redirects direct available channels navigation when the feature is disabled', async () => {
    authStore.isAdmin = false
    const { default: router } = await import('@/router')

    await router.push('/available-channels')

    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('loads public settings before evaluating direct available channels navigation', async () => {
    authStore.isAdmin = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = { channel_monitor_enabled: false }
    appStore.fetchPublicSettings.mockImplementation(async () => {
      appStore.publicSettingsLoaded = true
      appStore.cachedPublicSettings = {
        channel_monitor_enabled: false,
        available_channels_enabled: true,
      }
      return appStore.cachedPublicSettings
    })
    const { default: router } = await import('@/router')

    await router.push('/available-channels')

    expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.path).toBe('/available-channels')
  })

  it('does not admit a disabled monitor route before public settings resolve on cold load', async () => {
    authStore.isAdmin = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockImplementation(async () => {
      appStore.publicSettingsLoaded = true
      appStore.cachedPublicSettings = {
        channel_monitor_enabled: false,
        available_channels_enabled: false,
      }
      return appStore.cachedPublicSettings
    })
    const { default: router } = await import('@/router')

    await router.push('/monitor')

    expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })

  it('does not admit a disabled payment route before public settings resolve on cold load', async () => {
    authStore.isAdmin = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockImplementation(async () => {
      appStore.publicSettingsLoaded = true
      appStore.cachedPublicSettings = {
        channel_monitor_enabled: true,
        available_channels_enabled: false,
        payment_enabled: false,
      }
      return appStore.cachedPublicSettings
    })
    const { default: router } = await import('@/router')

    await router.push('/purchase')

    expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1)
    expect(router.currentRoute.value.path).toBe('/dashboard')
  })
})
