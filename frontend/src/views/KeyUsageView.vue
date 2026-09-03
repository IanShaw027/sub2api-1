<template>
  <PublicPageLayout>
    <template #nav>
      <header class="key-usage-nav glass">
        <nav class="key-usage-nav-inner">
          <router-link to="/home" class="key-usage-brand">
            <span class="brand-mark">
              <img :src="siteLogo || '/logo.svg'" alt="Logo" />
            </span>
            <span class="key-usage-brand-name">{{ siteName }}</span>
          </router-link>
          <div class="key-usage-nav-actions">
            <LocaleSwitcher />
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="header-icon-btn"
              :title="t('home.viewDocs')"
            >
              <Icon name="book" size="sm" />
            </a>
            <button
              type="button"
              class="header-icon-btn"
              :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
              @click="toggleTheme"
            >
              <Icon v-if="isDark" name="sun" size="sm" />
              <Icon v-else name="moon" size="sm" />
            </button>
          </div>
        </nav>
      </header>
    </template>

    <main class="key-usage-main">
      <!-- Hero -->
      <div class="key-usage-hero">
        <h1 class="section-title">{{ t('keyUsage.title') }}</h1>
        <p class="page-description key-usage-subtitle">{{ t('keyUsage.subtitle') }}</p>
      </div>

      <!-- Input Section -->
      <div class="key-usage-query">
        <div class="key-usage-query-row">
          <div class="key-usage-key-field">
            <Icon name="key" size="sm" class="key-usage-key-icon" />
            <input
              v-model="apiKey"
              :type="keyVisible ? 'text' : 'password'"
              :placeholder="t('keyUsage.placeholder')"
              class="field field-lg key-usage-key-input"
              @keydown.enter="queryKey"
            />
            <button
              type="button"
              class="icon-btn key-usage-key-toggle"
              :title="keyVisible ? t('keyUsage.hideKey') : t('keyUsage.showKey')"
              @click="keyVisible = !keyVisible"
            >
              <Icon :name="keyVisible ? 'eyeOff' : 'eye'" size="sm" />
            </button>
          </div>
          <Button size="lg" :loading="isQuerying" @click="queryKey">
            <Icon v-if="!isQuerying" name="search" size="sm" />
            {{ isQuerying ? t('keyUsage.querying') : t('keyUsage.query') }}
          </Button>
        </div>
        <p class="key-usage-privacy">{{ t('keyUsage.privacyNote') }}</p>

        <!-- Date Range Picker -->
        <div v-if="showDatePicker" class="key-usage-daterange">
          <span class="key-usage-daterange-label">{{ t('keyUsage.dateRange') }}</span>
          <button
            v-for="range in dateRanges"
            :key="range.key"
            type="button"
            class="tag key-usage-range-pill"
            :class="{ 'tag-accent key-usage-range-pill-active': currentRange === range.key }"
            @click="setDateRange(range.key)"
          >
            {{ range.label }}
          </button>
          <div v-if="currentRange === 'custom'" class="key-usage-custom-range">
            <input v-model="customStartDate" type="date" class="field key-usage-date-input" />
            <span class="key-usage-daterange-label">-</span>
            <input v-model="customEndDate" type="date" class="field key-usage-date-input" />
            <Button size="sm" @click="queryKey">{{ t('keyUsage.apply') }}</Button>
          </div>
        </div>
      </div>

      <!-- Results Container -->
      <div v-if="showResults" class="key-usage-results">
        <!-- Loading Skeleton -->
        <div v-if="showLoading" class="key-usage-loading">
          <div class="key-usage-stat-grid">
            <div v-for="i in 3" :key="i" class="glass-card key-usage-skeleton-card">
              <div class="skeleton h-3 w-20 mb-3"></div>
              <div class="skeleton h-7 w-24"></div>
            </div>
          </div>
          <div class="glass-card key-usage-skeleton-card">
            <div class="skeleton h-3 w-32 mb-4"></div>
            <div class="space-y-2">
              <div class="skeleton h-4 w-full"></div>
              <div class="skeleton h-4 w-3/4"></div>
              <div class="skeleton h-4 w-5/6"></div>
            </div>
          </div>
        </div>

        <!-- Result Content -->
        <div v-else-if="resultData" class="key-usage-result">
          <!-- Status Badge -->
          <div v-if="statusInfo" class="key-usage-status">
            <StatusBadge :tone="statusInfo.isActive ? 'success' : 'danger'" dot :pulse="statusInfo.isActive">
              {{ statusInfo.label }} · {{ statusInfo.statusText }}
            </StatusBadge>
          </div>

          <!-- Stat cards -->
          <div v-if="ringItems.length > 0" class="key-usage-stat-grid">
            <StatCard
              v-for="(ring, i) in ringItems"
              :key="i"
              :label="ring.title"
              :value="ring.isBalance ? ring.amount : `${displayPcts[i] ?? 0}%`"
              :sub="ring.isBalance ? undefined : ring.amount"
              :delta="ring.resetAt && formatResetTime(ring.resetAt) ? `⟳ ${formatResetTime(ring.resetAt)}` : undefined"
              :delta-tone="ring.pct > 90 ? 'down' : 'neutral'"
            />
          </div>

          <!-- Detail Card -->
          <div v-if="detailRows.length > 0" class="glass-card key-usage-card">
            <div class="card-header">
              <h3 class="card-title">{{ t('keyUsage.detailInfo') }}</h3>
            </div>
            <div class="key-usage-detail-list">
              <div v-for="(row, i) in detailRows" :key="i" class="key-usage-detail-row">
                <div class="key-usage-detail-left">
                  <span class="key-usage-detail-icon" :class="`key-usage-detail-icon-${row.tone}`">
                    <Icon :name="row.icon" size="xs" />
                  </span>
                  <span class="key-usage-detail-label">{{ row.label }}</span>
                </div>
                <span class="key-usage-detail-value" :class="row.valueClass">{{ row.value }}</span>
              </div>
            </div>
          </div>

          <!-- Usage Stats Card -->
          <div v-if="usageStatCells.length > 0" class="glass-card key-usage-card !p-0">
            <div class="card-header">
              <h3 class="card-title">{{ t('keyUsage.tokenStats') }}</h3>
            </div>
            <div class="summary-row key-usage-summary-row">
              <div v-for="(cell, i) in usageStatCells" :key="i" class="summary-chip">
                <span class="summary-chip-label">{{ cell.label }}</span>
                <span class="summary-chip-value">{{ cell.value }}</span>
              </div>
            </div>
          </div>

          <!-- Daily Usage Table -->
          <div v-if="showDailyUsage" class="glass-card key-usage-card !p-0">
            <div class="card-header key-usage-table-header">
              <h3 class="card-title">{{ t('keyUsage.dailyDetail') }}</h3>
              <SegmentedControl
                :model-value="dailyUsageDays"
                :options="dailyUsageOptions"
                size="sm"
                @update:model-value="setDailyUsageDays"
              />
            </div>
            <DataTable :columns="dailyUsageColumns" :data="dailyUsageRows" row-key="date">
              <template #cell-requests="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-input_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-output_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-cache_read_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-cache_write_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-cost="{ row }">{{ usd(row.actual_cost != null ? row.actual_cost : row.cost) }}</template>
            </DataTable>
          </div>

          <!-- Model Stats Table -->
          <div v-if="modelStats.length > 0" class="glass-card key-usage-card !p-0">
            <div class="card-header">
              <h3 class="card-title">{{ t('keyUsage.modelStats') }}</h3>
            </div>
            <DataTable :columns="modelStatsColumns" :data="modelStats" row-key="model">
              <template #cell-model="{ value }">{{ value || '-' }}</template>
              <template #cell-requests="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-input_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-output_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-cache_creation_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-cache_read_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-total_tokens="{ value }">{{ fmtNum(value) }}</template>
              <template #cell-cost="{ row }">{{ usd(row.actual_cost != null ? row.actual_cost : row.cost) }}</template>
            </DataTable>
          </div>
        </div>
      </div>
    </main>

    <!-- Footer -->
    <footer class="key-usage-footer">
      <div class="key-usage-footer-inner">
        <p class="key-usage-footer-copy">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="key-usage-footer-links">
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
        </div>
      </div>
    </footer>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import StatCard from '@/components/ui/StatCard.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import { buildGatewayUrl } from '@/api/client'
