<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6 lg:flex-row">
      <section class="flex-1 space-y-6">
        <div class="card overflow-hidden">
          <div class="border-b border-line bg-gradient-to-r from-dark-900 via-brand-950 to-accent-950 px-6 py-5 text-white dark:border-dark-700">
            <div class="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <p class="text-xs uppercase tracking-[0.35em] text-white/60">{{ t('ai.center.label', 'AI 创作中心') }}</p>
                <h1 class="mt-2 text-2xl font-bold">{{ t('ai.chat.title', 'AI 对话') }}</h1>
                <p class="mt-1 text-sm text-white/70">{{ t('ai.chat.subtitle', '先选线路，再开始对话，key 只在内部使用。') }}</p>
              </div>
              <div class="flex flex-wrap items-center gap-3">
                <button class="btn btn-secondary bg-white/10 text-white hover:bg-white/20" :disabled="loading" @click="refreshRuntime">
                  <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
                </button>
                <button class="btn btn-primary" :disabled="!canSend || sending" @click="sendMessage">
                  <Icon name="chatBubble" size="sm" class="mr-2" />
                  {{ sending ? t('common.processing', '处理中') : t('ai.chat.send', '发送') }}
                </button>
              </div>
            </div>
          </div>

          <div class="grid gap-6 p-6 xl:grid-cols-[280px_minmax(0,1fr)]">
            <div class="space-y-4">
              <div class="rounded-card border border-line bg-page p-4 dark:border-dark-700 dark:bg-dark-900">
                <label class="input-label mb-1.5 block">{{ t('ai.line.selector', '线路') }}</label>
                <Select
                  :model-value="aiStore.selectedLineId"
                  :options="lineOptions"
                  @update:model-value="updateSelectedLine"
                />
                <div class="mt-3">
                  <label class="input-label mb-1.5 block">{{ t('ai.line.key', '密钥') }}</label>
                  <Select
                    :model-value="aiStore.selectedKeyId"
                    :options="keyOptions"
                    @update:model-value="updateSelectedKey"
                  />
                </div>
                <div class="mt-3 space-y-1 text-sm text-ink-body dark:text-dark-400">
                  <p>{{ t('ai.line.availableKeys', '可用 key') }}: {{ aiStore.selectedLine?.key_count ?? 0 }}</p>
                  <p>{{ t('ai.line.platform', '平台') }}: {{ aiStore.selectedLine?.platform ?? '-' }}</p>
                </div>
                <p
                  v-if="showRuntimeLineNotice"
                  class="mt-3 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900/40 dark:bg-amber-900/20 dark:text-amber-100"
                >
                  {{ t('ai.chat.runtimeLineNotice', '当前 runtime 已接入，但还没有返回可用线路；前端不会再从本地或其他接口拼装线路。') }}
                </p>
              </div>

              <div class="rounded-card border border-line bg-card p-4 shadow-xs dark:border-dark-700 dark:bg-dark-900">
                <label class="input-label mb-1.5 block">{{ t('ai.chat.entry', '入口') }}</label>
                <Select :model-value="entryMode" :options="entryOptions" @update:model-value="updateEntryMode" />
                <p class="mt-3 text-xs text-ink-soft dark:text-dark-400">{{ entryHint }}</p>
                <div class="mt-3 flex flex-wrap gap-2 text-[11px] text-ink-soft dark:text-dark-400">
                  <span class="rounded-full bg-line px-2 py-1 dark:bg-dark-800">{{ t('ai.chat.runtimeLabel', 'runtime') }}: {{ runtimeSourceDomain }}</span>
                  <span class="rounded-full bg-line px-2 py-1 dark:bg-dark-800">{{ t('ai.chat.responsesLabel', 'responses') }}: {{ entryMode === 'responses' ? t('common.enabled', '启用') : t('common.disabled', '关闭') }}</span>
                </div>
              </div>

              <div class="rounded-card border border-line bg-card p-4 shadow-xs dark:border-dark-700 dark:bg-dark-900">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('ai.chat.recentSessions', '最近会话') }}</h2>
                    <p class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ t('ai.chat.recentSessionsHint', '刷新或分享链接后，可按 session 恢复历史消息。') }}</p>
                  </div>
                  <button class="btn btn-secondary btn-sm" type="button" @click="startNewSession">{{ t('ai.chat.newSession', '新会话') }}</button>
                </div>
                <div class="mt-3 space-y-2">
                  <button
                    v-for="session in aiStore.chatSessions.items"
                    :key="session.id"
                    type="button"
                    class="w-full rounded-xl border px-3 py-2 text-left transition-colors"
                    :class="session.id === aiStore.activeSessionId
                      ? 'border-brand-300 bg-brand-50 dark:border-brand-700 dark:bg-brand-900/20'
                      : 'border-line hover:border-brand-300 hover:bg-brand-50 dark:border-dark-700 dark:hover:border-brand-700 dark:hover:bg-brand-900/20'"
                    @click="selectSession(session.id)"
                  >
                    <div class="truncate text-sm font-medium text-ink dark:text-white">{{ session.title }}</div>
                    <div class="mt-1 text-xs text-ink-soft dark:text-dark-400">
                      {{ formatSessionTime(session.last_message_at || session.updated_at) }}
                    </div>
                  </button>
                  <EmptyState
                    v-if="!loading && !aiStore.loadingChatSessions && aiStore.chatSessions.items.length === 0"
                    :title="t('ai.chat.noSessions', '暂无历史会话')"
                    :description="t('ai.chat.noSessionsDesc', '发送第一条消息后会自动创建会话。')"
                  />
                </div>
              </div>

              <div class="rounded-card border border-line bg-card p-4 shadow-xs dark:border-dark-700 dark:bg-dark-900">
                <div class="flex items-center justify-between">
                  <h2 class="text-sm font-semibold text-ink dark:text-white">{{ t('ai.chat.quickPrompts', '快捷提示词') }}</h2>
                  <span class="text-xs text-ink-soft dark:text-dark-400">{{ quickPrompts.length }}</span>
                </div>
                <div class="mt-3 space-y-2">
                  <button
                    v-for="prompt in quickPrompts"
                    :key="prompt.id"
                    class="w-full rounded-xl border border-line px-3 py-2 text-left text-sm transition-colors hover:border-brand-300 hover:bg-brand-50 dark:border-dark-700 dark:hover:border-brand-700 dark:hover:bg-brand-900/20"
                    @click="draft = prompt.content"
                  >
                    <div class="font-medium text-ink dark:text-white">{{ prompt.title }}</div>
                    <div class="mt-1 line-clamp-2 text-xs text-ink-soft dark:text-dark-400">{{ prompt.content }}</div>
                  </button>
                  <EmptyState
                    v-if="!loading && quickPrompts.length === 0"
                    :title="t('ai.chat.noPrompts', '暂无提示词')"
                    :description="t('ai.chat.noPromptsDesc', '去提示词库创建一些模板吧。')"
                  />
                </div>
              </div>
            </div>

            <div class="flex min-h-[560px] flex-col rounded-card border border-line bg-card shadow-xs dark:border-dark-700 dark:bg-dark-900">
              <div class="flex-1 space-y-4 overflow-y-auto p-4">
                <div v-if="aiStore.activeSession" class="rounded-card border border-dashed border-line px-4 py-3 text-xs text-ink-soft dark:border-dark-700">
                  {{ t('ai.chat.currentSession', '当前会话：') }}{{ aiStore.activeSession.title }}
                </div>
                <template v-if="messages.length > 0">
                  <div
                    v-for="message in messages"
                    :key="message.id"
                    class="flex"
                    :class="message.role === 'user' ? 'justify-end' : 'justify-start'"
                  >
                    <div
                      class="max-w-[85%] rounded-card px-4 py-3 text-sm leading-6"
                      :class="message.role === 'user'
                        ? 'bg-brand-600 text-white'
                        : message.role === 'system'
                          ? 'bg-amber-50 text-amber-900 dark:bg-amber-900/20 dark:text-amber-100'
                          : 'bg-page text-ink dark:bg-dark-800 dark:text-ink'"
                    >
                      <p class="whitespace-pre-wrap">{{ message.content }}</p>
                      <p class="mt-2 text-[11px] opacity-70">{{ formatTime(message.created_at) }}</p>
                    </div>
                  </div>
                </template>
                <EmptyState
                  v-else
                  :title="t('ai.chat.emptyTitle', '开始一段对话')"
                  :description="t('ai.chat.emptyDesc', '选择线路和入口后输入问题，系统会自动使用线路内可用 key。')"
                />
                <div ref="chatEndRef" />
              </div>

              <div class="border-t border-line p-4 dark:border-dark-700">
                <TextArea
                  v-model="draft"
                  :rows="5"
                  :placeholder="t('ai.chat.placeholder', '输入你的需求，比如：生成一段更像产品经理口吻的总结。')"
                />
                <div class="mt-3 flex items-center justify-between gap-3">
                  <p class="text-xs text-ink-soft dark:text-dark-400">
                    {{ t('ai.chat.hint', '自动选取线路内第一个可用 key，不会在界面暴露 key 本身。') }}
                  </p>
                  <button class="btn btn-primary" :disabled="!canSend || sending" @click="sendMessage">
                    {{ sending ? t('common.processing', '处理中') : t('ai.chat.send', '发送') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { apiClient } from '@/api/client'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import { listAIPrompts } from '@/api'
import { useAppStore, useAiStudioStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AiPromptTemplate } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const aiStore = useAiStudioStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const sending = ref(false)
const draft = ref('')
const chatEndRef = ref<HTMLElement | null>(null)
const quickPrompts = ref<AiPromptTemplate[]>([])
const entryMode = ref<'responses' | 'chat_completions'>('responses')
const messages = computed(() => aiStore.sessionMessages)

let suppressRouteRestore = false

const lineOptions = computed(() =>
  aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: `${line.label} · ${line.key_count}`
  }))
)

