<!-- Local-first adaptation of chat-vue@80649c38 chat UI; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDown, Code2, Download, GitBranch, HardDrive, Languages, MessageSquare, PanelLeft, Save, Search, Settings2, SquarePen, X } from '@lucide/vue'
import Button from '@/components/ui/Button.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiDrawer from '@/components/ui/UiDrawer.vue'
import UiModal from '@/components/ui/UiModal.vue'
import { useAuthStore } from '@/stores/auth'
import { useIsMobile } from '@/composables/useIsMobile'
import { useChatWorkspace } from '../stores/chatWorkspace'
import { validateChatAttachments } from '../chatApi'
import { getChatReasoningEfforts, supportsChatTemperature } from '../chatCapabilities'
import type { ChatSettings, LocalChatMessage, LocalChatSession } from '../localChat'
import LocalChatComposer from './LocalChatComposer.vue'
import LocalChatMessageView from './LocalChatMessage.vue'
import LocalChatSessions from './LocalChatSessions.vue'
import LegacyChatHistory from './LegacyChatHistory.vue'
import { localChatMessages } from './localChatMessages'

const { t } = useI18n({ useScope: 'local', messages: localChatMessages })
const store = useChatWorkspace()
const auth = useAuthStore()
const { isMobile } = useIsMobile()
const drawerOpen = ref(false)
const legacyOpen = ref(false)
const search = ref('')
const text = ref('')
const files = ref<File[]>([])
const localError = ref('')
const notice = ref('')
const draftLoading = ref(false)
const actionPending = ref(false)
const sendPending = ref(false)
const renameTarget = ref<string | null>(null)
const renameTitle = ref('')
const deleteTarget = ref<string | null>(null)
const settingsOpen = ref(false)
const parameterDraft = ref<ChatSettings>({})
const attachmentPreview = ref<{ file: File; url: string; text?: string } | null>(null)
const composer = ref<InstanceType<typeof LocalChatComposer> | null>(null)
const stream = ref<HTMLElement | null>(null)
const nearBottom = ref(true)
const dragging = ref(false)
let dragDepth = 0
let revision = 0
let disposed = false
let navigation = 0
let previewRevision = 0
let draftTimer: ReturnType<typeof setTimeout> | undefined
const empty = computed(() => !store.messages.length && !store.loading)
const locked = computed(() => store.loading || actionPending.value || draftLoading.value || sendPending.value)
const suggestions = computed(() => [
  { label: t('write'), icon: SquarePen, prompt: t('writePrompt') },
  { label: t('code'), icon: Code2, prompt: t('codePrompt') },
  { label: t('translate'), icon: Languages, prompt: t('translatePrompt') },
  { label: t('think'), icon: MessageSquare, prompt: t('thinkPrompt') },
])
const temperatureAvailable = computed(() => supportsChatTemperature(store.model))
const reasoningOptions = computed(() => getChatReasoningEfforts(store.model, store.selectedSession?.platform || '', store.modelDetails[store.model]).map(value => ({ value, label: t(`effort_${value}`) })))
function fail(error: unknown) { if (!(error instanceof Error && error.name === 'AbortError')) localError.value = error instanceof Error ? error.message : t('error') }
function clearPreview() { previewRevision += 1; if (attachmentPreview.value) URL.revokeObjectURL(attachmentPreview.value.url); attachmentPreview.value = null }
async function previewAttachment(file: File) {
  const current = revision
  clearPreview()
  const preview = previewRevision
  const url = URL.createObjectURL(file)
  try {
    const content = file.type.startsWith('text/') || ['application/json', 'text/csv'].includes(file.type) || /\.(txt|md|markdown|json|csv)$/i.test(file.name) ? await file.text() : undefined
    if (current !== revision || preview !== previewRevision || disposed) { URL.revokeObjectURL(url); return }
    attachmentPreview.value = { file, url, text: content }
  } catch (error) { URL.revokeObjectURL(url); if (current === revision) fail(error) }
}
function addFiles(incoming: File[]) {
  if (store.streaming || locked.value) return
  try { validateChatAttachments([...files.value, ...incoming], store.selectedSession?.platform, store.model, store.modelDetails[store.model]); files.value = [...files.value, ...incoming]; localError.value = '' } catch (error) { fail(error) }
}
function hasFiles(event: DragEvent) { return Array.from(event.dataTransfer?.types || []).includes('Files') }
function onDragEnter(event: DragEvent) { if (hasFiles(event)) { event.preventDefault(); dragDepth += 1; dragging.value = true } }
function onDragOver(event: DragEvent) { if (hasFiles(event)) event.preventDefault() }
function onDragLeave() { dragDepth = Math.max(0, dragDepth - 1); if (!dragDepth) dragging.value = false }
function onDrop(event: DragEvent) { dragging.value = false; dragDepth = 0; const incoming = Array.from(event.dataTransfer?.files || []); if (incoming.length) { event.preventDefault(); addFiles(incoming) } }
async function persistDraft() {
  const current = revision
  const id = store.selectedSessionId
  if (!id || draftLoading.value || !auth.isAuthenticated) return
  try { await store.saveDraft({ text: text.value, files: [...files.value] }, id) } catch (error) { if (!disposed && current === revision) fail(error) }
}
async function syncDraft() {
  const current = ++revision
  clearTimeout(draftTimer)
  draftLoading.value = true
  sendPending.value = false
  actionPending.value = false
  clearPreview()
  renameTarget.value = null; deleteTarget.value = null; settingsOpen.value = false
  try {
    const draft = store.selectedSessionId ? await store.loadDraft(store.selectedSessionId) : null
    if (disposed || current !== revision) return
    text.value = draft?.text || ''
    files.value = [...(draft?.files || [])]
    localError.value = ''
  } catch (error) { if (current === revision) fail(error) }
  finally { await nextTick(); if (current === revision && !disposed) { draftLoading.value = false; void scrollLatest() } }
}
async function send() {
  if (locked.value || store.streaming || (!text.value.trim() && !files.value.length)) return
  const current = revision
  const prompt = text.value
  const attachments = [...files.value]
  sendPending.value = true
  try {
    validateChatAttachments(attachments, store.selectedSession?.platform, store.model, store.modelDetails[store.model])
    localError.value = ''; text.value = ''; files.value = []; nearBottom.value = true
    await store.send({ text: prompt, files: attachments })
  } catch (error) {
    if (current !== revision) return
    fail(error)
    if (!(error instanceof Error && error.name === 'AbortError') && !text.value && !files.value.length) { text.value = prompt; files.value = attachments }
  } finally { if (current === revision) sendPending.value = false }
}
async function stop() { try { await store.stop() } catch (error) { fail(error) } }
async function setGroup(groupId: number) { const current = revision; await persistDraft(); if (current !== revision || disposed) return; try { await store.setParameters({ groupId }) } catch (error) { if (current === revision) fail(error) } }
async function setModel(model: string) { try { await store.setParameters({ model }) } catch (error) { fail(error) } }
async function scrollLatest() { await nextTick(); if (stream.value) stream.value.scrollTop = stream.value.scrollHeight; nearBottom.value = true }
function onScroll() { if (stream.value) nearBottom.value = stream.value.scrollHeight - stream.value.scrollTop - stream.value.clientHeight < 100 }
function downloadBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a'); anchor.href = url; anchor.download = name
  document.body.appendChild(anchor); anchor.click(); anchor.remove(); setTimeout(() => URL.revokeObjectURL(url), 1000)
}
function messageMarkdown(message: LocalChatMessage) { return [`## ${message.role === 'user' ? t('you') : message.model || t('assistant')}`, '', message.content, ...message.files.map(file => `\n[${t('attachment')}: ${file.name}]`), ...(message.toolResults || []).map(result => `\n${result.content}`), ''].join('\n') }
async function messageAction(action: string, message: LocalChatMessage, updatedText?: string) {
  const current = revision
  localError.value = ''
  try {
    if (action === 'copy') { await navigator.clipboard.writeText(message.content); if (current === revision) notice.value = t('copied'); return }
    if (action === 'download') { downloadBlob(new Blob([messageMarkdown(message)], { type: 'text/markdown' }), `${message.role}-${message.id}.md`); return }
    if (action === 'upvote' || action === 'downvote') { const vote = action === 'upvote' ? 'up' : 'down'; await store.vote(message.id, message.vote === vote ? null : vote); return }
    await persistDraft()
    if (current !== revision || disposed) return
    if (action === 'edit' && updatedText?.trim()) await store.editAndRegenerate(message.id, updatedText)
    else if (action === 'regenerate') await store.regenerate(message.id)
  } catch (error) { if (current === revision) fail(error) }
}
async function sessionAction(action: string, session?: LocalChatSession) {
  const current = revision
  localError.value = ''
  try {
    if (action === 'legacy') { legacyOpen.value = true; return }
    if (action === 'rename' && session) { renameTarget.value = session.id; renameTitle.value = session.title; return }
    if (action === 'delete' && session) { deleteTarget.value = session.id; return }
    if (action === 'download' && session) { downloadBlob(new Blob([`# ${session.title}\n\n${session.messages.map(messageMarkdown).join('\n')}`], { type: 'text/markdown' }), `conversation-${session.id}.md`); return }
    const request = ++navigation
    await persistDraft()
    if (current !== revision || request !== navigation || disposed) return
    if (action === 'new') await store.create()
    else if (action === 'select' && session) await store.select(session.id)
    drawerOpen.value = false
  } catch (error) { if (current === revision) fail(error) }
}
async function confirmRename() {
  if (!renameTarget.value || !renameTitle.value.trim()) return
  actionPending.value = true
  try { await store.rename(renameTarget.value, renameTitle.value.trim()); renameTarget.value = null } catch (error) { fail(error) }
  finally { actionPending.value = false }
}
async function confirmDelete() {
  if (!deleteTarget.value) return
  actionPending.value = true
  try { await store.deleteSession(deleteTarget.value); deleteTarget.value = null } catch (error) { fail(error) }
  finally { actionPending.value = false }
}
function openSettings() { localError.value = ''; parameterDraft.value = { ...store.settings }; settingsOpen.value = true }
async function saveSettings() {
  const current = revision
  const settings = { ...parameterDraft.value }
  if (!temperatureAvailable.value) settings.temperature = undefined
  if (settings.temperature !== undefined && (!Number.isFinite(settings.temperature) || settings.temperature < 0 || settings.temperature > 2)) { localError.value = t('invalidTemperature'); return }
  if (settings.maxTokens !== undefined && (!Number.isInteger(settings.maxTokens) || settings.maxTokens < 1 || settings.maxTokens > 131072)) { localError.value = t('invalidMaxTokens'); return }
  actionPending.value = true
  try { await store.setParameters({ settings }); if (current === revision) settingsOpen.value = false } catch (error) { if (current === revision) fail(error) } finally { if (current === revision) actionPending.value = false }
}
function numberSetting(key: 'temperature' | 'maxTokens', event: Event) { const value = (event.target as HTMLInputElement).value; parameterDraft.value = { ...parameterDraft.value, [key]: value === '' ? undefined : Number(value) } }
async function retrySave() { try { await store.saveSession() } catch (error) { fail(error) } }

