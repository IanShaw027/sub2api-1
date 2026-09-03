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

const copyToClipboard = vi.fn().mockResolvedValue(true)
vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard })
}))

import HomeCodeTabs from '../HomeCodeTabs.vue'

describe('HomeCodeTabs', () => {
  it('renders one tab button per client and defaults to the first tab active', () => {
    const wrapper = mount(HomeCodeTabs, { props: { apiBaseUrl: 'https://api.example.com' } })
    const tabs = wrapper.findAll('.code-tab')
    expect(tabs.map((t) => t.text())).toEqual(['Claude Code', 'Codex CLI', 'OpenAI SDK', 'cURL'])
    expect(tabs[0]?.classes()).toContain('is-active')
  })

  it('switches snippet content when a different tab is clicked', async () => {
    const wrapper = mount(HomeCodeTabs, { props: { apiBaseUrl: 'https://api.example.com' } })
    const body = () => wrapper.find('.code-tabs-body').text()
    expect(body()).toContain('ANTHROPIC_BASE_URL')

    await wrapper.findAll('.code-tab')[3]!.trigger('click')
    expect(wrapper.findAll('.code-tab')[3]?.classes()).toContain('is-active')
    expect(body()).toContain('curl')
  })

  it('interpolates the given apiBaseUrl into the snippet text', () => {
    const wrapper = mount(HomeCodeTabs, { props: { apiBaseUrl: 'https://my-host.test/' } })
    expect(wrapper.find('.code-tabs-body').text()).toContain('https://my-host.test')
  })

  it('copies the active snippet to the clipboard when the copy button is clicked', async () => {
    const wrapper = mount(HomeCodeTabs, { props: { apiBaseUrl: 'https://api.example.com' } })
    await wrapper.find('.code-tabs-copy').trigger('click')
    expect(copyToClipboard).toHaveBeenCalledTimes(1)
    expect(copyToClipboard.mock.calls[0]?.[0]).toContain('ANTHROPIC_BASE_URL')
  })
})