const keyOptions = computed(() => {
  const currentLine = aiStore.selectedLine
  if (!currentLine) {
    return [{ value: null, label: t('ai.line.noKey', '暂无可用密钥') }]
  }
  const keys = currentLine.keys.length > 0
    ? currentLine.keys
    : currentLine.key_ids.map((id) => ({ id, name: `Key ${id}` }))
  return keys.length > 0
    ? keys.map((key) => ({
        value: key.id,
        label: `${key.name} (#${key.id})`
      }))
    : [{ value: null, label: t('ai.line.noKey', '暂无可用密钥') }]
})

const canSend = computed(() => !!draft.value.trim() && !!aiStore.selectedLineId && !!aiStore.selectedKeyId)
const showRuntimeLineNotice = computed(() => !loading.value && !!aiStore.runtimeInfo && aiStore.availableLines.length === 0)
const supportedEntryModes = computed<Array<'responses' | 'chat_completions'>>(() => {
  const runtimeEntries = aiStore.runtimeInfo?.chat?.supported_entries ?? []
  return runtimeEntries.length > 0 ? runtimeEntries : ['responses']
})
const chatCompletionsAvailable = computed(() => supportedEntryModes.value.includes('chat_completions'))

const entryOptions = computed(() => [
  { value: 'responses', label: t('ai.chat.responses', 'Responses') },
  ...(chatCompletionsAvailable.value ? [{ value: 'chat_completions', label: t('ai.chat.chatCompletions', 'Chat Completions') }] : [])
])

