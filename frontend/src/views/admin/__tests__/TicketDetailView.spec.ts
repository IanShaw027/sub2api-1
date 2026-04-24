import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { describe, expect, it, vi, beforeEach } from 'vitest'

import TicketDetailView from '../TicketDetailView.vue'

const { getAdminTicket, listAdminTicketMessages, listAdminTicketReplyTemplates, replaceAdminTicketReplyTemplates, showError, showSuccess } = vi.hoisted(() => ({
  getAdminTicket: vi.fn(),
  listAdminTicketMessages: vi.fn(),
  listAdminTicketReplyTemplates: vi.fn(),
  replaceAdminTicketReplyTemplates: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/adminTickets', () => ({
  default: {
    getAdminTicket,
    listAdminTicketMessages,
    listAdminTicketReplyTemplates,
    replaceAdminTicketReplyTemplates,
    replyAdminTicket: vi.fn(),
    updateAdminTicketStatus: vi.fn(),
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
  useAuthStore: () => ({
    user: {
      id: 1,
      username: 'admin',
      email: 'admin@example.com',
      avatar_url: '',
    },
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({
    params: {
      id: '42',
    },
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

describe('admin TicketDetailView reply template menu', () => {
  beforeEach(() => {
    getAdminTicket.mockReset()
    listAdminTicketMessages.mockReset()
    listAdminTicketReplyTemplates.mockReset()
    replaceAdminTicketReplyTemplates.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getAdminTicket.mockResolvedValue({
      id: 42,
      ticket_no: 'TK-42',
      status: 'waiting_admin',
    })
    listAdminTicketMessages.mockResolvedValue([])
    listAdminTicketReplyTemplates.mockResolvedValue([
      {
        id: 'tpl-1',
        title: 'Greeting',
        content: 'template reply',
      },
    ])
  })

  it('keeps the template menu keyboard reachable for selecting and managing templates', async () => {
    const wrapper = mount(TicketDetailView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: {
            props: ['replyContent'],
            emits: ['update:replyContent', 'reply'],
            template: `
              <div>
                <div data-test="reply-draft">{{ replyContent }}</div>
                <slot name="composer-actions" />
              </div>
            `,
          },
          TicketDetailPane: { template: '<div><slot name="actions" /></div>' },
          TicketReplyTemplatesDialog: {
            props: ['show'],
            template: '<div data-test="template-dialog" :data-open="String(show)" />',
          },
        },
      },
    })

    await flushPromises()

    const trigger = wrapper.get('button[aria-haspopup="menu"]')
    await trigger.trigger('keydown.enter')
    await flushPromises()
    await nextTick()

    const templateItem = wrapper.get('button[role="menuitem"]')
    expect(document.activeElement).toBe(templateItem.element)

    await templateItem.trigger('click')
    await nextTick()
    expect(wrapper.get('[data-test="reply-draft"]').text()).toBe('template reply')

    await trigger.trigger('keydown.enter')
    await flushPromises()
    await nextTick()

    const manageButton = wrapper.findAll('button[role="menuitem"]').at(-1)
    expect(manageButton).toBeTruthy()
    manageButton!.element.focus()
    await manageButton!.trigger('click')
    await nextTick()

    expect(wrapper.get('[data-test="template-dialog"]').attributes('data-open')).toBe('true')
  })
})
