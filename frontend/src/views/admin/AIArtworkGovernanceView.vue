<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
          <Input v-model="filters.search" :label="t('common.search', '搜索')" :placeholder="t('ai.gallery.searchPlaceholder', '搜标题、提示词、作者')" />

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.visibility', '可见性') }}</label>
            <Select :model-value="filters.visibility" :options="visibilityOptions" @update:model-value="updateVisibilityFilter" />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.status', '状态') }}</label>
            <Select :model-value="filters.status" :options="statusOptions" @update:model-value="updateStatusFilter" />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.line', '线路') }}</label>
            <Select :model-value="filters.line_id" :options="lineOptions" @update:model-value="updateLineFilter" />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.gallery.featured', '精选') }}</label>
            <Select :model-value="featuredFilter" :options="featuredOptions" @update:model-value="updateFeaturedFilter" />
          </div>
        </div>
      </template>

      <template #actions>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
          <button class="btn btn-secondary" :disabled="loading" @click="loadArtworks">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="artworks" :loading="loading">
          <template #cell-title="{ row }">
            <div class="flex min-w-[260px] items-center gap-3">
              <img :src="row.thumbnail_url || row.image_url" :alt="row.title" class="h-14 w-14 rounded-xl object-cover" />
              <div class="min-w-0">
                <div class="font-medium text-ink dark:text-white">{{ row.title }}</div>
                <div class="mt-1 line-clamp-2 text-xs text-ink-soft dark:text-dark-400">{{ row.prompt }}</div>
              </div>
            </div>
          </template>

          <template #cell-line_name="{ row }">
            <span class="text-sm text-ink-body dark:text-dark-300">{{ resolveLineLabel(row.line_id, row.line_name) }}</span>
          </template>

          <template #cell-visibility="{ value }">
            <span class="rounded-full px-2.5 py-1 text-xs font-medium"
              :class="value === 'private'
                ? 'bg-page text-ink-soft dark:bg-dark-800 dark:text-dark-200'
                : 'bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-200'"
            >
              {{ value === 'private' ? t('ai.prompt.private', '私有') : t('ai.prompt.public', '公开') }}
            </span>
          </template>

          <template #cell-status="{ value }">
            <span class="rounded-full bg-warning-soft px-2.5 py-1 text-xs font-medium text-warning dark:bg-amber-900/30 dark:text-amber-100">
              {{ statusLabel(value) }}
            </span>
          </template>

          <template #cell-owner_name="{ value }">
            <span class="text-sm text-ink-body dark:text-dark-300">{{ value || '-' }}</span>
          </template>

          <template #cell-featured="{ value }">
            <span :class="value ? 'text-success dark:text-emerald-400' : 'text-ink-faint dark:text-dark-500'">
              {{ value ? t('common.yes', '是') : t('common.no', '否') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-ink-soft dark:text-dark-400">{{ formatTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button class="btn btn-secondary btn-sm" @click="toggleFeatured(row)">
                {{ row.featured ? t('ai.gallery.unfeature', '取消精选') : t('ai.gallery.feature', '设为精选') }}
              </button>
              <button class="btn btn-secondary btn-sm" @click="toggleVisibility(row)">
                {{ row.visibility === 'public' ? t('ai.prompt.makePrivate', '转私有') : t('ai.prompt.makePublic', '转公开') }}
              </button>
              <button class="btn btn-danger btn-sm" @click="requestDelete(row)">{{ t('common.delete', '删除') }}</button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('ai.artworkGovernance.emptyTitle', '暂无作品记录')"
              :description="t('ai.artworkGovernance.emptyDesc', '用户生成的作品会在这里等待治理。')"
            />
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > pagination.page_size"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <ConfirmDialog
      :show="artworkToDelete !== null"
      :title="t('ai.artwork.delete', '删除作品')"
      :message="t('ai.artwork.deleteConfirm', '删除后不可恢复，确认继续？')"
      danger
      @cancel="artworkToDelete = null"
      @confirm="confirmDeleteArtwork"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import adminAIAPI from '@/api/admin/ai'
import { useAiStudioStore, useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Column } from '@/components/common/types'
import type { AiArtwork, BasePaginationResponse } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const aiStore = useAiStudioStore()

const loading = ref(false)
const artworks = ref<AiArtwork[]>([])
const artworkToDelete = ref<AiArtwork | null>(null)
const featuredFilter = ref<'all' | 'featured' | 'normal'>('all')

const pagination = reactive<BasePaginationResponse<AiArtwork>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1
})