import { formatDateLocalInput } from '@/utils/format'
import { sanitizeUrl } from '@/utils/url'
import { useTheme } from '@/composables/useTheme'

const { t, locale } = useI18n()
const appStore = useAppStore()

// ==================== Site Settings (same as HomeView) ====================

const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// ==================== Theme (same as HomeView) ====================

const { isDark, toggleTheme } = useTheme()

const currentYear = computed(() => new Date().getFullYear())

// ==================== Key Query State ====================

const apiKey = ref('')
const keyVisible = ref(false)
const isQuerying = ref(false)
const showResults = ref(false)
const showLoading = ref(false)
const showDatePicker = ref(false)
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const resultData = ref<any>(null)
const now = ref(new Date())
let resetTimer: ReturnType<typeof setInterval> | null = null

// ==================== Date Range State ====================

type DateRangeKey = 'today' | '7d' | '30d' | 'custom'
const currentRange = ref<DateRangeKey>('today')
const customStartDate = ref('')
const customEndDate = ref('')
const dailyUsageDays = ref<'7' | '30' | '90'>('30')

const dateRanges = computed(() => [
  { key: 'today' as const, label: t('keyUsage.dateRangeToday') },
  { key: '7d' as const, label: t('keyUsage.dateRange7d') },
  { key: '30d' as const, label: t('keyUsage.dateRange30d') },
  { key: 'custom' as const, label: t('keyUsage.dateRangeCustom') },
])

