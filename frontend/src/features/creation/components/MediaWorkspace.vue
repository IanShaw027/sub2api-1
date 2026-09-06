<!-- Adapted from chat-vue@80649c38 studio.vue and useStudioTasks; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ArrowUp, CheckCheck, Clock, Download, Grid2X2, History, Image as ImageIcon, ImagePlus, Lightbulb, List, LoaderCircle, Pencil, Search, SquareCheck, Trash2, Video, X } from '@lucide/vue'
import Button from '@/components/ui/Button.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import UiDrawer from '@/components/ui/UiDrawer.vue'
import UiModal from '@/components/ui/UiModal.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import { useAuthStore } from '@/stores/auth'
import { useMediaWorkspace } from '../stores/mediaWorkspace'
import type { MediaMode, MediaRequest, MediaSettings, MediaTask, VideoOperation } from '../localMedia'
import { inspectSourceVideo } from '../mediaApi'
import { isGeminiImageModel } from '../mediaModels'
import type { PublishableWork } from '../publicationApi'
import PublishMediaDialog from './PublishMediaDialog.vue'
import LegacyImageHistory from './LegacyImageHistory.vue'
import MediaTaskTile from './MediaTaskTile.vue'
import MediaParameters from './MediaParameters.vue'
import MediaPreview from './MediaPreview.vue'
import { mediaMessages } from './mediaMessages'
import { useMediaQuote } from '../useMediaQuote'
import MediaBillingLabel from './MediaBillingLabel.vue'

