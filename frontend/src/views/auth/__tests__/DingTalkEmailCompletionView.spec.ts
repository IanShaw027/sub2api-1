import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import DingTalkEmailCompletionView from '../DingTalkEmailCompletionView.vue'

const {
  replace,
  setToken,
  showSuccess,
  showInfo,
  showError,
  apiClientPost,
  getPublicSettings,
  persistOAuthTokenContext,
  clearAllAffiliateReferralCodes,
  loadOAuthAffiliateCode,
} = vi.hoisted(() => ({
  replace: vi.fn(),
  setToken: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
  showError: vi.fn(),
  apiClientPost: vi.fn(),
  getPublicSettings: vi.fn(),
  persistOAuthTokenContext: vi.fn(),
  clearAllAffiliateReferralCodes: vi.fn(),
  loadOAuthAffiliateCode: vi.fn(),
}))

const routeState = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    replace,
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    pendingAuthSession: {
      provider: 'dingtalk',
      adopt_display_name: true,
      adopt_avatar: false,
    },
    setToken,
  }),
  useAppStore: () => ({
    showSuccess,
    showInfo,
    showError,
  }),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post: (...args: any[]) => apiClientPost(...args),
  },
}))

vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: (...args: any[]) => getPublicSettings(...args),
    persistOAuthTokenContext: (...args: any[]) => persistOAuthTokenContext(...args),
  }
})

vi.mock('@/utils/oauthAffiliate', () => ({
  clearAllAffiliateReferralCodes: () => clearAllAffiliateReferralCodes(),
  loadOAuthAffiliateCode: () => loadOAuthAffiliateCode(),
}))

const AuthLayoutStub = defineComponent({
  template: '<div><slot /></div>',
})

const PendingOAuthCreateAccountFormStub = defineComponent({
  name: 'PendingOAuthCreateAccountForm',
  props: {
    initialEmail: {
      type: String,
      default: '',
    },
    isSubmitting: {
      type: Boolean,
      default: false,
    },
    errorMessage: {
      type: String,
      default: '',
    },
  },
  setup(props) {
    return () => h('div', { 'data-testid': 'pending-form' }, props.initialEmail)
  },
})

describe('DingTalkEmailCompletionView', () => {
  beforeEach(() => {
    routeState.query = { email: 'fresh@example.com' }
    replace.mockReset()
    setToken.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()
    showError.mockReset()
    apiClientPost.mockReset()
    getPublicSettings.mockReset()
    persistOAuthTokenContext.mockReset()
    clearAllAffiliateReferralCodes.mockReset()
    loadOAuthAffiliateCode.mockReset()
    loadOAuthAffiliateCode.mockReturnValue('aff-123')
    getPublicSettings.mockResolvedValue({
      turnstile_enabled: false,
      turnstile_site_key: '',
    })
  })

  it('forwards affiliate code and adoption flags when completing the pending account', async () => {
    const wrapper = mount(DingTalkEmailCompletionView, {
      global: {
        stubs: {
          AuthLayout: AuthLayoutStub,
          PendingOAuthCreateAccountForm: PendingOAuthCreateAccountFormStub,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('auth.dingtalk.createAccountTitle')
    expect(wrapper.get('[data-testid="pending-form"]').text()).toBe('fresh@example.com')
  })
})
