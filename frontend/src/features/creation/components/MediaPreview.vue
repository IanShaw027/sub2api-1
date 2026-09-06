<!-- Adapted from chat-vue@80649c38 MediaPreviewOverlay.vue; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, Download, FileText, ImagePlus, Pencil, Plus, RotateCcw, Share2, Video } from '@lucide/vue'
import UiModal from '@/components/ui/UiModal.vue'
import Button from '@/components/ui/Button.vue'
import type { MediaTask } from '../localMedia'
import { mediaMessages } from './mediaMessages'
import MediaBillingLabel from './MediaBillingLabel.vue'

const props = defineProps<{ task: MediaTask | null; source?: { url: string; name: string; mode?: 'image' | 'video' } | null }>()
const emit = defineEmits<{ close: []; action: [action: string, task: MediaTask] }>()
const { t } = useI18n({ useScope: 'local', messages: mediaMessages })
const mediaFailed = ref(false)
watch(() => [props.task?.id, props.source?.url], () => { mediaFailed.value = false })
function action(name: string) { if (props.task) emit('action', name, props.task) }
</script>

<template>
  <UiModal :open="Boolean(task || source)" :title="source ? t('sourcePreview') : t('preview')" :subtitle="task?.model" width="xl" :close-label="t('close')" @close="emit('close')">
    <div class="media-preview-stage">
      <div v-if="mediaFailed" class="media-preview-error"><span>{{ t('unavailable') }}</span><Button v-if="task" variant="secondary" size="sm" @click="action('retry')"><RotateCcw :size="14" />{{ t('regenerate') }}</Button></div>
      <video v-else-if="task?.mode === 'video' && task.mediaUrl" :src="task.mediaUrl" controls autoplay loop playsinline @error="mediaFailed = true" />
      <video v-else-if="source?.mode === 'video'" :src="source.url" controls playsinline @error="mediaFailed = true" />
      <img v-else-if="task?.mediaUrl || source?.url" :src="task?.mediaUrl || source?.url" :alt="task?.revisedPrompt || task?.prompt || source?.name || ''" @error="mediaFailed = true" />
    </div>
    <div v-if="task" class="media-preview-details">
      <MediaBillingLabel :cost="task.billing" :can-refresh="!!task.providerTaskId" @refresh="action('billing')" />
      <p v-if="task.observationPersisted === false" class="text-warning-text text-xs" role="status">{{ t('observationWarning') }}</p>
      <div class="media-preview-label"><span>{{ t('prompt') }}</span><button type="button" :title="t('copyPrompt')" :aria-label="t('copyPrompt')" @click="action('copyPrompt')"><Copy :size="14" /></button></div>
      <p class="media-preview-prompt">{{ task.prompt }}</p>
      <template v-if="task.revisedPrompt"><div class="media-preview-label"><span>{{ t('revisedPrompt') }}</span><button type="button" :title="t('copyPrompt')" :aria-label="t('copyPrompt')" @click="action('copyRevised')"><Copy :size="14" /></button></div><p class="media-preview-prompt">{{ task.revisedPrompt }}</p></template>
      <dl class="media-preview-meta"><div v-if="task.mode === 'video'"><dt>{{ t('videoOperation') }}</dt><dd>{{ t(task.videoOperation === 'edit' ? 'editVideo' : task.videoOperation === 'extension' ? 'extendVideo' : 'generateVideo') }}</dd></div><div v-if="task.sourceDuration"><dt>{{ t('sourceDuration') }}</dt><dd>{{ t('seconds', { count: Number(task.sourceDuration.toFixed(2)) }) }}</dd></div><div v-if="task.settings.ratio || task.settings.aspectRatio"><dt>{{ t('ratio') }}</dt><dd>{{ task.settings.ratio || task.settings.aspectRatio }}</dd></div><div v-if="task.settings.size || task.settings.resolution"><dt>{{ t('resolution') }}</dt><dd>{{ task.settings.size || task.settings.resolution }}</dd></div><div v-if="task.settings.quality"><dt>{{ t('quality') }}</dt><dd>{{ task.settings.quality }}</dd></div><div v-if="task.settings.duration"><dt>{{ t(task.videoOperation === 'extension' ? 'extensionDuration' : 'duration') }}</dt><dd>{{ t('seconds', { count: task.settings.duration }) }}</dd></div></dl>
    </div>
    <template v-if="task" #footer>
      <div class="media-preview-actions">
        <Button v-if="task.mode === 'video' && task.status === 'completed' && task.blob" variant="ghost" size="sm" @click="action('editVideo')"><Pencil :size="14" />{{ t('editVideo') }}</Button>
        <Button v-if="task.mode === 'video' && task.status === 'completed' && task.blob" variant="ghost" size="sm" @click="action('extendVideo')"><Plus :size="14" />{{ t('extendVideo') }}</Button>
        <Button v-if="task.mode === 'image'" variant="ghost" size="sm" @click="action('edit')"><Pencil :size="14" />{{ t('edit') }}</Button>
        <Button v-if="task.mode === 'image'" variant="ghost" size="sm" @click="action('reference')"><ImagePlus :size="14" />{{ t('reference') }}</Button>
        <Button v-if="task.mode === 'image'" variant="ghost" size="sm" @click="action('video')"><Video :size="14" />{{ t('toVideo') }}</Button>
        <Button v-if="task.mode === 'image'" variant="ghost" size="sm" @click="action('copy')"><Copy :size="14" />{{ t('copy') }}</Button>
        <Button variant="ghost" size="sm" @click="action('reuse')"><FileText :size="14" />{{ t('reuse') }}</Button>
        <Button variant="secondary" size="sm" @click="action('publish')"><Share2 :size="14" />{{ t('publish') }}</Button>
        <Button size="sm" @click="action('download')"><Download :size="14" />{{ t('download') }}</Button>
      </div>
    </template>
  </UiModal>
</template>

<style scoped>
.media-preview-stage { min-height: 160px; display: flex; align-items: center; justify-content: center; background: var(--surface-secondary); border-radius: 8px; overflow: hidden; }
.media-preview-stage img, .media-preview-stage video { display: block; max-height: 58dvh; max-width: 100%; object-fit: contain; }
.media-preview-error { display: flex; flex-direction: column; align-items: center; gap: 12px; padding: 32px; color: var(--muted); }
.media-preview-details { padding-top: 16px; }
.media-preview-label { display: flex; justify-content: space-between; align-items: center; color: var(--muted); font-size: 11px; margin-bottom: 6px; }
.media-preview-label button { border: 0; color: var(--muted); background: none; padding: 5px; }
.media-preview-prompt { font-size: 13px; line-height: 1.65; white-space: pre-wrap; overflow-wrap: anywhere; max-height: 140px; overflow: auto; color: var(--foreground); margin-bottom: 14px; }
.media-preview-meta { display: flex; flex-wrap: wrap; gap: 18px; padding-top: 10px; border-top: 1px solid var(--border); font-size: 11px; }
.media-preview-meta dt { color: var(--muted); margin-bottom: 4px; }
.media-preview-meta dd { color: var(--foreground); font-family: var(--font-mono); }
.media-preview-actions { display: flex; justify-content: flex-end; gap: 6px; flex-wrap: wrap; }
</style>
