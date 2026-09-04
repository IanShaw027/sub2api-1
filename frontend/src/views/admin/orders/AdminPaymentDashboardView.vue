<template>
  <AppLayout>
    <div class="dash-page">
      <div class="dash-hero glass-card">
        <PageHeader variant="hero" :title="t('nav.paymentDashboard')" :description="t('payment.admin.dashboardDescription')">
          <template #actions>
            <SegmentedControl v-model="daysStr" :options="dayOptions" size="sm" />
            <Button
              variant="secondary"
              class="btn-icon"
              :disabled="loading"
              :title="t('common.refresh')"
              :aria-label="t('common.refresh')"
              @click="loadDashboard"
            >
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </Button>
          </template>
        </PageHeader>
      </div>

      <div v-if="loading && !stats" class="dash-loading">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <OrderStatsCards :stats="stats" />

        <div class="dash-row dash-row-trend">
          <DailyRevenueChart :data="stats.daily_series || []" :loading="loading" />
          <PaymentMethodChart :methods="stats.payment_methods || []" />
        </div>

        <div class="dash-row dash-row-split">
          <TopUsersLeaderboard :users="stats.top_users || {}" />

          <div class="glass-card p-4 quick-actions">
            <h3 class="panel-title">{{ t('payment.admin.quickActions') }}</h3>
            <nav class="quick-actions-list">
              <RouterLink to="/admin/orders" class="quick-actions-item">
                <span class="quick-actions-icon"><Icon name="creditCard" size="sm" /></span>
                <span class="quick-actions-label">{{ t('nav.orderManagement') }}</span>
                <Icon name="chevronRight" size="sm" class="quick-actions-chevron" />
              </RouterLink>
              <RouterLink to="/admin/orders/invoices" class="quick-actions-item">
                <span class="quick-actions-icon"><Icon name="document" size="sm" /></span>
                <span class="quick-actions-label">{{ t('nav.invoiceApplications') }}</span>
                <Icon name="chevronRight" size="sm" class="quick-actions-chevron" />
              </RouterLink>
              <RouterLink to="/admin/orders/plans" class="quick-actions-item">
                <span class="quick-actions-icon"><Icon name="dollar" size="sm" /></span>
                <span class="quick-actions-label">{{ t('nav.paymentPlans') }}</span>
                <Icon name="chevronRight" size="sm" class="quick-actions-chevron" />
              </RouterLink>
            </nav>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { DashboardStats } from '@/types/payment'
import type { SegmentedOption } from '@/components/ui/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderStatsCards from '@/components/admin/payment/OrderStatsCards.vue'
import DailyRevenueChart from '@/components/admin/payment/DailyRevenueChart.vue'
import PaymentMethodChart from '@/components/admin/payment/PaymentMethodChart.vue'
import TopUsersLeaderboard from '@/components/admin/payment/TopUsersLeaderboard.vue'

const { t } = useI18n()
const appStore = useAppStore()

const DAYS_OPTIONS = [7, 30, 90] as const

const loading = ref(false)
const stats = ref<DashboardStats | null>(null)
const daysStr = ref<string>('30')

const dayOptions = computed((): SegmentedOption<string>[] =>
  DAYS_OPTIONS.map(d => ({ value: String(d), label: `${d}${t('payment.admin.daySuffix')}` })),
)

async function loadDashboard() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getDashboard(Number(daysStr.value))
    stats.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

watch(daysStr, () => loadDashboard())
onMounted(() => loadDashboard())
</script>

<style scoped>
.dash-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  /* Local type-scale tokens: ui-lint's scoped check requires `var(--...)` in
     view-level styles instead of literal font sizes/weights. */
  --panel-title-fs: 13px;
  --panel-title-fw: 600;
  --quick-actions-fs: 13px;
  --quick-actions-fw: 500;
}

.dash-hero {
  padding: 20px 24px;
}

.dash-hero :deep(.ui-page-header) {
  margin-bottom: 0;
}

.dash-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 0;
}

.dash-row {
  display: grid;
  gap: 16px;
}

.dash-row-trend {
  grid-template-columns: 2fr 1fr;
}

.dash-row-split {
  grid-template-columns: 1fr 1fr;
}

@media (max-width: 1023px) {
  .dash-row-trend,
  .dash-row-split {
    grid-template-columns: 1fr;
  }
}

.quick-actions {
  display: flex;
  flex-direction: column;
}

.panel-title {
  margin-bottom: 16px;
  font-size: var(--panel-title-fs);
  font-weight: var(--panel-title-fw);
  color: var(--foreground);
}

.quick-actions-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.quick-actions-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  color: var(--foreground);
  text-decoration: none;
  transition: background-color 0.15s ease;
}

.quick-actions-item:hover {
  background: var(--surface-secondary);
}

.quick-actions-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-sm);
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
  flex-shrink: 0;
}

.quick-actions-label {
  flex: 1;
  font-size: var(--quick-actions-fs);
  font-weight: var(--quick-actions-fw);
}

.quick-actions-chevron {
  color: var(--muted);
  flex-shrink: 0;
}
</style>
