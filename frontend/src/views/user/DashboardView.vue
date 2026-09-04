<template>
  <AppLayout>
    <div class="dash-page">
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <!-- ============================= 1 · Hero ============================= -->
        <section class="glass-card dash-hero">
          <span class="dash-hero-deco" aria-hidden="true">
            <span class="dash-hero-dots"></span>
            <span class="dash-hero-orb dash-hero-orb-accent"></span>
            <span class="dash-hero-orb dash-hero-orb-success"></span>
          </span>

          <div class="dash-hero-copy">
            <span class="dash-hero-kicker">{{ heroKicker }}</span>
            <h1 class="dash-hero-title">
              {{ t('dashboard.heroTitleLead') }}<span class="dash-hero-accent">{{ formatNumber(stats.today_requests) }}</span>{{ t('dashboard.heroTitleTail') }}
            </h1>
            <p class="dash-hero-desc">{{ heroDescription }}</p>
            <div class="dash-hero-actions">
              <Button @click="router.push('/usage')">{{ t('dashboard.viewUsage') }}</Button>
              <Button variant="secondary" @click="router.push('/keys')">{{ t('dashboard.createApiKey') }}</Button>
            </div>
          </div>

          <div class="dash-hero-side">
            <div class="dash-hero-tools">
              <DateRangePicker
                class="dash-daterange"
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
            </div>

            <div class="dash-hero-stats">
              <div
                class="dash-hero-mini glass-inset"
                :class="{ 'dash-hero-mini-clickable': !authStore.isSimpleMode }"
                @click="!authStore.isSimpleMode && (showBalanceHistory = true)"
              >
                <template v-if="!authStore.isSimpleMode">
                  <span class="dash-mini-label">{{ t('dashboard.balance') }}</span>
                  <span class="dash-mini-value">${{ formatBalance(user?.balance || 0) }}</span>
                  <span class="dash-mini-sub">{{ t('common.available') }}</span>
                </template>
                <template v-else>
                  <span class="dash-mini-label">{{ t('dashboard.apiKeys') }}</span>
                  <span class="dash-mini-value">{{ stats.total_api_keys }}</span>
                  <span class="dash-mini-sub">{{ stats.active_api_keys }} {{ t('common.active') }}</span>
                </template>
              </div>
              <div class="dash-hero-mini glass-inset">
                <span class="dash-mini-label">{{ t('dashboard.liveRpm') }}</span>
                <span class="dash-mini-value">{{ liveRpm?.user_rpm_used ?? 0 }}</span>
                <span class="dash-mini-sub">
                  {{ liveRpm?.user_rpm_limit ? `RPM ${liveRpm.user_rpm_used ?? 0}/${liveRpm.user_rpm_limit}` : `${formatTokens(stats.rpm)} ${t('dashboard.avgRpm')}` }}
                </span>
              </div>
              <div class="dash-hero-mini glass-inset">
                <span class="dash-mini-label">{{ t('dashboard.currentConcurrency') }}</span>
                <span class="dash-mini-value">{{ liveRpm?.current_concurrency ?? 0 }}</span>
                <span class="dash-mini-sub">{{ t('dashboard.avgResponse') }} {{ formatDuration(stats.average_duration_ms) }}</span>
              </div>
            </div>
          </div>
        </section>

        <UserDashboardStats
          :stats="stats"
          :is-simple="authStore.isSimpleMode"
          :platform-quotas="platformQuotas"
          :trend="trendData"
        />

        <UserDashboardCharts
          v-model:granularity="granularity"
          :loading="loadingCharts"
          :trend="trendData"
          :models="modelStats"
          @granularityChange="loadCharts"
        />

        <div class="dash-row-split">
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
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
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
const router = useRouter()
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

// ---------------------------------------------------------------- formatters
const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

const formatNumber = (value: number | null | undefined): string => toFiniteNumber(value).toLocaleString()

const formatTokens = (value: number | undefined): string => {
  const v = toFiniteNumber(value)
  if (v >= 1_000_000_000) return `${(v / 1_000_000_000).toFixed(2)}B`
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M`
  if (v >= 1_000) return `${(v / 1_000).toFixed(2)}K`
  return v.toLocaleString()
}

const formatCost = (value: number | null | undefined): string => {
  const v = toFiniteNumber(value)
  if (v >= 1000) return `${(v / 1000).toFixed(2)}K`
  if (v >= 1) return v.toFixed(2)
  if (v >= 0.01) return v.toFixed(3)
  return v.toFixed(4)
}

const formatDuration = (ms: number): string => (ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${Math.round(ms)}ms`)

const formatBalance = (b: number) =>
  new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }).format(b)

// ---------------------------------------------------------------- hero copy
const displayLocale = (): string => {
  const lang = typeof document !== 'undefined' ? document.documentElement.lang : ''
  return lang === 'zh' ? 'zh-CN' : 'en-US'
}