const dailyUsageOptions = computed(() => [
  { value: '7' as const, label: t('keyUsage.dateRange7d') },
  { value: '30' as const, label: t('keyUsage.dateRange30d') },
  { value: '90' as const, label: t('keyUsage.dateRange90d') },
])

function setDateRange(key: DateRangeKey) {
  currentRange.value = key
  if (key !== 'custom') {
    queryKey()
  }
}

function getDateParams(): string {
  const now = new Date()
  const params = new URLSearchParams()

  if (currentRange.value === 'custom') {
    if (customStartDate.value && customEndDate.value) {
      params.set('start_date', customStartDate.value)
      params.set('end_date', customEndDate.value)
    }
  } else {
    const end = formatDateLocalInput(now)
    let start: string
    switch (currentRange.value) {
      case 'today': start = end; break
      case '7d': start = formatDateLocalInput(new Date(now.getTime() - 7 * 86400000)); break
      case '30d': start = formatDateLocalInput(new Date(now.getTime() - 30 * 86400000)); break
      default: start = formatDateLocalInput(new Date(now.getTime() - 30 * 86400000))
    }
    params.set('start_date', start)
    params.set('end_date', end)
  }
  params.set('days', dailyUsageDays.value)
  params.set('timezone', getBrowserTimezone())
  return params.toString()
}

function setDailyUsageDays(days: '7' | '30' | '90') {
  if (dailyUsageDays.value === days) return
  dailyUsageDays.value = days
  if (resultData.value && apiKey.value.trim()) {
    queryKey()
  }
}

// ==================== Result Animation ====================

const ringAnimated = ref(false)
const displayPcts = ref<number[]>([])

interface RingItem {
  title: string
  pct: number
  amount: string
  isBalance?: boolean
  iconType: 'clock' | 'calendar' | 'dollar'
  resetAt?: string | null
}

