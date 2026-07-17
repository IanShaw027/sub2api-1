<template>
  <AppLayout>
    <div v-if="showRunPage" class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <SkillCenterNav
        active="runs"
        :skill-id="skillId"
        :can-edit-skill="Boolean(skill?.editable)"
        :can-view-runs="Boolean(skill?.owned)"
        :can-view-revenue="Boolean(skill?.owned)"
      />

      <TablePageLayout>
        <template #filters>
          <div class="card p-6">
            <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <Input
                v-model="skillsStore.runFilters.search"
                :label="t('common.search', '搜索')"
                :placeholder="t('skills.runs.searchPlaceholder', '搜索输入摘要、输出摘要或错误信息')"
              />

              <div>
                <label class="input-label mb-1.5 block">{{ t('common.status', '状态') }}</label>
                <Select
                  :model-value="skillsStore.runFilters.status"
                  :options="statusOptions"
                  @update:model-value="(value) => (skillsStore.runFilters.status = normalizeStatus(value))"
                />
              </div>

              <div>
                <label class="input-label mb-1.5 block">{{ t('skills.versions.version', '版本号') }}</label>
                <Select
                  :model-value="skillsStore.runFilters.version_id"
                  :options="skillsStore.versionOptions"
                  @update:model-value="(value) => (skillsStore.runFilters.version_id = typeof value === 'number' ? value : 'all')"
                />
              </div>

              <div class="flex items-end justify-end gap-3">
                <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
                <button class="btn btn-primary" :disabled="skillsStore.loadingRuns" @click="applySearchFilters">
                  <Icon name="refresh" size="sm" class="mr-2" :class="skillsStore.loadingRuns ? 'animate-spin' : ''" />
                  {{ t('common.search', '搜索') }}
                </button>
              </div>
            </div>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="skillsStore.runsPagination.items" :loading="skillsStore.loadingRuns">
            <template #cell-status="{ row }">
              <span class="rounded-full px-2.5 py-1 text-[11px] font-medium" :class="skillRunBadgeClass(row.status)">
                {{ skillRunStatusLabel(row.status, t) }}
              </span>
            </template>

            <template #cell-trigger="{ row }">
              <span class="text-sm font-medium text-ink dark:text-white">{{ skillRunTriggerLabel(row.trigger, t) }}</span>
            </template>

            <template #cell-version="{ row }">
              <span class="font-medium text-ink dark:text-white">{{ row.version || '-' }}</span>
            </template>

            <template #cell-duration_ms="{ row }">
              <span class="text-sm text-ink-body dark:text-dark-300">{{ formatDuration(row.duration_ms) }}</span>
            </template>

            <template #cell-cost="{ row }">
              <span class="text-sm font-medium text-ink dark:text-white">
                {{ row.cost !== null ? formatCurrency(row.cost, row.currency) : '-' }}
              </span>
            </template>

            <template #cell-input_preview="{ row }">
              <div class="max-w-[24rem] whitespace-normal break-words text-sm text-ink-body dark:text-dark-300">
                {{ row.input_preview || '-' }}
              </div>
            </template>

            <template #cell-output_preview="{ row }">
              <div class="max-w-[24rem] whitespace-normal break-words text-sm text-ink-body dark:text-dark-300">
                {{ row.output_preview || row.error_message || '-' }}
              </div>
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="skillsStore.runsPagination.total > skillsStore.runsPagination.page_size"
            :page="skillsStore.runsPagination.page"
            :total="skillsStore.runsPagination.total"
            :page-size="skillsStore.runsPagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </template>
      </TablePageLayout>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, watch, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
import { skillPaths } from '@/components/skills/paths'
import { formatCurrency, skillRunBadgeClass, skillRunStatusLabel, skillRunTriggerLabel } from '@/components/skills/presentation'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { SkillRunStatus } from '@/types/skills'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const skillsStore = useSkillsCenterStore()
const router = useRouter()

let suppressRouteRunsReload = false

const skillId = computed(() => {
  const value = Number(route.params.id)
  return Number.isFinite(value) && value > 0 ? value : 0
})
const skill = computed(() => skillsStore.detail)
const loadedSkillId = ref<number | null>(null)
const showRunPage = computed(() => loadedSkillId.value === skillId.value)

const statusOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'queued', label: skillRunStatusLabel('queued', t) },
  { value: 'running', label: skillRunStatusLabel('running', t) },
  { value: 'succeeded', label: skillRunStatusLabel('succeeded', t) },
  { value: 'failed', label: skillRunStatusLabel('failed', t) },
  { value: 'cancelled', label: skillRunStatusLabel('cancelled', t) }
])

