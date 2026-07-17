<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <MonitorFiltersBar
          v-model:search="searchQuery"
          v-model:provider="providerFilter"
          v-model:enabled="enabledFilter"
          :loading="loading"
          @reload="reload"
          @filter-change="applyFilters"
          @create="openCreateDialog"
          @manage-templates="showTemplateManager = true"
          @search-input="handleSearch"
        />
      </template>

      <template #table>
        <div
          class="h-full"
          :class="groupedMonitors.length > 0 ? 'overflow-y-auto pr-1' : ''"
        >
          <div class="space-y-6" :class="groupedMonitors.length === 0 ? 'h-full' : 'pb-1'">
          <template v-if="groupedMonitors.length === 0">
            <DataTable :columns="columns" :data="monitors" :loading="loading">
              <template #cell-name="{ row, value }">
                <div class="flex items-center gap-1.5">
                  <span class="font-medium text-ink dark:text-white">{{ value }}</span>
                  <HelpTooltip v-if="row.api_key_decrypt_failed" :content="t('admin.channelMonitor.apiKeyDecryptFailed')">
                    <Icon name="exclamationTriangle" size="sm" class="text-danger" />
                  </HelpTooltip>
                </div>
              </template>

              <template #cell-provider="{ row }">
                <span class="inline-flex items-center rounded-chip px-2 py-0.5 text-xs font-medium" :class="providerBadgeClass(row.provider)">
                  {{ providerLabel(row.provider) }}
                </span>
              </template>

              <template #cell-primary_model="{ row }">
                <MonitorPrimaryModelCell :row="row" />
              </template>

              <template #cell-availability_7d="{ row }">
                <span class="text-sm text-ink dark:text-white">{{ formatAvailability(row) }}</span>
              </template>

              <template #cell-latency="{ row }">
                <span class="text-sm text-ink dark:text-white">{{ formatLatency(row.primary_latency_ms) }}</span>
              </template>

              <template #cell-image_usage="{ row }">
                <div v-if="row.provider === 'openai'" class="min-w-[220px] whitespace-normal">
                  <div v-if="imageUsageLoadingMap[row.id]" class="text-xs text-ink-faint dark:text-dark-500">
                    {{ t('common.loading') }}
                  </div>
                  <div v-else-if="openAIImageUsageByMonitorId[row.id]?.length" class="space-y-1">
                    <div
                      v-for="item in openAIImageUsageByMonitorId[row.id]"
                      :key="item.key"
                      class="rounded-control border border-line/80 px-2 py-1 dark:border-dark-700"
                    >
                      <div class="mb-1 flex flex-wrap items-center gap-1.5 text-[10px] text-ink-soft dark:text-dark-400">
                        <span class="rounded-chip bg-page px-1.5 py-0.5 font-medium text-ink-body dark:bg-dark-700 dark:text-dark-200">
                          {{ item.label }}
                        </span>
                        <span class="rounded-chip bg-page px-1.5 py-0.5 dark:bg-dark-700">
                          {{ item.requestsLabel }}
                        </span>
                        <span class="rounded-chip bg-page px-1.5 py-0.5 dark:bg-dark-700">
                          {{ item.costLabel }}
                        </span>
                      </div>
                      <div class="text-[10px] text-ink-faint dark:text-dark-500">
                        {{ item.resetLabel }}
                      </div>
                    </div>
                  </div>
                  <span v-else class="text-sm text-ink-faint dark:text-dark-500">-</span>
                </div>
                <span v-else class="text-sm text-ink-faint dark:text-dark-500">-</span>
              </template>

              <template #cell-enabled="{ row }">
                <Toggle :modelValue="row.enabled" @update:modelValue="toggleEnabled(row)" />
              </template>

              <template #cell-actions="{ row }">
                <MonitorActionsCell
                  :row="row"
                  :running="runningId === row.id"
                  @run="handleRunNow"
                  @edit="openEditDialog"
                  @adjust-availability="openAvailabilityAdjustDialog"
                  @delete="handleDelete"
                />
              </template>

              <template #empty>
                <EmptyState
                  :title="t('admin.channelMonitor.noMonitorsYet')"
                  :description="t('admin.channelMonitor.createFirstMonitor')"
                  :action-text="t('admin.channelMonitor.createButton')"
                  @action="openCreateDialog"
                />
              </template>
            </DataTable>
          </template>

          <template v-else>
            <section
              v-for="group in groupedMonitors"
              :key="group.provider"
              class="space-y-3"
            >
              <button
                type="button"
                class="flex w-full items-center justify-between gap-3 rounded-card border border-line bg-card px-3 py-2 text-left shadow-xs transition-colors hover:border-ink-faint hover:bg-page dark:border-dark-700 dark:bg-dark-900 dark:hover:border-dark-600 dark:hover:bg-dark-800"
                @click="toggleProviderCollapse(group.provider)"
              >
                <div class="flex items-center gap-2">
                  <Icon
                    :name="isProviderCollapsed(group.provider) ? 'chevronRight' : 'chevronDown'"
                    size="sm"
                    class="text-ink-faint dark:text-dark-500"
                  />
                  <span class="inline-flex items-center rounded-chip px-2 py-0.5 text-xs font-medium" :class="providerBadgeClass(group.provider)">
                    {{ providerLabel(group.provider) }}
                  </span>
                  <span class="text-sm text-ink-soft dark:text-dark-400">{{ group.items.length }}</span>
                </div>
              </button>

              <DataTable
                v-if="!isProviderCollapsed(group.provider)"
                :columns="columns"
                :data="group.items"
                :loading="loading"
              >
                <template #cell-name="{ row, value }">
                  <div class="flex items-center gap-1.5">
                    <span class="font-medium text-ink dark:text-white">{{ value }}</span>
                    <HelpTooltip v-if="row.api_key_decrypt_failed" :content="t('admin.channelMonitor.apiKeyDecryptFailed')">
                      <Icon name="exclamationTriangle" size="sm" class="text-danger" />
                    </HelpTooltip>
                  </div>
                </template>

                <template #cell-provider="{ row }">
                  <span class="inline-flex items-center rounded-chip px-2 py-0.5 text-xs font-medium" :class="providerBadgeClass(row.provider)">
                    {{ providerLabel(row.provider) }}
                  </span>
                </template>

                <template #cell-primary_model="{ row }">
                  <MonitorPrimaryModelCell :row="row" />
                </template>

                <template #cell-availability_7d="{ row }">
                  <span class="text-sm text-ink dark:text-white">{{ formatAvailability(row) }}</span>
                </template>

                <template #cell-latency="{ row }">
                  <span class="text-sm text-ink dark:text-white">{{ formatLatency(row.primary_latency_ms) }}</span>
                </template>

                <template #cell-image_usage="{ row }">
                  <div v-if="row.provider === 'openai'" class="min-w-[220px] whitespace-normal">
                    <div v-if="imageUsageLoadingMap[row.id]" class="text-xs text-ink-faint dark:text-dark-500">
                      {{ t('common.loading') }}
                    </div>
                    <div v-else-if="openAIImageUsageByMonitorId[row.id]?.length" class="space-y-1">
                      <div
                        v-for="item in openAIImageUsageByMonitorId[row.id]"
                        :key="item.key"
                        class="rounded-control border border-line/80 px-2 py-1 dark:border-dark-700"
                      >
                        <div class="mb-1 flex flex-wrap items-center gap-1.5 text-[10px] text-ink-soft dark:text-dark-400">
                          <span class="rounded-chip bg-page px-1.5 py-0.5 font-medium text-ink-body dark:bg-dark-700 dark:text-dark-200">
                            {{ item.label }}
                          </span>
                          <span class="rounded-chip bg-page px-1.5 py-0.5 dark:bg-dark-700">
                            {{ item.requestsLabel }}
                          </span>
                          <span class="rounded-chip bg-page px-1.5 py-0.5 dark:bg-dark-700">
                            {{ item.costLabel }}
                          </span>
                        </div>
                        <div class="text-[10px] text-ink-faint dark:text-dark-500">
                          {{ item.resetLabel }}
                        </div>
                      </div>
                    </div>
                    <span v-else class="text-sm text-ink-faint dark:text-dark-500">-</span>
                  </div>
                  <span v-else class="text-sm text-ink-faint dark:text-dark-500">-</span>
                </template>

                <template #cell-enabled="{ row }">
                  <Toggle :modelValue="row.enabled" @update:modelValue="toggleEnabled(row)" />
                </template>

                <template #cell-actions="{ row }">
                  <MonitorActionsCell
                    :row="row"
                    :running="runningId === row.id"
                    @run="handleRunNow"
                    @edit="openEditDialog"
                    @adjust-availability="openAvailabilityAdjustDialog"
                    @delete="handleDelete"
                  />
                </template>
              </DataTable>
            </section>
          </template>
          </div>
        </div>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />
      </template>
    </TablePageLayout>

    <MonitorFormDialog
      :show="showDialog"
      :monitor="editing"
      @close="closeDialog"
      @saved="reload"
    />

    <MonitorTemplateManagerDialog
      :show="showTemplateManager"
      @close="showTemplateManager = false"
      @updated="reload"
    />

    <MonitorRunResultDialog
      :show="showRunResult"
      :results="runResults"
      @close="showRunResult = false"
    />

    <MonitorAvailabilityAdjustDialog
      :show="showAvailabilityAdjust"
      :monitor="adjustingAvailability"
      @close="closeAvailabilityAdjustDialog"
      @adjusted="handleAvailabilityAdjusted"
    />

    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('common.delete')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type {
  AvailabilityAdjustResult,
  ChannelMonitor,
  CheckResult,
  ListParams,
  Provider,
} from '@/api/admin/channelMonitor'
import type {
  Account,
  AccountUsageInfo,
  UsageProgress,
} from '@/types'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import MonitorFiltersBar from '@/components/admin/monitor/MonitorFiltersBar.vue'
import MonitorFormDialog from '@/components/admin/monitor/MonitorFormDialog.vue'
import MonitorTemplateManagerDialog from '@/components/admin/monitor/MonitorTemplateManagerDialog.vue'
import MonitorRunResultDialog from '@/components/admin/monitor/MonitorRunResultDialog.vue'
import MonitorAvailabilityAdjustDialog from '@/components/admin/monitor/MonitorAvailabilityAdjustDialog.vue'
import MonitorPrimaryModelCell from '@/components/admin/monitor/MonitorPrimaryModelCell.vue'
import MonitorActionsCell from '@/components/admin/monitor/MonitorActionsCell.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'
import { PROVIDERS } from '@/constants/channelMonitor'
import { formatCurrency, formatDateTime } from '@/utils/format'

