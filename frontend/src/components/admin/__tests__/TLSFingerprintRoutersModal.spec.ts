import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'

const {
  showErrorMock,
  showSuccessMock,
  listRoutersMock,
  createRouterMock,
  updateRouterMock,
  deleteRouterMock,
  listProfilesMock
} = vi.hoisted(() => ({
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  listRoutersMock: vi.fn(),
  createRouterMock: vi.fn(),
  updateRouterMock: vi.fn(),
  deleteRouterMock: vi.fn(),
  listProfilesMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tlsFingerprintRouters: {
      list: listRoutersMock,
      create: createRouterMock,
      update: updateRouterMock,
      delete: deleteRouterMock
    },
    tlsFingerprintProfiles: {
      list: listProfilesMock
    }
  }
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (params?.count !== undefined) return `${key}:${params.count}`
      return key
    }
  })
}))

import TLSFingerprintRoutersModal from '../TLSFingerprintRoutersModal.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const ConfirmDialogStub = defineComponent({
  name: 'ConfirmDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /></div>'
})

const IconStub = defineComponent({
  name: 'Icon',
  template: '<span />'
})

function mountModal() {
  return mount(TLSFingerprintRoutersModal, {
    props: { show: true },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: ConfirmDialogStub,
        Icon: IconStub
      }
    }
  })
}

describe('TLSFingerprintRoutersModal', () => {
  beforeEach(() => {
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    listRoutersMock.mockReset()
    createRouterMock.mockReset()
    updateRouterMock.mockReset()
    deleteRouterMock.mockReset()
    listProfilesMock.mockReset()
    listRoutersMock.mockResolvedValue([])
    listProfilesMock.mockResolvedValue([])
    createRouterMock.mockResolvedValue({})
  })

  it('loads routers and profiles on first mount when already shown', async () => {
    listRoutersMock.mockResolvedValue([
      {
        id: 7,
        name: 'UA Router',
        description: null,
        enabled: true,
        rules: [],
        created_at: '2026-06-18T00:00:00Z',
        updated_at: '2026-06-18T00:00:00Z'
      }
    ])

    const wrapper = mountModal()
    await flushPromises()

    expect(listRoutersMock).toHaveBeenCalledTimes(1)
    expect(listProfilesMock).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('UA Router')
  })

  it('creates an empty-rule router instead of submitting enabled profile zero when no profiles exist', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const createOpenButton = wrapper.findAll('button').find(button => button.text().includes('createRouter'))
    expect(createOpenButton).toBeTruthy()
    await createOpenButton!.trigger('click')

    const addRuleButton = wrapper.findAll('button').find(button => button.text().includes('addRule'))
    expect(addRuleButton).toBeTruthy()
    expect(addRuleButton!.attributes('disabled')).toBeDefined()

    const nameInput = wrapper.find('input[type="text"]')
    await nameInput.setValue('Empty Profile Router')

    const submitButton = wrapper.findAll('button').find(button => button.text().includes('common.create'))
    expect(submitButton).toBeTruthy()
    await submitButton!.trigger('click')
    await flushPromises()

    expect(createRouterMock).toHaveBeenCalledWith({
      name: 'Empty Profile Router',
      description: null,
      enabled: true,
      rules: []
    })
  })

  it('preserves rule transport when creating a router', async () => {
    listProfilesMock.mockResolvedValue([
      {
        id: 42,
        name: 'Codex WS',
        platform: 'openai',
        transport: 'websocket-h2',
        os: '',
        client_type: '',
      },
    ])
    const wrapper = mountModal()
    await flushPromises()

    const createOpenButton = wrapper.findAll('button').find(button => button.text().includes('createRouter'))
    expect(createOpenButton).toBeTruthy()
    await createOpenButton!.trigger('click')

    const textInputs = wrapper.findAll('input[type="text"]')
    expect(textInputs.length).toBeGreaterThanOrEqual(4)
    await textInputs[0].setValue('WS Router')
    await textInputs[2].setValue('codex ws')
    await textInputs[3].setValue('codex')

    const transportSelect = wrapper.findAll('select').find(select => select.find('option[value="websocket"]').exists())
    expect(transportSelect).toBeTruthy()
    await transportSelect!.setValue('websocket')

    const submitButton = wrapper.findAll('button').find(button => button.text().includes('common.create'))
    expect(submitButton).toBeTruthy()
    await submitButton!.trigger('click')
    await flushPromises()

    expect(createRouterMock).toHaveBeenCalledWith({
      name: 'WS Router',
      description: null,
      enabled: true,
      rules: [
        expect.objectContaining({
          name: 'codex ws',
          transport: 'websocket',
          pattern: 'codex',
          tls_fingerprint_profile_id: 42,
        }),
      ],
    })
  })
})
