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
const getPublicSettings = vi.fn()
const login2FA = vi.fn()
const apiClientPost = vi.fn()
const sendVerifyCode = vi.fn()
const sendPendingOAuthVerifyCode = vi.fn()

const routeState: { query: Record<string, unknown> } = { query: {} }

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    replace
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'auth.oauthFlow.totpHint') {
          return `verify ${params?.account ?? ''}`.trim()
        }
        return key
      },
      te: (key: string) => key === 'auth.dingtalk.error.csrf'
    })
  }
})

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    setToken,
    setPendingAuthSession,
    clearPendingAuthSession
  }),
  useAppStore: () => ({
    showSuccess,
    showError
  })
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post: (...args: any[]) => apiClientPost(...args)
  }
}))

vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    exchangePendingOAuthCompletion: (...args: any[]) => exchangePendingOAuthCompletion(...args),
    getPublicSettings: (...args: any[]) => getPublicSettings(...args),
    login2FA: (...args: any[]) => login2FA(...args),
    sendVerifyCode: (...args: any[]) => sendVerifyCode(...args),
    sendPendingOAuthVerifyCode: (...args: any[]) => sendPendingOAuthVerifyCode(...args)
  }
})

const globalStubs = {
  AuthLayout: { template: '<div><slot /></div>' },
  Icon: true,
  RouterLink: { template: '<a><slot /></a>' },
  transition: false
}

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
    getPublicSettings.mockReset()
    login2FA.mockReset()
    apiClientPost.mockReset()
    sendVerifyCode.mockReset()
    sendPendingOAuthVerifyCode.mockReset()
    getPublicSettings.mockResolvedValue({
      turnstile_enabled: false,
      turnstile_site_key: ''
    })
    window.location.hash = ''
    localStorage.clear()
    sessionStorage.clear()
  })

  it('renders a CallbackStatusCard loading state before the exchange settles', () => {
    exchangePendingOAuthCompletion.mockReturnValue(new Promise(() => {}))

    const wrapper = mount(DingTalkCallbackView, { global: { stubs: globalStubs } })

    const card = wrapper.find('[data-testid="callback-status-card"]')
    expect(card.exists()).toBe(true)
    expect(card.attributes('data-status')).toBe('loading')
  })

  it('renders a CallbackStatusCard error state for a known fragment error and offers a back-to-login action', async () => {
    window.location.hash = '#error=csrf'
    exchangePendingOAuthCompletion.mockRejectedValue(new Error('should not be called for fragment errors'))

    const wrapper = mount(DingTalkCallbackView, { global: { stubs: globalStubs } })
    await flushPromises()

    const card = wrapper.find('[data-testid="callback-status-card"]')
    expect(card.exists()).toBe(true)
    expect(card.attributes('data-status')).toBe('error')
    expect(card.text()).toContain('auth.dingtalk.error.csrf')

    await wrapper.find('button').trigger('click')
    expect(replace).toHaveBeenCalledWith('/login')
  })

  it('accepts the legacy fragment token success callback without pending-session exchange', async () => {
    window.location.hash =
      '#access_token=legacy-access-token&refresh_token=legacy-refresh-token&expires_in=3600&token_type=Bearer&redirect=%2Flegacy-dashboard'
    setToken.mockResolvedValue({})

    mount(DingTalkCallbackView, { global: { stubs: globalStubs } })
    await flushPromises()

    expect(exchangePendingOAuthCompletion).not.toHaveBeenCalled()
    expect(setToken).toHaveBeenCalledWith('legacy-access-token')
    expect(showSuccess).toHaveBeenCalledWith('auth.loginSuccess')
    expect(replace).toHaveBeenCalledWith('/legacy-dashboard')
  })
})
