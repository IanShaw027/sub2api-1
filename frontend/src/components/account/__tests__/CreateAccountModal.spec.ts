import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref } from 'vue'

const {
  showErrorMock,
  showSuccessMock,
  showInfoMock,
  createMock,
  checkMixedChannelRiskMock,
  getWebSearchEmulationConfigMock,
  listTlsFingerprintProfilesMock,
  kiroValidateRefreshTokenMock
} = vi.hoisted(() => ({
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showInfoMock: vi.fn(),
  createMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn(),
  getWebSearchEmulationConfigMock: vi.fn(),
  listTlsFingerprintProfilesMock: vi.fn(),
  kiroValidateRefreshTokenMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
    showInfo: showInfoMock
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
      getWebSearchEmulationConfig: getWebSearchEmulationConfigMock
    },
    tlsFingerprintProfiles: {
      list: listTlsFingerprintProfilesMock
    },
    accounts: {
      create: createMock,
      checkMixedChannelRisk: checkMixedChannelRiskMock
    }
  }
}))

vi.mock('@/composables/useQuotaNotifyState', () => ({
  useQuotaNotifyState: () => ({
    globalEnabled: ref(false),
    state: ref({
      daily: {
        enabled: false,
        threshold: null,
        threshold_type: 'percentage'
      },
      weekly: {
        enabled: false,
        threshold: null,
        threshold_type: 'percentage'
      },
      total: {
        enabled: false,
        threshold: null,
        threshold_type: 'percentage'
      }
    }),
    loadGlobalState: vi.fn(),
    writeToExtra: vi.fn()
  })
}))

vi.mock('@/composables/useModelWhitelist', () => ({
  claudeModels: ['claude-sonnet-4'],
  commonErrorCodes: [],
  getPresetMappingsByPlatform: vi.fn(() => []),
  getModelsByPlatform: vi.fn(() => []),
  buildModelMappingObject: vi.fn((mode: string, allowed: string[], mappings: Array<{ from: string; to: string }>) => {
    if (mode === 'whitelist') {
      if (!Array.isArray(allowed) || allowed.length === 0) {
        return undefined
      }
      return Object.fromEntries(allowed.map((model) => [model, model]))
    }
    if (!Array.isArray(mappings) || mappings.length === 0) {
      return undefined
    }
    return Object.fromEntries(
      mappings
        .filter((mapping) => mapping.from && mapping.to)
        .map((mapping) => [mapping.from, mapping.to])
    )
  }),
  fetchAntigravityDefaultMappings: vi.fn().mockResolvedValue([]),
  isValidWildcardPattern: vi.fn(() => true)
}))

vi.mock('@/components/account/credentialsBuilder', () => ({
  applyInterceptWarmup: vi.fn()
}))

function buildOAuthComposable() {
  return {
    authUrl: ref(''),
    sessionId: ref('session-id'),
    callbackBaseUrl: ref('http://localhost:3128'),
    loading: ref(false),
    error: ref(''),
    oauthState: ref('oauth-state'),
    state: ref('oauth-state'),
    resetState: vi.fn(),
    generateAuthUrl: vi.fn(),
    exchangeAuthCode: vi.fn(),
    buildCredentials: vi.fn(() => ({})),
    buildExtraInfo: vi.fn(() => undefined),
    buildAccountName: vi.fn((_tokenInfo?: unknown, name?: string) => name || 'auto-generated'),
    parseSessionKeys: vi.fn(() => []),
    validateRefreshToken: vi.fn(),
    exchangeCallback: vi.fn()
  }
}

vi.mock('@/composables/useAccountOAuth', () => ({
  useAccountOAuth: () => buildOAuthComposable()
}))

vi.mock('@/composables/useOpenAIOAuth', () => ({
  useOpenAIOAuth: () => buildOAuthComposable()
}))

vi.mock('@/composables/useGeminiOAuth', () => ({
  useGeminiOAuth: () => buildOAuthComposable()
}))

vi.mock('@/composables/useAntigravityOAuth', () => ({
  useAntigravityOAuth: () => buildOAuthComposable()
}))