const { t } = useI18n()
const appStore = useAppStore()
const {
  providerLabel,
  providerBadgeClass,
  formatLatency,
  formatAvailability,
} = useChannelMonitorFormat()

const monitors = ref<ChannelMonitor[]>([])
const loading = ref(false)
const runningId = ref<number | null>(null)
const searchQuery = ref('')
const providerFilter = ref<Provider | ''>('')
const enabledFilter = ref<'' | 'true' | 'false'>('')
const pagination = reactive({ page: 1, page_size: getPersistedPageSize(), total: 0 })

const showDialog = ref(false)
const showTemplateManager = ref(false)
const editing = ref<ChannelMonitor | null>(null)
const showDeleteDialog = ref(false)
const deleting = ref<ChannelMonitor | null>(null)
const showRunResult = ref(false)
const runResults = ref<CheckResult[]>([])
const showAvailabilityAdjust = ref(false)
const adjustingAvailability = ref<ChannelMonitor | null>(null)
const collapsedProviders = ref<Record<string, boolean>>({})
const openAIOAuthAccounts = ref<Account[]>([])
const openAIOAuthAccountsLoaded = ref(false)
const openAIImageUsageByMonitorId = ref<Record<number, OpenAIImageUsageRow[]>>({})
const imageUsageLoadingMap = ref<Record<number, boolean>>({})

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
let imageUsageRequestToken = 0

