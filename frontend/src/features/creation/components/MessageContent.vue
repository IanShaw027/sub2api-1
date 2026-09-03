<script setup lang="ts">
import { computed } from 'vue'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import creationAPI from '../api'
import type { CreationMessageRole } from '../types'

const props = defineProps<{
  role: CreationMessageRole | 'assistant' | 'user' | 'system'
  content: unknown
  streaming?: boolean
}>()

const text = computed(() => creationAPI.extractMessageText(props.content))

const html = computed(() => {
  if (props.role !== 'assistant') return ''
  const raw = marked.parse(text.value || '', { breaks: true }) as string
  return DOMPurify.sanitize(raw)
})
</script>

<template>
  <div class="studio-message-content">
    <div v-if="role === 'assistant'" class="studio-markdown text-foreground" v-html="html" />
    <p v-else class="text-sm text-foreground whitespace-pre-wrap">{{ text }}</p>
    <span v-if="streaming" class="studio-cursor" aria-hidden="true">▍</span>
  </div>
</template>

<style scoped>
.studio-message-content {
  font-size: 14px;
  line-height: 1.55;
}

.studio-markdown :deep(p) {
  margin: 0 0 8px;
}

.studio-markdown :deep(p:last-child) {
  margin-bottom: 0;
}

.studio-markdown :deep(pre) {
  background: var(--code-bg);
  border-radius: 10px;
  padding: 10px 12px;
  overflow: auto;
}

.studio-cursor {
  display: inline-block;
  margin-left: 2px;
  animation: studio-cursor-pulse 1s step-end infinite;
}

@keyframes studio-cursor-pulse {
  50% {
    opacity: 0;
  }
}
</style>
