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
  CreationMessage,
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

function studioError(key: string): string {
  return String(i18n.global.t(key))
}

function isAbortError(err: unknown): boolean {
  return (
    (err instanceof DOMException && err.name === 'AbortError') ||
    (err instanceof Error && err.name === 'AbortError')
  )
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

  let streamAbort: AbortController | null = null
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
    groups.value = await userGroupsAPI.getAvailable()
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
      error.value = err instanceof Error ? err.message : studioError('studio.errors.loadSessions')
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
        messages.value = items
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
      await loadModelsForGroup(session.group_id)
      if (requestId !== selectionRequestId || selectedSessionId.value !== sessionId) return

      if (session.mode === 'image') {
        await loadImageTasks(sessionId)
      } else {
        await loadMessages(sessionId)
      }
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
    await loadModelsForGroup(groupId.value)
    const defaultModel = pickDefaultModel(
      mode === 'image' ? imageModels.value : chatModels.value,
      model.value,
    )
    const session = await creationAPI.createSession({
      group_id: groupId.value,
      mode,
      model: defaultModel,
      title: mode === 'image' ? 'Image session' : 'New chat',
    })
    sessions.value = [session, ...sessions.value]
    sessionModeFilter.value = mode
    model.value = session.model || defaultModel
    await selectSession(session.id)
    return session
  }

  async function deleteSession(sessionId: number) {
    if (selectedSessionId.value === sessionId) {
      stopStreaming()
      pendingQueue.value = []
      lastFailedSend.value = null
    }
    clearImagePollersForSession(sessionId)
    await creationAPI.deleteSession(sessionId)
    sessions.value = sessions.value.filter((session) => session.id !== sessionId)
    if (selectedSessionId.value === sessionId) {
      selectedSessionId.value = null
      messages.value = []
      imageTasks.value = []
    }
  }

  async function setGroupId(nextGroupId: number | null) {
    if (nextGroupId === groupId.value) return
    stopStreaming()
    clearImagePollers()
    pendingQueue.value = []
    lastFailedSend.value = null
    selectedSessionId.value = null
    messages.value = []
    imageTasks.value = []
    groupId.value = nextGroupId
    if (!nextGroupId) return
    model.value = ''
    await loadModelsForGroup(nextGroupId)
    model.value = pickDefaultModel(
      sessionModeFilter.value === 'image' ? imageModels.value : chatModels.value,
      model.value,
    )
    const mode = sessionModeFilter.value === 'image' && imageModels.value.length === 0
      ? 'chat'
      : sessionModeFilter.value
    sessionModeFilter.value = mode
    await createSession(mode)
  }

  async function setModel(nextModel: string) {
    const session = selectedSession.value
    if (session && modelUpdates.value.has(session.id)) return
    model.value = nextModel
    if (!session) return
    modelUpdates.value.add(session.id)
    try {
      await creationAPI.updateSession(session.id, { model: nextModel })
      session.model = nextModel
      if (selectedSessionId.value === session.id) model.value = nextModel
    } catch (err) {
      if (selectedSessionId.value === session.id) {
        model.value = session.model
        error.value = err instanceof Error ? err.message : studioError('studio.errors.send')
      }
      throw err
    } finally {
      modelUpdates.value.delete(session.id)
    }
  }

  function enqueueSend(sessionId: number, text: string) {
    lastFailedSend.value = null
    pendingQueue.value.push({ sessionId, text })
  }

  async function submitText(text: string) {
    const trimmed = text.trim()
    if (!trimmed || !selectedSessionId.value || sessionLoading.value || messagesLoading.value || modelUpdating.value) return
    enqueueSend(selectedSessionId.value, trimmed)
    if (!queueRunning && !streaming.value) {
      await processQueue()
    }
  }

  async function processQueue() {
    if (queueRunning || pendingQueue.value.length === 0) return
    queueRunning = true
    try {
      while (pendingQueue.value.length > 0) {
        const job = pendingQueue.value[0]
        try {
          await sendMessage(job.text, job.sessionId)
          if (pendingQueue.value[0] === job) {
            pendingQueue.value.shift()
          }
          lastFailedSend.value = null
        } catch (err) {
          lastFailedSend.value = { sessionId: job.sessionId, text: job.text }
          if (pendingQueue.value[0] === job) {
            pendingQueue.value.shift()
          }
          return
        }
      }
    } finally {
      queueRunning = false
    }
  }

  async function sendMessage(text: string, sessionId = selectedSessionId.value) {
    const trimmed = text.trim()
    if (!trimmed || !sessionId) return
    if (sessionId !== selectedSessionId.value) await selectSession(sessionId)
    if (sessionId !== selectedSessionId.value) throw new DOMException('Aborted', 'AbortError')
    if (sessionLoading.value || messagesLoading.value || modelUpdating.value) {
      throw new Error(studioError('common.loading'))
    }

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
      })
      if (controller.signal.aborted) {
        throw new DOMException('Aborted', 'AbortError')
      }

      const finalAssistant = assistantText || streamingContent.value
      if (!finalAssistant.trim()) throw new Error(studioError('studio.errors.stream'))

      await creationAPI.createSessionMessage(sessionId, {
        role: 'user',
        content: trimmed,
      })
      await creationAPI.createSessionMessage(sessionId, {
        role: 'assistant',
        content: finalAssistant,
        model: requestModel,
      })
      if (selectedSessionId.value === sessionId) {
        try {
          await loadMessages(sessionId)
        } catch {
          error.value = error.value || studioError('studio.errors.loadMessages')
        }
      }
    } catch (err) {
      if (selectedSessionId.value === sessionId) {
        messages.value = messages.value.filter((msg) => msg.id !== optimisticUser.id)
      }
      if (!isAbortError(err)) {
        error.value = err instanceof Error ? err.message : studioError('studio.errors.send')
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
      error.value = err instanceof Error ? err.message : studioError('studio.errors.generate')
      throw err
    } finally {
      streaming.value = false
    }
  }

  function pollImageTask(
    taskId: string,
    sessionId: number,
    placeholderId: number,
    pollGroupId: number,
  ) {
    if (imagePollers.has(taskId)) return

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
        const status = task.status
        const mediaUrl = extractImageUrlFromTask(task)

        if (selectedSessionId.value === sessionId) {
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
        if (selectedSessionId.value === sessionId) {
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
    if (streaming.value || sessionLoading.value || modelUpdating.value) return
    const failed = lastFailedSend.value
    if (!failed) return
    lastFailedSend.value = null
    error.value = null
    try {
      await sendMessage(failed.text, failed.sessionId)
    } catch {
      lastFailedSend.value = failed
    }
  }

  async function initialize() {
    if (initialized.value) return
    if (initialization) return initialization
    initialization = (async () => {
      await loadGroups()
      await loadSessions()

      const modeSessions = sessions.value.filter((session) => session.mode === sessionModeFilter.value)
      if (modeSessions.length > 0) {
        await selectSession(modeSessions[0].id)
      } else if (groupId.value) {
        await createSession(sessionModeFilter.value)
      }
      initialized.value = true
    })()
    try {
      await initialization
    } finally {
      initialization = null
    }
  }

  function reset() {
    stopStreaming()
    clearImagePollers()
    sessions.value = []
    sessionsLoading.value = false
    sessionLoading.value = false
    modelUpdates.value.clear()
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
