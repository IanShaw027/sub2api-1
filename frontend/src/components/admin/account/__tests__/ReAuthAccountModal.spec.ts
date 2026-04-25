import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'

const {
  updateAccountMock,
  clearErrorMock,
  exchangeCallbackMock,
  validateRefreshTokenMock,
  buildCredentialsMock,
  buildExtraInfoMock,
  buildAccountNameMock
} = vi.hoisted(() => ({
  updateAccountMock: vi.fn(),
  clearErrorMock: vi.fn(),
  exchangeCallbackMock: vi.fn(),
  validateRefreshTokenMock: vi.fn(),
  buildCredentialsMock: vi.fn(),
  buildExtraInfoMock: vi.fn(),
  buildAccountNameMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      update: updateAccountMock,
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
    oauthState: ref('oauth-state'),
    state: ref('oauth-state'),
    resetState: vi.fn(),
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
  useGeminiOAuth: () => buildOAuthComposable()
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
    resetState: vi.fn(),
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
  setup(_, { expose }) {
    expose({
      authCode: '',
      oauthState: '',
      projectId: '',
      sessionKey: '',
      inputMethod: 'manual',
      reset: vi.fn()
    })
    return () => h('div', { 'data-testid': 'oauth-flow' })
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
    clearErrorMock.mockReset()
    exchangeCallbackMock.mockReset()
    validateRefreshTokenMock.mockReset()
    buildCredentialsMock.mockReset()
    buildExtraInfoMock.mockReset()
    buildAccountNameMock.mockReset()

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
    clearErrorMock.mockResolvedValue(buildKiroAccount('oauth'))
  })

  it('does not mount the Kiro OAuth reauthorization flow for Kiro API key accounts', () => {
    const wrapper = mountModal(buildKiroAccount('apikey'))

    expect(wrapper.find('[data-testid="kiro-flow-submit"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="oauth-flow"]').exists()).toBe(false)
    expect(updateAccountMock).not.toHaveBeenCalled()
  })

  it('reauthorizes Kiro OAuth accounts from callback submission without preserving runtime-only extra fields', async () => {
    const wrapper = mountModal(buildKiroAccount('oauth'))

    await wrapper.get('[data-testid="kiro-flow-submit"]').trigger('click')
    await flushPromises()

    expect(exchangeCallbackMock).toHaveBeenCalledWith(
      'http://localhost:3128/callback?code=abc',
      null
    )
    expect(updateAccountMock).toHaveBeenCalledWith(42, expect.objectContaining({
      name: 'Kiro OAuth',
      type: 'oauth',
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
    expect(updateAccountMock).toHaveBeenCalledWith(42, {
      name: 'Kiro OAuth',
      type: 'oauth',
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
})
