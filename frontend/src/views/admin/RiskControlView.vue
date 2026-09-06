<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-16">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-accent"></div>
      </div>

      <template v-else>
        <PageHeader :title="t('admin.riskControl.title')" :description="t('admin.riskControl.description')">
          <template #actions>
            <Button variant="secondary" :disabled="statusLoading" @click="loadStatus(false)">
              <Icon name="refresh" size="sm" :class="statusLoading ? 'animate-spin' : ''" />
              {{ t('admin.riskControl.refreshStatus') }}
            </Button>
            <Button @click="openSettings">
              <Icon name="cog" size="sm" />
              {{ t('admin.riskControl.openSettings') }}
            </Button>
          </template>
        </PageHeader>

        <OverviewStatCards :items="overviewItems" />

        <div
          v-if="showPreBlockRuntimeCard"
          data-test="pre-block-runtime-cards"
          class="grid grid-cols-1 gap-6 xl:grid-cols-[minmax(0,520px)_minmax(0,1fr)]"
        >
          <GlassCard data-test="pre-block-sync-card" class="glass-card">
            <div class="flex flex-col gap-4 border-b border-line px-6 py-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-base font-semibold text-foreground">{{ t('admin.riskControl.preBlockSyncStatus') }}</h2>
                <p class="mt-1 text-sm text-muted">{{ t('admin.riskControl.preBlockSyncHint') }}</p>
              </div>
              <span class="inline-flex w-fit items-center rounded-full bg-surface-2 px-2.5 py-1 text-xs font-medium text-muted">
                {{ modeLabel(status?.mode ?? configForm.mode) }}
              </span>
            </div>

            <div class="p-6">
              <div data-test="pre-block-metric-grid" class="grid grid-cols-2 gap-3 md:grid-cols-3">
                <div
                  v-for="item in preBlockMetricItems"
                  :key="item.key"
                  class="rounded-lg p-4"
                  :class="item.class"
                >
                  <p class="text-xs text-muted">{{ item.label }}</p>
                  <p class="mt-2 truncate text-2xl font-semibold leading-8" :class="item.valueClass">{{ item.value }}</p>
                  <p v-if="item.meta" class="mt-1 truncate text-xs text-muted">{{ item.meta }}</p>
                </div>
              </div>
            </div>
          </GlassCard>

          <GlassCard data-test="pre-block-api-key-load-card" class="glass-card">
            <div class="flex flex-col gap-4 border-b border-line px-6 py-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h2 class="text-base font-semibold text-foreground">{{ t('admin.riskControl.preBlockAPIKeyLoad') }}</h2>
                <p class="mt-1 text-sm text-muted">
                  {{ t('admin.riskControl.preBlockAPIKeyLoadHint') }}
                </p>
              </div>
              <span class="inline-flex w-fit items-center rounded-full bg-surface-2 px-2.5 py-1 text-xs font-medium text-muted">
                {{ preBlockAPIKeyLoadSummaryText }}
              </span>
            </div>

            <div class="p-6">
              <div
                v-if="preBlockAPIKeyLoads.length > 0"
                data-test="pre-block-api-key-load-list"
                class="max-h-[280px] space-y-3 overflow-y-auto pr-1"
              >
                <div
                  v-for="item in preBlockAPIKeyLoads"
                  :key="item.key_hash || item.index"
                  class="rounded-lg bg-surface-2 p-3"
                >
                  <div class="flex min-w-0 flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <div class="min-w-0">
                      <div class="flex min-w-0 items-center gap-2">
                        <span class="font-mono text-sm font-semibold text-foreground">#{{ item.index + 1 }}</span>
                        <span class="truncate font-mono text-sm text-foreground">{{ item.masked || '-' }}</span>
                        <span class="h-2 w-2 flex-shrink-0 rounded-full" :class="apiKeyStatusDotClass(item.status)"></span>
                      </div>
                      <p class="mt-1 text-xs text-muted">
                        {{ t('admin.riskControl.preBlockAPIKeyTotals', { total: formatNumber(item.total), success: formatNumber(item.success), errors: formatNumber(item.errors) }) }}
                      </p>
                    </div>
                    <div class="grid grid-cols-4 gap-2 text-right text-xs text-muted sm:min-w-[280px]">
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyActiveShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-accent ">{{ formatNumber(item.active) }}</p>
                      </div>
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyTotalShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-foreground">{{ formatNumber(item.total) }}</p>
                      </div>
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyAvgShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-foreground">{{ formatNumber(item.avg_latency_ms) }} ms</p>
                      </div>
                      <div>
                        <p>{{ t('admin.riskControl.preBlockKeyLastShort') }}</p>
                        <p class="mt-1 text-sm font-semibold text-foreground">{{ formatNumber(item.last_latency_ms) }} ms</p>
                      </div>
                    </div>
                  </div>
                  <div class="mt-3 h-1.5 overflow-hidden rounded-full bg-surface">
                    <div class="h-full rounded-full bg-accent-500" :style="{ width: preBlockAPIKeyLoadWidth(item.total) }"></div>
                  </div>
                </div>
              </div>
              <p v-else class="rounded-lg bg-surface-2 p-4 text-sm text-muted">
                {{ t('admin.riskControl.preBlockAPIKeyLoadEmpty') }}
              </p>
            </div>
          </GlassCard>
        </div>

        <GlassCard v-if="showWorkerRuntimeCard">
          <div class="flex flex-col gap-4 border-b border-line px-6 py-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h2 class="text-base font-semibold text-foreground">{{ t('admin.riskControl.workerStatus') }}</h2>
              <p class="mt-1 text-sm text-muted">{{ t('admin.riskControl.workerStatusHint') }}</p>
            </div>
            <div class="flex flex-wrap items-center gap-2 text-sm text-muted">
              <span>{{ t('admin.riskControl.autoRefresh') }}</span>
              <span v-if="status?.last_cleanup_at">
                {{ t('admin.riskControl.lastCleanup', { time: formatDateTime(status.last_cleanup_at) }) }}
              </span>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-6 p-6 xl:grid-cols-[minmax(0,360px)_1fr]">
            <div class="space-y-4">
              <div class="rounded-lg border border-line p-4">
                <div class="flex items-center justify-between gap-3">
                  <div>
                    <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.queueUsage') }}</p>
                    <p class="mt-1 text-xs text-muted">
                      {{ formatNumber(status?.queue_length ?? 0) }} / {{ formatNumber(status?.queue_size ?? configForm.queue_size) }}
                    </p>
                  </div>
                  <span class="text-sm font-semibold text-foreground">{{ queueUsagePercent }}</span>
                </div>
                <div class="mt-4 h-2 overflow-hidden rounded-full bg-surface-2">
                  <div class="h-full rounded-full bg-accent transition-all duration-300" :style="queueUsageStyle"></div>
                </div>
              </div>

              <div class="grid grid-cols-2 gap-3">
                <div class="rounded-lg bg-surface-2 p-4">
                  <p class="text-xs text-muted">{{ t('admin.riskControl.activeWorkers') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-foreground">{{ status?.active_workers ?? 0 }}</p>
                </div>
                <div class="rounded-lg bg-[color-mix(in_oklch,var(--success)_16%,transparent)] p-4 ">
                  <p class="text-xs text-muted">{{ t('admin.riskControl.idleWorkers') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-success-text ">{{ status?.idle_workers ?? configForm.worker_count }}</p>
                </div>
                <div class="rounded-lg bg-surface-2 p-4">
                  <p class="text-xs text-muted">{{ t('admin.riskControl.processed') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-foreground">{{ formatNumber(status?.processed ?? 0) }}</p>
                </div>
                <div class="rounded-lg bg-surface-2 p-4">
                  <p class="text-xs text-muted">{{ t('admin.riskControl.droppedErrors') }}</p>
                  <p class="mt-2 text-2xl font-semibold text-foreground">{{ formatNumber((status?.dropped ?? 0) + (status?.errors ?? 0)) }}</p>
                </div>
              </div>
            </div>

            <div>
              <div class="mb-3 flex items-center justify-between gap-3">
                <div>
                  <p class="text-sm font-medium text-foreground">{{ t('admin.riskControl.workerPool') }}</p>
                  <p class="mt-1 text-xs text-muted">
                    {{ t('admin.riskControl.workerPoolMeta', { active: status?.active_workers ?? 0, idle: status?.idle_workers ?? configForm.worker_count, total: status?.worker_count ?? configForm.worker_count }) }}
                  </p>
                </div>
                <span class="inline-flex items-center rounded-full bg-surface-2 px-2.5 py-1 text-xs font-medium text-muted">
                  {{ modeLabel(status?.mode ?? configForm.mode) }}
                </span>
              </div>
              <div class="grid grid-cols-2 gap-2 sm:grid-cols-4 md:grid-cols-6 xl:grid-cols-8 2xl:grid-cols-10">
                <div
                  v-for="worker in workerSlots"
                  :key="worker.id"
                  class="flex h-12 items-center justify-between rounded-lg border px-3 transition-colors"
                  :class="workerSlotClass(worker.state)"
                  :title="worker.label"
                >
                  <span class="text-sm font-semibold">#{{ worker.id }}</span>
                  <span class="h-2.5 w-2.5 rounded-full" :class="workerDotClass(worker.state)"></span>
                </div>
              </div>
            </div>
          </div>
        </GlassCard>

        <RiskRecordsTable
          v-model:filters="filters"
          :logs="logs"
          :logs-loading="logsLoading"
          :pagination="pagination"
          :result-options="resultOptions"
          :group-filter-options="groupFilterOptions"
          :endpoint-options="endpointOptions"
          :model-filter-summary="modelFilterSummary"
          :model-filter-tooltip="modelFilterTooltip"
          :unbanningUserID="unbanningUserID"
          @refresh="loadLogs"
          @reload-first-page="reloadLogsFromFirstPage"
          @unban="unbanUser"
          @open-input-detail="openInputDetail"
          @update:page="onPageChange"
          @update:pageSize="onPageSizeChange"
        />

        <RiskControlSettingsModal
          v-model:open="settingsOpen"
          v-model:flagged-hash-input="flaggedHashInput"
          v-model:pending-delete-api-key-hashes="pendingDeleteApiKeyHashes"
          :config-form="configForm"
          :proxies="proxies"
          :groups="groups"
          :status="status"
          :load-status="loadStatus"
          :load-logs="loadLogs"
          :apply-config="applyConfig"
          :hash-action-loading="hashActionLoading"
          :is-flagged-hash-input-valid="isFlaggedHashInputValid"
          @delete-flagged-hash="deleteFlaggedHash"
          @clear-flagged-hashes="clearFlaggedHashes"
        />

        <InputDetailModal :row="inputDetailRow" @close="closeInputDetail" />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import Icon from '@/components/icons/Icon.vue'
import OverviewStatCards from '@/components/admin/risk-control/OverviewStatCards.vue'
import RiskRecordsTable from '@/components/admin/risk-control/RiskRecordsTable.vue'
import RiskControlSettingsModal from '@/components/admin/risk-control/RiskControlSettingsModal.vue'
import InputDetailModal from '@/components/admin/risk-control/InputDetailModal.vue'
import type { ContentModerationLog, ModerationMode } from '@/api/admin/riskControl'
import { useRiskControlData } from '@/components/admin/risk-control/useRiskControlData'
import { useRiskControlOverview } from '@/components/admin/risk-control/useRiskControlOverview'
import { apiKeyStatusDotClass, formatDateTime, formatNumber, moderationModeLabel, workerDotClass, workerSlotClass } from '@/components/admin/risk-control/riskControlUtils'

const { t } = useI18n()

const settingsOpen = ref(false)
const inputDetailRow = ref<ContentModerationLog | null>(null)

function modeLabel(mode: ModerationMode): string {
  return moderationModeLabel(t, mode)
}

const {
  loading,
  statusLoading,
  logsLoading,
  hashActionLoading,
  unbanningUserID,
  flaggedHashInput,
  pendingDeleteApiKeyHashes,
  apiKeyHealthSummary,
  selectedGroupCount,
  groups,
  proxies,
  logs,
  status,
  configForm,
  pagination,
  filters,
  resultOptions,
  endpointOptions,
  groupFilterOptions,
  modelFilterSummary,
  modelFilterTooltip,
  applyConfig,
  loadStatus,
  loadLogs,
  reloadLogsFromFirstPage,
  onPageChange,
  onPageSizeChange,
  unbanUser,
  isFlaggedHashInputValid,
  deleteFlaggedHash,
  clearFlaggedHashes,
} = useRiskControlData()

const {
  overviewItems,
  showPreBlockRuntimeCard,
  showWorkerRuntimeCard,
  preBlockMetricItems,
  preBlockAPIKeyLoads,
  preBlockAPIKeyLoadSummaryText,
  preBlockAPIKeyLoadWidth,
  queueUsagePercent,
  queueUsageStyle,
  workerSlots,
} = useRiskControlOverview(configForm, status, pagination, {
  apiKeyHealthSummary,
  modelFilterSummary,
  selectedGroupCount,
  modeLabel,
})

function openSettings() {
  settingsOpen.value = true
}

function openInputDetail(row: ContentModerationLog) {
  inputDetailRow.value = row
}

function closeInputDetail() {
  inputDetailRow.value = null
}
</script>