const heroKicker = computed(() => {
  const now = new Date()
  let date = ''
  try {
    date = new Intl.DateTimeFormat(displayLocale(), { dateStyle: 'full' }).format(now)
  } catch {
    date = now.toDateString()
  }
  const hour = now.getHours()
  const key =
    hour < 12 ? 'dashboard.heroGreetingMorning' : hour < 18 ? 'dashboard.heroGreetingAfternoon' : 'dashboard.heroGreetingEvening'
  return `${date} · ${t(key)}`
})

const heroDescription = computed(() => {
  if (!stats.value) return ''
  return t('dashboard.heroSummary', {
    tokens: formatTokens(stats.value.today_tokens),
    cost: formatCost(stats.value.today_actual_cost),
    duration: formatDuration(stats.value.average_duration_ms),
  })
})
</script>
<style scoped>
.dash-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* ================================= hero ================================= */
.dash-hero {
  position: relative;
  border-radius: var(--radius-hero);
  padding: 26px 30px;
  display: grid;
  grid-template-columns: 1.25fr 1fr;
  gap: 24px;
  flex: none;
  min-height: 219px;
}

.dash-hero-deco {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  overflow: hidden;
  pointer-events: none;
}

.dash-hero-dots {
  position: absolute;
  inset: 0;
  left: 45%;
  background: radial-gradient(color-mix(in oklch, var(--foreground) 7%, transparent) 1px, transparent 1.3px) 0 0 /
    16px 16px;
  mask-image: linear-gradient(90deg, transparent, black 45%);
  -webkit-mask-image: linear-gradient(90deg, transparent, black 45%);
}

.dash-hero-orb {
  position: absolute;
  border-radius: 50%;
}

.dash-hero-orb-accent {
  right: -90px;
  top: -140px;
  width: 380px;
  height: 380px;
  background: radial-gradient(
    circle at 35% 35%,
    color-mix(in oklch, var(--accent) 60%, white) 0%,
    color-mix(in oklch, var(--accent) 30%, transparent) 42%,
    transparent 70%
  );
}

.dash-hero-orb-success {
  right: 220px;
  bottom: -160px;
  width: 260px;
  height: 260px;
  background: radial-gradient(circle at 50% 50%, color-mix(in oklch, var(--success) 30%, transparent) 0%, transparent 65%);
}

.dash-hero-copy {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  gap: 10px;
}

.dash-hero-kicker {
  font-size: 12.5px;
  color: var(--muted);
  line-height: 1.3;
  margin-bottom: 3px;
}

.dash-hero-title {
  margin: 0;
  font-family: var(--display);
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.15;
}

.dash-hero-accent {
  color: var(--accent);
}

.dash-hero-desc {
  margin: 0;
  font-size: 13.5px;
  color: var(--muted);
  max-width: 540px;
  line-height: 1.55;
}

.dash-hero-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}

.dash-hero-side {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: flex-start;
  gap: 12px;
  min-width: 0;
}

.dash-hero-tools {
  position: relative;
  flex: none;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.dash-daterange :deep(.date-picker-trigger) {
  height: 28px;
  padding: 0 10px;
  gap: 6px;
  font-size: 12px;
}

.dash-refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 10px;
  border-radius: 8px;
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--foreground);
  font-size: 12px;
  font-weight: 600;
  border: 1px solid var(--border);
  cursor: pointer;
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

.dash-hero-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.dash-hero-mini {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  min-width: 0;
  min-height: 111px;
}

.dash-hero-mini-clickable {
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.dash-hero-mini-clickable:hover {
  border-color: color-mix(in oklch, var(--accent) 30%, transparent);
}

.dash-mini-label {
  font-size: 11.5px;
  color: var(--muted);
  font-weight: 600;
  line-height: 1.3;
}

.dash-mini-value {
  font-family: var(--display);
  font-size: 24px;
  font-weight: 800;
  letter-spacing: -0.03em;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.dash-mini-sub {
  font-size: 11.5px;
  color: var(--muted);
  line-height: 1.3;
}

/* ================================ bottom row ================================ */
.dash-row-split {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  align-items: start;
}

/* ============================== responsive ============================== */
@media (max-width: 1180px) {
  .dash-hero {
    grid-template-columns: 1fr;
  }
  .dash-hero-tools {
    position: static;
    justify-content: flex-start;
  }
  .dash-hero-mini {
    min-height: 0;
  }
}

@media (max-width: 1023px) {
  .dash-row-split {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 767px) {
  .dash-page {
    gap: 14px;
  }
  .dash-hero {
    padding: 18px 18px 16px;
    border-radius: 18px;
    gap: 12px;
  }
  .dash-hero-tools {
    flex-wrap: wrap;
    justify-content: flex-start;
  }
  .dash-hero-stats {
    grid-template-columns: 1fr;
  }
  .dash-hero-title {
    font-size: 22px;
  }
}
</style>
