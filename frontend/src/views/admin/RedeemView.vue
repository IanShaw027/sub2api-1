<template>
  <AppLayout>
    <div class="list-page">
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
          <Button
            variant="secondary"
            data-test="batch-update-open"
            :disabled="selectedCount === 0 || batchUpdating"
            @click="openBatchUpdateDialog"
          >
            <Icon name="edit" size="sm" />
            {{ t('admin.redeem.batchUpdate') }}
          </Button>
          <Button class="redeem-create-desktop" @click="showGenerateDialog = true">
            <Icon name="plus" size="sm" />
            {{ t('admin.redeem.generateCodes') }}
          </Button>
        </template>
      </PageHeader>

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
      <div v-if="selectedCount > 0" class="selection-bar">
        <span class="selection-bar-text">
          {{ t('admin.redeem.selectedCount', { count: selectedCount }) }}
        </span>
        <div class="selection-bar-actions">
          <button type="button" class="btn btn-ghost btn-xs" @click="clearSelectedCodes">
            {{ t('admin.redeem.clearSelection') }}
          </button>
          <button type="button" class="btn btn-primary btn-xs" @click="openBatchUpdateDialog">
            {{ t('admin.redeem.batchUpdate') }}
          </button>
        </div>
      </div>

      <section class="table-card">
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
              class="row-checkbox"
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
              class="row-checkbox"
              :aria-label="row.code"
              :checked="selectedCodeIds.has(row.id)"
              @click.stop
              @change="toggleSelectRow(row.id, $event)"
            />
          </template>

          <template #cell-code="{ value, row }">
            <div class="cell-code">
              <div class="cell-primary">
                <span class="cell-code-value">{{ value }}</span>
                <span class="cell-primary-meta">#{{ row.id }} · {{ t('admin.redeem.types.' + row.type) }}</span>
              </div>
              <button
                type="button"
                class="icon-btn"
                :class="{ 'is-copied': copiedCode === value }"
                :title="copiedCode === value ? t('admin.redeem.copied') : t('keys.copyToClipboard')"
                :aria-label="t('keys.copyToClipboard')"
                @click="copyToClipboard(value)"
              >
                <Icon :name="copiedCode === value ? 'check' : 'copy'" size="sm" :stroke-width="2" />
              </button>
            </div>
          </template>

          <template #cell-type="{ value }">
            <span class="tag" :class="typeTagClass(value)">{{ t('admin.redeem.types.' + value) }}</span>
          </template>

          <template #cell-value="{ value, row }">
            <div class="cell-amount">
              <span class="cell-amount-value">
                <template v-if="row.type === 'balance'">${{ value.toFixed(2) }}</template>
                <template v-else-if="row.type === 'subscription'">
                  {{ row.validity_days || 30 }} {{ t('admin.redeem.days') }}
                </template>
                <template v-else>{{ value }}</template>
              </span>
              <span v-if="row.type === 'subscription' && row.group" class="cell-amount-meta">
                {{ row.group.name }}
              </span>
            </div>
          </template>

          <template #cell-status="{ value }">
            <StatusBadge dot :tone="statusTone(value)" :label="t('admin.redeem.status.' + value)" />
          </template>

          <template #cell-used_by="{ value, row }">
            <div v-if="row.user?.email || value" class="cell-primary">
              <span class="cell-primary-name">{{ row.user?.email || t('admin.redeem.userPrefix', { id: value }) }}</span>
              <span v-if="value" class="cell-primary-meta">#{{ value }}</span>
            </div>
            <span v-else class="cell-muted">-</span>
          </template>

          <template #cell-used_at="{ value }">
            <span :class="value ? 'cell-time' : 'cell-muted'">{{ value ? formatDateTime(value) : '-' }}</span>
          </template>

          <template #cell-expires_at="{ value, row }">
            <span
              :class="[
                value ? 'cell-time' : 'cell-muted',
                row.status === 'expired' ? 'cell-time-danger' : ''
              ]"
            >
              {{ value ? formatDateTime(value) : t('admin.redeem.neverExpires') }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="row-actions">
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
              <span v-else class="cell-muted">-</span>
            </div>
          </template>
        </DataTable>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>

      <div v-if="filters.status === 'unused'" class="bulk-danger-row">
        <button type="button" class="btn btn-secondary btn-sm bulk-danger-btn" @click="showDeleteUnusedDialog = true">
          <Icon name="trash" size="sm" />
          {{ t('admin.redeem.deleteAllUnused') }}
        </button>
      </div>
    </div>

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
    <UiModal
      :open="showGenerateDialog"
      :title="t('admin.redeem.generateCodesTitle')"
      width="md"
      :close-label="t('common.cancel')"
      @close="showGenerateDialog = false"
    >
      <form id="generate-redeem-form" class="modal-form" @submit.prevent="handleGenerateCodes">
        <div class="form-field">
          <label class="input-label">{{ t('admin.redeem.codeType') }}</label>
          <Select v-model="generateForm.type" :options="typeOptions" />
        </div>

        <div v-if="generateForm.type !== 'subscription' && generateForm.type !== 'invitation'" class="form-field">
          <label class="input-label">
            {{ generateForm.type === 'balance' ? t('admin.redeem.amount') : t('admin.redeem.columns.value') }}
          </label>
          <input
            v-model.number="generateForm.value"
            type="number"
            :step="generateForm.type === 'balance' ? '0.01' : '1'"
            :min="generateForm.type === 'balance' ? '0.01' : '1'"
            required
            class="field"
          />
        </div>

        <div v-if="generateForm.type === 'invitation'" class="notice notice-info">
          {{ t('admin.redeem.invitationHint') }}
        </div>

        <template v-if="generateForm.type === 'subscription'">
          <div class="form-field">
            <label class="input-label">{{ t('admin.redeem.selectGroup') }}</label>
            <Select
              v-model="generateForm.group_id"
              :options="subscriptionGroupOptions"
              :placeholder="t('admin.redeem.selectGroupPlaceholder')"
            >
              <template #selected="{ option }">
                <GroupBadge
                  v-if="option"
                  :name="(option as unknown as GroupOption).label"
                  :platform="(option as unknown as GroupOption).platform"
                  :subscription-type="(option as unknown as GroupOption).subscriptionType"
                  :rate-multiplier="(option as unknown as GroupOption).rate"
                />
                <span v-else class="text-muted">{{ t('admin.redeem.selectGroupPlaceholder') }}</span>
              </template>
              <template #option="{ option, selected }">
                <GroupOptionItem
                  :name="(option as unknown as GroupOption).label"
                  :platform="(option as unknown as GroupOption).platform"
                  :subscription-type="(option as unknown as GroupOption).subscriptionType"
                  :rate-multiplier="(option as unknown as GroupOption).rate"
                  :description="(option as unknown as GroupOption).description"
                  :selected="selected"
                />
              </template>
            </Select>
          </div>
          <div class="form-field">
            <label class="input-label">{{ t('admin.redeem.validityDays') }}</label>
            <input v-model.number="generateForm.validity_days" type="number" min="1" max="365" required class="field" />
          </div>
        </template>

        <div class="form-field">
          <label class="input-label">{{ t('admin.redeem.codeExpiry') }}</label>
          <div class="expiry-grid">
            <button
              v-for="option in redeemCodeExpiryOptions"
              :key="option.value"
              type="button"
              class="expiry-option"
              :class="{ 'is-active': generateForm.expiry_option === option.value }"
              @click="generateForm.expiry_option = option.value"
            >
              {{ option.label }}
            </button>
          </div>
          <input
            v-if="generateForm.expiry_option === 'custom'"
            v-model.number="generateForm.custom_expiry_days"
            type="number"
            min="1"
            max="3650"
            required
            class="field expiry-custom"
            :placeholder="t('admin.redeem.customExpiryDays')"
          />
        </div>

        <div class="form-field">
          <label class="input-label">{{ t('admin.redeem.count') }}</label>
          <input v-model.number="generateForm.count" type="number" min="1" max="100" required class="field" />
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="showGenerateDialog = false">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" form="generate-redeem-form" :disabled="generating" class="btn btn-primary">
          {{ generating ? t('admin.redeem.generating') : t('admin.redeem.generate') }}
        </button>
      </template>
    </UiModal>

    <!-- Batch Update Dialog -->
    <UiModal
      :open="showBatchUpdateDialog"
      :title="t('admin.redeem.batchUpdateTitle')"
      width="md"
      :close-label="t('common.cancel')"
      @close="closeBatchUpdateDialog"
    >
      <p class="modal-lede">{{ t('admin.redeem.selectedCount', { count: selectedCount }) }}</p>

      <form id="batch-update-redeem-form" data-test="batch-update-form" class="modal-form" @submit.prevent="handleBatchUpdate">
        <div class="form-field">
          <label class="batch-toggle">
            <input
              data-test="batch-field-status"
              v-model="batchUpdateForm.update_status"
              type="checkbox"
              class="row-checkbox"
            />
            {{ t('admin.redeem.batchFields.status') }}
          </label>
          <Select
            v-if="batchUpdateForm.update_status"
            v-model="batchUpdateForm.status"
            data-test="batch-status-select"
            :options="batchStatusOptions"
          />
        </div>

        <div class="form-field">
          <label class="batch-toggle">
            <input v-model="batchUpdateForm.update_expires_at" type="checkbox" class="row-checkbox" />
            {{ t('admin.redeem.batchFields.expiresAt') }}
          </label>
          <template v-if="batchUpdateForm.update_expires_at">
            <Select v-model="batchUpdateForm.expires_mode" :options="batchExpiryModeOptions" />
            <input
              v-if="batchUpdateForm.expires_mode === 'custom'"
              v-model="batchUpdateForm.expires_at_local"
              type="datetime-local"
              class="field"
            />
            <p v-if="batchUpdateForm.expires_mode === 'custom'" class="input-hint">
              {{ t('admin.redeem.localTimeZoneHint', { timezone: browserTimeZone }) }}
            </p>
          </template>
        </div>

        <div class="form-field">
          <label class="batch-toggle">
            <input
              data-test="batch-field-notes"
              v-model="batchUpdateForm.update_notes"
              type="checkbox"
              class="row-checkbox"
            />
            {{ t('admin.redeem.batchFields.notes') }}
          </label>
          <textarea
            v-if="batchUpdateForm.update_notes"
            data-test="batch-notes-input"
            v-model="batchUpdateForm.notes"
            rows="3"
            class="field field-textarea"
            :placeholder="t('admin.redeem.batchNotesPlaceholder')"
          ></textarea>
        </div>

        <div class="form-field">
          <label class="batch-toggle">
            <input v-model="batchUpdateForm.update_group_id" type="checkbox" class="row-checkbox" />
            {{ t('admin.redeem.batchFields.group') }}
          </label>
          <Select
            v-if="batchUpdateForm.update_group_id"
            v-model="batchUpdateForm.group_id"
            :options="batchGroupOptions"
            :placeholder="t('admin.redeem.selectGroupPlaceholder')"
          />
        </div>
      </form>

      <template #footer>
        <button type="button" class="btn btn-secondary" @click="closeBatchUpdateDialog">
          {{ t('common.cancel') }}
        </button>
        <button
          data-test="batch-update-submit"
          type="submit"
          form="batch-update-redeem-form"
          :disabled="batchUpdating"
          class="btn btn-primary"
        >
          {{ batchUpdating ? t('common.submitting') : t('admin.redeem.batchUpdate') }}
        </button>
      </template>
    </UiModal>

    <!-- Generated Codes Result Dialog -->
    <UiModal
      :open="showResultDialog"
      :title="t('admin.redeem.generatedSuccessfully')"
      width="md"
      :close-label="t('common.close')"
      @close="closeResultDialog"
    >
      <p class="modal-lede">{{ t('admin.redeem.codesCreated', { count: generatedCodes.length }) }}</p>
      <textarea
        readonly
        :value="generatedCodesText"
        :style="{ height: textareaHeight }"
        class="code-block generated-codes"
      ></textarea>

      <template #footer>
        <button
          type="button"
          :class="['btn', copiedAll ? 'btn-success' : 'btn-secondary']"
          @click="copyGeneratedCodes"
        >
          <Icon :name="copiedAll ? 'check' : 'copy'" size="sm" :stroke-width="2" />
          {{ copiedAll ? t('admin.redeem.copied') : t('admin.redeem.copyAll') }}
        </button>
        <button type="button" class="btn btn-primary" @click="downloadGeneratedCodes">
          <Icon name="download" size="sm" :stroke-width="2" />
          {{ t('admin.redeem.download') }}
        </button>
      </template>
    </UiModal>

    <Fab class="redeem-fab" :label="t('admin.redeem.generateCodes')" @click="showGenerateDialog = true">
      <Icon name="plus" size="md" />
      {{ t('admin.redeem.generateCodes') }}
    </Fab>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useClipboard } from '@/composables/useClipboard'
