<template>
  <AppLayout>
    <div class="mx-auto flex w-full max-w-7xl flex-col gap-6">
      <div class="card p-6">
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <p class="text-xs uppercase tracking-[0.35em] text-ink-soft dark:text-dark-400">{{ t('ai.center.label', 'AI 创作中心') }}</p>
            <h1 class="mt-2 text-2xl font-bold text-ink dark:text-white">{{ t('ai.promptLibrary.title', '提示词库') }}</h1>
            <p class="mt-1 text-sm text-ink-body dark:text-dark-400">{{ t('ai.promptLibrary.subtitle', '支持公开/私有、克隆、编辑、删除。') }}</p>
          </div>
          <div class="flex gap-3">
            <button class="btn btn-secondary" :disabled="loading" @click="reloadPrompts">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="openCreateDialog">
              <Icon name="plus" size="sm" class="mr-2" />
              {{ t('ai.promptLibrary.create', '新建提示词') }}
            </button>
          </div>
        </div>

        <div class="mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <Input v-model="filters.search" :label="t('common.search', '搜索')" :placeholder="t('ai.prompt.searchPlaceholder', '搜标题、内容、标签')" />

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.promptLibrary.scope', '范围') }}</label>
            <Select :model-value="scopeFilter" :options="scopeOptions" @update:model-value="updateScopeFilter" />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.status', '状态') }}</label>
            <Select :model-value="filters.status" :options="statusOptions" @update:model-value="updateStatusFilter" />
          </div>

          <div>
            <label class="input-label mb-1.5 block">{{ t('ai.prompt.line', '线路') }}</label>
            <Select :model-value="filters.line_id" :options="lineFilterOptions" @update:model-value="updateLineFilter" />
          </div>
        </div>

        <p class="mt-3 text-xs text-ink-soft dark:text-dark-400">
          公共模板和个人模板由后端分开查询，当前页面不会伪造“全部混合分页”；切换范围会直接切到对应契约。
        </p>

        <div class="mt-4 flex justify-end gap-3">
          <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
          <button class="btn btn-primary" :disabled="loading" @click="applySearchFilters">{{ t('common.search', '搜索') }}</button>
        </div>
      </div>

      <div class="grid gap-5 md:grid-cols-2 xl:grid-cols-3">
        <article
          v-for="prompt in prompts"
          :key="prompt.id"
          class="card flex h-full flex-col p-5"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="min-w-0">
              <h2 class="line-clamp-2 text-lg font-semibold text-ink dark:text-white">{{ prompt.title }}</h2>
              <p class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ resolveLineLabel(prompt.line_id, prompt.line_name) }}</p>
            </div>
            <div class="flex flex-col items-end gap-2">
              <span class="rounded-full px-2.5 py-1 text-[11px] font-medium"
                :class="prompt.visibility === 'private'
                  ? 'bg-page text-ink-body dark:bg-dark-800 dark:text-dark-200'
                  : 'bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-200'"
              >
                {{ prompt.visibility === 'private' ? t('ai.prompt.private', '私有') : t('ai.prompt.public', '公开') }}
              </span>
              <span class="rounded-full bg-amber-50 px-2.5 py-1 text-[11px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-100">
                {{ statusLabel(prompt.status) }}
              </span>
            </div>
          </div>

          <p class="mt-4 line-clamp-5 flex-1 whitespace-pre-wrap text-sm text-ink-body dark:text-dark-400">{{ prompt.content }}</p>

          <div class="mt-4 flex flex-wrap gap-2">
            <span
              v-for="tag in prompt.tags"
              :key="tag"
              class="rounded-full bg-line px-2 py-1 text-[11px] text-ink-body dark:bg-dark-800 dark:text-dark-300"
            >
              #{{ tag }}
            </span>
          </div>

          <div class="mt-5 flex flex-wrap gap-2">
            <button class="btn btn-secondary btn-sm" @click="clonePromptItem(prompt)">{{ t('ai.prompt.clone', '克隆') }}</button>
            <button v-if="prompt.is_mine" class="btn btn-secondary btn-sm" @click="openEditDialog(prompt)">{{ t('common.edit', '编辑') }}</button>
            <button v-if="prompt.is_mine" class="btn btn-danger btn-sm" @click="requestDelete(prompt)">{{ t('common.delete', '删除') }}</button>
          </div>
        </article>

        <div v-if="!loading && prompts.length === 0" class="md:col-span-2 xl:col-span-3">
          <div class="card p-12">
            <EmptyState
              :title="t('ai.promptLibrary.emptyTitle', '暂无提示词')"
              :description="t('ai.promptLibrary.emptyDesc', '创建一个新模板，或克隆公开模板作为你的私有版本。')"
              :action-text="t('ai.promptLibrary.create', '新建提示词')"
              @action="openCreateDialog"
            />
          </div>
        </div>
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

    <AiPromptEditorDialog
      :show="showEditor"
      :title="editingPrompt ? t('ai.prompt.edit', '编辑提示词') : t('ai.prompt.create', '新建提示词')"
      :submit-label="editingPrompt ? t('common.save', '保存') : t('common.create', '创建')"
      :prompt="editingPrompt"
      :line-options="editorLineOptions"
      @close="closeEditor"
      @save="handleSavePrompt"
    />

    <ConfirmDialog
      :show="promptToDelete !== null"
      :title="t('ai.prompt.delete', '删除提示词')"
      :message="t('ai.prompt.deleteConfirm', '删除后不可恢复，确认继续？')"
      danger
      @cancel="promptToDelete = null"
      @confirm="confirmDeletePrompt"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import AiPromptEditorDialog from '@/components/ai/AiPromptEditorDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { cloneAIPrompt, createAIPrompt, deleteAIPrompt, listAIPrompts, updateAIPrompt } from '@/api'
