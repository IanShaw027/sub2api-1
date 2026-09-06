<!-- Adapted from chat-vue@80649c38 MessageActions/MessageEdit; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Brain, Copy, Download, ExternalLink, LoaderCircle, Pencil, RotateCcw, ThumbsDown, ThumbsUp } from '@lucide/vue'
import Button from '@/components/ui/Button.vue'
import type { LocalChatMessage } from '../localChat'
import MessageContent from './MessageContent.vue'
import LocalChatFiles from './LocalChatFiles.vue'
import LocalChatChart from './LocalChatChart.vue'
import { localChatMessages } from './localChatMessages'

const props = defineProps<{ message: LocalChatMessage; busy?: boolean }>()
const emit = defineEmits<{ action: [action: string, message: LocalChatMessage, text?: string]; preview: [file: File] }>()
const { t, locale } = useI18n({ useScope: 'local', messages: localChatMessages })
const editing = ref(false)
const text = ref('')
const timestamp = computed(() => new Date(props.message.createdAt).toLocaleTimeString(locale.value, { hour: '2-digit', minute: '2-digit' }))
const sources = computed(() => (props.message.sources || []).flatMap(source => {
  try {
    const url = new URL(source.url)
    return ['http:', 'https:'].includes(url.protocol) ? [{ ...source, label: source.title || url.hostname }] : []
  } catch { return [] }
}))
const charts = computed(() => (props.message.toolResults || []).flatMap(result => {
  const data = result.data as { type?: string; chart?: unknown } | undefined
  return !result.isError && data?.type === 'chart' ? [{ id: result.toolCallId, data: data.chart }] : []
}))
function startEdit() { text.value = props.message.content; editing.value = true }
function saveEdit() { if (!text.value.trim() || props.busy) return; editing.value = false; emit('action', 'edit', props.message, text.value) }
</script>

<template>
  <article class="local-chat-message" :class="`is-${message.role}`" :data-message-id="message.id" :aria-label="`${message.role === 'user' ? t('you') : t('assistant')}, ${timestamp}`">
    <div class="local-chat-message-body">
      <div class="local-chat-message-meta"><strong>{{ message.role === 'user' ? t('you') : message.model || t('assistant') }}</strong><time :datetime="message.createdAt">{{ timestamp }}</time><span v-if="message.status === 'stopped'">{{ t('stopped') }}</span></div>
      <LocalChatFiles :files="message.files" @preview="emit('preview', $event)" />
      <div v-if="editing" class="local-chat-message-edit"><textarea v-model="text" :aria-label="t('edit')" rows="4" /><div><Button variant="secondary" size="sm" @click="editing = false">{{ t('cancel') }}</Button><Button size="sm" :disabled="busy || !text.trim() || text === message.content" @click="saveEdit">{{ t('saveGenerate') }}</Button></div></div>
      <template v-else>
        <details v-if="message.reasoning" class="local-chat-reasoning"><summary><Brain :size="14" />{{ t('reasoning') }}</summary><p>{{ message.reasoning }}</p></details>
        <MessageContent :role="message.role" :content="message.content" :streaming="message.status === 'streaming'" :input-tokens="message.input_tokens ?? message.usage?.input_tokens" :output-tokens="message.output_tokens ?? message.usage?.output_tokens" />
        <LocalChatChart v-for="chart in charts" :key="chart.id" :data="chart.data" />
        <div v-if="sources.length" class="local-chat-sources"><span>{{ t('sources') }}</span><a v-for="(source, index) in sources" :key="source.url" :href="source.url" target="_blank" rel="noopener noreferrer"><span>{{ index + 1 }}. {{ source.label }}</span><ExternalLink :size="11" /></a></div>
        <p v-if="message.error" class="local-chat-message-error" role="alert">{{ message.error }}</p>
        <div v-if="message.status === 'streaming' && !message.content" class="local-chat-message-generating" role="status"><LoaderCircle :size="15" />{{ t('generating') }}</div>
      </template>
      <div v-if="!editing && message.status !== 'streaming'" class="local-chat-message-actions">
        <button type="button" :title="t('copy')" :aria-label="t('copy')" :disabled="!message.content" @click="emit('action', 'copy', message)"><Copy :size="15" /></button>
        <button type="button" :title="t('download')" :aria-label="t('download')" :disabled="!message.content && !message.files.length" @click="emit('action', 'download', message)"><Download :size="15" /></button>
        <button v-if="message.role === 'user'" type="button" :title="t('edit')" :aria-label="t('edit')" :disabled="busy" @click="startEdit"><Pencil :size="15" /></button>
        <template v-if="message.role === 'assistant'">
          <button type="button" :title="t('upvote')" :aria-label="t('upvote')" :aria-pressed="message.vote === 'up'" @click="emit('action', 'upvote', message)"><ThumbsUp :size="15" /></button>
          <button type="button" :title="t('downvote')" :aria-label="t('downvote')" :aria-pressed="message.vote === 'down'" @click="emit('action', 'downvote', message)"><ThumbsDown :size="15" /></button>
          <button type="button" :title="t('regenerate')" :aria-label="t('regenerate')" :disabled="busy" @click="emit('action', 'regenerate', message)"><RotateCcw :size="15" /></button>
        </template>
      </div>
    </div>
  </article>
