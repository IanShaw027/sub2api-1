import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import type { LocalChatMessage, LocalChatSession } from '../localChat'
import ChatWorkspace from '../components/ChatWorkspace.vue'
import LocalChatMessageView from '../components/LocalChatMessage.vue'
import LocalChatComposer from '../components/LocalChatComposer.vue'
import UiSelect from '@/components/ui/UiSelect.vue'

const state = vi.hoisted(() => ({ store: null as unknown, auth: null as unknown, mobile: false }))
vi.mock('../stores/chatWorkspace', () => ({ useChatWorkspace: () => state.store }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => state.auth }))
vi.mock('@/composables/useIsMobile', async () => {
  const { ref } = await import('vue')
  return { useIsMobile: () => ({ isMobile: ref(state.mobile) }) }
})
vi.mock('vue-i18n', async (original) => {
  const actual = await original<typeof import('vue-i18n')>()
  const { ref } = await import('vue')
  return { ...actual, useI18n: (options?: { messages?: { en?: Record<string, unknown> } }) => ({
    locale: ref('en'), t: (key: string) => typeof options?.messages?.en?.[key] === 'string' ? options.messages.en[key] : key,
  }) }
})

function session(id = 'one', title = 'Project planning'): LocalChatSession {
  return { id, title, userId: 1, groupId: 1, model: 'gpt-4o', platform: 'openai', settings: {}, createdAt: '2026-09-06T00:00:00Z', updatedAt: '2026-09-06T00:00:00Z', messages: [], draft: { text: '', files: [] }, persisted: true }
}
function message(role: LocalChatMessage['role'], overrides: Partial<LocalChatMessage> = {}): LocalChatMessage {
  return { id: role, role, content: `${role} content`, files: [], createdAt: '2026-09-06T00:00:00Z', status: 'completed', ...overrides }
}
function createStore() {
  return reactive({
    sessions: [session(), session('two', 'Translate a letter')], selectedSessionId: 'one' as string | null,
    get selectedSession(): LocalChatSession | null { return this.sessions.find(item => item.id === this.selectedSessionId) || null },
    get messages(): LocalChatMessage[] { return this.selectedSession?.messages || [] },
    get settings() { return this.selectedSession?.settings || {} },
    groups: [{ id: 1, name: 'My group', platform: 'openai' }], groupId: 1, model: 'gpt-4o', models: ['gpt-4o'], modelDetails: {},
    loading: false, modelsLoading: false, streaming: false, error: null, initialized: true,
    init: vi.fn(), loadDraft: vi.fn(), saveDraft: vi.fn(), send: vi.fn(), stop: vi.fn(), create: vi.fn(), select: vi.fn(),
    rename: vi.fn(), deleteSession: vi.fn(), setParameters: vi.fn(), vote: vi.fn(), editAndRegenerate: vi.fn(), regenerate: vi.fn(), saveSession: vi.fn(), clearError: vi.fn(),
  })
}
const stubs = {
  UiSelect: true, MessageContent: { props: ['content'], template: '<div>{{ content }}</div>' },
  LocalChatChart: { props: ['data'], template: '<div class="chart-result" />' }, LegacyChatHistory: true,
  UiDrawer: { props: ['open'], template: '<div v-if="open" role="dialog"><slot /></div>' },
  UiModal: { props: ['open'], template: '<div v-if="open" role="dialog"><slot /><slot name="footer" /></div>' },
}
let store: ReturnType<typeof createStore>
let auth: { user: { id: number } | null; isAuthenticated: boolean }
let wrapper: VueWrapper | undefined
const clipboard = vi.fn()
beforeEach(() => {
  vi.clearAllMocks()
  store = createStore(); state.store = store; state.mobile = false
  auth = reactive({ user: { id: 1 }, isAuthenticated: true }); state.auth = auth
  store.loadDraft.mockImplementation(async (id: string) => store.sessions.find(item => item.id === id)?.draft)
  store.saveDraft.mockResolvedValue(undefined)
  store.send.mockResolvedValue(undefined)
  vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: vi.fn(() => 'blob:attachment'), revokeObjectURL: vi.fn() }))
  Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText: clipboard.mockResolvedValue(undefined) } })
})
afterEach(() => { wrapper?.unmount(); wrapper = undefined; vi.restoreAllMocks(); vi.unstubAllGlobals(); document.body.innerHTML = '' })
async function render() {
  wrapper = mount(ChatWorkspace, { attachTo: document.body, global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en: {} } })], stubs } })
  await flushPromises()
  return wrapper
}

