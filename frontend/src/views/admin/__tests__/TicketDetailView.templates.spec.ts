import { defineComponent, nextTick } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import TicketDetailView from '../TicketDetailView.vue'

const mocks = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  listTemplates: vi.fn(),
  createTemplate: vi.fn(),
  updateTemplate: vi.fn(),
  deleteTemplate: vi.fn(),
}))

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRoute: () => ({ params: { id: '1' } }),
  useRouter: () => ({ back: vi.fn(), push: vi.fn() }),
}))
vi.mock('@/stores', () => ({ useAppStore: () => mocks }))
vi.mock('@/utils/ticketForm', () => ({ notifyTicketUnreadChanged: vi.fn() }))
vi.mock('@/api/admin/tickets', () => ({
  adminTicketsAPI: {
    ...mocks,
    get: vi.fn().mockResolvedValue({ data: { id: 1, ticket_no: 'T-1', status: 'processing', category: 'other' } }),
    messages: vi.fn().mockResolvedValue({ data: [] }),
  },
}))

const templates = [
  { id: 1, title: 'First reply', content: 'First visible template body', sort_order: 0 },
  { id: 2, title: 'Second reply', content: 'Second visible template body', sort_order: 1 },
]

const conversation = defineComponent({
  name: 'TicketConversationPane',
  props: ['replyContent'],
  template: '<section><output>{{ replyContent }}</output><slot name="composer-actions" /></section>',
})
const dialog = defineComponent({
  name: 'TicketReplyTemplatesDialog',
  props: ['show', 'templates', 'saving'],
  emits: ['save', 'close'],
  template: '<div v-if="show" />',
})

function render() {
  return mount(TicketDetailView, {
    attachTo: document.body,
    global: {
      stubs: {
        AppLayout: { template: '<main><slot /></main>' },
        DetailPageLayout: { template: '<div><slot name="main" /><slot name="side" /></div>' },
        PageHeader: true,
        GlassCard: true,
        TicketConversationPane: conversation,
        TicketDetailPane: true,
        TicketReplyTemplatesDialog: dialog,
      },
    },
  })
}

describe('TicketDetailView reply template fidelity', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listTemplates.mockResolvedValue({ data: templates })
  })
  afterEach(() => { document.body.innerHTML = '' })

  it('renders template body previews and applies each real v-for button without losing focus', async () => {
    const wrapper = render()
    await flushPromises()
    const buttons = wrapper.findAll('.template-option')
    expect(buttons).toHaveLength(2)
    for (const [index, button] of buttons.entries()) {
      expect(button.get('.template-option-preview').text()).toBe(templates[index].content)
      expect(button.get('.template-option-preview').isVisible()).toBe(true)
      await button.trigger('click')
      await flushPromises()
      expect(wrapper.get('output').text()).toBe(templates[index].content)
      expect(document.activeElement).toBe(button.element)
    }
    expect(mocks.showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('restores focus to the unique manage button after save even when all templates are deleted', async () => {
    const wrapper = render()
    await flushPromises()
    const manage = wrapper.get('.template-pill')
    await manage.trigger('click')
    const editor = wrapper.getComponent(dialog)
    expect(editor.props('show')).toBe(true)
    mocks.listTemplates.mockResolvedValueOnce({ data: [] })
    editor.vm.$emit('save', [])
    await flushPromises()
    await nextTick()
    expect(mocks.deleteTemplate.mock.calls).toEqual([[1], [2]])
    expect(editor.props('show')).toBe(false)
    expect(document.activeElement).toBe(manage.element)
    expect(wrapper.findAll('.template-option')).toHaveLength(0)
    expect(wrapper.get('.template-empty').text()).toBe('tickets.templates.empty')
    expect(mocks.showSuccess).toHaveBeenCalledWith('common.saved')
    expect(mocks.showError).not.toHaveBeenCalled()
    wrapper.unmount()
  })
})