vi.mock('@/composables/useKiroOAuth', () => ({
  useKiroOAuth: () => ({
    ...buildOAuthComposable(),
    validateRefreshToken: kiroValidateRefreshTokenMock,
    buildExtraInfo: (tokenInfo?: any, overrides?: Record<string, unknown>) => {
      const extra: Record<string, unknown> = { ...(overrides || {}) }
      if (tokenInfo?.email) extra.email = tokenInfo.email
      if (tokenInfo?.subscription_type || tokenInfo?.plan_name) {
        extra.subscription_type = tokenInfo.subscription_type || tokenInfo.plan_name
      }
      return extra
    },
    buildAccountName: (tokenInfo?: any, fallbackName?: string) => {
      return fallbackName?.trim() || tokenInfo?.name || tokenInfo?.email || 'Kiro OAuth Account'
    }
  })
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

import CreateAccountModal from '../CreateAccountModal.vue'

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

const OAuthAuthorizationFlowStub = defineComponent({
  name: 'OAuthAuthorizationFlow',
  setup(_, { expose }) {
    expose({
      authCode: '',
      oauthState: '',
      projectId: '',
      sessionKey: '',
      refreshToken: '',
      sessionToken: '',
      inputMethod: 'oauth',
      reset: vi.fn()
    })
    return () => h('div', { 'data-testid': 'oauth-flow' })
  }
})

const KiroAuthorizationFlowStub = defineComponent({
  name: 'KiroAuthorizationFlow',
  setup(_, { expose }) {
    expose({
      reset: vi.fn()
    })
    return () => h('div', { 'data-testid': 'kiro-flow' })
  }
})

function mountModal() {
  return mount(CreateAccountModal, {
    props: {
      show: true,
      proxies: [],
      groups: []
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        Select: true,
        Icon: true,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: true,
        QuotaLimitCard: true,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        KiroAuthorizationFlow: KiroAuthorizationFlowStub
      }
    }
  })
}

function findButtonByText(wrapper: VueWrapper<any>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  expect(button, `button containing "${text}" should exist`).toBeTruthy()
  return button!
}

describe('CreateAccountModal', () => {
  beforeEach(() => {
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    showInfoMock.mockReset()
    createMock.mockReset()
    checkMixedChannelRiskMock.mockReset()
    getWebSearchEmulationConfigMock.mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    kiroValidateRefreshTokenMock.mockReset()

    createMock.mockResolvedValue(undefined)
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    kiroValidateRefreshTokenMock.mockResolvedValue({
      refresh_token: 'rt-test',
      access_token: 'at-test',
      region: 'us-east-1',
      email: 'manual@example.com',
      plan_name: 'Kiro Pro'
    })
  })

  it('allows an empty name for OAuth flows so the auto-naming step can continue', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const nameInput = wrapper.get('[data-tour="account-form-name"]')
    expect(nameInput.attributes('required')).toBeUndefined()

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(showErrorMock).not.toHaveBeenCalledWith('admin.accounts.pleaseEnterAccountName')
    expect(wrapper.find('[data-testid="oauth-flow"]').exists()).toBe(true)
    expect(createMock).not.toHaveBeenCalled()
  })

  it('still blocks API key creation when the name is blank', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const accountTypeButtons = wrapper.get('[data-tour="account-form-type"]').findAll('button')
    expect(accountTypeButtons).toHaveLength(3)
    await accountTypeButtons[1].trigger('click')
    await nextTick()

    const nameInput = wrapper.get('[data-tour="account-form-name"]')
    expect(nameInput.attributes('required')).toBeDefined()

    await wrapper.get('input[placeholder="sk-ant-..."]').setValue('sk-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.pleaseEnterAccountName')
    expect(createMock).not.toHaveBeenCalled()
  })

  it('allows an empty name for Kiro OAuth creation', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Kiro').trigger('click')
    await nextTick()

    const nameInput = wrapper.get('[data-tour="account-form-name"]')
    expect(nameInput.attributes('required')).toBeUndefined()

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(wrapper.find('[data-testid="kiro-flow"]').exists()).toBe(true)
    expect(createMock).not.toHaveBeenCalled()
  })

  it('creates a Kiro API key account without entering the OAuth step', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Kiro').trigger('click')
    await nextTick()
    await findButtonByText(wrapper, 'API Key').trigger('click')
    await nextTick()

    await wrapper.get('[data-tour="account-form-name"]').setValue('kiro-api')
    await wrapper.get('input[placeholder="admin.accounts.kiro.apiKeyPlaceholder"]').setValue('kiro-token')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'kiro-api',
      platform: 'kiro',
      type: 'apikey',
      credentials: expect.objectContaining({
        api_key: 'kiro-token',
        region: 'us-east-1'
      })
    }))
    expect(createMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('model_mapping')
    expect(createMock.mock.calls[0]?.[0]?.extra).toBeUndefined()
    expect(wrapper.find('[data-testid="kiro-flow"]').exists()).toBe(false)
  })

  it('syncs form.type when switching away from Kiro API key to an OAuth platform', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Kiro').trigger('click')
    await nextTick()
    await findButtonByText(wrapper, 'API Key').trigger('click')
    await nextTick()
    expect((wrapper.vm as any).form.type).toBe('apikey')

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()

    expect((wrapper.vm as any).form.platform).toBe('openai')
    expect((wrapper.vm as any).form.type).toBe('oauth')
  })

  it('clears stale OpenAI compact settings after switching to another platform', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()

    ;(wrapper.vm as any).openAICompactMode = 'force_on'
    ;(wrapper.vm as any).openAICompactModelMappings = [
      { from: 'gpt-5.4', to: 'gpt-5.4-openai-compact' }
    ]

    await findButtonByText(wrapper, 'Anthropic').trigger('click')
    await nextTick()
    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findButtonByText(wrapper, 'API Key').trigger('click')
    await nextTick()

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'openai-api',
      platform: 'openai',
      type: 'apikey',
      credentials: expect.not.objectContaining({
        compact_model_mapping: expect.anything()
      })
    }))
    expect(createMock.mock.calls[0]?.[0]?.extra).not.toHaveProperty('openai_compact_mode')
  })

  it('creates an OpenAI API key account with TLS fingerprint settings', async () => {
    listTlsFingerprintProfilesMock.mockResolvedValue([{ id: 12, name: 'Chrome 124' }])
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findButtonByText(wrapper, 'API Key').trigger('click')
    await nextTick()

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('[data-testid="openai-tls-fingerprint-toggle"]').trigger('click')
    await nextTick()
    await flushPromises()
    expect(wrapper.get('[data-testid="openai-tls-fingerprint-profile"]').text()).toContain('Chrome 124')
    await wrapper.get('[data-testid="openai-tls-fingerprint-profile"]').setValue('12')
    await nextTick()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'openai-api',
      platform: 'openai',
      type: 'apikey',
      extra: expect.objectContaining({
        enable_tls_fingerprint: true,
        tls_fingerprint_profile_id: 12
      })
    }))
  })

  it('creates a Kiro OAuth account from manual refresh token input', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Kiro').trigger('click')
    await nextTick()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    await wrapper.getComponent(KiroAuthorizationFlowStub).vm.$emit('submit-refresh-token', {
      credentials: {
        refresh_token: 'rt-1',
        auth_method: 'social',
        region: 'us-east-1'
      },
      extra: {}
    })
    await flushPromises()

    expect(kiroValidateRefreshTokenMock).toHaveBeenCalledWith(
      expect.objectContaining({
        refresh_token: 'rt-1',
        auth_method: 'social',
        region: 'us-east-1'
      }),
      {},
      null
    )
    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'manual@example.com',
      platform: 'kiro',
      type: 'oauth',
      credentials: expect.objectContaining({
        refresh_token: 'rt-test',
        access_token: 'at-test'
      }),
      extra: expect.objectContaining({
        email: 'manual@example.com',
        subscription_type: 'Kiro Pro'
      })
    }))
    expect(createMock.mock.calls[0]?.[0]?.credentials).not.toHaveProperty('model_mapping')
  })
})
