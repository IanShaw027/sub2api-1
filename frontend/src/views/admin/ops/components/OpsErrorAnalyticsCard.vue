<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Line } from 'vue-chartjs'
import type { OpsMetrics } from '@/api/admin/ops'
import { opsAPI, type OpsWindowStatsGroupedItem } from '@/api/admin/ops'
import { useAppStore } from '@/stores/app'
import Select from '@/components/common/Select.vue'
import { parseTimeRangeMinutes, formatHistoryLabel } from '../utils/opsFormatters'
import EmptyState from '@/components/common/EmptyState.vue'

const { t } = useI18n()
const appStore = useAppStore()

const props = defineProps<{
  timeRange: string
}>()

const loadingTrend = ref(false)
const loadingGrouped = ref(false)

const trendItems = ref<OpsMetrics[]>([])
const groupedItems = ref<OpsWindowStatsGroupedItem[]>([])

const groupBy = ref<'platform' | 'phase' | 'severity'>('phase')
const groupByOptions = computed(() => [
  { value: 'phase', label: t('admin.ops.errorAnalytics.groupBy.phase') },
  { value: 'platform', label: t('admin.ops.errorAnalytics.groupBy.platform') },
  { value: 'severity', label: t('admin.ops.errorAnalytics.groupBy.severity') }
])

function handleGroupByChange(val: string | number | boolean | null) {
  const next = String(val || 'phase')
  groupBy.value = next === 'platform' || next === 'severity' || next === 'phase' ? next : 'phase'
}

function computeTimeRange(): { start: string, end: string } {
  const minutes = parseTimeRangeMinutes(props.timeRange)
  const end = new Date()
  const start = new Date(end.getTime() - minutes * 60 * 1000)
  return { start: start.toISOString(), end: end.toISOString() }
}

function pickInterval(minutes: number): '1m' | '5m' | '1h' {
  if (minutes <= 60) return '1m'
  if (minutes <= 6 * 60) return '5m'
  return '1h'
}

async function loadTrend() {
  loadingTrend.value = true
  try {
    const minutes = parseTimeRangeMinutes(props.timeRange)
    const { start, end } = computeTimeRange()
    const interval = pickInterval(minutes)
    const res = await opsAPI.getErrorTimeseries({
      start_time: start,
      end_time: end,
      interval
    })
    trendItems.value = res.items || []
  } catch (err: any) {
    console.error('[OpsErrorAnalyticsCard] Failed to load error timeseries', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.errorAnalytics.trend.loadFailed'))
    trendItems.value = []
  } finally {
    loadingTrend.value = false
  }
}

async function loadGrouped() {
  loadingGrouped.value = true
  try {
    const { start, end } = computeTimeRange()
    const res = await opsAPI.getErrorStatsGrouped({
      start_time: start,
      end_time: end,
      group_by: groupBy.value
    })
    groupedItems.value = res.items || []
  } catch (err: any) {
    console.error('[OpsErrorAnalyticsCard] Failed to load error stats grouped', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.errorAnalytics.grouped.loadFailed'))
    groupedItems.value = []
  } finally {
    loadingGrouped.value = false
  }
}

async function loadAll() {
  await Promise.all([loadTrend(), loadGrouped()])
}

onMounted(() => {
  loadAll()
})

watch(
  () => props.timeRange,
  () => {
    loadAll()
  }
)

watch(groupBy, () => {
  loadGrouped()
})

const hasTrendData = computed(() => (trendItems.value || []).some(i => (i.error_count || 0) > 0))
const trendChartData = computed(() => {
  if (!hasTrendData.value) return null
  return {
    labels: trendItems.value.map(m => formatHistoryLabel(m.updated_at, props.timeRange)),
    datasets: [
      {
        label: t('admin.ops.errorAnalytics.trend.errorCount'),
        data: trendItems.value.map(m => m.error_count || 0),
        borderColor: '#ef4444',
        backgroundColor: '#ef444420',
        fill: true,
        tension: 0.35,
        pointRadius: 0,
        pointHitRadius: 10
      },
      {
        label: t('admin.ops.errorAnalytics.trend.errorRate'),
        data: trendItems.value.map(m => m.error_rate || 0),
        borderColor: '#f59e0b',
        backgroundColor: '#f59e0b20',
        fill: false,
        tension: 0.35,
        pointRadius: 0,
        pointHitRadius: 10,
        yAxisID: 'y1'
      }
    ]
  }
})

const trendOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      position: 'top' as const,
      align: 'end' as const,
      labels: { font: { size: 10 } }
    }
  },
  scales: {
    x: {
      grid: { display: false },
      ticks: { font: { size: 10 }, maxTicksLimit: 8 }
    },
    y: {
      beginAtZero: true,
      ticks: { font: { size: 10 } }
    },
    y1: {
      beginAtZero: true,
      position: 'right' as const,
      grid: { display: false },
      ticks: { font: { size: 10 } }
    }
  }
}))

