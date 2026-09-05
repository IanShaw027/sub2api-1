<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import UiSelect from '@/components/ui/UiSelect.vue'
import { useCreationStore } from '../stores/creation'

const props = withDefaults(
  defineProps<{
    compact?: boolean
  }>(),
  {
    compact: false,
  },
)

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
  <div class="studio-model-menu" :class="{ 'studio-model-menu-compact': props.compact }">
    <label v-if="!props.compact" class="text-xs font-medium text-muted">{{ t('studio.models') }}</label>
    <UiSelect
      :model-value="store.model"
      :options="modelOptions"
      :placeholder="t('studio.selectModel')"
      :aria-label="props.compact ? t('studio.models') : undefined"
      :disabled="store.isImageSession && !store.hasImageModels"
      searchable="auto"
      @update:model-value="onModelChange"
    />
    <p v-if="!props.compact && store.isImageSession && !store.hasImageModels" class="text-xs text-muted">
      {{ t('studio.emptyImageModels') }}
    </p>
  </div>
</template>

<style scoped>
.studio-model-menu {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.studio-model-menu-compact {
  gap: 0;
  min-width: 160px;
}
</style>
