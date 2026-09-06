import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { userGroupsAPI } from '@/api/groups'
import { i18n } from '@/i18n'
import type { Group, GroupPlatform } from '@/types'
import creationAPI, {
  extractImageUrlFromTask,
  mapAsyncTaskToImageJob,
} from '../api'
import { classifyModels, pickDefaultModel } from '../mediaModels'
import type {
  CreationImageJob,
  CreationExchangeRequest,
  CreationMessage,
  CreationTokenUsage,
  CreationSession,
  CreationSessionMode,
  PendingSendRequest,
} from '../types'

const IMAGE_POLL_INTERVAL_MS = 3000

async function loadAllPages<T>(fetchPage: (page: number) => Promise<{ items: T[]; total: number }>): Promise<T[]> {
  const items: T[] = []
  for (let page = 1; ; page += 1) {
    const response = await fetchPage(page)
    items.push(...response.items)
    if (response.items.length === 0 || items.length >= response.total) return items
  }
}

type ImagePoller = {
  timer: ReturnType<typeof setInterval>
  sessionId: number
  inFlight: boolean
}

type PendingExchange = {
  request: CreationExchangeRequest
  user: CreationMessage
  assistant: CreationMessage
}

function studioError(key: string): string {
  return String(i18n.global.t(key))
}

function isAbortError(err: unknown): boolean {
  return (
    (err instanceof DOMException && err.name === 'AbortError') ||
    (err instanceof Error && err.name === 'AbortError')
  )
}

class InterruptedCreationSend extends Error {
  constructor() {
    super('Aborted')
    this.name = 'AbortError'
  }
}

