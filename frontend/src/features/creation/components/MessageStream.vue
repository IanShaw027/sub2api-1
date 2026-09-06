<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { CornerDownLeft } from '@lucide/vue'
import { useClipboard } from '@/composables/useClipboard'
import MessageContent from './MessageContent.vue'
import { useCreationStore } from '../stores/creation'
import { formatDateTimeToMinute } from '@/utils/format'
import type { CreationMessage } from '../types'
import creationAPI from '../api'

const { t, locale } = useI18n()
const store = useCreationStore()
const props = withDefaults(defineProps<{ interactive?: boolean; fullHeight?: boolean }>(), { interactive: false, fullHeight: false })
const emit = defineEmits<{ reuse: [text: string] }>()
const { copyToClipboard } = useClipboard()
const streamRef = ref<HTMLElement | null>(null)
const followingBottom = ref(true)
const reuseLabel = computed(() => locale?.value?.startsWith('zh') ? '复用提示词' : 'Reuse prompt')
const reuseDisabled = computed(() => store.streaming || store.sessionLoading || store.messagesLoading || store.modelUpdating || store.hasUnsavedExchange || !store.generationAvailable)

function onScroll() {
  const el = streamRef.value
  if (el) followingBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 80
}

watch(() => [store.selectedSessionId, store.messages.length, store.streamingContent], async (value, previous) => {
  if (!props.fullHeight) return
  const follow = followingBottom.value || value[0] !== previous?.[0]
  await nextTick()
  if (follow && streamRef.value) streamRef.value.scrollTop = streamRef.value.scrollHeight
}, { immediate: true })

function messageLabel(message: CreationMessage): string {
  const timestamp = formatDateTimeToMinute(message.created_at)
  if (message.role === 'user') return t('studio.a11y.userMessage', { timestamp })
  if (message.role === 'assistant') return t('studio.a11y.assistantMessage', { timestamp })
  return t('studio.a11y.systemMessage', { timestamp })
}

</script>

<template>
  <div
    ref="streamRef"
    class="studio-message-stream"
    :class="{ 'studio-message-stream-full': fullHeight }"
    role="log"
    aria-live="polite"
    aria-relevant="additions text"
    :aria-label="t('studio.a11y.messageLog')"
    :aria-busy="store.messagesLoading || store.streaming"
    @scroll="onScroll"
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
        <div class="studio-message-bubble">
          <div class="studio-message-meta">
            <span v-if="message.role === 'assistant' && message.model" class="studio-message-meta-name">{{ message.model }}</span>
            <span class="studio-message-meta-time">{{ formatDateTimeToMinute(message.created_at) }}</span>
          </div>
          <MessageContent
            :role="message.role"
            :content="message.content"
            :input-tokens="message.input_tokens"
            :output-tokens="message.output_tokens"
          />
          <div v-if="interactive && creationAPI.extractMessageText(message.content)" class="studio-message-actions">
            <button type="button" class="icon-btn" :title="t('common.copy')" :aria-label="t('common.copy')" @click="copyToClipboard(creationAPI.extractMessageText(message.content))">
              <Icon name="copy" size="sm" />
            </button>
            <button v-if="message.role === 'user'" type="button" class="icon-btn" :title="reuseLabel" :aria-label="reuseLabel" :disabled="reuseDisabled" @click="emit('reuse', creationAPI.extractMessageText(message.content))">
              <CornerDownLeft :size="16" aria-hidden="true" />
            </button>
          </div>
        </div>
      </article>

      <article
        v-if="store.streaming"
        class="studio-message studio-message-assistant"
        :aria-label="t('studio.a11y.assistantStreaming')"
        aria-live="polite"
        aria-busy="true"
      >
        <div class="studio-message-bubble">
          <MessageContent role="assistant" :content="store.streamingContent" :streaming="true" />
          <p class="text-xs text-muted" role="status">{{ t('studio.streaming') }}</p>
        </div>
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

@media (min-width: 1101px) {
  .studio-message-stream {
    max-height: none;
  }
}

.studio-message-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.studio-message {
  display: flex;
  max-width: 82%;
  min-width: 0;
  overflow-wrap: anywhere;
}

.studio-message-actions {
  display: flex;
  gap: 4px;
  margin-top: 8px;
}

.studio-message-actions .icon-btn {
  width: 32px;
  height: 32px;
  color: var(--muted);
}

.studio-message-stream.studio-message-stream-full {
  flex: 1;
  width: 100%;
  max-height: none;
  padding: 20px max(20px, calc((100% - 800px) / 2));
}

.studio-message-stream-full .studio-message-list {
  gap: 24px;
}

.studio-message-stream-full .studio-message-assistant {
  width: 100%;
  max-width: 100%;
}

.studio-message-stream-full .studio-message-assistant .studio-message-bubble {
  width: 100%;
  padding: 0;
  background: transparent;
}

.studio-message-user {
  align-self: flex-end;
}

.studio-message-assistant,
.studio-message-system {
  align-self: flex-start;
}

.studio-message-bubble {
  min-width: 0;
  border-radius: var(--radius-field);
  padding: 12px 14px;
  background: var(--surface-secondary);
}

.studio-message-user .studio-message-bubble {
  background: color-mix(in oklch, var(--accent) 12%, transparent);
}

.studio-message-meta {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 4px;
  font-size: 12.5px;
  color: var(--muted);
}

.studio-message-meta-name {
  font-weight: 600;
}

.studio-message-meta-time {
  font-family: var(--font-mono);
}

@media (max-width: 767px) {
  .studio-message-stream {
    max-height: min(38vh, 360px);
  }

  .studio-message {
    max-width: 92%;
  }
}
</style>
