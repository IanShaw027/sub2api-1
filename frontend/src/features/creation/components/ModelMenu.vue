<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import UiSelect from '@/components/ui/UiSelect.vue'
import { useCreationStore } from '../stores/creation'

const { t } = useI18n()
const store = useCreationStore()

const modelOptions = computed(() =>
  store.availableModels.map((id) => ({ value: id, label: id })),
)

async function onModelChange(value: string | number | boolean | null) {
  if (typeof value === 'string') {
    await store.setModel(value)
  }
}
</script>

<template>
  <div class="studio-model-menu">
    <label class="text-xs font-medium text-muted">{{ t('studio.models') }}</label>
    <UiSelect
      :model-value="store.model"
      :options="modelOptions"
      :placeholder="t('studio.selectModel')"
      searchable="auto"
      @update:model-value="onModelChange"
    />
  </div>
</template>

<style scoped>
.studio-model-menu {
  display: flex;
  flex-direction: column;
  gap: 6px;
}
</style>
