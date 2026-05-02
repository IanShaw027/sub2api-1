import { beforeEach, describe, expect, it, vi } from 'vitest'

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
  publicSettingsLoaded: true,
  fetchPublicSettings: vi.fn(),
  cachedPublicSettings: {
    channel_monitor_enabled: false,
    available_channels_enabled: false,
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

vi.mock('@/composables/useRoutePrefetch', () => ({
  useRoutePrefetch: () => ({
    triggerPrefetch: vi.fn(),
    cancelPendingPrefetch: vi.fn(),
    resetPrefetchState: vi.fn(),
  }),
}))

vi.mock('@/i18n', () => ({
  i18n: {
    global: {
      t: (key: string) => key,
    },
  },
}))

describe('skill installed route', () => {
  beforeEach(() => {
    vi.resetModules()
    authStore.checkAuth.mockReset()
    appStore.fetchPublicSettings.mockReset()
    window.scrollTo = vi.fn()
  })

  it('registers /skills/installed with installedOnly props and installed meta copy', async () => {
    const { default: router } = await import('@/router')
    const route = router.getRoutes().find((item) => item.path === '/skills/installed')

    expect(route).toBeTruthy()
    expect(route?.name).toBe('SkillInstalled')
    expect(route?.meta.requiresAuth).toBe(true)
    expect(route?.meta.titleKey).toBe('skills.installed.title')
    expect(route?.meta.descriptionKey).toBe('skills.installed.subtitle')

    const normalizedProps =
      route && typeof route.props === 'object' && route.props !== null && 'default' in route.props
        ? route.props.default
        : route?.props

    expect(normalizedProps).toMatchObject({
      installedOnly: true,
    })
  })

  it('registers admin skill governance routes under /admin/skills', async () => {
    const { default: router } = await import('@/router')
    const routes = router.getRoutes().filter((item) => item.path.startsWith('/admin/skills'))
    const paths = routes.map((item) => item.path)

    expect(paths).toEqual(expect.arrayContaining([
      '/admin/skills',
      '/admin/skills/review',
      '/admin/skills/governance',
      '/admin/skills/runtime',
      '/admin/skills/settlements',
    ]))

    expect(routes.find((item) => item.path === '/admin/skills/review')?.meta.requiresAdmin).toBe(true)
    expect(routes.find((item) => item.path === '/admin/skills/governance')?.name).toBe('AdminSkillGovernance')
  })
})
