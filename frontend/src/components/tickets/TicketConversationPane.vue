<template>
  <div class="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border bg-white dark:border-dark-700 dark:bg-dark-800">
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ title }}</h2>
      <p v-if="subtitle" class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ subtitle }}</p>
    </div>

    <div ref="messageContainerRef" class="min-h-0 flex-1 space-y-4 overflow-y-auto px-5 py-4">
      <template v-if="messages.length > 0">
        <div
          v-for="message in messages"
          :key="message.id || `${message.sender_role}-${message.created_at}-${message.content}`"
          class="flex gap-3"
          :class="message.sender_role === 'user' ? 'justify-end' : 'justify-start'"
        >
          <template v-if="message.message_type === 'system'">
            <div class="w-full py-2">
              <div class="mx-auto flex max-w-2xl flex-wrap items-center justify-center gap-2 rounded-full bg-gray-50 px-4 py-2 text-center text-xs text-gray-500 dark:bg-dark-700/50 dark:text-gray-400">
                <span class="break-words">{{ message.content }}</span>
                <span class="text-gray-400 dark:text-gray-500">{{ formatDateTime(message.created_at) }}</span>
              </div>
            </div>
          </template>

          <template v-else>
            <template v-if="message.sender_role !== 'user'">
              <div class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-200 text-sm font-semibold text-gray-600 dark:bg-dark-600 dark:text-gray-200">
                <span>{{ (message.sender_name_snapshot || '?').slice(0, 1).toUpperCase() }}</span>
              </div>
            </template>

            <div class="max-w-[80%]">
              <div class="mb-1 flex items-center gap-2" :class="message.sender_role === 'user' ? 'justify-end' : 'justify-start'">
                <span class="text-sm font-medium text-gray-900 dark:text-white">{{ message.sender_name_snapshot }}</span>
                <span class="whitespace-nowrap text-xs text-gray-500 dark:text-gray-400">{{ formatDateTime(message.created_at) }}</span>
              </div>
              <div
                class="rounded-2xl px-4 py-3 text-sm leading-6"
                :class="bubbleClass(message.sender_role)"
              >
                <div v-if="message.content" class="whitespace-pre-wrap break-words">{{ message.content }}</div>
                <div v-if="message.attachments?.length" class="mt-2 flex flex-wrap gap-2" :class="{ 'mt-0': !message.content }">
                  <button
                    v-for="att in message.attachments"
                    :key="att.media_id"
                    type="button"
                    class="overflow-hidden rounded-lg border text-xs transition-colors"
                    :class="message.sender_role === 'user'
                      ? 'border-white/30 hover:bg-white/10'
                      : 'border-gray-200 hover:bg-gray-50 dark:border-dark-600 dark:hover:bg-dark-700'"
                    @click="openAttachment(att.media_id)"
                  >
                    <img
                      v-if="att.content_type?.startsWith('image/') && previewUrls[att.media_id]"
                      :src="previewUrls[att.media_id]"
                      :alt="att.file_name"
                      class="h-24 w-24 object-cover"
                    />
                    <span v-else class="flex items-center gap-2 px-3 py-2">
                      <span class="max-w-[160px] truncate">{{ att.file_name }}</span>
                      <span :class="message.sender_role === 'user' ? 'text-white/70' : 'text-gray-400'">{{ formatFileSize(att.size_bytes) }}</span>
                    </span>
                  </button>
                </div>
              </div>
            </div>

            <template v-if="message.sender_role === 'user'">
              <div class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-200 text-sm font-semibold text-gray-600 dark:bg-dark-600 dark:text-gray-200">
                <span>{{ (message.sender_name_snapshot || '?').slice(0, 1).toUpperCase() }}</span>
              </div>
            </template>
          </template>
        </div>
      </template>
      <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ emptyText }}
      </div>
    </div>

    <div v-if="showComposer" class="border-t border-gray-100 px-5 py-4 dark:border-dark-700">
      <textarea
        v-model="composerValue"
        class="input min-h-[96px]"
        :placeholder="composerPlaceholder"
        @compositionstart="handleCompositionStart"
        @compositionend="handleCompositionEnd"
        @keydown="handleComposerKeydown"
      />
      <div v-if="pendingAttachments.length > 0" class="mt-2 flex flex-wrap gap-2">
        <div
          v-for="(att, idx) in pendingAttachments"
          :key="att.media_id"
          class="group relative overflow-hidden rounded-lg border border-gray-200 dark:border-dark-600"
        >
          <img
            v-if="att.preview_url && att.content_type?.startsWith('image/')"
            :src="att.preview_url"
            :alt="att.file_name"
            class="h-16 w-16 object-cover"
          />
          <div v-else class="flex h-16 w-16 items-center justify-center bg-gray-50 text-xs text-gray-500 dark:bg-dark-700">
            {{ att.file_name?.split('.').pop() }}
          </div>
          <button
            type="button"
            class="absolute -right-1 -top-1 flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100"
            @click="removePendingAttachment(idx)"
          >
            &times;
          </button>
        </div>
      </div>
      <div class="mt-3 flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <slot name="composer-actions" />
          <label class="btn btn-secondary btn-sm cursor-pointer">
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

function bubbleClass(role: SupportTicketMessage['sender_role']) {
  if (role === 'user') {
    return 'bg-blue-600 text-white'
  }
  return 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-100'
}
</script>
