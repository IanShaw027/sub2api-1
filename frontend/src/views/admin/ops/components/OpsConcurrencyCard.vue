<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useIntervalFn } from '@vueuse/core'
import { opsAPI, type OpsConcurrencyStatsResponse, type OpsSystemHealthResponse } from '@/api/admin/ops'

interface Props {
  platformFilter?: string
  groupIdFilter?: number | null
}

const props = withDefaults(defineProps<Props>(), {
  platformFilter: '',
  groupIdFilter: null
})

const { t } = useI18n()

const loading = ref(false)
const errorMessage = ref('')

const concurrency = ref<OpsConcurrencyStatsResponse | null>(null)
const health = ref<OpsSystemHealthResponse | null>(null)

const nowMs = ref(Date.now())
const { pause: pauseNowTick, resume: resumeNowTick } = useIntervalFn(
  () => {
    nowMs.value = Date.now()
  },
  5000,
  { immediate: true }
)

const realtimeEnabled = computed(() => {
  // Prefer health.enabled because health polling continues even when realtime is disabled,
  // allowing the UI to automatically recover when the setting is turned back on.
  if (health.value && typeof health.value.enabled === 'boolean') return health.value.enabled
  if (concurrency.value && typeof concurrency.value.enabled === 'boolean') return concurrency.value.enabled
  return true
})

function safeNumber(n: unknown): number {
  return typeof n === 'number' && Number.isFinite(n) ? n : 0
}

const platformRows = computed(() => {
  const stats = concurrency.value?.platform || {}
  const rows = Object.values(stats)
    .filter(Boolean)
    .filter((row) => (props.platformFilter ? row.platform === props.platformFilter : true))
    .map((row) => ({
      platform: String(row.platform || ''),
      current_in_use: safeNumber(row.current_in_use),
      max_capacity: safeNumber(row.max_capacity),
      load_percentage: safeNumber(row.load_percentage),
      waiting_in_queue: safeNumber(row.waiting_in_queue)
    }))
  return rows.sort((a, b) => b.load_percentage - a.load_percentage)
})

const groupRows = computed(() => {
  const stats = concurrency.value?.group || {}
  const rows = Object.values(stats)
    .filter(Boolean)
    .filter((row) =>
      typeof props.groupIdFilter === 'number' && props.groupIdFilter > 0
        ? row.group_id === props.groupIdFilter
        : true
    )
    .map((row) => ({
      group_id: safeNumber(row.group_id),
      group_name: String(row.group_name || ''),
      platform: String(row.platform || ''),
      current_in_use: safeNumber(row.current_in_use),
      max_capacity: safeNumber(row.max_capacity),
      load_percentage: safeNumber(row.load_percentage),
      waiting_in_queue: safeNumber(row.waiting_in_queue)
    }))
  return rows.sort((a, b) => b.load_percentage - a.load_percentage)
})

const waitQueueTotal = computed(() => {
  return platformRows.value.reduce((sum, row) => sum + safeNumber(row.waiting_in_queue), 0)
})

const concurrencyCollectedAtMs = computed(() => {
  const raw = concurrency.value?.timestamp
  if (!raw) return null
  const ms = Date.parse(String(raw))
  return Number.isFinite(ms) ? ms : null
})

const concurrencyAgeSeconds = computed(() => {
  if (concurrencyCollectedAtMs.value == null) return null
  return Math.max(0, Math.floor((nowMs.value - concurrencyCollectedAtMs.value) / 1000))
})

