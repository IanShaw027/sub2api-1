import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const copyToClipboardMock = vi.fn()
const routeQuery = vi.hoisted(() => ({
  value: {
    capture_url: 'https://localhost:8444/capture',
    token: 'capture-token',
    platform: 'openai'
  } as Record<string, string>
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: copyToClipboardMock
  })
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: routeQuery.value
  }),
  RouterLink: {
    props: ['to'],
    template: '<a><slot /></a>'
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

import TLSFingerprintCollectorView from '../TLSFingerprintCollectorView.vue'

describe('TLSFingerprintCollectorView', () => {
  beforeEach(() => {
    copyToClipboardMock.mockReset()
    routeQuery.value = {
      capture_url: 'https://localhost:8444/capture',
      token: 'capture-token',
      platform: 'openai'
    }
  })

  it('renders native capture URLs and client instructions without JSON submit controls', () => {
    const wrapper = mount(TLSFingerprintCollectorView)
    const text = wrapper.text()

    expect(text).toContain('https://localhost:8444/capture/openai/v1')
    expect(text).toContain('https://localhost:8444/capture/openai/v1/responses')
    expect(text).toContain('tlsCollector.guides.codexCli.title')
    expect(text).toContain('tlsCollector.guides.codexExec.title')
    expect(text).toContain('tlsCollector.guides.claudeCode.title')
    expect(text).toContain('tlsCollector.guides.claudePrint.title')
    expect(text).toContain('tlsCollector.guides.node.title')
    expect(text).toContain('tlsCollector.guides.python.title')
    expect(text).toContain('tlsCollector.supportedTransportsTitle')
    expect(text).toContain('tlsCollector.successBehaviorTitle')
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('copies generated commands instead of posting to a submit API', async () => {
    const wrapper = mount(TLSFingerprintCollectorView)

    await wrapper.find('button.collector-button').trigger('click')

    expect(copyToClipboardMock).toHaveBeenCalled()
    const copiedText = String(copyToClipboardMock.mock.calls[0][0])
    expect(copiedText).toContain('codex exec')
    expect(copiedText).toContain('claude -p')
    expect(copiedText).toContain('https://localhost:8444/capture/openai/v1')
    expect(copiedText).toContain('https://localhost:8444/capture/anthropic/v1')
    expect(copiedText).toContain('capture-token')
    expect(copiedText).not.toContain('/api/v1/tls-fingerprint-captures/submit')
  })

  it('single quotes curl authorization header so token command substitution is inert', async () => {
    routeQuery.value = {
      capture_url: 'https://localhost:8444/capture',
      token: 'tok$(uname)`id`',
      platform: 'openai'
    }
    const wrapper = mount(TLSFingerprintCollectorView)

    await wrapper.find('button.collector-button').trigger('click')

    const copiedText = String(copyToClipboardMock.mock.calls[0][0])
    expect(copiedText).toContain("-H 'Authorization: Bearer tok$(uname)`id`'")
    expect(copiedText).not.toContain('-H "Authorization: Bearer tok$(uname)`id`"')
  })

  it('ignores capture_url query params from a different hostname', async () => {
    routeQuery.value = {
      capture_url: 'https://evil.example/capture',
      token: 'capture-token',
      platform: 'openai'
    }
    const wrapper = mount(TLSFingerprintCollectorView)

    await wrapper.find('button.collector-button').trigger('click')

    const copiedText = String(copyToClipboardMock.mock.calls[0][0])
    expect(copiedText).toContain('https://localhost:8444/capture/openai/v1')
    expect(copiedText).not.toContain('https://evil.example')
  })

  it('switches guide disclosure text after opening a command block', async () => {
    const wrapper = mount(TLSFingerprintCollectorView)
    const guide = wrapper.find('details')

    expect(guide.text()).toContain('common.expand')

    ;(guide.element as HTMLDetailsElement).open = true
    await guide.trigger('toggle')

    expect(guide.text()).toContain('common.collapse')
  })
})
