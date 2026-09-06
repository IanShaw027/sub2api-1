<template>
  <AppLayout>
    <PageHeader :title="t('admin.channels.title')" :description="t('admin.channels.description')">
      <template #actions>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh', 'Refresh')" @click="loadChannels">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <Button class="channels-create-desktop" @click="openCreateDialog">
          <Icon name="plus" size="md" />
          {{ t('admin.channels.createChannel', 'Create Channel') }}
        </Button>
      </template>
    </PageHeader>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <FilterBar
            :search-placeholder="t('admin.channels.searchChannels', 'Search channels...')"
            :filter-label="t('common.filter')"

          >
            <template #search>
              <div class="relative w-full">
                <Icon name="search" size="md" class="absolute left-3 top-1/2 -translate-y-1/2 text-muted" />
                <input
                  v-model="searchQuery"
                  type="text"
                  :placeholder="t('admin.channels.searchChannels', 'Search channels...')"
                  class="input pl-10"
                  @input="handleSearch"
                />
              </div>
            </template>
            <template #filters>
              <Select
                v-model="filters.status"
                :options="statusFilterOptions"
                :placeholder="t('admin.channels.allStatus', 'All Status')"
                class="w-40"
                @change="loadChannels"
              />
            </template>
          </FilterBar>
          <ChipScroller
            :model-value="String(filters.status || '')"
            :chips="channelStatusChips"
            @update:model-value="onChannelStatusChip"
          />
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="channels"
          :loading="loading"
          :server-side-sort="true"
          default-sort-key="created_at"
          default-sort-order="desc"
          @sort="handleSort"
        >
          <template #cell-name="{ value }">
            <span class="font-medium text-foreground">{{ value }}</span>
          </template>

          <template #cell-description="{ value }">
            <span class="text-sm text-muted">{{ value || '-' }}</span>
          </template>

          <template #cell-status="{ row }">
            <Toggle
              :modelValue="row.status === 'active'"
              @update:modelValue="toggleChannelStatus(row)"
            />
          </template>

          <template #cell-group_count="{ row }">
            <span class="tag">
              {{ (row.group_ids || []).length }}
              {{ t('admin.channels.groupsUnit', 'groups') }}
            </span>
          </template>

          <template #cell-pricing_count="{ row }">
            <span class="tag">
              {{ (row.model_pricing || []).length }}
              {{ t('admin.channels.pricingUnit', 'pricing rules') }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-muted">
              {{ formatDate(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <ActionsCell
              :edit-label="t('common.edit', 'Edit')"
              :items="[{ label: t('common.delete', 'Delete'), icon: 'trash', danger: true, onClick: () => handleDelete(row) }]"
              @edit="openEditDialog(row)"
            />
          </template>

          <template #empty>
            <EmptyState
              :title="t('admin.channels.noChannelsYet', 'No Channels Yet')"
              :description="t('admin.channels.createFirstChannel', 'Create your first channel to manage model pricing')"
              :action-text="t('admin.channels.createChannel', 'Create Channel')"
              @action="openCreateDialog"
            />
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
    <Fab class="channels-fab" :label="t('admin.channels.createChannel', 'Create Channel')" @click="openCreateDialog">
      <Icon name="plus" size="md" />
      {{ t('admin.channels.createChannel', 'Create Channel') }}
    </Fab>

    <!-- Create/Edit Dialog -->
    <ChannelFormDialog
      :show="showDialog"
      :editing-channel="editingChannel"
      @close="closeDialog"
      @saved="loadChannels"
    />

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteDialog"
      :title="t('admin.channels.deleteChannel', 'Delete Channel')"
      :message="deleteConfirmMessage"
      :confirm-text="t('common.delete', 'Delete')"
      :cancel-text="t('common.cancel', 'Cancel')"
      :danger="true"
      @confirm="confirmDelete"
      @cancel="showDeleteDialog = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { adminAPI } from '@/api/admin'
import type { Channel } from '@/api/admin/channels'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import ChipScroller from '@/components/ui/ChipScroller.vue'
import Fab from '@/components/ui/Fab.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import ActionsCell from '@/components/common/cells/ActionsCell.vue'
import ChannelFormDialog from '@/components/admin/channel/ChannelFormDialog.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const appStore = useAppStore()

// ── Table columns ──
const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.channels.columns.name', 'Name'), sortable: true },
  { key: 'description', label: t('admin.channels.columns.description', 'Description'), sortable: false },
  { key: 'status', label: t('admin.channels.columns.status', 'Status'), sortable: true },
  { key: 'group_count', label: t('admin.channels.columns.groups', 'Groups'), sortable: false },
  { key: 'pricing_count', label: t('admin.channels.columns.pricing', 'Pricing'), sortable: false },
  { key: 'created_at', label: t('admin.channels.columns.createdAt', 'Created'), sortable: true },
  { key: 'actions', label: t('admin.channels.columns.actions', 'Actions'), sortable: false }
])

const statusFilterOptions = computed(() => [
  { value: '', label: t('admin.channels.allStatus', 'All Status') },
  { value: 'active', label: t('admin.channels.statusActive', 'Active') },
  { value: 'disabled', label: t('admin.channels.statusDisabled', 'Disabled') }
])

const channelStatusChips = computed(() =>
  statusFilterOptions.value.map((opt) => ({ value: String(opt.value), label: opt.label }))
)

const onChannelStatusChip = (value: string) => {
  filters.status = value
  loadChannels()
}

// ── State ──
const channels = ref<Channel[]>([])
const loading = ref(false)
const searchQuery = ref('')
const filters = reactive({ status: '' })
const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0
})
const sortState = reactive({
  sort_by: 'created_at',
  sort_order: 'desc' as 'asc' | 'desc'
})

