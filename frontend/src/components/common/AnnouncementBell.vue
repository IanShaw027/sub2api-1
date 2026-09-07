<template>
 <div>
 <!-- 铃铛按钮 -->
 <button
 @click="openModal"
 class="header-icon-btn relative"
 :class="{ 'text-accent': unreadCount > 0 }"
 :aria-label="t('announcements.title')"
 :title="t('announcements.title')"
 >
 <Icon name="bell" size="md" />
 <!-- 未读计数徽章 -->
 <span
 v-if="unreadCount > 0"
 class="count-badge absolute -right-0.5 -top-0.5 shadow-[0_0_0_2px_var(--surface)]"
 >
 {{ unreadCount > 99 ? '99+' : unreadCount }}
 </span>
 </button>

 <!-- 公告列表 Modal -->
 <Teleport to="body">
 <Transition name="modal">
 <div
 v-if="isModalOpen"
 class="modal-overlay"
 @click="closeModal"
 >
 <div
 class="modal-content w-full max-w-[620px]"
 @click.stop
 >
 <!-- Header -->
 <div class="modal-header">
 <div class="flex items-center gap-2.5">
 <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-[color-mix(in_oklch,var(--accent)_14%,transparent)] text-accent">
 <Icon name="bell" size="sm" />
 </div>
 <div>
 <h2 class="modal-title">{{ t('announcements.title') }}</h2>
 <p v-if="unreadCount > 0" class="modal-subtitle">
 <span class="font-medium text-accent">{{ unreadCount }}</span>
 {{ t('announcements.unread') }}
 </p>
 </div>
 </div>
 <div class="flex items-center gap-2">
 <button
 v-if="unreadCount > 0"
 @click="markAllAsRead"
 :disabled="loading"
 class="btn btn-primary btn-sm"
 >
 {{ t('announcements.markAllRead') }}
 </button>
 <button
 @click="closeModal"
 class="modal-close"
 :aria-label="t('common.close')"
 >
 <Icon name="x" size="sm" />
 </button>
 </div>
 </div>

 <!-- Body -->
 <div class="modal-body !p-0">
 <div class="sticky top-0 z-10 border-b border-line bg-[color-mix(in_oklch,var(--surface)_95%,transparent)] px-5 py-3 backdrop-blur">
 <div class="segmented">
 <button
 v-for="option in readStatusOptions"
 :key="option.value"
 @click="changeReadStatus(option.value)"
 class="segmented-item"
 :class="{ 'segmented-item-active': selectedReadStatus === option.value }"
 >
 {{ option.label }}
 </button>
 </div>
 </div>

 <!-- Loading -->
 <div v-if="loading" class="flex items-center justify-center py-16">
 <LoadingSpinner size="lg" />
 </div>

 <!-- Announcements List -->
 <div v-else-if="announcements.length > 0">
 <div
 v-for="item in announcements"
 :key="item.id"
 class="group relative flex items-center gap-4 border-b border-line px-5 py-4 transition-colors hover:bg-surface-2"
 :class="{ 'bg-[color-mix(in_oklch,var(--accent)_5%,transparent)]': !item.read_at }"
 style="min-height: 72px"
 @click="openDetail(item)"
 >
 <!-- Status Indicator -->
 <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center">
 <div
 v-if="!item.read_at"
 class="flex h-10 w-10 items-center justify-center rounded-xl bg-[color-mix(in_oklch,var(--accent)_14%,transparent)] text-accent"
 >
 <Icon name="bell" size="sm" />
 </div>
 <div
 v-else
 class="flex h-10 w-10 items-center justify-center rounded-xl bg-surface-2 text-muted"
 >
 <Icon name="check" size="sm" />
 </div>
 </div>

 <!-- Content -->
 <div class="flex min-w-0 flex-1 items-center justify-between gap-4">
 <div class="min-w-0 flex-1">
 <h3 class="truncate text-sm font-medium text-foreground">
 {{ item.title }}
 </h3>
 <div class="mt-1 flex items-center gap-2">
 <time class="text-xs text-muted">
 {{ formatRelativeTime(item.created_at) }}
 </time>
 <span v-if="!item.read_at" class="tag tag-accent">
 {{ t('announcements.unread') }}
 </span>
 </div>
 </div>

 <!-- Arrow -->
 <Icon
 name="chevronRight"
 size="sm"
 class="flex-shrink-0 text-muted transition-transform group-hover:translate-x-1"
 />
 </div>

 <!-- Unread indicator bar -->
 <div
 v-if="!item.read_at"
 class="absolute left-0 top-0 h-full w-[3px] rounded-full bg-accent"
 ></div>
 </div>
 </div>

 <!-- Empty State -->
 <div v-else class="flex flex-col items-center justify-center py-16">
 <div class="mb-3 flex h-16 w-16 items-center justify-center rounded-full bg-surface-2">
 <Icon name="inbox" size="xl" class="text-muted" />
 </div>
 <p class="text-sm font-medium text-foreground">{{ emptyTitle }}</p>
 <p class="mt-1 text-xs text-muted">{{ emptyDescription }}</p>
 </div>
 </div>
 </div>
 </div>
 </Transition>
 </Teleport>

 <!-- 公告详情 Modal -->
 <Teleport to="body">
 <Transition name="modal">
 <div
 v-if="detailModalOpen && selectedAnnouncement"
 class="modal-overlay"
 @click="closeDetail"
 >
 <div
 class="modal-content w-full max-w-[780px]"
 @click.stop
 >
 <!-- Header -->
 <div class="modal-header items-start">
 <div class="min-w-0 flex-1">
 <div class="mb-3 flex items-center gap-2">
 <div class="flex h-9 w-9 items-center justify-center rounded-xl bg-[color-mix(in_oklch,var(--accent)_14%,transparent)] text-accent">
 <Icon name="bell" size="sm" />
 </div>
 <span class="tag">{{ t('announcements.title') }}</span>
 <span v-if="!selectedAnnouncement.read_at" class="tag tag-accent">
 {{ t('announcements.unread') }}
 </span>
 </div>

 <h2 class="mb-2 text-xl font-bold leading-tight text-foreground">
 {{ selectedAnnouncement.title }}
 </h2>

 <div class="flex items-center gap-4 text-sm text-muted">
 <div class="flex items-center gap-1.5">
 <Icon name="clock" size="sm" />
 <time>{{ formatRelativeWithDateTime(selectedAnnouncement.created_at) }}</time>
 </div>
 <div class="flex items-center gap-1.5">
 <Icon name="eye" size="sm" />
 <span>{{ selectedAnnouncement.read_at ? t('announcements.read') : t('announcements.unread') }}</span>
 </div>
 </div>
 </div>

 <button
 @click="closeDetail"
 class="modal-close"
 :aria-label="t('common.close')"
 >
 <Icon name="x" size="md" />
 </button>
 </div>

 <!-- Body -->
 <div class="modal-body">
 <div class="relative pl-6">
 <div class="absolute left-0 top-0 bottom-0 w-[3px] rounded-full bg-accent"></div>
 <div
 class="markdown-body prose prose-sm max-w-none"
 v-html="renderMarkdown(selectedAnnouncement.content)"
 ></div>
 </div>
 </div>

 <!-- Footer -->
 <div class="modal-footer !justify-between">
 <div class="flex items-center gap-2 text-xs text-muted">
 <Icon name="bell" size="sm" />
 <span>{{ selectedAnnouncement.read_at ? t('announcements.readStatus') : t('announcements.markReadHint') }}</span>
 </div>
 <div class="flex items-center gap-2">
 <button
 @click="closeDetail"
 class="btn btn-secondary btn-sm"
 >
 {{ t('common.close') }}
 </button>
 <button
 v-if="!selectedAnnouncement.read_at"
 @click="markAsReadAndClose(selectedAnnouncement.id)"
 class="btn btn-primary btn-sm"
 >
 <Icon name="check" size="sm" />
 {{ t('announcements.markRead') }}
 </button>
 </div>
 </div>
 </div>
 </div>
 </Transition>
 </Teleport>
 </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { storeToRefs } from 'pinia'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useAppStore } from '@/stores/app'
