<template>
 <div class="glass-card conversation-pane">
 <div class="card-header">
 <p class="card-title">{{ title }}</p>
 <p v-if="subtitle" class="card-subtitle font-mono">{{ subtitle }}</p>
 </div>

 <div ref="messageContainerRef" class="conversation-messages">
 <template v-if="messages.length > 0">
 <div
 v-for="message in messages"
 :key="message.id || `${message.sender_role}-${message.created_at}-${message.content}`"
 class="message-row"
 :class="message.sender_role === 'user' ? 'justify-end' : 'justify-start'"
 >
 <template v-if="message.message_type === 'system'">
 <div class="system-message-wrap">
 <div class="system-message">
 <span class="break-words">{{ message.content }}</span>
 <span class="font-mono">{{ formatDateTime(message.created_at) }}</span>
 </div>
 </div>
 </template>

 <template v-else>
 <template v-if="message.sender_role !== 'user'">
 <div class="message-avatar">
 <span>{{ (message.sender_name_snapshot || '?').slice(0, 1).toUpperCase() }}</span>
 </div>
 </template>

 <div class="message-content">
 <div class="message-meta" :class="message.sender_role === 'user' ? 'justify-end' : 'justify-start'">
 <span class="message-sender">{{ message.sender_name_snapshot }}</span>
 <span class="message-time font-mono">{{ formatDateTime(message.created_at) }}</span>
 </div>
 <div
 class="message-bubble"
 :class="message.sender_role === 'user' ? 'message-bubble-me' : 'message-bubble-other'"
 >
 <div v-if="message.content" class="whitespace-pre-wrap break-words">{{ message.content }}</div>
 <div v-if="message.attachments?.length" class="attachment-list" :class="{ 'mt-0': !message.content }">
 <template v-for="att in message.attachments" :key="att.media_id">
 <button
 v-if="att.content_type?.startsWith('image/') && previewUrls[att.media_id]"
 type="button"
 class="attachment-thumb"
 @click="openAttachment(att.media_id)"
 >
 <img :src="previewUrls[att.media_id]" :alt="att.file_name" />
 </button>
 <button v-else type="button" class="chip attachment-chip" @click="openAttachment(att.media_id)">
 <span class="max-w-[140px] truncate">{{ att.file_name }}</span>
 <span class="attachment-size">{{ formatFileSize(att.size_bytes) }}</span>
 </button>
 </template>
 </div>
 </div>
 </div>

 <template v-if="message.sender_role === 'user'">
 <div class="message-avatar">
 <span>{{ (message.sender_name_snapshot || '?').slice(0, 1).toUpperCase() }}</span>
 </div>
 </template>
 </template>
 </div>
 </template>
 <div v-else class="conversation-empty">
 {{ emptyText }}
 </div>
 </div>

 <div v-if="showComposer" class="composer">
 <textarea
 v-model="composerValue"
 class="field composer-textarea"
 :placeholder="composerPlaceholder"
 @compositionstart="handleCompositionStart"
 @compositionend="handleCompositionEnd"
 @keydown="handleComposerKeydown"
 />
 <div v-if="pendingAttachments.length > 0" class="pending-attachments">
 <div
 v-for="(att, idx) in pendingAttachments"
 :key="att.media_id"
 class="group pending-attachment"
 >
 <img
 v-if="att.preview_url && att.content_type?.startsWith('image/')"
 :src="att.preview_url"
 :alt="att.file_name"
 />
 <div v-else class="pending-attachment-icon">
 {{ att.file_name?.split('.').pop() }}
 </div>
 <button
 type="button"
 class="pending-attachment-remove"
 @click="removePendingAttachment(idx)"
 >
 &times;
 </button>
 </div>
 </div>
 <div class="composer-actions">
 <div class="flex items-center gap-2">
 <slot name="composer-actions" />
 <label class="btn btn-secondary cursor-pointer">
 <input
 type="file"
 multiple
 class="hidden"
 :disabled="uploadingAttachment"
 @change="handleAttachmentUpload"
 />
 {{ uploadingAttachment ? t('tickets.uploading') : t('tickets.attachImage') }}
 </label>
 </div>
 <button class="btn btn-primary" :disabled="sending || uploadingAttachment || (!composerValue.trim() && pendingAttachments.length === 0)" @click="submitReply">
 {{ sending ? resolvedSendingText : resolvedSubmitText }}
 </button>
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import type { TicketUploadResult } from '@/utils/tickets'
import type { SupportTicketMessage, TicketAttachment } from '@/types/ticket'

type PendingTicketAttachment = TicketAttachment & { preview_url?: string }

const props = withDefaults(defineProps<{
 title: string
 subtitle?: string
 messages: SupportTicketMessage[]
 emptyText: string
 showComposer?: boolean
 sending?: boolean
 composerPlaceholder?: string
 submitText?: string
 sendingText?: string
 clearComposerKey?: number
 replyContent?: string
 ticketId?: number
 uploadFn?: (file: File, ticketId: number | string) => Promise<TicketUploadResult>
 downloadFn?: (mediaId: number) => Promise<string>
}>(), {
 subtitle: '',
 showComposer: true,
 sending: false,
 composerPlaceholder: '',
 clearComposerKey: 0,
 replyContent: undefined,
 ticketId: 0,
 uploadFn: undefined,
 downloadFn: undefined,
})

