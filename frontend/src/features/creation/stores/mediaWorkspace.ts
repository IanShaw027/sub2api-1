import { defineStore } from 'pinia'
import { computed, onScopeDispose, ref, toRaw, watch } from 'vue'
import { userGroupsAPI } from '@/api/groups'
import { useAuthStore } from '@/stores/auth'
import type { Group } from '@/types'
import { GEMINI_IMAGE_ASPECT_RATIOS, isGeminiImageModel, isImageCapableModel } from '../mediaModels'
import { createLocalMediaId, localMedia, type MediaCost, type MediaDraft, type MediaMode, type MediaRequest, type MediaSettings, type MediaTask } from '../localMedia'
import { imageJobBlob, inspectSourceVideo, mediaAPI, type MediaJob } from '../mediaApi'

const POLL_INTERVAL = 3000
const POLL_TIMEOUT = 60 * 60 * 1000

function abortError(): DOMException {
  return new DOMException('The active account changed or the task was removed.', 'AbortError')
}

function isAbort(error: unknown): boolean {
  return error instanceof Error && error.name === 'AbortError'
}

function message(error: unknown): string {
  return error instanceof Error ? error.message : 'The media operation failed.'
}

function delay(signal: AbortSignal, milliseconds = POLL_INTERVAL): Promise<void> {
  return new Promise((resolve, reject) => {
    if (signal.aborted) return reject(abortError())
    const onAbort = () => { clearTimeout(timer); reject(abortError()) }
    const timer = setTimeout(() => { signal.removeEventListener('abort', onAbort); resolve() }, milliseconds)
    signal.addEventListener('abort', onAbort, { once: true })
  })
}