function triggerRingAnimation(items: RingItem[]) {
  ringAnimated.value = false
  displayPcts.value = items.map(() => 0)

  nextTick(() => {
    requestAnimationFrame(() => {
      setTimeout(() => {
        ringAnimated.value = true

        // Animate percentage numbers
        const duration = 700
        const startTime = performance.now()
        const targets = items.map(item => item.isBalance ? 0 : item.pct)

        function tick() {
          const elapsed = performance.now() - startTime
          const p = Math.min(elapsed / duration, 1)
          const ease = 1 - Math.pow(1 - p, 3)
          displayPcts.value = targets.map(target => Math.round(ease * target))
          if (p < 1) requestAnimationFrame(tick)
        }
        requestAnimationFrame(tick)
      }, 50)
    })
  })
}

// ==================== Computed Data ====================

const statusInfo = computed(() => {
  const data = resultData.value
  if (!data) return null

  if (data.mode === 'quota_limited') {
    const isValid = data.isValid !== false
    const statusMap: Record<string, string> = {
      active: 'Active',
      quota_exhausted: 'Quota Exhausted',
      expired: 'Expired',
    }
    return {
      label: t('keyUsage.quotaMode'),
      statusText: statusMap[data.status] || data.status || 'Unknown',
      isActive: isValid && data.status === 'active',
    }
  }

  return {
    label: data.planName || t('keyUsage.walletBalance'),
    statusText: 'Active',
    isActive: true,
  }
})

const ringItems = computed<RingItem[]>(() => {
  const data = resultData.value
  if (!data) return []

  const items: RingItem[] = []

  if (data.mode === 'quota_limited') {
    if (data.quota) {
      const pct = data.quota.limit > 0 ? Math.min(Math.round((data.quota.used / data.quota.limit) * 100), 100) : 0
      items.push({ title: t('keyUsage.totalQuota'), pct, amount: `${usd(data.quota.used)} / ${usd(data.quota.limit)}`, iconType: 'dollar' })
    }
    if (data.rate_limits) {
      const windowLabels: Record<string, string> = { '5h': t('keyUsage.limit5h'), '1d': t('keyUsage.limitDaily'), '7d': t('keyUsage.limit7d') }
      const windowIcons: Record<string, 'clock' | 'calendar'> = { '5h': 'clock', '1d': 'calendar', '7d': 'calendar' }
      for (const rl of data.rate_limits) {
        const pct = rl.limit > 0 ? Math.min(Math.round((rl.used / rl.limit) * 100), 100) : 0
        items.push({
          title: windowLabels[rl.window] || rl.window,
          pct,
          amount: `${usd(rl.used)} / ${usd(rl.limit)}`,
          iconType: windowIcons[rl.window] || 'clock',
          resetAt: rl.reset_at,
        })
      }
    }
  } else {
    if (data.subscription) {
      const sub = data.subscription
      const limits = [
        { label: t('keyUsage.limitDaily'), usage: sub.daily_usage_usd, limit: sub.daily_limit_usd },
        { label: t('keyUsage.limitWeekly'), usage: sub.weekly_usage_usd, limit: sub.weekly_limit_usd },
        { label: t('keyUsage.limitMonthly'), usage: sub.monthly_usage_usd, limit: sub.monthly_limit_usd },
      ]
      for (const l of limits) {
        if (l.limit != null && l.limit > 0) {
          const pct = Math.min(Math.round((l.usage / l.limit) * 100), 100)
          items.push({ title: l.label, pct, amount: `${usd(l.usage)} / ${usd(l.limit)}`, iconType: 'calendar' })
        }
      }
    }
    if (!data.subscription && data.balance != null) {
      items.push({ title: t('keyUsage.walletBalance'), pct: 0, amount: usd(data.balance), isBalance: true, iconType: 'dollar' })
    }
  }

  return items
})

