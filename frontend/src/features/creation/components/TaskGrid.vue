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
  <div class="studio-task-grid">
    <div v-if="store.imageTasksLoading" class="text-sm text-muted">
      {{ t('common.loading') }}
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

.studio-task-grid-inner {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}
</style>
