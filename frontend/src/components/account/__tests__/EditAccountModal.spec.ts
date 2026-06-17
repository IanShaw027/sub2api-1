import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

const {
  showErrorMock,
  updateAccountMock,
  checkMixedChannelRiskMock,
  getSettingsMock,
  getWebSearchEmulationConfigMock,
  listTlsFingerprintProfilesMock,
  listTlsFingerprintRoutersMock,
  getPaymentConfigMock,
  adminSettingsStoreMock
} = vi.hoisted(() => {
  const adminSettingsStoreMock = {
    loaded: false,
    platformDefaultAccountModelConfig: {} as Record<string, any>,
    fetch: vi.fn(async () => {
      const settings = await getSettingsMock()
      adminSettingsStoreMock.platformDefaultAccountModelConfig = settings?.platform_default_account_model_config || {}
      adminSettingsStoreMock.loaded = true
    })
  }
  return {
    showErrorMock: vi.fn(),
    updateAccountMock: vi.fn(),
    checkMixedChannelRiskMock: vi.fn(),
    getSettingsMock: vi.fn(),
    getWebSearchEmulationConfigMock: vi.fn(),
    listTlsFingerprintProfilesMock: vi.fn(),
    listTlsFingerprintRoutersMock: vi.fn(),
    getPaymentConfigMock: vi.fn(),
    adminSettingsStoreMock
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => adminSettingsStoreMock
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    settings: {
      getSettings: getSettingsMock,
      getWebSearchEmulationConfig: getWebSearchEmulationConfigMock
    },
    payment: {
      getConfig: getPaymentConfigMock
    },
    tlsFingerprintProfiles: {
      list: listTlsFingerprintProfilesMock
    },
    tlsFingerprintRouters: {
      list: listTlsFingerprintRoutersMock
    },
    accounts: {
      update: updateAccountMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'zh-CN' }
    })
  }
})

import EditAccountModal from '../EditAccountModal.vue'

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