watch(() => store.selectedSessionId, syncDraft, { immediate: true, flush: 'sync' })
watch([text, files], () => { if (draftLoading.value) return; clearTimeout(draftTimer); draftTimer = setTimeout(() => { void persistDraft() }, 300) }, { deep: true })
watch(() => store.messages.map(message => `${message.id}:${message.content.length}:${message.reasoning?.length || 0}:${message.toolResults?.length || 0}:${message.status}`).join('|'), () => { if (nearBottom.value) void scrollLatest() })
watch(isMobile, () => { drawerOpen.value = false })
watch(() => auth.user?.id, () => { revision += 1; navigation += 1; draftLoading.value = true; sendPending.value = false; actionPending.value = false; clearTimeout(draftTimer); text.value = ''; files.value = []; clearPreview(); drawerOpen.value = false; legacyOpen.value = false; renameTarget.value = null; deleteTarget.value = null; settingsOpen.value = false; localError.value = ''; notice.value = '' }, { flush: 'sync' })
onBeforeUnmount(() => { void persistDraft(); disposed = true; revision += 1; clearTimeout(draftTimer); clearPreview() })
</script>

<template>
  <div class="chat-workspace" @dragenter="onDragEnter" @dragover="onDragOver" @dragleave="onDragLeave" @drop="onDrop">
    <aside v-if="!isMobile" class="chat-history" :aria-label="t('history')"><label class="chat-search"><Search :size="16" /><input v-model="search" type="search" :placeholder="t('search')" :aria-label="t('search')" /></label><LocalChatSessions :sessions="store.sessions" :selected-id="store.selectedSessionId" :search="search" :loading="store.loading" @action="sessionAction" /></aside>
    <section class="chat-conversation" :aria-label="t('title')">
      <header class="chat-conversation-header"><button v-if="isMobile" type="button" class="icon-btn chat-icon-button" :title="t('history')" :aria-label="t('history')" :aria-expanded="drawerOpen" @click="drawerOpen = true"><PanelLeft :size="20" /></button><GitBranch v-if="store.selectedSession?.branchOf" :size="15" :aria-label="t('branch')" /><h2>{{ store.selectedSession?.title || t('title') }}</h2><span class="chat-storage-status"><HardDrive :size="15" /><span>{{ t('local') }}</span></span><button type="button" class="icon-btn chat-icon-button" :disabled="store.streaming || locked" :title="t('settings')" :aria-label="t('settings')" @click="openSettings"><Settings2 :size="17" /></button></header>
      <div v-if="empty" class="chat-welcome"><div class="chat-welcome-content"><MessageSquare :size="36" :stroke-width="1.5" class="chat-welcome-mark" /><h2>{{ t('welcome') }}</h2><div class="chat-suggestions"><button v-for="suggestion in suggestions" :key="suggestion.label" type="button" class="btn btn-secondary" :disabled="locked" @click="text = suggestion.prompt; composer?.focus()"><component :is="suggestion.icon" :size="18" :stroke-width="1.5" /><span>{{ suggestion.label }}</span></button></div></div></div>
      <div v-else ref="stream" class="chat-message-scroll" role="log" aria-live="polite" aria-relevant="additions text" :aria-busy="store.streaming" @scroll="onScroll"><div class="chat-message-content"><LocalChatMessageView v-for="message in store.messages" :key="message.id" :message="message" :busy="store.streaming || locked" @action="messageAction" @preview="previewAttachment" /></div></div>
      <button v-if="!nearBottom && !empty" type="button" class="chat-jump-latest" :aria-label="t('jumpLatest')" :title="t('jumpLatest')" @click="scrollLatest"><ArrowDown :size="17" /></button>
      <footer class="chat-compose-region"><LocalChatComposer ref="composer" v-model="text" :files="files" :groups="store.groups" :models="store.models" :group-id="store.groupId" :model="store.model" :streaming="store.streaming" :loading="locked" :models-loading="store.modelsLoading" @files="addFiles" @remove="files = files.filter((_, index) => index !== $event)" @preview="previewAttachment" @group="setGroup" @model="setModel" @submit="send" @stop="stop" /><div v-if="localError || store.error" class="chat-error" role="alert"><span>{{ localError || store.error }}</span><button v-if="store.selectedSession?.persisted === false" type="button" :title="t('retrySave')" :aria-label="t('retrySave')" @click="retrySave"><Save :size="15" /></button><button type="button" :aria-label="t('close')" @click="localError = ''; store.clearError()"><X :size="15" /></button></div><p v-if="notice" class="chat-notice" role="status">{{ notice }}</p><p class="chat-storage-note">{{ t('storage') }}</p></footer>
    </section>
    <UiDrawer :open="drawerOpen && isMobile" side="left" :title="t('history')" :close-label="t('close')" @close="drawerOpen = false"><div class="chat-drawer-content"><label class="chat-search"><Search :size="16" /><input v-model="search" type="search" :placeholder="t('search')" :aria-label="t('search')" /></label><LocalChatSessions :sessions="store.sessions" :selected-id="store.selectedSessionId" :search="search" :loading="store.loading" @action="sessionAction" /></div></UiDrawer>
    <UiModal :open="Boolean(renameTarget)" :title="t('rename')" width="sm" :close-label="t('close')" @close="renameTarget = null"><label class="chat-dialog-field"><span>{{ t('renameLabel') }}</span><input class="field" v-model="renameTitle" maxlength="200" :aria-label="t('renameLabel')" @keydown.enter.prevent="confirmRename" /></label><template #footer><Button variant="secondary" @click="renameTarget = null">{{ t('cancel') }}</Button><Button :loading="actionPending" :disabled="!renameTitle.trim()" @click="confirmRename">{{ t('save') }}</Button></template></UiModal>
    <UiModal :open="Boolean(deleteTarget)" :title="t('deleteTitle')" width="sm" :close-label="t('close')" @close="deleteTarget = null"><p class="chat-dialog-copy">{{ t('deleteConfirm') }}</p><template #footer><Button variant="secondary" @click="deleteTarget = null">{{ t('cancel') }}</Button><Button variant="danger" :loading="actionPending" @click="confirmDelete">{{ t('remove') }}</Button></template></UiModal>
    <UiModal :open="Boolean(attachmentPreview)" :title="attachmentPreview?.file.name || t('attachment')" width="lg" :close-label="t('close')" @close="clearPreview"><div v-if="attachmentPreview" class="chat-attachment-preview"><img v-if="attachmentPreview.file.type.startsWith('image/')" :src="attachmentPreview.url" :alt="attachmentPreview.file.name" /><pre v-else-if="attachmentPreview.text !== undefined">{{ attachmentPreview.text }}</pre><p v-else>{{ attachmentPreview.file.name }} · {{ Math.ceil(attachmentPreview.file.size / 1024) }} KB</p></div><template #footer><Button v-if="attachmentPreview" variant="secondary" @click="downloadBlob(attachmentPreview.file, attachmentPreview.file.name)"><Download :size="15" />{{ t('download') }}</Button></template></UiModal>
