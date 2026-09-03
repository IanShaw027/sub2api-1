import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AppSidebar from '../AppSidebar.vue'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingStore } from '@/stores/onboarding'
import { ensureSidebarSectionForSelector } from '@/composables/ensureSidebarSectionForSelector'
import type { User } from '@/types'

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

const originalMatchMedia = window.matchMedia

type Viewport = 'mobile' | 'tablet' | 'desktop'

function mockViewport(initial: Viewport) {
  const states: Record<Viewport, { tabletUp: boolean; desktop: boolean }> = {
    mobile: { tabletUp: false, desktop: false },
    tablet: { tabletUp: true, desktop: false },
    desktop: { tabletUp: true, desktop: true }
  }
  let current = states[initial]
  const listeners = new Map<string, Set<(event: MediaQueryListEvent) => void>>()

  function matchesFor(query: string) {
    if (query.includes('min-width: 1024px')) return current.desktop
    if (query.includes('min-width: 768px')) return current.tabletUp
    return false
  }

  window.matchMedia = ((query: string) => {
    if (!listeners.has(query)) listeners.set(query, new Set())
    const set = listeners.get(query)!
    return {
      get matches() {
        return matchesFor(query)
      },
      media: query,
      onchange: null,
      addListener: (cb: (event: MediaQueryListEvent) => void) => set.add(cb),
      removeListener: (cb: (event: MediaQueryListEvent) => void) => set.delete(cb),
      addEventListener: (_event: string, cb: (event: MediaQueryListEvent) => void) => set.add(cb),
      removeEventListener: (_event: string, cb: (event: MediaQueryListEvent) => void) => set.delete(cb),
      dispatchEvent: () => true
    }
  }) as unknown as typeof window.matchMedia

  function setViewport(next: Viewport) {
    current = states[next]
    for (const [query, set] of listeners) {
      const event = { matches: matchesFor(query), media: query } as MediaQueryListEvent
      set.forEach((cb) => cb(event))
    }
  }

  return { setViewport }
}

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

async function mountSidebar(
  path: string,
  role: 'admin' | 'user' = 'admin',
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
  setup?.({ appStore, onboardingStore })

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/dashboard', component: { template: '<div />' } },
      { path: '/admin/groups', component: { template: '<div />' } },
      { path: '/admin/channels', component: { template: '<div />' } },
      { path: '/admin/channels/pricing', component: { template: '<div />' } },
      { path: '/dashboard', component: { template: '<div />' } },
      { path: '/keys', component: { template: '<div />' } },
      { path: '/profile', component: { template: '<div />' } },
      { path: '/login', component: { template: '<div />' } },
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
    attachTo: document.body,
    global: {
      plugins: [pinia, router, i18n],
      stubs: {
        VersionBadge: true,
        // Instant enter/leave so v-if drawer assertions are not racing CSS transitions.
        Transition: {
          template: '<slot />'
        }
      }
    }
  })

  await flushPromises()
  await nextTick()
  return { wrapper, appStore, router }
}

