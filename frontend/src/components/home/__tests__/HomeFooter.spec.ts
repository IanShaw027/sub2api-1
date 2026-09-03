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

import HomeFooter from '../HomeFooter.vue'

const RouterLinkStub = {
  props: ['to'],
  template: '<a class="router-link-stub" :to="to"><slot /></a>'
}

function mountFooter(props: Partial<InstanceType<typeof HomeFooter>['$props']> = {}) {
  return mount(HomeFooter, {
    props: {
      currentYear: 2026,
      siteName: 'Sub2API',
      legalDocuments: [],
      githubUrl: 'https://github.com/example/sub2api',
      ...props
    },
    global: {
      stubs: { RouterLink: RouterLinkStub }
    }
  })
}

describe('HomeFooter', () => {
  it('renders the copyright line with the given year and site name', () => {
    const wrapper = mountFooter()
    expect(wrapper.text()).toContain('2026')
    expect(wrapper.text()).toContain('Sub2API')
  })

  it('falls back to the default /legal/terms and /legal/privacy links when legalDocuments is empty', () => {
    const wrapper = mountFooter({ legalDocuments: [] })
    const links = wrapper.findAll('a.router-link-stub')
    const hrefs = links.map((l) => l.attributes('to'))
    expect(hrefs).toContain('/legal/terms')
    expect(hrefs).toContain('/legal/privacy')
  })

  it('renders the configured legal documents instead of the defaults when present', () => {
    const wrapper = mountFooter({
      legalDocuments: [{ id: 'tos', title: 'Terms of Service' }]
    })
    const links = wrapper.findAll('a.router-link-stub')
    const hrefs = links.map((l) => l.attributes('to'))
    expect(hrefs).toEqual(['/legal/tos'])
    expect(wrapper.text()).toContain('Terms of Service')
    expect(hrefs).not.toContain('/legal/terms')
  })

  it('shows a docs link only when docUrl is provided', () => {
    const withoutDocs = mountFooter()
    expect(withoutDocs.find('a[href="https://docs.example.com"]').exists()).toBe(false)

    const withDocs = mountFooter({ docUrl: 'https://docs.example.com' })
    expect(withDocs.find('a[href="https://docs.example.com"]').exists()).toBe(true)
  })

  it('always renders the GitHub link', () => {
    const wrapper = mountFooter({ githubUrl: 'https://github.com/example/sub2api' })
    expect(wrapper.find('a[href="https://github.com/example/sub2api"]').exists()).toBe(true)
  })

  it('shows contact info only when provided', () => {
    const wrapper = mountFooter({ contactInfo: 'support@example.com' })
    expect(wrapper.text()).toContain('home.footerLinks.contact')
  })
})