import { useAnnouncementStore } from '@/stores/announcements'
import { formatRelativeTime, formatRelativeWithDateTime } from '@/utils/format'
import { acquireOverlayLock, releaseOverlayLock } from '@/components/ui/overlayLock'
import type { AnnouncementReadStatusFilter, UserAnnouncement } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import '@/styles/announcement-markdown.css'

const { t } = useI18n()
const appStore = useAppStore()
const announcementStore = useAnnouncementStore()

// Configure marked
marked.setOptions({
 breaks: true,
 gfm: true,
})

// Use store state (storeToRefs for reactivity)
const { announcements, loading, readStatus } = storeToRefs(announcementStore)
const unreadCount = computed(() => announcementStore.unreadCount)
const selectedReadStatus = computed(() => readStatus.value)
const readStatusOptions = computed(() => [
 { value: 'all' as AnnouncementReadStatusFilter, label: t('announcements.filters.all') },
 { value: 'unread' as AnnouncementReadStatusFilter, label: t('announcements.filters.unread') },
 { value: 'read' as AnnouncementReadStatusFilter, label: t('announcements.filters.read') },
])
const emptyTitle = computed(() => {
 switch (selectedReadStatus.value) {
 case 'unread':
 return t('announcements.emptyUnread')
 case 'read':
 return t('announcements.emptyRead')
 default:
 return t('announcements.empty')
 }
})
const emptyDescription = computed(() => {
 switch (selectedReadStatus.value) {
 case 'unread':
 return t('announcements.emptyUnreadDescription')
 case 'read':
 return t('announcements.emptyReadDescription')
 default:
 return t('announcements.emptyDescription')
 }
})

