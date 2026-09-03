<template>
 <Teleport to="body">
 <Transition name="modal">
 <div
 v-if="displayedAnnouncement"
 class="modal-overlay"
 >
 <div
 class="modal-content w-full max-w-[680px]"
 @click.stop
 >
 <!-- Header -->
 <div class="modal-header items-start">
 <div class="min-w-0 flex-1">
 <div class="mb-3 flex items-center gap-2">
 <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-[color-mix(in_oklch,var(--warning)_16%,transparent)] text-warning-text">
 <Icon name="bell" size="sm" />
 </div>
 <span class="tag tag-warning">
 {{ t('announcements.unread') }}
 </span>
 </div>

 <h2 class="mb-2 text-xl font-bold leading-tight text-foreground">
 {{ displayedAnnouncement.title }}
 </h2>

 <div class="flex items-center gap-1.5 text-sm text-muted">
 <Icon name="clock" size="sm" />
 <time>{{ formatRelativeWithDateTime(displayedAnnouncement.created_at) }}</time>
 </div>
 </div>
 </div>

 <!-- Body -->
 <div class="modal-body">
 <div class="relative pl-6">
 <div class="absolute left-0 top-0 bottom-0 w-[3px] rounded-full bg-[var(--warning)]"></div>
 <div
 class="markdown-body prose prose-sm max-w-none"
 v-html="renderedContent"
 ></div>
 </div>
 </div>

 <!-- Footer -->
 <div class="modal-footer">
 <button
 @click="handleDismiss"
 data-testid="announcement-popup-dismiss"
 class="btn btn-primary btn-sm"
 >
 <Icon :name="preview ? 'x' : 'check'" size="sm" />
 {{ preview ? t('common.close') : t('announcements.markRead') }}
 </button>
 </div>
 </div>
 </div>
 </Transition>
 </Teleport>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeWithDateTime } from '@/utils/format'
import type { Announcement, UserAnnouncement } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import '@/styles/announcement-markdown.css'

type PreviewAnnouncement = Pick<Announcement | UserAnnouncement, 'title' | 'content' | 'created_at'>

const props = withDefaults(defineProps<{
 announcement?: PreviewAnnouncement | null
 preview?: boolean
}>(), {
 announcement: null,
 preview: false,
})

const emit = defineEmits<{
 close: []
}>()

const { t } = useI18n()
const announcementStore = useAnnouncementStore()
const displayedAnnouncement = computed(() => (
 props.preview ? props.announcement : announcementStore.currentPopup
))

marked.setOptions({
 breaks: true,
 gfm: true,
})

const renderedContent = computed(() => {
 const content = displayedAnnouncement.value?.content
 if (!content) return ''
 const html = marked.parse(content) as string
 return DOMPurify.sanitize(html)
})

function handleDismiss() {
 if (props.preview) {
 emit('close')
 return
 }
 announcementStore.dismissPopup()
}

// Manage body overflow — only set, never unset (bell component handles restore)
watch(
 displayedAnnouncement,
 (popup) => {
 if (popup) {
 document.body.style.overflow = 'hidden'
 } else if (props.preview) {
 document.body.style.overflow = ''
 }
 },
 { immediate: true },
)

onBeforeUnmount(() => {
 if (props.preview) {
 document.body.style.overflow = ''
 }
})
</script>

<style scoped>
.overflow-y-auto::-webkit-scrollbar {
 width: 8px;
}

.overflow-y-auto::-webkit-scrollbar-track {
 background: transparent;
}

.overflow-y-auto::-webkit-scrollbar-thumb {
 background: color-mix(in oklch, var(--muted) 45%, transparent);
 border-radius: 4px;
}
</style>
