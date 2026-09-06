<template>
  <AppLayout>
    <PageHeader :title="t('userSubscriptions.title')" :description="t('userSubscriptions.description')" />
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-accent border-t-transparent"
        ></div>
      </div>

      <EmptyState
        v-else-if="subscriptions.length === 0"
        data-testid="subscriptions-empty"
        :title="t('userSubscriptions.noActiveSubscriptions')"
        :description="t('userSubscriptions.noActiveSubscriptionsDesc')"
        size="lg"
      />

      <div v-else class="subscription-grid">
        <GlassCard
          v-for="subscription in subscriptions"
          :key="subscription.id"
          padding="sm"
          data-testid="subscription-card"
          :class="[
            'subscription-card',
            platformBorderClass(subscription.group?.platform || ''),
            subscription.status !== 'active' ? 'subscription-card-muted' : null
          ]"
        >
          <div
            class="flex items-center justify-between border-b border-line p-4"
          >
            <div class="flex items-center gap-3">
              <div :class="['h-1.5 w-1.5 shrink-0 rounded-full', platformAccentDotClass(subscription.group?.platform || '')]" />
              <div>
                <div class="flex items-center gap-2">
                  <h3 class="font-semibold text-foreground">
                    {{ subscription.group?.name || `Group #${subscription.group_id}` }}
                  </h3>
                  <span :class="['rounded-md border px-2 py-0.5 text-[11px] font-medium', platformBadgeClass(subscription.group?.platform || '')]">
                    {{ platformLabel(subscription.group?.platform || '') }}
                  </span>
                </div>
                <p v-if="subscription.group?.description" class="mt-0.5 text-xs text-muted">
                  {{ subscription.group.description }}
                </p>
                <div class="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-muted">
                  <span>{{ t('payment.planCard.rate') }}: ×{{ subscription.group?.rate_multiplier ?? 1 }}</span>
                  <span v-if="subscriptionHasPeakRate(subscription)" class="text-warning-text">
                    {{ t('payment.planCard.peakRate') }}: {{ subscriptionPeakRateLabel(subscription) }}
                  </span>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-2">
              <StatusBadge
                :tone="subscription.status === 'active' ? 'success' : subscription.status === 'expired' ? 'muted' : 'danger'"
                :label="t(`userSubscriptions.status.${subscription.status}`)"
                dot
              />
              <Button
                v-if="subscription.status === 'active'"
                size="sm"
                @click="router.push({ path: '/purchase', query: { tab: 'subscription', group: String(subscription.group_id) } })"
              >
                {{ t('payment.renewNow') }}
              </Button>
            </div>
          </div>

          <div class="space-y-4 p-4">
            <div v-if="subscription.expires_at" class="flex items-center justify-between text-sm">
              <span class="text-muted">{{
                t('userSubscriptions.expires')
              }}</span>
              <span class="font-mono tabular-nums" :class="getExpirationClass(subscription.expires_at)">
                {{ formatExpirationDate(subscription.expires_at) }}
              </span>
            </div>
            <div v-else class="flex items-center justify-between text-sm">
              <span class="text-muted">{{
                t('userSubscriptions.expires')
              }}</span>
              <span class="text-foreground">{{
                t('userSubscriptions.noExpiration')
              }}</span>
            </div>

            <div v-if="subscription.group?.daily_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-foreground">
                  {{ t('userSubscriptions.daily') }}
                </span>
                <span class="font-mono tabular-nums text-sm text-muted">
                  ${{ (subscription.daily_usage_usd || 0).toFixed(2) }} / ${{
                    subscription.group.daily_limit_usd.toFixed(2)
                  }}
                </span>
              </div>
              <div
                class="usage-progress-track"
                role="progressbar"
                :aria-valuenow="Math.round(dailyUsagePct(subscription))"
                aria-valuemin="0"
                aria-valuemax="100"
              >
                <div
                  class="usage-progress-fill"
                  :style="{ width: `${dailyUsagePct(subscription)}%`, background: usageProgressColor(dailyUsagePct(subscription)) }"
                ></div>
              </div>
              <p
                v-if="subscription.daily_window_start"
                class="text-xs text-muted"
              >
                {{ formatDailyUsageWindow(subscription) }}
              </p>
            </div>

            <div v-if="subscription.group?.weekly_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-foreground">
                  {{ t('userSubscriptions.weekly') }}
                </span>
                <span class="font-mono tabular-nums text-sm text-muted">
                  ${{ (subscription.weekly_usage_usd || 0).toFixed(2) }} / ${{
                    subscription.group.weekly_limit_usd.toFixed(2)
                  }}
                </span>
              </div>
              <div
                class="usage-progress-track"
                role="progressbar"
                :aria-valuenow="Math.round(weeklyUsagePct(subscription))"
                aria-valuemin="0"
                aria-valuemax="100"
              >
                <div
                  class="usage-progress-fill"
                  :style="{ width: `${weeklyUsagePct(subscription)}%`, background: usageProgressColor(weeklyUsagePct(subscription)) }"
                ></div>
              </div>
              <p
                v-if="subscription.weekly_window_start"
                class="text-xs text-muted"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.weekly_window_start, 168)
                  })
                }}
              </p>
            </div>

            <div v-if="subscription.group?.monthly_limit_usd" class="space-y-2">
              <div class="flex items-center justify-between">
                <span class="text-sm font-medium text-foreground">
                  {{ t('userSubscriptions.monthly') }}
                </span>
                <span class="font-mono tabular-nums text-sm text-muted">
                  ${{ (subscription.monthly_usage_usd || 0).toFixed(2) }} / ${{
                    subscription.group.monthly_limit_usd.toFixed(2)
                  }}
                </span>
              </div>
              <div
                class="usage-progress-track"
                role="progressbar"
                :aria-valuenow="Math.round(monthlyUsagePct(subscription))"
                aria-valuemin="0"
                aria-valuemax="100"
              >
                <div
                  class="usage-progress-fill"
                  :style="{ width: `${monthlyUsagePct(subscription)}%`, background: usageProgressColor(monthlyUsagePct(subscription)) }"
                ></div>
              </div>
              <p
                v-if="subscription.monthly_window_start"
                class="text-xs text-muted"
              >
                {{
                  t('userSubscriptions.resetIn', {
                    time: formatResetTime(subscription.monthly_window_start, 720)
                  })
                }}
              </p>
            </div>

            <div
              v-if="
                !subscription.group?.daily_limit_usd &&
                !subscription.group?.weekly_limit_usd &&
                !subscription.group?.monthly_limit_usd
              "
              class="flex items-center justify-center rounded-xl bg-surface-2 py-6"
            >
              <div class="flex items-center gap-3">
                <span class="text-4xl text-success-text">∞</span>
                <div>
                  <p class="text-sm font-medium text-success-text">
                    {{ t('userSubscriptions.unlimited') }}
                  </p>
                  <p class="text-xs text-muted">
                    {{ t('userSubscriptions.unlimitedDesc') }}
                  </p>
                </div>
              </div>
            </div>
          </div>
        </GlassCard>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'