interface DetailRow {
  icon: 'shield' | 'calendar' | 'dollar' | 'check'
  tone: 'success' | 'warning' | 'danger' | 'accent'
  label: string
  value: string
  valueClass: string
}

function getUsageTone(pct: number): 'danger' | 'warning' | 'success' {
  if (pct > 90) return 'danger'
  if (pct > 70) return 'warning'
  return 'success'
}

function toneTextClass(tone: 'danger' | 'warning' | 'success' | ''): string {
  if (tone === 'danger') return 'text-danger-text'
  if (tone === 'warning') return 'text-warning-text'
  if (tone === 'success') return 'text-success-text'
  return ''
}

const detailRows = computed<DetailRow[]>(() => {
  const data = resultData.value
  if (!data) return []

  const rows: DetailRow[] = []

  if (data.mode === 'quota_limited') {
    if (data.quota) {
      const remainTone = data.quota.remaining <= 0 ? 'danger'
        : data.quota.remaining < data.quota.limit * 0.1 ? 'warning'
        : 'success'
      rows.push({
        icon: 'shield', tone: 'success',
        label: t('keyUsage.remainingQuota'), value: usd(data.quota.remaining), valueClass: toneTextClass(remainTone),
      })
    }
    if (data.expires_at) {
      const daysLeft = data.days_until_expiry
      let expiryStr = formatDate(data.expires_at)
      if (daysLeft != null) {
        expiryStr += daysLeft > 0 ? ` ${t('keyUsage.daysLeft', { days: daysLeft })}` : daysLeft === 0 ? ` ${t('keyUsage.todayExpires')}` : ''
      }
      rows.push({
        icon: 'calendar', tone: 'warning',
        label: t('keyUsage.expiresAt'), value: expiryStr, valueClass: '',
      })
    }
    if (data.rate_limits) {
      const windowMap: Record<string, string> = { '5h': '5H', '1d': locale.value === 'zh' ? '日' : 'D', '7d': '7D' }
      for (const rl of data.rate_limits) {
        const pct = rl.limit > 0 ? (rl.used / rl.limit) * 100 : 0
        let valueStr = `${usd(rl.used)} / ${usd(rl.limit)}`
        const resetStr = formatResetTime(rl.reset_at)
        if (resetStr) {
          valueStr += ` (⟳ ${resetStr})`
        }
        rows.push({
          icon: 'dollar', tone: 'accent',
          label: `${t('keyUsage.usedQuota')} (${windowMap[rl.window] || rl.window})`,
          value: valueStr,
          valueClass: toneTextClass(getUsageTone(pct)),
        })
      }
    }
  } else {
    rows.push({
      icon: 'check', tone: 'success',
      label: t('keyUsage.subscriptionType'), value: data.planName || t('keyUsage.walletBalance'), valueClass: '',
    })

    if (data.subscription) {
      const sub = data.subscription
      if (sub.daily_limit_usd > 0) {
        const pct = (sub.daily_usage_usd / sub.daily_limit_usd) * 100
        rows.push({
          icon: 'dollar', tone: 'accent',
          label: `${t('keyUsage.usedQuota')} (${locale.value === 'zh' ? '日' : 'D'})`, value: `${usd(sub.daily_usage_usd)} / ${usd(sub.daily_limit_usd)}`, valueClass: toneTextClass(getUsageTone(pct)),
        })
      }
      if (sub.weekly_limit_usd > 0) {
        const pct = (sub.weekly_usage_usd / sub.weekly_limit_usd) * 100
        rows.push({
          icon: 'dollar', tone: 'accent',
          label: `${t('keyUsage.usedQuota')} (${locale.value === 'zh' ? '周' : 'W'})`, value: `${usd(sub.weekly_usage_usd)} / ${usd(sub.weekly_limit_usd)}`, valueClass: toneTextClass(getUsageTone(pct)),
        })
      }
      if (sub.monthly_limit_usd > 0) {
        const pct = (sub.monthly_usage_usd / sub.monthly_limit_usd) * 100
        rows.push({
          icon: 'dollar', tone: 'accent',
          label: `${t('keyUsage.usedQuota')} (${locale.value === 'zh' ? '月' : 'M'})`, value: `${usd(sub.monthly_usage_usd)} / ${usd(sub.monthly_limit_usd)}`, valueClass: toneTextClass(getUsageTone(pct)),
        })
      }
      if (sub.expires_at) {
        rows.push({
          icon: 'calendar', tone: 'warning',
          label: t('keyUsage.subscriptionExpires'), value: formatDate(sub.expires_at), valueClass: '',
        })
      }
    }

    const remainTone = data.remaining != null
      ? (data.remaining <= 0 ? 'danger' : data.remaining < 10 ? 'warning' : 'success')
      : ''
    rows.push({
      icon: 'shield', tone: 'success',
      label: t('keyUsage.remainingQuota'), value: data.remaining != null ? usd(data.remaining) : '-', valueClass: toneTextClass(remainTone as 'danger' | 'warning' | 'success' | ''),
    })
  }

  return rows
})