interface OpenAIImageUsageRow {
  key: string
  label: string
  requestsLabel: string
  costLabel: string
  resetLabel: string
}

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.channelMonitor.columns.name'), sortable: false },
  { key: 'provider', label: t('admin.channelMonitor.columns.provider'), sortable: false },
  { key: 'primary_model', label: t('admin.channelMonitor.columns.primaryModel'), sortable: false },
  { key: 'availability_7d', label: t('admin.channelMonitor.columns.availability7d'), sortable: false },
  { key: 'latency', label: t('admin.channelMonitor.columns.latency'), sortable: false },
  {
    key: 'image_usage',
    label: t('admin.channelMonitor.columns.imageUsage'),
    sortable: false,
    class: '!whitespace-normal align-top min-w-[240px]',
  },
  { key: 'enabled', label: t('admin.channelMonitor.columns.enabled'), sortable: false },
  { key: 'actions', label: t('admin.channelMonitor.columns.actions'), sortable: false },
])

const groupedMonitors = computed(() => {
  if (monitors.value.length === 0) return []
  const order = new Map<string, number>(PROVIDERS.map((provider, index) => [provider, index]))
  const groups = new Map<string, ChannelMonitor[]>()
  for (const monitor of monitors.value) {
    const key = monitor.provider || ''
    const items = groups.get(key)
    if (items) items.push(monitor)
    else groups.set(key, [monitor])
  }
  return Array.from(groups.entries())
    .sort(([a], [b]) => (order.get(a) ?? Number.MAX_SAFE_INTEGER) - (order.get(b) ?? Number.MAX_SAFE_INTEGER) || a.localeCompare(b))
    .map(([provider, items]) => ({ provider, items }))
})

