import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import HomeCtaBanner from '../HomeCtaBanner.vue'

const RouterLinkStub = {
  props: ['to'],
  template: '<a class="router-link-stub" :to="to"><slot /></a>'
}

function mountBanner(props: { to: string; label: string }) {
  return mount(HomeCtaBanner, {
    props,
    global: {
      stubs: { RouterLink: RouterLinkStub }
    }
  })
}

describe('HomeCtaBanner', () => {
  it('renders the title and description via i18n', () => {
    const wrapper = mountBanner({ to: '/register', label: 'home.cta.button' })
    expect(wrapper.find('h2').text()).toBe('home.cta.title')
    expect(wrapper.find('p').text()).toBe('home.cta.description')
  })

  it('links to the given route with the given label', () => {
    const wrapper = mountBanner({ to: '/register', label: 'home.cta.button' })
    const link = wrapper.find('a.router-link-stub')
    expect(link.attributes('to')).toBe('/register')
    expect(link.text()).toBe('home.cta.button')
  })

  it('links to the dashboard path and a different label when authenticated', () => {
    const wrapper = mountBanner({ to: '/dashboard', label: 'home.goToDashboard' })
    const link = wrapper.find('a.router-link-stub')
    expect(link.attributes('to')).toBe('/dashboard')
    expect(link.text()).toBe('home.goToDashboard')
  })
})
