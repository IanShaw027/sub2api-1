<template>
  <AppLayout>
    <div class="dash-page">
      <PageHeader :title="t('dashboard.title')" :description="t('dashboard.welcomeMessage')">
        <template #actions>
          <DateRangePicker
            :start-date="startDate"
            :end-date="endDate"
            @update:startDate="startDate = $event"
            @update:endDate="endDate = $event"
            @change="loadCharts"
          />
          <button type="button" class="dash-refresh-btn" :disabled="loadingCharts" @click="refreshAll">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingCharts }" />
            {{ t('common.refresh') }}
          </button>
        </template>
      </PageHeader>
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <UserDashboardStats
          :stats="stats"
          :balance="user?.balance || 0"
          :is-simple="authStore.isSimpleMode"
          :platform-quotas="platformQuotas"
          :live-rpm-used="liveRpm?.user_rpm_used"
          :live-rpm-limit="liveRpm?.user_rpm_limit"
          :current-concurrency="liveRpm?.current_concurrency"
          :trend="trendData"
          @balance-history="showBalanceHistory = true"
        />
        <UserDashboardCharts
          v-model:granularity="granularity"
          :loading="loadingCharts"
          :trend="trendData"
          :models="modelStats"
          @granularityChange="loadCharts"
        />
        <div class="dash-row-2-1">
          <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" />
          <UserDashboardQuickActions />
        </div>
        <UserBalanceHistoryModal
          :show="showBalanceHistory"
          :email="user?.email"
          :balance="user?.balance || 0"
          @close="showBalanceHistory = false"
        />
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserBalanceHistoryModal from '@/components/user/UserBalanceHistoryModal.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas, getMyRPMStatus, type UserRPMStatus } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'

const authStore = useAuthStore()
const { t } = useI18n()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
const liveRpm = ref<UserRPMStatus | null>(null)
const showBalanceHistory = ref(false)

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateLocalInput(new Date()))
const granularity = ref('day')

const loadStats = async () => {
  loading.value = true
  try {
    const [, dashboardStats] = await Promise.all([
      authStore.refreshUser(),
      usageAPI.getDashboardStats(),
    ])
    stats.value = dashboardStats
  } catch (error) {
    console.error('Failed to load dashboard stats:', error)
  } finally {
    loading.value = false
  }
}

const loadCharts = async () => {
  loadingCharts.value = true
  try {
    const res = await Promise.all([
      usageAPI.getDashboardTrend({
        start_date: startDate.value,
        end_date: endDate.value,
        granularity: granularity.value as any,
      }),
      usageAPI.getDashboardModels({
        start_date: startDate.value,
        end_date: endDate.value,
      }),
    ])
    trendData.value = res[0].trend || []
    modelStats.value = res[1].models || []
  } catch (error) {
    console.error('Failed to load charts:', error)
  } finally {
    loadingCharts.value = false
  }
}

const loadRecent = async () => {
  loadingUsage.value = true
  try {
    const res = await usageAPI.getByDateRange(startDate.value, endDate.value)
    recentUsage.value = res.items.slice(0, 5)
  } catch (error) {
    console.error('Failed to load recent usage:', error)
  } finally {
    loadingUsage.value = false
  }
}

const loadPlatformQuotas = async () => {
  try {
    const data = await getMyPlatformQuotas()
    platformQuotas.value = data.platform_quotas ?? []
  } catch (error) {
    console.warn('Failed to load platform quotas:', error)
    platformQuotas.value = []
  }
}

const loadLiveRpm = async () => {
  try {
    liveRpm.value = await getMyRPMStatus()
  } catch (error) {
    console.warn('Failed to load live RPM:', error)
  }
}

const refreshAll = () => {
  loadStats()
  loadCharts()
  loadRecent()
  loadPlatformQuotas()
  loadLiveRpm()
}

onMounted(() => {
  refreshAll()
})
</script>
<style scoped>
.dash-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.dash-refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: 10px;
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--foreground);
  font-size: 13px;
  font-weight: 600;
  border: 1px solid var(--border);
  cursor: pointer;
  box-shadow: inset 0 1px 0 var(--btn-hi), 0 1px 2px rgba(16, 24, 40, 0.06);
  transition: background 0.15s ease, border-color 0.15s ease, transform 0.1s ease;
  flex: none;
  white-space: nowrap;
}

.dash-refresh-btn:hover:not(:disabled) {
  background: var(--surface);
  border-color: color-mix(in oklch, var(--foreground) 18%, transparent);
}

.dash-refresh-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.dash-refresh-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.dash-row-2-1 {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 12px;
  align-items: start;
}

@media (max-width: 1023px) {
  .dash-row-2-1 {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 767px) {
  .dash-page :deep(.ui-page-header-title) {
    font-size: 20px;
  }
}
</style>