const openAIOAuthAccountMap = computed(() => {
  const sorted = [...openAIOAuthAccounts.value].sort((a, b) => {
    if (a.status === b.status) return a.id - b.id
    if (a.status === 'active') return -1
    if (b.status === 'active') return 1
    return a.id - b.id
  })
  const mapped = new Map<string, Account>()
  for (const account of sorted) {
    const normalizedName = normalizeMonitorName(account.name)
    if (normalizedName && !mapped.has(normalizedName)) {
      mapped.set(normalizedName, account)
    }
  }
  return mapped
})

const deleteConfirmMessage = computed(() => {
  const name = deleting.value?.name || ''
  return t('admin.channelMonitor.deleteConfirm', { name })
})

async function reload() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    const params: ListParams = {
      page: pagination.page,
      page_size: pagination.page_size,
    }
    if (providerFilter.value) params.provider = providerFilter.value
    if (enabledFilter.value === 'true') params.enabled = true
    if (enabledFilter.value === 'false') params.enabled = false
    if (searchQuery.value.trim()) params.search = searchQuery.value.trim()

    const res = await adminAPI.channelMonitor.list(params, { signal: ctrl.signal })
    if (ctrl.signal.aborted || abortController !== ctrl) return
    monitors.value = res.items || []
    pagination.total = res.total
    void preloadOpenAIImageUsage(monitors.value)
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.loadError')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

function handleSearch() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    reload()
  }, 300)
}

function applyFilters() {
  pagination.page = 1
  reload()
}

function onPageChange(page: number) {
  pagination.page = page
  reload()
}

function onPageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  reload()
}

function openCreateDialog() {
  editing.value = null
  showDialog.value = true
}

function openEditDialog(row: ChannelMonitor) {
  editing.value = row
  showDialog.value = true
}

function closeDialog() {
  showDialog.value = false
  editing.value = null
}

function openAvailabilityAdjustDialog(row: ChannelMonitor) {
  adjustingAvailability.value = row
  showAvailabilityAdjust.value = true
}

function closeAvailabilityAdjustDialog() {
  showAvailabilityAdjust.value = false
  adjustingAvailability.value = null
}

function handleAvailabilityAdjusted(result: AvailabilityAdjustResult) {
  appStore.showSuccess(t('admin.channelMonitor.adjustAvailability.success', {
    value: `${result.actual_availability_pct.toFixed(2)}%`,
    rows: result.changed_rows,
  }))
  closeAvailabilityAdjustDialog()
  void reload()
}

async function toggleEnabled(row: ChannelMonitor) {
  const next = !row.enabled
  try {
    await adminAPI.channelMonitor.update(row.id, { enabled: next })
    row.enabled = next
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

async function handleRunNow(row: ChannelMonitor) {
  if (runningId.value != null) return
  runningId.value = row.id
  try {
    const res = await adminAPI.channelMonitor.runNow(row.id)
    runResults.value = res.results || []
    showRunResult.value = true
    appStore.showSuccess(t('admin.channelMonitor.runSuccess'))
    // Refresh row to get latest status from backend
    void reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.runFailed')))
  } finally {
    runningId.value = null
  }
}

function handleDelete(row: ChannelMonitor) {
  deleting.value = row
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deleting.value) return
  try {
    await adminAPI.channelMonitor.del(deleting.value.id)
    appStore.showSuccess(t('admin.channelMonitor.deleteSuccess'))
    showDeleteDialog.value = false
    deleting.value = null
    reload()
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('common.error')))
  }
}

function normalizeMonitorName(value: string | null | undefined): string {
  return (value || '').trim().toLowerCase()
}

function toggleProviderCollapse(provider: string) {
  collapsedProviders.value = {
    ...collapsedProviders.value,
    [provider]: !collapsedProviders.value[provider],
  }
}

