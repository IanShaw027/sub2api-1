import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ResetPasswordView from '@/views/auth/ResetPasswordView.vue'

const { routeState, showErrorMock, resetPasswordMock } = vi.hoisted(() => ({
  routeState: {
    path: '/reset-password',
    query: {} as Record<string, string>,
  },
  showErrorMock: vi.fn(),
  resetPasswordMock: vi.fn(),
}))

vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => routeState,
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/api/auth', () => ({
  resetPassword: (...args: unknown[]) => resetPasswordMock(...args),
}))

function mountResetPassword() {
  return mount(ResetPasswordView, {
    global: {
      stubs: {
        AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
        Icon: true,
        RouterLink: true,
        transition: false,
      },
    },
  })
}

describe('ResetPasswordView', () => {
  beforeEach(() => {
    routeState.query = { email: 'user@example.com', token: 'reset-secret' }
    showErrorMock.mockReset()
    resetPasswordMock.mockReset()
    resetPasswordMock.mockResolvedValue({})
  })

  it('removes reset credentials from the address bar immediately after reading them', () => {
    const replaceState = vi.spyOn(window.history, 'replaceState')

    mount(ResetPasswordView, {
      global: {
        stubs: {
          AuthLayout: { template: '<div><slot /><slot name="footer" /></div>' },
          Icon: true,
          RouterLink: true,
          transition: false,
        },
      },
    })

    expect(replaceState).toHaveBeenCalledWith(window.history.state, '', '/reset-password')
    replaceState.mockRestore()
  })

  it('preserves readonly email, independent reveal controls and password mismatch errors', async () => {
    const wrapper = mountResetPassword()
    await flushPromises()
    expect(wrapper.get<HTMLInputElement>('#email').element.value).toBe('user@example.com')
    expect(wrapper.get<HTMLInputElement>('#email').element.readOnly).toBe(true)
    expect(wrapper.get<HTMLInputElement>('#email').element.disabled).toBe(true)
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('different-password')
    await wrapper.get('[aria-controls="password"]').trigger('click')
    expect(wrapper.get('#password').attributes('type')).toBe('text')
    expect(wrapper.get('#confirmPassword').attributes('type')).toBe('password')
    await wrapper.get('[aria-controls="confirmPassword"]').trigger('click')
    expect(wrapper.get('#confirmPassword').attributes('type')).toBe('text')
    expect(wrapper.get('#password').attributes('autocomplete')).toBe('new-password')
    await wrapper.get('form').trigger('submit')
    expect(resetPasswordMock).not.toHaveBeenCalled()
    expect(wrapper.get('#confirmPassword').attributes('aria-invalid')).toBe('true')
    expect(wrapper.get('#confirmPassword').attributes('aria-describedby')).toBe('confirmPassword-error')
    expect(wrapper.get('#confirmPassword-error').text()).toBe('auth.passwordsDoNotMatch')
    wrapper.unmount()
  })

  it('submits the original reset token and disables all password controls until completion', async () => {
    let complete: () => void = () => {}
    resetPasswordMock.mockImplementationOnce(() => new Promise<void>((resolve) => { complete = resolve }))
    const wrapper = mountResetPassword()
    await flushPromises()
    await wrapper.get('#password').setValue('secret-123')
    await wrapper.get('#confirmPassword').setValue('secret-123')
    await wrapper.get('form').trigger('submit')
    expect(resetPasswordMock).toHaveBeenCalledWith({
      email: 'user@example.com',
      token: 'reset-secret',
      new_password: 'secret-123'
    })
    expect(wrapper.get<HTMLInputElement>('#password').element.disabled).toBe(true)
    expect(wrapper.get<HTMLButtonElement>('[aria-controls="password"]').element.disabled).toBe(true)
    expect(wrapper.get<HTMLButtonElement>('[aria-controls="confirmPassword"]').element.disabled).toBe(true)
    complete()
    await flushPromises()
    expect(wrapper.text()).toContain('auth.passwordResetSuccess')
    expect(wrapper.find('form').exists()).toBe(false)
    wrapper.unmount()
  })
})
