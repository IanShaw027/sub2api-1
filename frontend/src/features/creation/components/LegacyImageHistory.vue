<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiClient } from '@/api/client'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import UiModal from '@/components/ui/UiModal.vue'
import * as creationAPI from '../api'
import { useMediaWorkspace } from '../stores/mediaWorkspace'
import type { CreationImageJob } from '../types'
import { legacyImageHistoryMessages } from './legacyImageHistoryMessages'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; import: [legacyId: number] }>()
const { locale } = useI18n()
const text = computed(() => legacyImageHistoryMessages[locale.value.startsWith('zh') ? 'zh' : 'en'])
const workspace = useMediaWorkspace()
const pageSize = 24
const page = ref(1)
const total = ref(0)
const items = ref<CreationImageJob[]>([])
const query = ref('')
const loading = ref(false)
const error = ref('')
const busyId = ref<number | null>(null)
const importedIds = ref(new Set<number>())
const preview = ref<{ url: string; prompt: string } | null>(null)
let requestVersion = 0
let operationAbort: AbortController | null = null
let historyOwner: string | null = null

const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const filteredItems = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  return items.value.filter((item) => `${item.prompt} ${item.model}`.toLocaleLowerCase().includes(needle))
})

function safeMediaUrl(value?: string): string | undefined {
  if (!value) return undefined
  try {
    const url = new URL(value, window.location.origin)
    if (url.username || url.password) return undefined
    if (url.origin === window.location.origin) {
      return /^\/api\/v1\/media\/public\/\d+$/.test(url.pathname) ? url.href : undefined
    }
    return url.protocol === 'https:' ? url.href : undefined
  } catch {
    return undefined
  }
}

function assertOwner() {
  if (historyOwner !== localStorage.getItem('auth_user')) throw new Error(text.value.identityChanged)
}

