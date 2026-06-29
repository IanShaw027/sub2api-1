import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import TLSFingerprintProfilesModal from '../TLSFingerprintProfilesModal.vue'

const {
  listProfilesMock,
  listCaptureTasksMock,
  createProfileMock,
  showErrorMock,
} = vi.hoisted(() => ({
  listProfilesMock: vi.fn(),
  listCaptureTasksMock: vi.fn(),
  createProfileMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tlsFingerprintProfiles: {
      list: listProfilesMock,
      listCaptureTasks: listCaptureTasksMock,
      create: createProfileMock,
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
    createProfileMock.mockReset()
    showErrorMock.mockReset()
    listProfilesMock.mockResolvedValue([])
    listCaptureTasksMock.mockResolvedValue([])
    createProfileMock.mockResolvedValue({})
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
})
