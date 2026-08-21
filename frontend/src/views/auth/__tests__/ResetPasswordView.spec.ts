import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ResetPasswordView from '@/views/auth/ResetPasswordView.vue'

const { routeState, showErrorMock } = vi.hoisted(() => ({
  routeState: {
    path: '/reset-password',
    query: {} as Record<string, string>,
  },
  showErrorMock: vi.fn(),
}))

vi.mock('vue-router', () => ({
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

describe('ResetPasswordView', () => {
  beforeEach(() => {
    routeState.query = { email: 'user@example.com', token: 'reset-secret' }
    showErrorMock.mockReset()
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
})