function formatAge(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0s'
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`
  return `${Math.floor(seconds / 3600)}h`
}

const concurrencyUpdatedLabel = computed(() => {
  if (concurrencyAgeSeconds.value == null) return t('admin.ops.concurrency.updatedUnknown')
  return t('admin.ops.concurrency.updatedAgo', { age: formatAge(concurrencyAgeSeconds.value) })
})

const concurrencyIsStale = computed(() => {
  // Collector ticks every ~60s. If we're older than 90s, treat it as delayed.
  return concurrencyAgeSeconds.value != null && concurrencyAgeSeconds.value > 90
})

function progressBarStyle(percent: number) {
  const p = Math.min(Math.max(percent, 0), 100)
  return { width: `${p}%` }
}

function progressBarClass(percent: number) {
  if (percent >= 90) return 'bg-red-500'
  if (percent >= 70) return 'bg-yellow-500'
  return 'bg-green-500'
}

const healthBadge = computed(() => {
  if (!realtimeEnabled.value) return { label: t('admin.ops.concurrency.realtimeDisabled'), cls: 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200' }
  const status = health.value?.status
  if (status === 'unhealthy') return { label: t('admin.ops.healthStatus.unhealthy'), cls: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-300' }
  if (status === 'degraded') return { label: t('admin.ops.healthStatus.degraded'), cls: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300' }
  if (status === 'healthy') return { label: t('admin.ops.healthStatus.healthy'), cls: 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-300' }
  return { label: t('admin.ops.healthStatus.unknown'), cls: 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-200' }
})

async function loadHealth() {
  try {
    health.value = await opsAPI.getSystemHealth()
  } catch (err) {
    // Avoid spamming the UI: health is a secondary signal.
    console.warn('[OpsConcurrencyCard] Failed to load health:', err)
  }
}

async function loadConcurrency() {
  if (!realtimeEnabled.value) {
    concurrency.value = {
      enabled: false,
      platform: {},
      group: {}
    }
    return
  }
  loading.value = true
  errorMessage.value = ''
  try {
    concurrency.value = await opsAPI.getConcurrencyStats()
  } catch (err) {
    console.error('[OpsConcurrencyCard] Failed to load concurrency stats:', err)
    errorMessage.value = t('admin.ops.concurrency.failedToLoad')
    concurrency.value = {
      enabled: true,
      platform: {},
      group: {}
    }
  } finally {
    loading.value = false
  }
}

const { pause: pauseHealth, resume: resumeHealth } = useIntervalFn(loadHealth, 30000, { immediate: false })
const { pause: pauseConcurrency, resume: resumeConcurrency } = useIntervalFn(loadConcurrency, 5000, { immediate: false })

onMounted(async () => {
  await loadHealth()
  await loadConcurrency()
  resumeHealth()
  resumeNowTick()
  if (realtimeEnabled.value) {
    resumeConcurrency()
  }
})

onUnmounted(() => {
  pauseHealth()
  pauseConcurrency()
  pauseNowTick()
})

watch(realtimeEnabled, async (enabled) => {
  if (!enabled) {
    pauseConcurrency()
    await loadConcurrency()
  } else {
    resumeConcurrency()
    // When it flips back to enabled, refresh immediately.
    await loadConcurrency()
  }
})
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.concurrency.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.concurrency.description') }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <span class="rounded-full px-2.5 py-1 text-[11px] font-bold" :class="healthBadge.cls">
          {{ healthBadge.label }}
        </span>
        <span class="text-[11px] text-gray-500 dark:text-gray-400">
          {{ concurrencyUpdatedLabel }}
        </span>
        <span v-if="concurrencyIsStale" class="rounded-full bg-yellow-100 px-2.5 py-1 text-[11px] font-bold text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300">
          {{ t('admin.ops.concurrency.dataDelayed') }}
        </span>
        <span v-if="waitQueueTotal > 0" class="rounded-full bg-yellow-100 px-2.5 py-1 text-[11px] font-bold text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-300">
          {{ t('admin.ops.concurrency.waiting', { count: waitQueueTotal }) }}
        </span>
        <button
          class="flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
          :disabled="loading"
          @click="loadHealth(); loadConcurrency()"
        >
          <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div v-if="errorMessage" class="mb-4 rounded-2xl bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
      {{ errorMessage }}
    </div>

    <div v-if="!realtimeEnabled" class="rounded-xl border border-dashed border-gray-200 p-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
      {{ t('admin.ops.concurrency.disabledHint') }}
    </div>

    <div v-else class="grid grid-cols-1 gap-4 lg:grid-cols-2">
      <div class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
        <div class="mb-3 text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.concurrency.byPlatform') }}
        </div>

        <div v-if="platformRows.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.concurrency.empty') }}
        </div>

        <div v-else class="space-y-3">
          <div v-for="row in platformRows.slice(0, 8)" :key="row.platform" class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
            <div class="mb-2 flex items-center justify-between gap-2">
              <div class="text-xs font-bold text-gray-900 dark:text-white">
                {{ row.platform || t('admin.ops.concurrency.unknownPlatform') }}
              </div>
              <div class="text-xs text-gray-600 dark:text-gray-300">
                {{ row.current_in_use }} / {{ row.max_capacity }}
              </div>
            </div>

            <div class="h-2 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
              <div class="h-full rounded-full" :class="progressBarClass(row.load_percentage)" :style="progressBarStyle(row.load_percentage)"></div>
            </div>

            <div class="mt-2 flex items-center justify-between text-[11px] text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.ops.concurrency.load', { percent: row.load_percentage.toFixed(1) }) }}</span>
              <span v-if="row.waiting_in_queue > 0">{{ t('admin.ops.concurrency.waitingShort', { count: row.waiting_in_queue }) }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
        <div class="mb-3 text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.concurrency.byGroup') }}
        </div>
        <div class="mb-3 text-[11px] text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.concurrency.byGroupHint') }}
        </div>

        <div v-if="groupRows.length === 0" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.ops.concurrency.empty') }}
        </div>

        <div v-else class="space-y-3">
          <div v-for="row in groupRows.slice(0, 8)" :key="row.group_id" class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
            <div class="mb-2 flex items-center justify-between gap-2">
              <div class="text-xs font-bold text-gray-900 dark:text-white">
                {{ row.group_name || `Group ${row.group_id}` }}
              </div>
              <div class="text-xs text-gray-600 dark:text-gray-300">
                {{ row.current_in_use }} / {{ row.max_capacity }}
              </div>
            </div>

            <div class="h-2 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
              <div class="h-full rounded-full" :class="progressBarClass(row.load_percentage)" :style="progressBarStyle(row.load_percentage)"></div>
            </div>

            <div class="mt-2 flex items-center justify-between text-[11px] text-gray-500 dark:text-gray-400">
              <span>{{ t('admin.ops.concurrency.load', { percent: row.load_percentage.toFixed(1) }) }}</span>
              <span v-if="row.platform">{{ row.platform }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
