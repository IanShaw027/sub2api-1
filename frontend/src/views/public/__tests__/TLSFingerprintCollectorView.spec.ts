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

  it('renders native capture URLs and platform-scoped client instructions without JSON submit controls', () => {
    const wrapper = mount(TLSFingerprintCollectorView)
    const text = wrapper.text()

    expect(text).toContain('https://localhost:8444/capture/openai/v1')
    expect(text).toContain('https://localhost:8444/capture/openai/v1/responses')
    // openai platform shows only OpenAI client guides
    expect(text).toContain('tlsCollector.guides.codexCli.title')
    expect(text).toContain('tlsCollector.guides.codexExec.title')
    expect(text).toContain('tlsCollector.guides.node.title')
    expect(text).toContain('tlsCollector.guides.python.title')
    // anthropic guides are not shown under the openai platform
    expect(text).not.toContain('tlsCollector.guides.claudeCode.title')
    expect(text).not.toContain('tlsCollector.guides.claudePrint.title')
    expect(text).toContain('tlsCollector.supportedTransportsTitle')
    expect(text).toContain('tlsCollector.successBehaviorTitle')
    expect(wrapper.find('textarea').exists()).toBe(false)
  })

  it('copies platform-scoped commands instead of posting to a submit API', async () => {
    const wrapper = mount(TLSFingerprintCollectorView)

    await wrapper.find('button.collector-button').trigger('click')

    expect(copyToClipboardMock).toHaveBeenCalled()
    const copiedText = String(copyToClipboardMock.mock.calls[0][0])
    // openai platform: includes codex commands, excludes claude commands
    expect(copiedText).toContain('codex exec')
    expect(copiedText).not.toContain('claude -p')
    expect(copiedText).toContain('https://localhost:8444/capture/openai/v1')
    expect(copiedText).toContain('capture-token')
    expect(copiedText).not.toContain('/api/v1/tls-fingerprint-captures/submit')
  })

  it('shows Anthropic Claude guides and base URL for platform=anthropic', async () => {
    routeQuery.value = {
      capture_url: 'https://localhost:8444/capture',
      token: 'capture-token',
      platform: 'anthropic'
    }
    const wrapper = mount(TLSFingerprintCollectorView)
    const text = wrapper.text()

    expect(text).toContain('https://localhost:8444/capture/anthropic/v1')
    expect(text).toContain('tlsCollector.guides.claudeCode.title')
    expect(text).toContain('tlsCollector.guides.claudePrint.title')
    expect(text).not.toContain('tlsCollector.guides.codexCli.title')
  })

  it('offers custom collector platform and falls back to platform-scoped curl', async () => {
    routeQuery.value = {
      capture_url: 'https://localhost:8444/capture',
      token: 'capture-token',
      platform: 'custom'
    }
    const wrapper = mount(TLSFingerprintCollectorView)

    const platformSelect = wrapper.find('select')
    expect(platformSelect.find('option[value="custom"]').exists()).toBe(true)
    await wrapper.find('button.collector-button').trigger('click')

    const copiedText = String(copyToClipboardMock.mock.calls[0][0])
    expect(copiedText).toContain('curl -v')
    expect(copiedText).toContain('https://localhost:8444/capture/custom/v1/responses')
    expect(copiedText).not.toContain('codex exec')
    expect(copiedText).not.toContain('claude -p')
  })

  it('preserves unknown collector platforms from query', async () => {
    routeQuery.value = {
      capture_url: 'https://localhost:8444/capture',
      token: 'capture-token',
      platform: 'custom-acme'
    }
    const wrapper = mount(TLSFingerprintCollectorView)

    const platformSelect = wrapper.find('select')
    expect(platformSelect.find('option[value="custom-acme"]').exists()).toBe(true)
    await wrapper.find('button.collector-button').trigger('click')

    const copiedText = String(copyToClipboardMock.mock.calls[0][0])
    expect(copiedText).toContain('https://localhost:8444/capture/custom-acme/v1/responses')
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
