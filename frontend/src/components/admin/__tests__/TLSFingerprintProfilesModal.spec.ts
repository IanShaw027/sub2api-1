import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import TLSFingerprintProfilesModal from '../TLSFingerprintProfilesModal.vue'

const {
  listProfilesMock,
  listCaptureTasksMock,
  getCaptureTaskMock,
  listCaptureSamplesMock,
  startCaptureTaskMock,
  createProfileMock,
  updateProfileMock,
  showErrorMock,
} = vi.hoisted(() => ({
  listProfilesMock: vi.fn(),
  listCaptureTasksMock: vi.fn(),
  getCaptureTaskMock: vi.fn(),
  listCaptureSamplesMock: vi.fn(),
  startCaptureTaskMock: vi.fn(),
  createProfileMock: vi.fn(),
  updateProfileMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tlsFingerprintProfiles: {
      list: listProfilesMock,
      listCaptureTasks: listCaptureTasksMock,
      getCaptureTask: getCaptureTaskMock,
      listCaptureSamples: listCaptureSamplesMock,
      startCaptureTask: startCaptureTaskMock,
      create: createProfileMock,
      update: updateProfileMock,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const translations: Record<string, string> = {
    'admin.tlsFingerprintProfiles.tabs.capture': 'Capture',
    'admin.tlsFingerprintProfiles.tabs.profiles': 'Profiles',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => translations[key] ?? key,
    }),
  }
})

