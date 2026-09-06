<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import UiModal from '@/components/ui/UiModal.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import { getPublicationRequest, MAX_PUBLICATION_BYTES, PUBLICATION_MIME_TYPES, publishMedia, withdrawPublication, type Publication, type PublicationDraft, type PublishableWork } from '../publicationApi'
import { usePublicationText } from '../publicationText'

const props = defineProps<{ show: boolean; work: PublishableWork | null }>()
const emit = defineEmits<{ close: []; published: [publication: Publication] }>()
const auth = useAuthStore()
const text = usePublicationText()
const title = ref('')
const prompt = ref('')
const consent = ref(false)
const busy = ref(false)
const checking = ref(false)
const error = ref('')
const previewUrl = ref('')
const draft = ref<PublicationDraft | null>(null)
const result = ref<Publication | null>(null)
const pendingPublication = ref<Publication | null>(null)
const confirmCleanup = ref(false)
const cleaning = ref(false)
let generation = 0
const userId = computed(() => auth.user?.id)
const storageKey = computed(() => `creation:publication:${userId.value}:${props.work?.id}`)
const validFile = computed(() => !!props.work && props.work.blob instanceof Blob && props.work.blob.size > 0 && props.work.blob.size <= MAX_PUBLICATION_BYTES && PUBLICATION_MIME_TYPES[props.work.kind].includes(props.work.blob.type))

function remember(key: string, value: PublicationDraft) {
  try { localStorage.setItem(key, JSON.stringify(value)) } catch { /* Retry remains idempotent in this opening. */ }
}
function restoreDraft(): PublicationDraft | null {
  try {
    const value = JSON.parse(localStorage.getItem(storageKey.value) || 'null')
    if (value && ['request_id', 'title', 'prompt', 'model'].every(key => typeof value[key] === 'string')) return value
  } catch { /* A malformed local draft must not prevent publishing. */ }
  return null
}
function releasePreview() {
  if (previewUrl.value) URL.revokeObjectURL(previewUrl.value)
  previewUrl.value = ''
}
function isMissing(error: unknown) {
  const value = error as { status?: number; response?: { status?: number } }
  return (value?.status ?? value?.response?.status) === 404
}
function isWithdrawn(error: unknown) {
  const value = error as { status?: number; response?: { status?: number } }
  return (value?.status ?? value?.response?.status) === 410
}
function forgetDraft(key: string) {
  draft.value = null
  try { localStorage.removeItem(key) } catch { /* A later explicit publication replaces the local draft. */ }
}
function acceptStatus(publication: Publication, key: string, notify = false) {
  if (publication.status === 'pending') {
    pendingPublication.value = publication
    result.value = null
    return
  }
  if (publication.withdrawn_at) {
    forgetDraft(key)
    error.value = text('withdrawn')
  } else if (publication.status === 'published' && publication.media_url) {
    pendingPublication.value = null
    result.value = publication
    if (notify) emit('published', publication)
  } else error.value = text('unknown')
}

watch(() => [props.show, props.work, userId.value] as const, async ([show, work]) => {
  const current = ++generation
  releasePreview()
  busy.value = false
  checking.value = false
  error.value = ''
  consent.value = false
  result.value = null
  pendingPublication.value = null
  confirmCleanup.value = false
  cleaning.value = false
  draft.value = null
  title.value = work?.title || ''
  prompt.value = work?.prompt || ''
  if (!show || !work) return
  previewUrl.value = URL.createObjectURL(work.blob)
  draft.value = restoreDraft()
  if (!draft.value) return
  title.value = draft.value.title
  prompt.value = draft.value.prompt
  checking.value = true
  const key = storageKey.value
  try {
    const publication = await getPublicationRequest(draft.value.request_id)
    if (current === generation) acceptStatus(publication, key)
  } catch (cause) {
    if (current !== generation) return
    if (isWithdrawn(cause)) {
      draft.value = null
      try { localStorage.removeItem(key) } catch { /* In-memory draft is already cleared. */ }
    } else if (!isMissing(cause)) error.value = text('unknown')
  } finally {
    if (current === generation) checking.value = false
  }
}, { immediate: true })

async function publish() {
  if (!props.show || !props.work || !userId.value || busy.value || checking.value || result.value || pendingPublication.value?.withdrawn_at) return
  if (!validFile.value) { error.value = text('fileInvalid'); return }
  if (!title.value.trim() || !consent.value) { error.value = text('required'); return }
  const current = generation
  const work = props.work
  const key = storageKey.value
  const payload = draft.value || {
    request_id: crypto.randomUUID(), title: title.value.trim(), prompt: prompt.value, model: work.model || '',
  }
  draft.value = payload
  remember(key, payload)
  busy.value = true
  error.value = ''
  let publication: Publication | undefined
  try {
    publication = await publishMedia(work, payload)
  } catch (cause) {
    try { publication = await getPublicationRequest(payload.request_id) } catch (lookupError) {
      if (current === generation) {
        error.value = isMissing(lookupError) ? extractApiErrorMessage(cause, text('publishFailed')) : text('unknown')
        const status = (cause as { status?: number; response?: { status?: number } })?.status
          ?? (cause as { response?: { status?: number } })?.response?.status
        if (isMissing(lookupError) && status === 400) {
          draft.value = null
          consent.value = false
          try { localStorage.removeItem(key) } catch { /* Validation failed before publication. */ }
        }
      }
    }
  } finally {
    if (current === generation) busy.value = false
  }
  if (current === generation && publication) {
    acceptStatus(publication, key, true)
  }
}

