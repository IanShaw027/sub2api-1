<template>
  <AppLayout>
    <PageHeader :title="t('usage.title')" :description="t('usage.description')" />
    <div class="usage-page">
      <UsageStatsCards :stats="usageStats" :show-account-cost="false" :strike-standard-cost="true" />

      <div class="usage-page-section">
        <div class="usage-filter-row">
          <span class="usage-filter-label">{{ t('admin.dashboard.timeRange') }}</span>
          <DateRangePicker
            v-model:start-date="startDate"
            v-model:end-date="endDate"
            @change="onDateRangeChange"
          />
          <span class="usage-filter-sep" aria-hidden="true" />
          <span class="usage-filter-label">{{ t('admin.dashboard.granularity') }}</span>
          <SegmentedControl v-model="granularity" :options="granularityOptions" @update:model-value="loadChartData" />
        </div>

        <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
          <ModelDistributionChart
            v-model:metric="modelDistributionMetric"
            :model-stats="requestedModelStats"
            :loading="modelStatsLoading"
            :show-source-toggle="false"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :show-account-cost="false"
            :start-date="startDate"
            :end-date="endDate"
          />
          <GroupDistributionChart
            v-model:metric="groupDistributionMetric"
            :group-stats="groupStats"
            :loading="chartsLoading"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :show-account-cost="false"
            :start-date="startDate"
            :end-date="endDate"
          />
        </div>

        <div class="grid grid-cols-1 gap-3 lg:grid-cols-2">
          <EndpointDistributionChart
            v-model:source="endpointDistributionSource"
            v-model:metric="endpointDistributionMetric"
            :endpoint-stats="inboundEndpointStats"
            :upstream-endpoint-stats="upstreamEndpointStats"
            :endpoint-path-stats="endpointPathStats"
            :loading="endpointStatsLoading"
            :show-source-toggle="false"
            :show-metric-toggle="true"
            :enable-breakdown="false"
            :title="t('usage.endpointDistribution')"
            :start-date="startDate"
            :end-date="endDate"
          />
          <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
        </div>
      </div>

      <div class="usage-toolbar">
        <SegmentedControl
          v-if="errorViewEnabled"
          :model-value="activeTab"
          :options="tabOptions"
          @update:model-value="onTabChange"
        />
        <div class="usage-toolbar-actions">
          <Button variant="secondary" :disabled="activeTab === 'errors' ? errorLoading : loading" @click="refreshData">
            {{ t('common.refresh') }}
          </Button>
          <Button variant="secondary" @click="resetFilters">
            {{ t('common.reset') }}
          </Button>
          <div class="relative" ref="columnDropdownRef">
            <Button
              variant="secondary"
              data-testid="usage-column-settings"
              :title="t('admin.users.columnSettings')"
              @click="showColumnDropdown = !showColumnDropdown"
            >
              <Icon name="grid" size="sm" />
              <span class="hidden md:inline">{{ t('admin.users.columnSettings') }}</span>
            </Button>
            <div
              v-if="showColumnDropdown"
              class="dropdown usage-column-dropdown"
            >
              <button
                v-for="col in currentToggleableColumns"
                :key="col.key"
                type="button"
                :data-testid="`usage-column-toggle-${col.key}`"
                class="dropdown-item"
                @click="toggleCurrentColumn(col.key)"
              >
                <span>{{ col.label }}</span>
                <Icon v-if="isCurrentColumnVisible(col.key)" name="check" size="sm" class="text-accent" />
              </button>
            </div>
          </div>
          <Button v-if="activeTab !== 'errors'" :disabled="exporting" @click="exportToCSV">
            {{ exporting ? t('usage.exporting') : t('usage.exportCsv') }}
          </Button>
        </div>
      </div>

      <GlassCard padding="sm">
        <div class="flex flex-wrap items-end gap-4">
          <div v-if="activeTab === 'errors'" class="flex flex-1 flex-wrap items-end gap-4">
            <div class="w-full sm:w-auto sm:min-w-[220px]">
              <label class="input-label">{{ t('usage.errors.keyName') }}</label>
              <Select v-model="errorFilter.api_key_id" :options="errorKeyOptions" @change="applyErrorFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[220px]">
              <label class="input-label">{{ t('usage.errors.model') }}</label>
              <Select
                v-model="errorFilter.model"
                :options="errorModelOptions"
                searchable
                creatable
                clearable
                :placeholder="t('usage.errors.modelPlaceholder')"
                @change="applyErrorFilters"
              />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[200px]">
              <label class="input-label">{{ t('usage.errors.category') }}</label>
              <Select v-model="errorFilter.category" :options="errorCategoryOptions" @change="applyErrorFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[180px]">
              <label class="input-label">{{ t('usage.errors.status') }}</label>
              <Select v-model="errorFilter.status_code" :options="errorStatusOptions" @change="applyErrorFilters" />
            </div>
          </div>
          <div v-else class="flex flex-1 flex-wrap items-end gap-4">
            <div class="w-full sm:w-auto sm:min-w-[220px]">
              <label class="input-label">{{ t('usage.apiKeyFilter') }}</label>
              <Select v-model="filters.api_key_id" :options="apiKeyOptions" @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[220px]">
              <label class="input-label">{{ t('usage.model') }}</label>
              <Select v-model="filters.model" :options="modelOptions" searchable @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[200px]">
              <label class="input-label">{{ t('admin.usage.group') }}</label>
              <Select v-model="filters.group_id" :options="groupOptions" searchable @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[180px]">
              <label class="input-label">{{ t('usage.type') }}</label>
              <Select v-model="filters.request_type" :options="requestTypeOptions" @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[180px]">
              <label class="input-label">{{ t('usage.compactionFilter') }}</label>
              <Select v-model="filters.native_compaction_v2" :options="compactionOptions" @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[200px]">
              <label class="input-label">{{ t('admin.usage.billingType') }}</label>
              <Select v-model="filters.billing_type" :options="billingTypeOptions" @change="applyFilters" />
            </div>
            <div class="w-full sm:w-auto sm:min-w-[200px]">
              <label class="input-label">{{ t('admin.usage.billingMode') }}</label>
              <Select v-model="filters.billing_mode" :options="billingModeOptions" @change="applyFilters" />
            </div>
          </div>
        </div>
      </GlassCard>

      <div class="table-container usage-table-container">
        <div v-if="activeTab === 'usage'">
          <UsageTable
            :data="usageLogs"
            :loading="loading"
            :columns="visibleColumns"
            :server-side-sort="true"
            :show-account-billing="false"
            :show-upstream-endpoint="false"
            default-sort-key="created_at"
            default-sort-order="desc"
            @sort="handleSort"
          />

          <UiPagination
            v-if="pagination.total > 0"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>

        <UserErrorRequestsTable
          v-else-if="errorViewEnabled"
          :rows="errorRows"
          :total="errorTotal"
          :loading="errorLoading"
          :page="errorPage"
          :page-size="errorPageSize"
          :visible-column-keys="errVisibleColumnKeys"
          @sort="onErrorSort"
          @update:page="onErrorPage"
          @update:pageSize="onErrorPageSize"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import Button from '@/components/ui/Button.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import UsageStatsCards from '@/components/admin/usage/UsageStatsCards.vue'
