import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import type { User } from '@/types'

vi.mock('vue-router', () => ({
  useRoute: () => ({
    fullPath: '/profile'
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: null
  })
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
      t: (key: string, _params?: Record<string, string>) => {
        if (key === 'profile.accountBalance') return 'Account Balance'
        if (key === 'profile.concurrencyLimit') return 'Concurrency Limit'
        if (key === 'profile.memberSince') return 'Member Since'
        if (key === 'profile.administrator') return 'Administrator'
        if (key === 'profile.user') return 'User'
        if (key === 'profile.authBindings.providers.linuxdo') return 'LinuxDo'
        if (key === 'profile.authBindings.providers.oidc') return _params?.providerName ?? 'OIDC'
        if (key === 'profile.authBindings.providers.dingtalk') return '钉钉'
        if (key === 'profile.authBindings.providers.github') return 'GitHub'
        if (key === 'profile.authBindings.providers.google') return 'Google'
        if (key === 'profile.identity.source.avatar') return `Avatar from ${_params?.providerName}`
        if (key === 'profile.identity.source.username') return `Nickname from ${_params?.providerName}`
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
  it('renders basic account information inside the new overview shell', () => {
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
    expect(wrapper.text()).toContain('User')
    expect(wrapper.get('[data-testid="profile-basics-panel"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-auth-bindings-panel"]').exists()).toBe(true)
  })

  it('renders trusted avatars with privacy-preserving attributes and hides unsafe ones', () => {
    const safeWrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({ avatar_url: 'https://cdn.example.com/avatar.png' })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const safeAvatar = safeWrapper.get('img[alt="alice"]')
    expect(safeAvatar.attributes('src')).toBe('https://cdn.example.com/avatar.png')
    expect(safeAvatar.attributes('referrerpolicy')).toBe('no-referrer')
    expect(safeAvatar.attributes('loading')).toBe('lazy')

    const unsafeWrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({ avatar_url: 'javascript:alert(1)' })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(unsafeWrapper.find('img[alt="alice"]').exists()).toBe(false)
    expect(unsafeWrapper.text()).toContain('A')
  })

  it('renders compact profile source hints without restoring the old side column', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            avatar: { provider: 'linuxdo', source: 'linuxdo' },
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

    expect(wrapper.get('[data-testid="profile-source-hints"]').text()).toContain('Avatar from LinuxDo')
    expect(wrapper.get('[data-testid="profile-source-hints"]').text()).toContain('Nickname from ExampleID')
    expect(wrapper.find('[data-testid="profile-side-column"]').exists()).toBe(false)
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

  it('suppresses legacy string email source chips', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            avatar: 'email'
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('[data-testid="profile-source-hints"]').exists()).toBe(false)
  })

  it('renders the approved overview hero and two-column content shell', () => {
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

    expect(wrapper.get('[data-testid="profile-overview-hero"]').text()).toContain('alice@example.com')
    expect(wrapper.get('[data-testid="profile-overview-metric-balance"]').text()).toContain('Account Balance')
    expect(wrapper.get('[data-testid="profile-overview-metric-concurrency"]').text()).toContain('Concurrency Limit')
    expect(wrapper.get('[data-testid="profile-overview-metric-member-since"]').text()).toContain('Member Since')
    expect(wrapper.find('[data-testid="profile-info-summary-grid"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="profile-main-column"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="profile-side-column"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="profile-basics-panel"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="profile-auth-bindings-panel"]').exists()).toBe(true)
  })

  it('passes the DingTalk availability flag through to the identity bindings section', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser(),
        dingtalkEnabled: true,
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(
      wrapper.getComponent(ProfileIdentityBindingsSection).props('dingtalkEnabled'),
    ).toBe(true)
  })

  it('renders support contact info and accessible qr codes when provided', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser(),
        contactInfo: 'Telegram: @sub2api_support',
        supportQRCodes: [{ image_url: 'https://cdn.example.com/support.png', note: '客服' }],
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.get('[data-testid="profile-support-panel"]').text()).toContain('Telegram: @sub2api_support')
    expect(wrapper.get('[data-testid="profile-support-qr-grid"]').html()).toContain('https://cdn.example.com/support.png')
    expect(wrapper.get('[data-testid="profile-support-qr-grid"]').text()).toContain('客服')
    expect(wrapper.get('[data-testid="profile-support-qr-grid"] img').attributes('alt')).toContain('QR')
    expect(wrapper.get('[data-testid="profile-support-qr-grid"] img').attributes('aria-label')).toContain('QR')
  })

  it('maps legacy oidc source aliases to the configured provider label', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          username_source: 'oidc_connect'
        }),
        oidcProviderName: 'LegacyID'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-testid="profile-source-hints"]').text()).toContain('Nickname from LegacyID')
  })

  it('maps legacy oidc: and oidc/ source aliases to the configured provider label', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            avatar: 'oidc:',
            username: 'oidc/'
          }
        }),
        oidcProviderName: 'LegacyID'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const hints = wrapper.get('[data-testid="profile-source-hints"]').text()
    expect(hints).toContain('Avatar from LegacyID')
    expect(hints).toContain('Nickname from LegacyID')
    expect(hints).not.toContain('oidc:')
    expect(hints).not.toContain('oidc/')
  })

  it('uses the localized DingTalk provider label in source hints', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            avatar: 'dingtalk'
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-testid="profile-source-hints"]').text()).toContain('钉钉')
    expect(wrapper.get('[data-testid="profile-source-hints"]').text()).not.toContain('DingTalk')
  })

  it('maps legacy oidc source labels without surfacing raw aliases', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            username: { provider: '', source: 'oidc_connect', provider_label: 'oidc_connect' },
            avatar: { provider: '', source: 'oidc-connect', label: 'oidc-connect' },
          }
        }),
        oidcProviderName: 'LegacyID'
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const hints = wrapper.get('[data-testid="profile-source-hints"]').text()
    expect(hints).toContain('Avatar from LegacyID')
    expect(hints).toContain('Nickname from LegacyID')
    expect(hints).not.toContain('oidc_connect')
    expect(hints).not.toContain('oidc-connect')
  })

  it('keeps explicit legacy aliases ahead of normalized provider labels', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          profile_sources: {
            username: {
              provider: 'github',
              source: 'github',
              provider_label: 'Legacy GitHub',
            },
          },
        }),
      },
      global: {
        stubs: {
          Icon: true,
        },
      },
    })

    expect(wrapper.get('[data-testid="profile-source-hints"]').text()).toContain('Nickname from Legacy GitHub')
    expect(wrapper.get('[data-testid="profile-source-hints"]').text()).not.toContain('Nickname from GitHub')
  })

  it('does not render raw non-provider source sentinels as profile hints', () => {
    const wrapper = mount(ProfileInfoCard, {
      props: {
        user: createUser({
          avatar_source: 'remote_url',
          username_source: { provider: '', source: 'media', label: 'media' }
        }),
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.find('[data-testid="profile-source-hints"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('remote_url')
    expect(wrapper.text()).not.toContain('media')
  })
})