// Local modal state
const isModalOpen = ref(false)
const detailModalOpen = ref(false)
const selectedAnnouncement = ref<UserAnnouncement | null>(null)

// Methods
function renderMarkdown(content: string): string {
 if (!content) return ''
 const html = marked.parse(content) as string
 return DOMPurify.sanitize(html)
}

function openModal() {
 isModalOpen.value = true
 announcementStore.fetchAnnouncements(true, selectedReadStatus.value)
}

function closeModal() {
 isModalOpen.value = false
}

function openDetail(announcement: UserAnnouncement) {
 selectedAnnouncement.value = announcement
 detailModalOpen.value = true
 if (!announcement.read_at) {
 markAsRead(announcement.id)
 }
}

function closeDetail() {
 detailModalOpen.value = false
 selectedAnnouncement.value = null
}

async function markAsRead(id: number) {
 try {
 await announcementStore.markAsRead(id)
 } catch (err: any) {
 appStore.showError(err?.message || t('common.unknownError'))
 }
}

async function markAsReadAndClose(id: number) {
 await markAsRead(id)
 appStore.showSuccess(t('announcements.markedAsRead'))
 closeDetail()
}

async function markAllAsRead() {
 try {
 await announcementStore.markAllAsRead()
 appStore.showSuccess(t('announcements.allMarkedAsRead'))
 } catch (err: any) {
 appStore.showError(err?.message || t('common.unknownError'))
 }
}

async function changeReadStatus(filter: AnnouncementReadStatusFilter) {
 if (filter === selectedReadStatus.value && announcements.value.length > 0) {
 return
 }
 try {
 await announcementStore.fetchAnnouncements(true, filter)
 } catch (err: any) {
 appStore.showError(err?.message || t('common.unknownError'))
 }
}

function handleEscape(e: KeyboardEvent) {
 if (e.key === 'Escape') {
 if (detailModalOpen.value) {
 closeDetail()
 } else if (isModalOpen.value) {
 closeModal()
 }
 }
}

let overlayHeld = false

function syncOverlayLock(needLock: boolean) {
 if (needLock && !overlayHeld) {
 acquireOverlayLock()
 overlayHeld = true
 return
 }
 if (!needLock && overlayHeld) {
 releaseOverlayLock()
 overlayHeld = false
 }
}

onMounted(() => {
 document.addEventListener('keydown', handleEscape)
})

onBeforeUnmount(() => {
 document.removeEventListener('keydown', handleEscape)
 syncOverlayLock(false)
})

watch(
 [isModalOpen, detailModalOpen, () => announcementStore.currentPopup],
 ([modal, detail, popup]) => {
 syncOverlayLock(Boolean(modal || detail || popup))
 }
)
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

.overflow-y-auto::-webkit-scrollbar-thumb:hover {
 background: color-mix(in oklch, var(--muted) 65%, transparent);
}
</style>
