<template>
  <!-- 2) Realtime Traffic -->
  <div class="flex h-full flex-col justify-center py-2">
    <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
      <div class="flex items-center gap-2">
        <div class="relative flex h-3 w-3 shrink-0">
          <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-accent-400 opacity-75"></span>
          <span class="relative inline-flex h-3 w-3 rounded-full bg-accent-500"></span>
        </div>
        <h3 class="text-xs font-bold uppercase tracking-wider text-muted">{{ t('admin.ops.realtime.title') }}</h3>
        <HelpTooltip v-if="!fullscreen" :content="t('admin.ops.tooltips.qps')" />
      </div>

      <!-- Time Window Selector -->
      <div class="flex flex-wrap gap-1">
        <button
          v-for="window in availableRealtimeWindows"
          :key="window"
          type="button"
          class="rounded px-1.5 py-0.5 text-[9px] font-bold transition-colors sm:px-2 sm:text-[10px]"
          :class="realtimeWindow === window
            ? 'bg-accent-500 text-white'
            : 'bg-surface-3 text-muted hover:bg-surface-3   '"
          @click="realtimeWindow = window"
        >
          {{ window }}
        </button>
      </div>
    </div>

    <div :class="fullscreen ? 'space-y-4' : 'space-y-3'">
      <!-- Row 1: Current -->
      <div>
        <div :class="[fullscreen ? 'text-xs' : 'text-[10px]', 'font-bold uppercase text-muted']">{{ t('admin.ops.current') }}</div>
        <div class="mt-1 flex flex-wrap items-baseline gap-x-4 gap-y-2">
          <div class="flex items-baseline gap-1.5">
            <span :class="[fullscreen ? 'text-4xl' : 'text-xl sm:text-2xl', 'font-black text-foreground ']">{{ displayRealTimeQps.toFixed(1) }}</span>
            <span :class="[fullscreen ? 'text-sm' : 'text-xs', 'font-bold text-muted']">QPS</span>
          </div>
          <div class="flex items-baseline gap-1.5">
            <span :class="[fullscreen ? 'text-4xl' : 'text-xl sm:text-2xl', 'font-black text-foreground ']">{{ displayRealTimeTps.toFixed(1) }}</span>
            <span :class="[fullscreen ? 'text-sm' : 'text-xs', 'font-bold text-muted']">{{ t('admin.ops.tps') }}</span>
          </div>
        </div>
      </div>

      <!-- Row 2: Peak + Average -->
      <div class="grid grid-cols-2 gap-3">
        <!-- Peak -->
        <div>
          <div :class="[fullscreen ? 'text-xs' : 'text-[10px]', 'font-bold uppercase text-muted']">{{ t('admin.ops.peak') }}</div>
          <div :class="[fullscreen ? 'text-base' : 'text-sm', 'mt-1 space-y-0.5 font-medium text-muted ']">
            <div class="flex items-baseline gap-1.5">
              <span class="font-black text-foreground ">{{ realtimeQpsPeakLabel }}</span>
              <span class="text-xs">QPS</span>
            </div>
            <div class="flex items-baseline gap-1.5">
              <span class="font-black text-foreground ">{{ realtimeTpsPeakLabel }}</span>
              <span class="text-xs">{{ t('admin.ops.tps') }}</span>
            </div>
          </div>
        </div>

        <!-- Average -->
        <div>
          <div :class="[fullscreen ? 'text-xs' : 'text-[10px]', 'font-bold uppercase text-muted']">{{ t('admin.ops.average') }}</div>
          <div :class="[fullscreen ? 'text-base' : 'text-sm', 'mt-1 space-y-0.5 font-medium text-muted ']">
            <div class="flex items-baseline gap-1.5">
              <span class="font-black text-foreground ">{{ realtimeQpsAvgLabel }}</span>
              <span class="text-xs">QPS</span>
            </div>
            <div class="flex items-baseline gap-1.5">
              <span class="font-black text-foreground ">{{ realtimeTpsAvgLabel }}</span>
              <span class="text-xs">{{ t('admin.ops.tps') }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Animated Pulse Line (Heart Beat Animation) -->
      <div class="h-8 w-full overflow-hidden opacity-50">
        <svg class="h-full w-full" viewBox="0 0 280 32" preserveAspectRatio="none">
          <path
            d="M0 16 Q 20 16, 40 16 T 80 16 T 120 10 T 160 22 T 200 16 T 240 16 T 280 16"
            fill="none"
            stroke="var(--accent)"
            stroke-width="2"
            vector-effect="non-scaling-stroke"
          >
            <animate
              attributeName="d"
              dur="2s"
              repeatCount="indefinite"
              values="M0 16 Q 20 16, 40 16 T 80 16 T 120 10 T 160 22 T 200 16 T 240 16 T 280 16;
                      M0 16 Q 20 16, 40 16 T 80 16 T 120 16 T 160 16 T 200 10 T 240 22 T 280 16;
                      M0 16 Q 20 16, 40 16 T 80 16 T 120 16 T 160 16 T 200 16 T 240 16 T 280 16"
              keyTimes="0;0.5;1"
            />
          </path>
        </svg>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import type { RealtimeWindow } from './useOpsRealtimeTraffic'

defineProps<{
  fullscreen?: boolean
  availableRealtimeWindows: readonly RealtimeWindow[]
  displayRealTimeQps: number
  displayRealTimeTps: number
  realtimeQpsPeakLabel: string
  realtimeTpsPeakLabel: string
  realtimeQpsAvgLabel: string
  realtimeTpsAvgLabel: string
}>()

const realtimeWindow = defineModel<RealtimeWindow>({ required: true })

const { t } = useI18n()
</script>
