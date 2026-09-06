<template>
  <section class="glass-card monitor-toolbar-card">
    <div class="monitor-toolbar-status">
      <span class="relative flex h-2 w-2 shrink-0">
        <span
          class="relative inline-flex h-2 w-2 rounded-full"
          :class="loading || refreshing ? 'bg-muted' : 'bg-success'"
        ></span>
      </span>
      <span v-if="refreshing" class="inline-flex items-center gap-1 text-accent">
        <LoadingSpinner size="sm" />
        {{ t('channelMonitorV2.updating') }}
      </span>
      <span v-else-if="dataThrough">
        {{ t('channelMonitorV2.updatedTo', { time: formatTime(dataThrough) }) }}
      </span>
      <span v-else class="text-muted">{{ t('common.loading') }}</span>
      <span
        v-if="hasSnapshot && !coverageComplete && !bootstrapActive"
        class="badge badge-warning"
      >
        {{ t('channelMonitorV2.partialCoverage') }}
      </span>
      <span
        v-if="bootstrapActive"
        class="badge badge-primary inline-flex items-center gap-1"
      >
        <LoadingSpinner size="sm" />
        {{ t('channelMonitorV2.bootstrap.progress', { percent: bootstrapPercent }) }}
      </span>
    </div>

    <!-- First-upgrade silent backfill: show until 30d product window is covered -->
    <div
      v-if="bootstrapActive"
      class="monitor-toolbar-bootstrap"
      role="status"
      aria-live="polite"
    >
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div class="min-w-0 flex-1">
          <p class="text-sm font-semibold text-foreground">
            {{ t('channelMonitorV2.bootstrap.title') }}
          </p>
          <p class="mt-0.5 text-xs text-muted">
            {{ t('channelMonitorV2.bootstrap.description') }}
          </p>
        </div>
        <span class="shrink-0 text-xs font-medium tabular-nums text-accent">
          {{ t('channelMonitorV2.bootstrap.progress', { percent: bootstrapPercent }) }}
        </span>
      </div>
      <div
        class="mt-2.5 h-1.5 overflow-hidden rounded-full bg-surface-3"
        role="progressbar"
        :aria-valuenow="bootstrapPercent"
        aria-valuemin="0"
        aria-valuemax="100"
        :aria-label="t('channelMonitorV2.bootstrap.working')"
      >
        <div
          class="h-full rounded-full bg-accent transition-[width] duration-500 ease-out"
          :style="{ width: `${bootstrapPercent}%` }"
        />
      </div>
    </div>

    <!-- Single compact toolbar row: range · filters · view controls -->
    <div class="filter-row monitor-toolbar-row">
      <SegmentedControl
        :model-value="range"
        :options="ranges"
        size="sm"
        :aria-label="t('channelMonitorV2.timeRange')"
        @update:model-value="emit('update:range', $event)"
      />

      <span class="monitor-toolbar-sep" aria-hidden="true"></span>

      <FilterMultiSelect
        :model-value="platforms"
        compact
        :label="t('channelMonitorV2.filters.platform')"
        :all-label="t('channelMonitorV2.filters.allPlatforms')"
        :options="platformOptions"
        @update:model-value="emit('update:platforms', $event)"
      />
      <FilterMultiSelect
        :model-value="groupIds"
        compact
        :label="t('channelMonitorV2.filters.group')"
        :all-label="t('channelMonitorV2.filters.allGroups')"
        :options="groupOptions"
        @update:model-value="emit('update:groupIds', $event)"
      />
      <FilterMultiSelect
        :model-value="models"
        compact
        :label="t('channelMonitorV2.filters.model')"
        :all-label="t('channelMonitorV2.filters.allModels')"
        :options="modelOptions"
        @update:model-value="emit('update:models', $event)"
      />
      <button
        type="button"
        class="btn btn-ghost btn-sm shrink-0 !px-2 !py-1 text-xs"
        :disabled="!hasDimensionFilter"
        :class="!hasDimensionFilter ? 'opacity-40' : ''"
        @click="emit('clear')"
      >
        {{ t('channelMonitorV2.clearFilters') }}
      </button>

      <span class="monitor-toolbar-sep monitor-toolbar-sep-md" aria-hidden="true"></span>

      <Select
        :model-value="matrixGroupBy"
        :options="matrixGroupOptions"
        :placeholder="t('channelMonitorV2.groupBy.label')"
        class="monitor-toolbar-select w-[7.5rem] shrink-0 sm:w-[8.5rem]"
        @update:model-value="emit('update:matrixGroupBy', $event as MonitorMatrixGroupBy)"
      />

      <SegmentedControl
        :model-value="trendView"
        :options="trendViewOptions"
        size="sm"
        :aria-label="t('channelMonitorV2.trendView.label')"
        @update:model-value="emit('update:trendView', $event)"
      />

      <SegmentedControl
        v-if="trendView === 'pulse'"
        :model-value="healthMode"
        :options="healthModeOptions"
        size="sm"
        :aria-label="t('channelMonitorV2.healthMode.label')"
        @update:model-value="emit('update:healthMode', $event)"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Select from '@/components/common/Select.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import FilterMultiSelect from '@/features/channel-monitor-v2/FilterMultiSelect.vue'
