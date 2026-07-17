<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <div class="card p-6">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <p class="text-xs uppercase tracking-[0.35em] text-ink-soft dark:text-dark-400">{{ t('ai.center.label', 'AI 创作中心') }}</p>
            <h1 class="mt-2 text-2xl font-bold text-ink dark:text-white">{{ t('ai.gallery.title', '画廊') }}</h1>
            <p class="mt-1 text-sm text-ink-body dark:text-dark-400">{{ t('ai.gallery.subtitle', '支持筛选和瀑布流浏览。') }}</p>
          </div>
          <div class="flex gap-3">
            <button class="btn btn-secondary" :disabled="loading" @click="reloadGallery">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>

        <div class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Input v-model="filters.search" :label="t('common.search', '搜索')" :placeholder="t('ai.gallery.searchPlaceholder', '搜标题、提示词、标签')" />

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.visibility', '可见性') }}</label>
            <Select :model-value="filters.visibility" :options="visibilityOptions" @update:model-value="updateVisibility" />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.status', '状态') }}</label>
            <Select :model-value="filters.status" :options="statusOptions" @update:model-value="updateStatus" />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.line', '线路') }}</label>
            <Select :model-value="filters.line_id" :options="lineOptions" @update:model-value="updateLine" />
          </div>
        </div>

        <p class="mt-3 text-xs text-ink-soft dark:text-dark-400">
          精选筛选已收口到管理员治理页，用户侧仅保留后端真实支持的公共筛选项。
        </p>

        <div class="mt-4 flex justify-end gap-3">
          <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
          <button class="btn btn-primary" :disabled="loading" @click="applySearchFilters">{{ t('common.search', '搜索') }}</button>
        </div>
      </div>

      <div v-if="gallery.length > 0" class="columns-1 gap-5 md:columns-2 xl:columns-4">
        <article
          v-for="artwork in gallery"
          :key="artwork.id"
          class="mb-5 break-inside-avoid overflow-hidden rounded-card border border-line bg-card shadow-xs transition-transform hover:-translate-y-0.5 dark:border-dark-700 dark:bg-dark-900"
        >
          <img :src="artwork.image_url" :alt="artwork.title" class="w-full object-cover" />
          <div class="space-y-3 p-4">
            <div class="flex items-start justify-between gap-3">
              <div>
                <h2 class="line-clamp-2 text-sm font-semibold text-ink dark:text-white">{{ artwork.title }}</h2>
                <p class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ resolveLineLabel(artwork.line_id, artwork.line_name) }}</p>
              </div>
              <span class="rounded-full px-2.5 py-1 text-[11px] font-medium"
                :class="artwork.visibility === 'private'
                  ? 'bg-page text-ink-body dark:bg-dark-800 dark:text-dark-200'
                  : 'bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-200'"
              >
                {{ artwork.visibility === 'private' ? t('ai.prompt.private', '私有') : t('ai.prompt.public', '公开') }}
              </span>
            </div>
            <p class="line-clamp-4 whitespace-pre-wrap text-sm text-ink-body dark:text-dark-400">{{ artwork.prompt }}</p>
            <div class="flex flex-wrap gap-2">
              <span
                v-for="tag in artwork.tags"
                :key="tag"
                class="rounded-full bg-line px-2 py-1 text-[11px] text-ink-body dark:bg-dark-800 dark:text-dark-300"
              >
                #{{ tag }}
              </span>
            </div>
            <div class="flex items-center justify-between text-xs text-ink-soft dark:text-dark-400">
              <span>{{ artwork.style || '-' }}</span>
              <span>{{ formatTime(artwork.created_at) }}</span>
            </div>
          </div>
        </article>
      </div>

      <div v-else class="card p-12">
        <EmptyState
          :title="t('ai.gallery.emptyTitle', '暂无作品')"
          :description="t('ai.gallery.emptyDesc', '先去 AI 生图 创建一些作品。')"
          :action-text="t('ai.image.title', 'AI 生图')"
          action-to="/ai/image"
        />
      </div>

      <Pagination
        v-if="pagination.total > pagination.page_size"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { listAIArtworks } from '@/api'
import { useAiStudioStore, useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AiArtwork, BasePaginationResponse } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const aiStore = useAiStudioStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const gallery = ref<AiArtwork[]>([])
let suppressRouteGalleryReload = false
const pagination = reactive<BasePaginationResponse<AiArtwork>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 24,
  pages: 1
})

const filters = reactive({
  search: '',
  visibility: 'all' as 'all' | 'public' | 'private',
  status: 'all' as 'all' | 'pending' | 'ready' | 'hidden' | 'deleted',
  line_id: 'all' as number | 'all'
})

const lineOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  ...aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: `${line.label} · ${line.key_count}`
  }))
])

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

function extractQueryString(value: unknown): string | null {
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return null
}

