<template>
  <div class="pagination-bar">
    <!-- Mobile: prev / page / next -->
    <div class="flex flex-1 items-center justify-between gap-2 sm:hidden">
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="page === 1"
        @click="goToPage(page - 1)"
      >
        {{ t('pagination.previous') }}
      </button>
      <span class="pagination-info">
        {{ t('pagination.pageOf', { page, total: totalPages }) }}
      </span>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="page === totalPages"
        @click="goToPage(page + 1)"
      >
        {{ t('pagination.next') }}
      </button>
    </div>

    <div class="hidden gap-3 sm:flex sm:flex-1 sm:items-center sm:justify-between">
      <!-- Desktop pagination info · 12.5px muted, numbers in foreground -->
      <div class="flex flex-wrap items-center gap-x-3 gap-y-2">
        <p class="pagination-info">
          {{ t('pagination.showing') }}
          <b class="text-foreground">{{ fromItem }}</b
          >–<b class="text-foreground">{{ toItem }}</b
          >，{{ t('pagination.of') }}
          <b class="text-foreground">{{ total }}</b>
          {{ t('pagination.results') }}
          <template v-if="!showPageSizeSelector">
            · {{ t('pagination.perPage') }}
            <b class="text-foreground">{{ pageSize }}</b>
          </template>
        </p>

        <!-- Page size selector · filter-pill, 36px -->
        <div v-if="showPageSizeSelector" class="flex items-center gap-2">
          <span class="pagination-sep" aria-hidden="true">·</span>
          <div class="page-size-select">
            <Select
              variant="pill"
              :pill-label="t('pagination.perPage')"
              :aria-label="t('pagination.perPage')"
              :model-value="pageSize"
              :options="pageSizeSelectOptions"
              @update:model-value="handlePageSizeChange"
            />
          </div>
        </div>

        <div v-if="showJump" class="flex items-center gap-2">
          <span class="pagination-info">{{ t('pagination.jumpTo') }}</span>
          <input
            v-model="jumpPage"
            type="number"
            min="1"
            :max="totalPages"
            class="field pagination-jump"
            :placeholder="t('pagination.jumpPlaceholder')"
            @keyup.enter="submitJump"
          />
          <button type="button" class="btn btn-ghost btn-sm" @click="submitJump">
            {{ t('pagination.jumpAction') }}
          </button>
        </div>
      </div>

      <!-- Desktop pagination buttons · 28×28, radius 8 -->
      <nav class="flex items-center gap-1" aria-label="Pagination">
        <button
          type="button"
          class="pagination-btn"
          :disabled="page === 1"
          :aria-label="t('pagination.previous')"
          @click="goToPage(page - 1)"
        >
          <Icon name="chevronLeft" size="sm" :stroke-width="2" />
        </button>

        <template v-for="(pageNum, index) in visiblePages" :key="`${pageNum}-${index}`">
          <span v-if="typeof pageNum !== 'number'" class="pagination-ellipsis">{{ pageNum }}</span>
          <button
            v-else
            type="button"
            class="pagination-btn"
            :class="{ 'pagination-btn-current': pageNum === page }"
            :aria-label="t('pagination.goToPage', { page: pageNum })"
            :aria-current="pageNum === page ? 'page' : undefined"
            @click="goToPage(pageNum)"
          >
            {{ pageNum }}
          </button>
        </template>

        <button
          type="button"
          class="pagination-btn"
          :disabled="page === totalPages"
          :aria-label="t('pagination.next')"
          @click="goToPage(page + 1)"
        >
          <Icon name="chevronRight" size="sm" :stroke-width="2" />
        </button>
      </nav>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Select from './Select.vue'
