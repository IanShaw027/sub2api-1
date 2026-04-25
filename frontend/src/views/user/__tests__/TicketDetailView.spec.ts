import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import TicketDetailView from '../TicketDetailView.vue'

const routeState = reactive<{ params: { id: string }; query: { edit?: string } }>({
  params: { id: '42' },
  query: {},
})

const {
  getTicket,
  listTicketMessages,
  replyTicket,
  getAvailable,
  getUserGroupRates,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getTicket: vi.fn(),
  listTicketMessages: vi.fn(),
  replyTicket: vi.fn(),
  getAvailable: vi.fn(),
  getUserGroupRates: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('@/api/tickets', () => ({
  default: {
    getTicket,
    listTicketMessages,
    replyTicket,
    withdrawTicket: vi.fn(),
    closeTicket: vi.fn(),
    updateTicket: vi.fn(),
    submitTicket: vi.fn(),
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
    showSuccess,
  }),
  useAuthStore: () => ({
    user: {
      concurrency: 4,
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

describe('user TicketDetailView', () => {
  beforeEach(() => {
    routeState.params.id = '42'
    routeState.query = {}
    getTicket.mockReset()
    listTicketMessages.mockReset()
    replyTicket.mockReset()
    getAvailable.mockReset()
    getUserGroupRates.mockReset()
    showError.mockReset()
    showSuccess.mockReset()

    getTicket.mockResolvedValue({
      id: 42,
      ticket_no: 'TK-42',
      category: 'consult',
      title: 'Need help',
      status: 'waiting_admin',
      current_form_payload: {},
    })
    listTicketMessages.mockResolvedValue([])
    getAvailable.mockResolvedValue([])
    getUserGroupRates.mockResolvedValue({})
  })

  it('reloads detail data when route id changes', async () => {
    mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: true,
          TicketDetailPane: true,
          TicketEditorCard: true,
        },
      },
    })

    await flushPromises()
    expect(getTicket).toHaveBeenLastCalledWith(42)

    getTicket.mockResolvedValueOnce({
      id: 77,
      ticket_no: 'TK-77',
      category: 'consult',
      title: 'Need help',
      status: 'waiting_admin',
      current_form_payload: {},
    })
    listTicketMessages.mockResolvedValueOnce([])
    routeState.params.id = '77'
    await flushPromises()

    expect(getTicket).toHaveBeenLastCalledWith(77)
  })

  it('refreshes detail after sending a reply', async () => {
    const wrapper = mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: {
            emits: ['reply'],
            template: '<button type="button" class="send-reply" @click="$emit(\'reply\', \'Follow up\')">reply</button>',
          },
          TicketDetailPane: { template: '<div><slot name="actions" /></div>' },
          TicketEditorCard: true,
        },
      },
    })

    await flushPromises()
    replyTicket.mockResolvedValueOnce({ message: 'ok' })
    getTicket.mockResolvedValueOnce({
      id: 42,
      ticket_no: 'TK-42',
      category: 'consult',
      title: 'Need help',
      status: 'waiting_user',
      current_form_payload: {},
    })
    listTicketMessages.mockResolvedValueOnce([])
    const ticketLoadCallsBeforeReply = getTicket.mock.calls.length

    await wrapper.get('.send-reply').trigger('click')
    await flushPromises()

    expect(replyTicket).toHaveBeenCalledWith(42, 'Follow up')
    expect(getTicket.mock.calls.length).toBe(ticketLoadCallsBeforeReply + 1)
  })

  it('hides withdrawn reply and close actions while keeping edit action', async () => {
    getTicket.mockResolvedValueOnce({
      id: 42,
      ticket_no: 'TK-42',
      category: 'consult',
      title: 'Need help',
      status: 'withdrawn',
      current_form_payload: {},
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
          TicketEditorCard: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-test="show-composer"]').text()).toBe('false')
    expect(wrapper.text()).toContain('tickets.actions.edit')
    expect(wrapper.text()).not.toContain('tickets.actions.close')
  })
})
