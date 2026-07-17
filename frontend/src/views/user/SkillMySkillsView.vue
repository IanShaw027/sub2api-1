<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <SkillCenterNav active="my" />

      <section class="grid gap-4 md:grid-cols-3">
        <div class="card p-5">
          <p class="text-xs uppercase tracking-[0.3em] text-ink-faint dark:text-dark-500">{{ t('skills.my.total', '总技能') }}</p>
          <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ skillsStore.mySkillsPagination.total }}</p>
        </div>
        <div class="card p-5">
          <p class="text-xs uppercase tracking-[0.3em] text-ink-faint dark:text-dark-500">{{ t('skills.my.publishedCount', '已发布') }}</p>
          <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ publishedCount }}</p>
        </div>
        <div class="card p-5">
          <p class="text-xs uppercase tracking-[0.3em] text-ink-faint dark:text-dark-500">{{ t('skills.my.paidCount', '付费技能') }}</p>
          <p class="mt-2 text-3xl font-bold text-ink dark:text-white">{{ paidCount }}</p>
        </div>
      </section>

      <section class="card p-6">
        <div class="mb-4 flex flex-wrap gap-2">
          <button
            v-for="option in quickStatusOptions"
            :key="option.value"
            type="button"
            class="rounded-full border px-3 py-1.5 text-sm transition"
            :data-status-filter="option.value"
            :class="
              skillsStore.mySkillFilters.status === option.value
                ? 'border-ink bg-ink text-white dark:border-line dark:bg-card dark:text-ink'
                : 'border-line text-ink-body hover:border-ink/40 hover:text-ink dark:border-dark-700 dark:text-dark-300 dark:hover:border-dark-500 dark:hover:text-white'
            "
            @click="void updateStatusFilter(option.value)"
          >
            {{ option.label }}
          </button>
        </div>

        <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
          <Input
            v-model="skillsStore.mySkillFilters.search"
            :label="t('common.search', '搜索')"
            :placeholder="t('skills.my.searchPlaceholder', '搜索名称、标签、描述')"
          />

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.editor.type', '类型') }}</label>
            <Select
              :model-value="skillsStore.mySkillFilters.type"
              :options="typeOptions"
              @update:model-value="(value) => void updateTypeFilter(normalizeType(value))"
            />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('common.status', '状态') }}</label>
            <Select
              :model-value="skillsStore.mySkillFilters.status"
              :options="statusOptions"
              @update:model-value="(value) => void updateStatusFilter(normalizeStatus(value))"
            />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.my.visibility', '可见性') }}</label>
            <Select
              :model-value="skillsStore.mySkillFilters.visibility"
              :options="visibilityOptions"
              @update:model-value="(value) => void updateVisibilityFilter(normalizeVisibility(value))"
            />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('skills.market.sort', '排序') }}</label>
            <Select
              :model-value="skillsStore.mySkillFilters.sort"
              :options="sortOptions"
              @update:model-value="(value) => void updateSortFilter(normalizeSort(value))"
            />
          </div>
        </div>

        <div class="mt-4 flex flex-wrap justify-end gap-3">
          <button class="btn btn-secondary" type="button" @click="void resetFilters()">{{ t('common.reset', '重置') }}</button>
          <button class="btn btn-primary" type="button" :disabled="skillsStore.loadingMySkills" @click="void applySearchFilters()">
            <Icon name="refresh" size="sm" class="mr-2" :class="skillsStore.loadingMySkills ? 'animate-spin' : ''" />
            {{ t('common.search', '搜索') }}
          </button>
        </div>
      </section>

      <section class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <SkillCard
          v-for="skill in skillsStore.mySkillsPagination.items"
          :key="skill.id"
          :skill="skill"
          show-owner-actions
        />

        <div
          v-if="!skillsStore.loadingMySkills && skillsStore.mySkillsPagination.items.length === 0"
          class="md:col-span-2 xl:col-span-3"
        >
          <div class="card p-12">
            <EmptyState
              :title="t('skills.my.emptyTitle', '还没有我的技能')"
              :description="t('skills.my.emptyDescription', '先创建一个技能，再来管理它的版本、运行记录和收益。')"
            />
          </div>
        </div>
      </section>

      <Pagination
        v-if="skillsStore.mySkillsPagination.total > skillsStore.mySkillsPagination.page_size"
        :page="skillsStore.mySkillsPagination.page"
        :total="skillsStore.mySkillsPagination.total"
        :page-size="skillsStore.mySkillsPagination.page_size"
        @update:page="(page) => void updatePage(page)"
        @update:pageSize="(pageSize) => void updatePageSize(pageSize)"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import SkillCard from '@/components/skills/SkillCard.vue'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SkillSortKey, SkillStatus, SkillType, SkillVisibility } from '@/types/skills'