interface StatCell {
  label: string
  value: string
}

const usageStatCells = computed<StatCell[]>(() => {
  const usage = resultData.value?.usage
  if (!usage) return []

  const today = usage.today || {}
  const total = usage.total || {}

  return [
    { label: t('keyUsage.todayRequests'), value: fmtNum(today.requests) },
    { label: t('keyUsage.todayInputTokens'), value: fmtNum(today.input_tokens) },
    { label: t('keyUsage.todayOutputTokens'), value: fmtNum(today.output_tokens) },
    { label: t('keyUsage.todayTokens'), value: fmtNum(today.total_tokens) },
    { label: t('keyUsage.todayCacheCreation'), value: fmtNum(today.cache_creation_tokens) },
    { label: t('keyUsage.todayCacheRead'), value: fmtNum(today.cache_read_tokens) },
    { label: t('keyUsage.todayCost'), value: usd(today.actual_cost) },
    { label: t('keyUsage.rpmTpm'), value: `${usage.rpm || 0} / ${usage.tpm || 0}` },
    { label: t('keyUsage.totalRequests'), value: fmtNum(total.requests) },
    { label: t('keyUsage.totalInputTokens'), value: fmtNum(total.input_tokens) },
    { label: t('keyUsage.totalOutputTokens'), value: fmtNum(total.output_tokens) },
    { label: t('keyUsage.totalTokensLabel'), value: fmtNum(total.total_tokens) },
    { label: t('keyUsage.totalCacheCreation'), value: fmtNum(total.cache_creation_tokens) },
    { label: t('keyUsage.totalCacheRead'), value: fmtNum(total.cache_read_tokens) },
    { label: t('keyUsage.totalCost'), value: usd(total.actual_cost) },
    { label: t('keyUsage.avgDuration'), value: usage.average_duration_ms ? `${Math.round(usage.average_duration_ms)} ms` : '-' },
  ]
})

// eslint-disable-next-line @typescript-eslint/no-explicit-any
const modelStats = computed<any[]>(() => resultData.value?.model_stats || [])

interface DailyUsageRow {
  date: string
  requests: number
  input_tokens: number
  output_tokens: number
  cache_read_tokens: number
  cache_write_tokens: number
  cost: number
  actual_cost?: number
}

const dailyUsageRows = computed<DailyUsageRow[]>(() => {
  const rows = resultData.value?.daily_usage
  return Array.isArray(rows) ? rows : []
})

const showDailyUsage = computed(() => Boolean(resultData.value && Array.isArray(resultData.value.daily_usage)))

const dailyUsageColumns = computed<Column[]>(() => [
  { key: 'date', label: t('keyUsage.date') },
  { key: 'requests', label: t('keyUsage.requests') },
  { key: 'input_tokens', label: t('keyUsage.inputTokens') },
  { key: 'output_tokens', label: t('keyUsage.outputTokens') },
  { key: 'cache_read_tokens', label: t('keyUsage.cacheReadTokens') },
  { key: 'cache_write_tokens', label: t('keyUsage.cacheWriteTokens') },
  { key: 'cost', label: t('keyUsage.cost') },
])

