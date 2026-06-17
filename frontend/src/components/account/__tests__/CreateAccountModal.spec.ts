import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref } from 'vue'

const {
  showErrorMock,
  showSuccessMock,
  showInfoMock,
  createMock,
  checkMixedChannelRiskMock,
  getWebSearchEmulationConfigMock,
  listTlsFingerprintProfilesMock,
  exchangeCodeMock,
  listTlsFingerprintRoutersMock,
  kiroValidateRefreshTokenMock
} = vi.hoisted(() => ({
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showInfoMock: vi.fn(),
  createMock: vi.fn(),
  checkMixedChannelRiskMock: vi.fn(),
  getWebSearchEmulationConfigMock: vi.fn(),
  listTlsFingerprintProfilesMock: vi.fn(),
  exchangeCodeMock: vi.fn(),
  listTlsFingerprintRoutersMock: vi.fn(),
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
    tlsFingerprintRouters: {
      list: listTlsFingerprintRoutersMock
    },
    accounts: {
      create: createMock,
      exchangeCode: exchangeCodeMock,
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
    errorCode: ref(''),
    oauthState: ref('oauth-state'),
    state: ref('oauth-state'),
    continuation: ref(null),
    resetState: vi.fn(),
    cancelDeviceAuthorization: vi.fn(),
    generateAuthUrl: vi.fn(),
    exchangeAuthCode: vi.fn(),
    buildCredentials: vi.fn(() => ({})),
    buildExtraInfo: vi.fn(() => undefined),
    buildAccountName: vi.fn((_tokenInfo?: unknown, name?: string) => name || 'auto-generated'),
    parseSessionKeys: vi.fn((value: string) => (value ? [value] : [])),
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
  const translations: Record<string, string> = {
    'admin.accounts.apiKey': '__API_KEY__',
    'admin.accounts.types.oauth': '__OAUTH__',
    'admin.accounts.vertexLabel': '__VERTEX__',
    'admin.accounts.vertexDesc': '__SERVICE_ACCOUNT__',
    'admin.accounts.oauth.gemini.codeAssistTitle': 'GCP Code Assist'
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => translations[key] ?? key
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
  props: {
    platform: {
      type: String,
      default: ''
    },
    errorCode: {
      type: String,
      default: ''
    },
    showProjectId: {
      type: Boolean,
      default: false
    },
    showProjectIdRecovery: {
      type: Boolean,
      default: false
    },
    showGeminiProjectBootstrapTip: {
      type: Boolean,
      default: false
    }
  },
  setup(props, { expose }) {
    expose({
      authCode: '',
      oauthState: '',
      projectId: '',
      requiresProjectIdRecovery: false,
      sessionKey: '',
      refreshToken: '',
      sessionToken: '',
      inputMethod: 'oauth',
      reset: vi.fn()
    })
    return () => h('div', {
      'data-testid': 'oauth-flow',
      'data-platform': props.platform,
      'data-error-code': props.errorCode,
      'data-show-project-id': String(props.showProjectId),
      'data-show-project-id-recovery': String(props.showProjectIdRecovery),
      'data-show-gemini-project-bootstrap-tip': String(props.showGeminiProjectBootstrapTip)
    })
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

function findAccountTypeButton(wrapper: VueWrapper<any>, index: number) {
  const buttons = wrapper.get('[data-tour="account-form-type"]').findAll('button')
  const button = buttons[index]
  expect(button, `account type button at index ${index} should exist`).toBeTruthy()
  return button
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
    exchangeCodeMock.mockReset()
    listTlsFingerprintRoutersMock.mockReset()
    kiroValidateRefreshTokenMock.mockReset()

    createMock.mockResolvedValue(undefined)
    checkMixedChannelRiskMock.mockResolvedValue({ has_risk: false })
    getWebSearchEmulationConfigMock.mockResolvedValue({ enabled: false, providers: [] })
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    exchangeCodeMock.mockResolvedValue({ access_token: 'at-cookie' })
    listTlsFingerprintRoutersMock.mockResolvedValue([])
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

    await findAccountTypeButton(wrapper, 1).trigger('click')
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
    await findAccountTypeButton(wrapper, 1).trigger('click')
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
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()
    expect((wrapper.vm as any).form.type).toBe('apikey')

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()

    expect((wrapper.vm as any).form.platform).toBe('openai')
    expect((wrapper.vm as any).form.type).toBe('oauth')
  })

  it('does not keep a preselected Gemini tier for Code Assist OAuth', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Gemini').trigger('click')
    await nextTick()
    await findButtonByText(wrapper, 'GCP Code Assist').trigger('click')
    await nextTick()

    expect((wrapper.vm as any).geminiOAuthType).toBe('code_assist')
    expect(wrapper.html()).not.toContain('admin.accounts.gemini.tier.gcp.enterprise')
    expect(wrapper.html()).not.toContain('admin.accounts.gemini.tier.gcp.standard')
  })

  it('enables project-id recovery for Gemini Google One OAuth', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Gemini').trigger('click')
    await nextTick()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    const oauthFlow = wrapper.get('[data-testid="oauth-flow"]')
    expect(oauthFlow.attributes('data-platform')).toBe('gemini')
    expect(oauthFlow.attributes('data-show-project-id')).toBe('true')
    expect(oauthFlow.attributes('data-show-project-id-recovery')).toBe('true')
    expect(oauthFlow.attributes('data-show-gemini-project-bootstrap-tip')).toBe('false')
  })

  it('enables project-id recovery for Gemini Code Assist OAuth', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Gemini').trigger('click')
    await nextTick()
    await findButtonByText(wrapper, 'GCP Code Assist').trigger('click')
    await nextTick()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    const oauthFlow = wrapper.get('[data-testid="oauth-flow"]')
    expect(oauthFlow.attributes('data-platform')).toBe('gemini')
    expect(oauthFlow.attributes('data-show-project-id')).toBe('true')
    expect(oauthFlow.attributes('data-show-project-id-recovery')).toBe('true')
    expect(oauthFlow.attributes('data-show-gemini-project-bootstrap-tip')).toBe('true')
  })

  it('keeps Gemini API key tier selection for AI Studio accounts', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'Gemini').trigger('click')
    await nextTick()
    await wrapper.get('[data-tour="account-form-type"] button:nth-child(2)').trigger('click')
    await nextTick()

    expect(wrapper.html()).toContain('admin.accounts.gemini.tier.aiStudio.free')

    await wrapper.get('[data-tour="account-form-name"]').setValue('gemini-api')
    await wrapper.get('input[placeholder="AIza..."]').setValue('AIza-test')
    await wrapper.get('select.input').setValue('aistudio_paid')
    await nextTick()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'gemini-api',
      platform: 'gemini',
      type: 'apikey',
      credentials: expect.objectContaining({
        api_key: 'AIza-test',
        tier_id: 'aistudio_paid'
      })
    }))
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
    await findAccountTypeButton(wrapper, 1).trigger('click')
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

  it('serializes OpenAI API key responses mode on create', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

    ;(wrapper.vm as any).openAIResponsesMode = 'force_responses'
    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'openai-api',
      platform: 'openai',
      type: 'apikey',
      extra: expect.objectContaining({
        openai_responses_mode: 'force_responses'
      })
    }))
  })

  it('creates an OpenAI API key account with TLS fingerprint settings', async () => {
    listTlsFingerprintProfilesMock.mockResolvedValue([{ id: 12, name: 'Chrome 124' }])
    listTlsFingerprintRoutersMock.mockResolvedValue([{ id: 9, name: 'UA Router' }])
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('[data-testid="openai-tls-fingerprint-toggle"]').trigger('click')
    await nextTick()
    await flushPromises()
    expect(wrapper.get('[data-testid="openai-tls-fingerprint-profile"]').text()).toContain('Chrome 124')
    await wrapper.get('[data-testid="openai-tls-fingerprint-profile"]').setValue('12')
    expect(wrapper.get('[data-testid="openai-tls-fingerprint-router"]').text()).toContain('UA Router')
    await wrapper.get('[data-testid="openai-tls-fingerprint-router"]').setValue('9')
    await nextTick()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'openai-api',
      platform: 'openai',
      type: 'apikey',
      extra: expect.objectContaining({
        enable_tls_fingerprint: true,
        tls_fingerprint_profile_id: 12,
        tls_fingerprint_router_id: 9
      })
    }))
  })

  it('shows the OpenAI image generation toggle only for OpenAI flows and defaults it to enabled', async () => {
    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.find('[data-testid="openai-image-generation-toggle"]').exists()).toBe(false)

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

    expect(wrapper.get('[data-testid="openai-image-generation-toggle"]').exists()).toBe(true)
    expect((wrapper.vm as any).openaiImageGenerationEnabled).toBe(true)

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      extra: expect.objectContaining({
        openai_image_generation_enabled: true
      })
    }))
  })

  it('persists an explicit disabled OpenAI image generation setting on create', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

    await wrapper.get('[data-testid="openai-image-generation-toggle"]').trigger('click')
    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api-disabled')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      extra: expect.objectContaining({
        openai_image_generation_enabled: false
      })
    }))
  })

  it('shows OpenAI response rewrite rules only for OpenAI and serializes them on create', async () => {
    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.find('[data-testid="openai-response-rewrite-section"]').exists()).toBe(false)

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

    expect(wrapper.get('[data-testid="openai-response-rewrite-section"]').exists()).toBe(true)

    ;(wrapper.vm as any).responseRewriteRules = [
      {
        status_code: 503,
        keywords: '欠费, insufficient_quota',
        match_mode: 'all',
        response_message: 'Service temporarily unavailable',
        description: 'quota mask'
      }
    ]

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      credentials: expect.objectContaining({
        response_rewrite_rules: [
          {
            status_code: 503,
            keywords: ['欠费', 'insufficient_quota'],
            match_mode: 'all',
            response_message: 'Service temporarily unavailable',
            description: 'quota mask'
          }
        ]
      })
    }))
  })

  it('serializes temp-unschedulable rules with empty keywords on create', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

    ;(wrapper.vm as any).tempUnschedEnabled = true
    ;(wrapper.vm as any).tempUnschedRules = [
      {
        error_code: 524,
        keywords: '',
        duration_minutes: 10,
        description: 'Cloudflare timeout'
      }
    ]

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(showErrorMock).not.toHaveBeenCalledWith('admin.accounts.tempUnschedulable.rulesInvalid')
    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      credentials: expect.objectContaining({
        temp_unschedulable_enabled: true,
        temp_unschedulable_rules: [
          {
            error_code: 524,
            keywords: [],
            duration_minutes: 10,
            description: 'Cloudflare timeout'
          }
        ]
      })
    }))
  })

  it('blocks mixed valid and invalid temp-unschedulable rules on create', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

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

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.tempUnschedulable.rulesInvalid')
    expect(createMock).not.toHaveBeenCalled()
  })

  it('blocks fractional custom error codes on create instead of saving them', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await findAccountTypeButton(wrapper, 1).trigger('click')
    await nextTick()

    ;(wrapper.vm as any).customErrorCodesEnabled = true
    ;(wrapper.vm as any).selectedErrorCodes = [502.5]

    await wrapper.get('[data-tour="account-form-name"]').setValue('openai-api')
    await wrapper.get('input[placeholder="sk-proj-..."]').setValue('sk-proj-test')
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.invalidErrorCode')
    expect(createMock).not.toHaveBeenCalled()
  })

  it('blocks fractional custom error codes on cookie OAuth create instead of saving them', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()

    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm
    setupState.customErrorCodesEnabled = true
    setupState.selectedErrorCodes = [502.5]

    await wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('cookie-auth', 'session-key')
    await flushPromises()

    expect(showErrorMock).toHaveBeenCalledWith('admin.accounts.invalidErrorCode')
    expect(exchangeCodeMock).not.toHaveBeenCalled()
    expect(createMock).not.toHaveBeenCalled()
  })

  it('serializes valid custom error codes on cookie OAuth create', async () => {
    const wrapper = mountModal()
    await flushPromises()

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()
    await wrapper.get('form#create-account-form').trigger('submit.prevent')
    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm
    setupState.customErrorCodesEnabled = true
    setupState.selectedErrorCodes = [502]

    await wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('cookie-auth', 'session-key')
    await flushPromises()

    expect(createMock).toHaveBeenCalledWith(expect.objectContaining({
      credentials: expect.objectContaining({
        custom_error_codes_enabled: true,
        custom_error_codes: [502]
      })
    }))
  })

  it('offers all OpenAI service-unavailable temp-unschedulable presets separately on create', async () => {
    const wrapper = mountModal()
    await flushPromises()

    const presetRules = (wrapper.vm as any).tempUnschedPresets.map((preset: any) => preset.rule)

    expect(presetRules).toEqual(expect.arrayContaining([
      expect.objectContaining({
        error_code: 502,
        keywords: 'Upstream service temporarily unavailable',
        duration_minutes: 10
      }),
      expect.objectContaining({
        error_code: 502,
        keywords: 'Upstream request failed',
        duration_minutes: 10
      }),
      expect.objectContaining({
        error_code: 503,
        keywords: 'Service temporarily unavailable',
        duration_minutes: 10
      }),
      expect.objectContaining({
        error_code: 503,
        keywords: 'overloaded',
        duration_minutes: 10
      }),
      expect.objectContaining({
        error_code: 500,
        keywords: 'upstream connection failed, Upstream transport error',
        duration_minutes: 10
      }),
      expect.objectContaining({
        error_code: 502,
        keywords: 'Upstream access forbidden',
        duration_minutes: 10
      })
    ]))
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

  it('renders localized account type entry labels instead of hardcoded english copy', async () => {
    const wrapper = mountModal()
    await flushPromises()

    expect(wrapper.text()).toContain('__VERTEX__')
    expect(wrapper.text()).toContain('__SERVICE_ACCOUNT__')

    await findButtonByText(wrapper, 'OpenAI').trigger('click')
    await nextTick()

    expect(wrapper.text()).toContain('__OAUTH__')
    expect(wrapper.text()).toContain('__API_KEY__')
  })
})
