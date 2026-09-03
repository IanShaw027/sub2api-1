import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { userGroupsAPI } from '@/api/groups'
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
const MAX_SEND_RETRIES = 2

export const useCreationStore = defineStore('creation', () => {
  const sessions = ref<CreationSession[]>([])
  const sessionsLoading = ref(false)
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
  const initialized = ref(false)

  let streamAbort: AbortController | null = null
  const imagePollers = new Map<string, ReturnType<typeof setInterval>>

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

  function clearImagePollers() {
    for (const timer of imagePollers.values()) {
      clearInterval(timer)
    }
    imagePollers.clear()
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
    const list = await creationAPI.getModels(targetGroupId)
    const classified = classifyModels(list.data ?? [])
    chatModels.value = classified.chat
    imageModels.value = classified.image
    if (!model.value) {
      model.value = pickDefaultModel(
        isImageSession.value ? imageModels.value : chatModels.value,
        selectedSession.value?.model,
      )
    }
  }

  async function loadSessions() {
    sessionsLoading.value = true
    error.value = null
    try {
      const response = await creationAPI.listSessions({ page: 1, page_size: 100 })
      sessions.value = response.items ?? []
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'loadSessions failed'
      throw err
    } finally {
      sessionsLoading.value = false
    }
  }

  async function loadMessages(sessionId: number) {
    messagesLoading.value = true
    error.value = null
    try {
      messages.value = await creationAPI.listSessionMessages(sessionId)
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'loadMessages failed'
      throw err
    } finally {
      messagesLoading.value = false
    }
  }

  async function loadImageTasks(sessionId: number) {
    imageTasksLoading.value = true
    error.value = null
    try {
      const response = await creationAPI.listImages({ session_id: sessionId, page: 1, page_size: 100 })
      imageTasks.value = response.items ?? []
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'loadImageTasks failed'
      throw err
    } finally {
      imageTasksLoading.value = false
    }
  }

  async function selectSession(sessionId: number) {
    selectedSessionId.value = sessionId
    const session = sessions.value.find((item) => item.id === sessionId)
    if (!session) return

    groupId.value = session.group_id
    model.value = session.model || model.value
    await loadModelsForGroup(session.group_id)

    if (session.mode === 'image') {
      messages.value = []
      await loadImageTasks(sessionId)
    } else {
      imageTasks.value = []
      await loadMessages(sessionId)
    }
  }

  async function createSession(mode: CreationSessionMode) {
    if (!groupId.value) throw new Error('group required')
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
    await creationAPI.deleteSession(sessionId)
    sessions.value = sessions.value.filter((session) => session.id !== sessionId)
    if (selectedSessionId.value === sessionId) {
      selectedSessionId.value = null
      messages.value = []
      imageTasks.value = []
    }
  }

  async function setGroupId(nextGroupId: number | null) {
    if (selectedSessionId.value) return
    groupId.value = nextGroupId
    if (!nextGroupId) return
    await loadModelsForGroup(nextGroupId)
    model.value = pickDefaultModel(
      isImageSession.value ? imageModels.value : chatModels.value,
      model.value,
    )
  }

  async function setModel(nextModel: string) {
    model.value = nextModel
    if (selectedSession.value) {
      await creationAPI.updateSession(selectedSession.value.id, { model: nextModel })
      selectedSession.value.model = nextModel
    }
  }

  function enqueueSend(sessionId: number, text: string) {
    pendingQueue.value.push({ sessionId, text, retryCount: 0 })
    if (!streaming.value) {
      processQueue()
    }
  }

  async function processQueue() {
    if (streaming.value || pendingQueue.value.length === 0) return
    const job = pendingQueue.value[0]
    try {
      await sendMessage(job.text, job.sessionId)
      pendingQueue.value.shift()
    } catch {
      job.retryCount += 1
      if (job.retryCount > MAX_SEND_RETRIES) {
        pendingQueue.value.shift()
      }
    }
    if (pendingQueue.value.length > 0) {
      await processQueue()
    }
  }

  async function sendMessage(text: string, sessionId = selectedSessionId.value) {
    const trimmed = text.trim()
    if (!trimmed || !sessionId) return

    const session = sessions.value.find((item) => item.id === sessionId)
    if (!session) return
    if (!groupId.value || !model.value) throw new Error('group and model required')

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
    streamAbort = new AbortController()
    streaming.value = true
    streamingContent.value = ''
    error.value = null

    try {
      const assistantText = await creationAPI.streamCreationChat({
        groupId: groupId.value,
        sessionId,
        platform: selectedPlatform.value ?? 'openai',
        model: model.value,
        messages: messages.value.filter((msg) => msg.id !== optimisticUser.id),
        userText: trimmed,
        signal: streamAbort.signal,
        onDelta: (delta) => {
          streamingContent.value += delta
        },
      })

      const finalAssistant = assistantText || streamingContent.value
      messages.value = [
        ...messages.value,
        {
          id: Date.now() + 1,
          session_id: sessionId,
          role: 'assistant',
          content: finalAssistant,
          model: model.value,
          created_at: new Date().toISOString(),
        },
      ]

      await creationAPI.createSessionMessage(sessionId, {
        role: 'user',
        content: trimmed,
      })
      await creationAPI.createSessionMessage(sessionId, {
        role: 'assistant',
        content: finalAssistant,
        model: model.value,
      })
    } catch (err) {
      if (!(err instanceof DOMException && err.name === 'AbortError')) {
        error.value = err instanceof Error ? err.message : 'send failed'
        throw err
      }
    } finally {
      streaming.value = false
      streamingContent.value = ''
      streamAbort = null
    }
  }

  async function sendImagePrompt(prompt: string, session: CreationSession) {
    if (!groupId.value || !model.value) throw new Error('group and model required')

    const placeholder: CreationImageJob = {
      id: Date.now(),
      session_id: session.id,
      user_id: 0,
      group_id: groupId.value,
      status: 'processing',
      model: model.value,
      prompt,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    }
    imageTasks.value = [placeholder, ...imageTasks.value]

    const task = await creationAPI.submitImageGenerationAsync(groupId.value, session.id, {
      model: model.value,
      prompt,
    })

    const mapped = mapAsyncTaskToImageJob(task, session.id, groupId.value, model.value, prompt)
    imageTasks.value = imageTasks.value.map((item) =>
      item.id === placeholder.id ? { ...mapped, id: placeholder.id } : item,
    )

    pollImageTask(task.task_id, session.id, placeholder.id)
  }

  function pollImageTask(taskId: string, sessionId: number, placeholderId: number) {
    if (!groupId.value) return
    if (imagePollers.has(taskId)) return

    const timer = setInterval(async () => {
      try {
        const task = await creationAPI.getImageTask(groupId.value!, taskId)
        const status = task.status
        const mediaUrl = extractImageUrlFromTask(task)

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

        if (status === 'completed' || status === 'failed') {
          clearInterval(timer)
          imagePollers.delete(taskId)
          await loadImageTasks(sessionId)
        }
      } catch (err) {
        clearInterval(timer)
        imagePollers.delete(taskId)
        error.value = err instanceof Error ? err.message : 'poll failed'
      }
    }, IMAGE_POLL_INTERVAL_MS)

    imagePollers.set(taskId, timer)
  }

  async function retryLastFailed() {
    const failed = pendingQueue.value[0]
    if (!failed) return
    await processQueue()
  }

  async function initialize() {
    if (initialized.value) return
    initialized.value = true
    await loadGroups()
    await loadSessions()

    const modeSessions = sessions.value.filter((session) => session.mode === sessionModeFilter.value)
    if (modeSessions.length > 0) {
      await selectSession(modeSessions[0].id)
      return
    }

    if (groupId.value) {
      await createSession(sessionModeFilter.value)
    }
  }

  function reset() {
    stopStreaming()
    clearImagePollers()
    sessions.value = []
    sessionsLoading.value = false
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
    initialized.value = false
  }

  return {
    sessions,
    sessionsLoading,
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
    selectedSession,
    selectedGroup,
    selectedPlatform,
    isImageSession,
    visibleSessions,
    availableModels,
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
    retryLastFailed,
    reset,
  }
})
