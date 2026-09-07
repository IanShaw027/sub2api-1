import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { mount, flushPromises } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AppSidebar from '../AppSidebar.vue'
import { groupAdminNav, groupUserNav } from '../sidebar/navSections'
import { sectionKeyForSelector } from '@/constants/sidebar'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useOnboardingStore } from '@/stores/onboarding'
import type { User } from '@/types'
import type { NavItem } from '../sidebar/navSections'
import { creationModes, creationPath } from '@/features/creation/navigation'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/composables/useBatchImageAccess', () => ({
  useBatchImageAccess: () => ({
    canUseBatchImage: { value: true },
    refreshBatchImageAccess: vi.fn().mockResolvedValue(true)
  })
}))

vi.mock('@/api/admin/payment', () => ({
  adminPaymentAPI: {
    getInvoiceUnreadCount: vi.fn().mockResolvedValue({ data: { count: 0 } })
  }
}))

vi.mock('@/api/admin/tickets', () => ({
  adminTicketsAPI: {
    unreadCount: vi.fn().mockResolvedValue({ data: { count: 0 } })
  }
}))

vi.mock('@/api/tickets', () => ({
  ticketsAPI: {
    unreadCount: vi.fn().mockResolvedValue({ data: { count: 0 } })
  }
}))

vi.mock('@/api', () => ({
  adminAPI: {
    settings: {
      getSettings: vi.fn().mockResolvedValue({
        ops_monitoring_enabled: true,
        custom_menu_items: []
      })
    },
    payment: {
      getConfig: vi.fn().mockResolvedValue({ data: { enabled: true } })
    }
  }
}))

function fakeUser(role: 'admin' | 'user'): User {
  return {
    id: 1,
    username: role,
    email: `${role}@test.com`,
    role,
    balance: 42,
    concurrency: 0,
    status: 'active',
    allowed_groups: null,
    balance_notify_enabled: false,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: '',
    updated_at: ''
  } as User
}

function nav(path: string): NavItem {
  return { path, label: path, icon: null }
}

async function mountSidebar(
  path: string,
  role: 'admin' | 'user',
  collapsed = false,
  setup?: (ctx: {
    appStore: ReturnType<typeof useAppStore>
    onboardingStore: ReturnType<typeof useOnboardingStore>
  }) => void
) {
  const pinia = createPinia()
  setActivePinia(pinia)

  const authStore = useAuthStore()
  authStore.user = fakeUser(role)

  const appStore = useAppStore()
  const onboardingStore = useOnboardingStore()
  if (collapsed) {
    appStore.setSidebarCollapsed(true)
  }
  setup?.({ appStore, onboardingStore })

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/dashboard', component: { template: '<div />' } },
      { path: '/admin/ops', component: { template: '<div />' } },
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/keys', component: { template: '<div />' } },
      { path: '/:pathMatch(.*)*', component: { template: '<div />' } }
    ]
  })
  await router.push(path)
  await router.isReady()

  const i18n = createI18n({
    legacy: false,
    locale: 'en',
    messages: { en: {} }
  })

  const wrapper = mount(AppSidebar, {
    global: {
      plugins: [pinia, router, i18n],
      stubs: {
        VersionBadge: true
      }
    }
  })

  await flushPromises()
  await nextTick()
  return { wrapper, appStore, router }
}

