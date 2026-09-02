<template>
  <AppLayout>
    <div class="space-y-6">
      <PageHeader :title="t('nav.paymentDashboard')">
        <template #actions>
          <div class="flex rounded-lg border border-line">
            <button
              v-for="d in DAYS_OPTIONS"
              :key="d"
              type="button"
              class="px-3 py-1.5 text-xs font-medium transition-colors first:rounded-l-lg last:rounded-r-lg"
              :class="days === d
 ? 'bg-accent text-white'
 : 'text-muted hover:bg-surface-2'"
              @click="days = d"
            >
              {{ d }}{{ t('payment.admin.daySuffix') }}
            </button>
          </div>
          <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="loadDashboard">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </Button>
        </template>
      </PageHeader>

      <!-- Dashboard Content -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>
      <template v-else-if="stats">
        <OrderStatsCards :stats="stats" />
        <DailyRevenueChart :data="stats.daily_series || []" :loading="loading" />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
          <GlassCard>
            <h3 class="mb-4 text-sm font-semibold text-foreground">{{ t('payment.admin.paymentDistribution') }}</h3>
            <div v-if="!stats.payment_methods?.length" class="flex h-32 items-center justify-center text-sm text-muted">{{ t('payment.admin.noData') }}</div>
            <div v-else class="space-y-3">
              <div v-for="method in stats.payment_methods" :key="method.type" class="flex items-center justify-between">
                <div class="flex items-center gap-2">
                  <span :class="['inline-block h-3 w-3 rounded-full', methodColor(method.type)]"></span>
                  <span class="text-sm text-foreground">{{ t('payment.methods.' + method.type, method.type) }}</span>
                </div>
                <div class="space-y-1 text-right">
                  <span v-for="[currency, amount] in sortedAmounts(method.amount)" :key="currency" class="block text-sm font-medium text-foreground">{{ formatMoney(currency, amount) }}</span>
                  <span class="ml-2 text-xs text-muted">({{ method.count }})</span>
                </div>
              </div>
            </div>
          </GlassCard>
          <GlassCard>
            <h3 class="mb-4 text-sm font-semibold text-foreground">{{ t('payment.admin.topUsers') }}</h3>
            <div v-if="!hasTopUsers(stats.top_users)" class="flex h-32 items-center justify-center text-sm text-muted">{{ t('payment.admin.noData') }}</div>
            <div v-else class="space-y-2">
              <div v-for="[currency, users] in sortedTopUsers(stats.top_users)" :key="currency" class="space-y-2">
                <p class="text-xs font-semibold text-muted">{{ currency }}</p>
                <div v-for="(user, idx) in users" :key="user.user_id" class="flex items-center justify-between rounded-lg px-3 py-2 hover:bg-surface-2">
                  <div class="flex items-center gap-3">
                    <span :class="['flex h-6 w-6 items-center justify-center rounded-full text-xs font-bold', rankClass(idx)]">{{ idx + 1 }}</span>
                    <span class="text-sm text-foreground">{{ user.email }}</span>
                  </div>
                  <span class="text-sm font-medium text-foreground">{{ formatMoney(currency, user.amount) }}</span>
                </div>
              </div>
            </div>
          </GlassCard>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { CurrencyAmounts, DashboardStats, TopUserPaymentStats } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatsCards from '@/components/admin/payment/OrderStatsCards.vue'
import DailyRevenueChart from '@/components/admin/payment/DailyRevenueChart.vue'

const { t } = useI18n()
const appStore = useAppStore()

const DAYS_OPTIONS = [7, 30, 90] as const
const days = ref<number>(30)
const loading = ref(false)
const stats = ref<DashboardStats | null>(null)

function methodColor(type: string): string {
  const c: Record<string, string> = {
    alipay: 'bg-blue-500', wxpay: 'bg-green-500',
    alipay_direct: 'bg-blue-400', wxpay_direct: 'bg-green-400',
    stripe: 'bg-purple-500',
  }
  return c[type] || 'bg-surface-3'
}

function rankClass(idx: number): string {
  if (idx === 0) return 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-yellow-700  '
  if (idx === 1) return 'bg-surface-3 text-muted  '
  if (idx === 2) return 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text  '
  return 'bg-surface-2 text-muted  '
}

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function sortedTopUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): [string, TopUserPaymentStats[]][] {
  return Object.entries(usersByCurrency).sort(([left], [right]) => left.localeCompare(right))
}

function hasTopUsers(usersByCurrency: Record<string, TopUserPaymentStats[]>): boolean {
  return Object.values(usersByCurrency).some(users => users.length > 0)
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}

async function loadDashboard() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getDashboard(days.value)
    stats.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

watch(days, () => loadDashboard())
onMounted(() => loadDashboard())
</script>