const ModelWhitelistSelectorStub = defineComponent({
  name: 'ModelWhitelistSelector',
  props: {
    modelValue: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <div>
      <button
        type="button"
        data-testid="rewrite-to-snapshot"
        @click="$emit('update:modelValue', ['gpt-5.2-2025-12-11'])"
      >
        rewrite
      </button>
      <span data-testid="model-whitelist-value">
        {{ Array.isArray(modelValue) ? modelValue.join(',') : '' }}
      </span>
    </div>
  `
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

function buildAccount() {
  return {
    id: 1,
    name: 'OpenAI Key',
    notes: '',
    platform: 'openai',
    type: 'apikey',
    credentials: {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com',
      model_mapping: {
        'gpt-5.2': 'gpt-5.2'
      }
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildVertexAccount() {
  return {
    id: 2,
    name: 'Vertex SA',
    notes: '',
    platform: 'gemini',
    type: 'service_account',
    credentials: {
      service_account_json: '{"type":"service_account","client_email":"sa@example.iam.gserviceaccount.com"}',
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'active',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function resetCommonMocks() {
  showErrorMock.mockReset()
  updateAccountMock.mockReset()
  checkMixedChannelRiskMock.mockReset()
  getSettingsMock.mockReset()
  getWebSearchEmulationConfigMock.mockReset()
  listTlsFingerprintProfilesMock.mockReset()
  listTlsFingerprintRoutersMock.mockReset()
  getPaymentConfigMock.mockReset()
  adminSettingsStoreMock.loaded = false
  adminSettingsStoreMock.platformDefaultAccountModelConfig = {}
  adminSettingsStoreMock.fetch.mockReset()
  adminSettingsStoreMock.fetch.mockImplementation(async () => {
    const settings = await getSettingsMock()
    adminSettingsStoreMock.platformDefaultAccountModelConfig = settings?.platform_default_account_model_config || {}
    adminSettingsStoreMock.loaded = true
  })
  getSettingsMock.mockResolvedValue({})
  getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
  listTlsFingerprintProfilesMock.mockResolvedValue([])
  listTlsFingerprintRoutersMock.mockResolvedValue([])
  getPaymentConfigMock.mockResolvedValue({ data: { enabled: false } })
  checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
}

function mountModal(account = buildAccount(), show = false) {
  return mount(EditAccountModal, {
    props: {
      show,
      account,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Select: SelectStub,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: ModelWhitelistSelectorStub
      }
    }
  })
}

describe('EditAccountModal', () => {
  it('mounts without initialization errors when initially opened for Kiro accounts', () => {
    resetCommonMocks()
    const account = {
      ...buildAccount(),
      id: 20,
      name: 'Kiro OAuth',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        model_mapping: {
          'claude-sonnet-*': 'claude-sonnet-4.5'
        }
      },
      extra: {}
    } as any

    expect(() => mountModal(account, true)).not.toThrow()
  })

  it('offers OpenAI service-unavailable temp-unschedulable presets as separate rules', async () => {
    resetCommonMocks()
    const wrapper = mountModal()

    const presetRules = (wrapper.vm as any).tempUnschedPresets.map((preset: any) => preset.rule)

    expect(presetRules).toEqual(expect.arrayContaining([
      expect.objectContaining({
        error_code: 503,
        keywords: 'Service temporarily unavailable',
        duration_minutes: 10
      }),
      expect.objectContaining({
        error_code: 503,
        keywords: 'overloaded',
        duration_minutes: 10
      })
    ]))
  })

  it('blocks mixed valid and invalid temp-unschedulable rules on edit', async () => {
    resetCommonMocks()
    const account = buildAccount()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    ;(wrapper.vm as any).tempUnschedEnabled = true
    ;(wrapper.vm as any).tempUnschedRules = [
      {
        error_code: 524,
        keywords: '',
        duration_minutes: 10,
        description: 'Cloudflare timeout'
      },
      {
        error_code: 99,
        keywords: 'invalid',
        duration_minutes: 10,
        description: 'bad status'
      }
    ]

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.tempUnschedulable.rulesInvalid')
    expect(updateAccountMock).not.toHaveBeenCalled()
  })

  it('blocks invalid loaded custom error codes on edit instead of saving them', async () => {
    resetCommonMocks()
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      custom_error_codes_enabled: true,
      custom_error_codes: [429, 502.5]
    }
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.invalidErrorCode')
    expect(updateAccountMock).not.toHaveBeenCalled()
  })

  it('edits account scheduling threshold override for supported OpenAI accounts', async () => {
    resetCommonMocks()
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      account_scheduling_threshold: 82
    }
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    const enabled = wrapper.get('[data-testid="account-scheduling-threshold-override-enabled"]')
      .element as HTMLInputElement
    expect(enabled.checked).toBe(true)
    const input = wrapper.get('[data-testid="account-scheduling-threshold-override-value"]')
    expect((input.element as HTMLInputElement).value).toBe('82')

    await input.setValue('75')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalled()
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.account_scheduling_threshold).toBe(75)
  })

  it('does not show account scheduling threshold override for unsupported Kiro accounts', async () => {
    resetCommonMocks()
    const account = buildAccount()
    account.platform = 'kiro'
    account.type = 'oauth'

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.find('[data-testid="account-scheduling-threshold-override-enabled"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="account-scheduling-threshold-override-value"]').exists()).toBe(false)
  })

  it('does not show OpenAI WebProfile controls', async () => {
    resetCommonMocks()
    const wrapper = mountModal({
      ...buildAccount(),
      extra: {
        web_profile: {
          source: 'should-not-render'
        }
      }
    } as any)

    await wrapper.setProps({ show: true })

    expect(wrapper.find('[data-testid="openai-web-profile-section"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('should-not-render')
  })

  it('shows Gemini read-only account summary in edit modal', async () => {
    resetCommonMocks()
    const wrapper = mountModal({
      ...buildAccount(),
      id: 9,
      name: 'Gemini OAuth',
      platform: 'gemini',
      type: 'oauth',
      credentials: {
        oauth_type: 'google_one',
        tier_id: 'google_ai_pro',
        plan_name: 'Gemini Code Assist in Google One AI Pro',
        email: 'gemini@example.com',
        project_id: 'refreshing-center-hnmwg',
        scope: 'openid https://www.googleapis.com/auth/cloud-platform',
        gemini_available_credits: [
          {
            creditType: 'GOOGLE_ONE_AI',
            creditAmount: '100'
          }
        ]
      }
    } as any)

    await wrapper.setProps({ show: true })

    const section = wrapper.get('[data-testid="gemini-account-summary-section"]')
    expect(section.text()).toContain('admin.accounts.geminiAccount')
    expect(section.text()).toContain('Google One Pro')
  })

  it('reopening the same account rehydrates the OpenAI whitelist from props', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2-2025-12-11')

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('gpt-5.2')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2': 'gpt-5.2'
    })
  })

  it('preserves allow-all model access when OpenAI or Anthropic accounts have no explicit whitelist', async () => {
    const openAIAccount = buildAccount()
    openAIAccount.credentials = {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com'
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(openAIAccount)

    const openAIWrapper = mountModal(openAIAccount)
    await openAIWrapper.setProps({ show: true })
    expect(openAIWrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('')
    await openAIWrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('model_mapping')

    const anthropicWrapper = mountModal({
      ...buildAccount(),
      id: 5,
      name: 'Claude Key',
      platform: 'anthropic',
      credentials: {
        api_key: 'sk-ant-test',
        base_url: 'https://api.anthropic.com'
      }
    } as any)
    await anthropicWrapper.setProps({ show: true })
    expect(anthropicWrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('')
  })

  it('allows API key accounts to save when the stored api_key is redacted in the detail payload', async () => {
    const account = buildAccount()
    account.credentials = {
      base_url: 'https://api.openai.com'
    }
    account.credentials_status = { has_api_key: true }

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toEqual({
      base_url: 'https://api.openai.com'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('api_key')
  })

  it('allows API key accounts to save against legacy credentials without credentials_status', async () => {
    const account = buildAccount()

    resetCommonMocks()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.api_key).toBe('sk-test')
  })

  it('allows Vertex service accounts to save when service account JSON is redacted', async () => {
    const account = buildVertexAccount()
    account.credentials = {
      project_id: 'demo-project',
      client_email: 'sa@example.iam.gserviceaccount.com',
      location: 'us-central1',
      tier_id: 'vertex'
    }
    account.credentials_status = { has_service_account_json: true }

    resetCommonMocks()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.project_id).toBe('demo-project')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('service_account_json')
  })

  it('allows Vertex service accounts to save against legacy credentials without credentials_status', async () => {
    const account = buildVertexAccount()

    resetCommonMocks()
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.service_account_json).toContain('service_account')
  })

  it('updates OpenAI TLS fingerprint settings in extra', async () => {
    const account = {
      ...buildAccount(),
      extra: {
        keep_flag: true,
        enable_tls_fingerprint: false
      }
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    listTlsFingerprintRoutersMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([{ id: 12, name: 'Chrome 124' }])
    listTlsFingerprintRoutersMock.mockResolvedValue([{ id: 9, name: 'UA Router' }])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.vm.$nextTick()
    await flushPromises()

    await wrapper.get('[data-testid="openai-tls-fingerprint-toggle"]').trigger('click')
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-testid="openai-tls-fingerprint-profile"]').setValue('12')
    expect(wrapper.get('[data-testid="openai-tls-fingerprint-router"]').text()).toContain('UA Router')
    await wrapper.get('[data-testid="openai-tls-fingerprint-router"]').setValue('9')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toEqual(expect.objectContaining({
      keep_flag: true,
      enable_tls_fingerprint: true,
      tls_fingerprint_profile_id: 12,
      tls_fingerprint_router_id: 9
    }))
  })

  it('removes Kiro runtime version overrides from extra without resending unchanged Kiro OAuth credentials', async () => {
    const account = {
      id: 2,
      name: 'Kiro OAuth',
      notes: '',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        refresh_token: 'rt-test',
        region: 'us-east-1',
        machine_id: 'machine-1',
        model_mapping: {
          'claude-sonnet-*': 'claude-sonnet-4.5'
        }
      },
      extra: {
        keep_flag: true,
        kiro_version: '0.9.0',
        system_version: 'darwin#24.5.0',
        node_version: '22.20.0'
      },
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.text()).not.toContain('admin.accounts.kiro.kiroVersionLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.systemVersionLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.nodeVersionLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.refreshTokenLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.accessTokenLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.clientSecretLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.expiresAtLabel')
    expect((wrapper.get('input[placeholder="admin.accounts.requestModel"]').element as HTMLInputElement).value).toBe('claude-sonnet-*')
    expect((wrapper.get('input[placeholder="admin.accounts.actualModel"]').element as HTMLInputElement).value).toBe('claude-sonnet-4.5')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toBeUndefined()
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toEqual({
      keep_flag: true
    })
  })

  it('submits empty extra when Kiro OAuth only has old runtime overrides', async () => {
    const account = {
      id: 3,
      name: 'Kiro OAuth',
      notes: '',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        refresh_token: 'rt-test',
        region: 'us-east-1'
      },
      extra: {
        kiro_version: '0.9.0',
        system_version: 'darwin#24.5.0',
        node_version: '22.20.0'
      },
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toEqual({})
  })

  it('rehydrates Kiro legacy model_whitelist but persists through model_mapping', async () => {
    const account = {
      id: 8,
      name: 'Kiro OAuth',
      notes: '',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        model_whitelist: ['claude-sonnet-4.5'],
        model_mapping: {
          'claude-sonnet-4.5': 'claude-sonnet-4.5'
        }
      },
      extra: {},
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="model-whitelist-value"]').text()).toBe('claude-sonnet-4.5')

    const mappingButton = wrapper.findAll('button').find((btn) => btn.text().includes('admin.accounts.modelMapping'))
    expect(mappingButton).toBeTruthy()
    await mappingButton!.trigger('click')
    expect((wrapper.get('input[placeholder="admin.accounts.requestModel"]').element as HTMLInputElement).value).toBe('claude-sonnet-4.5')
    expect((wrapper.get('input[placeholder="admin.accounts.actualModel"]').element as HTMLInputElement).value).toBe('claude-sonnet-4.5')

    const whitelistButton = wrapper.findAll('button').find((btn) => btn.text().includes('admin.accounts.modelWhitelist'))
    expect(whitelistButton).toBeTruthy()
    await whitelistButton!.trigger('click')
    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_whitelist).toBeNull()
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'gpt-5.2-2025-12-11': 'gpt-5.2-2025-12-11'
    })
  })

  it('offers to sync Kiro mapping to the plan default and keeps dismissals scoped to the current mapping', async () => {
    const account = {
      id: 10,
      name: 'Kiro Pro',
      notes: '',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        refresh_token: 'rt-test',
        subscription_type: 'Kiro Pro',
        model_mapping: {
          'claude-opus-*': 'claude-opus-4.6'
        }
      },
      extra: {},
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({
      platform_default_account_model_config: {
        kiro: {
          kiro_subscription_type_model_config: {
            pro: {
              model_mapping: {
                'claude-opus-*': 'claude-opus-4.7'
              }
            }
          }
        }
      }
    })
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(updateAccountMock).toHaveBeenCalledTimes(0)
    expect(wrapper.text()).toContain('当前 Kiro pro 账号的模型映射与默认值不一致')

    const syncButton = wrapper.findAll('button').find((btn) => btn.text().includes('同步默认值'))
    expect(syncButton).toBeTruthy()
    await syncButton!.trigger('click')
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'claude-opus-*': 'claude-opus-4.7'
    })

    updateAccountMock.mockResolvedValue(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock).toHaveBeenCalledTimes(2)
  })

  it('normalizes Kiro [1m] aliases to canonical models on load and save', async () => {
    const account = {
      id: 12,
      name: 'Kiro 1M',
      notes: '',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        refresh_token: 'rt-test',
        model_mapping: {
          'claude-opus-4.7[1m]': 'claude-opus-4.6[1m]'
        }
      },
      extra: {},
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect((wrapper.get('input[placeholder="admin.accounts.requestModel"]').element as HTMLInputElement).value).toBe('claude-opus-4.7')
    expect((wrapper.get('input[placeholder="admin.accounts.actualModel"]').element as HTMLInputElement).value).toBe('claude-opus-4.6')

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')
    expect(updateAccountMock).toHaveBeenCalledTimes(1)

    const submittedMapping = updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping as
      | Record<string, string>
      | undefined
    if (submittedMapping) {
      expect(submittedMapping).toEqual({
        'claude-opus-4.7': 'claude-opus-4.6'
      })
    }
  })

  it('updates changed Kiro API key credentials and metadata', async () => {
    const account = {
      id: 4,
      name: 'Kiro API Key',
      notes: '',
      platform: 'kiro',
      type: 'apikey',
      credentials: {
        api_key: 'kiro-old-key',
        region: 'eu-west-1',
        auth_region: 'us-west-2',
        api_region: 'us-east-2',
        profile_arn: 'arn:aws:bedrock:us-east-2:123456789012:inference-profile/demo',
        machine_id: 'machine-1',
        base_url: 'https://wrong.example.com',
        model_whitelist: ['claude-sonnet-4'],
        model_mapping: {
          'claude-sonnet-4': 'claude-sonnet-4'
        },
        pool_mode: true,
        pool_mode_retry_count: 3,
        custom_error_codes_enabled: true,
        custom_error_codes: [429, 529],
        custom_error_codes_backoff_ms: 1000
      },
      extra: {
        kiro_version: '0.9.0',
        quota_limit: 100
      },
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.text()).not.toContain('admin.accounts.baseUrl')
    const apiKeyInput = wrapper.get('input[placeholder="admin.accounts.kiro.apiKeyPlaceholder"]')
    await apiKeyInput.setValue('kiro-new-key')
    expect(wrapper.find('input[placeholder="admin.accounts.kiro.regionPlaceholder"]').exists()).toBe(false)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toEqual({
      api_key: 'kiro-new-key',
      model_whitelist: null
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('base_url')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toEqual({
      quota_limit: 100
    })
  })

  it('allows Kiro OAuth edits to submit without requiring hidden refresh/client secrets', async () => {
    const account = {
      id: 6,
      name: 'Kiro OAuth',
      notes: '',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        region: 'us-east-1',
        auth_method: 'social'
      },
      extra: {},
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.refreshTokenLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.clientSecretLabel')
    expect(wrapper.find('input[placeholder="admin.accounts.kiro.regionPlaceholder"]').exists()).toBe(false)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toBeUndefined()
  })

  it('does not expose or mutate Kiro OAuth authentication credentials', async () => {
    const account = {
      id: 7,
      name: 'Kiro OAuth',
      notes: '',
      platform: 'kiro',
      type: 'oauth',
      credentials: {
        refresh_token: 'rt-test',
        access_token: 'at-test',
        client_id: 'client-id',
        client_secret: 'client-secret',
        expires_at: '2026-05-01T12:30:00Z',
        region: 'us-east-1',
        auth_method: 'social'
      },
      extra: {},
      proxy_id: null,
      concurrency: 1,
      priority: 1,
      rate_multiplier: 1,
      status: 'active',
      group_ids: [],
      expires_at: null,
      auto_pause_on_expired: false
    } as any

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.authManagedByReauth')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.refreshTokenLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.accessTokenLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.clientIdLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.clientSecretLabel')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.expiresAtLabel')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toBeUndefined()
  })

  it('submits OpenAI compact mode and compact-only model mapping', async () => {
    const account = buildAccount()
    account.extra = {
      openai_compact_mode: 'force_on'
    }
    account.credentials = {
      ...account.credentials,
      compact_model_mapping: {
        'gpt-5.4': 'gpt-5.4-openai-compact'
      }
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_compact_mode).toBe('force_on')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.compact_model_mapping).toEqual({
      'gpt-5.4': 'gpt-5.4-openai-compact'
    })
  })

  it('submits only an OpenAI OAuth credential patch when details are sanitized', async () => {
    const account = buildAccount()
    account.platform = 'openai'
    account.type = 'oauth'
    account.credentials = {
      email: 'user@example.com',
      id_token: 'redacted-detail-id-token',
      model_mapping: {
        'gpt-5.2': 'gpt-5.2'
      }
    }

    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getSettingsMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    getSettingsMock.mockResolvedValue({})
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })
    await wrapper.get('[data-testid="rewrite-to-snapshot"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toEqual({
      model_mapping: {
        'gpt-5.2-2025-12-11': 'gpt-5.2-2025-12-11'
      }
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('email')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('id_token')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('access_token')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('refresh_token')
  })

  it('updates the OpenAI compact status label from current form state', async () => {
    const account = buildAccount()
    account.extra = {
      openai_compact_mode: 'force_on',
      openai_compact_supported: false,
      openai_compact_checked_at: '2026-04-01T12:00:00Z'
    }

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.text()).toContain('admin.accounts.openai.compactSupported')

    const compactModeSelect = wrapper.findAll('select').find((candidate) =>
      candidate.find('option[value="force_on"]').exists()
    )
    expect(compactModeSelect).toBeTruthy()

    await compactModeSelect!.setValue('force_off')

    expect(wrapper.text()).toContain('admin.accounts.openai.compactUnsupported')
    expect(wrapper.text()).not.toContain('admin.accounts.openai.compactSupported')
  })

  it('submits OpenAI APIKey endpoint capabilities from credentials', async () => {
    const account = buildAccount()
    account.credentials.openai_capabilities = ['chat_completions']
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.findAll('input[type="checkbox"]').some((input) => (input.element as HTMLInputElement).checked)).toBe(true)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.openai_capabilities).toEqual([
      'chat_completions'
    ])
  })

  it('clears OpenAI APIKey endpoint capability override when restoring both capabilities', async () => {
    const account = buildAccount()
    account.credentials.openai_capabilities = ['embeddings']
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    const chatCheckbox = wrapper.get<HTMLInputElement>(
      '[data-testid="openai-endpoint-capability-chat_completions"]'
    )
    const embeddingsCheckbox = wrapper.get<HTMLInputElement>(
      '[data-testid="openai-endpoint-capability-embeddings"]'
    )

    expect(chatCheckbox.element.checked).toBe(false)
    expect(embeddingsCheckbox.element.checked).toBe(true)

    await chatCheckbox.setValue(true)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toHaveProperty(
      'openai_capabilities',
      null
    )
  })

  it('hides OpenAI quota auto-pause overrides for unsupported account types', async () => {
    const account = buildAccount()
    account.type = 'setup-token'
    account.credentials = {
      access_token: 'at-test'
    }

    resetCommonMocks()
    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.find('[data-testid="auto-pause-5h-disabled"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="auto-pause-5h-threshold"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="auto-pause-7d-disabled"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="auto-pause-7d-threshold"]').exists()).toBe(false)
  })

  it('hides OpenAI API key responses mode controls for OAuth accounts', async () => {
    const account = buildAccount()
    account.type = 'oauth'
    account.credentials = {
      access_token: 'at-test'
    }
    account.extra = {
      openai_responses_mode: 'force_responses'
    }

    resetCommonMocks()
    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.find('[data-testid="openai-responses-mode-select"]').exists()).toBe(false)
  })

  it('keeps at least one OpenAI APIKey endpoint capability selected', async () => {
    const account = buildAccount()
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    const chatCheckbox = wrapper.get<HTMLInputElement>(
      '[data-testid="openai-endpoint-capability-chat_completions"]'
    )
    const embeddingsCheckbox = wrapper.get<HTMLInputElement>(
      '[data-testid="openai-endpoint-capability-embeddings"]'
    )

    expect(chatCheckbox.element.checked).toBe(true)
    expect(embeddingsCheckbox.element.checked).toBe(true)

    await embeddingsCheckbox.setValue(false)

    expect(chatCheckbox.element.checked).toBe(true)
    expect(embeddingsCheckbox.element.checked).toBe(false)

    await chatCheckbox.setValue(false)

    expect(chatCheckbox.element.checked).toBe(true)
    expect(embeddingsCheckbox.element.checked).toBe(false)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.openai_capabilities).toEqual([
      'chat_completions'
    ])
  })

  it('disables text generation protocol when only embeddings requests are accepted', async () => {
    const account = buildAccount()
    account.credentials.openai_capabilities = ['embeddings']
    account.extra = {
      openai_responses_mode: 'force_responses',
      openai_responses_supported: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    const responsesModeSelect = wrapper.get<HTMLSelectElement>(
      '[data-testid="openai-responses-mode-select"]'
    )

    expect(responsesModeSelect.element.disabled).toBe(true)
    expect(wrapper.find('[data-testid="openai-responses-mode-not-applicable"]').exists()).toBe(true)

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.openai_capabilities).toEqual([
      'embeddings'
    ])
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('openai_responses_mode')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_responses_supported).toBe(true)
  })

  it('submits account-level Codex image generation bridge override', async () => {
    const account = buildAccount()
    account.extra = {
      codex_image_generation_bridge: false,
      codex_image_generation_bridge_enabled: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    await wrapper.get('button[data-testid="codex-image-bridge-enabled"]').trigger('click')
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.codex_image_generation_bridge).toBe(true)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).not.toHaveProperty('codex_image_generation_bridge_enabled')
  })

  it('hydrates OpenAI image generation from the account field and preserves explicit false on save', async () => {
    const account = buildAccount()
    account.openai_image_generation_enabled = false
    account.extra = {
      openai_image_generation_enabled: true
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect((wrapper.vm as any).openaiImageGenerationEnabled).toBe(false)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_image_generation_enabled).toBe(false)
  })

  it('falls back to legacy extra OpenAI image generation when the account field is absent', async () => {
    const account = buildAccount()
    account.extra = {
      openai_image_generation_enabled: false
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect((wrapper.vm as any).openaiImageGenerationEnabled).toBe(false)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_image_generation_enabled).toBe(false)
  })

  it('defaults OpenAI image generation to enabled when neither account shape provides a value', async () => {
    const account = buildAccount()
    account.extra = {}
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect((wrapper.vm as any).openaiImageGenerationEnabled).toBe(true)
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra?.openai_image_generation_enabled).toBe(true)
  })

  it('loads and patches OpenAI response rewrite rules', async () => {
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      response_rewrite_rules: [
        {
          status_code: 401,
          keywords: ['token_invalidated', 'auth'],
          match_mode: 'all',
          response_message: 'Service temporarily unavailable',
          description: 'masked auth'
        }
      ]
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    expect(wrapper.get('[data-testid="openai-response-rewrite-section"]').exists()).toBe(true)
    expect((wrapper.vm as any).responseRewriteRules).toEqual([
      {
        status_code: 401,
        keywords: 'token_invalidated, auth',
        match_mode: 'all',
        response_message: 'Service temporarily unavailable',
        description: 'masked auth'
      }
    ])

    ;(wrapper.vm as any).responseRewriteRules = [
      {
        status_code: 429,
        keywords: 'too many requests',
        match_mode: 'any',
        response_message: 'Retry later',
        description: 'mask 429'
      }
    ]

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.response_rewrite_rules).toEqual([
      {
        status_code: 429,
        keywords: ['too many requests'],
        match_mode: 'any',
        response_message: 'Retry later',
        description: 'mask 429'
      }
    ])
  })

  it('clears OpenAI API key response rewrite rules explicitly', async () => {
    resetCommonMocks()
    const account = buildAccount()
    account.credentials = {
      ...account.credentials,
      response_rewrite_rules: [
        {
          status_code: 401,
          keywords: ['token_invalidated'],
          match_mode: 'any',
          response_message: 'Service temporarily unavailable'
        }
      ]
    }
    updateAccountMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    updateAccountMock.mockResolvedValue(account)

    const wrapper = mountModal(account)
    await wrapper.setProps({ show: true })

    ;(wrapper.vm as any).responseRewriteRules = []
    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.response_rewrite_rules).toEqual([])
  })
})
