<template>
  <!-- Auth Type + Tier Badge (first line) -->
  <div v-if="geminiAuthTypeLabel" class="mb-1 flex items-center gap-1">
    <span
      :class="[
        'inline-block rounded px-1.5 py-0.5 text-[10px] font-medium',
        geminiTierClass
      ]"
    >
      {{ geminiAuthTypeLabel }}
    </span>
    <!-- Help icon -->
    <span
      class="group relative cursor-help"
    >
      <svg
        class="h-3.5 w-3.5 text-muted hover:text-muted"
        fill="currentColor"
        viewBox="0 0 20 20"
      >
        <path
          fill-rule="evenodd"
          d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z"
          clip-rule="evenodd"
        />
      </svg>
      <span
        class="pointer-events-none absolute left-0 top-full z-50 mt-1 w-80 whitespace-normal break-words rounded bg-[var(--code-bg)] px-3 py-2 text-xs leading-relaxed text-white opacity-0 shadow-[var(--shadow-pop)] transition-opacity group-hover:opacity-100"
      >
        <div class="font-semibold mb-1">{{ t('admin.accounts.gemini.quotaPolicy.title') }}</div>
        <div class="mb-2 text-muted">{{ t('admin.accounts.gemini.quotaPolicy.note') }}</div>
        <div class="space-y-1">
          <div><strong>{{ geminiQuotaPolicyChannel }}:</strong></div>
          <div class="pl-2">• {{ geminiQuotaPolicyLimits }}</div>
          <div class="mt-2">
            <a :href="geminiQuotaPolicyDocsUrl" target="_blank" rel="noopener noreferrer" class="text-accent-400 hover:text-accent-300 underline">
              {{ t('admin.accounts.gemini.quotaPolicy.columns.docs') }} →
            </a>
          </div>
        </div>
      </span>
    </span>
  </div>

  <!-- Usage data or unlimited flow -->
  <div class="space-y-1">
    <div
      v-if="showGeminiTodayStats && todayStats"
      class="mb-0.5 flex items-center"
    >
      <div class="flex items-center gap-1.5 text-[9px] text-muted">
        <span class="rounded bg-surface-2 px-1.5 py-0.5">
          {{ formatKeyRequests }} req
        </span>
        <span class="rounded bg-surface-2 px-1.5 py-0.5">
          {{ formatKeyTokens }}
        </span>
        <span class="rounded bg-surface-2 px-1.5 py-0.5" :title="t('usage.accountBilled')">
          A ${{ formatKeyCost }}
        </span>
        <span
          v-if="todayStats.user_cost != null"
          class="rounded bg-surface-2 px-1.5 py-0.5"
          :title="t('usage.userBilled')"
        >
          U ${{ formatKeyUserCost }}
        </span>
      </div>
    </div>
    <div
      v-else-if="showGeminiTodayStats && todayStatsLoading"
      class="mb-0.5 flex items-center gap-1"
    >
      <div class="h-3 w-10 animate-pulse rounded bg-surface-3"></div>
      <div class="h-3 w-8 animate-pulse rounded bg-surface-3"></div>
      <div class="h-3 w-12 animate-pulse rounded bg-surface-3"></div>
    </div>
    <div v-if="loading" class="space-y-1">
      <div class="flex items-center gap-1">
        <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
        <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
        <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
      </div>
    </div>
    <div v-else-if="error" class="text-xs text-danger-500">
      {{ error }}
    </div>
    <!-- Gemini: show daily usage bars when available -->
    <div v-else-if="geminiUsageAvailable" class="space-y-1">
      <UsageProgressBar
        v-for="bar in geminiUsageBars"
        :key="bar.key"
        :label="bar.label"
        :utilization="bar.utilization"
        :resets-at="bar.resetsAt"
        :window-stats="bar.windowStats"
        :color="bar.color"
      />
      <p class="mt-1 text-[9px] leading-tight text-muted italic">
        * {{ t('admin.accounts.gemini.quotaPolicy.simulatedNote') || 'Simulated quota' }}
      </p>
    </div>
    <!-- AI Studio Client OAuth: show unlimited flow (no usage tracking) -->
    <div v-else class="text-xs text-muted">
      {{ t('admin.accounts.gemini.rateLimit.unlimited') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountUsageInfo, WindowStats } from '@/types'
import UsageProgressBar from '../UsageProgressBar.vue'
import { useGeminiUsage } from './useGeminiUsage'
import { useTodayStatsFormat } from './useTodayStatsFormat'

const props = defineProps<{
  account: Account
  usageInfo: AccountUsageInfo | null
  loading: boolean
  error: string | null
  todayStats?: WindowStats | null
  todayStatsLoading?: boolean
}>()

const { t } = useI18n()

const account = computed(() => props.account)
const usageInfo = computed(() => props.usageInfo)
const todayStats = computed(() => props.todayStats)

const {
  showGeminiTodayStats,
  geminiUsageAvailable,
  geminiAuthTypeLabel,
  geminiTierClass,
  geminiQuotaPolicyChannel,
  geminiQuotaPolicyLimits,
  geminiQuotaPolicyDocsUrl,
  geminiUsageBars
} = useGeminiUsage(account, usageInfo)

const { formatKeyRequests, formatKeyTokens, formatKeyCost, formatKeyUserCost } = useTodayStatsFormat(todayStats)
</script>