describe('AppSidebar section grouping', () => {
  it('maps admin items into the specified section order without dropping items', () => {
    const admin = [
      nav('/admin/dashboard'),
      nav('/admin/ops'),
      nav('/admin/users'),
      nav('/admin/groups'),
      nav('/admin/channels'),
      nav('/admin/subscriptions'),
      nav('/admin/accounts'),
      nav('/admin/plugins'),
      nav('/admin/announcements'),
      nav('/admin/proxies'),
      nav('/admin/redeem'),
      nav('/admin/usage'),
      nav('/admin/settings')
    ]
    const sections = groupAdminNav(admin)

    expect(sections.map((section) => section.key)).toEqual([
      'overview',
      'usersResources',
      'channels',
      'operations',
      'securityAudit',
      'system'
    ])
    expect(sections.find((section) => section.key === 'overview')?.items.map((item) => item.path)).toEqual([
      '/admin/dashboard',
      '/admin/ops'
    ])
    expect(sections.find((section) => section.key === 'usersResources')?.items.map((item) => item.path)).toEqual([
      '/admin/users',
      '/admin/groups',
      '/admin/subscriptions',
      '/admin/accounts',
      '/admin/proxies'
    ])
    expect(sections.flatMap(section => section.items)).toHaveLength(admin.length)
  })

  it('separates API services, billing, account and support without dropping user items', () => {
    const items = [
      nav('/dashboard'),
      nav('/keys'),
      nav('/usage'),
      nav('/subscriptions'),
      nav('/redeem'),
      nav('/tickets'),
      nav('/profile')
    ]
    const sections = groupUserNav(items)
    expect(sections.map((section) => section.key)).toEqual(['workspace', 'billing', 'account', 'support'])
    expect(sections[0].items.map((item) => item.path)).toEqual(['/dashboard', '/keys', '/usage'])
    expect(sections[1].items.map((item) => item.path)).toEqual(['/subscriptions', '/redeem'])
    expect(sections[2].items.map((item) => item.path)).toEqual(['/profile'])
    expect(sections[3].items.map((item) => item.path)).toEqual(['/tickets'])
  })

  it('groups creation and batch images only in the user menu', () => {
    const creation = creationModes.map(mode => nav(creationPath(mode)))
    expect(groupAdminNav([nav('/admin/dashboard')]).map(section => section.key)).toEqual(['overview'])
    const sections = groupUserNav([nav('/keys'), ...creation, nav('/batch-image'), nav('/profile')])
    expect(sections.map(section => section.key)).toEqual(['workspace', 'creation', 'account'])
    expect(sections[1].items.map(item => item.path)).toEqual([...creation.map(item => item.path), '/batch-image'])
  })

  it.each(['admin', 'user'] as const)('shows the creation category and active child for %s, respecting the feature switch', async role => {
    const { wrapper, router, appStore } = await mountSidebar('/studio/image', role, false, ({ appStore }) => {
      appStore.cachedPublicSettings = { creation_center_enabled: true } as typeof appStore.cachedPublicSettings
    })
    const category = wrapper.get('[data-section="creation"]')
    for (const mode of creationModes) expect(category.find(`a[href="${creationPath(mode)}"]`).exists()).toBe(true)
    expect(wrapper.find('a[href="/studio"]').exists()).toBe(false)
    await router.push('/studio/video')
    await flushPromises()
    expect(category.get('a[href="/studio/video"]').classes().some(name => name.includes('active'))).toBe(true)
    appStore.cachedPublicSettings = { creation_center_enabled: false } as typeof appStore.cachedPublicSettings
    await flushPromises()
    expect(wrapper.find('[data-section="creation"] a[href="/studio/image"]').exists()).toBe(false)
    expect(wrapper.find('[data-section="creation"] a[href="/batch-image"]').exists()).toBe(true)
    wrapper.unmount()
  })
})

describe('sectionKeyForSelector', () => {
  it('maps tour selectors to the sidebar section that owns the target', () => {
    expect(sectionKeyForSelector('[data-tour="sidebar-my-keys"]')).toBe('workspace')
    expect(sectionKeyForSelector('#sidebar-group-manage')).toBe('usersResources')
    expect(sectionKeyForSelector('#sidebar-channel-manage')).toBe('usersResources')
    expect(sectionKeyForSelector('#sidebar-wallet')).toBe('operations')
    expect(sectionKeyForSelector('[data-tour="unrelated"]')).toBeUndefined()
  })
})

