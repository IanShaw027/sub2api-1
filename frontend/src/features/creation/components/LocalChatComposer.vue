<!-- Adapted from chat-vue@80649c38 ChatComposer.vue; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowUp, LoaderCircle, Paperclip, Square } from '@lucide/vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import type { Group } from '@/types'
import { CHAT_ATTACHMENT_ACCEPT } from '../chatApi'
import LocalChatFiles from './LocalChatFiles.vue'
import { localChatMessages } from './localChatMessages'

const props = defineProps<{ modelValue: string; files: File[]; groups: Group[]; models: string[]; groupId: number | null; model: string; streaming: boolean; loading?: boolean; modelsLoading?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; files: [files: File[]]; remove: [index: number]; preview: [file: File]; group: [value: number]; model: [value: string]; submit: []; stop: [] }>()
const { t } = useI18n({ useScope: 'local', messages: localChatMessages })
const input = ref<HTMLTextAreaElement | null>(null)
const fileInput = ref<HTMLInputElement | null>(null)
const composing = ref(false)
const canSend = computed(() => !props.streaming && !props.loading && !props.modelsLoading && Boolean(props.groupId && props.models.includes(props.model)) && Boolean(props.modelValue.trim() || props.files.length))
const groups = computed(() => props.groups.map(group => ({ value: group.id, label: group.name })))
const models = computed(() => props.models.map(model => ({ value: model, label: model })))
function onFiles(event: Event) { const element = event.target as HTMLInputElement; emit('files', Array.from(element.files || [])); element.value = '' }
function paste(event: ClipboardEvent) { const files = Array.from(event.clipboardData?.files || []); if (files.length) { event.preventDefault(); emit('files', files) } }
function submit() { if (canSend.value) emit('submit') }
function keydown(event: KeyboardEvent) {
  if (composing.value || event.isComposing || event.keyCode === 229) return
  if (event.key === 'Enter' && !event.shiftKey && !event.ctrlKey && !event.altKey && !event.metaKey) { event.preventDefault(); submit() }
}
function resize() { if (input.value) { input.value.style.height = 'auto'; input.value.style.height = `${Math.min(input.value.scrollHeight, 180)}px` } }
watch(() => props.modelValue, () => { void nextTick(resize) })
defineExpose({ focus: () => input.value?.focus() })
</script>

<template>
  <div class="local-chat-composer">
    <LocalChatFiles :files="files" compact removable :disabled="streaming || loading" @remove="emit('remove', $event)" @preview="emit('preview', $event)" />
    <textarea ref="input" :value="modelValue" :aria-label="t('prompt')" :placeholder="t('placeholder')" rows="2" :disabled="loading" @input="emit('update:modelValue', ($event.target as HTMLTextAreaElement).value); resize()" @paste="paste" @keydown="keydown" @compositionstart="composing = true" @compositionend="composing = false" />
    <div class="local-chat-compose-controls">
      <input ref="fileInput" type="file" multiple :accept="CHAT_ATTACHMENT_ACCEPT" hidden @change="onFiles" />
      <button type="button" class="icon-btn local-chat-attach" :title="t('attach')" :aria-label="t('attach')" :disabled="loading || streaming || files.length >= 8" @click="fileInput?.click()"><Paperclip :size="17" /><span v-if="files.length">{{ files.length }}/8</span></button>
      <UiSelect :model-value="groupId" :options="groups" :placeholder="t('chooseGroup')" :aria-label="t('group')" :disabled="loading || streaming" searchable="auto" class="local-chat-group" @update:model-value="typeof $event === 'number' && emit('group', $event)" />
      <UiSelect :model-value="model" :options="models" :placeholder="modelsLoading ? t('loading') : t('chooseModel')" :aria-label="t('model')" :disabled="loading || streaming || modelsLoading" searchable="auto" class="local-chat-model" @update:model-value="typeof $event === 'string' && emit('model', $event)" />
      <button v-if="streaming" type="button" class="btn btn-secondary local-chat-submit local-chat-stop" :title="t('stop')" :aria-label="t('stop')" @click="emit('stop')"><Square :size="16" fill="currentColor" /></button>
      <button v-else type="button" class="btn btn-primary local-chat-submit" :disabled="!canSend" :title="t('send')" :aria-label="t('send')" @click="submit"><LoaderCircle v-if="loading" class="local-chat-spin" :size="18" /><ArrowUp v-else :size="19" /></button>
    </div>
    <p v-if="!loading && !modelsLoading && groupId && !models.length" class="local-chat-no-model" role="status">{{ t('noModel') }}</p>
  </div>
</template>

<style scoped>
.local-chat-composer { padding: 14px; border: 1px solid var(--border); border-radius: var(--radius-field); background: color-mix(in oklch, var(--surface) 85%, transparent); box-shadow: var(--shadow); backdrop-filter: blur(20px); }
.local-chat-composer textarea { display: block; width: 100%; min-height: 60px; max-height: 180px; padding: 3px 3px 10px; border: 0; outline: none; resize: none; background: none; color: var(--foreground); font-size: 14px; line-height: 1.6; }
.local-chat-composer textarea::placeholder { color: var(--muted); }
.local-chat-compose-controls { display: flex; align-items: center; gap: 8px; padding-top: 12px; border-top: 1px solid var(--border); }
.local-chat-attach { gap: 4px; min-width: 34px; height: 36px; padding: 4px; font-size: var(--fs-11); }
.local-chat-group { flex: 0 1 150px; min-width: 90px; }
.local-chat-model { flex: 1; min-width: 100px; max-width: 320px; }
.local-chat-model { font-family: var(--font-mono); }
.local-chat-submit { margin-left: auto; flex-shrink: 0; width: 36px; height: 36px; padding: 0; }
.local-chat-composer:focus-within { border-color: color-mix(in oklch, var(--accent) 45%, var(--border)); }
.local-chat-submit:disabled, .local-chat-attach:disabled { opacity: 0.4; cursor: not-allowed; }
.local-chat-no-model { margin: 9px 0 0; color: var(--muted); font-size: 11px; }
.local-chat-spin { animation: local-chat-spin 1s linear infinite; }
@keyframes local-chat-spin { to { transform: rotate(360deg); } }
@media (max-width: 600px) { .local-chat-composer { padding: 10px; } .local-chat-composer textarea { font-size: 16px; } .local-chat-compose-controls { gap: 4px; flex-wrap: wrap; } .local-chat-group { flex-basis: 94px; min-width: 78px; } .local-chat-model { flex-basis: 130px; min-width: 90px; } }
@media (prefers-reduced-motion: reduce) { .local-chat-spin { animation: none; } }
</style>
