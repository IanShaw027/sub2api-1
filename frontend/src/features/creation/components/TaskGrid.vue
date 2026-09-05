<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import TaskCard from './TaskCard.vue'
import { useCreationStore } from '../stores/creation'

const emit = defineEmits<{
  preview: [url: string]
}>()

const { t } = useI18n()
const store = useCreationStore()
</script>

<template>
  <div class="studio-task-grid" :aria-busy="store.imageTasksLoading">
    <div
      v-if="store.imageTasksLoading && store.imageTasks.length === 0"
      class="text-sm text-muted"
      role="status"
      aria-live="polite"
      :aria-label="t('studio.a11y.loadingTasks')"
    >
      {{ t('common.loading') }}
    </div>

    <div
      v-else-if="store.isImageSession && !store.hasImageModels && store.imageTasks.length === 0"
      class="studio-task-empty"
    >
      <p class="text-sm text-foreground">{{ t('studio.emptyImageModels') }}</p>
      <p class="text-xs text-muted">{{ t('studio.errors.noImageModels') }}</p>
    </div>

    <div v-else-if="store.imageTasks.length === 0" class="text-sm text-muted">
      {{ t('studio.emptyTasks') }}
    </div>

    <div v-else class="studio-task-grid-inner">
      <TaskCard
        v-for="task in store.imageTasks"
        :key="task.id"
        :task="task"
        @preview="emit('preview', $event)"
      />
    </div>
  </div>
</template>

<style scoped>
.studio-task-grid {
  min-height: 0;
  overflow: auto;
  max-height: min(62vh, 680px);
}

@media (min-width: 1101px) {
  .studio-task-grid {
    max-height: none;
  }
}

.studio-task-grid-inner {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}

.studio-task-empty {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 12px 4px;
}
</style>
