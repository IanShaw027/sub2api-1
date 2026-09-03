<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import MessageContent from './MessageContent.vue'
import { useCreationStore } from '../stores/creation'
import { formatDateTimeToMinute } from '@/utils/format'
import type { CreationMessage } from '../types'

const { t } = useI18n()
const store = useCreationStore()

function messageLabel(message: CreationMessage): string {
  const timestamp = formatDateTimeToMinute(message.created_at)
  if (message.role === 'user') return t('studio.a11y.userMessage', { timestamp })
  if (message.role === 'assistant') return t('studio.a11y.assistantMessage', { timestamp })
  return t('studio.a11y.systemMessage', { timestamp })
}
</script>

<template>
  <div
    class="studio-message-stream"
    role="log"
    aria-live="polite"
    aria-relevant="additions text"
    :aria-label="t('studio.a11y.messageLog')"
    :aria-busy="store.messagesLoading || store.streaming"
  >
    <div v-if="store.messagesLoading" class="text-sm text-muted" role="status" aria-live="polite">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="store.messages.length === 0 && !store.streaming" class="text-sm text-muted">
      {{ t('studio.emptyMessages') }}
    </div>

    <div v-else class="studio-message-list">
      <article
        v-for="message in store.messages"
        :key="message.id"
        class="studio-message"
        :class="`studio-message-${message.role}`"
        :aria-label="messageLabel(message)"
      >
        <MessageContent
          :role="message.role"
          :content="message.content"
          :input-tokens="message.input_tokens"
          :output-tokens="message.output_tokens"
        />
      </article>

      <article
        v-if="store.streaming"
        class="studio-message studio-message-assistant"
        :aria-label="t('studio.a11y.assistantStreaming')"
        aria-live="polite"
        aria-busy="true"
      >
        <MessageContent role="assistant" :content="store.streamingContent" :streaming="true" />
        <p class="text-xs text-muted" role="status">{{ t('studio.streaming') }}</p>
      </article>
    </div>
  </div>
</template>

<style scoped>
.studio-message-stream {
  min-height: 0;
  overflow: auto;
  max-height: min(62vh, 680px);
}

.studio-message-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.studio-message {
  border: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
  border-radius: 14px;
  padding: 12px 14px;
  background: color-mix(in oklch, var(--surface) 82%, transparent);
}

.studio-message-user {
  background: color-mix(in oklch, var(--accent) 8%, var(--surface));
}

.studio-message-assistant {
  background: color-mix(in oklch, var(--surface-secondary, var(--surface)) 88%, transparent);
}

@media (max-width: 767px) {
  .studio-message-stream {
    max-height: min(38vh, 360px);
  }
}
</style>
