import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import RegisterView from '@/views/auth/RegisterView.vue'

const {
  pushMock,
  getPublicSettingsMock,
  sendVerifyCodeMock,
  showErrorMock,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  getPublicSettingsMock: vi.fn(),
  sendVerifyCodeMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
  useRoute: () => ({ query: {} }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'en' },
    }),
  }
})

vi.mock('@/stores', () => ({
  useAuthStore: () => ({
    register: vi.fn(),
  }),
  useAppStore: () => ({
    showError: showErrorMock,
    showWarning: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/auth', async () => {
  const actual = await vi.importActual<typeof import('@/api/auth')>('@/api/auth')
  return {
    ...actual,
    getPublicSettings: (...args: unknown[]) => getPublicSettingsMock(...args),
    sendVerifyCode: (...args: unknown[]) => sendVerifyCodeMock(...args),
    validatePromoCode: vi.fn(),
    validateInvitationCode: vi.fn(),
  }
})

const TurnstileStub = defineComponent({
  name: 'TurnstileWidget',
  emits: ['verify', 'expire', 'error'],
  template: '<button data-test="complete-turnstile" type="button" @click="$emit(\'verify\', \'turnstile-secret\')">verify</button>',
})

describe('RegisterView', () => {
  beforeEach(() => {
    sessionStorage.clear()
    localStorage.clear()
    pushMock.mockReset()
    getPublicSettingsMock.mockReset()
    sendVerifyCodeMock.mockReset()
    showErrorMock.mockReset()
    getPublicSettingsMock.mockResolvedValue({
      registration_enabled: true,
      email_verify_enabled: true,
      promo_code_enabled: false,
      invitation_code_enabled: false,
      turnstile_enabled: true,
      turnstile_site_key: 'site-key',
      site_name: 'Sub2API',
      linuxdo_oauth_enabled: false,
      wechat_oauth_enabled: false,
      oidc_oauth_enabled: false,
      github_oauth_enabled: false,
      google_oauth_enabled: false,
      registration_email_suffix_whitelist: [],
      login_agreement_enabled: false,
      login_agreement_documents: [],
    })
    sendVerifyCodeMock.mockResolvedValue({ countdown: 60 })
  })

  it('sends the challenge before navigation and persists no password or Turnstile token', async () => {
    const wrapper = mount(RegisterView, {
      global: {
        stubs: {
          AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
          LinuxDoOAuthSection: true,
          OidcOAuthSection: true,
          WechatOAuthSection: true,
          EmailOAuthButtons: true,
          LoginAgreementPrompt: true,
          Icon: true,
          TurnstileWidget: TurnstileStub,
          transition: false,
        },
      },
    })
    await flushPromises()

    await wrapper.get('#email').setValue('safe@example.com')
    await wrapper.get('#password').setValue('password-123')
    await wrapper.get('[data-test="complete-turnstile"]').trigger('click')
    await wrapper.get('form').trigger('submit.prevent')
    await flushPromises()

    expect(sendVerifyCodeMock).toHaveBeenCalledWith({
      email: 'safe@example.com',
      turnstile_token: 'turnstile-secret',
    })
    const raw = sessionStorage.getItem('register_data')
    expect(raw).not.toBeNull()
    expect(raw).not.toContain('password-123')
    expect(raw).not.toContain('turnstile-secret')
    expect(JSON.parse(raw || '{}')).toMatchObject({
      email: 'safe@example.com',
      code_countdown: 60,
    })
    expect(pushMock).toHaveBeenCalledWith('/email-verify')
    expect(showErrorMock).not.toHaveBeenCalled()
  })
})
