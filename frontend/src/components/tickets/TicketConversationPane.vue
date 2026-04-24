<template>
  <div class="flex h-full min-h-[520px] flex-col overflow-hidden rounded-2xl border bg-white dark:border-dark-700 dark:bg-dark-800">
    <div class="border-b border-gray-100 px-5 py-4 dark:border-dark-700">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ title }}</h2>
      <p v-if="subtitle" class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ subtitle }}</p>
    </div>

    <div class="flex-1 space-y-4 overflow-y-auto px-5 py-4">
      <template v-if="messages.length > 0">
        <div v-for="message in messages" :key="message.id || `${message.sender_role}-${message.created_at}-${message.content}`" class="flex gap-3" :class="message.sender_role === 'user' ? 'justify-end' : 'justify-start'">
          <template v-if="message.sender_role !== 'user'">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-200 text-sm font-semibold text-gray-600 dark:bg-dark-600 dark:text-gray-200">
              <img v-if="message.sender_avatar_snapshot" :src="message.sender_avatar_snapshot" :alt="message.sender_name_snapshot" class="h-full w-full object-cover" />
              <span v-else>{{ message.sender_name_snapshot.slice(0, 1).toUpperCase() }}</span>
            </div>
          </template>

          <div class="max-w-[80%]">
            <div class="mb-1 flex items-center gap-2" :class="message.sender_role === 'user' ? 'justify-end' : 'justify-start'">
              <span class="text-sm font-medium text-gray-900 dark:text-white">{{ message.sender_name_snapshot }}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatRelativeWithDateTime(message.created_at) }}</span>
            </div>
            <div
              class="rounded-2xl px-4 py-3 text-sm leading-6"
              :class="bubbleClass(message.sender_role, message.message_type)"
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
        </div>
      </template>
      <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">
        {{ emptyText }}
      </div>
    </div>

    <div v-if="showComposer" class="border-t border-gray-100 px-5 py-4 dark:border-dark-700">
      <textarea
        v-model="replyContent"
        class="input min-h-[96px]"
        :placeholder="composerPlaceholder"
      />
      <div class="mt-3 flex justify-end">
        <button class="btn btn-primary" :disabled="sending || !replyContent.trim()" @click="submitReply">
          {{ sending ? sendingText : submitText }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { formatRelativeWithDateTime } from '@/utils/format'
import type { SupportTicketMessage, TicketMessageType, TicketSenderRole } from '@/types'

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
}>(), {
  subtitle: '',
  showComposer: true,
  sending: false,
  composerPlaceholder: '',
  submitText: '发送',
  sendingText: '发送中...',
})

const emit = defineEmits<{ reply: [content: string] }>()
const replyContent = ref('')

watch(() => props.messages.length, () => {
  if (!props.sending) {
    replyContent.value = ''
  }
})

function submitReply() {
  const content = replyContent.value.trim()
  if (!content) return
  emit('reply', content)
}

function bubbleClass(role: TicketSenderRole, messageType: TicketMessageType) {
  if (messageType === 'system') {
    return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200'
  }
  if (role === 'user') {
    return 'bg-blue-600 text-white'
  }
  return 'bg-gray-100 text-gray-800 dark:bg-dark-700 dark:text-gray-100'
}
</script>