const groupedSorted = computed(() => {
  const rows = groupedItems.value || []
  return [...rows].sort((a, b) => (b.error_count || 0) - (a.error_count || 0))
})
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.errorAnalytics.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.errorAnalytics.description') }}</p>
      </div>

      <div class="flex items-center gap-2">
        <Select :model-value="groupBy" :options="groupByOptions" class="w-[160px]" @change="handleGroupByChange" />
        <button
          class="flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
          :disabled="loadingTrend || loadingGrouped"
          @click="loadAll"
        >
          <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loadingTrend || loadingGrouped }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <div class="mb-2 text-xs font-bold uppercase tracking-wider text-gray-400">
          {{ t('admin.ops.errorAnalytics.trend.title') }}
        </div>

        <div v-if="loadingTrend" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.errorAnalytics.trend.loading') }}
        </div>
        <div v-else-if="!trendChartData" class="py-6">
          <EmptyState :title="t('admin.ops.errorAnalytics.trend.empty')" :description="t('admin.ops.errorAnalytics.trend.emptyHint')" />
        </div>
        <div v-else class="h-[260px]">
          <Line :data="trendChartData" :options="trendOptions" />
        </div>
      </div>

      <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-700/50">
        <div class="mb-2 flex items-center justify-between gap-2">
          <div class="text-xs font-bold uppercase tracking-wider text-gray-400">
            {{ t('admin.ops.errorAnalytics.grouped.title') }}
          </div>
          <div class="text-[11px] text-gray-400">
            {{ t('admin.ops.errorAnalytics.grouped.groupBy') }}: {{ groupBy }}
          </div>
        </div>

        <div v-if="loadingGrouped" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.errorAnalytics.grouped.loading') }}
        </div>
        <div v-else-if="groupedSorted.length === 0" class="py-6">
          <EmptyState :title="t('admin.ops.errorAnalytics.grouped.empty')" :description="t('admin.ops.errorAnalytics.grouped.emptyHint')" />
        </div>
        <div v-else class="overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800">
          <div class="overflow-x-auto">
            <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
              <thead class="bg-gray-50 dark:bg-dark-900">
                <tr>
                  <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.ops.errorAnalytics.grouped.table.group') }}
                  </th>
                  <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.ops.errorAnalytics.grouped.table.errors') }}
                  </th>
                  <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    4xx
                  </th>
                  <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    5xx
                  </th>
                  <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                    {{ t('admin.ops.errorAnalytics.grouped.table.timeouts') }}
                  </th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                <tr v-for="row in groupedSorted" :key="row.group" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                  <td class="px-4 py-3 text-xs font-medium text-gray-700 dark:text-gray-200">
                    {{ row.group || '-' }}
                  </td>
                  <td class="px-4 py-3 text-right text-xs font-bold text-gray-900 dark:text-white">
                    {{ row.error_count }}
                  </td>
                  <td class="px-4 py-3 text-right text-xs text-gray-600 dark:text-gray-300">
                    {{ row.error_4xx_count }}
                  </td>
                  <td class="px-4 py-3 text-right text-xs text-gray-600 dark:text-gray-300">
                    {{ row.error_5xx_count }}
                  </td>
                  <td class="px-4 py-3 text-right text-xs text-gray-600 dark:text-gray-300">
                    {{ row.timeout_count }}
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
