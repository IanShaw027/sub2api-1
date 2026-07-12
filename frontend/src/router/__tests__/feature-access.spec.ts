import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'

type NavigationGuard = (
  to: Record<string, any>,
  from: Record<string, any>,
  next: ReturnType<typeof vi.fn>
) => Promise<void>

const routerHarness = vi.hoisted(() => ({
  guard: null as NavigationGuard | null,
}))

const authStore = vi.hoisted(() => ({
  checkAuth: vi.fn(),
  isAuthenticated: true,
  isAdmin: false,
  isSimpleMode: false,
  hasPendingAuthSession: false,
}))

const appStore = vi.hoisted(() => ({
  siteName: 'Sub2API',
  backendModeEnabled: false,
  publicSettingsLoaded: false,
  cachedPublicSettings: null as null | {
    payment_enabled?: boolean
    risk_control_enabled?: boolean
    custom_menu_items?: []
  },
  fetchPublicSettings: vi.fn(),
}))

const adminComplianceStore = vi.hoisted(() => ({
  initialized: true,
  required: false,
  unavailable: false,
  shouldShow: false,
  fetchStatus: vi.fn(),
  requireAcknowledgement: vi.fn((metadata?: Record<string, string>) => {
    void metadata
    adminComplianceStore.required = true
    adminComplianceStore.unavailable = false
    adminComplianceStore.shouldShow = true
  }),
  markStatusUnavailable: vi.fn(() => {
    adminComplianceStore.required = false
    adminComplianceStore.unavailable = true
    adminComplianceStore.shouldShow = true
  }),
}))

const navigationLoading = vi.hoisted(() => ({
  startNavigation: vi.fn(),
  endNavigation: vi.fn(),
  isLoading: { value: false },
}))

vi.mock('vue-router', () => ({
  createWebHistory: vi.fn(() => ({})),
  createRouter: vi.fn(() => ({
    beforeEach: vi.fn((guard: NavigationGuard) => {
      routerHarness.guard = guard
    }),
    afterEach: vi.fn(),
    onError: vi.fn(),
  })),
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
  useNavigationLoadingState: () => navigationLoading,
}))

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void
  const promise = new Promise<T>((resolvePromise) => {
    resolve = resolvePromise
  })
  return { promise, resolve }
}

function runGuard(meta: Record<string, unknown>, path: string) {
  if (!routerHarness.guard) {
    throw new Error('router guard was not registered')
  }

  const next = vi.fn()
  const navigation = routerHarness.guard(
    {
      path,
      fullPath: path,
      name: 'FeatureRoute',
      params: {},
      meta: { requiresAuth: true, ...meta },
    },
    {},
    next
  )
  return { navigation, next }
}

describe('feature route guard', () => {
  beforeAll(async () => {
    await import('@/router')
  })

  beforeEach(() => {
    authStore.isAuthenticated = true
    authStore.isAdmin = false
    authStore.isSimpleMode = false
    appStore.publicSettingsLoaded = false
    appStore.cachedPublicSettings = null
    appStore.fetchPublicSettings.mockReset()
    adminComplianceStore.initialized = true
    adminComplianceStore.required = false
    adminComplianceStore.unavailable = false
    adminComplianceStore.shouldShow = false
    adminComplianceStore.fetchStatus.mockReset()
    adminComplianceStore.requireAcknowledgement.mockReset()
    adminComplianceStore.markStatusUnavailable.mockReset()
    adminComplianceStore.requireAcknowledgement.mockImplementation((metadata?: Record<string, string>) => {
      void metadata
      adminComplianceStore.required = true
      adminComplianceStore.unavailable = false
      adminComplianceStore.shouldShow = true
    })
    adminComplianceStore.markStatusUnavailable.mockImplementation(() => {
      adminComplianceStore.required = false
      adminComplianceStore.unavailable = true
      adminComplianceStore.shouldShow = true
    })
    navigationLoading.startNavigation.mockReset()
    navigationLoading.endNavigation.mockReset()
  })

  it('waits for admin compliance status before entering an admin route', async () => {
    authStore.isAdmin = true
    adminComplianceStore.initialized = false
    const deferred = createDeferred<void>()
    adminComplianceStore.fetchStatus.mockReturnValue(deferred.promise)

    const { navigation, next } = runGuard(
      { requiresAdmin: true },
      '/admin/users',
    )

    await vi.waitFor(() =>
      expect(adminComplianceStore.fetchStatus).toHaveBeenCalledTimes(1),
    )
    expect(next).not.toHaveBeenCalled()

    deferred.resolve()
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
    expect(navigationLoading.endNavigation).not.toHaveBeenCalled()
  })

  it('hard-blocks admin navigation when compliance acknowledgement is required', async () => {
    authStore.isAdmin = true
    adminComplianceStore.initialized = false
    const metadata = { version: 'v2026.06.10' }
    adminComplianceStore.fetchStatus.mockRejectedValue({
      status: 423,
      code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
      metadata,
    })

    const { navigation, next } = runGuard(
      { requiresAdmin: true },
      '/admin/users',
    )
    await navigation

    expect(adminComplianceStore.requireAcknowledgement).toHaveBeenCalledWith(metadata)
    expect(navigationLoading.endNavigation).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith(false)
  })

  it('fail-closes admin navigation when compliance status fetch fails non-423', async () => {
    authStore.isAdmin = true
    adminComplianceStore.initialized = false
    adminComplianceStore.fetchStatus.mockRejectedValue(new Error('network down'))

    const { navigation, next } = runGuard(
      { requiresAdmin: true },
      '/admin/users',
    )
    await navigation

    expect(adminComplianceStore.markStatusUnavailable).toHaveBeenCalledOnce()
    expect(adminComplianceStore.requireAcknowledgement).not.toHaveBeenCalled()
    expect(navigationLoading.endNavigation).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith(false)
  })

  it('waits for the first public-settings request before deciding payment access', async () => {
    const deferred = createDeferred<{ payment_enabled: boolean }>()
    appStore.fetchPublicSettings.mockImplementation(async () => {
      const settings = await deferred.promise
      appStore.cachedPublicSettings = settings
      appStore.publicSettingsLoaded = true
      return settings
    })

    const { navigation, next } = runGuard({ requiresPayment: true }, '/purchase')

    await vi.waitFor(() => expect(appStore.fetchPublicSettings).toHaveBeenCalledTimes(1))
    expect(next).not.toHaveBeenCalled()

    deferred.resolve({ payment_enabled: true })
    await navigation
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, '/purchase'],
    ['risk control', { requiresRiskControl: true }, '/admin/risk-control'],
  ])('does not treat a failed %s settings load as explicitly disabled', async (_name, meta, path) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.fetchPublicSettings.mockResolvedValue(null)

    const { navigation, next } = runGuard(meta, path)
    await navigation

    expect(appStore.publicSettingsLoaded).toBe(false)
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith()
  })

  it.each([
    ['payment', { requiresPayment: true }, { payment_enabled: false }, '/dashboard'],
    [
      'risk control',
      { requiresRiskControl: true },
      { risk_control_enabled: false },
      '/admin/settings',
    ],
  ])('redirects when loaded settings explicitly disable %s', async (_name, meta, settings, target) => {
    authStore.isAdmin = meta.requiresRiskControl === true
    appStore.cachedPublicSettings = settings
    appStore.publicSettingsLoaded = true

    const { navigation, next } = runGuard(meta, '/feature')
    await navigation

    expect(appStore.fetchPublicSettings).not.toHaveBeenCalled()
    expect(next).toHaveBeenCalledOnce()
    expect(next).toHaveBeenCalledWith(target)
  })
})