function extractPositiveQueryNumber(value: unknown, fallback: number): number {
  const raw = extractQueryString(value)
  if (!raw) return fallback
  const parsed = Number.parseInt(raw, 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
}

function formatTime(value: string): string {
  return new Date(value).toLocaleDateString()
}

function resolveLineLabel(lineId?: number | null, fallback?: string | null): string {
  if (fallback) return fallback
  if (typeof lineId === 'number') {
    return aiStore.availableLines.find((line) => line.group_id === lineId)?.label ?? t('ai.line.unassigned', '未分配线路')
  }
  return t('ai.line.unassigned', '未分配线路')
}

function updateVisibility(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.visibility = next === 'public' || next === 'private' ? next : 'all'
}

function updateStatus(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.status = next === 'pending' || next === 'ready' || next === 'hidden' || next === 'deleted' ? next : 'all'
}

function updateLine(value: string | number | boolean | null) {
  filters.line_id = typeof value === 'number' ? value : 'all'
}

function syncRouteFilters(): void {
  filters.search = extractQueryString(route.query.search) ?? ''
  updateVisibility(extractQueryString(route.query.visibility))
  updateStatus(extractQueryString(route.query.status))
  const lineId = extractPositiveQueryNumber(route.query.line_id, 0)
  filters.line_id = lineId > 0 ? lineId : 'all'
}

function currentRoutePage(): number {
  return extractPositiveQueryNumber(route.query.page, 1)
}

function currentRoutePageSize(): number {
  return extractPositiveQueryNumber(route.query.page_size, pagination.page_size)
}

async function replaceGalleryQuery(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<boolean> {
  const nextQuery = { ...route.query }
  const nextSearch = filters.search.trim() || null
  const nextVisibility = filters.visibility !== 'all' ? filters.visibility : null
  const nextStatus = filters.status !== 'all' ? filters.status : null
  const nextLineId = typeof filters.line_id === 'number' && filters.line_id > 0 ? String(filters.line_id) : null
  const nextPage = page > 1 ? String(page) : null
  const nextPageSize = pageSize !== 24 ? String(pageSize) : null
  const currentSearch = extractQueryString(route.query.search)
  const currentVisibility = extractQueryString(route.query.visibility)
  const currentStatus = extractQueryString(route.query.status)
  const currentLineId = extractQueryString(route.query.line_id)
  const currentPage = extractQueryString(route.query.page)
  const currentPageSize = extractQueryString(route.query.page_size)

  if (nextSearch) nextQuery.search = nextSearch
  else delete nextQuery.search

  if (nextVisibility) nextQuery.visibility = nextVisibility
  else delete nextQuery.visibility

  if (nextStatus) nextQuery.status = nextStatus
  else delete nextQuery.status

  if (nextLineId) nextQuery.line_id = nextLineId
  else delete nextQuery.line_id

  if (nextPage) nextQuery.page = nextPage
  else delete nextQuery.page

  if (nextPageSize) nextQuery.page_size = nextPageSize
  else delete nextQuery.page_size

  if (
    currentSearch === nextSearch &&
    currentVisibility === nextVisibility &&
    currentStatus === nextStatus &&
    currentLineId === nextLineId &&
    currentPage === nextPage &&
    currentPageSize === nextPageSize
  ) {
    return false
  }

  suppressRouteGalleryReload = true
  await router.replace({ query: nextQuery })
  return true
}

async function loadGallery(page = currentRoutePage(), pageSize = currentRoutePageSize()) {
  loading.value = true
  try {
    await aiStore.loadRuntimeLines()
    const response = await listAIArtworks(page, pageSize, {
      search: filters.search.trim() || undefined,
      visibility: filters.visibility,
      status: filters.status,
      line_id: filters.line_id
    })
    gallery.value = response.items
    Object.assign(pagination, response)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

async function reloadGallery(): Promise<void> {
  await loadGallery()
}

async function applySearchFilters(): Promise<void> {
  await replaceGalleryQuery(1, pagination.page_size)
  await loadGallery(1, pagination.page_size)
}

async function resetFilters() {
  filters.search = ''
  filters.visibility = 'all'
  filters.status = 'all'
  filters.line_id = 'all'
  await replaceGalleryQuery(1, 24)
  await loadGallery(1, 24)
}

async function handlePageChange(page: number) {
  await replaceGalleryQuery(page, pagination.page_size)
  await loadGallery(page, pagination.page_size)
}

async function handlePageSizeChange(size: number) {
  await replaceGalleryQuery(1, size)
  await loadGallery(1, size)
}

watch(
  () => [route.query.search, route.query.visibility, route.query.status, route.query.line_id, route.query.page, route.query.page_size],
  async () => {
    syncRouteFilters()
    if (suppressRouteGalleryReload) {
      suppressRouteGalleryReload = false
      return
    }
    await loadGallery()
  },
  { immediate: true }
)
</script>