import { useAiStudioStore, useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { AiPromptTemplate, BasePaginationResponse, CreateAiPromptRequest } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const aiStore = useAiStudioStore()
const route = useRoute()
const router = useRouter()

const loading = ref(false)
const prompts = ref<AiPromptTemplate[]>([])
const showEditor = ref(false)
const editingPrompt = ref<AiPromptTemplate | null>(null)
const promptToDelete = ref<AiPromptTemplate | null>(null)
const scopeFilter = ref<'public' | 'private' | 'mine'>('public')
let suppressRoutePromptReload = false

const pagination = reactive<BasePaginationResponse<AiPromptTemplate>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 18,
  pages: 1
})

const filters = reactive({
  search: '',
  status: 'all' as 'all' | 'draft' | 'published' | 'archived' | 'hidden',
  line_id: 'all' as number | 'all'
})

const scopeOptions = [
  { value: 'public', label: t('ai.promptLibrary.publicTemplates', '公共模板') },
  { value: 'mine', label: t('ai.promptLibrary.mineTemplates', '我的模板') },
  { value: 'private', label: t('ai.promptLibrary.privateTemplates', '我的私有') }
]

const statusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'draft', label: t('ai.prompt.draft', '草稿') },
  { value: 'published', label: t('ai.prompt.published', '已发布') },
  { value: 'archived', label: t('ai.prompt.archived', '已归档') },
  { value: 'hidden', label: t('ai.prompt.hidden', '隐藏') }
]

const lineFilterOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  ...aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: `${line.label} · ${line.key_count}`
  }))
])

const editorLineOptions = computed(() => [
  { value: null, label: t('ai.prompt.noLine', '不绑定线路') },
  ...aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: line.label
  }))
])

function resolveLineLabel(lineId?: number | null, fallback?: string | null): string {
  if (fallback) return fallback
  if (typeof lineId === 'number') {
    return aiStore.availableLines.find((line) => line.group_id === lineId)?.label ?? t('ai.line.unassigned', '未分配线路')
  }
  return t('ai.line.unassigned', '未分配线路')
}

function statusLabel(status: AiPromptTemplate['status']): string {
  return {
    draft: t('ai.prompt.draft', '草稿'),
    published: t('ai.prompt.published', '已发布'),
    archived: t('ai.prompt.archived', '已归档'),
    hidden: t('ai.prompt.hidden', '隐藏')
  }[status]
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

function syncRouteFilters(): void {
  filters.search = extractQueryString(route.query.search) ?? ''
  updateScopeFilter(extractQueryString(route.query.scope))
  updateStatusFilter(extractQueryString(route.query.status))
  const lineId = extractPositiveQueryNumber(route.query.line_id, 0)
  filters.line_id = lineId > 0 ? lineId : 'all'
}

function currentRoutePage(): number {
  return extractPositiveQueryNumber(route.query.page, 1)
}

function currentRoutePageSize(): number {
  return extractPositiveQueryNumber(route.query.page_size, pagination.page_size)
}

async function replacePromptQuery(page = currentRoutePage(), pageSize = currentRoutePageSize()): Promise<boolean> {
  const nextQuery = { ...route.query }
  const nextSearch = filters.search.trim() || null
  const nextScope = scopeFilter.value !== 'public' ? scopeFilter.value : null
  const nextStatus = filters.status !== 'all' ? filters.status : null
  const nextLineId = typeof filters.line_id === 'number' && filters.line_id > 0 ? String(filters.line_id) : null
  const nextPage = page > 1 ? String(page) : null
  const nextPageSize = pageSize !== 18 ? String(pageSize) : null
  const currentSearch = extractQueryString(route.query.search)
  const currentScope = extractQueryString(route.query.scope)
  const currentStatus = extractQueryString(route.query.status)
  const currentLineId = extractQueryString(route.query.line_id)
  const currentPage = extractQueryString(route.query.page)
  const currentPageSize = extractQueryString(route.query.page_size)

  if (nextSearch) nextQuery.search = nextSearch
  else delete nextQuery.search

  if (nextScope) nextQuery.scope = nextScope
  else delete nextQuery.scope

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
    currentScope === nextScope &&
    currentStatus === nextStatus &&
    currentLineId === nextLineId &&
    currentPage === nextPage &&
    currentPageSize === nextPageSize
  ) {
    return false
  }

  suppressRoutePromptReload = true
  await router.replace({ query: nextQuery })
  return true
}

