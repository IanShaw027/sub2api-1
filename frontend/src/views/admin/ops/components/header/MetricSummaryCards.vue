<template>
  <!-- Right: 6 cards (3 cols x 2 rows) -->
  <div class="grid h-full grid-cols-1 content-center gap-4 sm:grid-cols-2 lg:col-span-7 lg:grid-cols-3">
    <!-- Card 1: Requests -->
    <div class="rounded-xl bg-surface-2 p-4 " style="order: 1;">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1">
          <span class="text-[10px] font-bold uppercase text-muted">{{ t('admin.ops.requestsTitle') }}</span>
          <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.totalRequests')" />
        </div>
        <button
          v-if="!fullscreen"
          class="text-[10px] font-bold text-accent-500 hover:underline"
          type="button"
          @click="openDetails({ title: t('admin.ops.requestDetails.title') })"
        >
          {{ t('admin.ops.requestDetails.details') }}
        </button>
      </div>
      <div class="mt-2 space-y-2 text-xs">
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.requests') }}:</span>
          <span class="font-bold text-foreground ">{{ totalRequestsLabel }}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.tokens') }}:</span>
          <span class="font-bold text-foreground ">{{ totalTokensLabel }}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.avgQps') }}:</span>
          <span class="font-bold text-foreground ">{{ qpsAvgLabel }}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.avgTps') }}:</span>
          <span class="font-bold text-foreground ">{{ tpsAvgLabel }}</span>
        </div>
      </div>
    </div>

    <!-- Card 2: SLA -->
    <div class="rounded-xl bg-surface-2 p-4 " style="order: 2;">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <span class="text-[10px] font-bold uppercase text-muted">{{ t('admin.ops.sla') }}</span>
          <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.sla')" />
          <span class="h-1.5 w-1.5 rounded-full" :class="getSLAThresholdLevel(slaPercent) === 'critical' ? 'bg-danger-500' : getSLAThresholdLevel(slaPercent) === 'warning' ? 'bg-warning-500' : 'bg-success-500'"></span>
        </div>
        <button
          v-if="!fullscreen"
          class="text-[10px] font-bold text-accent-500 hover:underline"
          type="button"
          @click="openDetails({ title: t('admin.ops.requestDetails.title'), kind: 'error' })"
        >
          {{ t('admin.ops.requestDetails.details') }}
        </button>
      </div>
      <div class="mt-2 text-3xl font-black" :class="getThresholdColorClass(getSLAThresholdLevel(slaPercent))">
        {{ slaPercent == null ? '-' : `${slaPercent.toFixed(3)}%` }}
      </div>
      <div class="mt-3 h-2 w-full overflow-hidden rounded-full bg-surface-3 ">
        <div class="h-full transition-all" :class="getSLAThresholdLevel(slaPercent) === 'critical' ? 'bg-danger-500' : getSLAThresholdLevel(slaPercent) === 'warning' ? 'bg-warning-500' : 'bg-success-500'" :style="{ width: `${Math.max((slaPercent ?? 0) - 90, 0) * 10}%` }"></div>
      </div>
      <div class="mt-3 text-xs">
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.exceptions') }}:</span>
          <span class="font-bold text-foreground ">{{ formatNumber((overview.request_count_sla ?? 0) - (overview.success_count ?? 0)) }}</span>
        </div>
      </div>
    </div>

    <!-- Card 4: Request Duration -->
    <div class="rounded-xl bg-surface-2 p-4 " style="order: 4;">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1">
          <span class="text-[10px] font-bold uppercase text-muted">{{ t('admin.ops.latencyDuration') }}</span>
          <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.latency')" />
        </div>
        <button
          v-if="!fullscreen"
          class="text-[10px] font-bold text-accent-500 hover:underline"
          type="button"
          @click="openDetails({ title: t('admin.ops.latencyDuration'), sort: 'duration_desc' })"
        >
          {{ t('admin.ops.requestDetails.details') }}
        </button>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <div class="text-3xl font-black text-foreground ">
          {{ durationP99Ms ?? '-' }}
        </div>
        <span class="text-xs font-bold text-muted">ms (P99)</span>
      </div>
      <div class="mt-3 grid grid-cols-1 gap-x-3 gap-y-1 text-xs 2xl:grid-cols-2">
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">P95:</span>
          <span class="font-bold text-foreground ">{{ durationP95Ms ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">P90:</span>
          <span class="font-bold text-foreground ">{{ durationP90Ms ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">P50:</span>
          <span class="font-bold text-foreground ">{{ durationP50Ms ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">Avg:</span>
          <span class="font-bold text-foreground ">{{ durationAvgMs ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">Max:</span>
          <span class="font-bold text-foreground ">{{ durationMaxMs ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
      </div>
    </div>

    <!-- Card 5: TTFT -->
    <div class="rounded-xl bg-surface-2 p-4 " style="order: 5;">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1">
          <span class="text-[10px] font-bold uppercase text-muted">TTFT</span>
          <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.ttft')" />
        </div>
        <button
          v-if="!fullscreen"
          class="text-[10px] font-bold text-accent-500 hover:underline"
          type="button"
          @click="openDetails({ title: t('admin.ops.ttftLabel'), sort: 'duration_desc' })"
        >
          {{ t('admin.ops.requestDetails.details') }}
        </button>
      </div>
      <div class="mt-2 flex items-baseline gap-2">
        <div class="text-3xl font-black" :class="getThresholdColorClass(getTTFTThresholdLevel(ttftP99Ms))">
          {{ ttftP99Ms ?? '-' }}
        </div>
        <span class="text-xs font-bold text-muted">ms (P99)</span>
      </div>
      <div class="mt-3 grid grid-cols-1 gap-x-3 gap-y-1 text-xs 2xl:grid-cols-2">
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">P95:</span>
          <span class="font-bold" :class="getThresholdColorClass(getTTFTThresholdLevel(ttftP95Ms))">{{ ttftP95Ms ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">P90:</span>
          <span class="font-bold" :class="getThresholdColorClass(getTTFTThresholdLevel(ttftP90Ms))">{{ ttftP90Ms ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">P50:</span>
          <span class="font-bold" :class="getThresholdColorClass(getTTFTThresholdLevel(ttftP50Ms))">{{ ttftP50Ms ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">Avg:</span>
          <span class="font-bold" :class="getThresholdColorClass(getTTFTThresholdLevel(ttftAvgMs))">{{ ttftAvgMs ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
        <div class="flex items-baseline gap-1 whitespace-nowrap">
          <span class="text-muted">Max:</span>
          <span class="font-bold" :class="getThresholdColorClass(getTTFTThresholdLevel(ttftMaxMs))">{{ ttftMaxMs ?? '-' }}</span>
          <span class="text-muted">ms</span>
        </div>
      </div>
    </div>

    <!-- Card 3: Request Errors -->
    <div class="rounded-xl bg-surface-2 p-4 " style="order: 3;">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1">
          <span class="text-[10px] font-bold uppercase text-muted">{{ t('admin.ops.requestErrors') }}</span>
          <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.errors')" />
        </div>
        <button v-if="!fullscreen" class="text-[10px] font-bold text-accent-500 hover:underline" type="button" @click="openErrorDetails('request')">
          {{ t('admin.ops.requestDetails.details') }}
        </button>
      </div>
      <div class="mt-2 text-3xl font-black" :class="getThresholdColorClass(getRequestErrorRateThresholdLevel(errorRatePercent))">
        {{ errorRatePercent == null ? '-' : `${errorRatePercent.toFixed(2)}%` }}
      </div>
      <div class="mt-3 space-y-1 text-xs">
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.errorCount') }}:</span>
          <span class="font-bold text-foreground ">{{ formatNumber(overview.error_count_sla ?? 0) }}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.businessLimited') }}:</span>
          <span class="font-bold text-foreground ">{{ formatNumber(overview.business_limited_count ?? 0) }}</span>
        </div>
      </div>
    </div>

    <!-- Card 6: Upstream Errors -->
    <div class="rounded-xl bg-surface-2 p-4 " style="order: 6;">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-1">
          <span class="text-[10px] font-bold uppercase text-muted">{{ t('admin.ops.upstreamErrors') }}</span>
          <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.upstreamErrors')" />
        </div>
        <button v-if="!fullscreen" class="text-[10px] font-bold text-accent-500 hover:underline" type="button" @click="openErrorDetails('upstream')">
          {{ t('admin.ops.requestDetails.details') }}
        </button>
      </div>
      <div class="mt-2 text-3xl font-black" :class="getThresholdColorClass(getUpstreamErrorRateThresholdLevel(upstreamErrorRatePercent))">
        {{ upstreamErrorRatePercent == null ? '-' : `${upstreamErrorRatePercent.toFixed(2)}%` }}
      </div>
      <div class="mt-3 space-y-1 text-xs">
        <div class="flex justify-between">
          <span class="text-muted">{{ t('admin.ops.errorCountExcl429529') }}:</span>
          <span class="font-bold text-foreground ">{{ formatNumber(overview.upstream_error_count_excl_429_529 ?? 0) }}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-muted">429/529:</span>
          <span class="font-bold text-foreground ">{{ formatNumber((overview.upstream_429_count ?? 0) + (overview.upstream_529_count ?? 0)) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import type { OpsDashboardOverview } from '@/api/admin/ops'
import type { OpsRequestDetailsPreset } from '../OpsRequestDetailsModal.vue'
import { formatNumber } from '@/utils/format'
import type { ThresholdLevel } from './useOpsThresholds'

const props = defineProps<{
  fullscreen?: boolean
  overview: OpsDashboardOverview
  totalRequestsLabel: string
  totalTokensLabel: string
  qpsAvgLabel: string
  tpsAvgLabel: string
  slaPercent: number | null
  errorRatePercent: number | null
  upstreamErrorRatePercent: number | null
  durationP99Ms: number | null
  durationP95Ms: number | null
  durationP90Ms: number | null
  durationP50Ms: number | null
  durationAvgMs: number | null
  durationMaxMs: number | null
  ttftP99Ms: number | null
  ttftP95Ms: number | null
  ttftP90Ms: number | null
  ttftP50Ms: number | null
  ttftAvgMs: number | null
  ttftMaxMs: number | null
  getSLAThresholdLevel: (slaPercent: number | null) => ThresholdLevel
  getTTFTThresholdLevel: (ttftMs: number | null) => ThresholdLevel
  getRequestErrorRateThresholdLevel: (errorRatePercent: number | null) => ThresholdLevel
  getUpstreamErrorRateThresholdLevel: (upstreamErrorRatePercent: number | null) => ThresholdLevel
  getThresholdColorClass: (level: ThresholdLevel) => string
}>()

const emit = defineEmits<{
  (e: 'openRequestDetails', preset?: OpsRequestDetailsPreset): void
  (e: 'openErrorDetails', kind: 'request' | 'upstream'): void
}>()

const { t } = useI18n()

function openDetails(preset?: OpsRequestDetailsPreset) {
  emit('openRequestDetails', preset)
}

function openErrorDetails(kind: 'request' | 'upstream') {
  emit('openErrorDetails', kind)
}

const {
  getSLAThresholdLevel,
  getTTFTThresholdLevel,
  getRequestErrorRateThresholdLevel,
  getUpstreamErrorRateThresholdLevel,
  getThresholdColorClass
} = props
</script>