describe('local chat workspace UI', () => {
  it('searches local history without initializing cloud state and sends quick prompts only on submit', async () => {
    const page = await render()
    expect(store.init).not.toHaveBeenCalled()
    expect(page.text()).toContain('Conversation saved on this device')
    expect(page.findAll('.local-chat-session-title')).toHaveLength(2)
    await page.get('input[type="search"]').setValue('translate')
    expect(page.findAll('.local-chat-session-title').map(item => item.text())).toEqual(['Translate a letter'])
    await page.get('.chat-suggestions button').trigger('click')
    expect(store.send).not.toHaveBeenCalled()
    expect(document.activeElement).toBe(page.get('textarea').element)
    await page.get('[aria-label="Send message"]').trigger('click')
    expect(store.send).toHaveBeenCalledWith({ text: expect.stringContaining('Polish the following text'), files: [] })
  })

  it('supports file selection, paste, drop, removal and sends real files', async () => {
    const page = await render()
    const image = new File(['png'], 'one.png', { type: 'image/png' })
    const text = new File(['notes'], 'notes.txt', { type: 'text/plain' })
    const pdf = new File(['pdf'], 'report.pdf', { type: 'application/pdf' })
    const input = page.get('input[type="file"]')
    Object.defineProperty(input.element, 'files', { configurable: true, value: [image] })
    await input.trigger('change')
    await page.get('textarea').trigger('paste', { clipboardData: { files: [text] } })
    await page.get('.chat-workspace').trigger('drop', { dataTransfer: { files: [pdf], types: ['Files'] } })
    expect(page.getComponent(LocalChatComposer).props('files')).toEqual([image, text, pdf])
    await page.findAll('[aria-label^="Remove attachment:"]')[1]!.trigger('click')
    await page.get('[aria-label="Send message"]').trigger('click')
    expect(store.send).toHaveBeenCalledWith({ text: '', files: [image, pdf] })
    expect(URL.revokeObjectURL).toHaveBeenCalled()
  })

  it('does not submit during IME composition or Shift+Enter, and preserves text on startup failure', async () => {
    const page = await render()
    await page.get('textarea').setValue('draft')
    await page.get('textarea').trigger('compositionstart')
    await page.get('textarea').trigger('keydown', { key: 'Enter' })
    await page.get('textarea').trigger('compositionend')
    await page.get('textarea').trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(store.send).not.toHaveBeenCalled()
    store.send.mockRejectedValue(new Error('Storage unavailable'))
    await page.get('textarea').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect((page.get('textarea').element as HTMLTextAreaElement).value).toBe('draft')
    expect(page.get('[role="alert"]').text()).toContain('Storage unavailable')
  })

  it('stops the actual stream without a second generation request', async () => {
    store.streaming = true
    const page = await render()
    await page.get('[aria-label="Stop generation"]').trigger('click')
    expect(store.stop).toHaveBeenCalledOnce()
    expect(store.send).not.toHaveBeenCalled()
    expect(store.regenerate).not.toHaveBeenCalled()
  })

  it('copies, branches user edits preserving attachments, regenerates and stores local feedback', async () => {
    const file = new File(['note'], 'note.txt', { type: 'text/plain' })
    store.sessions[0]!.messages = [message('user', { files: [file] }), message('assistant')]
    const page = await render()
    await page.get('.is-assistant [aria-label="Copy"]').trigger('click')
    expect(clipboard).toHaveBeenCalledWith('assistant content')
    await page.get('[aria-label="Edit message"]').trigger('click')
    await page.get('textarea[aria-label="Edit message"]').setValue('Revised prompt')
    await page.get('.local-chat-message-edit').findAll('button')[1]!.trigger('click')
    await flushPromises()
    expect(store.editAndRegenerate).toHaveBeenCalledWith('user', 'Revised prompt')
    expect(store.messages[0]!.files).toEqual([file])
    await page.get('[aria-label="Generate again"]').trigger('click')
    await flushPromises()
    expect(store.regenerate).toHaveBeenCalledWith('assistant')
    await page.get('[aria-label="Helpful response"]').trigger('click')
    expect(store.vote).toHaveBeenCalledWith('assistant', 'up')
  })

  it('requires rename and delete confirmation and targets the requested local session', async () => {
    const page = await render()
    await page.findAll('[aria-label="Conversation actions"]')[1]!.trigger('click')
    await page.findAll('.local-chat-session-menu button')[0]!.trigger('click')
    expect(store.rename).not.toHaveBeenCalled()
    await page.get('[aria-label="Conversation name"]').setValue('Renamed chat')
    await page.get('[aria-label="Conversation name"]').trigger('keydown', { key: 'Enter' })
    await flushPromises()
    expect(store.rename).toHaveBeenCalledWith('two', 'Renamed chat')
    await page.findAll('[aria-label="Conversation actions"]')[1]!.trigger('click')
    await page.get('.local-chat-session-menu .danger').trigger('click')
    expect(store.deleteSession).not.toHaveBeenCalled()
    const confirm = page.get('[role="dialog"]').findAll('button').find(button => button.text() === 'Delete conversation')!
    await confirm.trigger('click')
    expect(store.deleteSession).toHaveBeenCalledWith('two')
  })

  it('opens mobile history and persists the outgoing draft before selection', async () => {
    state.mobile = true
    const page = await render()
    await page.get('textarea').setValue('Unsent local draft')
    await page.get('[aria-label="Chat history"]').trigger('click')
    expect(page.find('.chat-history').exists()).toBe(false)
    await page.findAll('.local-chat-session-select')[1]!.trigger('click')
    await flushPromises()
    expect(store.saveDraft).toHaveBeenCalledWith({ text: 'Unsent local draft', files: [] }, 'one')
    expect(store.select).toHaveBeenCalledWith('two')
    expect(page.find('[role="dialog"]').exists()).toBe(false)
  })

  it('cancels an old edit awaiting draft persistence when the selected session changes', async () => {
    store.sessions[0]!.messages = [message('user')]
    const page = await render()
    let finish!: () => void
    store.saveDraft.mockImplementationOnce(() => new Promise<void>(resolve => { finish = resolve }))
    page.getComponent(LocalChatMessageView).vm.$emit('action', 'edit', store.messages[0], 'new text')
    store.selectedSessionId = 'two'
    finish()
    await flushPromises()
    expect(store.editAndRegenerate).not.toHaveBeenCalled()
    expect((page.get('textarea').element as HTMLTextAreaElement).value).toBe('')
  })

  it('never displays another account draft after a pending load resolves', async () => {
    let finish!: (draft: { text: string; files: File[] }) => void
    store.loadDraft.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const page = await render()
    auth.user = { id: 2 }
    store.selectedSessionId = null
    store.sessions = []
    finish({ text: 'Private old account draft', files: [] })
    await flushPromises()
    expect((page.get('textarea').element as HTMLTextAreaElement).value).toBe('')
    expect(store.saveDraft).not.toHaveBeenCalled()
  })

  it('renders actual stopped reasoning, sources and chart results without inventing them', async () => {
    store.sessions[0]!.messages = [message('assistant', { status: 'stopped', reasoning: 'Real reasoning', sources: [{ url: 'https://example.com/source' }, { url: 'javascript:alert(1)' }], toolResults: [{ toolCallId: 'chart-1', name: 'display_chart', content: 'chart', data: { type: 'chart', chart: { title: 'Actual chart' } } }] })]
    const page = await render()
    expect(page.text()).toContain('Stopped')
    expect(page.get('.local-chat-reasoning').text()).toContain('Real reasoning')
    expect(page.findAll('.local-chat-sources a')).toHaveLength(1)
    expect(page.get('.local-chat-sources a').text()).toContain('example.com')
    expect(page.findAll('.chart-result')).toHaveLength(1)
    store.sessions[0]!.messages = [message('assistant')]
    await flushPromises()
    expect(page.find('.local-chat-reasoning').exists()).toBe(false)
    expect(page.find('.chart-result').exists()).toBe(false)
  })

  it('rejects invalid generation parameters before saving', async () => {
    const page = await render()
    await page.get('[aria-label="Model settings"]').trigger('click')
    await page.get('input[type="number"][min="0"]').setValue('3')
    const save = page.get('[role="dialog"]').findAll('button').find(button => button.text() === 'Save')!
    await save.trigger('click')
    expect(store.setParameters).not.toHaveBeenCalled()
    expect(page.get('[role="dialog"]').text()).toContain('Temperature must be between 0 and 2')
  })

  it.each([
    ['gemini-3-flash-preview', 'gemini', ['minimal', 'low', 'medium', 'high']],
    ['gemini-3-pro-preview', 'antigravity', ['low', 'high']],
    ['gemini-3.1-pro-preview', 'gemini', ['low', 'medium', 'high']],
    ['gemini-2.5-flash', 'gemini', ['none', 'low', 'medium', 'high']],
    ['gemini-2.5-pro', 'gemini', ['low', 'medium', 'high']],
    ['claude-opus-4-6', 'anthropic', ['low', 'medium', 'high', 'max']],
    ['claude-opus-4-6-thinking', 'antigravity', ['low', 'medium', 'high']],
  ])('makes %s reasoning selectable on %s using a real id-only model response', async (model, platform, efforts) => {
    store.model = model as string
    store.models = [model as string]
    store.sessions[0]!.model = model as string
    store.sessions[0]!.platform = platform as LocalChatSession['platform']
    const page = await render()
    await page.get('[aria-label="Model settings"]').trigger('click')
    const selector = page.findAllComponents(UiSelect).find(component => component.attributes('aria-label') === 'Reasoning effort')!
    expect(selector.props('options').map(option => option.value)).toEqual(['', ...efforts])
    const effort = efforts.at(-1)!
    selector.vm.$emit('update:modelValue', effort)
    await flushPromises()
    const save = page.get('[role="dialog"]').findAll('button').find(button => button.text() === 'Save')!
    await save.trigger('click')
    expect(store.setParameters).toHaveBeenCalledWith({ settings: { reasoningEffort: effort } })
  })
})
