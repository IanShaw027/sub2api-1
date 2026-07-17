<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
          <div class="grid flex-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Input v-model="filters.search" :label="t('common.search', '搜索')" :placeholder="t('ai.prompt.searchPlaceholder', '搜标题、内容、标签')" />

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
          </div>
        </div>
      </template>

      <template #actions>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
          <button class="btn btn-secondary" :disabled="loading" @click="loadPrompts">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </template>

      <template #table>
        <DataTable :columns="columns" :data="prompts" :loading="loading">
          <template #cell-title="{ row }">
            <div class="min-w-[220px]">
              <div class="font-medium text-ink dark:text-white">{{ row.title }}</div>
              <div class="mt-1 line-clamp-2 text-xs text-ink-soft dark:text-ink-soft">{{ row.content }}</div>
            </div>
          </template>

          <template #cell-owner_name="{ value }">
            <span class="text-sm text-ink-body dark:text-ink-body">{{ value || '-' }}</span>
          </template>

          <template #cell-line_name="{ row }">
            <span class="text-sm text-ink-body dark:text-ink-body">{{ resolveLineLabel(row.line_id, row.line_name) }}</span>
          </template>

          <template #cell-visibility="{ value }">
            <span class="rounded-full px-2.5 py-1 text-xs font-medium"
              :class="value === 'private'
                ? 'bg-ink-faint/10 text-ink-body dark:bg-dark-800 dark:text-ink'
                : 'bg-brand-50 text-brand-700 dark:bg-brand-900/30 dark:text-brand-200'"
            >
              {{ value === 'private' ? t('ai.prompt.private', '私有') : t('ai.prompt.public', '公开') }}
            </span>
          </template>

          <template #cell-status="{ value }">
            <span class="rounded-full bg-amber-50 px-2.5 py-1 text-xs font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-100">
              {{ statusLabel(value) }}
            </span>
          </template>

          <template #cell-updated_at="{ value }">
            <span class="text-sm text-ink-soft dark:text-ink-soft">{{ formatTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center gap-2">
              <button class="btn btn-secondary btn-sm" @click="openEditDialog(row)">{{ t('common.edit', '编辑') }}</button>
              <button class="btn btn-secondary btn-sm" @click="togglePromptVisibility(row)">
                {{ row.visibility === 'public' ? t('ai.prompt.makePrivate', '转私有') : t('ai.prompt.makePublic', '转公开') }}
              </button>
              <button class="btn btn-danger btn-sm" @click="requestDelete(row)">{{ t('common.delete', '删除') }}</button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('ai.promptGovernance.emptyTitle', '暂无提示词记录')"
              :description="t('ai.promptGovernance.emptyDesc', '用户创建或克隆后的提示词会出现在这里。')"
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

    <AiPromptEditorDialog
      :show="showEditor"
      :title="t('ai.promptGovernance.editTitle', '治理提示词')"
      :submit-label="t('common.save', '保存')"
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
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import AiPromptEditorDialog from '@/components/ai/AiPromptEditorDialog.vue'
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
import type { AiPromptTemplate, BasePaginationResponse, CreateAiPromptRequest } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()
const aiStore = useAiStudioStore()

const loading = ref(false)
const prompts = ref<AiPromptTemplate[]>([])
const showEditor = ref(false)
const editingPrompt = ref<AiPromptTemplate | null>(null)
const promptToDelete = ref<AiPromptTemplate | null>(null)

const pagination = reactive<BasePaginationResponse<AiPromptTemplate>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1
})

const filters = reactive({
  search: '',
  visibility: 'all' as 'all' | 'public' | 'private',
  status: 'all' as 'all' | 'draft' | 'published' | 'archived' | 'hidden',
  line_id: 'all' as number | 'all'
})

const visibilityOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'public', label: t('ai.prompt.public', '公开') },
  { value: 'private', label: t('ai.prompt.private', '私有') }
]

const statusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'draft', label: t('ai.prompt.draft', '草稿') },
  { value: 'published', label: t('ai.prompt.published', '已发布') },
  { value: 'archived', label: t('ai.prompt.archived', '已归档') },
  { value: 'hidden', label: t('ai.prompt.hidden', '隐藏') }
]

const lineOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  ...aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: line.label
  }))
])

const editorLineOptions = computed(() => [
  { value: null, label: t('ai.prompt.noLine', '不绑定线路') },
  ...aiStore.availableLines.map((line) => ({
    value: line.group_id,
    label: line.label
  }))
])

const columns = computed<Column[]>(() => [
  { key: 'title', label: t('ai.prompt.title', '标题') },
  { key: 'owner_name', label: t('ai.prompt.owner', '作者') },
  { key: 'line_name', label: t('ai.prompt.line', '线路') },
  { key: 'visibility', label: t('ai.prompt.visibility', '可见性') },
  { key: 'status', label: t('ai.prompt.status', '状态') },
  { key: 'updated_at', label: t('common.updatedAt', '更新时间') },
  { key: 'actions', label: t('common.actions', '操作') }
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

function formatTime(value: string): string {
  return new Date(value).toLocaleString()
}

function updateVisibilityFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.visibility = next === 'public' || next === 'private' ? next : 'all'
}

function updateStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.status = next === 'draft' || next === 'published' || next === 'archived' || next === 'hidden' ? next : 'all'
}

function updateLineFilter(value: string | number | boolean | null) {
  filters.line_id = typeof value === 'number' ? value : 'all'
}

async function loadPrompts() {
  loading.value = true
  try {
    await aiStore.loadRuntimeLines()
    const response = await adminAIAPI.listPrompts(pagination.page, pagination.page_size, {
      search: filters.search.trim() || undefined,
      visibility: filters.visibility,
      status: filters.status,
      line_id: filters.line_id
    })
    prompts.value = response.items
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
  pagination.page = 1
  void loadPrompts()
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
  if (!editingPrompt.value) return
  try {
    await adminAIAPI.updatePrompt(editingPrompt.value.id, payload)
    appStore.showSuccess(t('ai.prompt.saved', '提示词已更新'))
    closeEditor()
    await loadPrompts()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

async function togglePromptVisibility(prompt: AiPromptTemplate) {
  try {
    await adminAIAPI.updatePrompt(prompt.id, {
      visibility: prompt.visibility === 'public' ? 'private' : 'public'
    })
    await loadPrompts()
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
    await adminAIAPI.deletePrompt(promptToDelete.value.id)
    promptToDelete.value = null
    appStore.showSuccess(t('ai.prompt.deleted', '提示词已删除'))
    await loadPrompts()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error')))
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadPrompts()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadPrompts()
}

onMounted(async () => {
  await loadPrompts()
})
</script>