const modelStatsColumns = computed<Column[]>(() => [
  { key: 'model', label: t('keyUsage.model') },
  { key: 'requests', label: t('keyUsage.requests') },
  { key: 'input_tokens', label: t('keyUsage.inputTokens') },
  { key: 'output_tokens', label: t('keyUsage.outputTokens') },
  { key: 'cache_creation_tokens', label: t('keyUsage.cacheCreationTokens') },
  { key: 'cache_read_tokens', label: t('keyUsage.cacheReadTokens') },
  { key: 'total_tokens', label: t('keyUsage.totalTokens') },
  { key: 'cost', label: t('keyUsage.cost') },
])

// ==================== Utility Functions ====================

function usd(value: number | null | undefined): string {
  if (value == null || value < 0) return '-'
  return '$' + Number(value).toFixed(2)
}

function fmtNum(val: number | null | undefined): string {
  if (val == null) return '-'
  return val.toLocaleString()
}

function formatDate(iso: string | null | undefined): string {
  if (!iso) return '-'
  const d = new Date(iso)
  const loc = locale.value === 'zh' ? 'zh-CN' : 'en-US'
  return d.toLocaleDateString(loc, { year: 'numeric', month: 'long', day: 'numeric' })
}

function getBrowserTimezone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
  } catch {
    return 'UTC'
  }
}

// ==================== API Query ====================

async function fetchUsage(key: string) {
  const dateParams = getDateParams()
  const url = buildGatewayUrl('/v1/usage') + (dateParams ? '?' + dateParams : '')
  const res = await fetch(url, {
    headers: { 'Authorization': 'Bearer ' + key },
  })
  if (!res.ok) {
    const body = await res.json().catch(() => null)
    const msg = body?.error?.message || body?.message || `${t('keyUsage.queryFailed')} (${res.status})`
    throw new Error(msg)
  }
  return await res.json()
}

async function queryKey() {
  if (isQuerying.value) return
  const key = apiKey.value.trim()
  if (!key) {
    appStore.showInfo(t('keyUsage.enterApiKey'))
    return
  }

  isQuerying.value = true
  showResults.value = true
  showLoading.value = true
  resultData.value = null

  try {
    const data = await fetchUsage(key)
    resultData.value = data
    showLoading.value = false
    showDatePicker.value = true

    // Trigger stat card count-up animation after DOM update
    nextTick(() => {
      triggerRingAnimation(ringItems.value)
    })

    appStore.showSuccess(t('keyUsage.querySuccess'))
  } catch (err) {
    showResults.value = false
    showLoading.value = false
    appStore.showError((err as Error).message || t('keyUsage.queryFailedRetry'))
  } finally {
    isQuerying.value = false
  }
}

// ==================== Lifecycle ====================

function formatResetTime(resetAt: string | null | undefined): string {
  if (!resetAt) return ''
  const diff = new Date(resetAt).getTime() - now.value.getTime()
  if (diff <= 0) return t('keyUsage.resetNow')
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const mins = Math.floor((diff % 3600000) / 60000)
  if (days > 0) return `${days}d ${hours}h`
  if (hours > 0) return `${hours}h ${mins}m`
  return `${mins}m`
}

onMounted(() => {
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
  resetTimer = setInterval(() => { now.value = new Date() }, 60000)
})

onUnmounted(() => {
  if (resetTimer) clearInterval(resetTimer)
})
</script>

<style scoped>
/* ---------- Nav ---------- */
.key-usage-nav {
  position: sticky;
  top: 0;
  z-index: 30;
  border-bottom: 1px solid var(--border);
}

.key-usage-nav-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  max-width: 1024px;
  margin: 0 auto;
  padding: 12px 24px;
}

