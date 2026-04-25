import { flushPromises, mount } from '@vue/test-utils'
import { nextTick, reactive } from 'vue'
import { describe, expect, it, vi, beforeEach } from 'vitest'

import TicketDetailView from '../TicketDetailView.vue'

const routeState = reactive<{ params: { id: string } }>({
  params: { id: '42' },
})

const {
  getAdminTicket,
  listAdminTicketMessages,
  listAdminTicketReplyTemplates,
  replaceAdminTicketReplyTemplates,
  replyAdminTicket,
  updateAdminTicketStatus,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getAdminTicket: vi.fn(),
  listAdminTicketMessages: vi.fn(),
  listAdminTicketReplyTemplates: vi.fn(),
  replaceAdminTicketReplyTemplates: vi.fn(),
  replyAdminTicket: vi.fn(),
  updateAdminTicketStatus: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/adminTickets', () => ({
  default: {
    getAdminTicket,
    listAdminTicketMessages,
    listAdminTicketReplyTemplates,
    replaceAdminTicketReplyTemplates,
    replyAdminTicket,
    updateAdminTicketStatus,
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
  useRoute: () => routeState,
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
    replyAdminTicket.mockReset()
    updateAdminTicketStatus.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    routeState.params.id = '42'

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

  it('reloads detail data when route id changes', async () => {
    mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: true,
          TicketDetailPane: true,
          TicketReplyTemplatesDialog: true,
        },
      },
    })

    await flushPromises()
    expect(getAdminTicket).toHaveBeenLastCalledWith(42)

    getAdminTicket.mockResolvedValueOnce({
      id: 108,
      ticket_no: 'TK-108',
      status: 'waiting_admin',
    })
    routeState.params.id = '108'
    await flushPromises()

    expect(getAdminTicket).toHaveBeenLastCalledWith(108)
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

  it('refreshes ticket detail after sending a reply', async () => {
    const wrapper = mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: {
            emits: ['reply'],
            template: '<button type="button" class="send-reply" @click="$emit(\'reply\', \'Need update\')">reply</button>',
          },
          TicketDetailPane: { template: '<div><slot name="actions" /></div>' },
          TicketReplyTemplatesDialog: true,
        },
      },
    })

    await flushPromises()
    replyAdminTicket.mockResolvedValueOnce({ message: 'ok' })
    getAdminTicket.mockResolvedValueOnce({
      id: 42,
      ticket_no: 'TK-42',
      status: 'waiting_user',
    })
    listAdminTicketMessages.mockResolvedValueOnce([])

    await wrapper.get('.send-reply').trigger('click')
    await flushPromises()

    expect(replyAdminTicket).toHaveBeenCalledWith(42, 'Need update')
    expect(getAdminTicket).toHaveBeenCalledTimes(2)
  })

  it('hides withdrawn reply and status update actions', async () => {
    getAdminTicket.mockResolvedValueOnce({
      id: 42,
      ticket_no: 'TK-42',
      status: 'withdrawn',
    })

    const wrapper = mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: {
            props: ['showComposer'],
            template: '<div data-test="show-composer">{{ String(showComposer) }}</div>',
          },
          TicketDetailPane: { template: '<div><slot name="actions" /></div>' },
          TicketReplyTemplatesDialog: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-test="show-composer"]').text()).toBe('false')
    expect(wrapper.text()).not.toContain('tickets.adminActions')
    expect(wrapper.text()).not.toContain('tickets.statuses.processing')
  })
})