// Dialog state
const showDialog = ref(false)
const editingChannel = ref<Channel | null>(null)
const showDeleteDialog = ref(false)
const deletingChannel = ref<Channel | null>(null)

let abortController: AbortController | null = null

// ── Helpers ──
function formatDate(value: string): string {
  if (!value) return '-'
  return new Date(value).toLocaleDateString()
}

const deleteConfirmMessage = computed(() => {
  const name = deletingChannel.value?.name || ''
  return t(
    'admin.channels.deleteConfirm',
    { name },
    `Are you sure you want to delete channel "${name}"? This action cannot be undone.`
  )
})

// ── Load data ──
async function loadChannels() {
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true

  try {
    const response = await adminAPI.channels.list(pagination.page, pagination.page_size, {
      status: filters.status || undefined,
      search: searchQuery.value || undefined,
      sort_by: sortState.sort_by,
      sort_order: sortState.sort_order
    }, { signal: ctrl.signal })

    if (ctrl.signal.aborted || abortController !== ctrl) return
    channels.value = response.items || []
    pagination.total = response.total
  } catch (error: unknown) {
    const e = error as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(error, t('admin.channels.loadError', 'Failed to load channels')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

let searchTimeout: ReturnType<typeof setTimeout>
function handleSearch() {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(() => {
    pagination.page = 1
    loadChannels()
  }, 300)
}

function handlePageChange(page: number) {
  pagination.page = page
  loadChannels()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadChannels()
}

function handleSort(key: string, order: 'asc' | 'desc') {
  sortState.sort_by = key
  sortState.sort_order = order
  pagination.page = 1
  loadChannels()
}

// ── Dialog ──
function openCreateDialog() {
  editingChannel.value = null
  showDialog.value = true
}

function openEditDialog(channel: Channel) {
  editingChannel.value = channel
  showDialog.value = true
}

function closeDialog() {
  showDialog.value = false
  editingChannel.value = null
}

// ── Toggle status ──
async function toggleChannelStatus(channel: Channel) {
  const newStatus = channel.status === 'active' ? 'disabled' : 'active'
  try {
    await adminAPI.channels.update(channel.id, { status: newStatus })
    if (filters.status && filters.status !== newStatus) {
      // Item no longer matches the active filter — reload list
      await loadChannels()
    } else {
      channel.status = newStatus
    }
  } catch (error) {
    appStore.showError(t('admin.channels.updateError', 'Failed to update channel'))
    console.error('Error toggling channel status:', error)
  }
}

// ── Delete ──
function handleDelete(channel: Channel) {
  deletingChannel.value = channel
  showDeleteDialog.value = true
}

async function confirmDelete() {
  if (!deletingChannel.value) return

  try {
    await adminAPI.channels.remove(deletingChannel.value.id)
    appStore.showSuccess(t('admin.channels.deleteSuccess', 'Channel deleted'))
    showDeleteDialog.value = false
    deletingChannel.value = null
    loadChannels()
  } catch (error: unknown) {
    appStore.showError(extractApiErrorMessage(error, t('admin.channels.deleteError', 'Failed to delete channel')))
  }
}

// ── Lifecycle ──
onMounted(() => {
  loadChannels()
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  abortController?.abort()
})
</script>

<style scoped>
.channels-mobile-filters {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.channels-fab {
  display: none;
}
@media (max-width: 767px) {
  .channels-create-desktop { display: none; }
  .channels-fab { display: inline-flex; }
}
</style>