import subscriptionsAPI from '@/api/subscriptions'
import type { UserSubscription } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import { formatDateTimeToMinute } from '@/utils/format'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { platformBorderClass, platformBadgeClass, platformLabel } from '@/utils/platformColors'
import {
  getExpirationDateRelation,
  getRemainingDurationParts,
  isOneTimeDailyQuota,
  type RemainingDurationParts
} from '@/utils/subscriptionQuota'

function platformAccentDotClass(p: string): string {
  switch (p) {
    case 'anthropic': return 'bg-warning-500'
    case 'openai': return 'bg-success-500'
    case 'antigravity': return 'bg-accent-500'
    case 'gemini': return 'bg-accent-500'
    default: return 'bg-surface-3'
  }
}

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const subscriptions = ref<UserSubscription[]>([])
const loading = ref(true)

function usageProgressColor(pct: number): string {
  if (pct > 95) return 'var(--danger)'
  if (pct > 80) return 'var(--warning)'
  return 'var(--accent)'
}

function usagePct(used: number | undefined, limit: number | undefined | null): number {
  if (!limit) return 0
  return Math.min(((used || 0) / limit) * 100, 100)
}

function dailyUsagePct(subscription: UserSubscription): number {
  return usagePct(subscription.daily_usage_usd, subscription.group?.daily_limit_usd)
}

