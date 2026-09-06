<!-- Adapted from chat-vue@80649c38 MediaTaskCard.vue; see MediaReferenceLicense.txt. -->
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Check, Clock, Download, Ellipsis, Eye, Image as ImageIcon, LoaderCircle, Pencil, Plus, RotateCcw, Save, Trash2, Video } from '@lucide/vue'
import type { MediaTask } from '../localMedia'
import { mediaMessages } from './mediaMessages'
import MediaBillingLabel from './MediaBillingLabel.vue'

const props = defineProps<{ task: MediaTask; selected?: boolean; editing?: boolean; batch?: boolean; now: number; list?: boolean }>()
const emit = defineEmits<{ action: [action: string, task: MediaTask] }>()
const { t, locale } = useI18n({ useScope: 'local', messages: mediaMessages })
const failedMedia = ref(false)
const menuOpen = ref(false)
const menuUp = ref(false)
const root = ref<HTMLElement | null>(null)
function closeOutside(event: MouseEvent) { if (!root.value?.contains(event.target as Node)) menuOpen.value = false }
watch(menuOpen, open => {
  if (open) document.addEventListener('click', closeOutside)
  else document.removeEventListener('click', closeOutside)
})
watch(() => props.task.mediaUrl, () => { failedMedia.value = false })
onBeforeUnmount(() => document.removeEventListener('click', closeOutside))
const hasMedia = computed(() => Boolean(props.task.mediaUrl))
const elapsed = computed(() => Math.max(0, Math.round(((props.task.completedAt ? new Date(props.task.completedAt).getTime() : props.now) - new Date(props.task.createdAt).getTime()) / 1000)))
const meta = computed(() => [props.task.model, props.task.settings.ratio || props.task.settings.aspectRatio, props.task.mode === 'video' ? props.task.settings.resolution : props.task.settings.size, props.task.mode === 'video' && props.task.settings.duration ? `${props.task.videoOperation === 'extension' ? '+' : ''}${props.task.settings.duration}s` : null].filter(Boolean).join(' · '))
const timestamp = computed(() => new Intl.DateTimeFormat(locale.value, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(new Date(props.task.createdAt)))

function action(value: string) {
  menuOpen.value = false
  emit('action', value, props.task)
}
function toggleMenu(event: MouseEvent) {
  const trigger = event.currentTarget as HTMLElement
  menuUp.value = trigger.getBoundingClientRect().bottom + 300 > window.innerHeight - 200
  menuOpen.value = !menuOpen.value
}
function play(event: MouseEvent) {
  void (event.currentTarget as HTMLVideoElement).play().catch(() => undefined)
}
function pause(event: MouseEvent) {
  const video = event.currentTarget as HTMLVideoElement
  video.pause()
  video.currentTime = 0
}
</script>

<template>
  <article ref="root" class="media-task" :class="{ 'is-selected': selected || editing, 'is-list': list }" :data-task-id="task.id">
    <div class="media-task-media">
      <div v-if="task.status === 'generating'" class="media-task-state" role="status">
        <LoaderCircle class="media-spin" :size="24" />
        <strong>{{ t('processing') }}</strong>
        <span>{{ t('elapsed', { seconds: elapsed }) }}</span>
        <p>{{ task.prompt }}</p>
      </div>
      <div v-else-if="task.status === 'error' || failedMedia || !hasMedia" class="media-task-state">
        <ImageIcon v-if="task.mode === 'image'" :size="24" /><Video v-else :size="24" />
        <strong>{{ task.status === 'error' ? t('failed') : t('unavailable') }}</strong>
        <p v-if="task.error" class="media-task-error" role="alert">{{ task.error }}</p>
        <div class="media-state-actions">
          <button type="button" @click="action(task.providerTaskId ? 'resume' : 'retry')"><RotateCcw :size="14" />{{ t(task.providerTaskId ? 'resume' : 'retry') }}</button>
          <button v-if="task.providerTaskId" type="button" :title="t('regenerate')" :aria-label="t('regenerate')" @click="action('retry')"><ImageIcon :size="14" /></button>
          <button type="button" :title="t('remove')" :aria-label="t('remove')" @click="action('delete')"><Trash2 :size="14" /></button>
        </div>
      </div>
      <button v-else type="button" class="media-task-open" :aria-label="batch ? t('select') : t('preview')" @click="action(batch ? 'select' : 'preview')">
        <video v-if="task.mode === 'video'" :src="task.mediaUrl" preload="metadata" muted playsinline loop @mouseenter="play" @mouseleave="pause" @error="failedMedia = true" />
        <img v-else :src="task.mediaUrl" :alt="task.revisedPrompt || task.prompt" loading="lazy" decoding="async" @error="failedMedia = true" />
      </button>
      <button v-if="batch" type="button" class="media-task-check" :class="{ checked: selected }" :aria-label="t('select')" :aria-pressed="selected" @click="action('select')"><Check :size="15" /></button>
      <span v-else class="media-task-kind"><ImageIcon v-if="task.mode === 'image'" :size="12" /><Video v-else :size="12" />{{ task.mode === 'image' ? t('images') : t('videos') }}</span>
      <div v-if="!batch && task.status === 'completed'" class="media-task-tools">
        <button type="button" :title="t('preview')" :aria-label="t('preview')" @click="action('preview')"><Eye :size="15" /></button>
        <button type="button" :title="t('download')" :aria-label="t('download')" @click="action('download')"><Download :size="15" /></button>
        <button type="button" :title="t('more')" :aria-label="t('more')" :aria-expanded="menuOpen" @click="toggleMenu"><Ellipsis :size="15" /></button>
      </div>
      <div v-if="menuOpen" class="media-task-menu" :class="{ 'opens-up': menuUp }" @keydown.esc="menuOpen = false">
        <button v-if="task.mode === 'video' && task.blob" type="button" @click="action('editVideo')"><Pencil :size="14" />{{ t('editVideo') }}</button>
        <button v-if="task.mode === 'video' && task.blob" type="button" @click="action('extendVideo')"><Plus :size="14" />{{ t('extendVideo') }}</button>
        <button v-if="task.mode === 'image'" type="button" @click="action('edit')"><Pencil :size="14" />{{ t('edit') }}</button>
        <button v-if="task.mode === 'image'" type="button" @click="action('reference')">{{ t('reference') }}</button>
        <button v-if="task.mode === 'image'" type="button" @click="action('video')"><Video :size="14" />{{ t('toVideo') }}</button>
        <button type="button" @click="action('reuse')">{{ t('reuse') }}</button>
        <button v-if="task.mode === 'image'" type="button" @click="action('copy')">{{ t('copy') }}</button>
        <button type="button" @click="action('publish')">{{ t('publish') }}</button>
        <button type="button" @click="action('retry')"><RotateCcw :size="14" />{{ t('regenerate') }}</button>
        <button type="button" class="is-danger" @click="action('delete')"><Trash2 :size="14" />{{ t('remove') }}</button>
      </div>
    </div>
    <div class="media-task-body">
      <p class="media-task-prompt" :title="task.prompt">{{ task.prompt }}</p>
      <p class="media-task-meta" :title="meta">{{ meta }}</p>
      <MediaBillingLabel :cost="task.status === 'generating' ? task.estimate : task.billing" :can-refresh="!!task.providerTaskId && task.status !== 'generating'" @refresh="action('billing')" />
      <p v-if="task.observationPersisted === false" class="media-task-warning" :title="task.observationWarning" role="status">{{ t('observationWarning') }}</p>
      <div class="media-task-bottom"><span>{{ timestamp }}</span><span v-if="elapsed"><Clock :size="11" />{{ elapsed }}s</span><span class="media-task-type">{{ task.mode === 'video' && task.videoOperation === 'edit' ? t('editVideo') : task.mode === 'video' && task.videoOperation === 'extension' ? t('extendVideo') : task.type === 'edit' ? (task.mode === 'video' ? t('imageToVideo') : t('editing')) : t('generation') }}</span></div>
      <button v-if="task.status === 'completed' && !task.persisted" type="button" class="media-save-local" @click="action('save')"><Save :size="13" />{{ t('saveLocal') }}</button>
    </div>
  </article>
</template>

<style scoped>
.media-task-warning { color: var(--warning-text); font-size: 11px; line-height: 1.5; overflow-wrap: anywhere; margin: 6px 0; }
.media-task { position: relative; min-width: 0; border: 1px solid var(--border); border-radius: var(--radius-card); background: color-mix(in oklch, var(--surface) 85%, transparent); box-shadow: var(--shadow); overflow: visible; transition: border-color 150ms ease, box-shadow 150ms ease; }
.media-task:hover { box-shadow: var(--shadow-hover); }
.media-task.is-selected { border-color: var(--accent); box-shadow: 0 0 0 1px var(--accent); }
.media-task-media { position: relative; aspect-ratio: 1; border-radius: var(--radius-card) var(--radius-card) 0 0; background: var(--surface-secondary); }
.media-task-open { display: block; border: 0; padding: 0; width: 100%; height: 100%; background: none; overflow: hidden; border-radius: inherit; }
.media-task-open img, .media-task-open video { width: 100%; height: 100%; object-fit: cover; }
.media-task-state { height: 100%; display: flex; flex-direction: column; justify-content: center; align-items: center; gap: 8px; padding: 28px 18px; text-align: center; color: var(--muted); font-size: 12px; }
.media-task-state strong { color: var(--foreground); font-size: 13px; }
.media-task-state p { overflow: hidden; display: -webkit-box; -webkit-line-clamp: 3; -webkit-box-orient: vertical; overflow-wrap: anywhere; }
.media-task-error { color: var(--danger-text); }
.media-task-kind { position: absolute; left: 9px; top: 9px; display: inline-flex; align-items: center; gap: 4px; border-radius: 6px; padding: 4px 6px; background: var(--surface); color: var(--foreground); font-size: 10px; pointer-events: none; }
.media-task-check { position: absolute; top: 9px; left: 9px; display: grid; place-items: center; width: 28px; height: 28px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: transparent; }
.media-task-check.checked { color: var(--on-tone); background: var(--accent); border-color: var(--accent); }
.media-task-tools { position: absolute; right: 8px; top: 8px; display: flex; gap: 4px; opacity: 0; transition: opacity 120ms ease; }
.media-task:hover .media-task-tools, .media-task:focus-within .media-task-tools { opacity: 1; }
.media-task-tools button, .media-state-actions button { display: inline-flex; justify-content: center; align-items: center; gap: 5px; min-width: 28px; min-height: 28px; border: 1px solid var(--border); border-radius: 6px; background: var(--surface); color: var(--foreground); padding: 5px; }
.media-state-actions { display: flex; gap: 6px; }
.media-task-menu { position: absolute; z-index: 15; right: 8px; top: 42px; width: min(184px, calc(100% - 16px)); padding: 5px; background: var(--surface); border: 1px solid var(--border); border-radius: 8px; box-shadow: var(--shadow-pop); }
.media-task-menu.opens-up { top: auto; bottom: calc(100% - 8px); }
.media-task-menu button { display: flex; align-items: center; gap: 8px; width: 100%; text-align: left; padding: 8px; border: 0; border-radius: 4px; background: none; color: var(--foreground); font-size: 12px; }
.media-task-menu button:hover { background: var(--surface-secondary); }
.media-task-menu .is-danger { color: var(--danger-text); }
.media-task-body { padding: 11px 12px; min-width: 0; }
.media-task-prompt { color: var(--foreground); font-size: 12px; line-height: 1.5; overflow: hidden; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; min-height: 36px; overflow-wrap: anywhere; }
.media-task-meta { margin-top: 7px; font-size: 10px; font-family: var(--font-mono); color: var(--muted); overflow: hidden; white-space: nowrap; text-overflow: ellipsis; }
.media-task-bottom { display: flex; align-items: center; flex-wrap: wrap; gap: 7px; margin-top: 6px; font-size: 10px; color: var(--muted); }
.media-task-bottom > span { display: inline-flex; align-items: center; gap: 3px; }
.media-task-type { margin-left: auto; color: var(--foreground); }
.media-save-local { display: flex; align-items: center; gap: 5px; margin-top: 10px; padding: 0; border: 0; background: none; color: var(--danger-text); font-size: 11px; }
.media-task.is-list { display: grid; grid-template-columns: 112px minmax(0, 1fr); }
.is-list .media-task-media { border-radius: var(--radius-card) 0 0 var(--radius-card); }
.is-list .media-task-body { display: flex; flex-direction: column; justify-content: center; }
.is-list .media-task-state { padding: 14px 6px; font-size: 10px; }
.is-list .media-task-state p, .is-list .media-task-state > span, .is-list .media-task-kind { display: none; }
.is-list .media-task-tools { right: 4px; top: 4px; flex-direction: column; }
.is-list .media-task-menu { top: 34px; left: 20px; }
.media-spin { animation: media-spin 1s linear infinite; }
@keyframes media-spin { to { transform: rotate(360deg); } }
@media (hover: none) { .media-task-tools { opacity: 1; } }
@media (prefers-reduced-motion: reduce) { .media-spin { animation: none; } }
</style>
