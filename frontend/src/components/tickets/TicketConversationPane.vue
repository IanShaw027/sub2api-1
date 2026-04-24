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
              <img v-if="message.sender_avatar_snapshot" :src="message.sender_avatar_snapshot" :alt="message.sender_name_snapshot" class="h-full w-full object-cover" />
              <span v-else>{{ message.sender_name_snapshot.slice(0, 1).toUpperCase() }}</span>
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
              <div class="whitespace-pre-wrap break-words">{{ message.content }}</div>
            </div>
          </div>

          <template v-if="message.sender_role === 'user'">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-200 text-sm font-semibold text-gray-600 dark:bg-dark-600 dark:text-gray-200">
              <img v-if="message.sender_avatar_snapshot" :src="message.sender_avatar_snapshot" :alt="message.sender_name_snapshot" class="h-full w-full object-cover" />
              <span v-else>{{ message.sender_name_snapshot.slice(0, 1).toUpperCase() }}</span>
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
      <div class="mt-3 flex flex-wrap items-center justify-between gap-3">
        <div class="flex items-center gap-2">
          <slot name="composer-actions" />
        </div>
        <button class="btn btn-primary" :disabled="sending || !composerValue.trim()" @click="submitReply">
          {{ sending ? sendingText : submitText }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { formatDateTime } from '@/utils/format'
import type { SupportTicketMessage, TicketSenderRole } from '@/types'

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
}>(), {
  subtitle: '',
  showComposer: true,
  sending: false,
  composerPlaceholder: '',
  submitText: '发送',
  sendingText: '发送中...',
  clearComposerKey: 0,
  replyContent: undefined,
})

const emit = defineEmits<{
  reply: [content: string]
  'update:replyContent': [content: string]
}>()
const localReplyContent = ref('')
const messageContainerRef = ref<HTMLDivElement | null>(null)
const isComposing = ref(false)
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

watch(() => props.clearComposerKey, () => {
  composerValue.value = ''
})

function submitReply() {
  const content = composerValue.value.trim()
  if (!content) return
  emit('reply', content)
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

function bubbleClass(role: TicketSenderRole) {
  if (role === 'user') {
    return 'bg-blue-600 text-white'
  }
  return 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-100'
}
</script>