describe('MobileDrawer', () => {
  let setViewport: (next: Viewport) => void

  beforeEach(() => {
    localStorage.clear()
    document.body.style.overflow = ''
    ;({ setViewport } = mockViewport('mobile'))
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
    document.body.style.overflow = ''
    document.body.innerHTML = ''
    localStorage.clear()
  })

  it('opens and closes the right-hand drawer', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    expect(document.querySelector('#mobile-drawer')).toBeNull()

    appStore.setMobileOpen(true)
    await nextTick()
    expect(document.querySelector('#mobile-drawer')).not.toBeNull()
    expect(document.querySelector('.mobile-drawer-overlay')).not.toBeNull()

    appStore.setMobileOpen(false)
    await nextTick()
    expect(document.querySelector('#mobile-drawer')).toBeNull()
    wrapper.unmount()
  })

  it('locks body scroll while the drawer is open and restores it on close', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    expect(document.body.style.overflow).not.toBe('hidden')

    appStore.setMobileOpen(true)
    await nextTick()
    expect(document.body.style.overflow).toBe('hidden')

    appStore.setMobileOpen(false)
    await nextTick()
    expect(document.body.style.overflow).not.toBe('hidden')
    wrapper.unmount()
  })

  it('closes on overlay click, close button, and Escape', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    appStore.setMobileOpen(true)
    await nextTick()

    document.querySelector('.mobile-drawer-overlay')?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await nextTick()
    expect(appStore.mobileOpen).toBe(false)

    appStore.setMobileOpen(true)
    await nextTick()
    const closeBtn = document.querySelector('.mobile-drawer-close') as HTMLButtonElement | null
    expect(closeBtn).not.toBeNull()
    closeBtn?.click()
    await nextTick()
    expect(appStore.mobileOpen).toBe(false)

    appStore.setMobileOpen(true)
    await nextTick()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(appStore.mobileOpen).toBe(false)
    wrapper.unmount()
  })

  it('closes on route change', async () => {
    const { wrapper, appStore, router } = await mountSidebar('/admin/dashboard')
    appStore.setMobileOpen(true)
    await nextTick()
    expect(document.querySelector('#mobile-drawer')).not.toBeNull()

    await router.push('/admin/groups')
    await nextTick()
    expect(appStore.mobileOpen).toBe(false)
    wrapper.unmount()
  })

  it('keeps onboarding tour anchors in the DOM on mobile without duplicating ids', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    expect(wrapper.find('#sidebar-group-manage').exists()).toBe(true)
    expect(wrapper.find('#sidebar-channel-manage').exists()).toBe(true)
    expect(wrapper.find('[data-tour="sidebar-my-keys"]').exists()).toBe(true)
    expect(wrapper.find('#sidebar-section-overview').exists()).toBe(true)

    appStore.setMobileOpen(true)
    await nextTick()
    expect(document.querySelectorAll('#sidebar-group-manage')).toHaveLength(1)
    expect(document.querySelectorAll('#sidebar-channel-manage')).toHaveLength(1)
    expect(document.querySelectorAll('[data-tour="sidebar-my-keys"]')).toHaveLength(1)
    expect(document.querySelector('#mobile-drawer .sidebar-item[id]')).toBeNull()
    expect(document.querySelector('#mobile-drawer [id^="sidebar-section-"]')).toBeNull()
    expect(document.querySelectorAll('#sidebar-section-overview')).toHaveLength(1)

    const groupBtn = [...document.querySelectorAll('#mobile-drawer button')].find((el) =>
      el.textContent?.includes('nav.channelManagement')
    ) as HTMLButtonElement | undefined
    expect(groupBtn).toBeDefined()
    groupBtn?.click()
    await nextTick()
    expect(document.querySelector('#mobile-drawer [id^="sidebar-group-admin"]')).toBeNull()
    expect(document.querySelector('#mobile-drawer [id^="sidebar-group-"]')).toBeNull()
    wrapper.unmount()
  })

  it('does not render the collapse-rail button in the drawer footer', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    appStore.setMobileOpen(true)
    await nextTick()

    const drawer = document.querySelector('#mobile-drawer')
    expect(drawer).not.toBeNull()
    expect(drawer?.querySelector('[aria-label="nav.collapse"]')).toBeNull()
    expect(drawer?.querySelector('[aria-label="nav.expand"]')).toBeNull()
    expect(drawer?.querySelector('a[href="/profile"]')).not.toBeNull()
    expect(drawer?.querySelector('a[href="/keys"]')).not.toBeNull()
    expect(drawer?.querySelector('[aria-label="nav.logout"]')).not.toBeNull()
    wrapper.unmount()
  })

  it('exposes profile and logout in the drawer footer', async () => {
    const { wrapper, appStore, router } = await mountSidebar('/admin/dashboard')
    const authStore = useAuthStore()
    const logoutSpy = vi.spyOn(authStore, 'logout').mockResolvedValue(undefined)

    appStore.setMobileOpen(true)
    await nextTick()

    const profile = document.querySelector('#mobile-drawer a[href="/profile"]')
    expect(profile).not.toBeNull()
    expect(profile?.textContent).toContain('nav.profile')
    expect(document.querySelector('#mobile-drawer a[href="/keys"]')).not.toBeNull()

    const logoutBtn = [...document.querySelectorAll('#mobile-drawer button')].find((el) =>
      el.textContent?.includes('nav.logout')
    ) as HTMLButtonElement | undefined
    expect(logoutBtn).toBeDefined()
    await logoutBtn?.click()
    await flushPromises()
    expect(logoutSpy).toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/login')
    expect(appStore.mobileOpen).toBe(false)
    wrapper.unmount()
  })

  it('closes the drawer when the viewport crosses tablet-up', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    appStore.setMobileOpen(true)
    await nextTick()
    expect(appStore.mobileOpen).toBe(true)
    expect(document.body.style.overflow).toBe('hidden')

    setViewport('tablet')
    await nextTick()
    expect(appStore.mobileOpen).toBe(false)
    expect(document.querySelector('#mobile-drawer')).toBeNull()
    expect(document.body.style.overflow).not.toBe('hidden')
    wrapper.unmount()
  })

  it('force-collapses the rail on tablet and hides the expand control', async () => {
    mockViewport('tablet')
    const { wrapper } = await mountSidebar('/admin/dashboard')
    expect(wrapper.find('aside.sidebar').classes()).toContain('is-collapsed')
    expect(wrapper.find('[aria-label="nav.expand"]').exists()).toBe(false)
    expect(wrapper.find('[aria-label="nav.collapse"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('hosts unique tour anchors in the open drawer while the tour is active', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard', 'admin', ({ onboardingStore }) => {
      onboardingStore.setDriverActive(true)
    })
    expect(appStore.mobileOpen).toBe(true)
    await nextTick()
    expect(document.querySelector('#mobile-drawer')).not.toBeNull()
    expect(document.querySelectorAll('#sidebar-group-manage')).toHaveLength(1)
    expect(document.querySelector('#mobile-drawer #sidebar-group-manage')).not.toBeNull()
    expect(wrapper.find('aside.sidebar #sidebar-group-manage').exists()).toBe(false)
    expect(document.querySelectorAll('[data-tour="sidebar-my-keys"]')).toHaveLength(1)
    expect(document.querySelector('#mobile-drawer [data-tour="sidebar-my-keys"]')).not.toBeNull()
    expect(document.querySelectorAll('#sidebar-section-overview')).toHaveLength(1)
    expect(document.querySelector('#mobile-drawer #sidebar-section-overview')).not.toBeNull()
    wrapper.unmount()
  })

  it('opens the drawer from ensureSidebarSectionForSelector on mobile', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    expect(appStore.mobileOpen).toBe(false)
    await ensureSidebarSectionForSelector('[data-tour="sidebar-my-keys"]')
    await nextTick()
    expect(appStore.mobileOpen).toBe(true)
    expect(appStore.sidebarSectionsForceOpen.myAccount).toBe(true)
    wrapper.unmount()
  })

  it('focuses the close button on open and restores focus on close', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard')
    const trigger = document.createElement('button')
    trigger.type = 'button'
    document.body.appendChild(trigger)
    trigger.focus()

    appStore.setMobileOpen(true)
    await flushPromises()
    await nextTick()
    expect(document.activeElement?.classList.contains('mobile-drawer-close')).toBe(true)

    appStore.setMobileOpen(false)
    await flushPromises()
    await nextTick()
    expect(document.activeElement).toBe(trigger)
    trigger.remove()
    wrapper.unmount()
  })

  it('opens the drawer when the tour becomes active after mount', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard', 'admin')
    const onboardingStore = useOnboardingStore()
    expect(appStore.mobileOpen).toBe(false)

    onboardingStore.setDriverActive(true)
    await nextTick()
    expect(appStore.mobileOpen).toBe(true)
    wrapper.unmount()
  })

  it('closes the drawer when the onboarding tour ends on mobile', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard', 'admin')
    const onboardingStore = useOnboardingStore()
    onboardingStore.setDriverActive(true)
    await nextTick()
    expect(appStore.mobileOpen).toBe(true)

    onboardingStore.setDriverActive(false)
    await nextTick()
    expect(appStore.mobileOpen).toBe(false)
    wrapper.unmount()
  })

  it('does not clear body overflow on mount while the drawer is closed', async () => {
    document.body.style.overflow = 'scroll'
    const { wrapper } = await mountSidebar('/admin/dashboard')
    expect(document.body.style.overflow).toBe('scroll')
    wrapper.unmount()
  })
})

