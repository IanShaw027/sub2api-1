<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import { creationMode, creationPath, creationTitleKey, type WorkspaceMode } from './navigation'
import { useChatWorkspace } from './stores/chatWorkspace'
import { useMediaWorkspace } from './stores/mediaWorkspace'

const ChatWorkspace = defineAsyncComponent(() => import('./components/ChatWorkspace.vue'))
const MediaWorkspace = defineAsyncComponent(() => import('./components/MediaWorkspace.vue'))
const VoiceWorkspace = defineAsyncComponent(() => import('./components/VoiceWorkspace.vue'))
const InspirationWall = defineAsyncComponent(() => import('./components/InspirationWall.vue'))
const route = useRoute()
const router = useRouter()
const chat = useChatWorkspace()
const media = useMediaWorkspace()
const { t } = useI18n()
const mode = computed(() => creationMode(route.meta.creationMode ?? route.params.mode ?? route.query.mode))
const chatLoading = ref(false)
const chatError = ref('')
const promptError = ref('')
const keyboardOffset = ref(0)
let chatOpening: Promise<void> | null = null
let disposed = false

function selectMode(next: WorkspaceMode) {
  if (mode.value !== next) void router.push(creationPath(next))
}

async function openChat() {
  if (chatOpening) return chatOpening
  chatLoading.value = true
  chatError.value = ''
  const opening = (async () => {
    try {
      await chat.init()
    } catch (error) {
      chatError.value = error instanceof Error ? error.message : String(error)
    } finally { chatLoading.value = false }
  })()
  chatOpening = opening
  try { await opening } finally { if (chatOpening === opening) chatOpening = null }
}

async function createFromInspiration(payload: { kind: 'image' | 'video'; prompt: string; model?: string }) {
  const origin = route.fullPath
  promptError.value = ''
  try {
    await media.applyInspiration(payload)
    if (disposed || route.fullPath !== origin) return
    await router.push(creationPath(payload.kind))
  } catch (error) { if (!disposed) promptError.value = error instanceof Error ? error.message : String(error) }
}

function updateViewport() {
  const viewport = window.visualViewport
  keyboardOffset.value = viewport ? Math.max(0, window.innerHeight - viewport.height - viewport.offsetTop) : 0
}

watch(mode, (next) => { if (next === 'chat') void openChat() }, { immediate: true })
// Consume prompt links once; opening an example never generates or publishes it.
watch(() => [mode.value, route.query.prompt] as const, async ([next, prompt]) => {
  if ((next !== 'image' && next !== 'video') || typeof prompt !== 'string' || !prompt.trim()) return
  promptError.value = ''
  try {
    await media.applyInspiration({ kind: next, prompt })
    if (disposed || mode.value !== next || route.query.prompt !== prompt) return
    const query = { ...route.query }
    delete query.prompt
    await router.replace({ path: route.path, query })
  } catch (error) { if (!disposed) promptError.value = error instanceof Error ? error.message : String(error) }
}, { immediate: true })

onMounted(() => {
  updateViewport()
  window.visualViewport?.addEventListener('resize', updateViewport)
  window.visualViewport?.addEventListener('scroll', updateViewport)
})
onUnmounted(() => {
  disposed = true
  window.visualViewport?.removeEventListener('resize', updateViewport)
  window.visualViewport?.removeEventListener('scroll', updateViewport)
})
</script>

<template>
  <AppLayout fill-height>
  <div class="creation-module" :style="{ '--creation-keyboard-offset': `${keyboardOffset}px` }">
    <PageHeader :title="t(creationTitleKey(mode))" />
    <div v-if="promptError" class="creation-load-error" role="alert">{{ promptError }}</div>
    <section id="creation-workspace" class="creation-workspace" :aria-label="t(creationTitleKey(mode))">
      <template v-if="mode === 'chat'">
        <div v-if="chatError" class="creation-load-error" role="alert"><p>{{ chatError }}</p><Button variant="secondary" @click="openChat">{{ t('common.retry') }}</Button></div>
        <ChatWorkspace v-else :aria-busy="chatLoading" />
      </template>
      <MediaWorkspace v-else-if="mode === 'image' || mode === 'video'" :mode="mode" @update:mode="selectMode" @inspiration="selectMode('gallery')" />
      <VoiceWorkspace v-else-if="mode === 'voice'" />
      <InspirationWall v-else @create="createFromInspiration" />
    </section>
  </div>
  </AppLayout>
</template>

<style scoped>
.creation-module { display: flex; flex: 1; flex-direction: column; min-width: 0; min-height: 0; padding-bottom: var(--creation-keyboard-offset, 0px); color: var(--foreground); letter-spacing: 0; }
.creation-module :deep(.ui-page-header) { flex-shrink: 0; }
.creation-module :deep(h1), .creation-module :deep(h2) { letter-spacing: 0; }
.creation-workspace { flex: 1; display: flex; flex-direction: column; min-height: 0; min-width: 0; overflow: auto; }
.creation-load-error { margin: auto; padding: 24px; display: grid; gap: 12px; text-align: center; color: var(--danger-text); }
</style>