import { useTableSelection } from '@/composables/useTableSelection'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { adminAPI } from '@/api/admin'
import {
  formatDateTime,
  getBrowserTimeZone,
  parseDateTimeLocalInput
} from '@/utils/format'
import type {
  RedeemCode,
  RedeemCodeType,
  Group,
  GroupPlatform,
  SubscriptionType,
  BatchUpdateRedeemCodeFields
} from '@/types'
import type { Column } from '@/components/common/types'
import type { StatusBadgeTone } from '@/components/ui/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import Fab from '@/components/ui/Fab.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiModal from '@/components/ui/UiModal.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GroupOptionItem from '@/components/common/GroupOptionItem.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const { copyToClipboard: clipboardCopy } = useClipboard()
const browserTimeZone = getBrowserTimeZone()

interface GroupOption {
  value: number
  label: string
  description: string | null
  platform: GroupPlatform
  subscriptionType: SubscriptionType
  rate: number
}

const showGenerateDialog = ref(false)
const showResultDialog = ref(false)
const generatedCodes = ref<RedeemCode[]>([])
const subscriptionGroups = ref<Group[]>([])

// 订阅类型分组选项
const subscriptionGroupOptions = computed(() => {
  return subscriptionGroups.value
    .filter((g) => g.subscription_type === 'subscription')
    .map((g) => ({
      value: g.id,
      label: g.name,
      description: g.description,
      platform: g.platform,
      subscriptionType: g.subscription_type,
      rate: g.rate_multiplier
    }))
})