</template>

<style scoped>
.local-chat-message { display: flex; width: 100%; padding: 13px 0; }
.local-chat-message-body { width: 100%; min-width: 0; }
.local-chat-message.is-user { justify-content: flex-end; }
.is-user .local-chat-message-body { width: auto; max-width: 88%; padding: 12px 15px; border-radius: 8px; background: var(--surface-secondary); }
.local-chat-message-meta { display: flex; align-items: center; flex-wrap: wrap; gap: 10px; margin-bottom: 8px; font-size: 11px; color: var(--muted); }
.local-chat-message-meta strong { font-weight: 600; overflow-wrap: anywhere; }
.local-chat-message-meta time { font-family: var(--font-mono); font-size: 10px; }
.local-chat-message-body :deep(.studio-message-content) { font-size: 14px; line-height: 1.75; overflow-wrap: anywhere; }
.local-chat-message-body :deep(.studio-markdown pre) { max-width: 100%; overflow-x: auto; }
.local-chat-message-body :deep(.studio-markdown table) { display: block; max-width: 100%; overflow: auto; }
.local-chat-message-body :deep(.studio-markdown img) { max-width: 100%; }
.local-chat-message-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 3px; margin-top: 10px; }
.local-chat-message-actions button { display: grid; place-items: center; width: 29px; height: 29px; border: 0; border-radius: 5px; color: var(--muted); background: none; }
.local-chat-message-actions button:hover, .local-chat-message-actions button[aria-pressed="true"] { background: var(--surface-secondary); color: var(--accent); }
.local-chat-message-actions button:disabled { opacity: 0.4; cursor: not-allowed; }
.local-chat-message-edit textarea { width: 100%; min-width: 240px; border: 1px solid var(--border); border-radius: 6px; padding: 10px; background: var(--surface); color: var(--foreground); resize: vertical; font-size: 14px; }
.local-chat-message-edit > div { display: flex; gap: 6px; justify-content: flex-end; margin-top: 8px; }
.local-chat-reasoning { border-left: 2px solid var(--border); margin-bottom: 12px; padding-left: 12px; color: var(--muted); }
.local-chat-reasoning summary { display: flex; align-items: center; gap: 6px; cursor: pointer; font-size: 12px; }
.local-chat-reasoning p { margin-top: 8px; font-size: 12px; line-height: 1.7; white-space: pre-wrap; max-height: 300px; overflow: auto; }
.local-chat-sources { display: flex; flex-direction: column; gap: 6px; margin: 14px 0 4px; color: var(--muted); font-size: 11px; }
.local-chat-sources a { display: flex; align-items: center; gap: 6px; color: var(--accent); text-decoration: none; }
.local-chat-sources a span { overflow-wrap: anywhere; }
.local-chat-message-error { color: var(--danger-text); font-size: 12px; margin-top: 8px; }
.local-chat-message-generating { display: flex; align-items: center; gap: 8px; color: var(--muted); font-size: 12px; }
.local-chat-message-generating svg { animation: local-chat-spin 1s linear infinite; }
@keyframes local-chat-spin { to { transform: rotate(360deg); } }
@media (max-width: 600px) { .is-user .local-chat-message-body { max-width: 96%; } .local-chat-message-edit textarea { min-width: 0; } }
@media (prefers-reduced-motion: reduce) { .local-chat-message-generating svg { animation: none; } }
</style>
