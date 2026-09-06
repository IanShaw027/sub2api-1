import { nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createI18n } from 'vue-i18n'
import { mount, flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import AppSidebar from '@/components/layout/AppSidebar.vue'
import { ensureSidebarSectionForSelector } from '@/composables/ensureSidebarSectionForSelector'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
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

async function mountSidebar(path: string, role: 'admin' | 'user') {
  const pinia = createPinia()
  setActivePinia(pinia)

  const authStore = useAuthStore()
  authStore.user = fakeUser(role)

  const appStore = useAppStore()

  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/admin/dashboard', component: { template: '<div />' } },
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
    attachTo: document.body,
    global: {
      plugins: [pinia, router, i18n],
      stubs: {
        VersionBadge: true
      }
    }
  })

  await flushPromises()
  await nextTick()
  return { wrapper, appStore }
}

describe('ensureSidebarSectionForSelector', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  afterEach(() => {
    document.body.innerHTML = ''
    localStorage.clear()
  })

  it('force-opens admin My Account for sidebar-my-keys without persisting preference', async () => {
    const { wrapper, appStore } = await mountSidebar('/admin/dashboard', 'admin')
    const setOpen = vi.spyOn(appStore, 'setSidebarSectionOpen')

    const myAccount = wrapper.find('[data-section="myAccount"]')
    expect(myAccount.find('.sidebar-section-items').classes()).not.toContain('hidden')

    await ensureSidebarSectionForSelector('[data-tour="sidebar-my-keys"]')
    await nextTick()

    expect(setOpen).not.toHaveBeenCalled()
    expect(appStore.sidebarSectionsForceOpen.myAccount).toBe(true)
    expect(appStore.sidebarSectionsOpen.myAccount).toBeUndefined()
    expect(localStorage.getItem('sidebar-sections-open')).toBeNull()
    expect(wrapper.find('[data-section="myAccount"] .sidebar-section-items').classes()).not.toContain('hidden')

    appStore.clearSidebarSectionsForceOpen()
    await nextTick()
    expect(wrapper.find('[data-section="myAccount"] .sidebar-section-items').classes()).not.toContain('hidden')

    wrapper.unmount()
  })

  it('resolves /keys to workspace from a mounted user sidebar', async () => {
    const { wrapper, appStore } = await mountSidebar('/dashboard', 'user')
    const setOpen = vi.spyOn(appStore, 'setSidebarSectionOpen')

    await ensureSidebarSectionForSelector('[data-tour="sidebar-my-keys"]')

    expect(setOpen).not.toHaveBeenCalled()
    expect(appStore.sidebarSectionsForceOpen.workspace).toBe(true)
    expect(appStore.sidebarSectionsForceOpen.myAccount).toBeUndefined()
    expect(localStorage.getItem('sidebar-sections-open')).toBeNull()

    wrapper.unmount()
  })

  it('falls back to sectionKeyForSelector when the target is not in the DOM', async () => {
    setActivePinia(createPinia())
    const appStore = useAppStore()

    await ensureSidebarSectionForSelector('[data-tour="sidebar-my-keys"]')

    expect(appStore.sidebarSectionsForceOpen.myAccount).toBe(true)
    expect(appStore.sidebarSectionsOpen.myAccount).toBeUndefined()
  })
})
