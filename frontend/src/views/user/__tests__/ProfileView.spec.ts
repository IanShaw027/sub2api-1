import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileView from '@/views/user/ProfileView.vue'

const {
  fetchPublicSettingsMock,
  refreshUserMock,
  authState
} = vi.hoisted(() => ({
  fetchPublicSettingsMock: vi.fn(),
  refreshUserMock: vi.fn(),
  authState: {
    user: null as Record<string, unknown> | null,
    refreshUser: vi.fn()
  }
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    fetchPublicSettings: fetchPublicSettingsMock
  })
}))

vi.mock('@/utils/format', () => ({
  formatDate: () => 'April 2026'
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('ProfileView', () => {
  beforeEach(() => {
    refreshUserMock.mockReset()
    fetchPublicSettingsMock.mockReset()
    refreshUserMock.mockResolvedValue(undefined)
    authState.refreshUser = refreshUserMock
    authState.user = {
      id: 1,
      username: 'alice',
      email: 'alice@example.com',
      role: 'user',
      balance: 10,
      concurrency: 2,
      status: 'active',
      allowed_groups: null,
      balance_notify_enabled: true,
      balance_notify_threshold: null,
      balance_notify_extra_emails: [],
      created_at: '2026-04-20T00:00:00Z',
      updated_at: '2026-04-20T00:00:00Z'
    }
    fetchPublicSettingsMock.mockResolvedValue({
      contact_info: '',
      support_qr_codes: [],
      balance_low_notify_enabled: false,
      balance_low_notify_threshold: 0,
      linuxdo_oauth_enabled: true,
      wechat_oauth_enabled: true,
      wechat_oauth_open_enabled: true,
      wechat_oauth_mp_enabled: false,
      oidc_oauth_enabled: true,
      oidc_oauth_provider_name: 'OIDC'
    })
  })

  it('renders the simplified single-column profile shell without separate stat cards', async () => {
    const wrapper = mount(ProfileView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          StatCard: { template: '<div class="stat-card" />' },
          ProfileInfoCard: { template: '<div data-testid="profile-info-card" />' },
          ProfileBalanceNotifyCard: { template: '<div data-testid="profile-balance-notify-card" />' },
          ProfilePasswordForm: { template: '<div data-testid="profile-password-form" />' },
          ProfileTotpCard: { template: '<div data-testid="profile-totp-card" />' },
          Icon: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.findAll('.stat-card')).toHaveLength(0)
    expect(wrapper.get('[data-testid="profile-shell"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-shell"]').html()).toContain('profile-info-card')
    expect(wrapper.get('[data-testid="profile-shell"]').html()).toContain('profile-password-form')
    expect(wrapper.get('[data-testid="profile-shell"]').html()).toContain('profile-totp-card')
  })

  it('passes profile support contact info and legacy OIDC provider names to the profile card', async () => {
    fetchPublicSettingsMock.mockResolvedValue({
      contact_info: 'Telegram: @sub2api_support',
      support_qr_codes: [
        { image_url: 'https://cdn.example.com/support.png', note: '客服' },
        { image_url: '   ', note: 'empty url' },
      ],
      balance_low_notify_enabled: false,
      balance_low_notify_threshold: 0,
      linuxdo_oauth_enabled: true,
      dingtalk_oauth_enabled: false,
      wechat_oauth_enabled: true,
      wechat_oauth_open_enabled: true,
      wechat_oauth_mp_enabled: false,
      oidc_oauth_enabled: true,
      oidc_oauth_provider_name: '',
      oidc_connect_provider_name: 'LegacyID',
    } as any)

    const wrapper = mount(ProfileView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          ProfileInfoCard: {
            props: ['contactInfo', 'supportQRCodes', 'oidcProviderName'],
            template: `
              <div
                data-testid="profile-info-card"
                :data-contact="contactInfo"
                :data-oidc="oidcProviderName"
                :data-qr-count="String((supportQRCodes || []).length)"
                :data-first-qr-url="supportQRCodes?.[0]?.image_url || ''"
                :data-second-qr-url="supportQRCodes?.[1]?.image_url || ''"
              />
            `
          },
          ProfileBalanceNotifyCard: { template: '<div data-testid="profile-balance-notify-card" />' },
          ProfilePasswordForm: { template: '<div data-testid="profile-password-form" />' },
          ProfileTotpCard: { template: '<div data-testid="profile-totp-card" />' },
          UserBalanceHistoryModal: true,
        }
      }
    })

    await flushPromises()

    const profileCard = wrapper.get('[data-testid="profile-info-card"]')
    expect(profileCard.attributes('data-contact')).toBe('Telegram: @sub2api_support')
    expect(profileCard.attributes('data-qr-count')).toBe('2')
    expect(profileCard.attributes('data-first-qr-url')).toBe('https://cdn.example.com/support.png')
    expect(profileCard.attributes('data-second-qr-url')).toBe('   ')
    expect(profileCard.attributes('data-oidc')).toBe('LegacyID')
  })
})