const batchGroupOptions = computed(() => [
  { value: null, label: t('admin.redeem.clearGroup') },
  ...subscriptionGroupOptions.value
])

const generatedCodesText = computed(() => {
  return generatedCodes.value.map((code) => code.code).join('\n')
})

const textareaHeight = computed(() => {
  const lineCount = generatedCodes.value.length
  const lineHeight = 24 // approximate line height in px
  const padding = 24 // top + bottom padding
  const minHeight = 60
  const maxHeight = 240
  const calculatedHeight = Math.min(
    Math.max(lineCount * lineHeight + padding, minHeight),
    maxHeight
  )
  return `${calculatedHeight}px`
})

const copiedAll = ref(false)

const closeResultDialog = () => {
  showResultDialog.value = false
  generatedCodes.value = []
  copiedAll.value = false
}

const copyGeneratedCodes = async () => {
  const success = await clipboardCopy(generatedCodesText.value, t('admin.redeem.copied'))
  if (success) {
    copiedAll.value = true
    setTimeout(() => {
      copiedAll.value = false
    }, 2000)
  }
}

const downloadGeneratedCodes = () => {
  const blob = new Blob([generatedCodesText.value], { type: 'text/plain' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.txt`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
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

const TYPE_TAG_CLASSES: Record<string, string> = {
  balance: 'tag-success',
  concurrency: 'tag-accent',
  subscription: 'tag-warning',
  invitation: 'tag-accent'
}

const typeTagClass = (type: string): string => TYPE_TAG_CLASSES[type] ?? 'tag-accent'

const STATUS_TONES: Record<string, StatusBadgeTone> = {
  unused: 'success',
  used: 'muted',
  expired: 'danger',
  disabled: 'danger'
}

const statusTone = (status: string): StatusBadgeTone => STATUS_TONES[status] ?? 'muted'

const typeOptions = computed(() => [
  { value: 'balance', label: t('admin.redeem.balance') },
  { value: 'concurrency', label: t('admin.redeem.concurrency') },
  { value: 'subscription', label: t('admin.redeem.subscription') },
  { value: 'invitation', label: t('admin.redeem.invitation') }
])

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

const batchStatusOptions = computed(() => [
  { value: 'unused', label: t('admin.redeem.status.unused') },
  { value: 'disabled', label: t('admin.redeem.status.disabled') }
])

const batchExpiryModeOptions = computed(() => [
  { value: 'clear', label: t('admin.redeem.neverExpires') },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

const codes = ref<RedeemCode[]>([])
const loading = ref(false)
const generating = ref(false)
const batchUpdating = ref(false)
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

const batchUpdateForm = reactive({
  update_status: false,
  status: 'disabled' as 'unused' | 'disabled',
  update_expires_at: false,
  expires_mode: 'clear' as 'clear' | 'custom',
  expires_at_local: '',
  update_notes: false,
  notes: '',
  update_group_id: false,
  group_id: null as number | null
})

type RedeemCodeExpiryOption = 'never' | '1' | '3' | '7' | 'custom'

const redeemCodeExpiryOptions = computed<{ value: RedeemCodeExpiryOption; label: string }[]>(() => [
  { value: 'never', label: t('admin.redeem.neverExpires') },
  { value: '1', label: t('admin.redeem.expiryPresetDays', { days: 1 }) },
  { value: '3', label: t('admin.redeem.expiryPresetDays', { days: 3 }) },
  { value: '7', label: t('admin.redeem.expiryPresetDays', { days: 7 }) },
  { value: 'custom', label: t('admin.redeem.customExpiry') }
])

const generateForm = reactive({
  type: 'balance' as RedeemCodeType,
  value: 10,
  count: 1,
  group_id: null as number | null,
  validity_days: 30,
  expiry_option: 'never' as RedeemCodeExpiryOption,
  custom_expiry_days: 7
})

// 监听类型变化，邀请码类型时自动设置 value 为 0
watch(
  () => generateForm.type,
  (newType) => {
    if (newType === 'invitation') {
      generateForm.value = 0
    } else if (generateForm.value === 0) {
      generateForm.value = 10
    }
  }
)

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

const getRedeemCodeExpiresInDays = () => {
  if (generateForm.expiry_option === 'never') {
    return undefined
  }
  if (generateForm.expiry_option === 'custom') {
    if (
      !Number.isFinite(generateForm.custom_expiry_days) ||
      generateForm.custom_expiry_days < 1
    ) {
      return null
    }
    return Math.floor(generateForm.custom_expiry_days)
  }
  return Number(generateForm.expiry_option)
}

const toDatetimeLocalInputValue = (date: Date) => {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`
}

const resetBatchUpdateForm = () => {
  batchUpdateForm.update_status = false
  batchUpdateForm.status = 'disabled'
  batchUpdateForm.update_expires_at = false
  batchUpdateForm.expires_mode = 'clear'
  batchUpdateForm.expires_at_local = toDatetimeLocalInputValue(
    new Date(Date.now() + 24 * 60 * 60 * 1000)
  )
  batchUpdateForm.update_notes = false
  batchUpdateForm.notes = ''
  batchUpdateForm.update_group_id = false
  batchUpdateForm.group_id = null
}

const openBatchUpdateDialog = () => {
  if (selectedCount.value === 0) {
    appStore.showInfo(t('admin.redeem.selectCodesFirst'))
    return
  }
  resetBatchUpdateForm()
  showBatchUpdateDialog.value = true
}

const closeBatchUpdateDialog = () => {
  showBatchUpdateDialog.value = false
}

const buildBatchUpdateFields = (): BatchUpdateRedeemCodeFields | null => {
  const fields: BatchUpdateRedeemCodeFields = {}

  if (batchUpdateForm.update_status) {
    fields.status = batchUpdateForm.status
  }
  if (batchUpdateForm.update_expires_at) {
    if (batchUpdateForm.expires_mode === 'clear') {
      fields.expires_at = null
    } else {
      const expiresAt = parseDateTimeLocalInput(batchUpdateForm.expires_at_local)
      if (expiresAt === null) {
        appStore.showError(t('admin.redeem.expiryDateRequired'))
        return null
      }
      fields.expires_at = new Date(expiresAt * 1000).toISOString()
    }
  }
  if (batchUpdateForm.update_notes) {
    fields.notes = batchUpdateForm.notes
  }
  if (batchUpdateForm.update_group_id) {
    fields.group_id =
      batchUpdateForm.group_id == null ? null : Number(batchUpdateForm.group_id)
  }

  return Object.keys(fields).length > 0 ? fields : null
}

const handleGenerateCodes = async () => {
  // 订阅类型必须选择分组
  if (generateForm.type === 'subscription' && !generateForm.group_id) {
    appStore.showError(t('admin.redeem.groupRequired'))
    return
  }

  const expiresInDays = getRedeemCodeExpiresInDays()
  if (expiresInDays === null) {
    appStore.showError(t('admin.redeem.expiryDaysRequired'))
    return
  }

  generating.value = true
  try {
    const result = await adminAPI.redeem.generate(
      generateForm.count,
      generateForm.type,
      generateForm.value,
      generateForm.type === 'subscription' ? generateForm.group_id : undefined,
      generateForm.type === 'subscription' ? generateForm.validity_days : undefined,
      expiresInDays
    )
    showGenerateDialog.value = false
    generatedCodes.value = result
    showResultDialog.value = true
    // 重置表单
    generateForm.group_id = null
    generateForm.validity_days = 30
    generateForm.expiry_option = 'never'
    generateForm.custom_expiry_days = 7
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToGenerate'))
    console.error('Error generating codes:', error)
  } finally {
    generating.value = false
  }
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

const handleBatchUpdate = async () => {
  const ids = Array.from(selectedCodeIds.value)
  if (ids.length === 0) {
    appStore.showInfo(t('admin.redeem.selectCodesFirst'))
    return
  }

  const hasSelectedFields =
    batchUpdateForm.update_status ||
    batchUpdateForm.update_expires_at ||
    batchUpdateForm.update_notes ||
    batchUpdateForm.update_group_id
  if (!hasSelectedFields) {
    appStore.showError(t('admin.redeem.noBatchFieldsSelected'))
    return
  }

  const fields = buildBatchUpdateFields()
  if (!fields) {
    return
  }

  batchUpdating.value = true
  try {
    const result = await adminAPI.redeem.batchUpdate(ids, fields)
    appStore.showSuccess(t('admin.redeem.batchUpdateSuccess', { count: result.updated }))
    showBatchUpdateDialog.value = false
    clearSelectedCodes()
    loadCodes()
  } catch (error: any) {
    appStore.showError(error.response?.data?.detail || t('admin.redeem.failedToBatchUpdate'))
    console.error('Error batch updating codes:', error)
  } finally {
    batchUpdating.value = false
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
.summary-row {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-search {
  width: 260px;
  flex: none;
}

.filter-count {
  margin-left: auto;
  font-size: 12.5px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.filter-count b {
  color: var(--foreground);
}

.redeem-fab {
  display: none;
}

@media (min-width: 768px) and (max-width: 1023px) {
  .summary-row {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 767px) {
  .summary-row {
    grid-template-columns: 1fr;
  }

  .filter-search {
    width: 100%;
  }

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