const emit = defineEmits<{
 reply: [content: string, attachments?: { media_id: number }[]]
 'update:replyContent': [content: string]
 'upload-error': [error: unknown]
}>()
const { t } = useI18n()
const localReplyContent = ref('')
const messageContainerRef = ref<HTMLDivElement | null>(null)
const isComposing = ref(false)
const attachmentUploadGeneration = ref(0)
const pendingAttachments = ref<PendingTicketAttachment[]>([])
const uploadingAttachment = ref(false)
const previewUrls = ref<Record<number, string>>({})
const resolvedSubmitText = computed(() => props.submitText ?? t('common.submit'))
const resolvedSendingText = computed(() => props.sendingText ?? t('common.submitting'))
const composerValue = computed({
 get() {
 return props.replyContent ?? localReplyContent.value
 },
 set(value: string) {
 if (props.replyContent !== undefined) {
 emit('update:replyContent', value)
 return
 }
 localReplyContent.value = value
 },
})

watch(() => props.messages.length, () => {
 nextTick(() => {
 const container = messageContainerRef.value
 if (container) {
 container.scrollTop = container.scrollHeight
 }
 })
}, { immediate: true })

watch(
 () => props.messages.map((message) => message.attachments?.map((att) => att.media_id).join(',') || '').join('|'),
 () => { void loadAttachmentPreviews() },
 { immediate: true },
)

watch(() => props.clearComposerKey, () => {
 composerValue.value = ''
 clearPendingAttachments()
})

watch(() => props.ticketId, (nextTicketID, previousTicketID) => {
 if (nextTicketID === previousTicketID) return
 attachmentUploadGeneration.value += 1
 composerValue.value = ''
 clearPendingAttachments()
 uploadingAttachment.value = false
 previewUrls.value = {}
})

function submitReply() {
 if (props.sending || uploadingAttachment.value) return
 const content = composerValue.value.trim()
 const atts = pendingAttachments.value.length > 0
 ? pendingAttachments.value.map((a) => ({ media_id: a.media_id }))
 : undefined
 if (!content && !atts) return
 emit('reply', content, atts)
}

async function handleAttachmentUpload(event: Event) {
 const input = event.target as HTMLInputElement
 const files = Array.from(input.files || [])
 if (files.length === 0 || !props.uploadFn) return

 const ticketId = props.ticketId
 const uploadGeneration = attachmentUploadGeneration.value
 uploadingAttachment.value = true
 try {
 for (const file of files) {
 if (!isCurrentAttachmentUpload(ticketId, uploadGeneration)) {
 break
 }
 const result = await props.uploadFn(file, ticketId)
 if (!isCurrentAttachmentUpload(ticketId, uploadGeneration)) {
 break
 }
 if (!result?.id) {
 continue
 }
 const previewURL = file.type.startsWith('image/') ? URL.createObjectURL(file) : ''
 pendingAttachments.value.push({
 media_id: result.id,
 file_name: result.original_file_name || result.filename || file.name,
 content_type: result.mime_type || result.mime || file.type,
 size_bytes: result.size_bytes || result.size || file.size,
 preview_url: previewURL || undefined,
 })
 }
 } catch (error) {
 if (isCurrentAttachmentUpload(ticketId, uploadGeneration)) {
 emit('upload-error', error)
 }
 } finally {
 if (isCurrentAttachmentUpload(ticketId, uploadGeneration)) {
 uploadingAttachment.value = false
 }
 input.value = ''
 }
}

function isCurrentAttachmentUpload(ticketId: number | undefined, uploadGeneration: number) {
 return props.ticketId === ticketId && attachmentUploadGeneration.value === uploadGeneration
}

function removePendingAttachment(index: number) {
 const [removed] = pendingAttachments.value.splice(index, 1)
 if (removed?.preview_url) URL.revokeObjectURL(removed.preview_url)
}

function clearPendingAttachments() {
 for (const attachment of pendingAttachments.value) {
 if (attachment.preview_url) URL.revokeObjectURL(attachment.preview_url)
 }
 pendingAttachments.value = []
}

onBeforeUnmount(() => {
 clearPendingAttachments()
})

async function loadAttachmentPreviews() {
 if (!props.downloadFn) return
 const images = props.messages.flatMap((message) =>
 (message.attachments || []).filter((att) => att.content_type?.startsWith('image/')),
 )
 for (const att of images) {
 if (previewUrls.value[att.media_id]) continue
 try {
 const url = await props.downloadFn(att.media_id)
 if (url) previewUrls.value = { ...previewUrls.value, [att.media_id]: url }
 } catch {
 // Grant failures fall back to the filename chip.
 }
 }
}

