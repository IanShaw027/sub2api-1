import { describe, expect, it, vi } from 'vitest'

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: false,
  isAdmin: false,
  isSimpleMode: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  cachedPublicSettings: null as null | Record<string, unknown>,
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authStore,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => appStore,
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: [],
  }),
}))

vi.mock('@/composables/useNavigationLoading', () => ({
  useNavigationLoadingState: () => ({
    startNavigation: vi.fn(),
    endNavigation: vi.fn(),
    isLoading: { value: false },
  }),
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

describe('router TLS fingerprint collector route', () => {
  it('registers the collector as a public route served by this gateway', async () => {
    const { default: router, isBackendModePublicRouteAllowed } = await import('@/router')
    const route = router.getRoutes().find((record) => record.name === 'TLSFingerprintCollector')

    expect(route?.path).toBe('/tls-fingerprint-collector')
    expect(route?.meta.requiresAuth).toBe(false)
    expect(route?.meta.title).toBe('TLS Fingerprint Collector')
    expect(isBackendModePublicRouteAllowed('/tls-fingerprint-collector', false)).toBe(true)
  })
})
