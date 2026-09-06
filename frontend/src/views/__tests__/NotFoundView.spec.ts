import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import NotFoundView from '../NotFoundView.vue'

const auth = vi.hoisted(() => ({ isAdmin: false }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

afterEach(() => { auth.isAdmin = false })

describe('NotFoundView preserved recovery actions', () => {
  it.each([false, true])('retains dashboard, home and back actions for admin=%s', async (isAdmin) => {
    auth.isAdmin = isAdmin
    const router = createRouter({
      history: createMemoryHistory(),
      routes: ['/missing', '/home', '/dashboard', '/admin/dashboard'].map(path => ({ path, component: { template: '<div />' } }))
    })
    await router.push('/missing')
    await router.isReady()
    const back = vi.spyOn(router, 'back').mockImplementation(() => {})
    const wrapper = mount(NotFoundView, { global: { plugins: [router], stubs: { Icon: true } } })
    const dashboardPath = isAdmin ? '/admin/dashboard' : '/dashboard'
    expect(wrapper.get(`a[href="${dashboardPath}"]`).text()).toBe('home.goToDashboard')
    expect(wrapper.get('a[href="/home"]').text()).toBe('notFound.backHome')
    await wrapper.get('button').trigger('click')
    expect(back).toHaveBeenCalledOnce()
    await wrapper.get(`a[href="${dashboardPath}"]`).trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe(dashboardPath)
    await wrapper.get('a[href="/home"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/home')
    wrapper.unmount()
  })
})