const props = defineProps<{ mode: MediaMode }>()
const emit = defineEmits<{ 'update:mode': [mode: MediaMode]; inspiration: [] }>()
const { t, locale } = useI18n({ useScope: 'local', messages: mediaMessages })
const route = useRoute()
const auth = useAuthStore()
const store = useMediaWorkspace()
const prompt = ref('')
const model = ref('')
const groupId = ref<number | null>(null)
const settings = ref<MediaSettings>({ ratio: 'auto', resolution: '1K', quality: 'high' })
const parentId = ref<string | undefined>()
const videoOperation = ref<VideoOperation>('generation')
type SourceImage = { id: string; file: File; url: string; name: string }
const sources = ref<SourceImage[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const composerInput = ref<HTMLTextAreaElement | null>(null)
const dock = ref<HTMLElement | null>(null)
const dockHeight = ref(230)
const search = ref('')
const listView = ref(false)
const showAll = ref(false)
const batch = ref(false)
const selection = ref<string[]>([])
const previewId = ref<string | null>(null)
const sourcePreview = ref<{ url: string; name: string; mode?: MediaMode } | null>(null)
const historyPanel = ref<'all' | 'edit' | null>(null)
const legacyOpen = ref(false)
const deleteIds = ref<string[]>([])
const publishing = ref<PublishableWork | null>(null)
const localError = ref('')
const notice = ref('')
const submitting = ref(false)
const deleting = ref(false)
const dragging = ref(false)
const composing = ref(false)
const now = ref(Date.now())
const hydrating = ref(true)
const activeDraftMode = ref<MediaMode>(props.mode)
let dragDepth = 0
let draftRevision = 0
let disposed = false
let saveTimer: ReturnType<typeof setTimeout> | undefined
let clock: ReturnType<typeof setInterval> | undefined
let resizeObserver: ResizeObserver | undefined

const models = computed(() => props.mode === 'video' ? store.videoModels : store.imageModels)
const groupOptions = computed(() => store.groups.map(group => ({ value: group.id, label: group.name })))
const modelOptions = computed(() => models.value.map(value => ({ value, label: value })))
const uploadLimit = computed(() => props.mode === 'video' ? 1 : isGeminiImageModel(model.value) ? 14 : /grok/i.test(model.value) ? 3 : 8)
const { quote, loading: quoteLoading } = useMediaQuote(() => auth.user?.id && groupId.value && model.value ? {
  mode: props.mode, groupId: groupId.value, model: model.value, prompt: '',
  videoOperation: videoOperation.value, settings: { ...settings.value },
} : null)
const needsVideo = computed(() => props.mode === 'video' && videoOperation.value !== 'generation')
const editingTask = computed(() => store.tasks.find(task => task.id === parentId.value) || null)
const editingTaskNumber = computed(() => {
  const task = editingTask.value
  return task ? store.tasks.length - store.tasks.findIndex(item => item.id === task.id) : 0
})
const previewTask = computed(() => store.tasks.find(task => task.id === previewId.value) || null)
const filteredTasks = computed(() => {
  const query = search.value.trim().toLowerCase()
  return store.tasks.filter(task => (showAll.value || task.mode === props.mode) && (!query || `${task.prompt} ${task.model}`.toLowerCase().includes(query)))
})
const selectedTasks = computed(() => store.tasks.filter(task => selection.value.includes(task.id)))
const downloadableSelected = computed(() => selectedTasks.value.filter(task => task.status === 'completed' && task.mediaUrl))
const historyTasks = computed(() => {
  if (historyPanel.value !== 'edit' || !parentId.value) return store.tasks.filter(task => !search.value.trim() || `${task.prompt} ${task.model}`.toLowerCase().includes(search.value.trim().toLowerCase()))
  const result: MediaTask[] = []
  const seen = new Set<string>()
  let current = editingTask.value
  while (current && !seen.has(current.id)) {
    result.unshift(current)
    seen.add(current.id)
    current = store.tasks.find(task => task.id === current?.parentId) || null
  }
  return result
})
const canSubmit = computed(() => !hydrating.value && !submitting.value && !store.modelsLoading && Boolean(prompt.value.trim() && groupId.value && models.value.includes(model.value)) && sources.value.length <= uploadLimit.value && (!needsVideo.value || sources.value.length === 1))
const promptPlaceholder = computed(() => props.mode === 'video' ? t(videoOperation.value === 'edit' ? 'editVideoPrompt' : videoOperation.value === 'extension' ? 'extendVideoPrompt' : sources.value.length ? 'videoEditPrompt' : 'videoPrompt') : t(sources.value.length ? 'imageEditPrompt' : 'imagePrompt'))
const submitLabel = computed(() => props.mode === 'video' ? t(videoOperation.value === 'edit' ? 'editVideo' : videoOperation.value === 'extension' ? 'extendVideo' : sources.value.length ? 'imageToVideo' : 'generateVideo') : t(sources.value.length ? 'editUploaded' : 'generateImage'))
const presets = computed(() => {
  const messages = locale.value.startsWith('zh') ? mediaMessages.zh : mediaMessages.en
  const labels = props.mode === 'video' ? messages.videoPresets : messages.imagePresets
  const prompts = props.mode === 'video' ? messages.videoPresetPrompts : messages.imagePresetPrompts
  return labels.map((label, index) => ({ label, prompt: prompts[index] || '' }))
})

function draft(): Omit<MediaRequest, 'mode'> {
  return { prompt: prompt.value, model: model.value, groupId: groupId.value || 0, settings: { ...settings.value }, files: sources.value.map(source => source.file), parentId: parentId.value, ...(activeDraftMode.value === 'video' ? { videoOperation: videoOperation.value } : {}) }
}
function changeVideoOperation(operation: VideoOperation) {
  if (operation === videoOperation.value) return
  const wasVideo = needsVideo.value
  videoOperation.value = operation
  settings.value = operation === 'edit' ? {} : operation === 'extension' ? { duration: 6 } : { duration: 5, resolution: '720p' }
  if (wasVideo !== needsVideo.value) { replaceSources([]); parentId.value = undefined }
  localError.value = ''
}
function fail(error: unknown, fallback = 'actionError') {
  if (error instanceof Error && error.name === 'AbortError') return
  localError.value = error instanceof Error ? error.message : t(fallback)
}
function replaceSources(files: File[]) {
  sources.value.forEach(source => URL.revokeObjectURL(source.url))
  sources.value = files.map(file => ({ id: globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random()}`, file, url: URL.createObjectURL(file), name: file.name }))
  sourcePreview.value = null
}
function removeSource(id: string) {
  const source = sources.value.find(item => item.id === id)
  if (source) URL.revokeObjectURL(source.url)
  sources.value = sources.value.filter(item => item.id !== id)
  if (sourcePreview.value?.url === source?.url) sourcePreview.value = null
  if (!sources.value.length) parentId.value = undefined
}
async function persistDraft(mode = activeDraftMode.value) {
  if (hydrating.value || !auth.isAuthenticated) return
  try { await store.saveDraft(mode, draft()) } catch (error) { if (!disposed) fail(error) }
}
async function restoreDraft(mode: MediaMode) {
  const revision = ++draftRevision
  hydrating.value = true
  clearTimeout(saveTimer)
  try {
    await store.init()
    const stored = await store.loadDraft(mode)
    if (disposed || revision !== draftRevision) return
    activeDraftMode.value = mode
    prompt.value = typeof route.query.prompt === 'string' ? route.query.prompt : stored?.prompt || ''
    groupId.value = store.groups.some(group => group.id === stored?.groupId) ? stored!.groupId : store.groupId || store.groups[0]?.id || null
    model.value = stored?.model || ''
    videoOperation.value = stored?.videoOperation || 'generation'
    settings.value = stored?.settings || (mode === 'video' ? { duration: 5, resolution: '720p' } : { ratio: 'auto', resolution: '1K', quality: 'high' })
    parentId.value = stored?.parentId
    replaceSources(stored?.files || [])
    if (groupId.value) await store.loadModels(groupId.value)
    if (disposed || revision !== draftRevision) return
    if (!models.value.includes(model.value)) model.value = models.value[0] || ''
  } catch (error) { if (revision === draftRevision && !disposed) fail(error) }
  finally { if (revision === draftRevision) hydrating.value = false }
}
async function changeGroup(value: string | number | boolean | null) {
  if (typeof value !== 'number') return
  groupId.value = value
  localError.value = ''
  try {
    await store.loadModels(value)
    if (groupId.value !== value) return
    if (!models.value.includes(model.value)) model.value = models.value[0] || ''
  } catch (error) { fail(error) }
}
function changeModel(value: string | number | boolean | null) { if (typeof value === 'string') model.value = value }
function resizeInput() {
  const input = composerInput.value
  if (!input) return
  input.style.height = 'auto'
  input.style.height = `${Math.min(input.scrollHeight, 152)}px`
}
function onKeydown(event: KeyboardEvent) {
  if (composing.value || event.isComposing || event.keyCode === 229) return
  if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) { event.preventDefault(); void submit() }
}
async function submit() {
  if (!canSubmit.value || !groupId.value) return
  const revision = draftRevision
  const submittedPrompt = prompt.value.trim()
  const submittedSources = sources.value.map(source => source.id)
  submitting.value = true
  localError.value = ''
  try {
    const task = await store.submit({ ...draft(), mode: props.mode, prompt: submittedPrompt, groupId: groupId.value, settings: { ...settings.value, n: 1 } })
    if (disposed || revision !== draftRevision) return
    quote.value = task?.estimate || null
    if (prompt.value.trim() === submittedPrompt) prompt.value = ''
    submittedSources.forEach(removeSource)
    parentId.value = undefined
    await persistDraft()
    await nextTick()
    resizeInput()
  } catch (error) { if (revision === draftRevision) fail(error) }
  finally { if (revision === draftRevision) submitting.value = false }
}

async function normalizeImage(file: File): Promise<File> {
  if (['image/png', 'image/jpeg', 'image/webp'].includes(file.type) && file.size <= 3 * 1024 * 1024) return file
  const url = URL.createObjectURL(file)
  try {
    const image = await new Promise<HTMLImageElement>((resolve, reject) => { const element = new window.Image(); element.onload = () => resolve(element); element.onerror = reject; element.src = url })
    const scale = Math.min(1, 2560 / Math.max(image.naturalWidth, image.naturalHeight, 1))
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(image.naturalWidth * scale))
    canvas.height = Math.max(1, Math.round(image.naturalHeight * scale))
    const context = canvas.getContext('2d')
    if (!context) throw new Error(t('invalidImage'))
    context.drawImage(image, 0, 0, canvas.width, canvas.height)
    const blob = await new Promise<Blob | null>(resolve => canvas.toBlob(resolve, 'image/jpeg', 0.92))
    if (!blob) throw new Error(t('invalidImage'))
    return new File([blob], `${file.name.replace(/\.[^.]*$/, '') || 'image'}.jpg`, { type: 'image/jpeg' })
  } finally { URL.revokeObjectURL(url) }
}
async function appendImages(files: File[]) {
  const revision = draftRevision
  try {
    const accepted = await Promise.all(files.filter(file => file.type.startsWith('image/') || !file.type).map(normalizeImage))
    if (disposed || revision !== draftRevision) return
    const remaining = Math.max(0, uploadLimit.value - sources.value.length)
    if (accepted.length > remaining) localError.value = t('sourceLimit', { limit: uploadLimit.value })
    sources.value.push(...accepted.slice(0, remaining).map(file => ({ id: globalThis.crypto?.randomUUID?.() || `${Date.now()}-${Math.random()}`, file, url: URL.createObjectURL(file), name: file.name })))
    parentId.value = undefined
  } catch (error) { if (revision === draftRevision) fail(error, 'invalidImage') }
}
async function appendSources(files: File[]) {
  if (!needsVideo.value) return appendImages(files)
  const revision = draftRevision
  const operation = videoOperation.value
  try {
    if (files.length !== 1) throw new Error(t('oneVideo'))
    const file = files[0]!
    await inspectSourceVideo(file, operation)
    if (disposed || revision !== draftRevision || operation !== videoOperation.value) return
    replaceSources([new File([file], file.name, { type: 'video/mp4' })])
    parentId.value = undefined
    localError.value = ''
  } catch (error) { if (revision === draftRevision) fail(error) }
}
function onFiles(event: Event) {
  const input = event.target as HTMLInputElement
  void appendSources(Array.from(input.files || []))
  input.value = ''
}
function onPaste(event: ClipboardEvent) {
  const files = Array.from(event.clipboardData?.files || []).filter(file => file.type.startsWith(needsVideo.value ? 'video/' : 'image/'))
  if (files.length) { event.preventDefault(); void appendSources(files) }
}
function hasImages(event: DragEvent) { return Array.from(event.dataTransfer?.items || []).some(item => item.type.startsWith(needsVideo.value ? 'video/' : 'image/')) }
function onDragEnter(event: DragEvent) { if (hasImages(event)) { event.preventDefault(); dragDepth += 1; dragging.value = true } }
function onDragOver(event: DragEvent) { if (hasImages(event)) event.preventDefault() }
function onDragLeave() { dragDepth = Math.max(0, dragDepth - 1); if (!dragDepth) dragging.value = false }
function onDrop(event: DragEvent) {
  dragDepth = 0; dragging.value = false
  const files = Array.from(event.dataTransfer?.files || []).filter(file => file.type.startsWith(needsVideo.value ? 'video/' : 'image/') || !file.type)
  if (!files.length) return
  event.preventDefault()
  void appendSources(files)
}
function toggleSelection(id: string) { selection.value = selection.value.includes(id) ? selection.value.filter(value => value !== id) : [...selection.value, id] }
function closePreview() { previewId.value = null; sourcePreview.value = null }
async function download(task: MediaTask) {
  const owner = auth.user?.id
  const blob = await store.getTaskBlob(task.id)
  if (disposed || auth.user?.id !== owner) return
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `${task.mode}-${task.id}.${blob.type.includes('webm') ? 'webm' : task.mode === 'video' ? 'mp4' : blob.type.includes('jpeg') ? 'jpg' : blob.type.includes('webp') ? 'webp' : 'png'}`
  document.body.appendChild(anchor); anchor.click(); anchor.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
async function downloadBatch() { for (const task of downloadableSelected.value) { try { await download(task) } catch (error) { fail(error, 'downloadError') } } }
async function setImageSource(task: MediaTask, add: boolean) {
  const revision = draftRevision
  const blob = await store.getTaskBlob(task.id)
  if (revision !== draftRevision || disposed) return
  const file = new File([blob], `source-${task.id}.${blob.type.includes('jpeg') ? 'jpg' : 'png'}`, { type: blob.type })
  if (add) await appendImages([file])
  else { replaceSources([file]); parentId.value = task.id; prompt.value = '' }
  closePreview()
  composerInput.value?.focus()
}
async function onTaskAction(action: string, task: MediaTask) {
  const revision = draftRevision
  const current = () => !disposed && revision === draftRevision
  localError.value = ''
  try {
    if (action === 'billing') return await store.refreshBilling(task.id)
    if (action === 'select') return toggleSelection(task.id)
    if (action === 'preview') { previewId.value = task.id; sourcePreview.value = null; return }
    if (action === 'delete') { deleteIds.value = [task.id]; return }
    if (action === 'download') return await download(task)
    if (action === 'retry') { await store.retry(task.id); return }
    if (action === 'resume') { await store.resume(task.id); return }
    if (action === 'save') { await store.saveTask(task.id); return }
    if (action === 'reuse') { prompt.value = task.prompt; closePreview(); historyPanel.value = null; composerInput.value?.focus(); return }
    if (action === 'editVideo' || action === 'extendVideo') {
      if (task.mode !== 'video' || task.status !== 'completed') return
      const operation = action === 'editVideo' ? 'edit' : 'extension'
      const blob = await store.getTaskBlob(task.id)
      await inspectSourceVideo(blob, operation)
      if (!current()) return
      const input = { prompt: '', videoOperation: operation as VideoOperation, model: task.model, groupId: task.groupId, settings: operation === 'edit' ? {} : { duration: 6 }, files: [new File([blob], `source-${task.id}.mp4`, { type: 'video/mp4' })], parentId: task.id }
      if (props.mode === 'video') {
        videoOperation.value = input.videoOperation
        settings.value = input.settings
        replaceSources(input.files)
        parentId.value = task.id
        prompt.value = ''
        await changeGroup(task.groupId)
        if (!current()) return
        if (models.value.includes(task.model)) model.value = task.model
        await persistDraft()
      } else {
        await store.saveDraft('video', input)
        if (!current()) return
        emit('update:mode', 'video')
      }
      closePreview(); historyPanel.value = null; composerInput.value?.focus(); return
    }
    if (action === 'edit') {
      if (props.mode !== 'image') {
        const blob = await store.getTaskBlob(task.id)
        if (!current()) return
        await store.saveDraft('image', { prompt: '', model: task.model, groupId: task.groupId, settings: { ...task.settings }, files: [new File([blob], 'source.png', { type: blob.type })], parentId: task.id })
        if (!current()) return
        closePreview(); emit('update:mode', 'image'); return
      }
      return await setImageSource(task, false)
    }
    if (action === 'reference') return await setImageSource(task, true)
    if (action === 'video') {
      const blob = await store.getTaskBlob(task.id)
      if (!current()) return
      const existing = await store.loadDraft('video')
      if (!current()) return
      await store.saveDraft('video', { prompt: '', videoOperation: 'generation', model: existing?.model || '', groupId: existing?.groupId || task.groupId, settings: existing?.videoOperation && existing.videoOperation !== 'generation' ? { duration: 5, resolution: '720p' } : existing?.settings || { duration: 5, resolution: '720p' }, files: [new File([blob], 'source.png', { type: blob.type })], parentId: task.id })
      if (!current()) return
      closePreview()
      if (props.mode === 'video') await restoreDraft('video')
      else emit('update:mode', 'video')
      return
    }
    if (action === 'publish') {
      const blob = await store.getTaskBlob(task.id)
      if (!current()) return
      publishing.value = { id: task.id, kind: task.mode, blob, prompt: task.prompt, model: task.model }
      closePreview(); return
    }
    if (action === 'copy') {
      const blob = await store.getTaskBlob(task.id)
      if (!current()) return
      if (!navigator.clipboard?.write || typeof ClipboardItem === 'undefined') throw new Error(t('clipboardError'))
      await navigator.clipboard.write([new ClipboardItem({ [blob.type]: blob })])
    } else if (action === 'copyPrompt' || action === 'copyRevised') {
      await navigator.clipboard.writeText(action === 'copyRevised' ? task.revisedPrompt || '' : task.prompt)
    }
    if (current()) notice.value = t('copied')
  } catch (error) { if (current()) fail(error) }
}
async function confirmDelete() {
  deleting.value = true
  try {
    const ids = [...deleteIds.value]
    await store.deleteTasks(ids)
    selection.value = selection.value.filter(id => !ids.includes(id))
    if (previewId.value && ids.includes(previewId.value)) closePreview()
    if (parentId.value && ids.includes(parentId.value)) { parentId.value = undefined; replaceSources([]) }
    deleteIds.value = []
    if (!selection.value.length) batch.value = false
  } catch (error) { fail(error) }
  finally { deleting.value = false }
}

watch([prompt, model, groupId, settings, sources, parentId, videoOperation], () => {
  if (hydrating.value) return
  clearTimeout(saveTimer)
  saveTimer = setTimeout(() => { void persistDraft() }, 300)
  void nextTick(resizeInput)
}, { deep: true })
watch(uploadLimit, limit => { if (sources.value.length > limit) { sources.value.slice(limit).forEach(source => removeSource(source.id)); localError.value = t('sourceLimit', { limit }) } })
watch(() => props.mode, async mode => {
  await persistDraft(activeDraftMode.value)
  previewId.value = null; batch.value = false; selection.value = []; localError.value = ''; submitting.value = false
  await restoreDraft(mode)
})
watch(() => route.query.prompt, value => { if (typeof value === 'string' && value.trim()) prompt.value = value })
watch(() => auth.user?.id, () => {
  draftRevision += 1; clearTimeout(saveTimer); hydrating.value = true
  replaceSources([]); prompt.value = ''; model.value = ''; groupId.value = null; parentId.value = undefined
  closePreview(); publishing.value = null; selection.value = []; deleteIds.value = []; localError.value = ''
})
onMounted(async () => {
  clock = setInterval(() => { now.value = Date.now() }, 1000)
  if (typeof ResizeObserver !== 'undefined') {
    resizeObserver = new ResizeObserver(() => { dockHeight.value = dock.value?.offsetHeight || 230 })
    if (dock.value) resizeObserver.observe(dock.value)
  }
  await restoreDraft(props.mode)
  await nextTick()
  resizeInput()
})
onBeforeUnmount(() => {
  void persistDraft()
  disposed = true; draftRevision += 1; clearTimeout(saveTimer); clearInterval(clock); resizeObserver?.disconnect()
  replaceSources([])
})
</script>

<template>
  <section class="media-workspace" :aria-label="mode === 'video' ? t('emptyVideos') : t('emptyImages')" @dragenter="onDragEnter" @dragover="onDragOver" @dragleave="onDragLeave" @drop="onDrop">
    <header class="media-workspace-toolbar">
      <div class="media-workspace-title"><span>{{ t('records', { count: filteredTasks.length }) }}</span><span class="media-local-indicator">{{ t('local') }}</span></div>
      <div class="media-workspace-tools">
        <div class="media-search"><Search :size="14" /><input v-model="search" type="search" :placeholder="t('search')" :aria-label="t('search')" /></div>
        <button type="button" class="icon-btn media-icon-button" :class="{ active: showAll }" :title="t('all')" :aria-label="t('all')" :aria-pressed="showAll" @click="showAll = !showAll"><ImageIcon :size="16" /><Video :size="14" /></button>
        <button type="button" class="icon-btn media-icon-button" :title="listView ? t('grid') : t('list')" :aria-label="listView ? t('grid') : t('list')" @click="listView = !listView"><Grid2X2 v-if="listView" :size="17" /><List v-else :size="17" /></button>
        <button type="button" class="icon-btn media-icon-button" :title="t('history')" :aria-label="t('history')" @click="historyPanel = 'all'"><History :size="17" /></button>
        <button type="button" class="icon-btn media-icon-button" :class="{ active: batch }" :title="t('select')" :aria-label="t('select')" :aria-pressed="batch" :disabled="!filteredTasks.length" @click="batch = !batch; selection = []"><SquareCheck :size="17" /></button>
      </div>
    </header>
    <div v-if="batch" class="media-batch-toolbar"><span>{{ t('selected', { count: selection.length }) }}</span><button type="button" :title="t('selectAll')" :aria-label="t('selectAll')" @click="selection = filteredTasks.map(task => task.id)"><CheckCheck :size="16" /></button><button type="button" :disabled="!downloadableSelected.length" :title="t('downloadSelected')" :aria-label="t('downloadSelected')" @click="downloadBatch"><Download :size="16" /></button><button type="button" :disabled="!selection.length" :title="t('removeSelected')" :aria-label="t('removeSelected')" @click="deleteIds = [...selection]"><Trash2 :size="16" /></button><button type="button" :title="t('cancel')" :aria-label="t('cancel')" @click="batch = false; selection = []"><X :size="16" /></button></div>
    <div class="media-workspace-scroll" :style="{ paddingBottom: `${dockHeight + 24}px` }">
      <div v-if="store.loading" class="media-workspace-empty" role="status"><LoaderCircle class="media-loading" :size="30" /><span>{{ t('loading') }}</span></div>
      <div v-else-if="filteredTasks.length" class="media-workspace-grid" :class="{ 'is-list': listView }"><MediaTaskTile v-for="task in filteredTasks" :key="task.id" :task="task" :selected="selection.includes(task.id)" :editing="parentId === task.id" :batch="batch" :list="listView" :now="now" @action="onTaskAction" /></div>
      <div v-else class="media-workspace-empty"><Video v-if="mode === 'video'" :size="38" :stroke-width="1.3" /><ImageIcon v-else :size="38" :stroke-width="1.3" /><h2>{{ search ? t('emptyResults') : mode === 'video' ? t('emptyVideos') : t('emptyImages') }}</h2><button type="button" class="media-inspiration-link" @click="emit('inspiration')"><Lightbulb :size="15" />{{ locale.startsWith('zh') ? '灵感墙' : 'Inspiration' }}</button></div>
    </div>
    <div ref="dock" class="media-composer-dock">
      <div v-if="!prompt.trim() && !sources.length" class="media-preset-row"><button v-for="preset in presets" :key="preset.label" type="button" @click="prompt = preset.prompt; composerInput?.focus()">{{ preset.label }}</button></div>
      <div class="media-composer">
        <SegmentedControl v-if="mode === 'video'" class="media-operation-segments" :model-value="videoOperation" :options="(['generation', 'edit', 'extension'] as const).map(value => ({ value, label: t(value === 'generation' ? 'generateVideo' : value === 'edit' ? 'editVideo' : 'extendVideo'), disabled: hydrating || submitting }))" :aria-label="t('videoOperation')" @update:model-value="changeVideoOperation" />
        <div v-if="editingTask" class="media-editing-context"><button type="button" @click="previewId = editingTask.id"><img v-if="editingTask.mode === 'image' && editingTask.mediaUrl" :src="editingTask.mediaUrl" alt="" /><Video v-else-if="editingTask.mode === 'video'" :size="14" /><Pencil v-else :size="14" /><span>{{ t('editingItem', { number: editingTaskNumber }) }}</span></button><button type="button" :title="t('editHistory')" :aria-label="t('editHistory')" @click="historyPanel = 'edit'"><History :size="15" /></button><button type="button" :title="t('stopEditing')" :aria-label="t('stopEditing')" @click="parentId = undefined; replaceSources([])"><X :size="15" /></button></div>
        <div v-if="sources.length" class="media-source-strip"><div v-for="source in sources" :key="source.id" class="media-source"><button type="button" :aria-label="t('sourcePreview')" @click="sourcePreview = { url: source.url, name: source.name, mode: needsVideo ? 'video' : 'image' }; previewId = null"><video v-if="needsVideo" :src="source.url" muted playsinline preload="metadata" /><img v-else :src="source.url" :alt="source.name" /></button><button type="button" class="media-source-remove" :title="t('removeSource')" :aria-label="t('removeSource')" @click="removeSource(source.id)"><X :size="12" /></button></div></div>
        <textarea ref="composerInput" v-model="prompt" :aria-label="t('prompt')" :placeholder="promptPlaceholder" rows="2" maxlength="5000" :disabled="hydrating || submitting" @input="resizeInput" @paste="onPaste" @keydown="onKeydown" @compositionstart="composing = true" @compositionend="composing = false" />
        <div class="media-composer-controls">
          <UiSelect :model-value="groupId" :options="groupOptions" :placeholder="t('chooseGroup')" :aria-label="t('group')" :disabled="submitting" class="media-group-select" searchable="auto" @update:model-value="changeGroup" />
          <UiSelect :model-value="model" :options="modelOptions" :placeholder="store.modelsLoading ? t('loading') : t('chooseModel')" :aria-label="t('model')" :disabled="store.modelsLoading || submitting" class="media-model-select" searchable="auto" @update:model-value="changeModel" />
          <MediaParameters v-model:settings="settings" :mode="mode" :model="model" :video-operation="videoOperation" :disabled="submitting" />
          <input ref="fileInput" type="file" :accept="needsVideo ? 'video/mp4,.mp4' : 'image/png,image/jpeg,image/webp'" :multiple="mode === 'image'" class="media-file-input" @change="onFiles" />
          <button type="button" class="icon-btn media-icon-button media-upload" :disabled="sources.length >= uploadLimit" :title="t(needsVideo ? 'uploadVideo' : 'upload')" :aria-label="t(needsVideo ? 'uploadVideo' : 'upload')" @click="fileInput?.click()"><Video v-if="needsVideo" :size="17" /><ImagePlus v-else :size="17" /><span v-if="sources.length">{{ sources.length }}/{{ uploadLimit }}</span></button>
          <button type="button" class="btn btn-primary media-submit" :disabled="!canSubmit" :title="submitLabel" :aria-label="submitLabel" :aria-busy="submitting" @click="submit"><LoaderCircle v-if="submitting" class="media-loading" :size="18" /><ArrowUp v-else :size="19" /></button>
        </div>
        <MediaBillingLabel v-if="groupId && model" :cost="quote" :loading="quoteLoading" />
        <div v-if="localError || store.error" class="media-workspace-error" role="alert"><span>{{ localError || store.error }}</span><button type="button" :aria-label="t('close')" @click="localError = ''; store.clearError()"><X :size="14" /></button></div>
        <p v-else-if="!hydrating && !store.modelsLoading && groupId && !models.length" class="media-no-models" role="status">{{ t('noModels') }}</p>
        <p v-if="notice" class="media-notice" role="status" @click="notice = ''">{{ notice }}</p>
      </div>
    </div>
    <div v-if="dragging" class="media-drag-overlay"><Video v-if="needsVideo" :size="32" /><ImagePlus v-else :size="32" /><strong>{{ t(needsVideo ? 'dropVideo' : 'drop') }}</strong></div>
    <MediaPreview :task="previewTask" :source="sourcePreview" @close="closePreview" @action="onTaskAction" />
    <UiDrawer :open="Boolean(historyPanel)" :title="historyPanel === 'edit' ? t('editHistory') : t('history')" :close-label="t('close')" @close="historyPanel = null">
      <div class="media-history-head"><div class="media-search"><Search :size="14" /><input v-model="search" :aria-label="t('search')" :placeholder="t('search')" /></div><button type="button" class="icon-btn media-icon-button" :title="locale.startsWith('zh') ? '旧图片历史' : 'Legacy image history'" :aria-label="locale.startsWith('zh') ? '旧图片历史' : 'Legacy image history'" @click="legacyOpen = true"><Clock :size="17" /></button></div>
      <div class="media-history-list"><article v-for="(task, index) in historyTasks" :key="task.id" :class="{ active: parentId === task.id }"><button type="button" class="media-history-thumb" :aria-label="t('preview')" @click="previewId = task.id"><img v-if="task.mode === 'image' && task.mediaUrl" :src="task.mediaUrl" :alt="task.prompt" /><Video v-else-if="task.mode === 'video'" :size="24" /><ImageIcon v-else :size="24" /></button><div><span v-if="historyPanel === 'edit'" class="media-history-step">{{ t('step', { number: index + 1 }) }}</span><p>{{ task.prompt }}</p><span>{{ task.model }}</span><div class="media-history-actions"><button type="button" :title="t('reuse')" :aria-label="t('reuse')" @click="onTaskAction('reuse', task)"><Pencil :size="14" /></button><button v-if="task.mode === 'image' && task.mediaUrl" type="button" :title="t('setCurrent')" :aria-label="t('setCurrent')" @click="onTaskAction('edit', task)"><ImagePlus :size="14" /></button><button type="button" :title="t('remove')" :aria-label="t('remove')" @click="deleteIds = [task.id]"><Trash2 :size="14" /></button></div></div></article></div>
    </UiDrawer>
    <UiModal :open="deleteIds.length > 0" :title="t('deleteTitle')" width="sm" :close-label="t('close')" @close="!deleting && (deleteIds = [])"><p class="media-delete-copy">{{ t('deleteConfirm', { count: deleteIds.length }) }}</p><template #footer><Button variant="secondary" :disabled="deleting" @click="deleteIds = []">{{ t('cancel') }}</Button><Button variant="danger" :loading="deleting" @click="confirmDelete">{{ t('remove') }}</Button></template></UiModal>
    <LegacyImageHistory :open="legacyOpen" @close="legacyOpen = false" />
    <PublishMediaDialog :show="Boolean(publishing)" :work="publishing" @close="publishing = null" />
  </section>
</template>

<style scoped>
.media-operation-segments { margin-bottom: 12px; max-width: 100%; }
.media-source video { width: 100%; height: 100%; object-fit: cover; }
.media-workspace { position: relative; display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; }
.media-workspace-toolbar { display: flex; justify-content: space-between; align-items: center; gap: 16px; padding: 0 0 14px; flex-shrink: 0; }
.media-workspace-title { display: flex; align-items: baseline; gap: 11px; min-width: 0; flex-wrap: wrap; }
.media-workspace-title h1 { font-size: 18px; font-weight: 700; color: var(--foreground); margin: 0; }
.media-workspace-title > span { font-size: 11px; color: var(--muted); }
.media-local-indicator { padding-left: 10px; border-left: 1px solid var(--border); }
.media-workspace-tools { display: flex; align-items: center; gap: 8px; }
.media-search { display: flex; align-items: center; gap: 8px; color: var(--muted); min-width: 0; height: 36px; padding: 0 12px; border: 1px solid var(--border); border-radius: var(--radius-field); background: color-mix(in oklch, var(--surface) 85%, transparent); box-shadow: var(--field-shadow); }
.media-search:focus-within { border-color: var(--accent); box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent); }
.media-search input { min-width: 0; width: 172px; font-size: 12px; outline: none; background: transparent; border: 0; color: var(--foreground); }
.media-search input::placeholder { color: var(--muted); }
.media-icon-button { gap: 4px; width: 34px; height: 34px; }
.media-icon-button:hover, .media-icon-button.active { background: var(--surface-secondary); color: var(--foreground); }
.media-icon-button:disabled, .media-batch-toolbar button:disabled { opacity: 0.4; cursor: not-allowed; }
.media-workspace-scroll { flex: 1; min-height: 0; overflow: auto; padding: 2px 0; }
.media-workspace-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(216px, 1fr)); gap: 12px; max-width: 1440px; margin: 0 auto; }
.media-workspace-grid.is-list { grid-template-columns: 1fr; max-width: 960px; gap: 10px; }
.media-workspace-empty { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 14px; color: var(--muted); min-height: min(42dvh, 380px); padding: 28px; text-align: center; }
.media-workspace-empty h2 { font-size: 20px; color: var(--foreground); font-weight: 600; }
.media-inspiration-link { display: flex; align-items: center; gap: 6px; padding: 5px 0; border: 0; background: none; color: var(--accent); font-size: 12px; }
.media-batch-toolbar { display: flex; align-items: center; gap: 8px; margin: 0 0 14px; padding: 8px 12px; border-top: 1px solid var(--border); border-bottom: 1px solid var(--border); color: var(--foreground); font-size: 12px; }
.media-batch-toolbar > span { margin-right: auto; }
.media-batch-toolbar button { display: grid; place-items: center; width: 30px; height: 30px; border: 0; border-radius: 5px; background: none; color: var(--foreground); }
.media-composer-dock { position: absolute; bottom: 0; left: 0; right: 0; z-index: 20; display: flex; flex-direction: column; gap: 10px; align-items: center; padding: 14px 0 0; pointer-events: none; }
.media-preset-row { display: flex; justify-content: flex-start; gap: 7px; max-width: min(100%, 850px); overflow-x: auto; pointer-events: auto; scrollbar-width: none; }
.media-preset-row button { flex-shrink: 0; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--muted); font-size: 11px; padding: 6px 9px; }
.media-preset-row button:hover { border-color: var(--accent); color: var(--foreground); }
.media-composer { width: min(100%, 850px); padding: 13px; border: 1px solid var(--border); border-radius: var(--radius-field); background: color-mix(in oklch, var(--surface) 85%, transparent); backdrop-filter: blur(20px); box-shadow: var(--shadow); pointer-events: auto; }
.media-composer textarea { resize: none; outline: none; border: 0; width: 100%; background: none; color: var(--foreground); font-size: 14px; line-height: 1.65; min-height: 58px; max-height: 152px; padding: 3px 4px 10px; overflow-y: auto; }
.media-composer textarea::placeholder { color: var(--muted); }
.media-composer-controls { display: flex; align-items: center; gap: 7px; flex-wrap: wrap; border-top: 1px solid var(--border); padding-top: 10px; }
.media-group-select { flex: 0 1 130px; min-width: 90px; }
.media-model-select { flex: 1 1 180px; min-width: 120px; max-width: 270px; }
.media-model-select { font-family: var(--font-mono); }
.media-upload { flex-shrink: 0; font-size: 10px; }
.media-submit { margin-left: auto; width: 36px; height: 36px; padding: 0; flex-shrink: 0; }
.media-submit:disabled { opacity: 0.4; cursor: not-allowed; }
.media-source-strip { display: flex; gap: 8px; overflow-x: auto; padding: 2px 0 10px; }
.media-source { position: relative; width: 56px; height: 56px; flex-shrink: 0; }
.media-source > button:first-child { width: 100%; height: 100%; padding: 0; border: 1px solid var(--border); border-radius: 7px; overflow: hidden; background: var(--surface-secondary); }
.media-source img { width: 100%; height: 100%; object-fit: cover; }
.media-source-remove { position: absolute; top: 2px; right: 2px; display: grid; place-items: center; width: 20px; height: 20px; border: 0; border-radius: 4px; background: var(--surface); color: var(--foreground); }
.media-editing-context { display: flex; gap: 6px; align-items: center; padding-bottom: 10px; }
.media-editing-context button { display: flex; align-items: center; gap: 6px; border: 0; border-radius: 5px; background: var(--surface-secondary); color: var(--foreground); padding: 5px 6px; font-size: 11px; }
.media-editing-context img { width: 20px; height: 20px; object-fit: cover; border-radius: 3px; }
.media-file-input { display: none; }
.media-workspace-error { display: flex; align-items: flex-start; gap: 8px; font-size: 12px; line-height: 1.5; color: var(--danger-text); padding-top: 9px; }
.media-workspace-error span { flex: 1; overflow-wrap: anywhere; }
.media-workspace-error button { flex-shrink: 0; background: none; border: 0; color: inherit; padding: 2px; }
.media-no-models, .media-notice { color: var(--muted); font-size: 11px; padding-top: 8px; }
.media-drag-overlay { position: absolute; inset: 12px; z-index: 40; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 16px; border: 2px dashed var(--accent); border-radius: 8px; background: color-mix(in oklch, var(--surface) 92%, transparent); color: var(--accent); pointer-events: none; }
.media-history-head { display: flex; align-items: center; gap: 10px; margin-bottom: 20px; }
.media-history-head .media-search { flex: 1; }
.media-history-head input { width: 100%; }
.media-history-list { display: flex; flex-direction: column; gap: 16px; }
.media-history-list article { display: grid; grid-template-columns: 88px minmax(0, 1fr); gap: 12px; padding-bottom: 15px; border-bottom: 1px solid var(--border); }
.media-history-list article.active { border-bottom-color: var(--accent); }
.media-history-thumb { display: grid; place-items: center; height: 88px; padding: 0; border: 0; border-radius: 7px; overflow: hidden; background: var(--surface-secondary); color: var(--muted); }
.media-history-thumb img { width: 100%; height: 100%; object-fit: cover; }
.media-history-list article p { color: var(--foreground); font-size: 12px; line-height: 1.5; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; overflow-wrap: anywhere; }
.media-history-list article span { font-size: 10px; color: var(--muted); overflow-wrap: anywhere; }
.media-history-step { display: block; padding-bottom: 5px; }
.media-history-actions { display: flex; gap: 8px; margin-top: 8px; }
.media-history-actions button { color: var(--muted); border: 0; background: none; padding: 3px; }
.media-delete-copy { color: var(--foreground); font-size: 14px; line-height: 1.7; }
.media-loading { animation: media-loading 1s linear infinite; }
@keyframes media-loading { to { transform: rotate(360deg); } }
@media (max-width: 900px) { .media-workspace-toolbar { align-items: flex-start; flex-direction: column; gap: 12px; } .media-workspace-tools { width: 100%; } .media-search { flex: 1; } .media-search input { width: 100%; } }
@media (max-width: 600px) { .media-workspace-toolbar { padding: 0 0 12px; } .media-workspace-title h1 { font-size: 16px; } .media-workspace-scroll { padding-left: 0; padding-right: 0; } .media-workspace-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; } .media-composer-dock { padding: 10px 0 max(0px, env(safe-area-inset-bottom, 0px)); } .media-composer { padding: 10px; } .media-composer textarea { font-size: 16px; } .media-composer-controls { gap: 8px; } .media-group-select { flex-basis: 100px; } .media-model-select { flex: 1 1 calc(100% - 110px); max-width: none; } .media-workspace-empty { min-height: 24dvh; } .media-local-indicator { display: none; } .media-batch-toolbar { margin: 0 0 12px; } }
@media (max-width: 360px) { .media-workspace-grid { grid-template-columns: 1fr; } }
@media (prefers-reduced-motion: reduce) { .media-loading { animation: none; } }
</style>