import UsageTable from '@/components/admin/usage/UsageTable.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import GroupDistributionChart from '@/components/charts/GroupDistributionChart.vue'
import EndpointDistributionChart from '@/components/charts/EndpointDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import Icon from '@/components/icons/Icon.vue'
import UserErrorRequestsTable from '@/components/user/UserErrorRequestsTable.vue'
import { useUserUsage } from './usage/useUserUsage'

const { t } = useI18n()

const {
  usageStats,
  usageLogs,
  trendData,
  requestedModelStats,
  groupStats,
  inboundEndpointStats,
  upstreamEndpointStats,
  endpointPathStats,
  loading,
  chartsLoading,
  modelStatsLoading,
  endpointStatsLoading,
  exporting,
  errorLoading,
  errorRows,
  errorTotal,
  errorPage,
  errorPageSize,
  errorFilter,
  errorKeyOptions,
  errorModelOptions,
  errorCategoryOptions,
  errorStatusOptions,
  applyErrorFilters,
  onErrorSort,
  onErrorPage,
  onErrorPageSize,
  startDate,
  endDate,
  granularity,
  granularityOptions,
  onDateRangeChange,
  modelDistributionMetric,
  groupDistributionMetric,
  endpointDistributionMetric,
  endpointDistributionSource,
  activeTab,
  errorViewEnabled,
  tabOptions,
  onTabChange,
  filters,
  apiKeyOptions,
  modelOptions,
  groupOptions,
  requestTypeOptions,
  compactionOptions,
  billingTypeOptions,
  billingModeOptions,
  applyFilters,
  refreshData,
  resetFilters,
  pagination,
  handlePageChange,
  handlePageSizeChange,
  handleSort,
  exportToCSV,
  visibleColumns,
  currentToggleableColumns,
  isCurrentColumnVisible,
  toggleCurrentColumn,
  errVisibleColumnKeys,
  showColumnDropdown,
  columnDropdownRef,
  loadChartData,
} = useUserUsage()
</script>

<style scoped>
.usage-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.usage-page-section {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

/* Chart surface follows the dense glass-light reference while preserving the shared chart components. */
.usage-page-section :deep(.glass-card) {
  min-width: 0;
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  border-radius: var(--radius-card);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  box-shadow: var(--shadow);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
}

.usage-page-section :deep(canvas) {
  display: block;
  max-width: 100%;
}

.usage-page-section :deep(.h-48) {
  min-height: 192px;
}

@media (max-width: 640px) {
  .usage-page-section :deep(.glass-card) {
    border-radius: var(--radius-card);
  }
}

.usage-filter-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  min-height: 36px;
}

.usage-filter-label {
  font-size: var(--fs-13);
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.usage-filter-sep {
  width: 1px;
  height: 20px;
  margin: 0 4px;
  background: var(--border);
}

.usage-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.usage-toolbar-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.usage-column-dropdown {
  right: 0;
  top: calc(100% + 4px);
  width: 208px;
  max-height: 320px;
  overflow-y: auto;
}

/* ListPage table geometry (thead 42/43, rows 61, matches AccountsView/KeysView recipe) */
.usage-table-container :deep(.table-wrapper thead th) {
  height: 42px;
}

.usage-table-container :deep(.table-wrapper tbody td) {
  height: 61px;
}
</style>
