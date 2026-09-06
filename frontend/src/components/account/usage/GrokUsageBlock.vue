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
  <div v-else-if="needsReauth" class="space-y-1">
    <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text">
      {{ t('admin.accounts.needsReauth') }}
    </span>
  </div>
  <div v-else-if="isForbidden" class="space-y-1">
    <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text">
      {{ usageInfo?.grok_entitlement_status || t('admin.accounts.forbidden') }}
    </span>
  </div>
  <div v-else-if="usageInfo" class="space-y-1">
    <!-- Free: only rolling 24h soft-gate bar. Paid: 7d + 30d + prepaid money. -->
    <template v-if="grokIsFree">
      <UsageProgressBar
        v-if="grokFreeTokenBar"
        label="24h"
        :title="t('admin.accounts.usageWindow.grokFreeQuota24hHint', { limit: formatCompactNumber(grokFreeTokenBar.limit) })"
        :utilization="grokFreeTokenBar.utilization"
        :window-stats="grokFreeQuotaUsage"
        :show-now-when-idle="true"
        color="emerald"
      />
      <div v-else-if="grokQuotaUnknown" class="text-[10px] text-muted">
        {{ grokQuotaUnknownLabel }}
      </div>
    </template>
    <template v-else>
      <UsageProgressBar
        v-if="grokWeeklyBillingBar"
        label="7d"
        :utilization="grokWeeklyBillingBar.utilization"
        :resets-at="grokWeeklyBillingBar.resetsAt"
        :window-stats="grokWeeklyBillingBar.windowStats"
        :predicted-total-cost="grokWeeklyBillingBar.predictedTotalCost"
        :show-now-when-idle="true"
        color="indigo"
      />
      <UsageProgressBar
        v-if="grokMonthlyBillingBar"
        label="30d"
        :utilization="grokMonthlyBillingBar.utilization"
        :resets-at="grokMonthlyBillingBar.resetsAt"
        :window-stats="grokMonthlyBillingBar.windowStats"
        :show-now-when-idle="true"
        color="indigo"
      />
      <div
        v-if="grokPrepaidMoneyLine"
        class="flex flex-wrap items-center gap-1 text-[10px] text-muted"
      >
        <span
          v-if="grokPrepaidMoneyLine.showPrepaid"
          class="rounded bg-[color-mix(in_oklch,var(--success)_16%,transparent)] px-1 py-0.5 text-success-text"
          :title="t('admin.accounts.usageWindow.grokPrepaid')"
        >
          {{ t('admin.accounts.usageWindow.grokPrepaid') }} ${{ grokPrepaidMoneyLine.prepaid }}
        </span>
        <span
          v-if="grokPrepaidMoneyLine.showUsedLimit"
          :title="t('admin.accounts.usageWindow.grokMonthlyLimit')"
        >
          {{ t('admin.accounts.usageWindow.grokUsed') }}
          {{ grokPrepaidMoneyLine.used }}/{{ grokPrepaidMoneyLine.limit }}
        </span>
      </div>
      <div v-if="grokQuotaUnknown" class="text-[10px] text-muted">
        {{ grokQuotaUnknownLabel }}
      </div>
    </template>
    <div v-if="usageInfo.error" class="truncate text-xs text-warning-text max-w-[200px]" :title="usageInfo.error">
      {{ usageErrorLabel }}
    </div>
    <div v-if="grokRetryAfterLabel" class="text-[10px] text-warning-text">
      {{ t('admin.accounts.usageWindow.grokRetryAfter', { time: grokRetryAfterLabel }) }}
    </div>
    <GrokQuotaProbeCell :account="account" compact @probed="handleGrokProbed" />
  </div>
  <div v-else class="space-y-1">
    <div class="text-xs text-muted">-</div>
    <GrokQuotaProbeCell :account="account" compact @probed="handleGrokProbed" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountUsageInfo } from '@/types'
import UsageProgressBar from '../UsageProgressBar.vue'
import GrokQuotaProbeCell from '../GrokQuotaProbeCell.vue'
import { useGrokUsage } from './useGrokUsage'
import { useUsageErrorState } from './useUsageErrorState'

const props = defineProps<{
  account: Account
  usageInfo: AccountUsageInfo | null
  loading: boolean
  error: string | null
}>()

const emit = defineEmits<{
  probed: []
}>()

const { t } = useI18n()

const account = computed(() => props.account)
const usageInfo = computed(() => props.usageInfo)

const {
  grokWeeklyBillingBar,
  grokMonthlyBillingBar,
  grokPrepaidMoneyLine,
  grokIsFree,
  grokFreeQuotaUsage,
  grokFreeTokenBar,
  grokQuotaUnknown,
  grokQuotaUnknownLabel,
  grokRetryAfterLabel,
  formatCompactNumber
} = useGrokUsage(account, usageInfo)

const { isForbidden, needsReauth, usageErrorLabel } = useUsageErrorState(usageInfo)

// The probe persists upstream quota state; ask the parent cell to refresh so this
// block's compact bars and entitlement status reflect the newly observed snapshot.
const handleGrokProbed = () => {
  emit('probed')
}
</script>