describe('mobile shell breakpoints', () => {
  const dir = dirname(fileURLToPath(import.meta.url))
  const styleSource = readFileSync(resolve(dir, '../../../style.css'), 'utf8')
  const layoutSource = readFileSync(resolve(dir, '../AppLayout.vue'), 'utf8')
  const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')

  it('unhides the desktop sidebar from the tablet breakpoint', () => {
    expect(styleSource).toMatch(/\.sidebar\.is-mobile-hidden[\s\S]*?@media \(min-width: 768px\)/)
    expect(styleSource).not.toMatch(/\.sidebar\.is-mobile-hidden[\s\S]*?@media \(min-width: 1024px\)/)
  })

  it('offsets content 72px on tablet and 224px/72px on desktop', () => {
    expect(layoutSource).toContain('@media (min-width: 768px) and (max-width: 1023px)')
    expect(layoutSource).toContain('@media (min-width: 1024px)')
    expect(layoutSource).toContain('margin-left: 72px')
    expect(layoutSource).toContain('margin-left: 224px')
    expect(layoutSource).toContain('!isDesktop || sidebarCollapsed')
  })

  it('renders a compact mobile topbar and keeps the hamburger off tablet+', () => {
    expect(headerSource).toContain('mobile-topbar')
    expect(headerSource).toContain('header-icon-btn mobile-topbar-icon md:hidden')
    expect(headerSource).toContain(':aria-label="t(\'common.toggleMenu\')"')
    expect(headerSource).toContain(':aria-expanded="appStore.mobileOpen"')
    expect(headerSource).toContain('aria-controls="mobile-drawer"')
    expect(headerSource).not.toContain('header-icon-btn lg:hidden')
    expect(headerSource).not.toContain('.header-icon-btn {')
    expect(styleSource).toContain('.header-icon-btn {')
    expect((headerSource.match(/<AnnouncementBell/g) ?? []).length).toBe(1)
  })

  it('keeps the drawer panel above its overlay', () => {
    const drawerSource = readFileSync(resolve(dir, '../MobileDrawer.vue'), 'utf8')
    const overlayZ = Number(drawerSource.match(/\.mobile-drawer-overlay\s*\{[\s\S]*?z-index:\s*(\d+)/)?.[1])
    const panelZ = Number(drawerSource.match(/\.mobile-drawer\s*\{[\s\S]*?z-index:\s*(\d+)/)?.[1])
    expect(panelZ).toBeGreaterThan(overlayZ)
  })
})
