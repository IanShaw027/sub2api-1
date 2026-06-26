import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileBalanceNotifyCard from '@/components/user/profile/ProfileBalanceNotifyCard.vue'

const {
  updateProfileMock,
  toggleNotifyEmailMock,
  sendNotifyEmailCodeMock,
  verifyNotifyEmailMock,
  removeNotifyEmailMock,
  getProfileMock,
  showSuccessMock,
  showErrorMock,
  authState,
} = vi.hoisted(() => ({
  updateProfileMock: vi.fn(),
  toggleNotifyEmailMock: vi.fn(),
  sendNotifyEmailCodeMock: vi.fn(),
  verifyNotifyEmailMock: vi.fn(),
  removeNotifyEmailMock: vi.fn(),
  getProfileMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
  authState: {
    user: {
      id: 1,
      email: 'owner@example.com',
      balance_notify_extra_emails: [] as Array<{ email: string; disabled: boolean; verified: boolean }>,
    },
  },
}))

vi.mock('@/api', () => ({
  userAPI: {
    updateProfile: updateProfileMock,
    toggleNotifyEmail: toggleNotifyEmailMock,
    sendNotifyEmailCode: sendNotifyEmailCodeMock,
    verifyNotifyEmail: verifyNotifyEmailMock,
    removeNotifyEmail: removeNotifyEmailMock,
    getProfile: getProfileMock,
  },
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showSuccess: showSuccessMock,
    showError: showErrorMock,
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function mountCard(props: Record<string, unknown> = {}) {
  return mount(ProfileBalanceNotifyCard, {
    props: {
      enabled: true,
      threshold: null,
      extraEmails: [],
      systemDefaultThreshold: 0,
      userEmail: 'owner@example.com',
      ...props,
    },
  })
}

function findRowByText(wrapper: ReturnType<typeof mount>, text: string) {
  const row = wrapper.findAll('div')
    .filter((node) => node.text().includes(text) && node.findAll('button').length > 0)
    .sort((a, b) => a.text().length - b.text().length)[0]
  expect(row).toBeDefined()
  return row!
}

function findButtonByText(root: ReturnType<typeof mount> | ReturnType<typeof findRowByText>, text: string) {
  const button = root.findAll('button').find((node) => node.text() === text)
  expect(button).toBeDefined()
  return button!
}

describe('ProfileBalanceNotifyCard request reduction', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    authState.user = {
      id: 1,
      email: 'owner@example.com',
      balance_notify_extra_emails: [],
    }
    updateProfileMock.mockResolvedValue(authState.user)
    toggleNotifyEmailMock.mockResolvedValue(authState.user)
    sendNotifyEmailCodeMock.mockResolvedValue(undefined)
    verifyNotifyEmailMock.mockResolvedValue(undefined)
    removeNotifyEmailMock.mockResolvedValue(undefined)
    getProfileMock.mockResolvedValue(authState.user)
  })

  it('verifies a pending email without reloading the full profile', async () => {
    const wrapper = mountCard()

    await wrapper.get('input[type="email"]').setValue('new@example.com')
    await findButtonByText(wrapper, 'common.add').trigger('click')
    await flushPromises()

    await findButtonByText(wrapper, 'profile.balanceNotify.sendCode').trigger('click')
    await flushPromises()

    await wrapper.get('input[maxlength="6"]').setValue('123456')
    await findButtonByText(wrapper, 'profile.balanceNotify.verify').trigger('click')
    await flushPromises()

    expect(verifyNotifyEmailMock).toHaveBeenCalledWith('new@example.com', '123456')
    expect(getProfileMock).not.toHaveBeenCalled()
    expect(authState.user.balance_notify_extra_emails).toEqual([
      { email: 'new@example.com', disabled: false, verified: true },
    ])

    wrapper.unmount()
  })

  it('verifies a saved email without reloading the full profile', async () => {
    authState.user.balance_notify_extra_emails = [
      { email: 'saved@example.com', disabled: false, verified: false },
    ]

    const wrapper = mountCard({
      extraEmails: authState.user.balance_notify_extra_emails,
    })

    await findButtonByText(wrapper, 'profile.balanceNotify.verify').trigger('click')
    await flushPromises()

    const savedRow = findRowByText(wrapper, 'saved@example.com')
    await savedRow.get('input[maxlength="6"]').setValue('654321')
    await findButtonByText(savedRow, 'profile.balanceNotify.verify').trigger('click')
    await flushPromises()

    expect(verifyNotifyEmailMock).toHaveBeenCalledWith('saved@example.com', '654321')
    expect(getProfileMock).not.toHaveBeenCalled()
    expect(authState.user.balance_notify_extra_emails).toEqual([
      { email: 'saved@example.com', disabled: false, verified: true },
    ])

    wrapper.unmount()
  })

  it('removes a saved email without reloading the full profile', async () => {
    authState.user.balance_notify_extra_emails = [
      { email: 'keep@example.com', disabled: false, verified: true },
      { email: 'remove@example.com', disabled: false, verified: true },
    ]

    const wrapper = mountCard({
      extraEmails: authState.user.balance_notify_extra_emails,
    })

    const row = findRowByText(wrapper, 'remove@example.com')
    await findButtonByText(row, 'profile.balanceNotify.removeEmail').trigger('click')
    await flushPromises()

    expect(removeNotifyEmailMock).toHaveBeenCalledWith('remove@example.com')
    expect(getProfileMock).not.toHaveBeenCalled()
    expect(authState.user.balance_notify_extra_emails).toEqual([
      { email: 'keep@example.com', disabled: false, verified: true },
    ])

    wrapper.unmount()
  })
})
