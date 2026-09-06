import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { useChatWorkspace } from '../stores/chatWorkspace'
import { localChatStorage, LocalChatStorageError, snapshotChatSession, type LocalChatSession } from '../localChat'
import { chatAPI, type ChatStreamOptions } from '../chatApi'
import type { CreationSession } from '../types'

vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('@/api/groups', () => ({ userGroupsAPI: { getAvailable: vi.fn(async () => [{ id: 2, name: 'Chat', platform: 'openai' }, { id: 3, name: 'Claude', platform: 'anthropic' }]) } }))
vi.mock('../chatTools', () => ({ chatTools: [], executeChatTool: vi.fn() }))
vi.mock('../chatApi', () => ({ chatAPI: { getModels: vi.fn(), stream: vi.fn() }, validateChatAttachments: vi.fn() }))
vi.mock('../localChat', async original => ({
  ...await original<typeof import('../localChat')>(),
  localChatStorage: { listSessions: vi.fn(), saveSession: vi.fn(), deleteSession: vi.fn(), getSelection: vi.fn(), setSelection: vi.fn() },
}))
const auth = reactive({ user: { id: 1 }, isAuthenticated: true })
const records = new Map<string, LocalChatSession>()
let store: ReturnType<typeof useChatWorkspace>
const saved = (): LocalChatSession => ({
  id: 'saved', userId: 1, title: 'Saved', groupId: 2, model: 'gpt-5', platform: 'openai', settings: {},
  createdAt: '', updatedAt: '', messages: [], draft: { text: '', files: [] }, persisted: true,
})

beforeEach(() => {
  vi.clearAllMocks()
  records.clear()
  auth.user = { id: 1 }
  auth.isAuthenticated = true
  vi.mocked(localChatStorage.listSessions).mockImplementation(async userId => [...records.values()].filter(item => item.userId === userId).map(snapshotChatSession))
  vi.mocked(localChatStorage.saveSession).mockImplementation(async session => { records.set(`${session.userId}:${session.id}`, snapshotChatSession(session)) })
  vi.mocked(localChatStorage.getSelection).mockResolvedValue('saved')
  vi.mocked(localChatStorage.setSelection).mockResolvedValue()
  vi.mocked(localChatStorage.deleteSession).mockImplementation(async (userId, id) => { records.delete(`${userId}:${id}`) })
  vi.mocked(chatAPI.getModels).mockResolvedValue({ data: [{ id: 'gpt-5' }] })
  vi.mocked(chatAPI.stream).mockImplementation(async options => { options.onDelta('Reply'); return 'Reply' })
  setActivePinia(createPinia())
  store = useChatWorkspace()
})
afterEach(() => { store.$dispose(); vi.useRealTimers() })