const { t } = useI18n()
const appStore = useAppStore()
const skillsStore = useSkillsCenterStore()
const route = useRoute()
const router = useRouter()

let suppressRouteStatusReload = false

function skillTypeLabel(type: SkillType): string {
  const labels: Record<SkillType, string> = {
    prompt_chat: t('skills.labels.typePromptChat', '对话提示词'),
    prompt_image: t('skills.labels.typePromptImage', '图像提示词'),
    script: t('skills.labels.typeScript', '脚本')
  }
  return labels[type]
}

function skillStatusLabel(status: SkillStatus): string {
  const labels: Record<SkillStatus, string> = {
    draft: t('skills.labels.statusDraft', '草稿'),
    published: t('skills.labels.statusPublished', '已发布'),
    archived: t('skills.labels.statusArchived', '已归档'),
    hidden: t('skills.labels.statusHidden', '已隐藏')
  }
  return labels[status]
}

function skillVisibilityLabel(visibility: SkillVisibility): string {
  const labels: Record<SkillVisibility, string> = {
    public: t('skills.labels.visibilityPublic', '公开'),
    private: t('skills.labels.visibilityPrivate', '私有')
  }
  return labels[visibility]
}

const typeOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'prompt_chat', label: skillTypeLabel('prompt_chat') },
  { value: 'prompt_image', label: skillTypeLabel('prompt_image') }
])

const statusOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'draft', label: skillStatusLabel('draft') },
  { value: 'published', label: skillStatusLabel('published') },
  { value: 'archived', label: skillStatusLabel('archived') },
  { value: 'hidden', label: skillStatusLabel('hidden') }
])

const visibilityOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'public', label: skillVisibilityLabel('public') },
  { value: 'private', label: skillVisibilityLabel('private') }
])

const sortOptions = computed(() => [
  { value: 'latest', label: t('skills.market.sortLatest', '最新') },
  { value: 'popular', label: t('skills.market.sortPopular', '热门') },
  { value: 'runs', label: t('skills.market.sortRuns', '运行量') },
  { value: 'revenue', label: t('skills.market.sortRevenue', '收益') }
])
const quickStatusOptions = computed(() => [
  { value: 'all' as const, label: t('common.all', '全部') },
  { value: 'draft' as const, label: t('skills.my.statusDraft', '草稿') },
  { value: 'published' as const, label: t('skills.my.statusPublished', '已发布') },
  { value: 'archived' as const, label: t('skills.my.statusArchived', '已归档') },
  { value: 'hidden' as const, label: t('skills.my.statusHidden', '已隐藏') }
])

const publishedCount = computed(() =>
  skillsStore.mySkillsPagination.items.filter((item) => item.status === 'published').length
)
const paidCount = computed(() =>
  skillsStore.mySkillsPagination.items.filter((item) => item.pricing.mode === 'paid').length
)

function normalizeType(value: string | number | boolean | null): SkillType | 'all' {
  return value === 'prompt_chat' || value === 'prompt_image' ? value : 'all'
}

function normalizeStatus(value: string | number | boolean | null): SkillStatus | 'all' {
  return value === 'draft' || value === 'published' || value === 'archived' || value === 'hidden' ? value : 'all'
}

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

function normalizeVisibility(value: string | number | boolean | null): SkillVisibility | 'all' {
  return value === 'public' || value === 'private' ? value : 'all'
}

function normalizeSort(value: string | number | boolean | null): SkillSortKey {
  return value === 'popular' || value === 'runs' || value === 'revenue' ? value : 'latest'
}

