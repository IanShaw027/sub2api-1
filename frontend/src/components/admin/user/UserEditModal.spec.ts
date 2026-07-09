import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import UserEditModal from './UserEditModal.vue'

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      update: vi.fn()
    },
    userAttributes: {
      updateUserAttributeValues: vi.fn()
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const user: AdminUser = {
  id: 7,
  username: 'local-name',
  email: 'user@example.com',
  avatar_url: 'https://cdn.example.com/local-avatar.png',
  auth_bindings: {
    linuxdo: {
      provider: 'linuxdo',
      bound: true,
      display_name: 'LinuxDo Nick',
      avatar_url: 'https://cdn.example.com/linuxdo-avatar.png',
      subject_hint: 'linuxdo-user'
    }
  },
  role: 'user',
  balance: 0,
  concurrency: 2,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-20T00:00:00Z',
  updated_at: '2026-04-20T00:00:00Z',
  notes: ''
}

describe('UserEditModal', () => {
  it('shows the user avatar and third-party login profile details', () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div v-if="show"><slot /><slot name="footer" /></div>'
          },
          UserAttributeForm: true,
          Icon: true
        }
      }
    })

    expect(wrapper.get('[data-test="user-avatar"]').attributes('src')).toBe('https://cdn.example.com/local-avatar.png')
    expect(wrapper.get('[data-test="user-avatar"]').attributes('referrerpolicy')).toBe('no-referrer')
    expect(wrapper.get('[data-test="user-avatar"]').attributes('loading')).toBe('lazy')
    expect(wrapper.text()).toContain('LinuxDo Nick')
    expect(wrapper.text()).toContain('linuxdo-user')
    expect(wrapper.get('[data-test="identity-avatar-linuxdo"]').attributes('src')).toBe('https://cdn.example.com/linuxdo-avatar.png')
    expect(wrapper.get('[data-test="identity-avatar-linuxdo"]').attributes('referrerpolicy')).toBe('no-referrer')
    expect(wrapper.get('[data-test="identity-avatar-linuxdo"]').attributes('loading')).toBe('lazy')
  })

  it('does not render unsafe avatar URLs', () => {
    const wrapper = mount(UserEditModal, {
      props: {
        show: true,
        user: {
          ...user,
          avatar_url: 'javascript:alert(1)',
          auth_bindings: {
            linuxdo: {
              provider: 'linuxdo',
              bound: true,
              display_name: 'LinuxDo Nick',
              avatar_url: 'javascript:alert(1)',
              subject_hint: 'linuxdo-user'
            }
          }
        }
      },
      global: {
        stubs: {
          BaseDialog: {
            props: ['show', 'title'],
            template: '<div v-if="show"><slot /><slot name="footer" /></div>'
          },
          UserAttributeForm: true,
          Icon: true
        }
      }
    })

    expect(wrapper.find('[data-test="user-avatar"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="identity-avatar-linuxdo"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('U')
  })
})