const BaseDialogStub = defineComponent({
  props: {
    show: {
      type: Boolean,
      default: false,
    },
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

describe('TLSFingerprintProfilesModal', () => {
  beforeEach(() => {
    listProfilesMock.mockReset()
    listCaptureTasksMock.mockReset()
    getCaptureTaskMock.mockReset()
    listCaptureSamplesMock.mockReset()
    startCaptureTaskMock.mockReset()
    createProfileMock.mockReset()
    updateProfileMock.mockReset()
    showErrorMock.mockReset()
    listProfilesMock.mockResolvedValue([])
    listCaptureTasksMock.mockResolvedValue([])
    listCaptureSamplesMock.mockResolvedValue([])
    startCaptureTaskMock.mockResolvedValue({})
    createProfileMock.mockResolvedValue({})
    updateProfileMock.mockResolvedValue({})
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('includes Grok as a default TLS fingerprint capture target', async () => {
    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: {
        show: true,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('Grok / xAI')
  })

  it('uses backend TLS transport values in the profile form', async () => {
    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: {
        show: true,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    const createOpenButton = wrapper.findAll('button').find(button => button.text().includes('createProfile'))
    expect(createOpenButton).toBeTruthy()
    await createOpenButton!.trigger('click')
    await flushPromises()

    const transportSelect = wrapper.findAll('select').find(select =>
      select.find('option[value="http"]').exists() || select.find('option[value="http1"]').exists()
    )
    expect(transportSelect).toBeTruthy()
    const values = transportSelect!.findAll('option').map(option => option.attributes('value'))
    expect(values).toEqual(expect.arrayContaining(['', 'http1', 'h2', 'websocket-http1', 'websocket-h2']))
    expect(values).not.toContain('http')
    expect(values).not.toContain('websocket')
    const labels = transportSelect!.findAll('option').map(option => option.text())
    expect(labels).toEqual(expect.arrayContaining([
      'admin.tlsFingerprintProfiles.form.transportH2',
      'admin.tlsFingerprintProfiles.form.transportWebsocketH2',
    ]))
    expect(wrapper.text()).toContain('admin.tlsFingerprintProfiles.form.transportReplayHint')
  })

  it('exposes replay-only TLS fields in the profile form', async () => {
    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: {
        show: true,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    const createOpenButton = wrapper.findAll('button').find(button => button.text().includes('createProfile'))
    expect(createOpenButton).toBeTruthy()
    await createOpenButton!.trigger('click')
    await flushPromises()

    expect(wrapper.text()).toContain('signatureAlgorithmsCert')
    expect(wrapper.text()).toContain('extensionPayloads')
  })

  it('parses transport and dimension fields from pasted YAML before creating a profile', async () => {
    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: {
        show: true,
      },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    const createOpenButton = wrapper.findAll('button').find(button => button.text().includes('createProfile'))
    expect(createOpenButton).toBeTruthy()
    await createOpenButton!.trigger('click')
    await flushPromises()

    const yamlTextarea = wrapper.find('textarea[placeholder="admin.tlsFingerprintProfiles.form.pasteYamlPlaceholder"]')
    expect(yamlTextarea.exists()).toBe(true)
    await yamlTextarea.setValue(`
name: "Captured Codex"
platform: "openai"
transport: "h2"
os: "macos"
client_type: "codex-cli"
http2_fingerprint: "1:65536;2:0|15663105|m,a,s,p"
enable_grease: false
cipher_suites: [4865, 4866]
curves: [29, 23]
point_formats: [0]
signature_algorithms: [1027]
alpn_protocols: ["h2", "http/1.1"]
supported_versions: [772, 771]
key_share_groups: [29]
psk_modes: [1]
extensions: [0, 10, 11, 13, 16, 43, 45, 51]
`)
    const parseButton = wrapper.findAll('button').find(button => button.text().includes('parseYaml'))
    expect(parseButton).toBeTruthy()
    await parseButton!.trigger('click')

    const submitButton = wrapper.findAll('button').find(button => button.text().includes('common.create'))
    expect(submitButton).toBeTruthy()
    await submitButton!.trigger('click')
    await flushPromises()

    expect(createProfileMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Captured Codex',
      platform: 'openai',
      transport: 'h2',
      os: 'macos',
      client_type: 'codex-cli',
      http2_fingerprint: '1:65536;2:0|15663105|m,a,s,p',
    }))
  })

  it('loads selected task details and preserves its token across redacted polling responses', async () => {
    vi.useFakeTimers()
    const redactedTask = {
      id: 42,
      name: 'Live capture',
      status: 'running',
      targets: { openai: 1 },
      counts: { openai: 0 },
      ua_keywords: [],
      capture_url: 'https://localhost:8444/capture',
      created_at: '2026-07-14T00:00:00Z',
      updated_at: '2026-07-14T00:00:00Z',
    }
    listCaptureTasksMock.mockResolvedValue([redactedTask])
    getCaptureTaskMock
      .mockResolvedValueOnce({ ...redactedTask, token: 'detail-token' })
      .mockResolvedValue(redactedTask)

    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    expect(getCaptureTaskMock).toHaveBeenCalledWith(42)
    await wrapper.findAll('button').find(button => button.text() === 'Capture')!.trigger('click')
    expect(wrapper.find('a[href*="tls-fingerprint-collector"]').attributes('href')).toContain('token=detail-token')

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(getCaptureTaskMock).toHaveBeenCalledTimes(2)
    expect(wrapper.find('a[href*="tls-fingerprint-collector"]').attributes('href')).toContain('token=detail-token')
    wrapper.unmount()
  })

  it('loads task details when the operator selects a different capture task', async () => {
    const firstTask = {
      id: 1,
      name: 'First task',
      status: 'stopped',
      targets: { openai: 1 },
      counts: {},
      ua_keywords: [],
      created_at: '2026-07-14T00:00:00Z',
      updated_at: '2026-07-14T00:00:00Z',
    }
    const secondTask = { ...firstTask, id: 2, name: 'Second task' }
    listCaptureTasksMock.mockResolvedValue([firstTask, secondTask])
    getCaptureTaskMock.mockImplementation(async (id: number) => ({
      ...(id === 1 ? firstTask : secondTask),
      token: `token-${id}`,
    }))
    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text() === 'Capture')!.trigger('click')

    await wrapper.findAll('button').find(button => button.text().includes('Second task'))!.trigger('click')
    await flushPromises()

    expect(getCaptureTaskMock).toHaveBeenCalledWith(2)
    expect(wrapper.find('a[href*="tls-fingerprint-collector"]').attributes('href')).toContain('token=token-2')
    wrapper.unmount()
  })

  it('submits the opt-in request body capture filter only when enabled', async () => {
    startCaptureTaskMock.mockResolvedValue({
      id: 7,
      name: 'Body capture',
      status: 'running',
      token: 'token',
      targets: { openai: 100 },
      counts: {},
      ua_keywords: [],
      created_at: '2026-07-14T00:00:00Z',
      updated_at: '2026-07-14T00:00:00Z',
    })
    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.findAll('button').find(button => button.text() === 'Capture')!.trigger('click')
    await wrapper.get('[data-testid="capture-store-body"]').setValue(true)
    await wrapper.findAll('button').find(button => button.text().includes('capture.start'))!.trigger('click')
    await flushPromises()

    expect(startCaptureTaskMock).toHaveBeenCalledWith(expect.objectContaining({
      capture_filters: { store_body: true },
    }))
    wrapper.unmount()
  })

  it('hydrates and updates http2_fingerprint for an existing H2 profile', async () => {
    listProfilesMock.mockResolvedValue([{
      id: 9,
      platform: 'openai',
      transport: 'h2',
      os: 'linux',
      client_type: 'codex-cli',
      name: 'H2 profile',
      user_agent: '',
      originator: '',
      http2_fingerprint: 'old-h2-fingerprint',
      description: null,
      enable_grease: false,
      cipher_suites: [],
      curves: [],
      point_formats: [],
      signature_algorithms: [],
      signature_algorithms_cert: [],
      alpn_protocols: ['h2'],
      supported_versions: [],
      key_share_groups: [],
      psk_modes: [],
      extensions: [],
      extension_payloads: {},
      compress_cert_algos: [],
      delegated_credentials_algorithms: [],
      application_settings_protocols: [],
      created_at: '2026-07-14T00:00:00Z',
      updated_at: '2026-07-14T00:00:00Z',
    }])
    const wrapper = mount(TLSFingerprintProfilesModal, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          ConfirmDialog: true,
          Icon: true,
        },
      },
    })
    await flushPromises()

    await wrapper.get('button[title="common.edit"]').trigger('click')
    const fingerprint = wrapper.get('[data-testid="http2-fingerprint"]')
    expect((fingerprint.element as HTMLTextAreaElement).value).toBe('old-h2-fingerprint')
    await fingerprint.setValue('new-h2-fingerprint')
    await wrapper.findAll('button').find(button => button.text().includes('common.update'))!.trigger('click')
    await flushPromises()

    expect(updateProfileMock).toHaveBeenCalledWith(9, expect.objectContaining({
      http2_fingerprint: 'new-h2-fingerprint',
    }))
  })
})
