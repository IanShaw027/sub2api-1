import { afterEach, describe, expect, it } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import { createMemoryHistory, createRouter } from 'vue-router'
import AppHeader from '../AppHeader.vue'

afterEach(() => { Object.defineProperty(window, 'scrollY', { value: 0, configurable: true }) })

describe('AppHeader navigation and scrolling', () => {
  it('links ancestors, keeps the current page as text, and fades in the scroll surface', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/admin/dashboard', component: { template: '<div />' } },
      { path: '/admin/orders', component: { template: '<div />' }, meta: { title: 'Orders' } },
      { path: '/admin/orders/invoices', component: { template: '<div />' }, meta: { title: 'Invoices' } }
    ] })
    await router.push('/admin/orders/invoices')
    const wrapper = shallowMount(AppHeader, { global: {
      plugins: [createPinia(), router, createI18n({ legacy: false, locale: 'en', missingWarn: false, fallbackWarn: false, messages: { en: {} } })],
      stubs: { RouterLink: false }
    } })
    expect(wrapper.get('.topbar-crumbs a').attributes('href')).toBe('/admin/dashboard')
    expect(wrapper.get('[aria-current="page"]').text()).toBe('Invoices')
    expect(wrapper.get('[aria-current="page"]').element.tagName).toBe('SPAN')
    expect(wrapper.attributes('style')).toContain('--header-scroll-progress: 0')
    for (const [scroll, opacity] of [[24, 0.5], [96, 1], [0, 0]]) {
      Object.defineProperty(window, 'scrollY', { value: scroll, configurable: true })
      window.dispatchEvent(new Event('scroll'))
      await flushPromises()
      expect(wrapper.attributes('style')).toContain(`--header-scroll-progress: ${opacity}`)
    }
    await wrapper.get('a[href="/admin/orders"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/admin/orders')
    wrapper.unmount()
  })
})
