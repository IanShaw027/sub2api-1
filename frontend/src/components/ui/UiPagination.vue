<script setup lang="ts">
import Pagination from '@/components/common/Pagination.vue'

defineOptions({ inheritAttrs: false })

withDefaults(
  defineProps<{
    page: number
    pageSize: number
    total: number
    pageSizeOptions?: number[]
    showPageSizeSelector?: boolean
    showJump?: boolean
  }>(),
  {
    showPageSizeSelector: true,
    showJump: false
  }
)

defineEmits<{
  'update:page': [page: number]
  'update:pageSize': [size: number]
}>()
</script>

<template>
  <Pagination
    v-bind="$attrs"
    class="ui-pagination"
    :page="page"
    :page-size="pageSize"
    :total="total"
    :page-size-options="pageSizeOptions"
    :show-page-size-selector="showPageSizeSelector"
    :show-jump="showJump"
    @update:page="$emit('update:page', $event)"
    @update:page-size="$emit('update:pageSize', $event)"
  />
</template>

<style scoped>
.ui-pagination {
  border-color: color-mix(in oklch, var(--border) 80%, transparent);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  color: var(--muted);
  font-size: 12.5px;
}

.ui-pagination :deep(button) {
  border-radius: 8px;
  border-color: var(--border);
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--foreground);
}

.ui-pagination :deep(button:disabled) {
  opacity: 0.45;
}

.ui-pagination :deep(.input),
.ui-pagination :deep(.select-trigger) {
  height: 36px;
  border-radius: var(--radius-field);
  box-shadow: var(--field-shadow);
}
</style>