async function cleanupPending() {
  const target = pendingPublication.value
  if (!target || target.owner_user_id !== userId.value || busy.value || checking.value) return
  const current = generation
  const key = storageKey.value
  busy.value = true
  cleaning.value = true
  error.value = ''
  try {
    await withdrawPublication(target.id)
    if (current !== generation) return
    forgetDraft(key)
    pendingPublication.value = null
    consent.value = false
    confirmCleanup.value = false
  } catch (cause) {
    if (current === generation) error.value = extractApiErrorMessage(cause, text('cleanupFailed'))
  } finally {
    if (current === generation) { busy.value = false; cleaning.value = false }
  }
}

function close() {
  if (busy.value || checking.value) return
  generation++
  releasePreview()
  emit('close')
}
onBeforeUnmount(() => { generation++; releasePreview() })
</script>

<template>
  <UiModal :open="show" :title="result ? text('published') : text('publish')" :close-label="text('close')" :show-close="!busy && !checking" :close-on-overlay="!busy && !checking" :close-on-escape="!busy && !checking" width="lg" @close="close">
    <div v-if="work" class="publish-layout">
      <div class="publish-preview">
        <img v-if="work.kind === 'image'" :src="previewUrl" :alt="title || text('preview')" />
        <video v-else :src="previewUrl" controls playsinline preload="metadata" />
      </div>
      <div v-if="result" class="publish-result" role="status">
        <Icon name="check" size="lg" class="text-success" />
        <h3>{{ result.title }}</h3>
        <p class="text-muted">{{ text('published') }}</p>
        <a :href="result.media_url" target="_blank" rel="noopener noreferrer" class="btn-glass-secondary">{{ text('publicLink') }}</a>
      </div>
      <form v-else id="publish-media-form" class="publish-form" @submit.prevent="publish">
        <p v-if="pendingPublication" class="text-warning-text" role="status">{{ pendingPublication.withdrawn_at ? text('pendingWithdrawn') : text('pending') }}</p>
        <p class="publish-notice"><Icon name="globe" size="md" /><span>{{ text('publicNotice') }}</span></p>
        <label class="publish-field"><span>{{ text('title') }}</span><input v-model="title" class="input" required maxlength="160" :disabled="busy || checking || !!draft" /></label>
        <label class="publish-field"><span>{{ text('prompt') }}</span><textarea v-model="prompt" class="input" rows="5" maxlength="16000" :disabled="busy || checking || !!draft" /></label>
        <p v-if="work.model" class="publish-model text-muted">{{ text('model') }}: {{ work.model }}</p>
        <label class="publish-consent"><input v-model="consent" type="checkbox" :disabled="busy || checking" /><span>{{ text('consent') }}</span></label>
        <p v-if="!validFile" class="text-danger-text" role="alert">{{ text('fileInvalid') }}</p>
        <p v-if="error" class="text-danger-text" role="alert">{{ error }}</p>
        <p v-if="checking" class="text-muted" role="status">{{ text('checking') }}</p>
      </form>
    </div>
    <template #footer>
      <button type="button" class="btn-glass-secondary" :disabled="busy || checking" @click="close">{{ result ? text('close') : text('cancel') }}</button>
      <button v-if="pendingPublication && pendingPublication.owner_user_id === userId" type="button" class="btn-glass-secondary" :disabled="busy || checking" @click="confirmCleanup = true">{{ text('cleanup') }}</button>
      <button v-if="!result && !pendingPublication?.withdrawn_at" type="submit" form="publish-media-form" class="btn-glass-primary" :disabled="busy || checking || !userId || !validFile || !consent || !title.trim()"><Icon name="upload" size="sm" />{{ cleaning ? text('withdrawing') : busy ? text('publishing') : pendingPublication ? text('resume') : text('publish') }}</button>
    </template>
  </UiModal>
  <ConfirmDialog :show="confirmCleanup" :title="text('cleanup')" :message="text('cleanupNotice')" :confirm-text="cleaning ? text('withdrawing') : text('cleanup')" :cancel-text="text('cancel')" :confirming="cleaning" tone="danger" @confirm="cleanupPending" @cancel="!cleaning && (confirmCleanup = false)"><p v-if="error" role="alert" class="text-danger-text">{{ error }}</p></ConfirmDialog>
</template>

<style scoped>
.publish-layout { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 24px; }
.publish-preview { display: flex; align-items: center; justify-content: center; aspect-ratio: 1; min-width: 0; background: var(--surface-secondary); border-radius: 8px; overflow: hidden; }
.publish-preview img, .publish-preview video { display: block; width: 100%; height: 100%; max-height: 56vh; object-fit: contain; }
.publish-form, .publish-result { display: flex; flex-direction: column; gap: 16px; min-width: 0; }
.publish-notice, .publish-consent { display: flex; align-items: flex-start; gap: 10px; font-size: 13px; line-height: 1.6; }
.publish-notice { color: var(--muted); }
.publish-notice svg, .publish-consent input { flex-shrink: 0; margin-top: 3px; }
.publish-field { display: flex; flex-direction: column; gap: 6px; font-size: 13px; }
.publish-field textarea { resize: vertical; }
.publish-model, .publish-result h3 { overflow-wrap: anywhere; }
.publish-result h3 { font-size: 18px; }
:deep(.ui-modal-footer) { flex-wrap: wrap; }
@media (max-width: 639px) { .publish-layout { grid-template-columns: minmax(0, 1fr); } .publish-preview { max-height: 240px; aspect-ratio: 16 / 9; } }
</style>
