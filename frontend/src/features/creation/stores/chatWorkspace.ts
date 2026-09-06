import { defineStore } from 'pinia'
import { computed, onScopeDispose, ref, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { userGroupsAPI } from '@/api/groups'
import type { Group } from '@/types'
import type { CreationMessage, CreationSession, GatewayModelItem } from '../types'
import { isChatCapableModel } from '../mediaModels'
import { chatAPI, validateChatAttachments, type ChatModelMetadata } from '../chatApi'
import { chatTools, executeChatTool } from '../chatTools'
import { getChatReasoningEfforts, supportsChatTemperature } from '../chatCapabilities'
import {
  createLocalChatId, localChatStorage, snapshotChatSession,
  type ChatSettings, type LocalChatDraft, type LocalChatMessage, type LocalChatSession,
} from '../localChat'

type ChatModelDetail = GatewayModelItem & ChatModelMetadata
interface ActiveSend {
  controller: AbortController
  session: LocalChatSession
  assistant: LocalChatMessage
  epoch: number
  timer?: ReturnType<typeof setTimeout>
}
function stopped(): DOMException { return new DOMException('Stopped or active account changed', 'AbortError') }
function isAbort(error: unknown): boolean { return error instanceof Error && error.name === 'AbortError' }
function errorText(error: unknown): string { return error instanceof Error ? error.message : 'The local conversation operation failed.' }
function importedText(content: unknown): string {
  if (typeof content === 'string') {
    if (!content.trim().startsWith('[') && !content.trim().startsWith('{')) return content
    try { return importedText(JSON.parse(content)) } catch { return content }
  }
  if (Array.isArray(content)) return content.map(importedText).filter(Boolean).join('\n')
  if (content && typeof content === 'object' && 'text' in content && typeof content.text === 'string') return content.text
  return ''
}

export const useChatWorkspace = defineStore('chatWorkspace', () => {
  const auth = useAuthStore()
  const sessions = ref<LocalChatSession[]>([])
  const selectedSessionId = ref<string | null>(null)
  const groups = ref<Group[]>([])
  const groupId = ref<number | null>(null)
  const model = ref('')
  const models = ref<string[]>([])
  const modelDetails = ref<Record<string, ChatModelDetail>>({})
  const loading = ref(false)
  const modelsLoading = ref(false)
  const initialized = ref(false)
  const error = ref<string | null>(null)
  const activeSessionId = ref<string | null>(null)
  const selectedSession = computed(() => sessions.value.find(session => session.id === selectedSessionId.value) ?? null)
  const messages = computed(() => selectedSession.value?.messages ?? [])
  const draft = computed<LocalChatDraft>(() => selectedSession.value?.draft ?? { text: '', files: [] })
  const settings = computed(() => selectedSession.value?.settings ?? {})
  const streaming = computed(() => activeSessionId.value === selectedSessionId.value && activeSessionId.value != null)
  let epoch = 0
  let selectionRevision = 0
  let active: ActiveSend | null = null
  let modelController: AbortController | null = null
  let initialization: Promise<void> | null = null
  let historyLoaded = false
  const saves = new Map<string, Promise<void>>()
  const deleting = new Set<string>()

  function owner(): number {
    if (!auth.isAuthenticated || !auth.user?.id) throw new Error('Sign in to use local conversations.')
    return auth.user.id
  }
  function assertOwner(userId: number, started: number): void {
    if (userId !== auth.user?.id || !auth.isAuthenticated || started !== epoch) throw stopped()
  }
  function sessionById(id: string): LocalChatSession {
    const userId = owner()
    const session = sessions.value.find(item => item.id === id && item.userId === userId)
    if (!session) throw new Error('This conversation is not available for the active account.')
    return session
  }
  function reset(): void {
    epoch += 1
    selectionRevision += 1
    if (active) { active.controller.abort(); clearTimeout(active.timer) }
    active = null
    activeSessionId.value = null
    modelController?.abort()
    modelController = null
    sessions.value = []
    selectedSessionId.value = null
    groups.value = []
    groupId.value = null
    model.value = ''
    models.value = []
    modelDetails.value = {}
    loading.value = false
    modelsLoading.value = false
    initialized.value = false
    historyLoaded = false
    initialization = null
    saves.clear()
    deleting.clear()
    error.value = null
  }
  watch([() => auth.user?.id, () => auth.isAuthenticated], reset, { flush: 'sync' })
  onScopeDispose(reset)

  function persist(session: LocalChatSession, started = epoch): Promise<void> {
    assertOwner(session.userId, started)
    if (deleting.has(session.id)) return Promise.reject(new Error('This conversation is being deleted.'))
    const snapshot = snapshotChatSession(session)
    const previous = saves.get(session.id) ?? Promise.resolve()
    const operation = previous.catch(() => undefined).then(async () => {
      assertOwner(session.userId, started)
      await localChatStorage.saveSession(snapshot)
      assertOwner(session.userId, started)
      session.persisted = true
    }).catch(failure => {
      if (started === epoch) { session.persisted = false; error.value = errorText(failure) }
      throw failure
    })
    saves.set(session.id, operation)
    void operation.finally(() => { if (saves.get(session.id) === operation) saves.delete(session.id) }).catch(() => undefined)
    return operation
  }

  async function saveSession(id = selectedSessionId.value): Promise<void> {
    if (!id) return
    await persist(sessionById(id))
    error.value = null
  }

  async function loadModels(targetGroupId: number): Promise<void> {
    const userId = owner()
    const started = epoch
    modelController?.abort()
    const controller = new AbortController()
    modelController = controller
    modelsLoading.value = true
    models.value = []
    modelDetails.value = {}
    try {
      const result = await chatAPI.getModels(targetGroupId, controller.signal)
      assertOwner(userId, started)
      if (controller !== modelController) return
      const available = result.data.filter(item => isChatCapableModel(item.id) && !/video|speech|transcri|audio/i.test(item.id))
      models.value = [...new Set(available.map(item => item.id))]
      modelDetails.value = Object.fromEntries(available.map(item => [item.id, item]))
    } catch (failure) {
      if (!controller.signal.aborted && started === epoch) { error.value = errorText(failure); throw failure }
    } finally {
      if (controller === modelController) modelsLoading.value = false
    }
  }

  async function stop(): Promise<void> {
    const run = active
    if (!run) return
    active = null
    activeSessionId.value = null
    clearTimeout(run.timer)
    run.assistant.status = 'stopped'
    run.session.updatedAt = new Date().toISOString()
    run.controller.abort()
    await persist(run.session, run.epoch)
  }

  async function select(id: string, options?: { signal?: AbortSignal }): Promise<void> {
    const session = sessionById(id)
    const userId = owner()
    const started = epoch
    const revision = ++selectionRevision
    await stop()
    assertOwner(userId, started)
    if (options?.signal?.aborted) throw stopped()
    if (revision !== selectionRevision) return
    selectedSessionId.value = id
    groupId.value = session.groupId || groups.value[0]?.id || null
    model.value = session.model
    await localChatStorage.setSelection(userId, id)
    assertOwner(userId, started)
    if (revision !== selectionRevision) return
    if (groupId.value) await loadModels(groupId.value)
    if (revision !== selectionRevision || started !== epoch) return
    if (!session.model && models.value[0]) {
      session.model = models.value[0]
      model.value = session.model
      session.groupId = groupId.value ?? 0
      session.platform = groups.value.find(group => group.id === session.groupId)?.platform ?? 'openai'
      await persist(session, started)
    }
  }

  function newSession(input: { title?: string; groupId?: number; model?: string; branchOf?: string } = {}): LocalChatSession {
    const targetGroup = input.groupId ?? groupId.value ?? groups.value[0]?.id ?? 0
    const now = new Date().toISOString()
    return {
      id: createLocalChatId(), userId: owner(), title: input.title || 'New conversation', groupId: targetGroup,
      model: input.model ?? model.value, platform: groups.value.find(group => group.id === targetGroup)?.platform ?? 'openai',
      settings: {}, createdAt: now, updatedAt: now, messages: [], draft: { text: '', files: [] }, persisted: false,
      branchOf: input.branchOf,
    }
  }

  async function create(input: { title?: string; groupId?: number; model?: string } = {}): Promise<LocalChatSession> {
    const userId = owner()
    const started = epoch
    const revision = ++selectionRevision
    await stop()
    assertOwner(userId, started)
    const session = newSession(input)
    await persist(session, started)
    assertOwner(userId, started)
    sessions.value.unshift(session)
    if (revision === selectionRevision) await select(session.id)
    return sessionById(session.id)
  }

  async function init(): Promise<void> {
    if (initialized.value) return
    if (initialization) return initialization
    const userId = owner()
    const started = epoch
    loading.value = true
    const operation = (async () => {
      if (!historyLoaded) {
        const saved = await localChatStorage.listSessions(userId)
        assertOwner(userId, started)
        sessions.value = saved
        historyLoaded = true
        for (const session of sessions.value) {
          if (session.messages.some(message => message.status === 'streaming')) {
            session.messages.forEach(message => { if (message.status === 'streaming') message.status = 'stopped' })
            await persist(session, started)
          }
        }
      }
      const available = await userGroupsAPI.getAvailable()
      assertOwner(userId, started)
      groups.value = available
      if (!sessions.value.length) await create({ groupId: groups.value[0]?.id })
      else {
        const savedSelection = await localChatStorage.getSelection(userId)
        assertOwner(userId, started)
        await select(sessions.value.some(session => session.id === savedSelection) ? savedSelection! : sessions.value[0]!.id)
      }
      assertOwner(userId, started)
      initialized.value = true
    })()
    initialization = operation
    try { await operation } catch (failure) {
      if (!isAbort(failure) && started === epoch) error.value = errorText(failure)
      throw failure
    } finally {
      if (started === epoch) { loading.value = false; initialization = null }
    }
  }

  async function setParameters(input: { groupId?: number; model?: string; settings?: ChatSettings }): Promise<void> {
    const temperature = input.settings?.temperature
    const maxTokens = input.settings?.maxTokens
    if (temperature != null && (!Number.isFinite(temperature) || temperature < 0 || temperature > 2)) throw new Error('Temperature must be between 0 and 2.')
    if (maxTokens != null && (!Number.isSafeInteger(maxTokens) || maxTokens < 1 || maxTokens > 131072)) throw new Error('Maximum tokens must be an integer between 1 and 131072.')
    const id = selectedSessionId.value
    if (!id) return
    const session = sessionById(id)
    const started = epoch
    const revision = ++selectionRevision
    await stop()
    assertOwner(session.userId, started)
    if (revision !== selectionRevision) return
    if (input.groupId != null) {
      const group = groups.value.find(item => item.id === input.groupId)
      if (!group) throw new Error('Select an available group.')
      session.groupId = group.id
      session.platform = group.platform
      groupId.value = group.id
      await loadModels(group.id)
      assertOwner(session.userId, started)
      if (revision !== selectionRevision) return
      session.model = input.model ?? (models.value.includes(session.model) ? session.model : models.value[0] ?? '')
    } else if (input.model != null) session.model = input.model
    if (input.settings) session.settings = { ...session.settings, ...input.settings }
    const effort = session.settings.reasoningEffort
    if (effort && effort !== 'auto' && !getChatReasoningEfforts(session.model, session.platform, modelDetails.value[session.model]).includes(effort)) session.settings.reasoningEffort = 'auto'
    if (!supportsChatTemperature(session.model)) session.settings.temperature = undefined
    model.value = session.model
    session.updatedAt = new Date().toISOString()
    await persist(session, started)
  }

  async function rename(id: string, title: string): Promise<void> {
    if (!title.trim()) throw new Error('A conversation title is required.')
    const session = sessionById(id)
    session.title = title.trim().slice(0, 200)
    session.updatedAt = new Date().toISOString()
    await persist(session)
  }

  async function saveDraft(input: LocalChatDraft, id = selectedSessionId.value): Promise<void> {
    const userId = owner()
    const started = epoch
    if (!id) id = (await create()).id
    assertOwner(userId, started)
    validateChatAttachments(input.files)
    const session = sessionById(id)
    session.draft = { text: input.text, files: [...input.files] }
    await persist(session, started)
  }
  async function loadDraft(id = selectedSessionId.value): Promise<LocalChatDraft> {
    return id ? { text: sessionById(id).draft.text, files: [...sessionById(id).draft.files] } : { text: '', files: [] }
  }

  async function deleteSession(id: string): Promise<void> {
    const session = sessionById(id)
    const started = epoch
    if (active?.session.id === id) await stop()
    deleting.add(id)
    try {
      await saves.get(id)?.catch(() => undefined)
      assertOwner(session.userId, started)
      await localChatStorage.deleteSession(session.userId, id)
      assertOwner(session.userId, started)
    } catch (failure) {
      if (started === epoch) { deleting.delete(id); error.value = errorText(failure) }
      throw failure
    }
    sessions.value = sessions.value.filter(item => item.id !== id)
    if (selectedSessionId.value === id) {
      selectedSessionId.value = null
      if (sessions.value[0]) await select(sessions.value[0].id)
      else await create()
    }
  }

  function assertRun(run: ActiveSend): void {
    assertOwner(run.session.userId, run.epoch)
    if (active !== run || run.controller.signal.aborted) throw stopped()
  }
  function scheduleSave(run: ActiveSend): void {
    if (run.timer) return
    run.timer = setTimeout(() => {
      run.timer = undefined
      if (active !== run) return
      void persist(run.session, run.epoch).catch(failure => {
        if (active !== run) return
        run.assistant.status = 'error'
        run.assistant.error = errorText(failure)
        run.controller.abort()
      })
    }, 300)
  }

  async function generate(run: ActiveSend): Promise<void> {
    const { session, assistant } = run
    const request = snapshotChatSession(session)
    try {
      const text = await chatAPI.stream({
        groupId: request.groupId, platform: request.platform, model: request.model, settings: request.settings,
        modelMetadata: modelDetails.value[request.model],
        messages: request.messages.slice(0, -1), signal: run.controller.signal,
        tools: chatTools, executeTool: executeChatTool,
        onDelta: delta => { assertRun(run); assistant.content += delta; scheduleSave(run) },
        onReasoningDelta: delta => { assertRun(run); assistant.reasoning = (assistant.reasoning ?? '') + delta; scheduleSave(run) },
        onUsage: usage => { assertRun(run); assistant.usage = { ...usage }; assistant.input_tokens = usage.input_tokens; assistant.output_tokens = usage.output_tokens },
        onSource: source => { assertRun(run); assistant.sources ??= []; if (!assistant.sources.some(item => item.url === source.url)) assistant.sources.push(source) },
        onToolCall: call => { assertRun(run); assistant.toolCalls ??= []; assistant.toolCalls.push(call); scheduleSave(run) },
        onToolResult: result => { assertRun(run); assistant.toolResults ??= []; assistant.toolResults.push(result); scheduleSave(run) },
        onTurn: turn => { assertRun(run); assistant.turns ??= []; assistant.turns.push(turn); scheduleSave(run) },
      })
      assertRun(run)
      assistant.content = text
      assistant.status = 'completed'
    } catch (failure) {
      if (run.epoch !== epoch || active !== run) return
      if (assistant.status !== 'error') assistant.status = isAbort(failure) ? 'stopped' : 'error'
      if (!isAbort(failure)) { assistant.error = errorText(failure); error.value = assistant.error }
    } finally {
      clearTimeout(run.timer)
      if (active === run) {
        active = null
        activeSessionId.value = null
        session.updatedAt = new Date().toISOString()
        await persist(session, run.epoch)
      }
    }
  }

  async function send(input: { text: string; files?: File[] }): Promise<void> {
    const userId = owner()
    const started = epoch
    const revision = selectionRevision
    const intendedId = selectedSessionId.value
    const wasInitialized = initialized.value
    await init()
    assertOwner(userId, started)
    // An explicit user selection during initialization must not receive an older draft.
    if (intendedId && selectedSessionId.value !== intendedId || wasInitialized && revision !== selectionRevision) throw stopped()
    if (active) throw new Error('Stop the current reply before sending another message.')
    const session = selectedSession.value ?? await create()
    const files = input.files ?? []
    if (!input.text.trim() && !files.length) return
    validateChatAttachments(files, session.platform, session.model, modelDetails.value[session.model])
    if (!groups.value.some(group => group.id === session.groupId) || !session.model) throw new Error('Select an available group and model.')
    const previousDraft = session.draft
    const user: LocalChatMessage = { id: createLocalChatId(), role: 'user', content: input.text, files: [...files], createdAt: new Date().toISOString(), status: 'completed' }
    const assistant: LocalChatMessage = { id: createLocalChatId(), role: 'assistant', content: '', files: [], createdAt: new Date().toISOString(), status: 'streaming', model: session.model }
    session.messages.push(user, assistant)
    session.draft = { text: '', files: [] }
    if (session.messages.length === 2 && session.title === 'New conversation') session.title = input.text.trim().slice(0, 70) || files[0]?.name || session.title
    session.updatedAt = new Date().toISOString()
    const run: ActiveSend = { controller: new AbortController(), session, assistant: session.messages[session.messages.length - 1]!, epoch: started }
    active = run
    activeSessionId.value = session.id
    try { await persist(session, started) } catch (failure) {
      if (started === epoch && active === run) {
        session.messages.splice(-2)
        if (!session.draft.text && !session.draft.files.length) session.draft = previousDraft
        active = null
        activeSessionId.value = null
      }
      throw failure
    }
    if (active !== run) return
    assertRun(run)
    error.value = null
    await generate(run)
  }

  async function branchAndGenerate(messageId: string, edit?: { text: string; files?: File[] }): Promise<void> {
    const original = selectedSession.value
    if (!original) return
    const started = epoch
    const revision = selectionRevision
    await stop()
    assertOwner(original.userId, started)
    if (revision !== selectionRevision) throw stopped()
    const index = original.messages.findIndex(message => message.id === messageId)
    const target = original.messages[index]
    if (!target || (edit ? target.role !== 'user' : target.role !== 'assistant')) throw new Error('Select a user message to edit or an assistant reply to regenerate.')
    const branch = newSession({ title: `${original.title} (branch)`, groupId: original.groupId, model: original.model, branchOf: original.id })
    branch.platform = original.platform
    branch.settings = { ...original.settings }
    branch.messages = snapshotChatSession(original).messages.slice(0, edit ? index + 1 : index)
    if (edit) {
      const user = branch.messages[branch.messages.length - 1]!
      validateChatAttachments(edit.files ?? user.files, branch.platform, branch.model, modelDetails.value[branch.model])
      user.content = edit.text
      user.files = [...(edit.files ?? user.files)]
      if (edit.files) user.rawContent = undefined
    }
    if (branch.messages[branch.messages.length - 1]?.role !== 'user') throw new Error('The reply has no preceding user message.')
    branch.messages.push({ id: createLocalChatId(), role: 'assistant', content: '', files: [], createdAt: new Date().toISOString(), status: 'streaming', model: branch.model })
    await persist(branch, started)
    assertOwner(original.userId, started)
    sessions.value.unshift(branch)
    if (revision !== selectionRevision) return
    await select(branch.id)
    assertOwner(original.userId, started)
    if (selectedSessionId.value !== branch.id) return
    const selected = sessionById(branch.id)
    const run: ActiveSend = { controller: new AbortController(), session: selected, assistant: selected.messages[selected.messages.length - 1]!, epoch: started }
    if (active) throw new Error('Stop the current reply before regenerating.')
    active = run
    activeSessionId.value = selected.id
    await generate(run)
  }

  async function vote(id: string, value: 'up' | 'down' | null): Promise<void> {
    const session = selectedSession.value
    const message = session?.messages.find(item => item.id === id)
    if (!session || !message) return
    message.vote = value
    await persist(session)
  }

  async function importLegacyConversation(input: { session: CreationSession; messages: CreationMessage[]; signal?: AbortSignal }): Promise<LocalChatSession> {
    const userId = owner()
    const started = epoch
    const revision = selectionRevision
    await init()
    assertOwner(userId, started)
    if (input.signal?.aborted) throw stopped()
    if (input.session.user_id !== userId || input.messages.some(message => message.session_id !== input.session.id)) throw new Error('This conversation belongs to another account or session.')
    const existing = sessions.value.find(session => session.legacySessionId === input.session.id)
    if (existing) { await select(existing.id, { signal: input.signal }); return existing }
    const session = newSession({ title: input.session.title, groupId: input.session.group_id, model: input.session.model })
    session.legacySessionId = input.session.id
    session.legacySession = input.session
    session.createdAt = input.session.created_at
    session.messages = input.messages.map(message => ({
      id: createLocalChatId(), role: message.role, content: importedText(message.content), rawContent: message.content, legacyMessage: message,
      files: [], createdAt: message.created_at, model: message.model || undefined, status: 'completed',
      input_tokens: message.input_tokens ?? undefined, output_tokens: message.output_tokens ?? undefined,
    }))
    await persist(session, started)
    assertOwner(userId, started)
    sessions.value.unshift(session)
    if (!input.signal?.aborted && (revision === selectionRevision || !selectedSession.value?.messages.length)) await select(session.id, { signal: input.signal })
    return sessionById(session.id)
  }

  return {
    sessions, selectedSessionId, selectedSession, messages, groups, groupId, model, models, modelDetails,
    loading, sessionLoading: loading, modelsLoading, streaming, initialized, error, draft, settings,
    init, create, select, rename, deleteSession, send, stop, setParameters, loadModels, saveDraft, loadDraft, saveSession, vote,
    editAndRegenerate: (id: string, text: string, files?: File[]) => branchAndGenerate(id, { text, files }),
    regenerate: (id: string) => branchAndGenerate(id), importLegacyConversation, reset,
    createSession: create, selectSession: select, renameSession: rename,
    setGroupId: (id: number) => setParameters({ groupId: id }), setModel: (id: string) => setParameters({ model: id }),
    submitText: (text: string, files?: File[]) => send({ text, files }), stopStreaming: stop,
    clearError: () => { error.value = null },
  }
})