async function loadPage(nextPage: number) {
  const version = ++requestVersion
  loading.value = true
  error.value = ''
  try {
    assertOwner()
    const response = await creationAPI.listImages({ page: nextPage, page_size: pageSize })
    if (version !== requestVersion || !props.open) return
    assertOwner()
    items.value = response.items
    page.value = response.page
    total.value = response.total
  } catch {
    if (version === requestVersion) error.value = text.value.loadError
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function clearPreview() {
  preview.value = null
}

async function resolveOriginal(item: CreationImageJob, signal: AbortSignal): Promise<string> {
  assertOwner()
  // Refresh signed object URLs before a user-initiated download; never forward credentials to storage.
  const { data } = await apiClient.get(`/creation/images/${item.id}`, { signal })
  const fresh = creationAPI.mapCreationImageJob(data)
  const url = fresh.status === 'completed' ? safeMediaUrl(fresh.media_url) : undefined
  if (!url) throw new Error(text.value.unavailable)
  assertOwner()
  return url
}

async function readOriginal(url: string, signal: AbortSignal): Promise<Blob> {
  const response = await fetch(url, { signal, credentials: 'omit', referrerPolicy: 'no-referrer' })
  if (!response.ok) throw new Error(text.value.mediaError)
  const blob = await response.blob()
  if (!blob.size || !blob.type.startsWith('image/')) throw new Error(text.value.mediaError)
  assertOwner()
  return blob
}

async function useOriginal(item: CreationImageJob, action: 'preview' | 'download' | 'import') {
  if (busyId.value !== null) return
  busyId.value = item.id
  error.value = ''
  const controller = new AbortController()
  operationAbort = controller
  let importing = false
  try {
    const source = await resolveOriginal(item, controller.signal)
    if (controller.signal.aborted || !props.open) return
    if (action === 'preview') {
      preview.value = { url: source, prompt: item.prompt }
      return
    }
    const blob = await readOriginal(source, controller.signal)
    if (controller.signal.aborted || !props.open) return
    if (action === 'import') {
      importing = true
      await workspace.importExistingImage({
        blob, prompt: item.prompt, model: item.model, groupId: item.group_id,
        createdAt: item.created_at, legacyId: item.id,
      })
      importedIds.value = new Set([...importedIds.value, item.id])
      emit('import', item.id)
    } else {
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      const extension = ({ 'image/jpeg': 'jpg', 'image/webp': 'webp', 'image/gif': 'gif', 'image/avif': 'avif' } as Record<string, string>)[blob.type] ?? 'png'
      anchor.download = `legacy-image-${item.id}.${extension}`
      anchor.click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    }
  } catch (cause) {
    if (!controller.signal.aborted) {
      error.value = cause instanceof Error && cause.message === text.value.identityChanged
        ? cause.message : importing ? text.value.importError : text.value.mediaError
    }
  } finally {
    if (operationAbort === controller) {
      operationAbort = null
      busyId.value = null
    }
  }
}

function cleanup() {
  requestVersion++
  operationAbort?.abort()
  clearPreview()
}

watch(() => props.open, (open) => {
  if (!open) { cleanup(); return }
  const owner = localStorage.getItem('auth_user')
  if (owner !== historyOwner) importedIds.value = new Set()
  historyOwner = owner
  items.value = []
  query.value = ''
  page.value = 1
  total.value = 0
  void loadPage(1)
}, { immediate: true })
onBeforeUnmount(cleanup)
</script>

<template>
  <UiModal :open="open" :title="text.title" :subtitle="text.subtitle" :close-label="text.close" width="xl" @close="emit('close')">
    <div class="legacy-toolbar">
      <input v-model="query" class="field" type="search" :placeholder="text.search" :aria-label="text.search" />
      <Button variant="secondary" :disabled="loading" :title="text.retry" :aria-label="text.retry" @click="loadPage(page)"><Icon name="refresh" /></Button>
    </div>
    <p v-if="error" role="alert" class="legacy-error">{{ error }}</p>
    <p v-if="loading" role="status" class="legacy-empty">{{ text.loading }}</p>
    <p v-else-if="!filteredItems.length" class="legacy-empty">{{ query ? text.noMatches : text.empty }}</p>
    <div v-else class="legacy-grid">
      <article v-for="item in filteredItems" :key="item.id" class="legacy-item" :data-legacy-id="item.id">
        <button class="legacy-thumbnail" type="button" :disabled="item.status !== 'completed' || !safeMediaUrl(item.media_url) || busyId !== null" :aria-label="text.preview" @click="useOriginal(item, 'preview')">
          <img v-if="item.status === 'completed' && safeMediaUrl(item.media_url)" :src="safeMediaUrl(item.media_url)" :alt="item.prompt || text.untitled" loading="lazy" referrerpolicy="no-referrer" />
          <span v-else>{{ item.status === 'completed' ? text.unavailable : text[item.status] }}</span>
        </button>
        <div class="legacy-details">
          <p class="legacy-prompt" :title="item.prompt">{{ item.prompt || text.untitled }}</p>
          <p class="legacy-meta">{{ item.model }} · {{ text.group }} #{{ item.group_id }}</p>
          <time class="legacy-meta" :datetime="item.created_at">{{ new Date(item.created_at).toLocaleString(locale) }}</time>
          <p v-if="item.error" class="legacy-error">{{ item.error }}</p>
          <div class="legacy-actions">
            <Button variant="secondary" size="sm" :title="text.download" :aria-label="text.download" :disabled="item.status !== 'completed' || !safeMediaUrl(item.media_url) || busyId !== null" @click="useOriginal(item, 'download')"><Icon name="download" size="sm" /></Button>
            <Button size="sm" :disabled="item.status !== 'completed' || !safeMediaUrl(item.media_url) || busyId !== null || importedIds.has(item.id)" :loading="busyId === item.id" @click="useOriginal(item, 'import')">
              <Icon :name="importedIds.has(item.id) ? 'check' : 'download'" size="sm" />{{ importedIds.has(item.id) ? text.imported : text.import }}
            </Button>
          </div>
        </div>
      </article>
    </div>
    <template #footer>
      <nav class="legacy-pagination" :aria-label="text.title">
        <Button variant="secondary" :disabled="loading || page <= 1" :title="text.previous" :aria-label="text.previous" @click="loadPage(page - 1)"><Icon name="chevronLeft" /></Button>
        <span>{{ page }} / {{ pages }} · {{ total }}</span>
        <Button variant="secondary" :disabled="loading || page >= pages" :title="text.next" :aria-label="text.next" @click="loadPage(page + 1)"><Icon name="chevronRight" /></Button>
      </nav>
    </template>
  </UiModal>
  <UiModal :open="preview !== null" :title="text.preview" :close-label="text.close" width="xl" @close="clearPreview">
    <img v-if="preview" :src="preview.url" :alt="preview.prompt || text.untitled" class="legacy-preview" referrerpolicy="no-referrer" />
    <template #footer>
      <a v-if="preview" :href="preview.url" target="_blank" rel="noopener noreferrer" class="legacy-original-link"><Icon name="externalLink" size="sm" />{{ text.openOriginal }}</a>
    </template>
  </UiModal>
</template>

<style scoped>
.legacy-toolbar, .legacy-actions, .legacy-pagination { display: flex; align-items: center; gap: 8px; }
.legacy-toolbar { margin-bottom: 16px; }
.legacy-toolbar input { min-width: 0; flex: 1; }
.legacy-grid { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 16px; }
.legacy-item { min-width: 0; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; }
.legacy-thumbnail { width: 100%; aspect-ratio: 1; display: flex; align-items: center; justify-content: center; background: var(--surface-secondary); color: var(--muted); border: 0; padding: 0; }
.legacy-thumbnail img { width: 100%; height: 100%; object-fit: contain; }
.legacy-details { padding: 12px; }
.legacy-prompt { font-size: 13px; color: var(--foreground); overflow-wrap: anywhere; display: -webkit-box; -webkit-box-orient: vertical; -webkit-line-clamp: 3; overflow: hidden; min-height: 3.6em; }
.legacy-meta { display: block; font-size: 11px; color: var(--muted); overflow-wrap: anywhere; margin-top: 4px; }
.legacy-actions { flex-wrap: wrap; margin-top: 12px; }
.legacy-pagination { justify-content: center; font-size: 12px; color: var(--muted); }
.legacy-empty { padding: 32px 0; text-align: center; color: var(--muted); }
.legacy-error { font-size: 12px; color: var(--danger-text); margin: 8px 0; overflow-wrap: anywhere; }
.legacy-preview { display: block; max-width: 100%; max-height: 70vh; object-fit: contain; margin: auto; }
.legacy-original-link { display: inline-flex; align-items: center; gap: 6px; color: var(--accent); font-size: 13px; }
@media (max-width: 767px) { .legacy-grid { grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; } }
@media (max-width: 420px) { .legacy-grid { grid-template-columns: minmax(0, 1fr); } }
</style>
