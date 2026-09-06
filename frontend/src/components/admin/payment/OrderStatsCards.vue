<template>
  <div class="stats-grid">
    <StatCard
      :label="t('payment.admin.todayRevenue')"
      :value="primaryAmountText(stats.today_amount)"
      :sub="statSub(stats.today_amount, stats.today_count)"
    />
    <StatCard
      :label="t('payment.admin.totalRevenue')"
      :value="primaryAmountText(stats.total_amount)"
      :sub="statSub(stats.total_amount, stats.total_count)"
    />
    <StatCard
      :label="t('payment.admin.todayOrders')"
      :value="(stats.today_count ?? 0).toLocaleString()"
      :sub="t('payment.admin.orders')"
    />
    <StatCard
      :label="t('payment.admin.avgAmount')"
      :value="primaryAmountText(stats.avg_amount)"
      :sub="extraAmountsText(stats.avg_amount)"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import StatCard from '@/components/ui/StatCard.vue'
import type { CurrencyAmounts, DashboardStats } from '@/types/payment'

const { t } = useI18n()

defineProps<{
  stats: DashboardStats
}>()

function sortedAmounts(amounts: CurrencyAmounts): [string, number][] {
  if (!amounts || typeof amounts !== 'object') return []
  return Object.entries(amounts).sort(([left], [right]) => left.localeCompare(right))
}

function formatMoney(currency: string, amount: number): string {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(amount)
}

function primaryAmountText(amounts: CurrencyAmounts): string {
  const [first] = sortedAmounts(amounts)
  return first ? formatMoney(first[0], first[1]) : '—'
}

function extraAmountsText(amounts: CurrencyAmounts): string | undefined {
  const rest = sortedAmounts(amounts).slice(1)
  if (!rest.length) return undefined
  return rest.map(([currency, amount]) => formatMoney(currency, amount)).join(' · ')
}

function statSub(amounts: CurrencyAmounts, count: number): string {
  const extra = extraAmountsText(amounts)
  const base = `${(count ?? 0).toLocaleString()} ${t('payment.admin.orders')}`
  return extra ? `${base} · ${extra}` : base
}
</script>

<style scoped>
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

@media (max-width: 1023px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 639px) {
  .stats-grid {
    grid-template-columns: 1fr;
  }
}
</style>