function isProviderCollapsed(provider: string): boolean {
  return collapsedProviders.value[provider] === true
}

function formatImageRequests(value: number): string {
  return `${value.toLocaleString()} img`
}

function formatResetCountdown(progress: UsageProgress | null | undefined): string {
  if (!progress) return '-'
  const remainingSeconds = Number(progress.remaining_seconds || 0)
  if (remainingSeconds > 0) {
    const days = Math.floor(remainingSeconds / 86400)
    const hours = Math.floor((remainingSeconds % 86400) / 3600)
    const minutes = Math.floor((remainingSeconds % 3600) / 60)
    if (days > 0) return hours > 0 ? `${days}d ${hours}h` : `${days}d`
    if (hours > 0) return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`
    if (minutes > 0) return `${minutes}m`
    return `${remainingSeconds}s`
  }
  if (progress.resets_at) {
    return formatDateTime(progress.resets_at)
  }
  return '-'
}

function buildUsageRow(key: string, label: string, progress: UsageProgress | null | undefined): OpenAIImageUsageRow | null {
  if (!progress) return null
  const requests = Number(progress.window_stats?.requests ?? progress.used_requests ?? 0)
  const cost = Number(progress.window_stats?.cost ?? 0)
  return {
    key,
    label,
    requestsLabel: formatImageRequests(requests),
    costLabel: formatCurrency(cost),
    resetLabel: formatResetCountdown(progress),
  }
}

function buildOpenAIImageUsageRows(usage: AccountUsageInfo | null | undefined): OpenAIImageUsageRow[] {
  if (!usage) return []
  return [
    buildUsageRow('codex-5h', 'codex 5h', usage.openai_image_codex_five_hour),
    buildUsageRow('codex-7d', 'codex 7d', usage.openai_image_codex_seven_day),
  ].filter((item): item is OpenAIImageUsageRow => item !== null)
}

async function ensureOpenAIOAuthAccountsLoaded(forceReload = false) {
  if (!forceReload && openAIOAuthAccountsLoaded.value) return
  const accounts: Account[] = []
  let page = 1
  let pages = 1

  do {
    const res = await adminAPI.accounts.list(page, 200, {
      platform: 'openai',
      type: 'oauth',
    })
    accounts.push(...(res.items || []))
    pages = Number(res.pages || 1)
    page += 1
  } while (page <= pages)

  openAIOAuthAccounts.value = accounts
  openAIOAuthAccountsLoaded.value = true
}

async function preloadOpenAIImageUsage(rows: ChannelMonitor[]) {
  const token = ++imageUsageRequestToken
  const openAIRows = rows.filter((row) => row.provider === 'openai')
  if (openAIRows.length === 0) {
    openAIImageUsageByMonitorId.value = {}
    imageUsageLoadingMap.value = {}
    return
  }

  openAIImageUsageByMonitorId.value = {}
  imageUsageLoadingMap.value = Object.fromEntries(openAIRows.map((row) => [row.id, true]))

  try {
    await ensureOpenAIOAuthAccountsLoaded(true)
  } catch {
    if (token === imageUsageRequestToken) {
      openAIImageUsageByMonitorId.value = {}
      imageUsageLoadingMap.value = {}
    }
    return
  }

  if (token !== imageUsageRequestToken) return

  const matchedRows = openAIRows
    .map((row) => ({
      row,
      account: openAIOAuthAccountMap.value.get(normalizeMonitorName(row.name)) || null,
    }))
    .filter((item): item is { row: ChannelMonitor; account: Account } => item.account !== null)

  const usageEntries = await Promise.all(
    matchedRows.map(async ({ row, account }) => {
      try {
        const usage = await adminAPI.accounts.getUsage(account.id)
        return [row.id, buildOpenAIImageUsageRows(usage)] as const
      } catch {
        return [row.id, []] as const
      }
    })
  )

  if (token !== imageUsageRequestToken) return

  openAIImageUsageByMonitorId.value = Object.fromEntries(usageEntries)
  imageUsageLoadingMap.value = {}
}

watch(
  groupedMonitors,
  (groups) => {
    const nextState: Record<string, boolean> = {}
    for (const group of groups) {
      nextState[group.provider] = collapsedProviders.value[group.provider] ?? false
    }
    collapsedProviders.value = nextState
  },
  { immediate: true }
)

onMounted(reload)
onUnmounted(() => {
  if (searchTimeout) clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>