const columns = computed<Column[]>(() => [
  { key: 'status', label: t('common.status', '状态'), class: 'w-36' },
  { key: 'trigger', label: t('skills.runs.trigger', '触发方式'), class: 'w-28' },
  { key: 'version', label: t('skills.versions.version', '版本号'), class: 'w-28' },
  { key: 'duration_ms', label: t('skills.runs.duration', '耗时'), class: 'w-28' },
  { key: 'cost', label: t('skills.runs.cost', '费用'), class: 'w-32' },
  { key: 'input_preview', label: t('skills.runs.inputPreview', '输入摘要'), class: 'min-w-[18rem] whitespace-normal' },
  { key: 'output_preview', label: t('skills.runs.outputPreview', '输出摘要'), class: 'min-w-[18rem] whitespace-normal' }
])

function normalizeStatus(value: string | number | boolean | null): SkillRunStatus | 'all' {
  return value === 'queued' || value === 'running' || value === 'succeeded' || value === 'failed' || value === 'cancelled'
    ? value
    : 'all'
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

function formatDuration(value: number | null): string {
  if (value === null || value <= 0) return '-'
  if (value < 1000) return `${value}ms`
  return `${(value / 1000).toFixed(2)}s`
}

function syncRouteFilters(): void {
  skillsStore.runFilters.search = extractQueryString(route.query.search) ?? ''
  skillsStore.runFilters.status = normalizeStatus(extractQueryString(route.query.status))
  const versionId = extractPositiveQueryNumber(route.query.version_id, 0)
  skillsStore.runFilters.version_id = versionId > 0 ? versionId : 'all'
}

function currentRoutePage(): number {
  return extractPositiveQueryNumber(route.query.page, 1)
}

function currentRoutePageSize(): number {
  return extractPositiveQueryNumber(route.query.page_size, skillsStore.runsPagination.page_size)
}

async function replaceRunsQuery(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<boolean> {
  const nextQuery = { ...route.query }
  const nextSearch = (skillsStore.runFilters.search ?? '').trim() || null
  const nextStatus = skillsStore.runFilters.status !== 'all' ? skillsStore.runFilters.status : null
  const nextVersionId = typeof skillsStore.runFilters.version_id === 'number' && skillsStore.runFilters.version_id > 0
    ? String(skillsStore.runFilters.version_id)
    : null
  const nextPage = page > 1 ? String(page) : null
  const nextPageSize = pageSize !== 20 ? String(pageSize) : null
  const currentSearch = extractQueryString(route.query.search)
  const currentStatus = extractQueryString(route.query.status)
  const currentVersionId = extractQueryString(route.query.version_id)
  const currentPage = extractQueryString(route.query.page)
  const currentPageSize = extractQueryString(route.query.page_size)

  if (nextSearch) nextQuery.search = nextSearch
  else delete nextQuery.search

  if (nextStatus) nextQuery.status = nextStatus
  else delete nextQuery.status

  if (nextVersionId) nextQuery.version_id = nextVersionId
  else delete nextQuery.version_id

  if (nextPage) nextQuery.page = nextPage
  else delete nextQuery.page

  if (nextPageSize) nextQuery.page_size = nextPageSize
  else delete nextQuery.page_size

  if (
    currentSearch === nextSearch &&
    currentStatus === nextStatus &&
    currentVersionId === nextVersionId &&
    currentPage === nextPage &&
    currentPageSize === nextPageSize
  ) {
    return false
  }

  suppressRouteRunsReload = true
  await router.replace({ query: nextQuery })
  return true
}

async function loadPage(force = false): Promise<void> {
  if (!skillId.value) return
  loadedSkillId.value = null
  try {
    syncRouteFilters()
    await skillsStore.loadSkillDetail(skillId.value, force)
    if (!skill.value?.owned) {
      await router.replace(skillPaths.detail(skillId.value))
      return
    }
    await skillsStore.loadVersions(skillId.value, 1, 100)
    await skillsStore.loadRuns(skillId.value, currentRoutePage(), currentRoutePageSize())
    loadedSkillId.value = skillId.value
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function reloadRuns(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<void> {
  try {
    await skillsStore.loadRuns(skillId.value, page, pageSize)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function applySearchFilters(): Promise<void> {
  await replaceRunsQuery(1, skillsStore.runsPagination.page_size)
  await reloadRuns(1, skillsStore.runsPagination.page_size)
}

async function resetFilters(): Promise<void> {
  skillsStore.resetRunFilters()
  await replaceRunsQuery(1, 20)
  await reloadRuns(1, 20)
}

async function handlePageChange(page: number): Promise<void> {
  await replaceRunsQuery(page, skillsStore.runsPagination.page_size)
  await reloadRuns(page, skillsStore.runsPagination.page_size)
}

async function handlePageSizeChange(pageSize: number): Promise<void> {
  await replaceRunsQuery(1, pageSize)
  await reloadRuns(1, pageSize)
}

watch(
  () => skillId.value,
  () => {
    void loadPage(true)
  }
)

watch(
  () => [route.query.search, route.query.status, route.query.version_id, route.query.page, route.query.page_size],
  async () => {
    syncRouteFilters()
    if (suppressRouteRunsReload) {
      suppressRouteRunsReload = false
      return
    }
    if (!skillId.value) return
    await reloadRuns()
  }
)

onMounted(() => {
  void loadPage()
})
</script>
