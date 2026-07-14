import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'

const {
  updateAccountMock,
  applyOAuthCredentialsMock,
  reauthorizeKiroOAuthMock,
  createAccountMock,
  clearErrorMock,
  exchangeCallbackMock,
  validateRefreshTokenMock,
  grokValidateRefreshTokenMock,
  grokValidateSSOTokenMock,
  grokAuthorizePasswordMock,
  grokExchangeAuthCodeMock,
  grokGenerateAuthUrlMock,
  geminiGenerateAuthUrlMock,
  buildCredentialsMock,
  buildExtraInfoMock,
  buildAccountNameMock
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  applyOAuthCredentialsMock: vi.fn(),
  reauthorizeKiroOAuthMock: vi.fn(),
  createAccountMock: vi.fn(),
  clearErrorMock: vi.fn(),
  exchangeCallbackMock: vi.fn(),
  validateRefreshTokenMock: vi.fn(),
  grokValidateRefreshTokenMock: vi.fn(),
  grokValidateSSOTokenMock: vi.fn(),
  grokAuthorizePasswordMock: vi.fn(),
  grokExchangeAuthCodeMock: vi.fn(),
  grokGenerateAuthUrlMock: vi.fn(),
  geminiGenerateAuthUrlMock: vi.fn(),
  buildCredentialsMock: vi.fn(),
  buildExtraInfoMock: vi.fn(),
  buildAccountNameMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showWarning: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: updateAccountMock,
      applyOAuthCredentials: applyOAuthCredentialsMock,
      reauthorizeKiroOAuth: reauthorizeKiroOAuthMock,
      create: createAccountMock,
      clearError: clearErrorMock
    }
  }
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
    validateRefreshToken: vi.fn(),
    buildCredentials: vi.fn(() => ({})),
    buildExtraInfo: vi.fn(() => ({})),
    buildAccountName: vi.fn((_tokenInfo?: unknown, name?: string) => name || 'Account')
  }
}

vi.mock('@/composables/useAccountOAuth', () => ({
  useAccountOAuth: () => buildOAuthComposable()
}))

vi.mock('@/composables/useOpenAIOAuth', () => ({
  useOpenAIOAuth: () => buildOAuthComposable()
}))

vi.mock('@/composables/useGeminiOAuth', () => ({
  useGeminiOAuth: () => ({
    ...buildOAuthComposable(),
    generateAuthUrl: geminiGenerateAuthUrlMock
  })
}))

vi.mock('@/composables/useAntigravityOAuth', () => ({
  useAntigravityOAuth: () => buildOAuthComposable()
}))

vi.mock('@/composables/useGrokOAuth', () => ({
  useGrokOAuth: () => ({
    authUrl: ref(''),
    sessionId: ref('grok-session-id'),
    loading: ref(false),
    error: ref(''),
    state: ref('grok-oauth-state'),
    resetState: vi.fn(),
    generateAuthUrl: grokGenerateAuthUrlMock,
    exchangeAuthCode: grokExchangeAuthCodeMock,
    validateRefreshToken: grokValidateRefreshTokenMock,
    validateSSOToken: grokValidateSSOTokenMock,
    authorizePassword: grokAuthorizePasswordMock,
    buildCredentials: (tokenInfo?: any) => {
      const credentials: Record<string, unknown> = {
        access_token: tokenInfo?.access_token,
        refresh_token: tokenInfo?.refresh_token,
        sso_token: tokenInfo?.sso_token,
        email: tokenInfo?.email
      }
      return Object.fromEntries(Object.entries(credentials).filter(([, value]) => value !== undefined && value !== ''))
    },
    buildExtraInfo: (tokenInfo?: any) => {
      const extra: Record<string, unknown> = {}
      if (tokenInfo?.email) extra.email = tokenInfo.email
      return extra
    }
  })
}))

