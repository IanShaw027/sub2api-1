<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowDownToLine, ChevronLeft, ChevronRight, Download, MessageSquare, RefreshCw } from '@lucide/vue'
import { useAuthStore } from '@/stores/auth'
import Button from '@/components/ui/Button.vue'
import UiModal from '@/components/ui/UiModal.vue'
import * as creationAPI from '../api'
import { useChatWorkspace } from '../stores/chatWorkspace'
import type { CreationSession } from '../types'
import { legacyChatHistoryMessages } from './legacyChatHistoryMessages'

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: []; import: [localSessionId: string] }>()
const { locale } = useI18n()
const auth = useAuthStore()
const workspace = useChatWorkspace()
const text = computed(() => legacyChatHistoryMessages[locale.value.startsWith('zh') ? 'zh' : 'en'])
const pageSize = 20
const page = ref(1)
const total = ref(0)
const query = ref('')
const sessions = ref<CreationSession[]>([])
const loading = ref(false)
const error = ref('')
const busyId = ref<number | null>(null)
const importedIds = ref(new Set<number>())
let ownerId: number | null = null
let lifecycleVersion = 0
let listVersion = 0
let operationVersion = 0
let operationAbort: AbortController | null = null

const pages = computed(() => Math.max(1, Math.ceil(total.value / pageSize)))
const visibleSessions = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  return sessions.value.filter((session) => `${session.title} ${session.model}`.toLocaleLowerCase().includes(needle))
})

function currentOwner(): number | null {
  try {
    const stored = JSON.parse(localStorage.getItem('auth_user') || 'null') as { id?: unknown } | null
    const id = auth.user?.id
    if (!id || !Number.isSafeInteger(id) || id <= 0 || stored?.id !== id) return null
    return id
  } catch {
    return null
  }
}

function assertOwner() {
  if (!ownerId || currentOwner() !== ownerId) throw new Error(text.value.identityChanged)
}

function displayError(cause: unknown, fallback: string): string {
  if (cause instanceof Error && [text.value.identityChanged, text.value.invalidOwner, text.value.invalidMessages].includes(cause.message)) return cause.message
  return fallback
}

async function loadPage(nextPage: number) {
  const lifecycle = lifecycleVersion
  const version = ++listVersion
  loading.value = true
  error.value = ''
  try {
    assertOwner()
    const response = await creationAPI.listSessions({ mode: 'chat', page: nextPage, page_size: pageSize })
    if (!props.open || lifecycle !== lifecycleVersion || version !== listVersion) return
    assertOwner()
    if (response.items.some((session) => session.user_id !== ownerId || session.mode !== 'chat')) throw new Error(text.value.invalidOwner)
    sessions.value = response.items
    page.value = response.page
    total.value = response.total
  } catch (cause) {
    if (lifecycle === lifecycleVersion && version === listVersion) error.value = displayError(cause, text.value.loadError)
  } finally {
    if (lifecycle === lifecycleVersion && version === listVersion) loading.value = false
  }
}

async function useConversation(session: CreationSession, action: 'download' | 'import') {
  if (busyId.value !== null) return
  const lifecycle = lifecycleVersion
  const operation = ++operationVersion
  const controller = new AbortController()
  operationAbort = controller
  busyId.value = session.id
  error.value = ''
  try {
    assertOwner()
    if (session.user_id !== ownerId || session.mode !== 'chat') throw new Error(text.value.invalidOwner)
    const messages = await creationAPI.listSessionMessages(session.id)
    if (!props.open || lifecycle !== lifecycleVersion || operation !== operationVersion) return
    assertOwner()
    if (!Array.isArray(messages) || messages.some((message) => message.session_id !== session.id)) throw new Error(text.value.invalidMessages)
    if (action === 'import') {
      const local = await workspace.importLegacyConversation({ session, messages, signal: controller.signal })
      if (!props.open || lifecycle !== lifecycleVersion || operation !== operationVersion) return
      assertOwner()
      importedIds.value = new Set([...importedIds.value, session.id])
      emit('import', local.id)
    } else {
      const data = JSON.stringify({
        format: 'sub2api-legacy-chat', version: 1, exported_at: new Date().toISOString(), session, messages,
      }, null, 2)
      const url = URL.createObjectURL(new Blob([data], { type: 'application/json' }))
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `legacy-chat-${session.id}.json`
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    }
  } catch (cause) {
    if (lifecycle === lifecycleVersion && operation === operationVersion) {
      error.value = displayError(cause, action === 'import' ? text.value.importError : text.value.downloadError)
    }
  } finally {
    if (operation === operationVersion) {
      busyId.value = null
      operationAbort = null
    }
  }
}

function invalidate() {
  operationAbort?.abort()
  operationAbort = null
  lifecycleVersion++
  listVersion++
  operationVersion++
  loading.value = false
  busyId.value = null
  sessions.value = []
}

