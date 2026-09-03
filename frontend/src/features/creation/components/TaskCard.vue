<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { CreationImageJob } from '../types'

const props = defineProps<{
  task: CreationImageJob
}>()

const emit = defineEmits<{
  preview: [url: string]
}>()

const { t } = useI18n()

const tone = computed(() => {
  switch (props.task.status) {
    case 'completed':
      return 'success'
    case 'failed':
      return 'danger'
    case 'processing':
      return 'warning'
    default:
      return 'muted'
  }
})

const statusLabel = computed(() => t(`studio.taskStatus.${props.task.status}`))

function openPreview() {
  if (props.task.media_url) emit('preview', props.task.media_url)
}
</script>

<template>
  <article class="studio-task-card glass-card">
    <button
      type="button"
      class="studio-task-preview"
      :disabled="!task.media_url"
      @click="openPreview"
    >
      <img v-if="task.media_url" :src="task.media_url" alt="" class="studio-task-image" />
      <div v-else class="studio-task-placeholder text-muted">{{ statusLabel }}</div>
    </button>
    <div class="studio-task-body">
      <StatusBadge :tone="tone" dot>{{ statusLabel }}</StatusBadge>
      <p class="studio-task-prompt text-foreground">{{ task.prompt }}</p>
      <p class="text-xs text-muted">{{ task.model }}</p>
      <p v-if="task.error" class="text-xs text-danger">{{ task.error }}</p>
    </div>
  </article>
</template>

<style scoped>
.studio-task-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border-radius: 14px;
  border: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
}

.studio-task-preview {
  width: 100%;
  aspect-ratio: 1;
  border: none;
  border-radius: 12px;
  overflow: hidden;
  background: color-mix(in oklch, var(--foreground) 4%, transparent);
}

.studio-task-image {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.studio-task-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
}

.studio-task-body {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.studio-task-prompt {
  font-size: 13px;
  line-height: 1.45;
  display: -webkit-box;
  -webkit-line-clamp: 3;
  -webkit-box-orient: vertical;
  overflow: hidden;
}
</style>
