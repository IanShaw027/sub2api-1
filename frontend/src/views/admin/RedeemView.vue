<template>
  <AppLayout>
    <PageHeader :title="t('admin.redeem.title')" :description="t('admin.redeem.description')">
      <template #actions>
        <Button
          variant="secondary"
          class="btn-icon"
          :disabled="loading"
          :title="t('common.refresh')"
          :aria-label="t('common.refresh')"
          @click="loadCodes"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <Button variant="secondary" @click="handleExportCodes">
          <Icon name="download" size="sm" />
          {{ t('admin.redeem.exportCsv') }}
        </Button>
        <Button class="redeem-create-desktop" @click="showGenerateDialog = true">
          <Icon name="plus" size="sm" />
          {{ t('admin.redeem.generateCodes') }}
        </Button>
      </template>
    </PageHeader>

    <TablePageLayout>
      <template #filters>
        <!-- Summary chips · 5 cols gap 10, tone dot + 20px display value -->
        <div class="summary-row">
          <button
            v-for="chip in summaryChips"
            :key="chip.key"
            type="button"
            class="summary-chip"
            :class="{ 'is-active': filters.status === chip.status }"
            :aria-pressed="filters.status === chip.status"
            @click="applyStatusChip(chip.status)"
          >
            <span class="summary-chip-label">
              <span class="summary-chip-dot" :style="{ background: chip.color }"></span>
              {{ chip.label }}
            </span>
            <span class="summary-chip-value">{{ chip.count }}</span>
          </button>
        </div>

        <!-- Filter row · 36px controls, gap 8 -->
        <div class="filter-row">
          <div class="filter-search">
            <SearchInput
              v-model="searchQuery"
              :placeholder="t('admin.redeem.searchCodes')"
              @search="handleSearchCommit"
            />
          </div>
          <Select
            v-model="filters.type"
            variant="pill"
            :pill-label="t('admin.redeem.columns.type')"
            :aria-label="t('admin.redeem.allTypes')"
            :options="filterTypeOptions"
            @change="reloadFromFirstPage"
          />
          <Select
            v-model="filters.status"
            variant="pill"
            :pill-label="t('admin.redeem.columns.status')"
            :aria-label="t('admin.redeem.allStatus')"
            :options="filterStatusOptions"
            @change="reloadFromFirstPage"
          />
          <span class="filter-count">
            {{ t('admin.redeem.selectedLabel') }} <b>{{ selectedCount }}</b> / {{ pagination.total }}
          </span>
        </div>

        <!-- Selection strip -->
        <div v-if="selectedCount > 0" class="notice notice-info redeem-selection-bar">
          <span class="font-medium">
            {{ t('admin.redeem.selectedCount', { count: selectedCount }) }}
          </span>
          <div class="flex items-center gap-2">
            <button type="button" class="btn btn-ghost btn-xs" @click="clearSelectedCodes">
              {{ t('admin.redeem.clearSelection') }}
            </button>
            <button type="button" class="btn btn-primary btn-xs" data-test="batch-update-open" @click="openBatchUpdateDialog">
              {{ t('admin.redeem.batchUpdate') }}
            </button>
          </div>
        </div>

        <div v-if="filters.status === 'unused'" class="redeem-bulk-danger-row">
          <button type="button" class="btn btn-danger btn-sm" @click="showDeleteUnusedDialog = true">
            <Icon name="trash" size="sm" />
            {{ t('admin.redeem.deleteAllUnused') }}
          </button>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="codes"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="id"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #header-select>
            <input
              data-test="select-all-codes"
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-line text-accent focus:ring-accent"
              :aria-label="t('common.selectAll')"
              :checked="allVisibleSelected"
              @click.stop
              @change="toggleSelectAllVisible($event)"
            />
          </template>

          <template #cell-select="{ row }">
            <input
              data-test="select-code"
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-line text-accent focus:ring-accent"
              :aria-label="row.code"
              :checked="selectedCodeIds.has(row.id)"
              @click.stop
              @change="toggleSelectRow(row.id, $event)"
            />
          </template>

          <template #cell-code="{ value, row }">
            <div class="flex items-center gap-1.5 min-w-0">
              <NameIdCell :name="value" :id="row.id" :meta="t('admin.redeem.types.' + row.type)">
                <template #name><code class="code redeem-code-chip">{{ value }}</code></template>
              </NameIdCell>
              <button
                type="button"
                class="icon-btn"
                :class="copiedCode === value ? 'text-success-text' : ''"
                :title="copiedCode === value ? t('admin.redeem.copied') : t('keys.copyToClipboard')"
                :aria-label="t('keys.copyToClipboard')"
                @click="copyToClipboard(value)"
              >
                <Icon :name="copiedCode === value ? 'check' : 'copy'" size="sm" :stroke-width="2" />
              </button>
            </div>
          </template>

          <template #cell-type="{ value }">
            <TypeTagCell :label="t('admin.redeem.types.' + value)" :tone="typeTagTone(value)" />
          </template>

          <template #cell-value="{ value, row }">
            <div class="flex flex-col gap-0.5 min-w-0">
              <span class="font-mono text-sm text-foreground tabular-nums">
                <template v-if="row.type === 'balance'">${{ value.toFixed(2) }}</template>
                <template v-else-if="row.type === 'subscription'">
                  {{ row.validity_days || 30 }} {{ t('admin.redeem.days') }}
                </template>
                <template v-else>{{ value }}</template>
              </span>
              <span v-if="row.type === 'subscription' && row.group" class="text-xs text-muted truncate">
                {{ row.group.name }}
              </span>
            </div>
          </template>

          <template #cell-status="{ value }">
            <StatusBadge dot :tone="statusTone(value)" :label="t('admin.redeem.status.' + value)" />
          </template>

          <template #cell-used_by="{ value, row }">
            <NameIdCell
              v-if="row.user?.email || value"
              :name="row.user?.email || t('admin.redeem.userPrefix', { id: value })"
              :id="value || null"
            />
            <span v-else class="text-muted">-</span>
          </template>

          <template #cell-used_at="{ value }">
            <span class="font-mono text-xs text-muted tabular-nums">{{ value ? formatDateTime(value) : '-' }}</span>
          </template>

          <template #cell-expires_at="{ value, row }">
            <span
              class="font-mono text-xs tabular-nums"
              :class="value && row.status === 'expired' ? 'text-danger-text' : 'text-muted'"
            >
              {{ value ? formatDateTime(value) : t('admin.redeem.neverExpires') }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-1">
              <button
                v-if="row.status === 'unused'"
                type="button"
                class="icon-btn icon-btn-danger"
                :title="t('common.delete')"
                :aria-label="t('common.delete')"
                @click="handleDelete(row)"
              >
                <Icon name="trash" size="sm" />
              </button>
              <span v-else class="text-muted">-</span>
            </div>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.redeem.deleteCode')"
      :message="t('admin.redeem.deleteCodeConfirm')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Delete Unused Codes Dialog -->
    <ConfirmDialog
      :show="showDeleteUnusedDialog"
      :title="t('admin.redeem.deleteAllUnused')"
      :message="t('admin.redeem.deleteAllUnusedConfirm')"
      :confirm-text="t('admin.redeem.deleteAll')"
      :cancel-text="t('common.cancel')"
      danger
      @confirm="confirmDeleteUnused"
      @cancel="showDeleteUnusedDialog = false"
    />

    <!-- Generate Codes Dialog -->
    <RedeemGenerateModal
      :open="showGenerateDialog"
      :subscription-groups="subscriptionGroups"
      @close="showGenerateDialog = false"
      @generated="handleGenerated"
    />

    <!-- Batch Update Dialog -->
    <RedeemBatchUpdateModal
      :open="showBatchUpdateDialog"
      :selected-ids="Array.from(selectedCodeIds)"
      :subscription-groups="subscriptionGroups"
      @close="showBatchUpdateDialog = false"
      @updated="handleBatchUpdated"
    />

    <!-- Generated Codes Result Dialog -->
    <RedeemResultModal
      :open="showResultDialog"
      :codes="generatedCodes"
      @close="closeResultDialog"
    />

    <Fab class="redeem-fab" :label="t('admin.redeem.generateCodes')" @click="showGenerateDialog = true">
      <Icon name="plus" size="md" />
      {{ t('admin.redeem.generateCodes') }}
    </Fab>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { useTableSelection } from '@/composables/useTableSelection'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import { formatDateTime } from '@/utils/format'
import type { RedeemCode, RedeemCodeType, Group } from '@/types'
import type { Column } from '@/components/common/types'
import type { StatusBadgeTone } from '@/components/ui/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import Fab from '@/components/ui/Fab.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import NameIdCell from '@/components/common/cells/NameIdCell.vue'
import TypeTagCell from '@/components/common/cells/TypeTagCell.vue'
import type { TypeTagTone } from '@/components/common/cells/TypeTagCell.vue'
import Icon from '@/components/icons/Icon.vue'
import RedeemGenerateModal from '@/components/admin/redeem/RedeemGenerateModal.vue'
import RedeemBatchUpdateModal from '@/components/admin/redeem/RedeemBatchUpdateModal.vue'
import RedeemResultModal from '@/components/admin/redeem/RedeemResultModal.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard: clipboardCopy } = useClipboard()

const showGenerateDialog = ref(false)
const showResultDialog = ref(false)
const generatedCodes = ref<RedeemCode[]>([])
const subscriptionGroups = ref<Group[]>([])

const closeResultDialog = () => {
  showResultDialog.value = false
  generatedCodes.value = []
}

const handleGenerated = (result: RedeemCode[]) => {
  showGenerateDialog.value = false
  generatedCodes.value = result
  showResultDialog.value = true
  loadCodes()
}

const columns = computed<Column[]>(() => [
  { key: 'select', label: '' },
  { key: 'code', label: t('admin.redeem.columns.code') },
  { key: 'type', label: t('admin.redeem.columns.type'), sortable: true },
  { key: 'value', label: t('admin.redeem.columns.value'), sortable: true },
  { key: 'status', label: t('admin.redeem.columns.status'), sortable: true },
  { key: 'used_by', label: t('admin.redeem.columns.usedBy') },
  { key: 'used_at', label: t('admin.redeem.columns.usedAt'), sortable: true },
  { key: 'expires_at', label: t('admin.redeem.columns.expiresAt'), sortable: true },
  { key: 'actions', label: t('admin.redeem.columns.actions') }
])

const TYPE_TAG_TONES: Record<string, TypeTagTone> = {
  balance: 'success',
  concurrency: 'accent',
  subscription: 'warning',
  invitation: 'accent'
}

const typeTagTone = (type: string): TypeTagTone => TYPE_TAG_TONES[type] ?? 'accent'

const STATUS_TONES: Record<string, StatusBadgeTone> = {
  unused: 'success',
  used: 'muted',
  expired: 'danger',
  disabled: 'danger'
}

const statusTone = (status: string): StatusBadgeTone => STATUS_TONES[status] ?? 'muted'

const filterTypeOptions = computed(() => [
  { value: '', label: t('admin.redeem.allTypes') },
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

const filterStatusOptions = computed(() => [
  { value: '', label: t('admin.redeem.allStatus') },
  { value: 'unused', label: t('admin.redeem.unused') },
  { value: 'used', label: t('admin.redeem.used') },
  { value: 'expired', label: t('admin.redeem.status.expired') },
  { value: 'disabled', label: t('admin.redeem.status.disabled') }
])

type RedeemSummaryChip = {
  key: string
  label: string
  color: string
  count: number
  status: '' | 'unused' | 'used' | 'expired' | 'disabled'
}

// Per-status counts reflect the codes currently loaded on the page (same
// convention as the tickets/orders admin lists), while the "all" chip uses
// the server-reported total for the active type/search filters.
const pageStatusCount = (status: RedeemCode['status']) =>
  codes.value.filter((code) => code.status === status).length

const summaryChips = computed<RedeemSummaryChip[]>(() => [
  { key: 'all', label: t('common.all'), color: 'var(--muted)', count: pagination.total, status: '' },
  { key: 'unused', label: t('admin.redeem.status.unused'), color: 'var(--success)', count: pageStatusCount('unused'), status: 'unused' },
  { key: 'used', label: t('admin.redeem.status.used'), color: 'var(--muted)', count: pageStatusCount('used'), status: 'used' },
  { key: 'expired', label: t('admin.redeem.status.expired'), color: 'var(--danger)', count: pageStatusCount('expired'), status: 'expired' },
  { key: 'disabled', label: t('admin.redeem.status.disabled'), color: 'var(--warning)', count: pageStatusCount('disabled'), status: 'disabled' }
])

const applyStatusChip = (status: RedeemSummaryChip['status']) => {
  filters.status = status
  reloadFromFirstPage()
}

const codes = ref<RedeemCode[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({
  type: '',
  status: ''
})
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
  pages: 0
})
const sortState = reactive({
  sort_by: 'id',
  sort_order: 'desc' as 'asc' | 'desc'
})

let abortController: AbortController | null = null

const showDeleteDialog = ref(false)
const showDeleteUnusedDialog = ref(false)
const showBatchUpdateDialog = ref(false)
const deletingCode = ref<RedeemCode | null>(null)
const copiedCode = ref<string | null>(null)

const {
  selectedSet: selectedCodeIds,
  selectedCount,
  allVisibleSelected,
  select,
  deselect,
  clear: clearSelectedCodes,
  toggleVisible
} = useTableSelection<RedeemCode>({
  rows: codes,
  getId: (code) => code.id
})

const buildRedeemQueryFilters = () => ({
  type: (filters.type || undefined) as RedeemCodeType | undefined,
  status: (filters.status || undefined) as 'used' | 'expired' | 'unused' | 'disabled' | undefined,
  search: searchQuery.value || undefined,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadCodes = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentController = new AbortController()
  abortController = currentController
  loading.value = true
  try {
    const response = await adminAPI.redeem.list(
      pagination.page,
      pagination.page_size,
      buildRedeemQueryFilters(),
      {
        signal: currentController.signal
      }
    )
    if (currentController.signal.aborted) {
      return
    }
    codes.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(t('admin.redeem.failedToLoad'))
    console.error('Error loading redeem codes:', error)
  } finally {
    if (abortController === currentController && !currentController.signal.aborted) {
      loading.value = false
      abortController = null
    }
  }
}

const reloadFromFirstPage = () => {
  pagination.page = 1
  loadCodes()
}

// SearchInput debounces internally before emitting `search`, so no extra
// debounce is needed here.
const handleSearchCommit = () => {
  reloadFromFirstPage()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadCodes()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadCodes()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadCodes()
}

const toggleSelectRow = (id: number, event: Event) => {
  const target = event.target as HTMLInputElement
  if (target.checked) {
    select(id)
    return
  }
  deselect(id)
}

const toggleSelectAllVisible = (event: Event) => {
  const target = event.target as HTMLInputElement
  toggleVisible(target.checked)
}

const openBatchUpdateDialog = () => {
  if (selectedCount.value === 0) {
    appStore.showInfo(t('admin.redeem.selectCodesFirst'))
    return
  }
  showBatchUpdateDialog.value = true
}

const handleBatchUpdated = () => {
  showBatchUpdateDialog.value = false
  clearSelectedCodes()
  loadCodes()
}

const copyToClipboard = async (text: string) => {
  const success = await clipboardCopy(text, t('admin.redeem.copied'))
  if (success) {
    copiedCode.value = text
    setTimeout(() => {
      copiedCode.value = null
    }, 2000)
  }
}

const handleExportCodes = async () => {
  try {
    const blob = await adminAPI.redeem.exportCodes(buildRedeemQueryFilters())

    // Create download link
    const url = window.URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.csv`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    appStore.showSuccess(t('admin.redeem.codesExported'))
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToExport'))
    console.error('Error exporting codes:', error)
  }
}

const handleDelete = (code: RedeemCode) => {
  deletingCode.value = code
  showDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingCode.value) return

  try {
    await adminAPI.redeem.delete(deletingCode.value.id)
    appStore.showSuccess(t('admin.redeem.codeDeleted'))
    showDeleteDialog.value = false
    deletingCode.value = null
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDelete'))
    console.error('Error deleting code:', error)
  }
}

const confirmDeleteUnused = async () => {
  try {
    // Get all unused codes and delete them
    const unusedCodesResponse = await adminAPI.redeem.list(1, 1000, { status: 'unused' })
    const unusedCodeIds = unusedCodesResponse.items.map((code) => code.id)

    if (unusedCodeIds.length === 0) {
      appStore.showInfo(t('admin.redeem.noUnusedCodes'))
      showDeleteUnusedDialog.value = false
      return
    }

    const result = await adminAPI.redeem.batchDelete(unusedCodeIds)
    appStore.showSuccess(t('admin.redeem.codesDeleted', { count: result.deleted }))
    showDeleteUnusedDialog.value = false
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToDeleteUnused'))
    console.error('Error deleting unused codes:', error)
  }
}

// 加载订阅类型分组
const loadSubscriptionGroups = async () => {
  try {
    const groups = await adminAPI.groups.getAll()
    subscriptionGroups.value = groups
  } catch (error) {
    console.error('Error loading subscription groups:', error)
  }
}

onMounted(() => {
  loadCodes()
  loadSubscriptionGroups()
})

onUnmounted(() => {
  abortController?.abort()
})
</script>
<style scoped>
.filter-row {
  margin-top: 12px;
}

.redeem-selection-bar {
  margin-top: 12px;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.redeem-bulk-danger-row {
  margin-top: 12px;
}

.redeem-fab {
  display: none;
}

/* Compact code chip: tighten the shared .code chip's line-height/padding so the
   two-line NameIdCell (chip + type meta) fits within the recipe's 59-61px row band. */
.redeem-code-chip {
  padding: 1px 6px;
  line-height: 15px;
}

/* Tighten this table's two-line NameIdCell stacks (code+type meta, used_by email+id)
   so their combined content height keeps tbody rows within the 59-61px recipe band.
   NameIdCell's root (.cell-name-id) carries this page's scope attribute (Vue stamps a
   child component's root with the parent's scope id), so it matches directly; its
   inner name/meta lines are the child's own template and need :deep() to reach. */
.cell-name-id {
  gap: 1px;
}

.cell-name-id :deep(.cell-name-id-name) {
  line-height: 16px;
}

.cell-name-id :deep(.cell-name-id-meta) {
  line-height: 13px;
}

@media (max-width: 767px) {
  .filter-count {
    margin-left: 0;
  }

  .redeem-create-desktop {
    display: none;
  }
  .redeem-fab {
    display: inline-flex;
  }
}
</style>
