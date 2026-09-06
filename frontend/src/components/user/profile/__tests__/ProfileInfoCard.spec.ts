import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import type { User } from '@/types'

const authState = vi.hoisted(() => ({ user: null, isSimpleMode: false }))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    fullPath: '/profile'
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        if (key === 'profile.authBindings.providers.email') return 'Email'
        if (key === 'profile.authBindings.providers.linuxdo') return 'LinuxDo'
        if (key === 'profile.authBindings.providers.wechat') return 'WeChat'
        if (key === 'profile.authBindings.providers.oidc') return params?.providerName || 'OIDC'
        if (key === 'profile.authBindings.source.avatar') {
          return `Avatar synced from ${params?.providerName || 'provider'}`
        }
        if (key === 'profile.authBindings.source.username') {
          return `Username synced from ${params?.providerName || 'provider'}`
        }
        if (key === 'profile.authBindings.status.notBound') return 'Not bound'
        return key
      }
    })
  }
})

function createUser(overrides: Partial<User> = {}): User {
  return {
    id: 5,
    username: 'alice',
    email: 'alice@example.com',
    avatar_url: null,
    role: 'user',
    balance: 10,
    concurrency: 2,
    status: 'active',
    allowed_groups: null,
    balance_notify_enabled: true,
    balance_notify_threshold: null,
    balance_notify_extra_emails: [],
    created_at: '2026-04-20T00:00:00Z',
    updated_at: '2026-04-20T00:00:00Z',
    ...overrides
  }
}

describe('ProfileInfoCard', () => {
  it('renders basic account information as setting rows', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser()
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('alice@example.com')
    expect(wrapper.text()).toContain('alice')
  })

  it.each([false, true])('preserves all overview information in simple mode=%s', (simpleMode) => {
    authState.isSimpleMode = simpleMode
    const wrapper = mount(ProfileInfoCard, {
      props: { user: createUser({ balance: 123.45, concurrency: 7, role: 'admin' }) },
      global: { stubs: { Icon: true } }
    })
    expect(wrapper.get('[data-testid="profile-basics-panel"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-overview-metric-balance"]').text()).toContain('$123.45')
    expect(wrapper.get('[data-testid="profile-overview-metric-concurrency"]').text()).toContain('7')
    expect(wrapper.get('[data-testid="profile-overview-metric-member-since"]').text()).toContain(
      new Intl.DateTimeFormat(undefined, { year: 'numeric', month: 'short' }).format(new Date('2026-04-20T00:00:00Z'))
    )
    expect(wrapper.get('[data-testid="profile-overview-role"]').text()).toContain('profile.administrator')
    expect(wrapper.get('[data-testid="profile-overview-status"]').text()).toContain('common.active')
    expect(wrapper.get('[data-testid="profile-overview-hero"]').findAll('.ui-setting-row')).toHaveLength(5)
    expect(wrapper.get('[data-testid="profile-overview-hero"]').find('.glass-card').exists()).toBe(false)
    wrapper.unmount()
    authState.isSimpleMode = false
  })

  it('updates overview values after refreshing the user and handles missing or malformed dates', async () => {
    const wrapper = mount(ProfileInfoCard, {
      props: { user: createUser() },
      global: { stubs: { Icon: true } }
    })
    await wrapper.setProps({
      user: createUser({ balance: 0, concurrency: 0, role: 'user', status: 'disabled', created_at: 'invalid' })
    })
    expect(wrapper.get('[data-testid="profile-overview-metric-balance"]').text()).toContain('$0.00')
    expect(wrapper.get('[data-testid="profile-overview-metric-concurrency"]').text()).toContain('0')
    expect(wrapper.get('[data-testid="profile-overview-metric-member-since"]').text()).toContain('-')
    expect(wrapper.get('[data-testid="profile-overview-role"]').text()).toContain('profile.user')
    expect(wrapper.get('[data-testid="profile-overview-status"]').text()).toContain('common.disabled')
    await wrapper.setProps({ user: createUser({ created_at: '' }) })
    expect(wrapper.get('[data-testid="profile-overview-metric-member-since"]').text()).toContain('-')
    wrapper.unmount()
  })

  it('renders third-party source hints from profile sources', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          avatar_url: 'https://cdn.example.com/linuxdo.png',
          profile_sources: {
            avatar: { provider: 'linuxdo', source: 'linuxdo' },
            username: { provider: 'linuxdo', source: 'linuxdo' }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Avatar synced from LinuxDo')
    expect(wrapper.text()).toContain('Username synced from LinuxDo')
  })

  it('uses the configured OIDC provider name in source hints', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            username: { provider: 'oidc', source: 'oidc' }
          }
        }),
        oidcProviderName: 'ExampleID'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('Username synced from ExampleID')
  })

  it('does not display synthetic oauth-only emails as a real bound email', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          email: 'legacy-user@oidc-connect.invalid',
          email_bound: false,
          auth_bindings: {
            email: { bound: false }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('legacy-user@oidc-connect.invalid')
    expect(wrapper.text()).toContain('Not bound')
  })

  it('does not display synthetic oauth-only emails when only legacy identity bindings mark email as unbound', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          email: 'legacy-user@wechat-connect.invalid',
          identity_bindings: {
            email: { bound: false }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).not.toContain('legacy-user@wechat-connect.invalid')
  })
})
