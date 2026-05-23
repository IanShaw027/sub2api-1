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
  apiPost,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getAdminTicket: vi.fn(),
  listAdminTicketMessages: vi.fn(),
  listAdminTicketReplyTemplates: vi.fn(),
  replaceAdminTicketReplyTemplates: vi.fn(),
  replyAdminTicket: vi.fn(),
  updateAdminTicketStatus: vi.fn(),
  apiPost: vi.fn(),
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

vi.mock('@/api/client', () => ({
  apiClient: {
    post: apiPost,
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

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('admin TicketDetailView reply template menu', () => {
  beforeEach(() => {
    getAdminTicket.mockReset()
    listAdminTicketMessages.mockReset()
    listAdminTicketReplyTemplates.mockReset()
    replaceAdminTicketReplyTemplates.mockReset()
    replyAdminTicket.mockReset()
    updateAdminTicketStatus.mockReset()
    apiPost.mockReset()
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

  it('does not keep a reopened template dialog saving or close it from a stale save response', async () => {
    const saveRequest = createDeferred<void>()
    replaceAdminTicketReplyTemplates.mockReturnValueOnce(saveRequest.promise)

    const wrapper = mount(TicketDetailView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: {
            template: `
              <div>
                <slot name="composer-actions" />
              </div>
            `,
          },
          TicketDetailPane: { template: '<div><slot name="actions" /></div>' },
          TicketReplyTemplatesDialog: {
            props: ['show', 'templates', 'saving'],
            emits: ['close', 'save'],
            template: `
              <div v-if="show" data-test="template-dialog" :data-saving="String(Boolean(saving))">
                <button type="button" class="template-save" :disabled="saving" @click="$emit('save', templates)">save</button>
                <button type="button" class="template-close" @click="$emit('close')">close</button>
              </div>
            `,
          },
        },
      },
    })

    await flushPromises()

    const openTemplateDialog = async () => {
      await wrapper.get('button[aria-haspopup="menu"]').trigger('click')
      await flushPromises()
      const manageButton = wrapper.findAll('button[role="menuitem"]').at(-1)
      expect(manageButton).toBeTruthy()
      await manageButton!.trigger('click')
      await nextTick()
    }

    await openTemplateDialog()
    await wrapper.get('[data-test="template-dialog"] .template-save').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="template-dialog"]').attributes('data-saving')).toBe('true')

    await wrapper.get('[data-test="template-dialog"] .template-close').trigger('click')
    await nextTick()

    await openTemplateDialog()

    expect(wrapper.get('[data-test="template-dialog"]').attributes('data-saving')).toBe('false')
    expect(wrapper.get('[data-test="template-dialog"] .template-save').attributes('disabled')).toBeUndefined()

    saveRequest.resolve()
    await flushPromises()

    expect(wrapper.get('[data-test="template-dialog"]').attributes('data-saving')).toBe('false')
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

    expect(replyAdminTicket).toHaveBeenCalledWith(42, 'Need update', undefined)
    expect(getAdminTicket).toHaveBeenCalledTimes(2)
  })

  it('clears the admin reply draft when the route ticket changes', async () => {
    const wrapper = mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: {
            props: ['replyContent'],
            emits: ['update:replyContent'],
            template: `
              <div>
                <button type="button" class="set-draft" @click="$emit('update:replyContent', 'draft from old ticket')">set</button>
                <div data-test="reply-draft">{{ replyContent }}</div>
              </div>
            `,
          },
          TicketDetailPane: { template: '<div><slot name="actions" /></div>' },
          TicketReplyTemplatesDialog: true,
        },
      },
    })

    await flushPromises()
    await wrapper.get('.set-draft').trigger('click')
    expect(wrapper.get('[data-test="reply-draft"]').text()).toBe('draft from old ticket')

    getAdminTicket.mockResolvedValueOnce({
      id: 108,
      ticket_no: 'TK-108',
      status: 'waiting_admin',
    })
    listAdminTicketMessages.mockResolvedValueOnce([])
    routeState.params.id = '108'

    await flushPromises()

    expect(wrapper.get('[data-test="reply-draft"]').text()).toBe('')
  })

  it('does not surface stale upload errors after switching to another ticket', async () => {
    let rejectUpload!: (reason?: unknown) => void
    apiPost.mockReturnValueOnce(new Promise((_, reject) => {
      rejectUpload = reject
    }))

    const wrapper = mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketDetailPane: true,
          TicketReplyTemplatesDialog: true,
        },
      },
    })

    await flushPromises()

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['image'], 'ticket.png', { type: 'image/png' })],
      configurable: true,
    })

    const uploadPromise = fileInput.trigger('change')
    await Promise.resolve()

    getAdminTicket.mockResolvedValueOnce({
      id: 108,
      ticket_no: 'TK-108',
      status: 'waiting_admin',
    })
    listAdminTicketMessages.mockResolvedValueOnce([])
    routeState.params.id = '108'
    await flushPromises()

    rejectUpload(new Error('stale admin upload failed'))
    await uploadPromise
    await flushPromises()

    expect(showError).not.toHaveBeenCalledWith('tickets.uploadFailed')
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

  it('hides admin status actions that the backend would reject', async () => {
    getAdminTicket.mockResolvedValueOnce({
      id: 42,
      ticket_no: 'TK-42',
      status: 'submitted',
      last_reply_role: 'system',
    })

    const wrapper = mount(TicketDetailView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TicketConversationPane: true,
          TicketDetailPane: { template: '<div><slot name="actions" /></div>' },
          TicketReplyTemplatesDialog: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('tickets.statuses.processing')
    expect(wrapper.text()).toContain('tickets.statuses.waiting_admin')
    expect(wrapper.text()).toContain('tickets.statuses.closed')
    expect(wrapper.text()).not.toContain('tickets.statuses.waiting_user')
    expect(wrapper.text()).not.toContain('tickets.statuses.resolved')
  })
})
