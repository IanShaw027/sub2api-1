import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import TLSFingerprintProfilesModal from '../TLSFingerprintProfilesModal.vue'

const {
  listProfilesMock,
  listCaptureTasksMock,
  showErrorMock,
} = vi.hoisted(() => ({
  listProfilesMock: vi.fn(),
  listCaptureTasksMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tlsFingerprintProfiles: {
      list: listProfilesMock,
      listCaptureTasks: listCaptureTasksMock,
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
  template: '<div v-if="show"><slot /></div>',
})

describe('TLSFingerprintProfilesModal', () => {
  beforeEach(() => {
    listProfilesMock.mockReset()
    listCaptureTasksMock.mockReset()
    showErrorMock.mockReset()
    listProfilesMock.mockResolvedValue([])
    listCaptureTasksMock.mockResolvedValue([])
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
})
