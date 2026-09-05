<template>
  <div
    v-if="visibleCount > 0 || page > 1"
    class="batch-pagination-bar flex flex-col gap-3 border-t border-line bg-surface px-4 py-2.5 sm:flex-row sm:items-center sm:justify-between"
  >
    <div class="flex flex-wrap items-center gap-3 text-[12.5px] text-muted tabular-nums">
      <i18n-t keypath="batchImage.pagination.pageNumber" tag="span" scope="global">
        <template #page>
          <span class="font-medium">{{ page }}</span>
        </template>
      </i18n-t>
      <i18n-t keypath="batchImage.pagination.pageItems" tag="span" scope="global">
        <template #count>
          <span class="font-medium">{{ visibleCount }}</span>
        </template>
      </i18n-t>
      <div class="flex items-center gap-2">
        <span>{{ t('pagination.perPage') }}</span>
        <Select
          :model-value="pageSize"
          :options="pageSizeOptions"
          class="batch-page-size w-24"
          @change="$emit('change-page-size', $event)"
        />
      </div>
    </div>
    <div class="flex items-center justify-end gap-2">
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="page <= 1 || loading"
        @click="$emit('change-page', page - 1)"
      >
        <Icon name="chevronLeft" size="sm" class="mr-1" />
        {{ t('pagination.previous') }}
      </button>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="!hasMore || loading"
        @click="$emit('change-page', page + 1)"
      >
        {{ t('pagination.next') }}
        <Icon name="chevronRight" size="sm" class="ml-1" />
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import type { SelectOption } from '@/components/common/Select.vue'

defineProps<{
  visibleCount: number
  page: number
  pageSize: number
  hasMore: boolean
  loading: boolean
  pageSizeOptions: SelectOption[]
}>()

defineEmits<{
  'change-page': [page: number]
  'change-page-size': [value: string | number | boolean | null]
}>()

const { t } = useI18n()
</script>

<style scoped>
/* Match the shared Pagination footer: 10 + 30 + 10 + 1 border = 51px. */
.batch-pagination-bar .btn-sm {
  height: 30px;
}

.batch-page-size :deep(.select-trigger) {
  height: 30px;
  min-height: 30px;
  font-size: 12px;
}
</style>