describe('AppSidebar sections render', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
  })

  it('renders categorized admin-only navigation with a mode switch', async () => {
    const { wrapper } = await mountSidebar('/admin/dashboard', 'admin')
    const keys = wrapper.findAll('[data-section]').map((el) => el.attributes('data-section'))
    expect(keys).toContain('overview')
    expect(keys).not.toContain('workspace')
    expect(keys).not.toContain('creation')
    expect(wrapper.find('[data-section="overview"]').exists()).toBe(true)
    expect(wrapper.find('.sidebar-mode-switch').exists()).toBe(true)
    expect(wrapper.find('#sidebar-group-manage').exists()).toBe(true)
    wrapper.unmount()
  })

  it('renders user sections per role', async () => {
    const { wrapper } = await mountSidebar('/dashboard', 'user')
    const keys = wrapper.findAll('[data-section]').map((el) => el.attributes('data-section'))
    expect(keys).toEqual(expect.arrayContaining(['workspace', 'billing', 'support']))
    expect(keys).not.toContain('overview')
    expect(wrapper.find('.sidebar-mode-switch').exists()).toBe(false)
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('places /keys in workspace for user nav so DOM resolution is not myAccount', async () => {
    const { wrapper } = await mountSidebar('/dashboard', 'user')
    const keysItem = wrapper.find('[data-tour="sidebar-my-keys"]')
    expect(keysItem.exists()).toBe(true)
    expect(keysItem.element.closest('[data-section]')?.getAttribute('data-section')).toBe('workspace')
    wrapper.unmount()
  })

  it('switches admin/user routes and synchronizes navigation on back', async () => {
    const { wrapper, router } = await mountSidebar('/admin/dashboard', 'admin')
    await wrapper.get('button[aria-label="nav.userView"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/dashboard')
    expect(wrapper.find('[data-section="overview"]').exists()).toBe(false)
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    expect(wrapper.get('button[aria-label="nav.userView"]').attributes('aria-pressed')).toBe('true')
    await wrapper.get('button[aria-label="nav.adminView"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/dashboard')
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(false)
    router.back()
    await flushPromises()
    expect(wrapper.find('[data-section="workspace"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('opens user navigation transiently when the tour highlights sidebar-my-keys', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard', 'admin', false, ({ onboardingStore }) => {
      vi.spyOn(onboardingStore, 'isDriverActive').mockReturnValue(true)
      onboardingStore.setSidebarMode('user')
      vi.spyOn(onboardingStore, 'isCurrentStep').mockImplementation(
        (selector) => selector === '[data-tour="sidebar-my-keys"]'
      )
    })
    expect(appStore.sidebarSectionsOpen.workspace).toBeUndefined()
    const myAccount = wrapper.find('[data-section="workspace"]')
    expect(myAccount.exists()).toBe(true)
    expect(myAccount.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    expect(myAccount.find('.sidebar-section-items').classes()).not.toContain('hidden')
    wrapper.unmount()
  })

  it('opens the user workspace from transient force-open without persisting preference', async () => {
    const { wrapper, appStore } = await mountSidebar('/keys', 'admin')
    expect(wrapper.find('[data-section="workspace"] .sidebar-section-items').classes()).not.toContain('hidden')

    appStore.forceOpenSidebarSection('workspace')
    await nextTick()

    expect(wrapper.find('[data-section="workspace"] .sidebar-section-items').classes()).not.toContain('hidden')
    expect(appStore.sidebarSectionsOpen.workspace).toBeUndefined()
    expect(localStorage.getItem('sidebar-sections-open')).toBeNull()
    wrapper.unmount()
  })

  it('clears force-open on explicit toggle and persists closed', async () => {
    const { wrapper, appStore } = await mountSidebar('/keys', 'admin')
    appStore.forceOpenSidebarSection('workspace')
    await nextTick()
    expect(wrapper.find('[data-section="workspace"] .sidebar-section-items').classes()).not.toContain('hidden')

    await wrapper.get('[data-section="workspace"] .sidebar-section-title').trigger('click')
    await nextTick()

    expect(appStore.sidebarSectionsForceOpen.workspace).toBeUndefined()
    expect(appStore.sidebarSectionsOpen.workspace).toBe(false)
    expect(localStorage.getItem('sidebar-sections-open')).toContain('"workspace":false')
    expect(wrapper.find('[data-section="workspace"] .sidebar-section-items').classes()).toContain('hidden')
    wrapper.unmount()
  })

  it('keeps tour anchors in the DOM for an admin on the dashboard', async () => {
    const { wrapper } = await mountSidebar('/admin/dashboard', 'admin')
    expect(wrapper.find('#sidebar-group-manage').exists()).toBe(true)
    expect(wrapper.find('#sidebar-channel-manage').exists()).toBe(true)
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('persists section collapse via appStore', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard', 'admin')
    expect(appStore.sidebarSectionsOpen.overview).toBeUndefined()

    await wrapper.get('[data-section="overview"] .sidebar-section-title').trigger('click')
    await nextTick()
    expect(appStore.sidebarSectionsOpen.overview).toBe(false)
    expect(localStorage.getItem('sidebar-sections-open')).toContain('"overview":false')
    const overviewItems = wrapper.find('[data-section="overview"] .sidebar-section-items')
    expect(overviewItems.exists()).toBe(true)
    expect(overviewItems.classes()).toContain('hidden')
    expect(overviewItems.attributes('aria-hidden')).toBe('true')
    wrapper.unmount()
  })

  it('hides section titles when the sidebar is collapsed', async () => {
    const { wrapper } = await mountSidebar('/admin/dashboard', 'admin', true)
    expect(wrapper.find('.sidebar-section-title').exists()).toBe(false)
    expect(wrapper.find('.sidebar-section-divider').exists()).toBe(true)
    wrapper.unmount()
  })

  it('keeps tour anchors measurable when the sidebar is collapsed', async () => {
    const { wrapper } = await mountSidebar('/admin/dashboard', 'admin', true)
    expect(wrapper.find('#sidebar-group-manage').exists()).toBe(true)
    expect(wrapper.find('#sidebar-channel-manage').exists()).toBe(true)
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(false)
    const items = wrapper.findAll('.sidebar-section-items')
    expect(items.length).toBeGreaterThan(0)
    for (const item of items) {
      expect(item.classes()).not.toContain('hidden')
    }
    wrapper.unmount()
  })

  it('omits a feature-flagged item from the section DOM', async () => {
    const { wrapper } = await mountSidebar('/admin/dashboard', 'admin', false, () => {
      const adminSettings = useAdminSettingsStore()
      adminSettings.setOpsMonitoringEnabledLocal(false)
      vi.spyOn(adminSettings, 'fetch').mockResolvedValue(undefined)
    })
    expect(wrapper.find('[data-section="overview"]').exists()).toBe(true)
    expect(wrapper.find('a[href="/admin/dashboard"]').exists()).toBe(true)
    expect(wrapper.find('a[href="/admin/ops"]').exists()).toBe(false)
    wrapper.unmount()
  })
})