import { getConfiguredTablePageSizeOptions, normalizeTablePageSize } from '@/utils/tablePreferences'
import { setPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()

interface Props {
  total: number
  page: number
  pageSize: number
  pageSizeOptions?: number[]
  showPageSizeSelector?: boolean
  showJump?: boolean
}

interface Emits {
  (e: 'update:page', page: number): void
  (e: 'update:pageSize', pageSize: number): void
}

const props = withDefaults(defineProps<Props>(), {
  pageSizeOptions: () => getConfiguredTablePageSizeOptions(),
  showPageSizeSelector: true,
  showJump: false
})

const emit = defineEmits<Emits>()

const totalPages = computed(() => Math.ceil(props.total / props.pageSize))

const fromItem = computed(() => {
  if (props.total === 0) return 0
  return (props.page - 1) * props.pageSize + 1
})

const toItem = computed(() => {
  const to = props.page * props.pageSize
  return to > props.total ? props.total : to
})

const pageSizeSelectOptions = computed(() => {
  const options = Array.from(
    new Set([
      ...getConfiguredTablePageSizeOptions(),
      normalizeTablePageSize(props.pageSize)
    ])
  ).sort((a, b) => a - b)

  return options.map((size) => ({
    value: size,
    label: String(size)
  }))
})

const jumpPage = ref('')

const visiblePages = computed(() => {
  const pages: (number | string)[] = []
  const maxVisible = 7
  const total = totalPages.value

  if (total <= maxVisible) {
    // Show all pages if total is small
    for (let i = 1; i <= total; i++) {
      pages.push(i)
    }
  } else {
    // Always show first page
    pages.push(1)

    const start = Math.max(2, props.page - 2)
    const end = Math.min(total - 1, props.page + 2)

    // Add ellipsis before if needed
    if (start > 2) {
      pages.push('...')
    }

    // Add middle pages
    for (let i = start; i <= end; i++) {
      pages.push(i)
    }

    // Add ellipsis after if needed
    if (end < total - 1) {
      pages.push('...')
    }

    // Always show last page
    pages.push(total)
  }

  return pages
})

const goToPage = (newPage: number) => {
  if (newPage >= 1 && newPage <= totalPages.value && newPage !== props.page) {
    emit('update:page', newPage)
  }
}

const handlePageSizeChange = (value: string | number | boolean | null) => {
  if (value === null || typeof value === 'boolean') return
  const newPageSize = normalizeTablePageSize(typeof value === 'string' ? parseInt(value, 10) : value)
  setPersistedPageSize(newPageSize)
  emit('update:pageSize', newPageSize)
}

const submitJump = () => {
  const value = jumpPage.value.trim()
  if (!value) return
  const pageNum = Number.parseInt(value, 10)
  if (Number.isNaN(pageNum)) return
  const nextPage = Math.min(Math.max(pageNum, 1), totalPages.value)
  jumpPage.value = ''
  goToPage(nextPage)
}
</script>

<style scoped>
.pagination-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 16px;
  border-top: 1px solid var(--border);
}

/* Prototype footer is 51px (10 + 30 + 10 + 1 border): the page-size pill shrinks to 30px here. */
.page-size-select :deep(.select-trigger) {
  height: 30px;
  min-height: 30px;
  font-size: 12px;
}

.pagination-info {
  font-size: 12.5px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.pagination-sep {
  font-size: 12.5px;
  color: var(--muted);
}

/* 28×28 page buttons · radius 8, hairline border, transparent ground. */
.pagination-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 28px;
  height: 28px;
  padding: 0 6px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--foreground);
  font-size: 12.5px;
  font-weight: 500;
  font-variant-numeric: tabular-nums;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.pagination-btn:hover:not(:disabled) {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
}

.pagination-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.pagination-btn-current {
  background: var(--accent);
  border-color: transparent;
  color: #fff;
  font-weight: 600;
}

.pagination-btn-current:hover:not(:disabled) {
  background: var(--accent);
}

.pagination-ellipsis {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 28px;
  font-size: 12.5px;
  color: var(--muted);
}

.pagination-jump {
  width: 68px;
  text-align: center;
}
</style>
