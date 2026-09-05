<template>
  <div v-if="subscriptions.length > 0">
    <p class="mb-2 text-xs font-medium text-muted">{{ t('payment.activeSubscription') }}</p>
    <div class="space-y-2">
      <div v-for="sub in subscriptions" :key="sub.id"
        class="flex items-center gap-3 rounded-xl border border-line bg-surface px-3 py-2">
        <div :class="['h-6 w-1 shrink-0 rounded-full', platformAccentBarClass(sub.group?.platform || '')]" />
        <div class="min-w-0 flex-1">
          <div class="flex items-center gap-1.5">
            <span class="truncate text-xs font-semibold text-foreground">{{ sub.group?.name || t('payment.groupFallback', { id: sub.group_id }) }}</span>
            <span :class="['shrink-0 rounded-full px-1.5 py-0.5 text-[9px] font-medium', platformBadgeLightClass(sub.group?.platform || '')]">{{ platformLabel(sub.group?.platform || '') }}</span>
          </div>
          <div class="flex flex-wrap gap-x-3 text-[11px] text-muted">
            <span>{{ t('payment.planCard.rate') }}: ×{{ sub.group?.rate_multiplier ?? 1 }}</span>
            <span v-if="subscriptionHasPeakRate(sub)">{{ t('payment.planCard.peakRate') }}: {{ subscriptionPeakRateLabel(sub) }}</span>
            <span v-if="sub.group?.daily_limit_usd == null && sub.group?.weekly_limit_usd == null && sub.group?.monthly_limit_usd == null">{{ t('payment.planCard.quota') }}: {{ t('payment.planCard.unlimited') }}</span>
            <span v-if="sub.expires_at">{{ t('userSubscriptions.daysRemaining', { days: getDaysRemaining(sub.expires_at) }) }}</span>
            <span v-else>{{ t('userSubscriptions.noExpiration') }}</span>
          </div>
        </div>
        <span class="badge badge-success shrink-0 text-[10px]">{{ t('userSubscriptions.status.active') }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { platformAccentBarClass, platformBadgeLightClass, platformLabel } from '@/utils/platformColors'
import type { UserSubscription } from '@/types'

const { t } = useI18n()

defineProps<{
  subscriptions: UserSubscription[]
  subscriptionHasPeakRate: (sub: UserSubscription) => boolean
  subscriptionPeakRateLabel: (sub: UserSubscription) => string
  getDaysRemaining: (expiresAt: string) => number
}>()
</script>
