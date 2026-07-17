<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Chart as ChartJS, ArcElement, Legend, Tooltip } from 'chart.js'
import { Doughnut } from 'vue-chartjs'
import type { OpsErrorDistributionResponse } from '@/api/admin/ops'
import type { ChartState } from '../types'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

ChartJS.register(ArcElement, Tooltip, Legend)

interface Props {
  data: OpsErrorDistributionResponse | null
  loading: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'openDetails'): void
}>()
const { t } = useI18n()

const isDarkMode = computed(() => document.documentElement.classList.contains('dark'))
const colors = computed(() => ({
  blue: '#2f7bf6',
  red: '#ef4444',
  orange: '#f59e0b',
  gray: '#9ca3af',
  text: isDarkMode.value ? '#94a3b8' : '#64748b'
}))

const totalSlaErrors = computed(() =>
  (props.data?.items ?? []).reduce((total, item) => total + Number(item.sla || 0), 0)
)
const recoveredTelemetryTotal = computed(() => Number(props.data?.recovered_telemetry_total || 0))

const hasData = computed(() => totalSlaErrors.value > 0)

const state = computed<ChartState>(() => {
  if (hasData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

interface ErrorCategory {
  label: string
  count: number
  color: string
}

function buildOwnerCategories(): ErrorCategory[] {
  const ownerItems = props.data?.owners || []
  if (ownerItems.length === 0) return []

  const out: ErrorCategory[] = []
  for (const item of ownerItems) {
    const owner = String(item.owner || '').toLowerCase()
    const count = Number(item.sla || 0)
    if (!owner || !Number.isFinite(count) || count <= 0) continue

    if (owner === 'provider') out.push({ label: t('admin.ops.errorDetails.owner.provider'), count, color: colors.value.orange })
    else if (owner === 'account') out.push({ label: t('admin.ops.errorDetails.owner.account'), count, color: '#1fa2d6' })
    else if (owner === 'client') out.push({ label: t('admin.ops.errorDetails.owner.client'), count, color: colors.value.blue })
    else if (owner === 'platform') out.push({ label: t('admin.ops.errorDetails.owner.platform'), count, color: colors.value.red })
    else out.push({ label: t('admin.ops.other'), count, color: colors.value.gray })
  }
  return out
}

function buildStatusCategories(): ErrorCategory[] {
  if (!props.data) return []

  let upstream = 0 // 502, 503, 504
  let client = 0 // 4xx
  let system = 0 // 500
  let other = 0

  for (const item of props.data.items || []) {
    const code = Number(item.status_code || 0)
    const count = Number(item.sla || 0)
    if (!Number.isFinite(code) || !Number.isFinite(count)) continue

    if ([502, 503, 504].includes(code)) upstream += count
    else if (code >= 400 && code < 500) client += count
    else if (code === 500) system += count
    else other += count
  }

  const out: ErrorCategory[] = []
  if (upstream > 0) out.push({ label: t('admin.ops.upstream'), count: upstream, color: colors.value.orange })
  if (client > 0) out.push({ label: t('admin.ops.client'), count: client, color: colors.value.blue })
  if (system > 0) out.push({ label: t('admin.ops.system'), count: system, color: colors.value.red })
  if (other > 0) out.push({ label: t('admin.ops.other'), count: other, color: colors.value.gray })
  return out
}

const categories = computed<ErrorCategory[]>(() => {
  const ownerCategories = buildOwnerCategories()
  return ownerCategories.length > 0 ? ownerCategories : buildStatusCategories()
})

const topReason = computed(() => {
  if (categories.value.length === 0) return null
  return categories.value.reduce((prev, cur) => (cur.count > prev.count ? cur : prev))
})

const chartData = computed(() => {
  if (!hasData.value || categories.value.length === 0) return null
  return {
    labels: categories.value.map((c) => c.label),
    datasets: [
      {
        data: categories.value.map((c) => c.count),
        backgroundColor: categories.value.map((c) => c.color),
        borderWidth: 0
      }
    ]
  }
})

const options = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      backgroundColor: isDarkMode.value ? '#1e293b' : '#ffffff',
      titleColor: isDarkMode.value ? '#e2e8f0' : '#16314f',
      bodyColor: isDarkMode.value ? '#cbd5e1' : '#334a66'
    }
  }
}))
</script>

<template>
  <div class="flex h-full flex-col rounded-3xl bg-card p-6 shadow-xs ring-1 ring-line dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-bold text-ink dark:text-white">
        <svg class="h-4 w-4 text-danger" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
          />
        </svg>
        {{ t('admin.ops.errorDistribution') }}
        <HelpTooltip :content="t('admin.ops.tooltips.errorDistribution')" />
      </h3>
      <button
        type="button"
        class="inline-flex items-center rounded-lg border border-line bg-card px-2 py-1 text-[11px] font-semibold text-ink-soft hover:bg-page disabled:opacity-50 dark:border-dark-700 dark:bg-dark-900 dark:hover:bg-dark-800"
        :disabled="state !== 'ready'"
        :title="t('admin.ops.errorTrend')"
        @click="emit('openDetails')"
      >
        {{ t('admin.ops.requestDetails.details') }}
      </button>
    </div>

    <div class="relative min-h-0 flex-1">
      <div v-if="state === 'ready' && chartData" class="flex h-full flex-col">
        <div class="flex-1">
          <Doughnut :data="chartData" :options="{ ...options, cutout: '65%' }" />
        </div>
        <div class="mt-4 flex flex-col items-center gap-2">
          <div v-if="topReason" class="text-xs font-bold text-ink dark:text-white">
            {{ t('admin.ops.top') }}: <span :style="{ color: topReason.color }">{{ topReason.label }}</span>
          </div>
          <div
            v-if="recoveredTelemetryTotal > 0"
            class="rounded-full bg-accent-50 px-2 py-1 text-[11px] font-semibold text-accent-700 ring-1 ring-accent/20 dark:bg-sky-500/10 dark:text-sky-300 dark:ring-sky-500/20"
          >
            {{ t('admin.ops.recoveredTelemetry') }}: {{ recoveredTelemetryTotal }}
          </div>
          <div class="flex flex-wrap justify-center gap-3">
            <div v-for="item in categories" :key="item.label" class="flex items-center gap-1.5 text-xs">
              <span class="h-2 w-2 rounded-full" :style="{ backgroundColor: item.color }"></span>
              <span class="text-ink-soft">{{ item.label }} {{ item.count }}</span>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="flex h-full flex-col items-center justify-center gap-3">
        <div v-if="state === 'loading'" class="animate-pulse text-sm text-ink-faint">{{ t('common.loading') }}</div>
        <template v-else>
          <EmptyState :title="t('common.noData')" :description="t('admin.ops.charts.emptyError')" />
          <div
            v-if="recoveredTelemetryTotal > 0"
            class="rounded-full bg-accent-50 px-2 py-1 text-[11px] font-semibold text-accent-700 ring-1 ring-accent/20 dark:bg-sky-500/10 dark:text-sky-300 dark:ring-sky-500/20"
          >
            {{ t('admin.ops.recoveredTelemetry') }}: {{ recoveredTelemetryTotal }}
          </div>
        </template>
      </div>
    </div>
  </div>
</template>