async function loadPrompts(page = currentRoutePage(), pageSize = currentRoutePageSize()) {
  loading.value = true
  try {
    await aiStore.loadRuntimeLines()
    const visibility = scopeFilter.value === 'public' ? 'public' : scopeFilter.value === 'private' ? 'private' : undefined
    const scope = scopeFilter.value === 'public' ? 'library' : 'mine'
    const response = await listAIPrompts(page, pageSize, {
      search: filters.search.trim() || undefined,
      visibility,
      status: filters.status,
      line_id: filters.line_id,
      scope
    })
    prompts.value = response.items
    Object.assign(pagination, response)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  } finally {
    loading.value = false
  }
}

function updateScopeFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'public')
  scopeFilter.value = next === 'public' || next === 'private' || next === 'mine' ? next : 'public'
}

function updateStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.status = next === 'draft' || next === 'published' || next === 'archived' || next === 'hidden' ? next : 'all'
}

function updateLineFilter(value: string | number | boolean | null) {
  filters.line_id = typeof value === 'number' ? value : 'all'
}

async function reloadPrompts(): Promise<void> {
  await loadPrompts()
}

async function applySearchFilters(): Promise<void> {
  await replacePromptQuery(1, pagination.page_size)
  await loadPrompts(1, pagination.page_size)
}

async function resetFilters() {
  filters.search = ''
  filters.status = 'all'
  filters.line_id = 'all'
  scopeFilter.value = 'public'
  await replacePromptQuery(1, 18)
  await loadPrompts(1, 18)
}

function openCreateDialog() {
  editingPrompt.value = null
  showEditor.value = true
}

function openEditDialog(prompt: AiPromptTemplate) {
  editingPrompt.value = prompt
  showEditor.value = true
}

function closeEditor() {
  showEditor.value = false
  editingPrompt.value = null
}

async function handleSavePrompt(payload: CreateAiPromptRequest) {
  try {
    if (editingPrompt.value) {
      await updateAIPrompt(editingPrompt.value.id, payload)
      appStore.showSuccess(t('ai.prompt.saved', '提示词已更新'))
    } else {
      await createAIPrompt(payload)
      appStore.showSuccess(t('ai.prompt.created', '提示词已创建'))
    }
    closeEditor()
    await reloadPrompts()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function clonePromptItem(prompt: AiPromptTemplate) {
  try {
    await cloneAIPrompt(prompt.id)
    appStore.showSuccess(t('ai.prompt.cloned', '提示词已克隆'))
    await reloadPrompts()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

function requestDelete(prompt: AiPromptTemplate) {
  promptToDelete.value = prompt
}

async function confirmDeletePrompt() {
  if (!promptToDelete.value) return
  try {
    await deleteAIPrompt(promptToDelete.value.id)
    appStore.showSuccess(t('ai.prompt.deleted', '提示词已删除'))
    promptToDelete.value = null
    await reloadPrompts()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function handlePageChange(page: number) {
  await replacePromptQuery(page, pagination.page_size)
  await loadPrompts(page, pagination.page_size)
}

async function handlePageSizeChange(size: number) {
  await replacePromptQuery(1, size)
  await loadPrompts(1, size)
}

watch(
  () => [route.query.search, route.query.scope, route.query.status, route.query.line_id, route.query.page, route.query.page_size],
  async () => {
    syncRouteFilters()
    if (suppressRoutePromptReload) {
      suppressRoutePromptReload = false
      return
    }
    await loadPrompts()
  },
  { immediate: true }
)
</script>