const runtimeSourceDomain = computed(() => aiStore.runtimeInfo?.source_domain || '-')
const entryHint = computed(() => {
  if (!chatCompletionsAvailable.value) {
    return t('ai.chat.responsesOnlyHint', '当前 runtime 只暴露 responses 入口，chat_completions 已按后端能力隐藏。')
  }
  if (entryMode.value === 'chat_completions') {
    return t('ai.chat.chatCompletionsHint', '仅在后端真实支持时才会下发 chat_completions。')
  }
  return t('ai.chat.entryHint', '切换后会直接传给后端。')
})

function extractQueryString(value: unknown): string | null {
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return null
}

function extractPositiveQueryNumber(value: unknown): number | null {
  const raw = extractQueryString(value)
  if (!raw) return null
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : null
}

function normalizeRouteEntry(value: unknown): 'responses' | 'chat_completions' {
  return extractQueryString(value) === 'chat_completions' ? 'chat_completions' : 'responses'
}

function syncEntryMode(): void {
  if (!chatCompletionsAvailable.value && entryMode.value !== 'responses') {
    entryMode.value = 'responses'
    return
  }
  if (entryMode.value === 'chat_completions' && !chatCompletionsAvailable.value) {
    entryMode.value = 'responses'
  }
}

async function replaceChatQuery(): Promise<boolean> {
  const nextQuery = { ...route.query }
  const nextSession = aiStore.activeSessionId ? String(aiStore.activeSessionId) : null
  const nextLine = aiStore.selectedLineId ? String(aiStore.selectedLineId) : null
  const nextKey = aiStore.selectedKeyId ? String(aiStore.selectedKeyId) : null
  const nextEntry = entryMode.value === 'chat_completions' ? 'chat_completions' : null
  const currentSession = extractQueryString(route.query.session)
  const currentLine = extractQueryString(route.query.line)
  const currentKey = extractQueryString(route.query.key)
  const currentEntry = extractQueryString(route.query.entry)

  if (nextSession) nextQuery.session = nextSession
  else delete nextQuery.session

  if (nextLine) nextQuery.line = nextLine
  else delete nextQuery.line

  if (nextKey) nextQuery.key = nextKey
  else delete nextQuery.key

  if (nextEntry) nextQuery.entry = nextEntry
  else delete nextQuery.entry

  if (
    currentSession === nextSession &&
    currentLine === nextLine &&
    currentKey === nextKey &&
    currentEntry === nextEntry
  ) {
    return false
  }

  suppressRouteRestore = true
  await router.replace({ query: nextQuery })
  return true
}

