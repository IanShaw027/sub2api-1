import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import DingTalkCallbackView from '../DingTalkCallbackView.vue'

const replace = vi.fn()
const showSuccess = vi.fn()
const showError = vi.fn()
const setToken = vi.fn()
const setPendingAuthSession = vi.fn()
const clearPendingAuthSession = vi.fn()
const exchangePendingOAuthCompletion = vi.fn()
const login2FA = vi.fn()
const apiClientPost = vi.fn()

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
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'auth.dingtalk.providerName') {
          return 'DingTalk'
        }
        if (key === 'auth.oauthFlow.reviewProfileBeforeContinue') {
          return `Review the ${params?.providerName ?? ''} profile details before continuing.`
        }
        return key
      },
      te: () => false,
    }),
  }
})

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    setToken,
    setPendingAuthSession,
    clearPendingAuthSession,
  }),
  useAppStore: () => ({
    showSuccess,
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
    exchangePendingOAuthCompletion: (...args: any[]) => exchangePendingOAuthCompletion(...args),
    login2FA: (...args: any[]) => login2FA(...args),
  }
})

describe('DingTalkCallbackView', () => {
  beforeEach(() => {
    routeState.query = {}
    replace.mockReset()
    showSuccess.mockReset()
    showError.mockReset()
    setToken.mockReset()
    setPendingAuthSession.mockReset()
    clearPendingAuthSession.mockReset()
    exchangePendingOAuthCompletion.mockReset()
    login2FA.mockReset()
    apiClientPost.mockReset()
    window.location.hash = ''
    localStorage.clear()
    sessionStorage.clear()
  })

  it('persists the pending auth session before redirecting to the DingTalk email completion route', async () => {
    exchangePendingOAuthCompletion.mockResolvedValue({
      step: 'email_completion',
      redirect: '/profile',
    })

    mount(DingTalkCallbackView, {
      global: {
        stubs: {
          AuthLayout: { template: '<div><slot /></div>' },
          PendingOAuthCreateAccountForm: true,
          transition: false,
        },
      },
    })

    await flushPromises()

    expect(setPendingAuthSession).toHaveBeenCalledWith(expect.objectContaining({
      token: '',
      token_field: 'pending_oauth_token',
      provider: 'dingtalk',
      redirect: '/profile',
      adoption_required: false,
      adopt_display_name: false,
      adopt_avatar: false,
    }))
    expect(replace).toHaveBeenCalledWith('/auth/dingtalk/email-completion?redirect=%2Fprofile')
  })

  it('uses the localized DingTalk provider name in shared oauth flow copy', async () => {
    exchangePendingOAuthCompletion.mockResolvedValue({
      redirect: '/profile',
      adoption_required: true,
      suggested_display_name: 'Alice',
    })

    const wrapper = mount(DingTalkCallbackView, {
      global: {
        stubs: {
          AuthLayout: { template: '<div><slot /></div>' },
          PendingOAuthCreateAccountForm: true,
          transition: false,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Review the DingTalk profile details before continuing.')
    expect(wrapper.text()).not.toContain('Review the 钉钉 profile details before continuing.')
  })
})
