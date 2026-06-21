import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { submitCaptureToEndpointMock } = vi.hoisted(() => ({
  submitCaptureToEndpointMock: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tlsFingerprintProfiles: {
      submitCaptureToEndpoint: submitCaptureToEndpointMock
    }
  }
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {
      endpoint: 'https://gateway.example/api/v1/tls-fingerprint-captures/submit',
      token: 'capture-token',
      platform: 'openai',
      user_agent: 'codex-cli/1.2.3'
    }
  }),
  RouterLink: {
    props: ['to'],
    template: '<a><slot /></a>'
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (params?.counts) return `${key}:${params.counts}`
      return key
    }
  })
}))

import TLSFingerprintCollectorView from '../TLSFingerprintCollectorView.vue'

describe('TLSFingerprintCollectorView', () => {
  beforeEach(() => {
    submitCaptureToEndpointMock.mockReset()
    submitCaptureToEndpointMock.mockResolvedValue({
      accepted: true,
      duplicate: false,
      counts: { openai: 1 },
      task: { id: 1, targets: { openai: 100 }, counts: { openai: 1 } }
    })
    Object.defineProperty(window.navigator, 'userAgent', {
      value: 'vitest-user-agent',
      configurable: true
    })
  })

  it('submits the pasted TLS payload to the endpoint from query params', async () => {
    const wrapper = mount(TLSFingerprintCollectorView)
    const payloadInput = wrapper.find('textarea')

    await payloadInput.setValue('name: codex-cli\ncipher_suites: [0x1301]')
    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(submitCaptureToEndpointMock).toHaveBeenCalledWith(
      'https://gateway.example/api/v1/tls-fingerprint-captures/submit',
      {
        token: 'capture-token',
        platform: 'openai',
        user_agent: 'codex-cli/1.2.3',
        payload: 'name: codex-cli\ncipher_suites: [0x1301]'
      }
    )
  })

  it('shows a friendly message when the selected platform target is already reached', async () => {
    submitCaptureToEndpointMock.mockResolvedValue({
      accepted: false,
      duplicate: false,
      ignored_reason: 'platform_target_reached',
      counts: { openai: 100 },
      task: { id: 1, targets: { openai: 100 }, counts: { openai: 100 } }
    })
    const wrapper = mount(TLSFingerprintCollectorView)

    await wrapper.find('textarea').setValue('name: codex-cli\ncipher_suites: [0x1301]')
    await wrapper.find('button').trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('tlsCollector.ignored.platform_target_reached')
  })
})
