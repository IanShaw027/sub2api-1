import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import TicketCreateView from '../TicketCreateView.vue'

const { getAvailable, getUserGroupRates, showError } = vi.hoisted(() => ({
  getAvailable: vi.fn(),
  getUserGroupRates: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/tickets', () => ({
  default: {
    createTicket: vi.fn(),
  },
}))

vi.mock('@/api/groups', () => ({
  default: {
    getAvailable,
    getUserGroupRates,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
  }),
  useAuthStore: () => ({
    user: {
      concurrency: 7,
    },
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    replace: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('TicketCreateView', () => {
  beforeEach(() => {
    getAvailable.mockReset()
    getUserGroupRates.mockReset()
    showError.mockReset()

    getAvailable.mockResolvedValue([
      {
        id: 1,
        name: 'Standard Group',
        subscription_type: 'standard',
        rate_multiplier: 1.3,
      },
      {
        id: 2,
        name: 'Subscription Group',
        subscription_type: 'subscription',
        rate_multiplier: 1.8,
      },
    ])
    getUserGroupRates.mockResolvedValue({
      1: 1.1,
      2: 0.9,
    })
  })

  it('loads ticket context and passes required editor props on standalone create page', async () => {
    const wrapper = mount(TicketCreateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: {
            props: ['messages'],
            template: '<div data-test="sender-name">{{ messages?.[0]?.sender_name_snapshot }}</div>',
          },
          TicketEditorCard: {
            props: ['userConcurrency', 'availableGroups', 'userGroupRates'],
            template: `
              <div
                data-test="editor-card"
                :data-user-concurrency="String(userConcurrency)"
                :data-available-groups="JSON.stringify(availableGroups)"
                :data-user-group-rates="JSON.stringify(userGroupRates)"
              />
            `,
          },
        },
      },
    })

    await flushPromises()

    const editor = wrapper.get('[data-test="editor-card"]')
    expect(editor.attributes('data-user-concurrency')).toBe('7')
    expect(editor.attributes('data-available-groups')).toContain('"id":1')
    expect(editor.attributes('data-available-groups')).not.toContain('"id":2')
    expect(editor.attributes('data-user-group-rates')).toContain('"1":1.1')

    expect(wrapper.get('[data-test="sender-name"]').text()).toBe('admin.ops.system')
  })
})