async function reloadMySkills(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<void> {
  try {
    await skillsStore.loadMySkills(page, pageSize)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function applySearchFilters(): Promise<void> {
  const nextPageSize = skillsStore.mySkillsPagination.page_size
  await replaceQuery({
    type: skillsStore.mySkillFilters.type,
    status: skillsStore.mySkillFilters.status,
    visibility: skillsStore.mySkillFilters.visibility,
    search: skillsStore.mySkillFilters.search,
    sort: skillsStore.mySkillFilters.sort,
    page: 1,
    pageSize: nextPageSize,
  })
  await reloadMySkills(1, nextPageSize)
}

function currentRoutePage(): number {
  return extractPositiveQueryNumber(route.query.page, 1)
}

function currentRoutePageSize(): number {
  return extractPositiveQueryNumber(route.query.page_size, skillsStore.mySkillsPagination.page_size)
}

async function replaceQuery(filters: {
  type: SkillType | 'all'
  status: SkillStatus | 'all'
  visibility: SkillVisibility | 'all'
  search: string
  sort: SkillSortKey
  page: number
  pageSize: number
}): Promise<boolean> {
  const currentType = extractQueryString(route.query.type)
  const currentStatus = extractQueryString(route.query.status)
  const currentVisibility = extractQueryString(route.query.visibility)
  const currentSearch = extractQueryString(route.query.search) ?? ''
  const currentSort = extractQueryString(route.query.sort)
  const currentPage = extractQueryString(route.query.page)
  const currentPageSize = extractQueryString(route.query.page_size)
  const nextType = filters.type === 'all' ? null : filters.type
  const nextStatus = filters.status === 'all' ? null : filters.status
  const nextVisibility = filters.visibility === 'all' ? null : filters.visibility
  const nextSearch = filters.search.trim()
  const nextSort = filters.sort === 'latest' ? null : filters.sort
  const nextPage = filters.page > 1 ? String(filters.page) : null
  const nextPageSize = filters.pageSize !== 18 ? String(filters.pageSize) : null
  if (
    currentType === nextType &&
    currentStatus === nextStatus &&
    currentVisibility === nextVisibility &&
    currentSearch === nextSearch &&
    currentSort === nextSort &&
    currentPage === nextPage &&
    currentPageSize === nextPageSize
  ) return false

  const nextQuery = { ...route.query }
  if (nextType) {
    nextQuery.type = nextType
  } else {
    delete nextQuery.type
  }
  if (nextStatus) {
    nextQuery.status = nextStatus
  } else {
    delete nextQuery.status
  }
  if (nextVisibility) {
    nextQuery.visibility = nextVisibility
  } else {
    delete nextQuery.visibility
  }
  if (nextSearch) {
    nextQuery.search = nextSearch
  } else {
    delete nextQuery.search
  }
  if (nextSort) {
    nextQuery.sort = nextSort
  } else {
    delete nextQuery.sort
  }
  if (nextPage) {
    nextQuery.page = nextPage
  } else {
    delete nextQuery.page
  }
  if (nextPageSize) {
    nextQuery.page_size = nextPageSize
  } else {
    delete nextQuery.page_size
  }

  suppressRouteStatusReload = true
  await router.replace({ query: nextQuery })
  return true
}

async function replaceStatusQuery(status: SkillStatus | 'all'): Promise<boolean> {
  return replaceQuery({
    type: skillsStore.mySkillFilters.type,
    status,
    visibility: skillsStore.mySkillFilters.visibility,
    search: skillsStore.mySkillFilters.search,
    sort: skillsStore.mySkillFilters.sort,
    page: 1,
    pageSize: skillsStore.mySkillsPagination.page_size,
  })
}

async function updateStatusFilter(status: SkillStatus | 'all'): Promise<void> {
  skillsStore.mySkillFilters.status = status
  const nextPageSize = skillsStore.mySkillsPagination.page_size
  await replaceStatusQuery(status)
  await reloadMySkills(1, nextPageSize)
}

async function updateTypeFilter(type: SkillType | 'all'): Promise<void> {
  skillsStore.mySkillFilters.type = type
  const nextPageSize = skillsStore.mySkillsPagination.page_size
  await replaceQuery({
    type,
    status: skillsStore.mySkillFilters.status,
    visibility: skillsStore.mySkillFilters.visibility,
    search: skillsStore.mySkillFilters.search,
    sort: skillsStore.mySkillFilters.sort,
    page: 1,
    pageSize: nextPageSize,
  })
  await reloadMySkills(1, nextPageSize)
}

async function updateVisibilityFilter(visibility: SkillVisibility | 'all'): Promise<void> {
  skillsStore.mySkillFilters.visibility = visibility
  const nextPageSize = skillsStore.mySkillsPagination.page_size
  await replaceQuery({
    type: skillsStore.mySkillFilters.type,
    status: skillsStore.mySkillFilters.status,
    visibility,
    search: skillsStore.mySkillFilters.search,
    sort: skillsStore.mySkillFilters.sort,
    page: 1,
    pageSize: nextPageSize,
  })
  await reloadMySkills(1, nextPageSize)
}

async function updateSortFilter(sort: SkillSortKey): Promise<void> {
  skillsStore.mySkillFilters.sort = sort
  const nextPageSize = skillsStore.mySkillsPagination.page_size
  await replaceQuery({
    type: skillsStore.mySkillFilters.type,
    status: skillsStore.mySkillFilters.status,
    visibility: skillsStore.mySkillFilters.visibility,
    search: skillsStore.mySkillFilters.search,
    sort,
    page: 1,
    pageSize: nextPageSize,
  })
  await reloadMySkills(1, nextPageSize)
}

async function resetFilters(): Promise<void> {
  skillsStore.resetMySkillFilters()
  await replaceQuery({
    type: 'all',
    status: 'all',
    visibility: 'all',
    search: '',
    sort: 'latest',
    page: 1,
    pageSize: 18,
  })
  await reloadMySkills(1, 18)
}

async function updatePage(page: number): Promise<void> {
  const nextPageSize = skillsStore.mySkillsPagination.page_size
  await replaceQuery({
    type: skillsStore.mySkillFilters.type,
    status: skillsStore.mySkillFilters.status,
    visibility: skillsStore.mySkillFilters.visibility,
    search: skillsStore.mySkillFilters.search,
    sort: skillsStore.mySkillFilters.sort,
    page,
    pageSize: nextPageSize,
  })
  await reloadMySkills(page, nextPageSize)
}

async function updatePageSize(pageSize: number): Promise<void> {
  await replaceQuery({
    type: skillsStore.mySkillFilters.type,
    status: skillsStore.mySkillFilters.status,
    visibility: skillsStore.mySkillFilters.visibility,
    search: skillsStore.mySkillFilters.search,
    sort: skillsStore.mySkillFilters.sort,
    page: 1,
    pageSize,
  })
  await reloadMySkills(1, pageSize)
}

watch(
  () => [
    extractQueryString(route.query.type),
    extractQueryString(route.query.status),
    extractQueryString(route.query.visibility),
    extractQueryString(route.query.search),
    extractQueryString(route.query.sort),
    extractQueryString(route.query.page),
    extractQueryString(route.query.page_size),
  ],
  async ([typeValue, statusValue, visibilityValue, searchValue, sortValue]) => {
    const type = normalizeType(typeValue)
    const status = normalizeStatus(statusValue)
    const visibility = normalizeVisibility(visibilityValue)
    const sort = normalizeSort(sortValue)
    if (skillsStore.mySkillFilters.type !== type) {
      skillsStore.mySkillFilters.type = type
    }
    if (skillsStore.mySkillFilters.status !== status) {
      skillsStore.mySkillFilters.status = status
    }
    if (skillsStore.mySkillFilters.visibility !== visibility) {
      skillsStore.mySkillFilters.visibility = visibility
    }
    const search = searchValue ?? ''
    if (skillsStore.mySkillFilters.search !== search) {
      skillsStore.mySkillFilters.search = search
    }
    if (skillsStore.mySkillFilters.sort !== sort) {
      skillsStore.mySkillFilters.sort = sort
    }
    if (suppressRouteStatusReload) {
      suppressRouteStatusReload = false
      return
    }
    await reloadMySkills()
  },
  { immediate: true }
)
</script>