import type { MonitorMatrixGroupBy, MonitorRange } from '@/api/channelMonitorV2'
import type { MonitorHealthMode, MonitorTrendView } from '@/features/channel-monitor-v2/useChannelMonitorV2'

type Option<T extends string> = { value: T; label: string }

defineProps<{
  loading: boolean
  refreshing: boolean
  hasSnapshot: boolean
  dataThrough?: string | null
  coverageComplete: boolean
  bootstrapActive: boolean
  bootstrapPercent: number
  range: MonitorRange
  ranges: Option<MonitorRange>[]
  platforms: string[]
  platformOptions: Option<string>[]
  groupIds: string[]
  groupOptions: Option<string>[]
  models: string[]
  modelOptions: Option<string>[]
  hasDimensionFilter: boolean
  matrixGroupBy: MonitorMatrixGroupBy
  matrixGroupOptions: Option<MonitorMatrixGroupBy>[]
  trendView: MonitorTrendView
  trendViewOptions: Option<MonitorTrendView>[]
  healthMode: MonitorHealthMode
  healthModeOptions: Option<MonitorHealthMode>[]
}>()

const emit = defineEmits<{
  clear: []
  'update:range': [value: MonitorRange]
  'update:platforms': [value: string[]]
  'update:groupIds': [value: string[]]
  'update:models': [value: string[]]
  'update:matrixGroupBy': [value: MonitorMatrixGroupBy]
  'update:trendView': [value: MonitorTrendView]
  'update:healthMode': [value: MonitorHealthMode]
}>()

const { t, locale } = useI18n()

function formatTime(value: string) {
  return new Intl.DateTimeFormat(locale.value || undefined, {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}
</script>

<style scoped>
.monitor-toolbar-card {
  padding: 0;
}
.monitor-toolbar-status {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  border-bottom: 1px solid var(--border);
  padding: 10px 20px;
  font-size: 12px;
  color: var(--muted);
}
.monitor-toolbar-bootstrap {
  border-bottom: 1px solid var(--border);
  background: var(--surface-2);
  padding: 12px 20px;
}
.monitor-toolbar-row {
  flex-wrap: nowrap;
  overflow-x: auto;
  padding: 12px 16px;
  min-height: 36px;
}
.monitor-toolbar-sep {
  display: none;
  height: 20px;
  width: 1px;
  flex: none;
  background: var(--border);
}
@media (min-width: 640px) {
  .monitor-toolbar-sep {
    display: block;
  }
}
@media (min-width: 768px) {
  .monitor-toolbar-sep-md {
    display: block;
  }
}
</style>