export const useMediaWorkspace = defineStore('mediaWorkspace', () => {
  const auth = useAuthStore()
  const tasks = ref<MediaTask[]>([])
  const groups = ref<Group[]>([])
  const groupId = ref<number | null>(null)
  const imageModels = ref<string[]>([])
  const videoModels = ref<string[]>([])
  const loading = ref(false)
  const modelsLoading = ref(false)
  const error = ref<string | null>(null)
  const initialized = ref(false)
  const generating = computed(() => tasks.value.some(task => task.status === 'generating'))
  const running = new Map<string, AbortController>()
  const billingRequests = new Map<string, AbortController>()
  const quoteRequests = new Set<AbortController>()
  const objectURLs = new Set<string>()
  let epoch = 0
  let historyLoaded = false
  let initialization: Promise<void> | null = null
  let modelController: AbortController | null = null

  function userId(): number {
    const id = auth.user?.id
    if (!id || !auth.isAuthenticated) throw new Error('Sign in to use the local media workspace.')
    return id
  }

  function assertCurrent(owner: number, started: number): void {
    if (epoch !== started || auth.user?.id !== owner || !auth.isAuthenticated) throw abortError()
  }

  function reset(): void {
    epoch += 1
    running.forEach(controller => controller.abort())
    running.clear()
    billingRequests.forEach(controller => controller.abort())
    billingRequests.clear()
    quoteRequests.forEach(controller => controller.abort())
    quoteRequests.clear()
    modelController?.abort()
    modelController = null
    objectURLs.forEach(url => URL.revokeObjectURL(url))
    objectURLs.clear()
    tasks.value = []
    groups.value = []
    groupId.value = null
    imageModels.value = []
    videoModels.value = []
    error.value = null
    initialized.value = false
    historyLoaded = false
    initialization = null
    loading.value = false
    modelsLoading.value = false
  }

  watch([() => auth.user?.id, () => auth.isAuthenticated], reset, { flush: 'sync' })
  onScopeDispose(reset)

  function taskById(id: string): MediaTask {
    const task = tasks.value.find(item => item.id === id && item.userId === userId())
    if (!task) throw new Error('This local media task is not available for the active account.')
    return task
  }

  function attachURL(task: MediaTask): void {
    if (task.blob && !task.mediaUrl) {
      task.mediaUrl = URL.createObjectURL(task.blob)
      objectURLs.add(task.mediaUrl)
    }
  }

  function snapshot(task: MediaTask): MediaTask {
    return { ...toRaw(task), settings: { ...toRaw(task.settings) }, sourceFiles: [...toRaw(task.sourceFiles)], estimate: task.estimate ? { ...toRaw(task.estimate) } : undefined, billing: task.billing ? { ...toRaw(task.billing) } : undefined }
  }

  const unknownCost = (): MediaCost => ({ status: 'unavailable', amount: null, currency: 'USD', billing_target: null })
  async function refreshBilling(id: string): Promise<void> {
    const task = taskById(id)
    if (!task.providerTaskId || billingRequests.has(id) || ['settled', 'not_billed'].includes(task.billing?.status || '')) return
    const started = epoch
    const controller = new AbortController()
    billingRequests.set(id, controller)
    const current = () => started === epoch && !controller.signal.aborted && billingRequests.get(id) === controller && tasks.value.includes(task)
    try {
      for (let attempt = 0; attempt < 3; attempt++) {
        let cost: MediaCost
        try { cost = await mediaAPI.billing(task.mode, task.groupId, task.providerTaskId, controller.signal) }
        catch { cost = unknownCost() }
        if (!current()) return
        task.billing = cost
        // Cost metadata must never invalidate or delay a saved media result.
        try { await localMedia.saveTask(snapshot(task)) } catch { /* A reload can query the same receipt again. */ }
        if (!current() || cost.status !== 'pending' || attempt === 2) return
        await delay(controller.signal, 2000)
      }
    } catch (failure) {
      if (current() && !isAbort(failure)) task.billing = unknownCost()
    } finally {
      if (billingRequests.get(id) === controller) billingRequests.delete(id)
    }
  }

  async function persist(task: MediaTask, started: number): Promise<void> {
    assertCurrent(task.userId, started)
    try {
      await localMedia.saveTask(snapshot(task))
      assertCurrent(task.userId, started)
      task.persisted = true
    } catch (failure) {
      assertCurrent(task.userId, started)
      task.persisted = false
      error.value = message(failure)
      throw failure
    }
  }

  async function saveTask(id: string): Promise<void> {
    const task = taskById(id)
    const previousError = task.error
    if (task.status === 'completed') task.error = undefined
    try { await persist(task, epoch) } catch (failure) { task.error = previousError; throw failure }
    error.value = null
  }

  async function loadModels(targetGroupId: number): Promise<void> {
    const owner = userId()
    const started = epoch
    modelController?.abort()
    const controller = new AbortController()
    modelController = controller
    groupId.value = targetGroupId
    imageModels.value = []
    videoModels.value = []
    modelsLoading.value = true
    try {
      const response = await mediaAPI.getModels(targetGroupId, controller.signal)
      assertCurrent(owner, started)
      if (controller !== modelController) return
      const platform = groups.value.find(group => group.id === targetGroupId)?.platform
      const models = [...new Set((response.data ?? []).map(model => model.id).filter(Boolean))]
      if (platform === 'gemini') {
        imageModels.value = models.filter(isGeminiImageModel)
      } else if (platform === 'openai' || platform === 'grok' || platform === 'composite') {
        imageModels.value = models.filter(model => isImageCapableModel(model) && !/video/i.test(model) && (!/^gemini-/i.test(model) || isGeminiImageModel(model)))
      }
      if (platform === 'grok' || platform === 'composite') videoModels.value = models.filter(model => /video/i.test(model))
    } catch (failure) {
      if (!controller.signal.aborted && !isAbort(failure) && started === epoch) error.value = message(failure)
      if (!controller.signal.aborted) throw failure
    } finally {
      if (controller === modelController) modelsLoading.value = false
    }
  }

  function active(task: MediaTask, controller: AbortController, started: number): void {
    assertCurrent(task.userId, started)
    if (controller.signal.aborted || running.get(task.id) !== controller) throw abortError()
  }

  async function run(task: MediaTask, initial = false): Promise<void> {
    const started = epoch
    const controller = new AbortController()
    running.set(task.id, controller)
    try {
      let job: MediaJob | null = null
      if (initial) {
        job = await mediaAPI.submit({
          mode: task.mode, prompt: task.prompt, groupId: task.groupId,
          model: task.model, settings: task.settings, files: task.sourceFiles,
          videoOperation: task.videoOperation,
        }, controller.signal)
        active(task, controller, started)
        task.providerTaskId = job.task_id || job.request_id || job.id
        if (!task.providerTaskId) throw new Error('The server did not return a media task ID. Do not retry unless you intend a new generation.')
        if (job.observation_persisted === false) {
          task.observationPersisted = false
          task.observationWarning = job.observation_warning || 'Keep polling this accepted task. Background observation is unavailable; do not submit a new generation.'
        }
        // Save the remote ID before polling, so a reload never resubmits a paid request.
        await persist(task, started)
        active(task, controller, started)
      }
      const deadline = Date.now() + POLL_TIMEOUT
      while (true) {
        active(task, controller, started)
        if (!job) job = await mediaAPI.poll(task.mode, task.groupId, task.providerTaskId!, controller.signal)
        active(task, controller, started)
        if (['failed', 'error', 'cancelled', 'expired'].includes(job.status)) throw new Error(mediaAPI.errorMessage(job.error))
        if (['completed', 'done', 'succeeded', 'success'].includes(job.status)) {
          if (task.mode === 'image') {
            const result = imageJobBlob(job)
            task.blob = result.blob
            task.revisedPrompt = result.revisedPrompt
          } else {
            task.blob = await mediaAPI.videoBlob(task.groupId, task.providerTaskId!, controller.signal)
          }
          active(task, controller, started)
          task.status = 'completed'
          task.completedAt = new Date().toISOString()
          task.error = undefined
          attachURL(task)
          await persist(task, started)
          break
        }
        if (Date.now() > deadline) throw new Error('Media polling timed out. Retry polling before starting another generation.')
        await delay(controller.signal)
        job = null
      }
    } catch (failure) {
      if (isAbort(failure) || started !== epoch || controller.signal.aborted) return
      error.value = message(failure)
      // A quota failure must not discard a successful, still downloadable result.
      if (task.status !== 'completed') task.status = 'error'
      task.error = message(failure)
      try { await persist(task, started) } catch { /* The visible error and persisted=false retain the recovery state. */ }
    } finally {
      if (running.get(task.id) === controller) running.delete(task.id)
      if (started === epoch && !controller.signal.aborted && task.providerTaskId && task.status !== 'generating') void refreshBilling(task.id)
    }
  }

  async function init(): Promise<void> {
    if (initialized.value) return
    if (initialization) return initialization
    const owner = userId()
    const started = epoch
    loading.value = true
    const job = (async () => {
      if (!historyLoaded) {
        const saved = await localMedia.listTasks(owner)
        assertCurrent(owner, started)
        tasks.value = saved
        historyLoaded = true
        for (const task of tasks.value) {
          attachURL(task)
          if (task.providerTaskId && task.status !== 'generating' && !['settled', 'not_billed'].includes(task.billing?.status || '')) void refreshBilling(task.id)
          if (task.status === 'generating') {
            if (task.providerTaskId) void run(task)
            else {
              task.status = 'error'
              task.error = 'Generation was interrupted before a task ID was saved. Retrying will start a new paid request.'
              await persist(task, started)
            }
          }
        }
      }
      const available = await userGroupsAPI.getAvailable()
      assertCurrent(owner, started)
      groups.value = available.filter(group => group.platform === 'openai' || group.platform === 'grok' || group.platform === 'gemini' || group.platform === 'composite')
      if (groups.value.length) await loadModels(groups.value[0]!.id)
      assertCurrent(owner, started)
      initialized.value = true
    })()
    initialization = job
    try { await job } catch (failure) {
      if (!isAbort(failure) && started === epoch) error.value = message(failure)
      throw failure
    } finally {
      if (started === epoch) { loading.value = false; initialization = null }
    }
  }

  async function submit(request: MediaRequest): Promise<MediaTask> {
    const input: MediaRequest = { ...request, settings: { ...request.settings }, files: [...(request.files ?? [])] }
    const owner = userId()
    const started = epoch
    await init()
    assertCurrent(owner, started)
    const group = groups.value.find(item => item.id === input.groupId)
    if (!group || (input.mode === 'video' && group.platform !== 'grok' && group.platform !== 'composite')) throw new Error('Select an available group for this media type.')
    if (!input.prompt.trim() || input.prompt.length > 5000 || !input.model.trim()) throw new Error('A model and a prompt of 1 to 5000 characters are required.')
    const files = input.files ?? []
    const operation = input.videoOperation ?? 'generation'
    if (input.mode === 'video' && !['generation', 'edit', 'extension'].includes(operation)) throw new Error('Unsupported video operation.')
    const videoSource = input.mode === 'video' && operation !== 'generation'
    const geminiImage = input.mode === 'image' && isGeminiImageModel(input.model)
    const limit = input.mode === 'video' ? 1 : geminiImage ? 14 : group.platform === 'grok' || /^grok/i.test(input.model) ? 3 : 8
    if (input.mode === 'image' && group.platform === 'gemini' && !geminiImage) throw new Error('Select a Gemini 3 image model.')
    const geminiRatio = input.settings.ratio || input.settings.aspectRatio || 'auto'
    const geminiSize = input.settings.resolution || '1K'
    if (geminiImage) {
      if (!['1K', '2K', '4K'].includes(geminiSize) || (geminiRatio !== 'auto' && !GEMINI_IMAGE_ASPECT_RATIOS.includes(geminiRatio))) throw new Error('Select a supported Gemini image size and aspect ratio.')
      if (files.some(file => !['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) || files.reduce((sum, file) => sum + file.size, 0) > 19 * 1024 * 1024) throw new Error('Use PNG, JPEG or WebP references totaling at most 19 MiB.')
    }
    let sourceDuration: number | undefined
    if (videoSource) {
      if (files.length !== 1) throw new Error('Select one source MP4 video.')
      if (operation === 'extension' && (!Number.isInteger(input.settings.duration ?? 6) || (input.settings.duration ?? 6) < 2 || (input.settings.duration ?? 6) > 10)) throw new Error('Select an extension duration between 2 and 10 seconds.')
      sourceDuration = await inspectSourceVideo(files[0]!, operation)
      assertCurrent(owner, started)
    } else {
      if (files.length > limit || files.some(file => !/^image\//.test(file.type))) throw new Error(`Select at most ${limit} image files.`)
      if (input.mode === 'video' && (![5, 10, 15].includes(input.settings.duration ?? 5) || !['480p', '720p'].includes(input.settings.resolution ?? '720p'))) throw new Error('Select a supported video duration and resolution.')
    }
    if (input.parentId) taskById(input.parentId)
    const quoteController = new AbortController()
    quoteRequests.add(quoteController)
    let estimate: MediaCost
    try { estimate = await mediaAPI.quote(input, quoteController.signal) }
    catch { estimate = unknownCost() }
    finally { quoteRequests.delete(quoteController) }
    assertCurrent(owner, started)
    const task: MediaTask = {
      id: createLocalMediaId(), userId: owner, mode: input.mode,
      type: files.length ? 'edit' : 'generation', parentId: input.parentId,
      ...(input.mode === 'video' ? { videoOperation: operation, sourceDuration } : {}),
      model: input.model, prompt: input.prompt.trim(), groupId: input.groupId,
      settings: input.mode === 'video'
        ? operation === 'edit' ? {} : operation === 'extension' ? { duration: input.settings.duration ?? 6 } : { duration: input.settings.duration ?? 5, resolution: input.settings.resolution ?? '720p', n: 1 }
        : geminiImage ? { ratio: geminiRatio, aspectRatio: geminiRatio === 'auto' ? undefined : geminiRatio, resolution: geminiSize, n: 1 } : { ...input.settings, n: 1 },
      sourceFiles: [...files],
      createdAt: new Date().toISOString(), status: 'generating', persisted: false,
      estimate,
    }
    await persist(task, started)
    assertCurrent(owner, started)
    tasks.value.unshift(task)
    const reactiveTask = taskById(task.id)
    void run(reactiveTask, true)
    return reactiveTask
  }

  async function retry(id: string): Promise<MediaTask> {
    const task = taskById(id)
    return submit({ mode: task.mode, videoOperation: task.videoOperation, prompt: task.prompt, model: task.model, groupId: task.groupId, settings: { ...task.settings }, files: [...task.sourceFiles], parentId: task.id })
  }

  function resume(id: string): void {
    const task = taskById(id)
    if (!task.providerTaskId || running.has(id) || task.status === 'completed') return
    task.status = 'generating'
    task.error = undefined
    void run(task)
  }

  async function deleteTasks(ids: string[]): Promise<void> {
    const owner = userId()
    const started = epoch
    const selected = ids.map(taskById)
    selected.forEach(task => { running.get(task.id)?.abort(); running.delete(task.id); billingRequests.get(task.id)?.abort(); billingRequests.delete(task.id) })
    try {
      await localMedia.deleteTasks(owner, ids)
      assertCurrent(owner, started)
      selected.forEach(task => {
        if (task.mediaUrl) { URL.revokeObjectURL(task.mediaUrl); objectURLs.delete(task.mediaUrl) }
      })
      const removed = new Set(ids)
      tasks.value = tasks.value.filter(task => !removed.has(task.id))
    } catch (failure) {
      if (started === epoch) {
        error.value = message(failure)
        selected.forEach(task => {
          if (task.status === 'generating') {
            task.status = 'error'
            task.error = task.providerTaskId
              ? 'Local deletion failed. Resume querying this existing task without starting another generation.'
              : 'Local deletion failed and submission was interrupted. The remote task may still run; a retry starts a new generation.'
          }
        })
      }
      throw failure
    }
  }

  async function getTaskBlob(id: string): Promise<Blob> {
    const task = taskById(id)
    if (!task.blob) throw new Error('This task does not have a locally saved result yet.')
    return task.blob
  }

  function getSourceFiles(id: string): File[] {
    return [...taskById(id).sourceFiles]
  }

  async function loadDraft(mode: MediaMode): Promise<MediaDraft | null> {
    const owner = userId()
    const started = epoch
    const draft = await localMedia.loadDraft(owner, mode)
    assertCurrent(owner, started)
    return draft
  }

  async function saveDraft(mode: MediaMode, input: Omit<MediaRequest, 'mode'>): Promise<void> {
    const owner = userId()
    const started = epoch
    try {
      await localMedia.saveDraft({
        ...toRaw(input), settings: { ...toRaw(input.settings) }, files: [...toRaw(input.files ?? [])],
        mode, userId: owner, updatedAt: new Date().toISOString(),
      })
      assertCurrent(owner, started)
    } catch (failure) {
      if (started === epoch) error.value = message(failure)
      throw failure
    }
  }

  async function applyInspiration(input: { kind: MediaMode; prompt: string; model?: string }): Promise<void> {
    const owner = userId()
    const started = epoch
    await init()
    assertCurrent(owner, started)
    const existing = await loadDraft(input.kind)
    assertCurrent(owner, started)
    await saveDraft(input.kind, {
      groupId: existing?.groupId ?? groupId.value ?? 0,
      model: input.model ?? existing?.model ?? (input.kind === 'image' ? imageModels.value[0] : videoModels.value[0]) ?? '',
      settings: existing?.settings ?? {}, files: existing?.files ?? [], parentId: existing?.parentId,
      videoOperation: existing?.videoOperation,
      prompt: input.prompt,
    })
  }

  async function importExistingImage(input: { blob: Blob; prompt: string; model: string; groupId: number; createdAt?: string; settings?: MediaSettings; legacyId?: number }): Promise<MediaTask> {
    const owner = userId()
    const started = epoch
    await init()
    assertCurrent(owner, started)
    const existing = input.legacyId == null ? null : tasks.value.find(task => task.legacyId === input.legacyId)
    if (existing) return existing
    if (!input.blob.size || !input.blob.type.startsWith('image/')) throw new Error('The imported file must be an image.')
    const task: MediaTask = {
      id: createLocalMediaId(), userId: owner, mode: 'image', type: 'generation',
      prompt: input.prompt, model: input.model, groupId: input.groupId, settings: { ...input.settings },
      blob: input.blob, legacyId: input.legacyId, sourceFiles: [],
      createdAt: input.createdAt ?? new Date().toISOString(), completedAt: new Date().toISOString(),
      status: 'completed', persisted: false,
    }
    await persist(task, started)
    assertCurrent(owner, started)
    attachURL(task)
    tasks.value.unshift(task)
    return taskById(task.id)
  }

  return {
    tasks, groups, groupId, imageModels, videoModels, loading, modelsLoading, error, initialized, generating,
    init, loadModels, submit, retry, resume, refreshBilling, deleteTasks, getTaskBlob, getSourceFiles,
    loadDraft, saveDraft, saveTask, applyInspiration, importExistingImage, reset,
    clearError: () => { error.value = null },
  }
})