vi.mock('@/composables/useKiroOAuth', () => ({
  stripKiroRuntimeExtra: (extra?: Record<string, unknown> | null) => {
    const cleaned = { ...(extra || {}) }
    delete cleaned.kiro_version
    delete cleaned.system_version
    delete cleaned.node_version
    return cleaned
  },
  useKiroOAuth: () => ({
    authUrl: ref(''),
    sessionId: ref('session-id'),
    callbackBaseUrl: ref('http://localhost:3128'),
    loading: ref(false),
    error: ref(''),
    continuation: ref(null),
    externalIDPAuthorization: ref(null),
    resetState: vi.fn(),
    cancelDeviceAuthorization: vi.fn(),
    generateAuthUrl: vi.fn(),
    exchangeCallback: exchangeCallbackMock,
    validateRefreshToken: validateRefreshTokenMock,
    buildCredentials: buildCredentialsMock,
    buildExtraInfo: buildExtraInfoMock,
    buildAccountName: buildAccountNameMock
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

import ReAuthAccountModal from '../ReAuthAccountModal.vue'

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

const KiroAuthorizationFlowStub = defineComponent({
  name: 'KiroAuthorizationFlow',
  props: {
    mode: {
      type: String,
      default: ''
    },
    authUrl: {
      type: String,
      default: ''
    },
    callbackBaseUrl: {
      type: String,
      default: ''
    },
    accountId: {
      type: Number,
      default: null
    },
    proxyId: {
      type: Number,
      default: null
    },
    loading: {
      type: Boolean,
      default: false
    },
    error: {
      type: String,
      default: ''
    },
    initialCredentials: {
      type: Object,
      default: () => ({})
    },
    initialExtra: {
      type: Object,
      default: () => ({})
    },
    continuation: {
      type: Object,
      default: null
    },
    externalIDPAuthorization: {
      type: Object,
      default: null
    }
  },
  emits: ['generate-url', 'submit', 'submit-refresh-token', 'cancel-continuation'],
  setup(_, { emit }) {
    return () => [
      h('button', {
        'data-testid': 'kiro-flow-submit',
        type: 'button',
        onClick: () => emit('submit', {
          callbackUrl: 'http://localhost:3128/callback?code=abc',
          credentials: { machine_id: 'machine-1' },
          extra: { kiro_version: 'old-runtime', keep_flag: true }
        })
      }, 'submit kiro'),
      h('button', {
        'data-testid': 'kiro-flow-submit-refresh-token',
        type: 'button',
        onClick: () => emit('submit-refresh-token', {
          credentials: {
            refresh_token: 'refresh-manual',
            auth_method: 'social',
            region: 'eu-west-1',
            machine_id: 'machine-2'
          },
          extra: {
            keep_flag: false,
            custom_note: 'manual',
            system_version: 'runtime-only'
          }
        })
      }, 'submit kiro refresh token')
    ]
  }
})

const oauthFlowExposeState = {
  authCode: '',
  oauthState: '',
  projectId: '',
  requiresProjectIdRecovery: false,
  sessionKey: '',
  inputMethod: 'manual',
  reset: vi.fn()
}

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
  emits: ['generate-url', 'cookie-auth', 'validate-refresh-token', 'validate-sso-token', 'authorize-password'],
  setup(props, { expose, emit }) {
    expose(oauthFlowExposeState)
    return () => h('div', {}, [
      h('div', {
        'data-testid': 'oauth-flow',
        'data-platform': props.platform,
        'data-error-code': props.errorCode,
        'data-show-project-id': String(props.showProjectId),
        'data-show-project-id-recovery': String(props.showProjectIdRecovery),
        'data-show-gemini-project-bootstrap-tip': String(props.showGeminiProjectBootstrapTip)
      }),
      h('button', {
        'data-testid': 'oauth-flow-generate-url',
        type: 'button',
        onClick: () => emit('generate-url')
      }, 'generate oauth url')
    ])
  }
})

function buildKiroAccount(type: 'oauth' | 'apikey') {
  return {
    id: 42,
    name: type === 'oauth' ? 'Kiro OAuth' : 'Kiro API Key',
    notes: '',
    platform: 'kiro',
    type,
    credentials: type === 'oauth'
      ? { refresh_token: 'rt-old', region: 'us-east-1' }
      : { api_key: 'kiro-key', region: 'us-east-1' },
    extra: {
      keep_flag: true,
      kiro_version: '0.9.0'
    },
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'error',
    error_message: 'expired',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildGeminiOAuthAccount() {
  return {
    id: 43,
    name: 'Gemini OAuth',
    notes: '',
    platform: 'gemini',
    type: 'oauth',
    credentials: {
      oauth_type: 'google_one',
      tier_id: 'google_ai_pro'
    },
    extra: {},
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'error',
    error_message: 'expired',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function buildGrokOAuthAccount() {
  return {
    id: 54,
    name: 'Grok OAuth',
    notes: '',
    platform: 'grok',
    type: 'oauth',
    credentials: {
      refresh_token: 'grok-rt-old',
      access_token: 'grok-at-old'
    },
    extra: {
      email: 'grok-old@example.com'
    },
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    rate_multiplier: 1,
    status: 'error',
    error_message: 'expired',
    group_ids: [],
    expires_at: null,
    auto_pause_on_expired: false
  } as any
}

function mountModal(account = buildKiroAccount('apikey')) {
  return mount(ReAuthAccountModal, {
    props: {
      show: true,
      account
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Icon: true,
        OAuthAuthorizationFlow: OAuthAuthorizationFlowStub,
        KiroAuthorizationFlow: KiroAuthorizationFlowStub
      }
    }
  })
}

describe('admin ReAuthAccountModal', () => {
  beforeEach(() => {
    oauthFlowExposeState.authCode = ''
    oauthFlowExposeState.oauthState = ''
    oauthFlowExposeState.projectId = ''
    oauthFlowExposeState.requiresProjectIdRecovery = false
    oauthFlowExposeState.sessionKey = ''
    oauthFlowExposeState.inputMethod = 'manual'
    updateAccountMock.mockReset()
    applyOAuthCredentialsMock.mockReset()
    reauthorizeKiroOAuthMock.mockReset()
    createAccountMock.mockReset()
    clearErrorMock.mockReset()
    exchangeCallbackMock.mockReset()
    validateRefreshTokenMock.mockReset()
    grokValidateRefreshTokenMock.mockReset()
    grokValidateSSOTokenMock.mockReset()
    grokAuthorizePasswordMock.mockReset()
    grokExchangeAuthCodeMock.mockReset()
    grokGenerateAuthUrlMock.mockReset()
    geminiGenerateAuthUrlMock.mockReset()
    buildCredentialsMock.mockReset()
    buildExtraInfoMock.mockReset()
    buildAccountNameMock.mockReset()
    createAccountMock.mockResolvedValue({})
    applyOAuthCredentialsMock.mockResolvedValue({})

    exchangeCallbackMock.mockResolvedValue({
      access_token: 'access-new',
      refresh_token: 'refresh-new',
      expires_at: '2026-04-25T00:00:00Z'
    })
    validateRefreshTokenMock.mockResolvedValue({
      access_token: 'access-validated',
      refresh_token: 'refresh-validated',
      expires_at: '2026-05-01T00:00:00Z',
      email: 'manual@example.com',
      plan_name: 'Kiro Pro'
    })
    buildCredentialsMock.mockReturnValue({
      access_token: 'access-new',
      refresh_token: 'refresh-new',
      region: 'us-east-1'
    })
    buildExtraInfoMock.mockImplementation((tokenInfo?: any, overrides?: Record<string, unknown>) => {
      const extra: Record<string, unknown> = { ...(overrides || {}) }
      delete extra.kiro_version
      delete extra.system_version
      delete extra.node_version
      if (tokenInfo?.email) extra.email = tokenInfo.email
      if (tokenInfo?.subscription_type || tokenInfo?.plan_name) {
        extra.subscription_type = tokenInfo.subscription_type || tokenInfo.plan_name
      }
      return extra
    })
    buildAccountNameMock.mockReturnValue('Kiro OAuth')
    updateAccountMock.mockResolvedValue({})
    reauthorizeKiroOAuthMock.mockResolvedValue(buildKiroAccount('oauth'))
    clearErrorMock.mockResolvedValue(buildKiroAccount('oauth'))
    grokExchangeAuthCodeMock.mockResolvedValue({
      access_token: 'grok-at-new',
      refresh_token: 'grok-rt-new',
      email: 'grok@example.com'
    })
    grokValidateRefreshTokenMock.mockResolvedValue({
      access_token: 'grok-at-new',
      refresh_token: 'grok-rt-new',
      email: 'grok@example.com'
    })
    grokValidateSSOTokenMock.mockResolvedValue({
      access_token: 'grok-at-new',
      refresh_token: 'grok-rt-new',
      sso_token: 'grok-sso-new',
      email: 'grok@example.com'
    })
    grokAuthorizePasswordMock.mockResolvedValue({
      access_token: 'grok-at-new',
      refresh_token: 'grok-rt-new',
      sso_token: 'grok-password-sso',
      email: 'grok@example.com'
    })
  })

  it('renders the current account platform icon in the account summary', () => {
    const wrapper = mountModal(buildKiroAccount('apikey'))

    expect(wrapper.get('[data-testid="reauth-platform-icon"]').attributes('viewBox')).toBe('0 0 24 24')
  })

  it('does not mount the Kiro OAuth reauthorization flow for Kiro API key accounts', () => {
    const wrapper = mountModal(buildKiroAccount('apikey'))

    expect(wrapper.find('[data-testid="kiro-flow-submit"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="oauth-flow"]').exists()).toBe(false)
    expect(updateAccountMock).not.toHaveBeenCalled()
    expect(reauthorizeKiroOAuthMock).not.toHaveBeenCalled()
  })

  it('reauthorizes Gemini Google One accounts through the OAuth flow', () => {
    const wrapper = mountModal(buildGeminiOAuthAccount())

    expect(wrapper.find('[data-testid="oauth-flow"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-platform')).toBe('gemini')
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-project-id')).toBe('true')
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-project-id-recovery')).toBe('true')
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-gemini-project-bootstrap-tip')).toBe('false')
    expect(wrapper.findAll('button').some((button) => button.text().includes('admin.accounts.oauth.completeAuth'))).toBe(true)
  })

  it('generates Gemini Code Assist reauth URL without sending a preselected tier', async () => {
    const wrapper = mountModal({
      ...buildGeminiOAuthAccount(),
      credentials: {
        oauth_type: 'code_assist',
        tier_id: 'gcp_standard'
      }
    })

    await wrapper.get('[data-testid="oauth-flow-generate-url"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-project-id')).toBe('true')
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-project-id-recovery')).toBe('true')
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-gemini-project-bootstrap-tip')).toBe('true')
    expect(geminiGenerateAuthUrlMock).toHaveBeenCalledWith(
      null,
      undefined,
      'code_assist'
    )
  })

  it('exposes project recovery inputs for Gemini Google One reauth', () => {
    const wrapper = mountModal({
      ...buildGeminiOAuthAccount(),
      credentials: {
        oauth_type: 'google_one',
        tier_id: 'google_ai_pro'
      }
    })

    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-project-id')).toBe('true')
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-project-id-recovery')).toBe('true')
    expect(wrapper.get('[data-testid="oauth-flow"]').attributes('data-show-gemini-project-bootstrap-tip')).toBe('false')
  })

  it('infers Gemini Google One reauth mode from tier metadata when oauth_type is missing', async () => {
    const wrapper = mountModal({
      ...buildGeminiOAuthAccount(),
      credentials: {
        tier_id: 'g1-pro-tier'
      },
      extra: {
        gemini_paid_tier_id: 'g1-pro-tier'
      }
    })

    await wrapper.get('[data-testid="oauth-flow-generate-url"]').trigger('click')
    await flushPromises()

    expect(geminiGenerateAuthUrlMock).toHaveBeenCalledWith(
      null,
      undefined,
      'google_one'
    )
  })

  it('reauthorizes Kiro OAuth accounts from callback submission without preserving runtime-only extra fields', async () => {
    const wrapper = mountModal(buildKiroAccount('oauth'))

    await wrapper.get('[data-testid="kiro-flow-submit"]').trigger('click')
    await flushPromises()

    expect(exchangeCallbackMock).toHaveBeenCalledWith(
      'http://localhost:3128/callback?code=abc',
      null
    )
    expect(reauthorizeKiroOAuthMock).toHaveBeenCalledWith(42, expect.objectContaining({
      name: 'Kiro OAuth',
      credentials: expect.objectContaining({
        refresh_token: 'refresh-new'
      }),
      extra: {
        keep_flag: true
      }
    }))
    expect(clearErrorMock).not.toHaveBeenCalled()
  })

  it('reauthorizes Kiro OAuth accounts from manual refresh token submission with merged sanitized data', async () => {
    const wrapper = mountModal(buildKiroAccount('oauth'))

    await wrapper.get('[data-testid="kiro-flow-submit-refresh-token"]').trigger('click')
    await flushPromises()

    expect(validateRefreshTokenMock).toHaveBeenCalledWith(
      {
        refresh_token: 'refresh-manual',
        auth_method: 'social',
        region: 'eu-west-1',
        machine_id: 'machine-2'
      },
      {
        keep_flag: false,
        custom_note: 'manual',
        system_version: 'runtime-only'
      },
      null,
      false
    )
    expect(reauthorizeKiroOAuthMock).toHaveBeenCalledWith(42, {
      name: 'Kiro OAuth',
      credentials: {
        refresh_token: 'refresh-validated',
        region: 'eu-west-1',
        auth_method: 'social',
        machine_id: 'machine-2',
        access_token: 'access-validated',
        expires_at: '2026-05-01T00:00:00Z',
        email: 'manual@example.com',
        plan_name: 'Kiro Pro'
      },
      extra: {
        keep_flag: false,
        custom_note: 'manual',
        email: 'manual@example.com',
        subscription_type: 'Kiro Pro'
      }
    })
    expect(clearErrorMock).not.toHaveBeenCalled()
  })

  it('reauthorizes Kiro OAuth accounts from multiline refresh tokens by updating one account and creating the rest', async () => {
    validateRefreshTokenMock.mockImplementation(async (credentials: Record<string, unknown>) => ({
      access_token: `access-${credentials.refresh_token}`,
      refresh_token: `validated-${credentials.refresh_token}`,
      expires_at: '2026-05-01T00:00:00Z',
      email: `${credentials.refresh_token}@example.com`,
      plan_name: 'Kiro Pro'
    }))
    buildAccountNameMock.mockImplementation((tokenInfo?: any) => `Kiro ${tokenInfo?.email || 'OAuth'}`)
    const wrapper = mountModal(buildKiroAccount('oauth'))

    wrapper.getComponent(KiroAuthorizationFlowStub).vm.$emit('submit-refresh-token', {
      credentials: {
        refresh_token: 'rt-one\nrt-two',
        auth_method: 'social',
        region: 'eu-west-1',
        machine_id: 'machine-2'
      },
      extra: {
        custom_note: 'manual'
      }
    })
    await flushPromises()

    expect(validateRefreshTokenMock).toHaveBeenCalledTimes(2)
    expect(validateRefreshTokenMock).toHaveBeenNthCalledWith(
      1,
      {
        refresh_token: 'rt-one',
        auth_method: 'social',
        region: 'eu-west-1',
        machine_id: 'machine-2'
      },
      {
        custom_note: 'manual'
      },
      null,
      false
    )
    expect(validateRefreshTokenMock).toHaveBeenNthCalledWith(
      2,
      {
        refresh_token: 'rt-two',
        auth_method: 'social',
        region: 'eu-west-1',
        machine_id: 'machine-2'
      },
      {
        custom_note: 'manual'
      },
      null,
      false
    )
    expect(reauthorizeKiroOAuthMock).toHaveBeenCalledWith(42, expect.objectContaining({
      name: 'Kiro rt-one@example.com #1',
      credentials: expect.objectContaining({
        refresh_token: 'validated-rt-one',
        access_token: 'access-rt-one'
      })
    }))
    expect(createAccountMock).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Kiro rt-two@example.com #2',
      platform: 'kiro',
      type: 'oauth',
      credentials: expect.objectContaining({
        refresh_token: 'validated-rt-two',
        access_token: 'access-rt-two'
      })
    }))
    expect(clearErrorMock).not.toHaveBeenCalled()
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('keeps Kiro batch refresh-token reauth guarded while per-token validation toggles loading', async () => {
    let resolveFirstValidation: ((value: Record<string, unknown>) => void) | undefined
    validateRefreshTokenMock
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveFirstValidation = resolve
      }))
      .mockImplementation(async (credentials: Record<string, unknown>) => ({
        access_token: `access-${credentials.refresh_token}`,
        refresh_token: `validated-${credentials.refresh_token}`,
        expires_at: '2026-05-01T00:00:00Z',
        email: `${credentials.refresh_token}@example.com`,
        plan_name: 'Kiro Pro'
      }))
    buildAccountNameMock.mockImplementation((tokenInfo?: any) => `Kiro ${tokenInfo?.email || 'OAuth'}`)
    const wrapper = mountModal(buildKiroAccount('oauth'))
    const payload = {
      credentials: {
        refresh_token: 'rt-one\nrt-two',
        auth_method: 'social',
        region: 'eu-west-1'
      },
      extra: {}
    }

    wrapper.getComponent(KiroAuthorizationFlowStub).vm.$emit('submit-refresh-token', payload)
    await flushPromises()
    wrapper.getComponent(KiroAuthorizationFlowStub).vm.$emit('submit-refresh-token', payload)
    await flushPromises()

    expect(validateRefreshTokenMock).toHaveBeenCalledTimes(1)

    resolveFirstValidation?.({
      access_token: 'access-rt-one',
      refresh_token: 'validated-rt-one',
      expires_at: '2026-05-01T00:00:00Z',
      email: 'rt-one@example.com',
      plan_name: 'Kiro Pro'
    })
    await flushPromises()

    expect(validateRefreshTokenMock).toHaveBeenCalledTimes(2)
    expect(reauthorizeKiroOAuthMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock).toHaveBeenCalledTimes(1)
  })

  it('requests a list refresh when Kiro multiline reauth creates accounts before a later validation failure', async () => {
    validateRefreshTokenMock
      .mockImplementationOnce(async (credentials: Record<string, unknown>) => ({
        access_token: `access-${credentials.refresh_token}`,
        refresh_token: `validated-${credentials.refresh_token}`,
        email: `${credentials.refresh_token}@example.com`
      }))
      .mockImplementationOnce(async (credentials: Record<string, unknown>) => ({
        access_token: `access-${credentials.refresh_token}`,
        refresh_token: `validated-${credentials.refresh_token}`,
        email: `${credentials.refresh_token}@example.com`
      }))
      .mockResolvedValueOnce(null)
    buildAccountNameMock.mockImplementation((tokenInfo?: any) => `Kiro ${tokenInfo?.email || 'OAuth'}`)
    const wrapper = mountModal(buildKiroAccount('oauth'))

    wrapper.getComponent(KiroAuthorizationFlowStub).vm.$emit('submit-refresh-token', {
      credentials: {
        refresh_token: 'rt-one\nrt-two\nrt-three',
        auth_method: 'social',
        region: 'eu-west-1'
      },
      extra: {}
    })
    await flushPromises()

    expect(reauthorizeKiroOAuthMock).toHaveBeenCalledTimes(1)
    expect(createAccountMock).toHaveBeenCalledTimes(1)
    expect(wrapper.emitted('reauthorized')).toHaveLength(1)
    expect(wrapper.emitted('refresh')).toHaveLength(1)
  })

  it('removes stale IDC credentials when Kiro reauth switches back to social refresh tokens', async () => {
    const account = buildKiroAccount('oauth')
    account.credentials = {
      refresh_token: 'rt-old',
      auth_method: 'idc',
      client_id: 'old-client',
      client_secret: 'old-secret',
      issuer_url: 'https://old-idc.example.com',
      idc_region: 'us-west-2',
      region: 'us-east-1'
    }
    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="kiro-flow-submit-refresh-token"]').trigger('click')
    await flushPromises()

    const credentials = reauthorizeKiroOAuthMock.mock.calls[0]?.[1]?.credentials
    expect(credentials).toEqual(expect.objectContaining({
      auth_method: 'social',
      refresh_token: 'refresh-validated'
    }))
    expect(credentials).not.toHaveProperty('client_id')
    expect(credentials).not.toHaveProperty('client_secret')
    expect(credentials).not.toHaveProperty('issuer_url')
    expect(credentials).not.toHaveProperty('idc_region')
  })

  it('preserves ExternalIdp metadata when Kiro manual refresh-token reauth validates', async () => {
    validateRefreshTokenMock.mockResolvedValue({
      access_token: 'access-external',
      refresh_token: 'refresh-external-new',
      expires_at: '2026-07-01T07:57:02Z',
      auth_method: 'external_idp',
      client_id: 'microsoft-public-client',
      issuer_url: 'https://login.microsoftonline.com/tenant/v2.0',
      token_endpoint: 'https://login.microsoftonline.com/tenant/oauth2/v2.0/token',
      scopes: 'scope-a scope-b',
      login_hint: 'external@example.com',
      profile_arn: 'arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD',
      email: 'external@example.com',
      plan_name: 'Credit'
    })
    buildAccountNameMock.mockReturnValue('Kiro ExternalIdp')
    buildExtraInfoMock.mockReturnValue({
      email: 'external@example.com',
      subscription_type: 'Credit'
    })
    const account = buildKiroAccount('oauth')
    account.credentials = {
      refresh_token: 'rt-old',
      auth_method: 'social',
      region: 'us-east-1'
    }
    const wrapper = mountModal(account)

    wrapper.getComponent(KiroAuthorizationFlowStub).vm.$emit('submit-refresh-token', {
      credentials: {
        refresh_token: 'refresh-external',
        auth_method: 'external_idp',
        region: 'us-east-1',
        client_id: 'microsoft-public-client',
        issuer_url: 'https://login.microsoftonline.com/tenant/v2.0',
        token_endpoint: 'https://login.microsoftonline.com/tenant/oauth2/v2.0/token',
        scopes: 'scope-a scope-b',
        login_hint: 'external@example.com',
        profile_arn: 'arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD'
      },
      extra: {}
    })
    await flushPromises()

    expect(validateRefreshTokenMock).toHaveBeenCalledWith(
      expect.objectContaining({
        auth_method: 'external_idp',
        client_id: 'microsoft-public-client',
        token_endpoint: 'https://login.microsoftonline.com/tenant/oauth2/v2.0/token'
      }),
      {},
      null,
      false
    )
    const credentials = reauthorizeKiroOAuthMock.mock.calls[0]?.[1]?.credentials
    expect(credentials).toEqual(expect.objectContaining({
      auth_method: 'external_idp',
      refresh_token: 'refresh-external-new',
      access_token: 'access-external',
      client_id: 'microsoft-public-client',
      issuer_url: 'https://login.microsoftonline.com/tenant/v2.0',
      token_endpoint: 'https://login.microsoftonline.com/tenant/oauth2/v2.0/token',
      scopes: 'scope-a scope-b',
      login_hint: 'external@example.com',
      profile_arn: 'arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD'
    }))
  })

  it('shows Kiro diagnostic summary in reauth modal', async () => {
    const account = buildKiroAccount('oauth')
    account.credentials = {
      refresh_token: 'rt-old',
      region: 'us-east-1',
      profile_arn: 'arn:aws:codewhisperer:us-east-1:123:profile/CURRENT'
    }
    account.extra = {
      profile_id: 'PROFILE-123',
      login_provider: 'microsoft',
      kiro_status_reason: 'FEATURE_NOT_SUPPORTED'
    }

    const wrapper = mountModal(account)
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.kiro.diagnosticSummaryTitle')
    expect(wrapper.text()).toContain('admin.accounts.kiro.profileModeShort')
    expect(wrapper.text()).toContain('admin.accounts.kiro.profileStateManual')
    expect(wrapper.text()).toContain('PROFILE-123')
    expect(wrapper.text()).toContain('microsoft')
    expect(wrapper.text()).toContain('FEATURE_NOT_SUPPORTED')
  })

  it('removes stale IDC credentials when Kiro callback reauth switches back to social credentials', async () => {
    const account = buildKiroAccount('oauth')
    account.credentials = {
      refresh_token: 'rt-old',
      auth_method: 'idc',
      client_id: 'old-client',
      client_secret: 'old-secret',
      issuer_url: 'https://old-idc.example.com',
      idc_region: 'us-west-2',
      region: 'us-east-1'
    }
    buildCredentialsMock.mockReturnValue({
      access_token: 'access-new',
      refresh_token: 'refresh-new',
      auth_method: 'social',
      region: 'us-east-1'
    })
    const wrapper = mountModal(account)

    await wrapper.get('[data-testid="kiro-flow-submit"]').trigger('click')
    await flushPromises()

    const credentials = reauthorizeKiroOAuthMock.mock.calls[0]?.[1]?.credentials
    expect(credentials).toEqual(expect.objectContaining({
      auth_method: 'social',
      refresh_token: 'refresh-new'
    }))
    expect(credentials).not.toHaveProperty('client_id')
    expect(credentials).not.toHaveProperty('client_secret')
    expect(credentials).not.toHaveProperty('issuer_url')
    expect(credentials).not.toHaveProperty('idc_region')
  })

  it('reauthorizes Grok OAuth accounts from manual refresh token input', async () => {
    const wrapper = mountModal(buildGrokOAuthAccount())

    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('validate-refresh-token', 'grok-rt-manual')
    await flushPromises()

    expect(grokValidateRefreshTokenMock).toHaveBeenCalledWith('grok-rt-manual', null)
    expect(applyOAuthCredentialsMock).toHaveBeenCalledWith(54, expect.objectContaining({
      type: 'oauth',
      credentials: expect.objectContaining({
        refresh_token: 'grok-rt-new',
        access_token: 'grok-at-new'
      }),
      extra: expect.objectContaining({
        email: 'grok@example.com'
      })
    }))
  })

  it('reauthorizes Grok OAuth accounts from manual SSO token input', async () => {
    const wrapper = mountModal(buildGrokOAuthAccount())

    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('validate-sso-token', 'grok-sso-manual')
    await flushPromises()

    expect(grokValidateSSOTokenMock).toHaveBeenCalledWith('grok-sso-manual', null)
    expect(applyOAuthCredentialsMock).toHaveBeenCalledWith(54, expect.objectContaining({
      credentials: expect.objectContaining({
        sso_token: 'grok-sso-new',
        refresh_token: 'grok-rt-new'
      })
    }))
  })

  it('reauthorizes Grok OAuth accounts from manual email-password input without persisting password', async () => {
    const wrapper = mountModal(buildGrokOAuthAccount())

    wrapper.getComponent(OAuthAuthorizationFlowStub).vm.$emit('authorize-password', 'grok@example.com----super-secret')
    await flushPromises()

    expect(grokAuthorizePasswordMock).toHaveBeenCalledWith('grok@example.com----super-secret', null)
    const payload = applyOAuthCredentialsMock.mock.calls[0]?.[1]
    expect(payload.credentials).toEqual(expect.objectContaining({
      sso_token: 'grok-password-sso',
      refresh_token: 'grok-rt-new'
    }))
    expect(JSON.stringify(payload)).not.toContain('super-secret')
  })

  it('rejects multiline Grok credentials in single-account reauthorization', async () => {
    const wrapper = mountModal(buildGrokOAuthAccount())
    const flow = wrapper.getComponent(OAuthAuthorizationFlowStub)

    flow.vm.$emit('validate-refresh-token', 'rt-one\nrt-two')
    flow.vm.$emit('validate-sso-token', 'sso-one\nsso-two')
    flow.vm.$emit('authorize-password', 'one@example.com----one\ntwo@example.com----two')
    await flushPromises()

    expect(grokValidateRefreshTokenMock).not.toHaveBeenCalled()
    expect(grokValidateSSOTokenMock).not.toHaveBeenCalled()
    expect(grokAuthorizePasswordMock).not.toHaveBeenCalled()
    expect(applyOAuthCredentialsMock).not.toHaveBeenCalled()
  })

  it('shows Kiro diagnostics stored in account credentials', () => {
    const account = buildKiroAccount('oauth')
    account.credentials = {
      ...account.credentials,
      profile_id: 'PROFILE-CREDENTIALS',
      login_provider: 'google',
      status_reason: 'FEATURE_NOT_SUPPORTED'
    }
    const wrapper = mountModal(account)

    expect(wrapper.text()).toContain('admin.accounts.kiro.diagnosticSummaryTitle')
  })
})