<UiModal :open="settingsOpen" :title="t('settings')" width="sm" :close-label="t('close')" @close="settingsOpen = false"><div class="chat-parameters"><label v-if="temperatureAvailable" class="chat-dialog-field"><span>{{ t('temperature') }}</span><input class="field" :value="parameterDraft.temperature" type="number" min="0" max="2" step="0.1" :placeholder="t('auto')" @input="numberSetting('temperature', $event)" /></label><label class="chat-dialog-field"><span>{{ t('maxTokens') }}</span><input class="field" :value="parameterDraft.maxTokens" type="number" min="1" max="131072" step="1" :placeholder="t('auto')" @input="numberSetting('maxTokens', $event)" /></label><label v-if="reasoningOptions.length" class="chat-dialog-field"><span>{{ t('reasoningEffort') }}</span><UiSelect :model-value="parameterDraft.reasoningEffort || ''" :options="[{ value: '', label: t('auto') }, ...reasoningOptions]" :aria-label="t('reasoningEffort')" @update:model-value="parameterDraft.reasoningEffort = ($event || undefined) as ChatSettings['reasoningEffort']" /></label><label class="chat-dialog-field"><span>{{ t('systemPrompt') }}</span><textarea class="field" v-model="parameterDraft.systemPrompt" rows="5" /></label><p v-if="localError" class="chat-error" role="alert">{{ localError }}</p></div><template #footer><Button variant="secondary" @click="settingsOpen = false">{{ t('cancel') }}</Button><Button :loading="actionPending" @click="saveSettings">{{ t('save') }}</Button></template></UiModal>
    <LegacyChatHistory :open="legacyOpen" @close="legacyOpen = false" @import="legacyOpen = false; drawerOpen = false" />
    <div v-if="dragging" class="chat-drag-overlay"><Download :size="28" /><span>{{ t('drop') }}</span></div>
  </div>
