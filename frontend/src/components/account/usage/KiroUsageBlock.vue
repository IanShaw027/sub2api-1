<template>
  <div v-if="loading" class="space-y-1.5">
    <div class="flex items-center gap-1">
      <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
      <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
      <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
    </div>
  </div>
  <div v-else-if="error" class="text-xs text-danger-500">
    {{ error }}
  </div>
  <div v-else-if="usageInfo?.error" class="text-xs text-warning-text truncate max-w-[220px]" :title="usageInfo.error">
    {{ usageInfo.error }}
  </div>
  <div v-else-if="needsReauth" class="space-y-1">
    <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text">
      {{ t('admin.accounts.needsReauth') }}
    </span>
  </div>
  <div v-else-if="usageInfo?.kiro_quota" class="space-y-1">
    <UsageProgressBar
      label="30d"
      :utilization="usageInfo.kiro_quota.utilization"
      :resets-at="usageInfo.kiro_quota.resets_at"
      :window-stats="kiroQuotaStats"
      :show-empty-window-stats="true"
      color="cyan"
    />
    <div class="whitespace-nowrap text-[10px] text-muted">
      {{ t('admin.accounts.kiro.quotaCompact', {
        limit: formatKiroMoney(usageInfo.kiro_usage_limit),
        overage: kiroOverageDisplay,
        used: formatKiroMoney(usageInfo.kiro_current_usage)
      }) }}
    </div>
  </div>
  <div v-else class="text-xs text-muted">-</div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountUsageInfo } from '@/types'
import UsageProgressBar from '../UsageProgressBar.vue'
import { useKiroUsage } from './useKiroUsage'
import { useUsageErrorState } from './useUsageErrorState'

const props = defineProps<{
  usageInfo: AccountUsageInfo | null
  loading: boolean
  error: string | null
}>()

const { t } = useI18n()

const usageInfo = computed(() => props.usageInfo)

const { kiroQuotaStats, kiroOverageDisplay, formatKiroMoney } = useKiroUsage(usageInfo)
const { needsReauth } = useUsageErrorState(usageInfo)
</script>
