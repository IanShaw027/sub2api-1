<template>
  <AppLayout>
    <PageHeader class="proxies-header" :title="t('admin.proxies.title')" :description="t('admin.proxies.description')">
      <template #actions>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="loadProxies">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <Button variant="secondary" :disabled="batchTesting || loading" :title="t('admin.proxies.testConnection')" :aria-label="t('admin.proxies.testConnection')" @click="handleBatchTest">
          <Icon name="play" size="sm" :class="batchTesting ? 'animate-pulse' : ''" />
        </Button>
        <Button variant="secondary" :disabled="batchQualityChecking || loading" :title="t('admin.proxies.batchQualityCheck')" :aria-label="t('admin.proxies.batchQualityCheck')" @click="handleBatchQualityCheck">
          <Icon name="shield" size="sm" :class="batchQualityChecking ? 'animate-pulse' : ''" />
        </Button>
        <Button variant="secondary" :title="t('admin.proxies.dataImport')" :aria-label="t('admin.proxies.dataImport')" @click="showImportData = true"><Icon name="upload" size="sm" /></Button>
        <Button variant="secondary" :title="selectedCount > 0 ? t('admin.proxies.dataExportSelected') : t('admin.proxies.dataExport')" :aria-label="selectedCount > 0 ? t('admin.proxies.dataExportSelected') : t('admin.proxies.dataExport')" @click="showExportDataDialog = true"><Icon name="download" size="sm" /></Button>
        <Button variant="secondary" :disabled="selectedCount === 0" :title="t('admin.proxies.batchDeleteAction')" :aria-label="t('admin.proxies.batchDeleteAction')" @click="openBatchDelete"><Icon name="trash" size="sm" /></Button>
        <div class="proxies-menu">
          <Button variant="secondary" :aria-expanded="showMoreMenu" @click="showMoreMenu = !showMoreMenu">
            <span>{{ t('common.more') }}</span>
            <Icon name="chevronDown" size="xs" />
          </Button>
          <div v-if="showMoreMenu" class="dropdown proxies-dropdown">
            <button
              type="button"
              class="dropdown-item"
              :disabled="batchTesting || loading"
              @click="runMoreMenuAction(handleBatchTest)"
            >
              <Icon name="play" size="sm" />
              <span class="proxies-dropdown-text">{{ t('admin.proxies.testConnection') }}</span>
            </button>
            <button
              type="button"
              class="dropdown-item"
              :disabled="batchQualityChecking || loading"
              @click="runMoreMenuAction(handleBatchQualityCheck)"
            >
              <Icon name="shield" size="sm" />
              <span class="proxies-dropdown-text">{{ t('admin.proxies.batchQualityCheck') }}</span>
            </button>
            <div class="dropdown-divider"></div>
            <button type="button" class="dropdown-item" @click="runMoreMenuAction(() => (showImportData = true))">
              <Icon name="upload" size="sm" />
              <span class="proxies-dropdown-text">{{ t('admin.proxies.dataImport') }}</span>
            </button>
            <button type="button" class="dropdown-item" @click="runMoreMenuAction(() => (showExportDataDialog = true))">
              <Icon name="download" size="sm" />
              <span class="proxies-dropdown-text">
                {{ selectedCount > 0 ? t('admin.proxies.dataExportSelected') : t('admin.proxies.dataExport') }}
              </span>
            </button>
            <div class="dropdown-divider"></div>
            <button
              type="button"
              class="dropdown-item dropdown-item-danger"
              :disabled="selectedCount === 0"
              @click="runMoreMenuAction(openBatchDelete)"
            >
              <Icon name="trash" size="sm" />
              <span class="proxies-dropdown-text">{{ t('admin.proxies.batchDeleteAction') }}</span>
            </button>
          </div>
        </div>
        <Button class="proxies-create-desktop" @click="showCreateModal = true">
          <Icon name="plus" size="md" />
          {{ t('admin.proxies.createProxy') }}
        </Button>
      </template>
    </PageHeader>
    <TablePageLayout>
      <template #filters>
        <FilterBar
          :search-placeholder="t('admin.proxies.searchProxies')"
          :filter-label="t('common.filter')"

        >
          <template #search>
            <div class="relative w-full">
              <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
              <input
                v-model="searchQuery"
                type="text"
                :placeholder="t('admin.proxies.searchProxies')"
                class="input pl-10"
                @input="handleSearch"
              />
            </div>
          </template>
          <template #filters>
            <Select
              v-model="filters.protocol"
              :options="protocolOptions"
              :placeholder="t('admin.proxies.allProtocols')"
              class="w-40"
              @change="loadProxies"
            />
            <Select
              v-model="filters.status"
              :options="statusOptions"
              :placeholder="t('admin.proxies.allStatus')"
              class="w-36"
              @change="loadProxies"
            />
          </template>
        </FilterBar>

      </template>

      <template #table>
        <div ref="proxyTableRef" class="flex min-h-0 flex-1 flex-col overflow-hidden">
        <DataTable
          :columns="columns"
          :data="proxies"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="id"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #header-select>
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-line text-accent focus:ring-accent"
              :checked="allVisibleSelected"
              @click.stop
              @change="toggleSelectAllVisible($event)"
            />
          </template>

          <template #cell-select="{ row }">
            <input
              type="checkbox"
              class="h-4 w-4 cursor-pointer rounded border-line text-accent focus:ring-accent"
              :checked="selectedProxyIds.has(row.id)"
              @click.stop
              @change="toggleSelectRow(row.id, $event)"
            />
          </template>

          <template #cell-name="{ value }">
            <span class="font-medium text-foreground">{{ value }}</span>
          </template>

          <template #cell-protocol="{ value }">
            <span
              v-if="value"
              :class="['badge', value.startsWith('socks5') ? 'badge-primary' : 'badge-gray']"
            >
              {{ value.toUpperCase() }}
            </span>
            <span v-else class="text-sm text-muted">-</span>
          </template>

          <template #cell-address="{ row }">
            <div class="flex items-center gap-1.5">
              <code class="code text-xs">{{ row.host }}:{{ row.port }}</code>
              <div class="relative">
                <button
                  type="button"
                  class="rounded p-0.5 text-muted hover:text-accent"
                  :title="t('admin.proxies.copyProxyUrl')"
                  @click.stop="copyProxyUrl(row)"
                  @contextmenu.prevent="toggleCopyMenu(row)"
                >
                  <Icon name="copy" size="sm" />
                </button>
                <!-- 右键展开格式选择菜单 -->
                <div
                  v-if="copyMenuProxyId === row.id"
                  class="dropdown absolute left-0 top-full mt-1 w-auto"
                >
                  <button
                    v-for="fmt in getCopyFormats(row)"
                    :key="fmt.label"
                    type="button"
                    class="dropdown-item"
                    @click.stop="copyFormat(fmt.value)"
                  >
                    <span class="truncate font-mono text-xs text-muted">{{ fmt.label }}</span>
                  </button>
                </div>
              </div>
            </div>
          </template>

          <template #cell-auth="{ row }">
            <div v-if="row.username || row.has_password" class="flex items-center gap-1.5">
              <div class="flex flex-col text-xs">
                <span v-if="row.username" class="text-foreground">{{ row.username }}</span>
                <span v-if="row.has_password" class="font-mono text-muted">
                  ••••••
                </span>
              </div>
            </div>
            <span v-else class="text-sm text-muted">-</span>
          </template>

          <template #cell-location="{ row }">
            <div class="flex items-center gap-2">
              <img
                v-if="row.country_code"
                :src="flagUrl(row.country_code)"
                :alt="row.country || row.country_code"
                class="h-4 w-6 rounded-sm"
              />
              <span v-if="formatLocation(row)" class="text-sm text-foreground">
                {{ formatLocation(row) }}
              </span>
              <span v-else class="text-sm text-muted">-</span>
            </div>
          </template>

          <template #cell-account_count="{ row, value }">
            <button
              v-if="(value || 0) > 0"
              type="button"
              class="inline-flex items-center rounded bg-surface-2 px-2 py-0.5 text-xs font-medium text-accent hover:bg-surface-3   "
              @click="openAccountsModal(row)"
            >
              {{ t('admin.groups.accountsCount', { count: value || 0 }) }}
            </button>
            <span
              v-else
              class="inline-flex items-center rounded bg-surface-2 px-2 py-0.5 text-xs font-medium text-foreground "
            >
              {{ t('admin.groups.accountsCount', { count: 0 }) }}
            </span>
          </template>

          <template #cell-latency="{ row }">
            <div class="flex flex-col gap-1">
              <span
                v-if="row.latency_status === 'failed'"
                class="badge badge-danger"
                :title="row.latency_message || undefined"
              >
                {{ t('admin.proxies.latencyFailed') }}
              </span>
              <span
                v-else-if="typeof row.latency_ms === 'number'"
                :class="['badge', row.latency_ms < 200 ? 'badge-success' : 'badge-warning']"
              >
                {{ row.latency_ms }}ms
              </span>
              <span v-else class="text-sm text-muted">-</span>
              <div
                v-if="typeof row.quality_checked === 'number'"
                class="flex items-center gap-1 text-xs text-muted"
                :title="row.quality_summary || undefined"
              >
                <span>{{ t('admin.proxies.qualityInline', { grade: row.quality_grade || '-', score: row.quality_score ?? '-' }) }}</span>
                <span class="badge" :class="qualityOverallClass(row.quality_status)">
                  {{ qualityOverallLabel(row.quality_status) }}
                </span>
              </div>
            </div>
          </template>

          <template #cell-expiry="{ row }">
            <span v-if="!row.expires_at" class="text-sm text-muted">{{ t('admin.proxies.neverExpires') }}</span>
            <div v-else class="flex flex-col text-xs">
              <span class="text-foreground">{{ formatDateTime(row.expires_at) }}</span>
              <span :class="expiryBadgeClass(row)">{{ expiryLabel(row) }}</span>
            </div>
          </template>

          <template #cell-created_at="{ row }">
            <span class="text-xs text-muted">{{ formatDateTime(row.created_at) }}</span>
          </template>

          <template #cell-status="{ value }">
            <span
              :class="[
 'badge',
 value === 'active' ? 'badge-success' : value === 'expired' ? 'badge-danger' : 'badge-danger'
 ]"
            >
              {{ t('admin.accounts.status.' + value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex items-center justify-end gap-1">
              <button type="button" class="icon-btn" :disabled="testingProxyIds.has(row.id)" :title="t('admin.proxies.testConnection')" :aria-label="t('admin.proxies.testConnection')" @click.stop="handleTestConnection(row)">
                <Icon name="play" size="sm" :class="testingProxyIds.has(row.id) ? 'animate-pulse' : ''" />
              </button>
              <button type="button" class="icon-btn" :disabled="qualityCheckingProxyIds.has(row.id)" :title="t('admin.proxies.qualityCheck')" :aria-label="t('admin.proxies.qualityCheck')" @click.stop="handleQualityCheck(row)">
                <Icon name="shield" size="sm" :class="qualityCheckingProxyIds.has(row.id) ? 'animate-pulse' : ''" />
              </button>
              <button
                type="button"
                class="icon-btn"
                :title="t('common.edit')"
                :aria-label="t('common.edit')"
                @click="handleEdit(row)"
              >
                <Icon name="edit" size="sm" />
              </button>
              <button type="button" class="icon-btn icon-btn-danger" :title="t('common.delete')" :aria-label="t('common.delete')" @click.stop="handleDelete(row)"><Icon name="trash" size="sm" /></button>
              <button
                type="button"
                class="icon-btn proxies-more-btn"
                :title="t('common.more')"
                :aria-label="t('common.more')"
                @click.stop="toggleRowMenu(row, $event)"
              >
                <Icon name="more" size="sm" :stroke-width="2.4" />
              </button>
            </div>
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.proxies.noProxiesYet')"
              :description="t('admin.proxies.createFirstProxy')"
              :action-text="t('admin.proxies.createProxy')"
              @action="showCreateModal = true"
            />
          </template>
        </DataTable>
        <ProxyRowActionsMenu
          :proxy="rowMenuProxy"
          :position="rowMenuPosition"
          :testing="rowMenuProxy ? testingProxyIds.has(rowMenuProxy.id) : false"
          :checking="rowMenuProxy ? qualityCheckingProxyIds.has(rowMenuProxy.id) : false"
          @test-connection="runRowMenuAction(() => handleTestConnection(rowMenuProxy!))"
          @quality-check="runRowMenuAction(() => handleQualityCheck(rowMenuProxy!))"
          @delete="runRowMenuAction(() => handleDelete(rowMenuProxy!))"
        />
        </div>
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

    <!-- Create / Edit Proxy Modal -->
    <ProxyFormModal
      :open="showCreateModal || showEditModal"
      :editing-proxy="showEditModal ? editingProxy : null"
      :backup-proxies="allProxiesForBackup"
      @close="showEditModal ? closeEditModal() : closeCreateModal()"
      @saved="handleProxySaved"
    />

    <!-- Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.proxies.deleteProxy')"
      :message="t('admin.proxies.deleteConfirm', { name: deletingProxy?.name })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />

    <!-- Batch Delete Confirmation Dialog -->
    <ConfirmDialog
      :show="showBatchDeleteDialog"
      :title="t('admin.proxies.batchDelete')"
      :message="t('admin.proxies.batchDeleteConfirm', { count: selectedCount })"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmBatchDelete"
      @cancel="showBatchDeleteDialog = false"
    />
    <ConfirmDialog
      :show="showExportDataDialog"
      :title="t('admin.proxies.dataExport')"
      :message="t('admin.proxies.dataExportConfirmMessage')"
      :confirm-text="t('admin.proxies.dataExportConfirm')"
      :cancel-text="t('common.cancel')"
      @confirm="handleExportData"
      @cancel="showExportDataDialog = false"
    />

    <ImportDataModal
      :show="showImportData"
      @close="showImportData = false"
      @imported="handleDataImported"
    />

    <ProxyQualityReportModal
      :open="showQualityReportDialog"
      :proxy="qualityReportProxy"
      :report="qualityReport"
      @close="closeQualityReportDialog"
    />

    <!-- Proxy Accounts Dialog -->
    <ProxyAccountsModal
      :open="showAccountsModal"
      :proxy="accountsModalProxy"
      @close="closeAccountsModal"
    />
    <TotpStepUpDialog :controller="proxyExportStepUp" />
    <Fab class="proxies-fab" :label="t('admin.proxies.createProxy')" @click="showCreateModal = true">
      <Icon name="plus" size="md" />
      {{ t('admin.proxies.createProxy') }}
    </Fab>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Proxy, ProxyQualityCheckResult } from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import Fab from '@/components/ui/Fab.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import ImportDataModal from '@/components/admin/proxy/ImportDataModal.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import ProxyFormModal from '@/components/admin/proxies/ProxyFormModal.vue'
import ProxyQualityReportModal from '@/components/admin/proxies/ProxyQualityReportModal.vue'
import ProxyAccountsModal from '@/components/admin/proxies/ProxyAccountsModal.vue'
import ProxyRowActionsMenu from '@/components/admin/proxies/ProxyRowActionsMenu.vue'
import { useClipboard } from '@/composables/useClipboard'
import { useSwipeSelect } from '@/composables/useSwipeSelect'
import { useTableSelection } from '@/composables/useTableSelection'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import TotpStepUpDialog from '@/components/auth/TotpStepUpDialog.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatDateTime } from '@/utils/format'
import { proxyExpiryBadgeClass, proxyExpiryLabelKey } from '@/utils/proxyExpiry'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard } = useClipboard()
const proxyExportStepUp = useStepUp()

const columns = computed<Column[]>(() => [
  { key: 'select', label: '', sortable: false },
  { key: 'name', label: t('admin.proxies.columns.name'), sortable: true },
  { key: 'protocol', label: t('admin.proxies.columns.protocol'), sortable: true },
  { key: 'address', label: t('admin.proxies.columns.address'), sortable: false },
  { key: 'auth', label: t('admin.proxies.columns.auth'), sortable: false },
  { key: 'location', label: t('admin.proxies.columns.location'), sortable: false },
  { key: 'account_count', label: t('admin.proxies.columns.accounts'), sortable: true },
  { key: 'latency', label: t('admin.proxies.columns.latency'), sortable: false },
  { key: 'expiry', label: t('admin.proxies.columns.expiry'), sortable: true },
  { key: 'created_at', label: t('admin.proxies.columns.createdAt'), sortable: true },
  { key: 'status', label: t('admin.proxies.columns.status'), sortable: true },
  { key: 'actions', label: t('admin.proxies.columns.actions'), sortable: false }
])

// Filter options
const protocolOptions = computed(() => [
  { value: '', label: t('admin.proxies.allProtocols') },
  { value: 'http', label: 'HTTP' },
  { value: 'https', label: 'HTTPS' },
  { value: 'socks5', label: 'SOCKS5' },
  { value: 'socks5h', label: 'SOCKS5H' }
])

const statusOptions = computed(() => [
  { value: '', label: t('admin.proxies.allStatus') },
  { value: 'active', label: t('admin.accounts.status.active') },
  { value: 'inactive', label: t('admin.accounts.status.inactive') },
  { value: 'expired', label: t('admin.proxies.expired') }
])

const proxies = ref<Proxy[]>([])
const copyMenuProxyId = ref<number | null>(null)
const copyMenuSource = ref<Proxy | null>(null)
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({
  protocol: '',
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

const showCreateModal = ref(false)
const showEditModal = ref(false)
const showImportData = ref(false)
const showDeleteDialog = ref(false)
const showBatchDeleteDialog = ref(false)
const showExportDataDialog = ref(false)
const showAccountsModal = ref(false)
const exportingData = ref(false)
const testingProxyIds = ref<Set<number>>(new Set())
const qualityCheckingProxyIds = ref<Set<number>>(new Set())
const batchTesting = ref(false)
const batchQualityChecking = ref(false)
const proxyTableRef = ref<HTMLElement | null>(null)
const showMoreMenu = ref(false)
const rowMenuProxyId = ref<number | null>(null)
const rowMenuPosition = ref<{ top: number; left: number } | null>(null)
const {
  selectedSet: selectedProxyIds,
  selectedCount,
  allVisibleSelected,
  isSelected,
  select,
  deselect,
  clear: clearSelectedProxies,
  removeMany: removeSelectedProxies,
  toggleVisible,
  batchUpdate
} = useTableSelection<Proxy>({
  rows: proxies,
  getId: (proxy) => proxy.id
})
useSwipeSelect(proxyTableRef, {
  isSelected,
  select,
  deselect,
  batchUpdate
})
const accountsModalProxy = ref<Proxy | null>(null)
const editingProxy = ref<Proxy | null>(null)
const deletingProxy = ref<Proxy | null>(null)
const showQualityReportDialog = ref(false)
const qualityReportProxy = ref<Proxy | null>(null)
const qualityReport = ref<ProxyQualityCheckResult | null>(null)

const allProxiesForBackup = ref<Proxy[]>([])
const loadBackupProxyOptions = async () => {
  allProxiesForBackup.value = await adminAPI.proxies.getAllWithCount()
}

let abortController: AbortController | null = null

const isAbortError = (error: unknown) => {
  if (!error || typeof error !== 'object') return false
  const maybeError = error as { name?: string; code?: string }
  return maybeError.name === 'AbortError' || maybeError.code === 'ERR_CANCELED'
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

const buildProxyQueryFilters = () => ({
  protocol: filters.protocol || undefined,
  status: (filters.status || undefined) as 'active' | 'inactive' | 'expired' | undefined,
  search: searchQuery.value || undefined,
  sort_by: sortState.sort_by,
  sort_order: sortState.sort_order
})

const loadProxies = async () => {
  if (abortController) {
    abortController.abort()
  }
  const currentAbortController = new AbortController()
  abortController = currentAbortController
  loading.value = true
  try {
    const response = await adminAPI.proxies.list(
      pagination.page,
      pagination.page_size,
      buildProxyQueryFilters(),
      { signal: currentAbortController.signal }
    )
    if (currentAbortController.signal.aborted || abortController !== currentAbortController) {
      return
    }
    proxies.value = response.items
    pagination.total = response.total
    pagination.pages = response.pages
  } catch (error) {
    if (isAbortError(error)) {
      return
    }
    appStore.showError(t('admin.proxies.failedToLoad'))
    console.error('Error loading proxies:', error)
  } finally {
    if (abortController === currentAbortController) {
      loading.value = false
      abortController = null
    }
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
const handleSearch = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadProxies()
  }, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadProxies()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadProxies()
}

const handleSort = (key: string, order: 'asc' | 'desc') => {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadProxies()
}

const closeCreateModal = () => {
  showCreateModal.value = false
}

const handleDataImported = () => {
  showImportData.value = false
  loadProxies()
}

const handleProxySaved = () => {
  loadProxies()
}

const handleEdit = (proxy: Proxy) => {
  editingProxy.value = proxy
  showEditModal.value = true
}

const closeEditModal = () => {
  showEditModal.value = false
  editingProxy.value = null
}

const applyLatencyResult = (
  proxyId: number,
  result: {
    success: boolean
    latency_ms?: number
    message?: string
    ip_address?: string
    country?: string
    country_code?: string
    region?: string
    city?: string
  }
) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  if (result.success) {
    target.latency_status = 'success'
    target.latency_ms = result.latency_ms
    target.ip_address = result.ip_address
    target.country = result.country
    target.country_code = result.country_code
    target.region = result.region
    target.city = result.city
  } else {
    target.latency_status = 'failed'
    target.latency_ms = undefined
    target.ip_address = undefined
    target.country = undefined
    target.country_code = undefined
    target.region = undefined
    target.city = undefined
  }
  target.latency_message = result.message
}

const summarizeQualityStatus = (result: ProxyQualityCheckResult): Proxy['quality_status'] => {
  if (result.challenge_count > 0) return 'challenge'
  if (result.failed_count > 0) return 'failed'
  if (result.warn_count > 0) return 'warn'
  return 'healthy'
}

const applyQualityResult = (proxyId: number, result: ProxyQualityCheckResult) => {
  const target = proxies.value.find((proxy) => proxy.id === proxyId)
  if (!target) return
  target.quality_status = summarizeQualityStatus(result)
  target.quality_score = result.score
  target.quality_grade = result.grade
  target.quality_summary = result.summary
  target.quality_checked = result.checked_at
}

const formatLocation = (proxy: Proxy) => {
  const parts = [proxy.country, proxy.city].filter(Boolean) as string[]
  return parts.join(' · ')
}

const flagUrl = (code: string) =>
  `https://unpkg.com/flag-icons/flags/4x3/${code.toLowerCase()}.svg`

const startTestingProxy = (proxyId: number) => {
  testingProxyIds.value = new Set([...testingProxyIds.value, proxyId])
}

const stopTestingProxy = (proxyId: number) => {
  const next = new Set(testingProxyIds.value)
  next.delete(proxyId)
  testingProxyIds.value = next
}

const startQualityCheckingProxy = (proxyId: number) => {
  qualityCheckingProxyIds.value = new Set([...qualityCheckingProxyIds.value, proxyId])
}

const stopQualityCheckingProxy = (proxyId: number) => {
  const next = new Set(qualityCheckingProxyIds.value)
  next.delete(proxyId)
  qualityCheckingProxyIds.value = next
}

const runProxyTest = async (proxyId: number, notify: boolean) => {
  startTestingProxy(proxyId)
  try {
    const result = await adminAPI.proxies.testProxy(proxyId)
    applyLatencyResult(proxyId, result)
    if (notify) {
      if (result.success) {
        const message = result.latency_ms
          ? t('admin.proxies.proxyWorkingWithLatency', { latency: result.latency_ms })
          : t('admin.proxies.proxyWorking')
        appStore.showSuccess(message)
      } else {
        appStore.showError(result.message || t('admin.proxies.proxyTestFailed'))
      }
    }
    return result
  } catch (error: any) {
    const message = error.response?.data?.detail || t('admin.proxies.failedToTest')
    applyLatencyResult(proxyId, { success: false, message })
    if (notify) {
      appStore.showError(message)
    }
    console.error('Error testing proxy:', error)
    return null
  } finally {
    stopTestingProxy(proxyId)
  }
}

const handleTestConnection = async (proxy: Proxy) => {
  await runProxyTest(proxy.id, true)
}

const handleQualityCheck = async (proxy: Proxy) => {
  startQualityCheckingProxy(proxy.id)
  try {
    const result = await adminAPI.proxies.checkProxyQuality(proxy.id)
    qualityReportProxy.value = proxy
    qualityReport.value = result
    showQualityReportDialog.value = true

    const baseStep = result.items.find((item) => item.target === 'base_connectivity')
    if (baseStep && baseStep.status === 'pass') {
      applyLatencyResult(proxy.id, {
        success: true,
        latency_ms: result.base_latency_ms,
        message: result.summary,
        ip_address: result.exit_ip,
        country: result.country,
        country_code: result.country_code
      })
    }
    applyQualityResult(proxy.id, result)

    appStore.showSuccess(
      t('admin.proxies.qualityCheckDone', { score: result.score, grade: result.grade })
    )
  } catch (error: any) {
    const message = error.response?.data?.detail || t('admin.proxies.qualityCheckFailed')
    appStore.showError(message)
    console.error('Error checking proxy quality:', error)
  } finally {
    stopQualityCheckingProxy(proxy.id)
  }
}

const runBatchProxyQualityChecks = async (ids: number[]) => {
  if (ids.length === 0) return { total: 0, healthy: 0, warn: 0, challenge: 0, failed: 0 }

  const concurrency = 3
  let index = 0
  let healthy = 0
  let warn = 0
  let challenge = 0
  let failed = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      startQualityCheckingProxy(current)
      try {
        const result = await adminAPI.proxies.checkProxyQuality(current)
        const target = proxies.value.find((proxy) => proxy.id === current)
        if (target) {
          const baseStep = result.items.find((item) => item.target === 'base_connectivity')
          if (baseStep && baseStep.status === 'pass') {
            applyLatencyResult(current, {
              success: true,
              latency_ms: result.base_latency_ms,
              message: result.summary,
              ip_address: result.exit_ip,
              country: result.country,
              country_code: result.country_code
            })
          }
        }
        applyQualityResult(current, result)
        if (result.challenge_count > 0) {
          challenge++
        } else if (result.failed_count > 0) {
          failed++
        } else if (result.warn_count > 0) {
          warn++
        } else {
          healthy++
        }
      } catch {
        failed++
      } finally {
        stopQualityCheckingProxy(current)
      }
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
  return {
    total: ids.length,
    healthy,
    warn,
    challenge,
    failed
  }
}

const closeQualityReportDialog = () => {
  showQualityReportDialog.value = false
  qualityReportProxy.value = null
  qualityReport.value = null
}

const expiryLabel = (row: Proxy): string => {
  const { key, params } = proxyExpiryLabelKey(row.expires_at, row.status)
  return params ? t(key, params) : t(key)
}

const expiryBadgeClass = (row: Proxy): string =>
  proxyExpiryBadgeClass(row.expires_at, row.status)

const qualityOverallClass = (status?: string) => {
  if (status === 'healthy') return 'badge-success'
  if (status === 'warn') return 'badge-warning'
  if (status === 'challenge') return 'badge-danger'
  return 'badge-danger'
}

const qualityOverallLabel = (status?: string) => {
  if (status === 'healthy') return t('admin.proxies.qualityStatusHealthy')
  if (status === 'warn') return t('admin.proxies.qualityStatusWarn')
  if (status === 'challenge') return t('admin.proxies.qualityStatusChallenge')
  return t('admin.proxies.qualityStatusFail')
}

const fetchAllProxiesForBatch = async (): Promise<Proxy[]> => {
  const pageSize = 200
  const result: Proxy[] = []
  let page = 1
  let totalPages = 1

  while (page <= totalPages) {
    const response = await adminAPI.proxies.list(
      page,
      pageSize,
      {
        protocol: filters.protocol || undefined,
        status: filters.status as any,
        search: searchQuery.value || undefined,
        sort_by: sortState.sort_by,
        sort_order: sortState.sort_order
      }
    )
    result.push(...response.items)
    totalPages = response.pages || 1
    page++
  }

  return result
}

const runBatchProxyTests = async (ids: number[]) => {
  if (ids.length === 0) return
  const concurrency = 5
  let index = 0

  const worker = async () => {
    while (index < ids.length) {
      const current = ids[index]
      index++
      await runProxyTest(current, false)
    }
  }

  const workers = Array.from({ length: Math.min(concurrency, ids.length) }, () => worker())
  await Promise.all(workers)
}

const handleBatchTest = async () => {
  if (batchTesting.value) return

  batchTesting.value = true
  try {
    let ids: number[] = []
    if (selectedCount.value > 0) {
      ids = Array.from(selectedProxyIds.value)
    } else {
      const allProxies = await fetchAllProxiesForBatch()
      ids = allProxies.map((proxy) => proxy.id)
    }

    if (ids.length === 0) {
      appStore.showInfo(t('admin.proxies.batchTestEmpty'))
      return
    }

    await runBatchProxyTests(ids)
    appStore.showSuccess(t('admin.proxies.batchTestDone', { count: ids.length }))
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchTestFailed'))
    console.error('Error batch testing proxies:', error)
  } finally {
    batchTesting.value = false
  }
}

const handleBatchQualityCheck = async () => {
  if (batchQualityChecking.value) return

  batchQualityChecking.value = true
  try {
    let ids: number[] = []
    if (selectedCount.value > 0) {
      ids = Array.from(selectedProxyIds.value)
    } else {
      const allProxies = await fetchAllProxiesForBatch()
      ids = allProxies.map((proxy) => proxy.id)
    }

    if (ids.length === 0) {
      appStore.showInfo(t('admin.proxies.batchQualityEmpty'))
      return
    }

    const summary = await runBatchProxyQualityChecks(ids)
    appStore.showSuccess(
      t('admin.proxies.batchQualityDone', {
        count: summary.total,
        healthy: summary.healthy,
        warn: summary.warn,
        challenge: summary.challenge,
        failed: summary.failed
      })
    )
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchQualityFailed'))
    console.error('Error batch checking quality:', error)
  } finally {
    batchQualityChecking.value = false
  }
}

const formatExportTimestamp = () => {
  const now = new Date()
  const pad2 = (value: number) => String(value).padStart(2, '0')
  return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
}

const handleExportData = async () => {
  if (exportingData.value) return
  exportingData.value = true
  try {
    const dataPayload = await proxyExportStepUp.run(() =>
      adminAPI.proxies.exportData(
        selectedCount.value > 0
          ? { ids: Array.from(selectedProxyIds.value) }
          : {
              filters: buildProxyQueryFilters()
            }
      )
    )
    const timestamp = formatExportTimestamp()
    const filename = `sub2api-proxy-${timestamp}.json`
    const blob = new Blob([JSON.stringify(dataPayload, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    link.click()
    URL.revokeObjectURL(url)
    appStore.showSuccess(t('admin.proxies.dataExported'))
  } catch (error: any) {
    if (isStepUpCancelled(error)) return
    if (isStepUpBlocked(error)) {
      appStore.showError(
        stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
          ? t('stepUp.adminApiKeyForbidden')
          : t('stepUp.notEnabled')
      )
      return
    }
    appStore.showError(error?.message || t('admin.proxies.dataExportFailed'))
  } finally {
    exportingData.value = false
    showExportDataDialog.value = false
  }
}

const handleDelete = (proxy: Proxy) => {
  if ((proxy.account_count || 0) > 0) {
    appStore.showError(t('admin.proxies.deleteBlockedInUse'))
    return
  }
  deletingProxy.value = proxy
  showDeleteDialog.value = true
}

const openBatchDelete = () => {
  if (selectedCount.value === 0) {
    return
  }
  showBatchDeleteDialog.value = true
}

const confirmDelete = async () => {
  if (!deletingProxy.value) return

  try {
    await adminAPI.proxies.delete(deletingProxy.value.id)
    appStore.showSuccess(t('admin.proxies.proxyDeleted'))
    showDeleteDialog.value = false
    removeSelectedProxies([deletingProxy.value.id])
    deletingProxy.value = null
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.failedToDelete'))
    console.error('Error deleting proxy:', error)
  }
}

const confirmBatchDelete = async () => {
  const ids = Array.from(selectedProxyIds.value)
  if (ids.length === 0) {
    showBatchDeleteDialog.value = false
    return
  }

  try {
    const result = await adminAPI.proxies.batchDelete(ids)
    const deleted = result.deleted_ids?.length || 0
    const skipped = result.skipped?.length || 0

    if (deleted > 0) {
      appStore.showSuccess(t('admin.proxies.batchDeleteDone', { deleted, skipped }))
    } else if (skipped > 0) {
      appStore.showInfo(t('admin.proxies.batchDeleteSkipped', { skipped }))
    }

    clearSelectedProxies()
    showBatchDeleteDialog.value = false
    loadProxies()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.proxies.batchDeleteFailed'))
    console.error('Error batch deleting proxies:', error)
  }
}

const openAccountsModal = (proxy: Proxy) => {
  accountsModalProxy.value = proxy
  showAccountsModal.value = true
}

const closeAccountsModal = () => {
  showAccountsModal.value = false
  accountsModalProxy.value = null
}

// ── Proxy URL copy ──
function buildAuthPart(row: any): string {
  const user = row.username ? encodeURIComponent(row.username) : ''
  const pass = row.password ? encodeURIComponent(row.password) : ''
  if (user && pass) return `${user}:${pass}@`
  if (user) return `${user}@`
  if (pass) return `:${pass}@`
  return ''
}

function buildProxyUrl(row: any): string {
  return `${row.protocol}://${buildAuthPart(row)}${row.host}:${row.port}`
}

function getCopyFormats(row: any) {
	const source = copyMenuProxyId.value === row.id && copyMenuSource.value
		? copyMenuSource.value
		: row
	const hasAuth = source.username || source.password
	const fullUrl = buildProxyUrl(source)
	const formats = [
		{ label: fullUrl, value: fullUrl },
	]
  if (hasAuth) {
    const withoutProtocol = fullUrl.replace(/^[^:]+:\/\//, '')
    formats.push({ label: withoutProtocol, value: withoutProtocol })
  }
	formats.push({ label: `${source.host}:${source.port}`, value: `${source.host}:${source.port}` })
	return formats
}

async function copyProxyUrl(row: Proxy) {
  try {
    let source: Proxy = row
    if (row.has_password && !row.password) {
      const payload = await proxyExportStepUp.run(() => adminAPI.proxies.exportData({ ids: [row.id] }))
      const exported = payload.proxies[0]
      if (!exported?.password) {
        throw new Error(t('admin.proxies.dataExportFailed'))
      }
      source = { ...row, ...exported }
    }
    copyToClipboard(buildProxyUrl(source), t('admin.proxies.urlCopied'))
  } catch (error: any) {
    if (isStepUpCancelled(error)) return
    if (isStepUpBlocked(error)) {
      appStore.showError(
        stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
          ? t('stepUp.adminApiKeyForbidden')
          : t('stepUp.notEnabled')
      )
      return
    }
    appStore.showError(
      error.response?.data?.detail || error?.message || t('admin.proxies.dataExportFailed')
    )
	} finally {
		copyMenuProxyId.value = null
		copyMenuSource.value = null
	}
}

async function toggleCopyMenu(row: Proxy) {
	if (copyMenuProxyId.value === row.id) {
		copyMenuProxyId.value = null
		copyMenuSource.value = null
		return
	}
	try {
		let source = row
		if (row.has_password && !row.password) {
			const payload = await proxyExportStepUp.run(() => adminAPI.proxies.exportData({ ids: [row.id] }))
			const exported = payload.proxies[0]
			if (!exported?.password) {
				throw new Error(t('admin.proxies.dataExportFailed'))
			}
			source = { ...row, ...exported }
		}
		copyMenuSource.value = source
		copyMenuProxyId.value = row.id
	} catch (error: any) {
		copyMenuSource.value = null
		copyMenuProxyId.value = null
		if (isStepUpCancelled(error)) return
		if (isStepUpBlocked(error)) {
			appStore.showError(
				stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
					? t('stepUp.adminApiKeyForbidden')
					: t('stepUp.notEnabled')
			)
			return
		}
		appStore.showError(
			error.response?.data?.detail || error?.message || t('admin.proxies.dataExportFailed')
		)
	}
}

function copyFormat(value: string) {
	copyToClipboard(value, t('admin.proxies.urlCopied'))
	copyMenuProxyId.value = null
	copyMenuSource.value = null
}

function closeCopyMenu() {
  copyMenuProxyId.value = null
}

const rowMenuProxy = computed(() =>
  rowMenuProxyId.value === null ? null : proxies.value.find((p) => p.id === rowMenuProxyId.value) || null
)

const closeRowMenu = () => {
  rowMenuProxyId.value = null
  rowMenuPosition.value = null
}

const toggleRowMenu = (row: Proxy, event: MouseEvent) => {
  if (rowMenuProxyId.value === row.id) {
    closeRowMenu()
    return
  }
  const buttonEl = event.currentTarget as HTMLElement
  const rect = buttonEl.getBoundingClientRect()
  const menuEstWidth = 200
  const menuEstHeight = 150
  const left = Math.max(8, Math.min(rect.right - menuEstWidth, window.innerWidth - menuEstWidth - 8))
  const spaceBelow = window.innerHeight - rect.bottom
  const top =
    spaceBelow < menuEstHeight && rect.top > spaceBelow
      ? Math.max(8, rect.top - menuEstHeight)
      : rect.bottom + 4
  rowMenuPosition.value = { top, left }
  rowMenuProxyId.value = row.id
}

const runRowMenuAction = (action: () => void) => {
  action()
  closeRowMenu()
}

const runMoreMenuAction = (action: () => void) => {
  action()
  showMoreMenu.value = false
}

const handleGlobalClick = (event: MouseEvent) => {
  const target = event.target as HTMLElement
  closeCopyMenu()
  if (rowMenuProxyId.value !== null && !target.closest('.proxies-row-menu') && !target.closest('.proxies-more-btn')) {
    closeRowMenu()
  }
  if (showMoreMenu.value && !target.closest('.proxies-menu')) {
    showMoreMenu.value = false
  }
}

onMounted(() => {
  loadProxies()
  loadBackupProxyOptions()
  document.addEventListener('click', handleGlobalClick)
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
  document.removeEventListener('click', handleGlobalClick)
})
</script>
<style scoped>
.proxies-header { flex-wrap: wrap; }
.proxies-header :deep(.ui-page-header-actions) { flex-wrap: wrap; }
.proxies-mobile-filters {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 14px;
}

.proxies-fab {
  display: none;
}
@media (max-width: 767px) {
  .proxies-create-desktop {
    display: none;
  }
  .proxies-fab {
    display: inline-flex;
  }
}

.proxies-menu {
  position: relative;
  display: inline-flex;
  flex: none;
}

.proxies-dropdown {
  top: 100%;
  right: 0;
  margin-top: 6px;
  min-width: 220px;
}

.proxies-dropdown-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.proxies-dropdown .dropdown-item:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

/* ---------- Mobile card mode (DataTable built-in) ---------- */
@media (max-width: 767px) {
  :deep(.data-table-mobile-card [data-field='location']),
  :deep(.data-table-mobile-card [data-field='latency']) {
    flex-direction: column;
    align-items: stretch;
  }
  :deep(.data-table-mobile-card [data-field='location'] > span:first-child),
  :deep(.data-table-mobile-card [data-field='latency'] > span:first-child) {
    margin-bottom: 4px;
  }
}
</style>