async function hydrateRouteState(): Promise<void> {
  const routeLineId = extractPositiveQueryNumber(route.query.line)
  const routeKeyId = extractPositiveQueryNumber(route.query.key)
  const routeSessionId = extractPositiveQueryNumber(route.query.session)
  entryMode.value = normalizeRouteEntry(route.query.entry)
  syncEntryMode()

  if (routeLineId && aiStore.availableLines.some((line) => line.group_id === routeLineId)) {
    if (routeLineId !== aiStore.selectedLineId) {
      aiStore.setSelectedLine(routeLineId)
    }
  } else {
    aiStore.ensureSelection()
  }

  if (routeKeyId) {
    const hasCurrentKey = aiStore.selectedLine?.keys.some((item) => item.id === routeKeyId)
      || aiStore.selectedLine?.key_ids.includes(routeKeyId)
      || false
    if (hasCurrentKey && routeKeyId !== aiStore.selectedKeyId) {
      aiStore.setSelectedKey(routeKeyId)
    } else if (!hasCurrentKey) {
      aiStore.syncSelectedKey()
    }
  } else {
    aiStore.syncSelectedKey()
  }

  if (routeSessionId) {
    if (routeSessionId !== aiStore.activeSessionId || aiStore.sessionMessages.length === 0) {
      await aiStore.loadChatSession(routeSessionId)
    }
    const sessionLineId = aiStore.activeSession?.line_id ?? null
    if (!routeLineId && sessionLineId && aiStore.availableLines.some((line) => line.group_id === sessionLineId)) {
      aiStore.setSelectedLine(sessionLineId)
      aiStore.syncSelectedKey()
    }
  } else if (aiStore.activeSessionId) {
    aiStore.setActiveSession(null)
  }
}

