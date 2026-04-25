import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'

const {
  updateAccountMock,
  checkMixedChannelRiskMock,
  getSettingsMock,
  getWebSearchEmulationConfigMock,
  listTlsFingerprintProfilesMock
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn(),
  getSettingsMock: vi.fn(),
  getWebSearchEmulationConfigMock: vi.fn(),
  listTlsFingerprintProfilesMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isSimpleMode: true
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    settings: {
      getSettings: getSettingsMock,
      getWebSearchEmulationConfig: getWebSearchEmulationConfigMock
    },
    tlsFingerprintProfiles: {
      list: listTlsFingerprintProfilesMock
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
      t: (key: string) => key
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

function mountModal(account = buildAccount()) {
  return mount(EditAccountModal, {
    props: {
      show: false,
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

  it('falls back to legacy related-model coverage when OpenAI or Anthropic accounts have no explicit whitelist', async () => {
    const openAIAccount = buildAccount()
    openAIAccount.credentials = {
      api_key: 'sk-test',
      base_url: 'https://api.openai.com'
    }

    const openAIWrapper = mountModal(openAIAccount)
    await openAIWrapper.setProps({ show: true })
    expect(openAIWrapper.get('[data-testid="model-whitelist-value"]').text()).toContain('gpt-4o')
    expect(openAIWrapper.get('[data-testid="model-whitelist-value"]').text()).toContain('gpt-4o-mini')

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
    expect(anthropicWrapper.get('[data-testid="model-whitelist-value"]').text()).toContain('claude-sonnet-4')
    expect(anthropicWrapper.get('[data-testid="model-whitelist-value"]').text()).toContain('claude-3-7-sonnet')
  })

  it('removes Kiro runtime version overrides from extra while keeping account credentials', async () => {
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
          'claude-sonnet-4': 'claude-sonnet-4'
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

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toEqual(
      expect.objectContaining({
        refresh_token: 'rt-test',
        region: 'us-east-1',
        machine_id: 'machine-1',
        model_mapping: {
          'claude-sonnet-4': 'claude-sonnet-4'
        }
      })
    )
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

  it('round-trips Kiro API key credentials without generic base_url', async () => {
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

    await wrapper.get('form#edit-account-form').trigger('submit.prevent')

    expect(updateAccountMock).toHaveBeenCalledTimes(1)
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toEqual(
      expect.objectContaining({
        api_key: 'kiro-old-key',
        region: 'eu-west-1',
        auth_region: 'us-west-2',
        api_region: 'us-east-2',
        profile_arn: 'arn:aws:bedrock:us-east-2:123456789012:inference-profile/demo',
        machine_id: 'machine-1'
      })
    )
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('base_url')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('model_whitelist')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).toHaveProperty('model_mapping')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials?.model_mapping).toEqual({
      'claude-sonnet-4': 'claude-sonnet-4'
    })
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('pool_mode')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('pool_mode_retry_count')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('custom_error_codes_enabled')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('custom_error_codes')
    expect(updateAccountMock.mock.calls[0]?.[1]?.credentials).not.toHaveProperty('custom_error_codes_backoff_ms')
    expect(updateAccountMock.mock.calls[0]?.[1]?.extra).toEqual({
      quota_limit: 100
    })
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
})
