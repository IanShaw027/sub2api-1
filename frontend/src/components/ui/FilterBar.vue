<template>
  <div class="ui-filter-bar">
    <div class="ui-filter-bar-search">
      <slot name="search">
        <TextInput
          :model-value="search"
          :placeholder="searchPlaceholder"
          :aria-label="searchPlaceholder || 'Search'"
          @update:model-value="$emit('update:search', String($event))"
        />
      </slot>
    </div>
    <button
      v-if="isMobile && $slots.filters"
      type="button"
      class="ui-filter-bar-toggle"
      :aria-label="filterLabel"
      :aria-expanded="filtersExpanded"
      :aria-controls="filtersId"
      @click="toggleFilters"
    >
      <Icon name="filter" size="sm" />
    </button>
    <div v-if="$slots.filters" :id="filtersId" class="ui-filter-bar-filters" :class="{ 'is-mobile-hidden': isMobile && !filtersExpanded }">
      <slot name="filters" />
    </div>
    <div v-if="$slots.trailing || $slots.actions" class="ui-filter-bar-trailing">
      <slot name="trailing" />
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, useId } from 'vue'
import { useIsMobile } from '@/composables/useIsMobile'
import Icon from '@/components/icons/Icon.vue'
import TextInput from './TextInput.vue'

withDefaults(
  defineProps<{
    search?: string
    searchPlaceholder?: string
    filterLabel?: string
  }>(),
  {
    search: '',
    searchPlaceholder: '',
    filterLabel: 'Filter'
  }
)

const emit = defineEmits<{
  'update:search': [value: string]
  'open-filters': []
}>()

const { isMobile } = useIsMobile()
const filtersExpanded = ref(false)
const filtersId = `filter-bar-${useId()}`

function toggleFilters() {
  filtersExpanded.value = !filtersExpanded.value
  if (filtersExpanded.value) emit('open-filters')
}
</script>

<style scoped>
.ui-filter-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-bottom: 14px;
}

.ui-filter-bar-search {
  flex: 1 1 220px;
  min-width: 180px;
}

.ui-filter-bar-filters,
.ui-filter-bar-trailing {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.ui-filter-bar-filters.is-mobile-hidden {
  display: none;
}

.ui-filter-bar-toggle {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  flex: none;
  border-radius: var(--radius-field);
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--foreground);
  box-shadow: inset 0 1px 0 var(--btn-hi), 0 1px 2px rgba(16, 24, 40, 0.06);
  cursor: pointer;
}

@media (max-width: 767px) {
  .ui-filter-bar-filters {
    flex-basis: 100%;
    min-width: 0;
  }

  .ui-filter-bar-filters > :deep(*) {
    max-width: 100%;
  }

  .ui-filter-bar-search {
    flex: 1 1 auto;
    min-width: 0;
  }

  .ui-filter-bar-search :deep(.field) {
    height: 44px;
  }
}
</style>
