<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import MessageContent from './MessageContent.vue'
import { useCreationStore } from '../stores/creation'

const { t } = useI18n()
const store = useCreationStore()
</script>

<template>
  <div class="studio-message-stream">
    <div v-if="store.messagesLoading" class="text-sm text-muted">
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
      >
        <MessageContent :role="message.role" :content="message.content" />
      </article>

      <article v-if="store.streaming" class="studio-message studio-message-assistant">
        <MessageContent role="assistant" :content="store.streamingContent" :streaming="true" />
        <p class="text-xs text-muted">{{ t('studio.streaming') }}</p>
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
</style>
