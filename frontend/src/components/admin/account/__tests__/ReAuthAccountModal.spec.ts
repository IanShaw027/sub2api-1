import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'

const {
  updateAccountMock,
  reauthorizeKiroOAuthMock,
  createAccountMock,
  clearErrorMock,
  exchangeCallbackMock,
  validateRefreshTokenMock,
  geminiGenerateAuthUrlMock,
  buildCredentialsMock,
  buildExtraInfoMock,
  buildAccountNameMock
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  reauthorizeKiroOAuthMock: vi.fn(),
  createAccountMock: vi.fn(),
  clearErrorMock: vi.fn(),
  exchangeCallbackMock: vi.fn(),
  validateRefreshTokenMock: vi.fn(),
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
    }
  },
  emits: ['generate-url', 'submit', 'submit-refresh-token'],
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
  emits: ['generate-url'],
  setup(props, { expose, emit }) {
    expose({
      authCode: '',
      oauthState: '',
      projectId: '',
      requiresProjectIdRecovery: false,
      sessionKey: '',
      inputMethod: 'manual',
      reset: vi.fn()
    })
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
    updateAccountMock.mockReset()
    reauthorizeKiroOAuthMock.mockReset()
    createAccountMock.mockReset()
    clearErrorMock.mockReset()
    exchangeCallbackMock.mockReset()
    validateRefreshTokenMock.mockReset()
    geminiGenerateAuthUrlMock.mockReset()
    buildCredentialsMock.mockReset()
    buildExtraInfoMock.mockReset()
    buildAccountNameMock.mockReset()
    createAccountMock.mockResolvedValue({})

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
    reauthorizeKiroOAuthMock.mockResolvedValue({})
    clearErrorMock.mockResolvedValue(buildKiroAccount('oauth'))
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
    expect(clearErrorMock).toHaveBeenCalledWith(42)
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
      null
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
    expect(clearErrorMock).toHaveBeenCalledWith(42)
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
      null
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
      null
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
    expect(clearErrorMock).toHaveBeenCalledWith(42)
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
      null
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
})