function buildSessionTitle(prompt: string): string {
  const trimmed = prompt.trim()
  return trimmed.length > 48 ? trimmed.slice(0, 48) : trimmed
}

async function refreshRuntime() {
  loading.value = true
  try {
    void loadQuickPrompts().catch((error) => {
      quickPrompts.value = []
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    })

    await Promise.all([
      aiStore.loadRuntimeLines(true),
      aiStore.loadChatSessions()
    ])
    await hydrateRouteState()
    syncEntryMode()
    await replaceChatQuery()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

async function updateEntryMode(value: string | number | boolean | null) {
  const next = String(value ?? 'responses')
  entryMode.value = next === 'chat_completions' && chatCompletionsAvailable.value ? 'chat_completions' : 'responses'
  await replaceChatQuery()
}

async function updateSelectedLine(value: string | number | boolean | null) {
  aiStore.setSelectedLine(typeof value === 'number' ? value : null)
  await replaceChatQuery()
}

async function updateSelectedKey(value: string | number | boolean | null) {
  aiStore.setSelectedKey(typeof value === 'number' ? value : null)
  await replaceChatQuery()
}

async function loadQuickPrompts() {
  const response = await listAIPrompts(1, 8, { scope: 'library', visibility: 'public', status: 'published' })
  quickPrompts.value = response.items
}

function formatTime(value: string): string {
  return new Date(value).toLocaleString()
}

function formatSessionTime(value: string): string {
  return new Date(value).toLocaleString()
}

async function selectSession(sessionId: number): Promise<void> {
  try {
    await aiStore.loadChatSession(sessionId)
    const sessionLineId = aiStore.activeSession?.line_id ?? null
    if (sessionLineId && aiStore.availableLines.some((line) => line.group_id === sessionLineId)) {
      aiStore.setSelectedLine(sessionLineId)
      aiStore.syncSelectedKey()
    }
    await replaceChatQuery()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function startNewSession(): Promise<void> {
  aiStore.setActiveSession(null)
  await replaceChatQuery()
  await nextTick()
  chatEndRef.value?.scrollIntoView({ block: 'end' })
}

async function sendMessage() {
  const prompt = draft.value.trim()
  if (!prompt || !aiStore.selectedLineId || !aiStore.selectedKeyId) return

  draft.value = ''
  sending.value = true
  try {
    const session =
      aiStore.activeSessionId
        ? { id: aiStore.activeSessionId }
        : await aiStore.createChatSession(buildSessionTitle(prompt))
    await replaceChatQuery()

    await apiClient.post('/user/ai/chat', {
      prompt,
      session_id: session.id,
      line_id: aiStore.selectedLineId,
      key_id: aiStore.selectedKeyId,
      history: aiStore.sessionMessages.map((message) => ({ ...message })),
      use_responses: entryMode.value !== 'chat_completions' || !chatCompletionsAvailable.value
    })
    await Promise.all([
      aiStore.loadChatSession(session.id),
      aiStore.loadChatSessions()
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    sending.value = false
    await nextTick()
    chatEndRef.value?.scrollIntoView({ block: 'end' })
  }
}

watch(
  () => [route.query.session, route.query.line, route.query.key, route.query.entry],
  async () => {
    if (suppressRouteRestore) {
      suppressRouteRestore = false
      return
    }
    try {
      await hydrateRouteState()
    } catch (error) {
      appStore.showError(extractApiErrorMessage(error, t('common.error')))
    }
  }
)

watch(
  () => aiStore.sessionMessages.length,
  async () => {
    await nextTick()
    chatEndRef.value?.scrollIntoView({ block: 'end' })
  }
)

onMounted(async () => {
  await refreshRuntime()
  await nextTick()
  chatEndRef.value?.scrollIntoView({ block: 'end' })
})
</script>