function weeklyUsagePct(subscription: UserSubscription): number {
  return usagePct(subscription.weekly_usage_usd, subscription.group?.weekly_limit_usd)
}

function monthlyUsagePct(subscription: UserSubscription): number {
  return usagePct(subscription.monthly_usage_usd, subscription.group?.monthly_limit_usd)
}

function subscriptionHasPeakRate(subscription: UserSubscription): boolean {
  return hasPeakRate(subscription.group)
}

function subscriptionPeakRateLabel(subscription: UserSubscription): string {
  return formatPeakRateWindow(subscription.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
}

async function loadSubscriptions() {
  try {
    loading.value = true
    subscriptions.value = await subscriptionsAPI.getMySubscriptions()
  } catch (error) {
    console.error('Failed to load subscriptions:', error)
    appStore.showError(t('userSubscriptions.failedToLoad'))
  } finally {
    loading.value = false
  }
}

function formatExpirationDate(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))
  const relation = getExpirationDateRelation(expires, now)

  if (relation === null) return ''

  if (relation === 'expired') {
    return t('userSubscriptions.status.expired')
  }

  const dateStr = formatDateTimeToMinute(expires)

  if (relation === 'today') {
    return `${dateStr} (${t('common.today')})`
  }
  if (relation === 'tomorrow') {
    return `${dateStr} (${t('common.tomorrow')})`
  }

  return t('userSubscriptions.daysRemaining', { days }) + ` (${dateStr})`
}

function getExpirationClass(expiresAt: string): string {
  const now = new Date()
  const expires = new Date(expiresAt)
  const diff = expires.getTime() - now.getTime()
  const days = Math.ceil(diff / (1000 * 60 * 60 * 24))

  if (diff <= 0) return 'text-danger-text  font-medium'
  if (days <= 3) return 'text-danger-text '
  if (days <= 7) return 'text-warning-text '
  return 'text-foreground '
}

function formatDurationParts(parts: RemainingDurationParts): string {
  if (parts.days > 0) {
    return `${parts.days}d ${parts.hours}h`
  }

  if (parts.hours > 0) {
    return `${parts.hours}h ${parts.minutes}m`
  }

  return `${parts.minutes}m`
}

function formatDailyUsageWindow(subscription: UserSubscription): string {
  if (isOneTimeDailyQuota(subscription) && subscription.expires_at) {
    const parts = getRemainingDurationParts(subscription.expires_at)
    if (!parts) return t('userSubscriptions.windowNotActive')
    return t('userSubscriptions.quotaEndsIn', { time: formatDurationParts(parts) })
  }

  return t('userSubscriptions.resetIn', {
    time: formatResetTime(subscription.daily_window_start, 24)
  })
}

function formatResetTime(windowStart: string | null, windowHours: number): string {
  if (!windowStart) return t('userSubscriptions.windowNotActive')

  const start = new Date(windowStart)
  const end = new Date(start.getTime() + windowHours * 60 * 60 * 1000)
  const parts = getRemainingDurationParts(end)

  return parts ? formatDurationParts(parts) : t('userSubscriptions.windowNotActive')
}

onMounted(() => {
  loadSubscriptions()
})
</script>

<style scoped>
.subscription-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 1024px) {
  .subscription-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .subscription-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

.subscription-card {
  border-radius: var(--radius-hero);
  overflow: hidden;
}

.subscription-card-muted {
  opacity: 0.6;
}

.usage-progress-track {
  height: 6px;
  border-radius: var(--radius-card);
  background: var(--surface-2);
  overflow: hidden;
}

.usage-progress-fill {
  height: 100%;
  border-radius: var(--radius-card);
  transition: width 0.3s ease;
}
</style>
