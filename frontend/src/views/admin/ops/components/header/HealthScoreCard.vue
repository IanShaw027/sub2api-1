<template>
  <!-- 1) Health Score -->
  <div
    class="group relative flex cursor-pointer flex-col items-center justify-center rounded-xl py-2 transition-all hover:bg-[color-mix(in_oklch,var(--surface)_60%,transparent)]  md:border-r md:border-line md:pr-6 "
  >
    <!-- Diagnosis Popover (hover) -->
    <div
      class="pointer-events-none absolute left-1/2 top-full z-50 mt-2 w-72 -translate-x-1/2 opacity-0 transition-opacity duration-200 group-hover:pointer-events-auto group-hover:opacity-100 md:left-full md:top-0 md:ml-2 md:mt-0 md:translate-x-0"
    >
      <div class="rounded-xl bg-surface p-4 shadow-[var(--shadow-pop)] ring-1 ring-black/5  ">
        <h4 class="mb-3 border-b border-line pb-2 text-sm font-bold text-foreground   flex items-center gap-2">
          <Icon name="brain" size="sm" class="text-accent-500" />
          {{ t('admin.ops.diagnosis.title') }}
        </h4>

        <div class="space-y-3">
          <div v-for="(item, idx) in diagnosisReport" :key="idx" class="flex gap-3">
            <div class="mt-0.5 shrink-0">
              <svg v-if="item.type === 'critical'" class="h-4 w-4 text-danger-500" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fill-rule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                  clip-rule="evenodd"
                />
              </svg>
              <svg v-else-if="item.type === 'warning'" class="h-4 w-4 text-warning-500" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fill-rule="evenodd"
                  d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z"
                  clip-rule="evenodd"
                />
              </svg>
              <svg v-else class="h-4 w-4 text-accent-500" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fill-rule="evenodd"
                  d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 100 2 1 1 0 000-2zm-1 3a1 1 0 012 0v4a1 1 0 11-2 0v-4z"
                  clip-rule="evenodd"
                />
              </svg>
            </div>
            <div class="flex-1">
              <div class="text-xs font-semibold text-foreground ">{{ item.message }}</div>
              <div class="mt-0.5 text-[11px] text-muted ">{{ item.impact }}</div>
              <div v-if="item.action" class="mt-1 text-[11px] text-accent  flex items-center gap-1">
                <Icon name="lightbulb" size="xs" />
                {{ item.action }}
              </div>
            </div>
          </div>
        </div>

        <div class="mt-3 border-t border-line pt-2 text-[10px] text-muted ">
          {{ t('admin.ops.diagnosis.footer') }}
        </div>
      </div>
    </div>

    <div class="relative flex items-center justify-center">
      <svg :width="circleSize" :height="circleSize" class="-rotate-90 transform">
        <circle
          :cx="circleSize / 2"
          :cy="circleSize / 2"
          :r="radius"
          :stroke-width="strokeWidth"
          fill="transparent"
          class="text-muted "
          stroke="currentColor"
        />
        <circle
          :cx="circleSize / 2"
          :cy="circleSize / 2"
          :r="radius"
          :stroke-width="strokeWidth"
          fill="transparent"
          :stroke="healthScoreColor"
          stroke-linecap="round"
          :stroke-dasharray="circumference"
          :stroke-dashoffset="dashOffset"
          class="transition-all duration-1000 ease-out"
        />
      </svg>

      <div class="absolute flex flex-col items-center">
        <span :class="[fullscreen ? 'text-5xl' : 'text-3xl', 'font-black', healthScoreClass]">
          {{ isSystemIdle ? t('admin.ops.idleStatus') : (healthScoreRaw ?? '--') }}
        </span>
        <span :class="[fullscreen ? 'text-xs' : 'text-[10px]', 'font-bold uppercase tracking-wider text-muted']">{{ t('admin.ops.health') }}</span>
      </div>
    </div>

    <div class="mt-4 text-center" v-if="!fullscreen">
      <div class="flex items-center justify-center gap-1 text-xs font-medium text-muted">
        {{ t('admin.ops.healthCondition') }}
        <HelpTooltip :content="t('admin.ops.healthHelp')" />
      </div>
      <div class="mt-1 text-xs font-bold" :class="healthScoreClass">
        {{
          isSystemIdle
            ? t('admin.ops.idleStatus')
            : typeof healthScoreRaw === 'number' && healthScoreRaw >= 90
              ? t('admin.ops.healthyStatus')
              : t('admin.ops.riskyStatus')
        }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import type { DiagnosisItem } from './useOpsHealthScore'

defineProps<{
  fullscreen?: boolean
  isSystemIdle: boolean
  healthScoreRaw: number | null | undefined
  healthScoreColor: string
  healthScoreClass: string
  circleSize: number
  strokeWidth: number
  radius: number
  circumference: number
  dashOffset: number
  diagnosisReport: DiagnosisItem[]
}>()

const { t } = useI18n()
</script>