</template>

<style scoped>
.chat-workspace { position: relative; display: grid; grid-template-columns: 212px minmax(0, 1fr); width: 100%; height: 100%; min-height: 0; color: var(--foreground); background: transparent; }
.chat-history { display: flex; flex-direction: column; gap: 14px; min-height: 0; padding: 0 16px 0 0; border-right: 1px solid var(--border); background: transparent; }
.chat-search { display: flex; align-items: center; flex: none; gap: 8px; height: 36px; padding: 0 12px; border: 1px solid var(--border); border-radius: var(--radius-field); color: var(--muted); background: color-mix(in oklch, var(--surface) 85%, transparent); box-shadow: var(--field-shadow); }
.chat-search input { width: 100%; min-width: 0; outline: none; background: transparent; border: 0; color: var(--foreground); font-size: 12px; }
.chat-search:focus-within { border-color: var(--accent); box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent); }
.chat-conversation { position: relative; display: flex; flex-direction: column; min-width: 0; min-height: 0; overflow: hidden; }
.chat-conversation-header { display: flex; flex: none; align-items: center; gap: 10px; min-height: 44px; padding: 0 16px 8px; border-bottom: 1px solid var(--border); }
.chat-conversation-header h2 { flex: 1; min-width: 0; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 14px; font-weight: 600; }
.chat-storage-status { display: inline-flex; flex: none; align-items: center; gap: 6px; color: var(--muted); font-size: 11px; }
.chat-icon-button { width: 34px; height: 34px; }
.chat-icon-button:hover { background: var(--surface-secondary); color: var(--foreground); }
.chat-welcome { display: flex; flex: 1; min-height: 0; overflow: auto; padding: 32px 20px; }
.chat-welcome-content { width: 100%; max-width: 680px; margin: auto; text-align: center; }
.chat-welcome-mark { margin: 0 auto 20px; color: var(--accent); }
.chat-welcome h2 { margin: 0 0 28px; font-size: var(--fs-22); line-height: 1.3; font-weight: var(--fw-bold); }
.chat-suggestions { display: flex; justify-content: center; flex-wrap: wrap; gap: 10px; }
.chat-suggestions button { gap: 6px; }
.chat-suggestions button:hover:not(:disabled) { border-color: var(--accent); background: var(--surface-secondary); }
.chat-suggestions button:disabled { opacity: 0.5; cursor: not-allowed; }
.chat-message-scroll { flex: 1; min-height: 0; overflow: auto; padding: 14px 24px; }
.chat-message-content { max-width: 790px; margin: 0 auto; }
.chat-compose-region { flex: none; width: 100%; max-width: 840px; padding: 12px 20px max(10px, env(safe-area-inset-bottom)); margin: 0 auto; }
.chat-storage-note { margin: 7px 0 0; color: var(--muted); font-size: 10px; text-align: center; }
.chat-drawer-content { display: flex; flex-direction: column; gap: 18px; height: 100%; min-height: 0; }
.chat-error { display: flex; align-items: flex-start; gap: 8px; margin-top: 8px; font-size: 12px; line-height: 1.5; color: var(--danger-text); }
.chat-error span { flex: 1; overflow-wrap: anywhere; }
.chat-error button { border: 0; background: none; color: inherit; padding: 3px; flex-shrink: 0; }
.chat-notice { color: var(--muted); font-size: 11px; margin: 6px 0; text-align: right; }
.chat-jump-latest { position: absolute; right: 25px; bottom: 215px; display: grid; place-items: center; width: 34px; height: 34px; border: 1px solid var(--border); border-radius: 7px; background: var(--surface); color: var(--foreground); box-shadow: var(--shadow-pop); }
.chat-dialog-field { display: flex; flex-direction: column; gap: 8px; font-size: 12px; color: var(--muted); }
.chat-dialog-field .field { width: 100%; }
.chat-dialog-field textarea { min-height: 120px; padding: 12px; resize: vertical; }
.chat-dialog-copy { color: var(--foreground); font-size: 13px; line-height: 1.7; }
.chat-parameters { display: flex; flex-direction: column; gap: 17px; }
.chat-attachment-preview { display: flex; justify-content: center; min-width: 0; }
.chat-attachment-preview img { max-width: 100%; max-height: 65dvh; object-fit: contain; }
.chat-attachment-preview pre { width: 100%; max-height: 65dvh; overflow: auto; white-space: pre-wrap; overflow-wrap: anywhere; color: var(--foreground); font-size: 12px; line-height: 1.6; }
.chat-attachment-preview p { color: var(--muted); font-size: 13px; padding: 24px 0; }
.chat-drag-overlay { position: absolute; inset: 10px; z-index: 30; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 14px; border: 2px dashed var(--accent); border-radius: 8px; background: color-mix(in oklch, var(--surface) 94%, transparent); color: var(--accent); pointer-events: none; }
@media (max-width: 767px) { .chat-workspace { grid-template-columns: minmax(0, 1fr); } .chat-conversation-header { padding: 4px 12px; min-height: 50px; } .chat-storage-status span { display: none; } .chat-welcome { padding: 24px 16px; } .chat-welcome h2 { font-size: 24px; } .chat-suggestions { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); } .chat-suggestions button { justify-content: center; } .chat-compose-region { padding-left: 12px; padding-right: 12px; } .chat-message-scroll { padding: 10px 15px; } }
</style>
