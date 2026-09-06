<template>
  <!-- Integrated: System health (cards) -->
  <div class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
    <!-- CPU -->
    <div class="rounded-xl bg-surface-2 p-3 ">
      <div class="flex items-center gap-1">
        <div class="text-[10px] font-bold uppercase tracking-wider text-muted">CPU</div>
        <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.cpu')" />
      </div>
      <div class="mt-1 text-lg font-black" :class="cpuPercentClass">
        {{ cpuPercentValue == null ? '-' : `${cpuPercentValue.toFixed(1)}%` }}
      </div>
      <div v-if="!fullscreen" class="mt-1 text-[10px] text-muted ">
        {{ t('common.warning') }} 80% · {{ t('common.critical') }} 95%
      </div>
    </div>

    <!-- MEM -->
    <div class="rounded-xl bg-surface-2 p-3 ">
      <div class="flex items-center gap-1">
        <div class="text-[10px] font-bold uppercase tracking-wider text-muted">{{ t('admin.ops.memory') }}</div>
        <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.memory')" />
      </div>
      <div class="mt-1 text-lg font-black" :class="memPercentClass">
        {{ memPercentValue == null ? '-' : `${memPercentValue.toFixed(1)}%` }}
      </div>
      <div v-if="!fullscreen" class="mt-1 text-[10px] text-muted ">
        {{
          systemMetrics?.memory_used_mb == null || systemMetrics?.memory_total_mb == null
            ? '-'
            : `${formatMemorySizeMB(systemMetrics.memory_used_mb)} / ${formatMemorySizeMB(systemMetrics.memory_total_mb)}`
        }}
      </div>
    </div>

    <!-- DB -->
    <div class="rounded-xl bg-surface-2 p-3 ">
      <div class="flex items-center gap-1">
        <div class="text-[10px] font-bold uppercase tracking-wider text-muted">{{ t('admin.ops.db') }}</div>
        <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.db')" />
      </div>
      <div class="mt-1 text-lg font-black" :class="dbMiddleClass">
        {{ dbMiddleLabel }}
      </div>
      <div v-if="!fullscreen" class="mt-1 text-[10px] text-muted ">
        {{ t('admin.ops.conns') }} {{ dbConnOpenValue ?? '-' }} / {{ dbMaxOpenConnsValue ?? '-' }}
        · {{ t('admin.ops.active') }} {{ dbConnActiveValue ?? '-' }}
        · {{ t('admin.ops.idle') }} {{ dbConnIdleValue ?? '-' }}
        <span v-if="dbConnWaitingValue != null"> · {{ t('admin.ops.waiting') }} {{ dbConnWaitingValue }} </span>
      </div>
    </div>

    <!-- Redis -->
    <div class="rounded-xl bg-surface-2 p-3 ">
      <div class="flex items-center gap-1">
        <div class="text-[10px] font-bold uppercase tracking-wider text-muted">Redis</div>
        <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.redis')" />
      </div>
      <div class="mt-1 text-lg font-black" :class="redisMiddleClass">
        {{ redisMiddleLabel }}
      </div>
      <div v-if="!fullscreen" class="mt-1 text-[10px] text-muted ">
        {{ t('admin.ops.conns') }} {{ redisConnTotalValue ?? '-' }} / {{ redisPoolSizeValue ?? '-' }}
        <span v-if="redisConnActiveValue != null"> · {{ t('admin.ops.active') }} {{ redisConnActiveValue }} </span>
        <span v-if="redisConnIdleValue != null"> · {{ t('admin.ops.idle') }} {{ redisConnIdleValue }} </span>
      </div>
    </div>

    <!-- Goroutines -->
    <div class="rounded-xl bg-surface-2 p-3 ">
      <div class="flex items-center gap-1">
        <div class="text-[10px] font-bold uppercase tracking-wider text-muted">{{ t('admin.ops.goroutines') }}</div>
        <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.goroutines')" />
      </div>
      <div class="mt-1 text-lg font-black" :class="goroutineStatusClass">
        {{ goroutineStatusLabel }}
      </div>
      <div v-if="!fullscreen" class="mt-1 text-[10px] text-muted ">
        {{ t('admin.ops.current') }} <span class="font-mono">{{ goroutineCountValue ?? '-' }}</span>
        · {{ t('common.warning') }} <span class="font-mono">{{ goroutinesWarnThreshold }}</span>
        · {{ t('common.critical') }} <span class="font-mono">{{ goroutinesCriticalThreshold }}</span>
        <span v-if="systemMetrics?.concurrency_queue_depth != null">
          · {{ t('admin.ops.queue') }} <span class="font-mono">{{ systemMetrics.concurrency_queue_depth }}</span>
        </span>
      </div>
    </div>

    <!-- Jobs -->
    <div class="rounded-xl bg-surface-2 p-3 ">
      <div class="flex items-center justify-between gap-2">
        <div class="flex items-center gap-1">
          <div class="text-[10px] font-bold uppercase tracking-wider text-muted">{{ t('admin.ops.jobs') }}</div>
          <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.jobs')" />
        </div>
        <button v-if="!fullscreen" class="text-[10px] font-bold text-accent-500 hover:underline" type="button" @click="openJobsDetails">
          {{ t('admin.ops.requestDetails.details') }}
        </button>
      </div>

      <div class="mt-1 text-lg font-black" :class="jobsStatusClass">
        {{ jobsStatusLabel }}
      </div>

      <div v-if="!fullscreen" class="mt-1 text-[10px] text-muted ">
        {{ t('common.total') }} <span class="font-mono">{{ jobHeartbeatsCount }}</span>
        · {{ t('common.warning') }} <span class="font-mono">{{ jobsWarnCount }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import { formatMemorySizeMB } from '../../utils/opsFormatters'
import type { OpsSystemMetricsSnapshot } from '@/api/admin/ops'

defineProps<{
  fullscreen?: boolean
  systemMetrics: OpsSystemMetricsSnapshot | null
  cpuPercentValue: number | null
  cpuPercentClass: string
  memPercentValue: number | null
  memPercentClass: string
  dbConnActiveValue: number | null
  dbConnIdleValue: number | null
  dbConnWaitingValue: number | null
  dbConnOpenValue: number | null
  dbMaxOpenConnsValue: number | null
  dbMiddleLabel: string
  dbMiddleClass: string
  redisConnTotalValue: number | null
  redisConnIdleValue: number | null
  redisConnActiveValue: number | null
  redisPoolSizeValue: number | null
  redisMiddleLabel: string
  redisMiddleClass: string
  goroutineCountValue: number | null
  goroutinesWarnThreshold: number
  goroutinesCriticalThreshold: number
  goroutineStatusLabel: string
  goroutineStatusClass: string
  jobHeartbeatsCount: number
  jobsWarnCount: number
  jobsStatusLabel: string
  jobsStatusClass: string
}>()

const emit = defineEmits<{
  (e: 'openJobsDetails'): void
}>()

const { t } = useI18n()

function openJobsDetails() {
  emit('openJobsDetails')
}
</script>