async function openAttachment(mediaId: number) {
 if (!props.downloadFn) return
 const url = await props.downloadFn(mediaId)
 if (url) window.open(url, '_blank')
}

function formatFileSize(bytes: number): string {
 if (bytes < 1024) return `${bytes} B`
 if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
 return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function handleCompositionStart() {
 isComposing.value = true
}

function handleCompositionEnd() {
 isComposing.value = false
}

function handleComposerKeydown(event: KeyboardEvent) {
 if (event.key !== 'Enter' || event.shiftKey) {
 return
 }
 if (event.isComposing || isComposing.value) {
 return
 }
 event.preventDefault()
 submitReply()
}
</script>

<style scoped>
.conversation-pane {
 display: flex;
 height: 100%;
 min-height: 0;
 flex-direction: column;
 overflow: hidden;
 padding: 0;
}

.conversation-messages {
 min-height: 0;
 flex: 1;
 overflow-y: auto;
 padding: 16px 18px;
 display: flex;
 flex-direction: column;
 gap: 16px;
}

.conversation-empty {
 display: flex;
 height: 100%;
 align-items: center;
 justify-content: center;
 font-size: 13px;
 color: var(--muted);
}

.message-row {
 display: flex;
 gap: 10px;
}

.system-message-wrap {
 width: 100%;
 padding: 4px 0;
}

.system-message {
 margin: 0 auto;
 display: flex;
 max-width: 32rem;
 flex-wrap: wrap;
 align-items: center;
 justify-content: center;
 gap: 8px;
 border-radius: 999px;
 background: var(--surface-secondary);
 padding: 8px 16px;
 text-align: center;
 font-size: 12px;
 color: var(--muted);
}

.message-avatar {
 display: flex;
 height: 32px;
 width: 32px;
 flex: none;
 align-items: center;
 justify-content: center;
 border-radius: 999px;
 background: color-mix(in oklch, var(--accent) 18%, transparent);
 color: var(--accent);
 font-size: 13px;
 font-weight: 700;
}

.message-content {
 max-width: 80%;
 min-width: 0;
}

.message-meta {
 margin-bottom: 5px;
 display: flex;
 align-items: center;
 gap: 8px;
}

.message-sender {
 font-size: 12.5px;
 font-weight: 600;
 color: var(--foreground);
}

.message-time {
 white-space: nowrap;
 font-size: 12.5px;
 color: var(--muted);
}

.message-bubble {
 border-radius: var(--radius-field);
 padding: 12px 14px;
 font-size: 13px;
 line-height: 1.6;
}

.message-bubble-other {
 background: var(--surface-secondary);
 color: var(--foreground);
}

.message-bubble-me {
 background: color-mix(in oklch, var(--accent) 12%, transparent);
 color: var(--foreground);
}

.attachment-list {
 margin-top: 8px;
 display: flex;
 flex-wrap: wrap;
 gap: 8px;
}

.attachment-thumb {
 overflow: hidden;
 border-radius: 10px;
 border: 1px solid var(--border);
 padding: 0;
 cursor: pointer;
}

.attachment-thumb img {
 display: block;
 height: 72px;
 width: 72px;
 object-fit: cover;
}

.attachment-chip {
 cursor: pointer;
 height: 26px;
}

.attachment-size {
 color: var(--muted);
}

.composer {
 border-top: 1px solid var(--border);
 padding: 14px 18px;
}

.composer-textarea {
 min-height: 96px;
 padding: 10px 12px;
}

.pending-attachments {
 margin-top: 10px;
 display: flex;
 flex-wrap: wrap;
 gap: 8px;
}

.pending-attachment {
 position: relative;
 overflow: hidden;
 border-radius: 10px;
 border: 1px solid var(--border);
 height: 56px;
 width: 56px;
}

.pending-attachment img {
 height: 100%;
 width: 100%;
 object-fit: cover;
}

.pending-attachment-icon {
 display: flex;
 height: 100%;
 width: 100%;
 align-items: center;
 justify-content: center;
 background: var(--surface-secondary);
 font-size: 11px;
 color: var(--muted);
}

.pending-attachment-remove {
 position: absolute;
 right: -1px;
 top: -1px;
 display: flex;
 height: 18px;
 width: 18px;
 align-items: center;
 justify-content: center;
 border-radius: 999px;
 background: var(--danger);
 color: white;
 font-size: 11px;
 opacity: 0;
 transition: opacity 0.15s ease;
}

.pending-attachment:hover .pending-attachment-remove {
 opacity: 1;
}

.composer-actions {
 margin-top: 12px;
 display: flex;
 flex-wrap: wrap;
 align-items: center;
 justify-content: space-between;
 gap: 12px;
}

@media (max-width: 767px) {
 .conversation-pane {
 min-height: 0;
 }

 .message-content {
 max-width: 88%;
 }

 .composer {
 position: sticky;
 bottom: 0;
 background: color-mix(in oklch, var(--surface) 92%, transparent);
 backdrop-filter: blur(20px);
 }

 .composer-actions .btn {
 min-height: 44px;
 }
}
</style>