.key-usage-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  text-decoration: none;
}

.key-usage-brand img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.key-usage-brand-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--foreground);
}

.key-usage-nav-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* ---------- Main ---------- */
.key-usage-main {
  width: 100%;
  max-width: 1024px;
  margin: 0 auto;
  padding: 48px 24px 24px;
}

.key-usage-hero {
  margin-bottom: 32px;
  text-align: center;
}

.key-usage-subtitle {
  margin-left: auto;
  margin-right: auto;
  max-width: 28rem;
}

/* ---------- Query ---------- */
.key-usage-query {
  max-width: 560px;
  margin: 0 auto 32px;
}

.key-usage-query-row {
  display: flex;
  gap: 10px;
}

.key-usage-key-field {
  position: relative;
  flex: 1;
  min-width: 0;
}

.key-usage-key-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  color: var(--muted);
}

.key-usage-key-input {
  padding-left: 36px;
  padding-right: 40px;
}

.key-usage-key-toggle {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
}

.key-usage-privacy {
  margin-top: 10px;
  text-align: center;
  font-size: 12px;
  color: var(--muted);
}

.key-usage-daterange {
  margin-top: 14px;
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.key-usage-daterange-label {
  font-size: 12px;
  color: var(--muted);
}

.key-usage-range-pill {
  cursor: pointer;
  border: 0;
  transition: background 0.15s ease, color 0.15s ease;
}

.key-usage-custom-range {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: 4px;
}

.key-usage-date-input {
  width: auto;
  height: 30px;
  font-size: 12px;
}

/* ---------- Results ---------- */
.key-usage-results {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.key-usage-loading {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.key-usage-skeleton-card {
  padding: 16px;
}

.key-usage-result {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.key-usage-status {
  display: flex;
  justify-content: center;
}

.key-usage-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
  gap: 12px;
}

.key-usage-card {
  overflow: hidden;
}

.key-usage-table-header {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

/* ---------- Detail list ---------- */
.key-usage-detail-list {
  display: flex;
  flex-direction: column;
}

.key-usage-detail-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border);
}

.key-usage-detail-row:last-child {
  border-bottom: 0;
}

.key-usage-detail-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
}

.key-usage-detail-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: 8px;
}

.key-usage-detail-icon-success {
  background: color-mix(in oklch, var(--success) 16%, transparent);
  color: var(--success-text);
}

.key-usage-detail-icon-warning {
  background: color-mix(in oklch, var(--warning) 18%, transparent);
  color: var(--warning-text);
}

.key-usage-detail-icon-danger {
  background: color-mix(in oklch, var(--danger) 14%, transparent);
  color: var(--danger-text);
}

.key-usage-detail-icon-accent {
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
}

.key-usage-detail-label {
  font-size: 13px;
  color: var(--foreground);
}

.key-usage-detail-value {
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  color: var(--foreground);
}

/* ---------- Summary row (token stats) ---------- */
.key-usage-summary-row {
  padding: 16px 20px 20px;
}

@media (max-width: 767px) {
  .key-usage-summary-row {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

/* ---------- Footer ---------- */
.key-usage-footer {
  width: 100%;
  border-top: 1px solid var(--border);
  padding: 24px;
}

.key-usage-footer-inner {
  display: flex;
  max-width: 1024px;
  margin: 0 auto;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  text-align: center;
}

.key-usage-footer-copy {
  font-size: 13px;
  color: var(--muted);
}

.key-usage-footer-links {
  display: flex;
  align-items: center;
  gap: 16px;
}

.key-usage-footer-links a {
  font-size: 13px;
  color: var(--muted);
  text-decoration: none;
  transition: color 0.15s ease;
}

.key-usage-footer-links a:hover {
  color: var(--foreground);
}

@media (min-width: 640px) {
  .key-usage-footer-inner {
    flex-direction: row;
    justify-content: space-between;
    text-align: left;
  }
}
</style>