export const useCreationStore = defineStore('creation', () => {
  const sessions = ref<CreationSession[]>([])
  const sessionsLoading = ref(false)
  const sessionLoading = ref(false)
  const selectedSessionId = ref<number | null>(null)
  const sessionModeFilter = ref<CreationSessionMode>('chat')

  const messages = ref<CreationMessage[]>([])
  const messagesLoading = ref(false)

  const imageTasks = ref<CreationImageJob[]>([])
  const imageTasksLoading = ref(false)

  const groups = ref<Group[]>([])
  const groupId = ref<number | null>(null)
  const model = ref('')
  const chatModels = ref<string[]>([])
  const imageModels = ref<string[]>([])

  const streaming = ref(false)
  const streamingContent = ref('')
  const error = ref<string | null>(null)

  const pendingQueue = ref<PendingSendRequest[]>([])
  const lastFailedSend = ref<{ sessionId: number; text: string } | null>(null)
  const initialized = ref(false)
  const generationAvailable = ref(true)
  const pendingExchanges = ref(new Map<number, PendingExchange>())
  const savingSessions = ref(new Set<number>())

  let streamAbort: AbortController | null = null
  let stateEpoch = 0
  let queueRunId = 0
  let initialization: Promise<void> | null = null
  let modelRequestId = 0
  let messagesRequestId = 0
  let imageTasksRequestId = 0
  let selectionRequestId = 0
  let sessionsRequestId = 0
  const modelUpdates = ref(new Set<number>())
  const imagePollers = new Map<string, ImagePoller>
  let queueRunning = false

  const selectedSession = computed(() =>
    sessions.value.find((session) => session.id === selectedSessionId.value) ?? null,
  )

  const selectedGroup = computed(() =>
    groups.value.find((group) => group.id === groupId.value) ?? null,
  )

  const selectedPlatform = computed<GroupPlatform | null>(() => selectedGroup.value?.platform ?? null)

  const isImageSession = computed(() => selectedSession.value?.mode === 'image')

  const visibleSessions = computed(() =>
    sessions.value.filter((session) => session.mode === sessionModeFilter.value),
  )

  const availableModels = computed(() =>
    isImageSession.value ? imageModels.value : chatModels.value,
  )

  const hasImageModels = computed(() => imageModels.value.length > 0)
  const modelUpdating = computed(() => selectedSessionId.value != null && modelUpdates.value.has(selectedSessionId.value))
  const hasUnsavedExchange = computed(() => selectedSessionId.value != null && pendingExchanges.value.has(selectedSessionId.value))
  const saving = computed(() => selectedSessionId.value != null && savingSessions.value.has(selectedSessionId.value))

  function assertCurrentEpoch(epoch: number) {
    if (epoch !== stateEpoch) throw new DOMException('Aborted', 'AbortError')
  }

  function withPendingExchange(items: CreationMessage[], sessionId: number): CreationMessage[] {
    const pending = pendingExchanges.value.get(sessionId)
    if (!pending) return items
    const persisted = items.filter((item) => item.exchange_request_id === pending.request.request_id)
    if (persisted.some((item) => item.role === 'user') && persisted.some((item) => item.role === 'assistant')) {
      pendingExchanges.value.delete(sessionId)
      clearMatchingFailure(sessionId, pending.request.user_content)
      return items
    }
    const ids = new Set([pending.user.id, pending.assistant.id])
    return [...items.filter((item) => !ids.has(item.id)), pending.user, pending.assistant]
  }

  function clearMatchingFailure(sessionId: number, text: string) {
    if (lastFailedSend.value?.sessionId === sessionId && lastFailedSend.value.text === text) lastFailedSend.value = null
  }

  function clearImagePollers() {
    for (const poller of imagePollers.values()) {
      clearInterval(poller.timer)
    }
    imagePollers.clear()
  }

  function clearImagePollersForSession(sessionId: number) {
    for (const [taskId, poller] of imagePollers) {
      if (poller.sessionId !== sessionId) continue
      clearInterval(poller.timer)
      imagePollers.delete(taskId)
    }
  }

  function stopStreaming() {
    if (streamAbort) {
      streamAbort.abort()
      streamAbort = null
    }
    streaming.value = false
    streamingContent.value = ''
  }

  async function loadGroups() {
    const epoch = stateEpoch
    const items = await userGroupsAPI.getAvailable()
    assertCurrentEpoch(epoch)
    groups.value = items
    if (!groupId.value && groups.value.length > 0) {
      groupId.value = groups.value[0].id
    }
  }

  async function loadModelsForGroup(targetGroupId = groupId.value) {
    if (!targetGroupId) return
    const requestId = ++modelRequestId
    const list = await creationAPI.getModels(targetGroupId)
    if (requestId !== modelRequestId || groupId.value !== targetGroupId) return
    const classified = classifyModels(list.data ?? [])
    const platform = groups.value.find((group) => group.id === targetGroupId)?.platform
    chatModels.value = classified.chat
    imageModels.value = platform === 'openai' || platform === 'grok' ? classified.image : []
    if (!model.value) {
      model.value = pickDefaultModel(
        isImageSession.value ? imageModels.value : chatModels.value,
        selectedSession.value?.model,
      )
    }
  }

  async function loadSessions() {
    const requestId = ++sessionsRequestId
    sessionsLoading.value = true
    error.value = null
    try {
      const items = await loadAllPages((page) => creationAPI.listSessions({ page, page_size: 100 }))
      if (requestId === sessionsRequestId) sessions.value = [...new Map(items.map((item) => [item.id, item])).values()]
    } catch (err) {
      if (requestId === sessionsRequestId) error.value = err instanceof Error ? err.message : studioError('studio.errors.loadSessions')
      throw err
    } finally {
      if (requestId === sessionsRequestId) sessionsLoading.value = false
    }
  }

  async function loadMessages(sessionId: number) {
    const requestId = ++messagesRequestId
    messagesLoading.value = true
    error.value = null
    try {
      const items = await creationAPI.listSessionMessages(sessionId)
      if (requestId === messagesRequestId && selectedSessionId.value === sessionId) {
        messages.value = withPendingExchange(items, sessionId)
      }
    } catch (err) {
      if (requestId === messagesRequestId) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.loadMessages')
      }
      throw err
    } finally {
      if (requestId === messagesRequestId) {
        messagesLoading.value = false
      }
    }
  }

  function mergeImageTasksWithInFlight(serverItems: CreationImageJob[], sessionId: number): CreationImageJob[] {
    const serverTaskIds = new Set(
      serverItems
        .map((item) => item.provider_task_id)
        .filter((id): id is string => Boolean(id)),
    )
    const serverIds = new Set(serverItems.map((item) => item.id))

    const localsToKeep = imageTasks.value.filter((item) => {
      if (item.session_id !== sessionId) return false
      if (serverIds.has(item.id)) return false
      if (item.provider_task_id && serverTaskIds.has(item.provider_task_id)) return false
      return true
    })

    return [...localsToKeep, ...serverItems].sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    )
  }

  async function loadImageTasks(sessionId: number) {
    const requestId = ++imageTasksRequestId
    const silent = imageTasks.value.length > 0
    if (!silent) {
      imageTasksLoading.value = true
    }
    error.value = null
    try {
      const items = await loadAllPages((page) => creationAPI.listImages({ session_id: sessionId, page, page_size: 100 }))
      const merged = mergeImageTasksWithInFlight([...new Map(items.map((item) => [item.id, item])).values()], sessionId)
      if (requestId === imageTasksRequestId && selectedSessionId.value === sessionId) {
        imageTasks.value = merged
        for (const task of merged) {
          if (
            (task.status === 'pending' || task.status === 'processing') &&
            task.provider_task_id
          ) {
            pollImageTask(task.provider_task_id, sessionId, task.id, task.group_id)
          }
        }
      }
    } catch (err) {
      if (requestId === imageTasksRequestId) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.loadTasks')
      }
      throw err
    } finally {
      if (!silent && requestId === imageTasksRequestId) {
        imageTasksLoading.value = false
      }
    }
  }

  async function selectSession(sessionId: number) {
    const requestId = ++selectionRequestId
    stopStreaming()
    pendingQueue.value = []
    messagesRequestId += 1
    imageTasksRequestId += 1
    messages.value = []
    imageTasks.value = []
    messagesLoading.value = false
    imageTasksLoading.value = false
    selectedSessionId.value = sessionId
    const session = sessions.value.find((item) => item.id === sessionId)
    if (!session) return

    sessionLoading.value = true
    try {
      groupId.value = session.group_id
      model.value = session.model || ''
      let modelError: unknown
      try {
        await loadModelsForGroup(session.group_id)
      } catch (err) {
        modelError = err
      }
      if (requestId !== selectionRequestId || selectedSessionId.value !== sessionId) return
      generationAvailable.value = !modelError
      if (modelError) {
        chatModels.value = []
        imageModels.value = []
      }

      if (session.mode === 'image') {
        await loadImageTasks(sessionId)
      } else {
        await loadMessages(sessionId)
      }
      if (requestId !== selectionRequestId) return
      if (modelError) error.value = modelError instanceof Error ? modelError.message : studioError('studio.errors.noModel')
      const pending = pendingExchanges.value.get(sessionId)
      if (pending) lastFailedSend.value = { sessionId, text: pending.request.user_content }
    } catch (err) {
      if (requestId === selectionRequestId) {
        selectedSessionId.value = null
        error.value = err instanceof Error ? err.message : studioError('studio.errors.loadMessages')
      }
      throw err
    } finally {
      if (requestId === selectionRequestId) sessionLoading.value = false
    }
  }

  async function createSession(mode: CreationSessionMode) {
    if (!groupId.value) throw new Error(studioError('studio.errors.noGroup'))
    const epoch = stateEpoch
    const requestId = ++selectionRequestId
    const targetGroupId = groupId.value
    sessionLoading.value = true
    try {
      await loadModelsForGroup(targetGroupId)
      assertCurrentEpoch(epoch)
      if (requestId !== selectionRequestId || groupId.value !== targetGroupId) throw new DOMException('Aborted', 'AbortError')
      const defaultModel = pickDefaultModel(
        mode === 'image' ? imageModels.value : chatModels.value,
        model.value,
      )
      const session = await creationAPI.createSession({
        group_id: targetGroupId,
        mode,
        model: defaultModel,
        title: mode === 'image' ? 'Image session' : 'New chat',
      })
      assertCurrentEpoch(epoch)
      sessions.value = [session, ...sessions.value]
      if (requestId !== selectionRequestId) return session
      sessionModeFilter.value = mode
      model.value = session.model || defaultModel
      await selectSession(session.id)
      return session
    } catch (err) {
      if (epoch === stateEpoch && requestId === selectionRequestId && !isAbortError(err)) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.loadSessions')
      }
      throw err
    } finally {
      if (epoch === stateEpoch && requestId === selectionRequestId) sessionLoading.value = false
    }
  }

  async function deleteSession(sessionId: number) {
    const epoch = stateEpoch
    if (selectedSessionId.value === sessionId) {
      stopStreaming()
      pendingQueue.value = []
      lastFailedSend.value = null
    }
    clearImagePollersForSession(sessionId)
    try {
      await creationAPI.deleteSession(sessionId)
    } catch (err) {
      if (epoch === stateEpoch) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.loadSessions')
        const pending = pendingExchanges.value.get(sessionId)
        if (pending && selectedSessionId.value === sessionId) {
          lastFailedSend.value = { sessionId, text: pending.request.user_content }
        }
      }
      throw err
    }
    assertCurrentEpoch(epoch)
    pendingExchanges.value.delete(sessionId)
    if (lastFailedSend.value?.sessionId === sessionId) lastFailedSend.value = null
    sessions.value = sessions.value.filter((session) => session.id !== sessionId)
    if (selectedSessionId.value === sessionId) {
      selectedSessionId.value = null
      messages.value = []
      imageTasks.value = []
    }
  }

  async function setGroupId(nextGroupId: number | null) {
    if (nextGroupId === groupId.value) return
    const epoch = stateEpoch
    const requestId = ++selectionRequestId
    stopStreaming()
    clearImagePollers()
    pendingQueue.value = []
    lastFailedSend.value = null
    selectedSessionId.value = null
    messages.value = []
    imageTasks.value = []
    groupId.value = nextGroupId
    if (!nextGroupId) {
      sessionLoading.value = false
      return
    }
    model.value = ''
    sessionLoading.value = true
    generationAvailable.value = false
    try {
      await loadModelsForGroup(nextGroupId)
      assertCurrentEpoch(epoch)
      if (requestId !== selectionRequestId || groupId.value !== nextGroupId) return
      model.value = pickDefaultModel(
        sessionModeFilter.value === 'image' ? imageModels.value : chatModels.value,
        model.value,
      )
      const mode = sessionModeFilter.value === 'image' && imageModels.value.length === 0
        ? 'chat'
        : sessionModeFilter.value
      sessionModeFilter.value = mode
      await createSession(mode)
    } catch (err) {
      if (epoch === stateEpoch && requestId === selectionRequestId && !isAbortError(err)) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.noModel')
      }
      throw err
    } finally {
      if (epoch === stateEpoch && requestId === selectionRequestId) sessionLoading.value = false
    }
  }

  async function setModel(nextModel: string) {
    const epoch = stateEpoch
    const session = selectedSession.value
    if (session && modelUpdates.value.has(session.id)) return
    model.value = nextModel
    if (!session) return
    modelUpdates.value.add(session.id)
    try {
      await creationAPI.updateSession(session.id, { model: nextModel })
      assertCurrentEpoch(epoch)
      session.model = nextModel
      if (selectedSessionId.value === session.id) model.value = nextModel
    } catch (err) {
      if (epoch === stateEpoch && selectedSessionId.value === session.id) {
        model.value = session.model
        error.value = err instanceof Error ? err.message : studioError('studio.errors.send')
      }
      throw err
    } finally {
      if (epoch === stateEpoch) modelUpdates.value.delete(session.id)
    }
  }

  function enqueueSend(sessionId: number, text: string) {
    lastFailedSend.value = null
    pendingQueue.value.push({ sessionId, text })
  }

  async function submitText(text: string) {
    const trimmed = text.trim()
    if (!trimmed || !selectedSessionId.value || sessionLoading.value || messagesLoading.value || modelUpdating.value || !generationAvailable.value || hasUnsavedExchange.value) return
    enqueueSend(selectedSessionId.value, trimmed)
    if (!queueRunning && !streaming.value) {
      await processQueue()
    }
  }

  async function processQueue() {
    if (queueRunning || pendingQueue.value.length === 0) return
    queueRunning = true
    const epoch = stateEpoch
    const runId = ++queueRunId
    try {
      while (pendingQueue.value.length > 0) {
        const job = pendingQueue.value[0]
        try {
          await sendMessage(job.text, job.sessionId)
          if (epoch !== stateEpoch) return
          if (pendingQueue.value[0] === job) {
            pendingQueue.value.shift()
          }
          clearMatchingFailure(job.sessionId, job.text)
        } catch (err) {
          if (epoch !== stateEpoch) return
          if (isAbortError(err) && !(err instanceof InterruptedCreationSend)) {
            if (pendingQueue.value[0] === job) pendingQueue.value.shift()
            continue
          }
          if (selectedSessionId.value === job.sessionId || !lastFailedSend.value) {
            lastFailedSend.value = { sessionId: job.sessionId, text: job.text }
          }
          if (pendingQueue.value[0] === job) {
            pendingQueue.value.shift()
          }
          return
        }
      }
    } finally {
      if (runId === queueRunId) queueRunning = false
    }
  }

  async function savePendingExchange(sessionId: number) {
    const pending = pendingExchanges.value.get(sessionId)
    if (!pending || savingSessions.value.has(sessionId)) return
    const epoch = stateEpoch
    savingSessions.value.add(sessionId)
    try {
      const result = await creationAPI.createSessionExchange(sessionId, pending.request)
      assertCurrentEpoch(epoch)
      pendingExchanges.value.delete(sessionId)
      if (selectedSessionId.value === sessionId) {
        messages.value = [
          ...messages.value.filter((message) => ![pending.user.id, pending.assistant.id, result.user.id, result.assistant.id].includes(message.id)),
          result.user,
          result.assistant,
        ]
      }
    } catch (err) {
      if (epoch === stateEpoch && selectedSessionId.value === sessionId && !isAbortError(err)) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.send')
      }
      throw err
    } finally {
      if (epoch === stateEpoch) savingSessions.value.delete(sessionId)
    }
  }

  async function sendMessage(text: string, sessionId = selectedSessionId.value) {
    const epoch = stateEpoch
    const trimmed = text.trim()
    if (!trimmed || !sessionId) return
    if (sessionId !== selectedSessionId.value) await selectSession(sessionId)
    assertCurrentEpoch(epoch)
    if (sessionId !== selectedSessionId.value) throw new DOMException('Aborted', 'AbortError')
    if (sessionLoading.value || messagesLoading.value || modelUpdating.value) {
      throw new Error(studioError('common.loading'))
    }
    if (pendingExchanges.value.has(sessionId)) throw new Error(studioError('studio.errors.unsavedExchange'))
    if (!generationAvailable.value) throw new Error(studioError('studio.errors.noModel'))

    const session = sessions.value.find((item) => item.id === sessionId)
    if (!session) return
    const requestGroupId = session.group_id
    const requestModel = session.model || model.value
    const requestPlatform = groups.value.find((group) => group.id === requestGroupId)?.platform ?? 'openai'
    if (!requestGroupId || !requestModel) {
      throw new Error(studioError(requestGroupId ? 'studio.errors.noModel' : 'studio.errors.noGroup'))
    }

    if (session.mode === 'image') {
      await sendImagePrompt(trimmed, session)
      return
    }

    const exchangeRequestId = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
    const optimisticUser: CreationMessage = {
      id: Date.now(),
      session_id: sessionId,
      role: 'user',
      content: trimmed,
      created_at: new Date().toISOString(),
    }
    messages.value = [...messages.value, optimisticUser]

    stopStreaming()
    const controller = new AbortController()
    streamAbort = controller
    streaming.value = true
    streamingContent.value = ''
    error.value = null

    try {
      const usage: CreationTokenUsage = {}
      const assistantText = await creationAPI.streamCreationChat({
        groupId: requestGroupId,
        sessionId,
        platform: requestPlatform,
        model: requestModel,
        messages: messages.value.filter((msg) => msg.session_id === sessionId && msg.id !== optimisticUser.id),
        userText: trimmed,
        signal: controller.signal,
        onDelta: (delta) => {
          if (streamAbort === controller) {
            streamingContent.value += delta
          }
        },
        onUsage: (value) => { Object.assign(usage, value) },
      })
      assertCurrentEpoch(epoch)
      if (controller.signal.aborted) {
        throw new DOMException('Aborted', 'AbortError')
      }

      const finalAssistant = assistantText || streamingContent.value
      if (!finalAssistant.trim()) throw new Error(studioError('studio.errors.stream'))

      const assistant: CreationMessage = {
        id: -optimisticUser.id,
        session_id: sessionId,
        role: 'assistant',
        content: finalAssistant,
        model: requestModel,
        created_at: new Date().toISOString(),
        ...usage,
      }
      pendingExchanges.value.set(sessionId, {
        request: { request_id: exchangeRequestId, user_content: trimmed, assistant_content: finalAssistant, model: requestModel, ...usage },
        user: optimisticUser,
        assistant,
      })
      if (selectedSessionId.value === sessionId) {
        messages.value = withPendingExchange(messages.value, sessionId)
        streamingContent.value = ''
      }
      await savePendingExchange(sessionId)
      if (epoch === stateEpoch && selectedSessionId.value === sessionId && !pendingExchanges.value.has(sessionId)) {
        try {
          await loadMessages(sessionId)
        } catch {
          error.value = error.value || studioError('studio.errors.loadMessages')
        }
      }
    } catch (err) {
      if (epoch === stateEpoch && selectedSessionId.value === sessionId && !pendingExchanges.value.has(sessionId)) {
        messages.value = messages.value.filter((msg) => msg.id !== optimisticUser.id)
      }
      if (epoch === stateEpoch && selectedSessionId.value === sessionId && !isAbortError(err)) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.send')
      }
      if (epoch === stateEpoch && isAbortError(err) && sessions.value.some((session) => session.id === sessionId)) {
        throw new InterruptedCreationSend()
      }
      throw err
    } finally {
      if (streamAbort === controller) {
        streaming.value = false
        streamingContent.value = ''
        streamAbort = null
      }
    }
  }

  async function sendImagePrompt(prompt: string, session: CreationSession) {
    const epoch = stateEpoch
    const selectionId = selectionRequestId
    const capturedGroupId = session.group_id
    const capturedModel = session.model || model.value
    if (!capturedGroupId || !capturedModel) {
      throw new Error(studioError(capturedGroupId ? 'studio.errors.noModel' : 'studio.errors.noGroup'))
    }

    streaming.value = true
    error.value = null

    const placeholder: CreationImageJob = {
      id: Date.now(),
      session_id: session.id,
      user_id: 0,
      group_id: capturedGroupId,
      status: 'processing',
      model: capturedModel,
      prompt,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    imageTasks.value = [placeholder, ...imageTasks.value]

    try {
      const task = await creationAPI.submitImageGenerationAsync(capturedGroupId, session.id, {
        model: capturedModel,
        prompt,
      })
      assertCurrentEpoch(epoch)

      const mapped = mapAsyncTaskToImageJob(task, session.id, capturedGroupId, capturedModel, prompt)
      imageTasks.value = imageTasks.value.map((item) =>
        item.id === placeholder.id ? { ...mapped, id: placeholder.id } : item,
      )

      if (task.status === 'completed' || task.status === 'failed') {
        if (selectedSessionId.value === session.id) {
          await loadImageTasks(session.id)
        }
        return
      }

      pollImageTask(task.task_id, session.id, placeholder.id, capturedGroupId)
    } catch (err) {
      if (epoch !== stateEpoch) throw err
      imageTasks.value = imageTasks.value.map((item) =>
        item.id === placeholder.id
          ? {
              ...item,
              status: 'failed',
              error: err instanceof Error ? err.message : studioError('studio.errors.generate'),
              updated_at: new Date().toISOString(),
            }
          : item,
      )
      if (selectedSessionId.value === session.id) error.value = err instanceof Error ? err.message : studioError('studio.errors.generate')
      throw err
    } finally {
      if (epoch === stateEpoch && selectionId === selectionRequestId) streaming.value = false
    }
  }

  function pollImageTask(
    taskId: string,
    sessionId: number,
    placeholderId: number,
    pollGroupId: number,
  ) {
    if (imagePollers.has(taskId)) return
    const epoch = stateEpoch

    const poller: ImagePoller = {
      timer: setInterval(() => undefined, IMAGE_POLL_INTERVAL_MS),
      sessionId,
      inFlight: false,
    }
    clearInterval(poller.timer)

    const tick = async () => {
      if (poller.inFlight) return
      poller.inFlight = true
      try {
        const task = await creationAPI.getImageTask(pollGroupId, taskId)
        if (epoch !== stateEpoch || imagePollers.get(taskId) !== poller) return
        const status = task.status
        const mediaUrl = extractImageUrlFromTask(task)

        if (epoch === stateEpoch && selectedSessionId.value === sessionId) {
          imageTasks.value = imageTasks.value.map((item) => {
            if (item.id !== placeholderId) return item
            return {
              ...item,
              status:
                status === 'completed'
                  ? 'completed'
                  : status === 'failed'
                    ? 'failed'
                    : 'processing',
              media_url: mediaUrl ?? item.media_url,
              provider_task_id: task.task_id,
              error: typeof task.error === 'string' ? task.error : item.error,
              updated_at: new Date().toISOString(),
            }
          })
        }

        if (status === 'completed' || status === 'failed') {
          clearInterval(poller.timer)
          imagePollers.delete(taskId)
          if (selectedSessionId.value === sessionId) {
            await loadImageTasks(sessionId)
          }
        }
      } catch (err) {
        if (epoch === stateEpoch && selectedSessionId.value === sessionId && imagePollers.get(taskId) === poller) {
          error.value = err instanceof Error ? err.message : studioError('studio.errors.poll')
        }
      } finally {
        poller.inFlight = false
      }
    }

    poller.timer = setInterval(() => {
      void tick()
    }, IMAGE_POLL_INTERVAL_MS)
    imagePollers.set(taskId, poller)
  }

  async function retryLastFailed() {
    if (streaming.value || sessionLoading.value || modelUpdating.value || saving.value) return
    const epoch = stateEpoch
    const failed = lastFailedSend.value
    if (!failed) return
    lastFailedSend.value = null
    error.value = null
    try {
      if (pendingExchanges.value.has(failed.sessionId)) {
        if (selectedSessionId.value !== failed.sessionId) await selectSession(failed.sessionId)
        assertCurrentEpoch(epoch)
        await savePendingExchange(failed.sessionId)
        if (epoch === stateEpoch) clearMatchingFailure(failed.sessionId, failed.text)
      } else {
        await sendMessage(failed.text, failed.sessionId)
      }
    } catch (err) {
      if (epoch === stateEpoch && (!isAbortError(err) || err instanceof InterruptedCreationSend) && (selectedSessionId.value === failed.sessionId || !lastFailedSend.value)) {
        lastFailedSend.value = failed
      }
    }
  }

  async function initialize() {
    if (initialized.value) return
    if (initialization) return initialization
    const epoch = stateEpoch
    const initialSelectionId = selectionRequestId
    const request = (async () => {
      await loadGroups()
      assertCurrentEpoch(epoch)
      await loadSessions()
      assertCurrentEpoch(epoch)

      const modeSessions = sessions.value.filter((session) => session.mode === sessionModeFilter.value)
      if (initialSelectionId !== selectionRequestId) {
        initialized.value = true
        return
      }
      if (modeSessions.length > 0) {
        await selectSession(modeSessions[0].id)
      } else if (groupId.value) {
        await createSession(sessionModeFilter.value)
      }
      assertCurrentEpoch(epoch)
      initialized.value = true
    })()
    initialization = request
    try {
      await request
    } finally {
      if (initialization === request) initialization = null
    }
  }

  function reset() {
    stateEpoch += 1
    queueRunId += 1
    stopStreaming()
    clearImagePollers()
    sessions.value = []
    sessionsLoading.value = false
    sessionLoading.value = false
    modelUpdates.value.clear()
    pendingExchanges.value.clear()
    savingSessions.value.clear()
    generationAvailable.value = true
    selectedSessionId.value = null
    sessionModeFilter.value = 'chat'
    messages.value = []
    messagesLoading.value = false
    imageTasks.value = []
    imageTasksLoading.value = false
    groups.value = []
    groupId.value = null
    model.value = ''
    chatModels.value = []
    imageModels.value = []
    streaming.value = false
    streamingContent.value = ''
    error.value = null
    pendingQueue.value = []
    lastFailedSend.value = null
    initialized.value = false
    initialization = null
    queueRunning = false
    modelRequestId += 1
    messagesRequestId += 1
    imageTasksRequestId += 1
    selectionRequestId += 1
    sessionsRequestId += 1
  }

  return {
    sessions,
    sessionsLoading,
    sessionLoading,
    modelUpdating,
    generationAvailable,
    hasUnsavedExchange,
    saving,
    selectedSessionId,
    sessionModeFilter,
    messages,
    messagesLoading,
    imageTasks,
    imageTasksLoading,
    groups,
    groupId,
    model,
    chatModels,
    imageModels,
    streaming,
    streamingContent,
    error,
    pendingQueue,
    lastFailedSend,
    selectedSession,
    selectedGroup,
    selectedPlatform,
    isImageSession,
    visibleSessions,
    availableModels,
    hasImageModels,
    initialize,
    loadSessions,
    loadMessages,
    loadImageTasks,
    selectSession,
    createSession,
    deleteSession,
    setGroupId,
    setModel,
    sendMessage,
    enqueueSend,
    submitText,
    retryLastFailed,
    reset,
  }
})
