import { defineComponent } from 'vue'
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

import ConsolePreview from '../ConsolePreview.vue'

const PlatformIconStub = defineComponent({
  name: 'PlatformIcon',
  props: ['platform', 'size'],
  template: '<span class="platform-icon-stub" />'
})

function mountPreview(apiBaseUrl?: string) {
  return mount(ConsolePreview, {
    props: { apiBaseUrl },
    global: {
      stubs: { PlatformIcon: PlatformIconStub }
    }
  })
}

describe('ConsolePreview', () => {
  it('is decorative (aria-hidden) since values are illustrative', () => {
    const wrapper = mountPreview()
    expect(wrapper.find('.console-wrap').attributes('aria-hidden')).toBe('true')
  })

  it('falls back to i18n default url when no apiBaseUrl is provided', () => {
    const wrapper = mountPreview()
    expect(wrapper.find('.console-url').text()).toContain('home.console.url')
  })

  it('derives the dashboard url from apiBaseUrl when provided', () => {
    const wrapper = mountPreview('https://api.example.com/')
    expect(wrapper.find('.console-url').text()).toContain('api.example.com/dashboard')
  })

  it('renders one provider card per configured provider', () => {
    const wrapper = mountPreview()
    expect(wrapper.findAll('.console-provider')).toHaveLength(3)
    expect(wrapper.findAllComponents(PlatformIconStub)).toHaveLength(3)
  })

  it('renders the recent request log rows', () => {
    const wrapper = mountPreview()
    expect(wrapper.findAll('.console-log').length).toBeGreaterThan(0)
  })
})