const filters = reactive({
  search: '',
  visibility: 'all' as 'all' | 'public' | 'private',
  status: 'all' as 'all' | 'pending' | 'ready' | 'hidden' | 'deleted',
  line_id: 'all' as number | 'all'
})

const visibilityOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'public', label: t('ai.prompt.public', '公开') },
  { value: 'private', label: t('ai.prompt.private', '私有') }
]

const statusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'ready', label: '已就绪' },
  { value: 'pending', label: t('ai.artwork.pending', '处理中') },
  { value: 'hidden', label: t('ai.prompt.hidden', '隐藏') },
  { value: 'deleted', label: t('ai.artwork.deleted', '已删除') }
]

const featuredOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'featured', label: t('ai.gallery.featuredOnly', '仅精选') },
  { value: 'normal', label: t('ai.gallery.normalOnly', '非精选') }
]

const lineOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  ...aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: line.label
  }))
])

const columns = computed<Column[]>(() => [
  { key: 'title', label: t('ai.artwork.title', '作品') },
  { key: 'line_name', label: t('ai.prompt.line', '线路') },
  { key: 'visibility', label: t('ai.prompt.visibility', '可见性') },
  { key: 'status', label: t('ai.prompt.status', '状态') },
  { key: 'owner_name', label: t('ai.prompt.owner', '作者') },
  { key: 'featured', label: t('ai.gallery.featured', '精选') },
  { key: 'created_at', label: t('common.createdAt', '创建时间') },
  { key: 'actions', label: t('common.actions', '操作') }
])

function resolveLineLabel(lineId?: number | null, fallback?: string | null): string {
  if (fallback) return fallback
  if (typeof lineId === 'number') {
    return aiStore.availableLines.find((line) => line.group_id === lineId)?.label ?? t('ai.line.unassigned', '未分配线路')
  }
  return t('ai.line.unassigned', '未分配线路')
}

function statusLabel(status: AiArtwork['status']): string {
  return {
    pending: t('ai.artwork.pending', '处理中'),
    succeeded: '已就绪',
    failed: t('ai.artwork.failed', '失败'),
    hidden: t('ai.prompt.hidden', '隐藏'),
    deleted: t('ai.artwork.deleted', '已删除')
  }[status]
}

function formatTime(value: string): string {
  return new Date(value).toLocaleString()
}

function updateVisibilityFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.visibility = next === 'public' || next === 'private' ? next : 'all'
}

function updateStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.status = next === 'pending' || next === 'ready' || next === 'hidden' || next === 'deleted' ? next : 'all'
}

function updateLineFilter(value: string | number | boolean | null) {
  filters.line_id = typeof value === 'number' ? value : 'all'
}

function updateFeaturedFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  featuredFilter.value = next === 'featured' || next === 'normal' ? next : 'all'
}

async function loadArtworks() {
  loading.value = true
  try {
    await aiStore.loadRuntimeLines()
    const response = await adminAIAPI.listArtworks(pagination.page, pagination.page_size, {
      search: filters.search.trim() || undefined,
      visibility: filters.visibility,
      status: filters.status,
      line_id: filters.line_id,
      featured: featuredFilter.value === 'all' ? undefined : featuredFilter.value === 'featured'
    })
    artworks.value = response.items
    Object.assign(pagination, response)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.search = ''
  filters.visibility = 'all'
  filters.status = 'all'
  filters.line_id = 'all'
  featuredFilter.value = 'all'
  pagination.page = 1
  void loadArtworks()
}

async function toggleFeatured(row: AiArtwork) {
  try {
    await adminAIAPI.updateArtwork(row.id, { featured: !row.featured })
    await loadArtworks()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function toggleVisibility(row: AiArtwork) {
  try {
    await adminAIAPI.updateArtwork(row.id, {
      visibility: row.visibility === 'public' ? 'private' : 'public'
    })
    await loadArtworks()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

function requestDelete(row: AiArtwork) {
  artworkToDelete.value = row
}

async function confirmDeleteArtwork() {
  if (!artworkToDelete.value) return
  try {
    await adminAIAPI.deleteArtwork(artworkToDelete.value.id)
    artworkToDelete.value = null
    appStore.showSuccess(t('ai.artwork.deleted', '作品已删除'))
    await loadArtworks()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadArtworks()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadArtworks()
}

onMounted(async () => {
  await loadArtworks()
})
</script>
