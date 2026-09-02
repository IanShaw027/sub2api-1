<template>
  <div class="ui-filter-bar">
    <div class="ui-filter-bar-search">
      <TextInput
        :model-value="search"
        :placeholder="searchPlaceholder"
        @update:model-value="$emit('update:search', String($event))"
      />
    </div>
    <div v-if="$slots.filters" class="ui-filter-bar-filters">
      <slot name="filters" />
    </div>
    <div v-if="$slots.actions" class="ui-filter-bar-actions">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import TextInput from './TextInput.vue'

withDefaults(
  defineProps<{
    search?: string
    searchPlaceholder?: string
  }>(),
  {
    search: '',
    searchPlaceholder: ''
  }
)

defineEmits<{
  'update:search': [value: string]
}>()
</script>

<style scoped>
.ui-filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
  margin-bottom: 14px;
}

.ui-filter-bar-search {
  flex: 1 1 220px;
  min-width: 180px;
}

.ui-filter-bar-filters,
.ui-filter-bar-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

@media (max-width: 767px) {
  .ui-filter-bar-filters {
    display: none;
  }
}
</style>