describe('local-first chat workspace', () => {
  it('clears stale reasoning and temperature settings when changing the model or group', async () => {
    vi.mocked(chatAPI.getModels).mockResolvedValue({ data: [{ id: 'gpt-5' }, { id: 'gpt-4o' }, { id: 'claude-opus-4-6' }] })
    await store.init()
    await store.setParameters({ model: 'gpt-4o', settings: { temperature: 0.7 } })
    expect(store.settings.temperature).toBe(0.7)
    await store.setParameters({ model: 'gpt-5', settings: { reasoningEffort: 'high' } })
    expect(store.settings.temperature).toBeUndefined()
    expect(store.settings.reasoningEffort).toBe('high')
    await store.setParameters({ model: 'gpt-4o' })
    expect(store.settings.reasoningEffort).toBe('auto')
    await store.setParameters({ groupId: 3, model: 'claude-opus-4-6', settings: { reasoningEffort: 'max' } })
    expect(store.settings.reasoningEffort).toBe('max')
    await store.setParameters({ groupId: 2, model: 'gpt-5' })
    expect(store.settings.reasoningEffort).toBe('auto')
  })

  it('creates an initial local session and saves the user message and attachments before streaming', async () => {
    await store.init()
    const file = new File(['notes'], 'notes.txt', { type: 'text/plain' })
    await store.send({ text: 'Hello', files: [file] })
    expect(store.sessions).toHaveLength(1)
    expect(store.messages.map(message => message.content)).toEqual(['Hello', 'Reply'])
    expect(store.messages[0]!.files).toEqual([file])
    expect(store.messages[1]!.input_tokens).toBeUndefined()
    expect(vi.mocked(localChatStorage.saveSession).mock.invocationCallOrder[0]).toBeLessThan(vi.mocked(chatAPI.stream).mock.invocationCallOrder[0]!)
    expect(records.get(`1:${store.selectedSessionId}`)?.messages[1]?.content).toBe('Reply')
  })

  it('does not call the provider if the local preflight save fails', async () => {
    await store.init()
    await store.saveDraft({ text: 'Unsent', files: [] })
    vi.mocked(localChatStorage.saveSession).mockRejectedValue(new LocalChatStorageError('quota'))
    await expect(store.send({ text: 'Unsent' })).rejects.toThrow('Browser storage is full')
    expect(chatAPI.stream).not.toHaveBeenCalled()
    expect(store.messages).toHaveLength(0)
    expect(store.draft.text).toBe('Unsent')
    expect(store.streaming).toBe(false)
  })

  it('keeps final content when local storage fails and retry-save never resends', async () => {
    await store.init()
    const save = vi.mocked(localChatStorage.saveSession).getMockImplementation()!
    vi.mocked(localChatStorage.saveSession).mockImplementation(async session => {
      if (session.messages.some(message => message.content === 'Reply')) throw new LocalChatStorageError('quota')
      return save(session)
    })
    await expect(store.send({ text: 'Hello' })).rejects.toThrow('Browser storage is full')
    expect(store.messages[1]?.content).toBe('Reply')
    expect(store.selectedSession?.persisted).toBe(false)
    vi.mocked(localChatStorage.saveSession).mockImplementation(save)
    await store.saveSession()
    expect(chatAPI.stream).toHaveBeenCalledTimes(1)
    expect(store.selectedSession?.persisted).toBe(true)
  })

  it('retains partial content on stop and rejects late chunks without resending', async () => {
    await store.init()
    let options!: ChatStreamOptions
    let finish!: (text: string) => void
    vi.mocked(chatAPI.stream).mockImplementation(input => { options = input; input.onDelta('Partial'); return new Promise(resolve => { finish = resolve }) })
    const sending = store.send({ text: 'Hello' })
    await flushPromises()
    await store.stop()
    expect(options.signal.aborted).toBe(true)
    expect(store.messages[1]).toMatchObject({ content: 'Partial', status: 'stopped' })
    expect(() => options.onDelta('Late')).toThrow()
    finish('Late complete')
    await sending
    expect(store.messages[1]?.content).toBe('Partial')
    expect(chatAPI.stream).toHaveBeenCalledTimes(1)
  })

  it('reloads stored files and interrupted text without restarting the provider', async () => {
    const session = saved()
    const file = new File(['source'], 'source.png', { type: 'image/png' })
    session.draft = { text: 'Draft', files: [file] }
    session.messages = [{ id: 'partial', role: 'assistant', content: 'Partial', files: [], createdAt: '', status: 'streaming' }]
    records.set('1:saved', session)
    await store.init()
    expect(store.messages[0]).toMatchObject({ content: 'Partial', status: 'stopped' })
    expect(store.draft.files).toEqual([file])
    expect(chatAPI.stream).not.toHaveBeenCalled()
  })

  it('creates recoverable branches for user edits and assistant regeneration', async () => {
    await store.init()
    await store.send({ text: 'Original' })
    const originalId = store.selectedSessionId!
    const userId = store.messages[0]!.id
    await store.editAndRegenerate(userId, 'Edited')
    expect(store.selectedSession?.branchOf).toBe(originalId)
    expect(store.messages[0]?.content).toBe('Edited')
    expect(store.sessions.find(session => session.id === originalId)?.messages[0]?.content).toBe('Original')
    const editedId = store.selectedSessionId!
    await store.regenerate(store.messages[1]!.id)
    expect(store.selectedSession?.branchOf).toBe(editedId)
    expect(store.sessions).toHaveLength(3)
    expect(chatAPI.stream).toHaveBeenCalledTimes(3)
  })

  it('clears private memory and cancels the stream on account change', async () => {
    await store.init()
    let options!: ChatStreamOptions
    let finish!: (text: string) => void
    vi.mocked(chatAPI.stream).mockImplementation(input => { options = input; return new Promise(resolve => { finish = resolve }) })
    const sending = store.send({ text: 'Private' })
    await flushPromises()
    auth.user = { id: 2 }
    expect(options.signal.aborted).toBe(true)
    expect(store.sessions).toHaveLength(0)
    expect(() => options.onDelta('Private reply')).toThrow()
    finish('Private reply')
    await sending
    await store.init()
    expect(store.messages).toHaveLength(0)
    expect(store.selectedSession?.userId).toBe(2)
  })

  it('does not clear local state on same-user profile refresh', async () => {
    await store.init()
    const id = store.selectedSessionId
    auth.user = { id: 1 }
    expect(store.selectedSessionId).toBe(id)
  })

  it('prevents overlapping sends while a local save is still pending', async () => {
    await store.init()
    let finishSave!: () => void
    vi.mocked(localChatStorage.saveSession).mockImplementationOnce(() => new Promise(resolve => { finishSave = resolve }))
    const first = store.send({ text: 'First' })
    await flushPromises()
    await expect(store.send({ text: 'Second' })).rejects.toThrow('Stop the current reply')
    finishSave()
    await first
    expect(chatAPI.stream).toHaveBeenCalledTimes(1)
    expect(store.messages[0]?.content).toBe('First')
  })

  it('persists tool results, reasoning, sources, known usage and local votes', async () => {
    await store.init()
    vi.mocked(chatAPI.stream).mockImplementation(async options => {
      options.onReasoningDelta?.('Reasoning')
      options.onToolResult?.({ toolCallId: 'chart', name: 'chart', content: 'Chart', data: { type: 'chart' } })
      options.onSource?.({ url: 'https://example.test', title: 'Source' })
      options.onUsage?.({ input_tokens: 7 })
      options.onDelta('Reply')
      return 'Reply'
    })
    await store.send({ text: 'Chart' })
    const reply = store.messages[1]!
    expect(reply).toMatchObject({ reasoning: 'Reasoning', input_tokens: 7, toolResults: [{ data: { type: 'chart' } }] })
    expect(reply.output_tokens).toBeUndefined()
    await store.vote(reply.id, 'up')
    expect(records.get(`1:${store.selectedSessionId}`)?.messages[1]?.vote).toBe('up')
  })

  it('imports a legacy conversation explicitly, preserves raw metadata and deduplicates it', async () => {
    const session: CreationSession = { id: 99, user_id: 1, title: 'Old chat', group_id: 2, model: 'gpt-5', mode: 'chat', status: 'active', metadata: { source: 'legacy' }, created_at: '2025-01-01', updated_at: '2025-01-02' }
    const messages = [{ id: 88, session_id: 99, role: 'user' as const, content: [{ type: 'text', text: 'Legacy' }, { type: 'image_url', image_url: { url: 'https://example.test/source.png' } }], created_at: '2025-01-01' }]
    const local = await store.importLegacyConversation({ session, messages })
    expect(local.messages[0]?.rawContent).toEqual(messages[0]!.content)
    expect(local.legacySession?.metadata).toEqual({ source: 'legacy' })
    const again = await store.importLegacyConversation({ session, messages })
    expect(again.id).toBe(local.id)
    expect(chatAPI.stream).not.toHaveBeenCalled()
    await expect(store.importLegacyConversation({ session: { ...session, user_id: 2 }, messages })).rejects.toThrow('another account')
  })
})