watch(() => props.open, (open) => {
  invalidate()
  if (!open) return
  const nextOwner = currentOwner()
  if (nextOwner !== ownerId) importedIds.value = new Set()
  ownerId = nextOwner
  query.value = ''
  page.value = 1
  total.value = 0
  void loadPage(1)
}, { immediate: true })

watch(() => auth.user?.id, () => {
  if (!props.open || currentOwner() === ownerId) return
  invalidate()
  importedIds.value = new Set()
  error.value = text.value.identityChanged
})
onBeforeUnmount(invalidate)
</script>

<template>
  <UiModal :open="open" :title="text.title" :subtitle="text.subtitle" :close-label="text.close" width="lg" @close="emit('close')">
    <div class="legacy-chat-toolbar">
      <input v-model="query" class="field" type="search" :placeholder="text.search" :aria-label="text.search" />
      <Button variant="secondary" :disabled="loading" :title="text.retry" :aria-label="text.retry" @click="loadPage(page)"><RefreshCw :size="16" aria-hidden="true" /></Button>
    </div>
    <p v-if="error" role="alert" class="legacy-chat-error">{{ error }}</p>
    <p v-if="loading" role="status" class="legacy-chat-empty">{{ text.loading }}</p>
    <p v-else-if="!visibleSessions.length" class="legacy-chat-empty">{{ query ? text.noMatches : text.empty }}</p>
    <ul v-else class="legacy-chat-list">
      <li v-for="session in visibleSessions" :key="session.id" class="legacy-chat-row" :data-legacy-chat-id="session.id">
        <MessageSquare :size="20" class="legacy-chat-icon" aria-hidden="true" />
        <div class="legacy-chat-detail">
          <h3>{{ session.title || text.untitled }}</h3>
          <p>{{ session.model }} · {{ text.group }} #{{ session.group_id }}<span v-if="session.status === 'archived'"> · {{ text.archived }}</span></p>
          <time :datetime="session.created_at">{{ new Date(session.created_at).toLocaleString(locale) }}</time>
        </div>
        <div class="legacy-chat-actions">
          <Button variant="secondary" size="sm" :disabled="busyId !== null" :title="text.download" :aria-label="text.download" @click="useConversation(session, 'download')"><Download :size="16" aria-hidden="true" /></Button>
          <Button size="sm" :disabled="busyId !== null || importedIds.has(session.id)" :loading="busyId === session.id" @click="useConversation(session, 'import')"><ArrowDownToLine :size="16" aria-hidden="true" />{{ importedIds.has(session.id) ? text.imported : text.import }}</Button>
        </div>
      </li>
    </ul>
    <template #footer>
      <nav class="legacy-chat-pagination" :aria-label="text.title">
        <Button variant="secondary" :disabled="loading || page <= 1" :title="text.previous" :aria-label="text.previous" @click="loadPage(page - 1)"><ChevronLeft :size="16" aria-hidden="true" /></Button>
        <span>{{ page }} / {{ pages }} · {{ total }}</span>
        <Button variant="secondary" :disabled="loading || page >= pages" :title="text.next" :aria-label="text.next" @click="loadPage(page + 1)"><ChevronRight :size="16" aria-hidden="true" /></Button>
      </nav>
    </template>
  </UiModal>
</template>

<style scoped>
.legacy-chat-toolbar, .legacy-chat-actions, .legacy-chat-pagination { display: flex; align-items: center; gap: 8px; }
.legacy-chat-toolbar { margin-bottom: 16px; }
.legacy-chat-toolbar input { flex: 1; min-width: 0; }
.legacy-chat-list { margin: 0; padding: 0; list-style: none; }
.legacy-chat-row { display: flex; align-items: flex-start; gap: 12px; padding: 16px 0; border-bottom: 1px solid var(--border); }
.legacy-chat-icon { flex: none; color: var(--muted); margin-top: 2px; }
.legacy-chat-detail { flex: 1; min-width: 0; overflow-wrap: anywhere; }
.legacy-chat-detail h3 { margin: 0; font-size: 14px; font-weight: 600; color: var(--foreground); }
.legacy-chat-detail p, .legacy-chat-detail time { display: block; margin: 4px 0 0; font-size: 11px; color: var(--muted); }
.legacy-chat-actions { flex: none; flex-wrap: wrap; }
.legacy-chat-pagination { justify-content: center; font-size: 12px; color: var(--muted); }
.legacy-chat-empty { padding: 28px 0; text-align: center; color: var(--muted); }
.legacy-chat-error { color: var(--danger-text); font-size: 12px; overflow-wrap: anywhere; }
@media (max-width: 767px) {
  .legacy-chat-row { flex-wrap: wrap; }
  .legacy-chat-detail { flex-basis: calc(100% - 32px); }
  .legacy-chat-actions { width: 100%; justify-content: flex-end; }
  .legacy-chat-actions :deep(.ui-btn) { white-space: normal; }
}
</style>
