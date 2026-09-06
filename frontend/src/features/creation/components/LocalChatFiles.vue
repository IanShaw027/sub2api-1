<!-- Adapted from chat-vue@80649c38 AttachmentPreviewList/MessageFiles; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText, X } from '@lucide/vue'
import { localChatMessages } from './localChatMessages'

const props = defineProps<{ files: File[]; removable?: boolean; disabled?: boolean; compact?: boolean }>()
const emit = defineEmits<{ remove: [index: number]; preview: [file: File] }>()
const { t } = useI18n({ useScope: 'local', messages: localChatMessages })
const previews = ref<Array<{ file: File; url?: string }>>([])
function revoke() { previews.value.forEach(item => { if (item.url) URL.revokeObjectURL(item.url) }) }
watch(() => props.files, files => {
  revoke()
  previews.value = files.map(file => ({ file, url: file.type.startsWith('image/') ? URL.createObjectURL(file) : undefined }))
}, { immediate: true, deep: true })
onBeforeUnmount(revoke)
function size(value: number) { return value >= 1024 * 1024 ? `${(value / 1024 / 1024).toFixed(1)} MB` : `${Math.max(1, Math.round(value / 1024))} KB` }
</script>

<template>
  <div v-if="files.length" class="local-chat-files" :class="{ compact }">
    <div v-for="(item, index) in previews" :key="`${item.file.name}-${item.file.lastModified}-${index}`" class="local-chat-file" :class="{ 'is-image': item.url }" :title="item.file.name">
      <button type="button" class="local-chat-file-preview" :aria-label="`${t('preview')}: ${item.file.name}`" @click="emit('preview', item.file)">
        <img v-if="item.url" :src="item.url" :alt="item.file.name" />
        <template v-else><FileText :size="22" /><span class="local-chat-file-name">{{ item.file.name }}</span><span class="local-chat-file-size">{{ size(item.file.size) }}</span></template>
      </button>
      <button v-if="removable" type="button" class="local-chat-file-remove" :disabled="disabled" :aria-label="`${t('removeAttachment')}: ${item.file.name}`" :title="t('removeAttachment')" @click="emit('remove', index)"><X :size="13" /></button>
    </div>
  </div>
</template>

<style scoped>
.local-chat-files { display: flex; gap: 10px; flex-wrap: wrap; padding-bottom: 12px; }
.local-chat-file { position: relative; width: 144px; height: 106px; min-width: 0; border: 1px solid var(--border); border-radius: 8px; overflow: hidden; background: var(--surface-secondary); }
.local-chat-file-preview { display: flex; flex-direction: column; justify-content: center; align-items: center; gap: 6px; width: 100%; height: 100%; padding: 12px; border: 0; color: var(--muted); background: none; }
.local-chat-file.is-image { width: 132px; height: 132px; }
.is-image .local-chat-file-preview { padding: 0; }
.local-chat-file-preview img { width: 100%; height: 100%; object-fit: cover; }
.local-chat-file-name { width: 100%; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--foreground); font-size: 11px; }
.local-chat-file-size { font-size: 10px; color: var(--muted); }
.local-chat-file-remove { position: absolute; right: 4px; top: 4px; display: grid; place-items: center; width: 22px; height: 22px; padding: 0; border: 0; border-radius: 5px; background: var(--surface); color: var(--foreground); }
.compact { flex-wrap: nowrap; overflow-x: auto; padding-bottom: 10px; }
.compact .local-chat-file, .compact .local-chat-file.is-image { width: 80px; height: 80px; flex-shrink: 0; }
.compact .local-chat-file-preview { padding: 8px; gap: 4px; }
.compact .is-image .local-chat-file-preview { padding: 0; }
</style>
