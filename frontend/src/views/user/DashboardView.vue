<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Calm loading skeleton for KPI strip -->
      <div v-if="loading" class="space-y-4">
        <div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
          <div v-for="n in 4" :key="`s1-${n}`" class="stat-card min-w-0">
            <Skeleton variant="rect" :width="32" :height="32" class="shrink-0 rounded-control" />
            <div class="min-w-0 flex-1 space-y-2">
              <Skeleton variant="text" width="45%" height="12px" />
              <Skeleton variant="text" width="60%" height="22px" />
              <Skeleton variant="text" width="35%" height="11px" />
            </div>
          </div>
        </div>
        <div class="grid grid-cols-2 gap-4 xl:grid-cols-4">
          <div v-for="n in 4" :key="`s2-${n}`" class="stat-card min-w-0">
            <Skeleton variant="rect" :width="32" :height="32" class="shrink-0 rounded-control" />
            <div class="min-w-0 flex-1 space-y-2">
              <Skeleton variant="text" width="45%" height="12px" />
              <Skeleton variant="text" width="60%" height="22px" />
              <Skeleton variant="text" width="50%" height="11px" />
            </div>
          </div>
        </div>
      </div>
      <template v-else-if="stats">
        <UserDashboardStats
          :stats="stats"
          :balance="user?.balance || 0"
          :is-simple="authStore.isSimpleMode"
          :platform-quotas="platformQuotas"
          @balance-history="showBalanceHistory = true"
        />
        <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :trend="trendData" :models="modelStats" @dateRangeChange="loadCharts" @granularityChange="loadCharts" @refresh="refreshAll" />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="lg:col-span-2"><UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" /></div>
          <div class="lg:col-span-1"><UserDashboardQuickActions /></div>
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
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import { getMyPlatformQuotas } from '@/api/user'
import AppLayout from '@/components/layout/AppLayout.vue'
import Skeleton from '@/components/common/Skeleton.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import UserBalanceHistoryModal from '@/components/user/UserBalanceHistoryModal.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { formatDateLocalInput } from '@/utils/format'

const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const loading = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
const showBalanceHistory = ref(false)

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateLocalInput(new Date()))
const granularity = ref('day')

const loadStats = async () => {
  loading.value = true
  try {
    const [, dashboardStats] = await Promise.all([
      authStore.refreshUser({ touchActive: true }),
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
    const res = await usageAPI.getByDateRange(startDate.value, endDate.value, undefined, 5)
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

const refreshAll = () => {
  loadStats()
  loadCharts()
  loadRecent()
  loadPlatformQuotas()
}

onMounted(() => {
  refreshAll()
})
</script>
