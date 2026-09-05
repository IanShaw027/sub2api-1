<template>
 <AppLayout>
 <PageHeader :title="t('redeem.title')" :description="t('redeem.description')" />
 <div class="redeem-page">
 <!-- Balance / concurrency / total recharged mini stats -->
 <div class="redeem-stats">
 <StatCard :label="t('redeem.currentBalance')" :value="`$${(user?.balance ?? 0).toFixed(2)}`" />
 <StatCard :label="t('redeem.concurrency')" :value="`${user?.concurrency ?? 0}`" :sub="t('redeem.requests')" />
 <StatCard :label="t('redeem.totalRecharged')" :value="`$${totalRecharged.toFixed(2)}`" />
 </div>

 <!-- Redeem Form -->
 <div class="glass-card">
 <div class="card-header">
 <p class="card-title">{{ t('redeem.redeemCodeLabel') }}</p>
 <p class="card-subtitle">{{ t('redeem.redeemCodeHint') }}</p>
 </div>
 <div class="card-body">
 <form class="redeem-form" @submit.prevent="handleRedeem">
 <input
 id="code"
 v-model="redeemCode"
 type="text"
 required
 :placeholder="t('redeem.redeemCodePlaceholder')"
 :disabled="submitting"
 class="field redeem-code-field"
 />
 <button type="submit" class="btn btn-primary redeem-submit-btn" :disabled="!redeemCode || submitting">
 <span v-if="submitting" class="spinner" aria-hidden="true"></span>
 <Icon v-else name="checkCircle" size="sm" />
 {{ submitting ? t('redeem.redeeming') : t('redeem.redeemButton') }}
 </button>
 </form>
 </div>
 </div>

 <!-- Success Message -->
 <transition name="fade">
 <div v-if="redeemResult" class="notice notice-success">
 <Icon name="checkCircle" size="sm" class="notice-icon" />
 <div class="min-w-0 flex-1">
 <p class="notice-title">{{ t('redeem.redeemSuccess') }}</p>
 <p>{{ redeemResult.message }}</p>
 <div class="mt-1.5 space-y-0.5">
 <p v-if="redeemResult.type === 'balance'">
 {{ t('redeem.added') }}: <span class="num font-semibold">${{ redeemResult.value.toFixed(2) }}</span>
 </p>
 <p v-else-if="redeemResult.type === 'concurrency'">
 {{ t('redeem.added') }}: <span class="num font-semibold">{{ redeemResult.value }}</span>
 {{ t('redeem.concurrentRequests') }}
 </p>
 <p v-else-if="redeemResult.type === 'subscription'">
 {{ t('redeem.subscriptionAssigned') }}
 <span v-if="redeemResult.group_name"> - {{ redeemResult.group_name }}</span>
 <span v-if="redeemResult.validity_days">
 ({{ t('redeem.subscriptionDays', { days: redeemResult.validity_days }) }})</span
 >
 </p>
 <p v-if="redeemResult.new_balance !== undefined">
 {{ t('redeem.newBalance') }}: <span class="num font-semibold">${{ redeemResult.new_balance.toFixed(2) }}</span>
 </p>
 <p v-if="redeemResult.new_concurrency !== undefined">
 {{ t('redeem.newConcurrency') }}:
 <span class="num font-semibold">{{ redeemResult.new_concurrency }} {{ t('redeem.requests') }}</span>
 </p>
 </div>
 </div>
 </div>
 </transition>

 <!-- Error Message -->
 <transition name="fade">
 <div v-if="errorMessage" class="notice notice-danger">
 <Icon name="exclamationCircle" size="sm" class="notice-icon" />
 <div class="min-w-0 flex-1">
 <p class="notice-title">{{ t('redeem.redeemFailed') }}</p>
 <p>{{ errorMessage }}</p>
 </div>
 </div>
 </transition>

 <!-- Information Card -->
 <div class="glass-card">
 <div class="card-body">
 <p class="card-title">{{ t('redeem.aboutCodes') }}</p>
 <ul class="redeem-rules">
 <li>{{ t('redeem.codeRule1') }}</li>
 <li>{{ t('redeem.codeRule2') }}</li>
 <li>
 {{ t('redeem.codeRule3') }}
 <span v-if="contactInfo" class="tag tag-accent ml-1">{{ contactInfo }}</span>
 </li>
 <li>{{ t('redeem.codeRule4') }}</li>
 </ul>
 </div>
 </div>

 <!-- Recent Activity -->
 <div class="glass-card redeem-history-card">
 <div class="card-header redeem-history-header">
 <p class="card-title">{{ t('redeem.recentActivity') }}</p>
 <UiSelect
 :model-value="historyType"
 class="redeem-history-filter"
 :options="historyTypeOptions"
 variant="pill"
 @update:model-value="handleHistoryTypeChange"
 />
 </div>

 <DataTable :columns="historyColumns" :data="history" :loading="loadingHistory" row-key="id">
 <template #cell-type="{ row }">
 <div class="rh-type-cell">
 <StatusBadge :tone="historyBadgeTone(row)">
 <Icon :name="historyIcon(row)" size="xs" />
 {{ getHistoryItemTitle(row) }}
 </StatusBadge>
 </div>
 </template>
 <template #cell-code="{ row }">
 <span v-if="!isAdminAdjustment(row.type)" class="text-mono rh-code">
 {{ row.code.slice(0, 8) }}...
 </span>
 <span v-else class="rh-muted">{{ t('redeem.adminAdjustment') }}</span>
 </template>
 <template #cell-used_at="{ row }">
 <span class="text-mono rh-muted">{{ formatDateTime(row.used_at) }}</span>
 </template>
 <template #cell-value="{ row }">
 <span class="num" :class="historyValueClass(row)">{{ formatHistoryValue(row) }}</span>
 </template>
 <template #empty>
 <div class="empty-state">
 <Icon name="clock" size="xl" class="empty-state-icon" />
 <p class="empty-state-description">{{ t('redeem.historyWillAppear') }}</p>
 </div>
 </template>
 </DataTable>

 <div v-if="historyTotal > historyPageSize" class="redeem-pagination">
 <UiPagination
 :page="historyPage"
 :page-size="historyPageSize"
 :total="historyTotal"
 :show-page-size-selector="false"
 @update:page="handleHistoryPageChange"
 />
 </div>
 </div>
 </div>
 </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { redeemAPI, authAPI, type RedeemHistoryItem } from '@/api'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import StatCard from '@/components/ui/StatCard.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiSelect from '@/components/ui/UiSelect.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import DataTable from '@/components/common/DataTable.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

const { t } = useI18n()
const authStore = useAuthStore()
const appStore = useAppStore()
const subscriptionStore = useSubscriptionStore()

const user = computed(() => authStore.user)

const redeemCode = ref('')
const submitting = ref(false)
const redeemResult = ref<{
  message: string
  type: string
  value: number
  new_balance?: number
  new_concurrency?: number
  group_name?: string
  validity_days?: number
} | null>(null)
const errorMessage = ref('')

// History data
const history = ref<RedeemHistoryItem[]>([])
const loadingHistory = ref(false)
const contactInfo = ref('')
const totalRecharged = ref(0)
const historyType = ref('')
const historyPage = ref(1)
const historyPageSize = ref(8)
const historyTotal = ref(0)

const historyColumns = computed<Column[]>(() => [
  { key: 'type', label: t('redeem.columns.type') },
  { key: 'code', label: t('redeem.columns.code') },
  { key: 'used_at', label: t('redeem.columns.time') },
  { key: 'value', label: t('redeem.columns.amount') }
])

const historyTypeOptions = computed(() => [
  { value: '', label: t('redeem.filter.all') },
  { value: 'balance', label: t('redeem.filter.balance') },
  { value: 'concurrency', label: t('redeem.filter.concurrency') },
  { value: 'subscription', label: t('redeem.filter.subscription') },
  { value: 'admin_balance', label: t('redeem.filter.adminAdjustment') },
  { value: 'admin_concurrency', label: t('redeem.filter.adminAdjustment') }
])

// Helper functions for history display
const isBalanceType = (type: string) => {
  return type === 'balance' || type === 'admin_balance'
}

const isSubscriptionType = (type: string) => {
  return type === 'subscription'
}

const isAdminAdjustment = (type: string) => {
  return type === 'admin_balance' || type === 'admin_concurrency'
}

const getHistoryItemTitle = (item: RedeemHistoryItem) => {
  if (item.type === 'balance') {
    return t('redeem.balanceAddedRedeem')
  } else if (item.type === 'admin_balance') {
    return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
  } else if (item.type === 'concurrency') {
    return t('redeem.concurrencyAddedRedeem')
  } else if (item.type === 'admin_concurrency') {
    return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
  } else if (item.type === 'subscription') {
    return t('redeem.subscriptionAssigned')
  }
  return t('common.unknown')
}

const historyIcon = (item: RedeemHistoryItem) => {
  if (isBalanceType(item.type)) return 'dollar'
  if (isSubscriptionType(item.type)) return 'badge'
  return 'bolt'
}

const historyBadgeTone = (item: RedeemHistoryItem): 'accent' | 'success' | 'danger' => {
  if (isSubscriptionType(item.type)) return 'accent'
  return item.value >= 0 ? 'success' : 'danger'
}

const historyValueClass = (item: RedeemHistoryItem) => {
  if (isSubscriptionType(item.type)) return 'text-accent'
  return item.value >= 0 ? 'text-success-text' : 'text-danger-text'
}

const formatHistoryValue = (item: RedeemHistoryItem) => {
  if (isBalanceType(item.type)) {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}$${item.value.toFixed(2)}`
  } else if (isSubscriptionType(item.type)) {
    // 订阅类型显示有效天数和分组名称
    const days = item.validity_days || Math.round(item.value)
    const groupName = item.group?.name || ''
    return groupName ? `${days}${t('redeem.days')} - ${groupName}` : `${days}${t('redeem.days')}`
  } else {
    const sign = item.value >= 0 ? '+' : ''
    return `${sign}${item.value} ${t('redeem.requests')}`
  }
}

const fetchHistory = async () => {
  loadingHistory.value = true
  try {
    const res = await redeemAPI.getHistoryPaginated(historyPage.value, historyPageSize.value, historyType.value || undefined)
    history.value = res.items || []
    historyTotal.value = res.total || 0
    totalRecharged.value = res.total_recharged || 0
  } catch (error) {
    console.error('Failed to fetch history:', error)
  } finally {
    loadingHistory.value = false
  }
}

const handleHistoryTypeChange = (value: string | number | boolean | null) => {
  historyType.value = typeof value === 'string' ? value : ''
  historyPage.value = 1
  fetchHistory()
}

const handleHistoryPageChange = (page: number) => {
  historyPage.value = page
  fetchHistory()
}

const handleRedeem = async () => {
  if (!redeemCode.value.trim()) {
    appStore.showError(t('redeem.pleaseEnterCode'))
    return
  }

  submitting.value = true
  errorMessage.value = ''
  redeemResult.value = null

  try {
    const result = await redeemAPI.redeem(redeemCode.value.trim())

    redeemResult.value = result

    // Refresh user data to get updated balance/concurrency
    await authStore.refreshUser()

    // If subscription type, immediately refresh subscription status
    if (result.type === 'subscription') {
      try {
        await subscriptionStore.fetchActiveSubscriptions(true) // force refresh
      } catch (error) {
        console.error('Failed to refresh subscriptions after redeem:', error)
        appStore.showWarning(t('redeem.subscriptionRefreshFailed'))
      }
    }

    // Clear the input
    redeemCode.value = ''

    // Refresh history
    historyPage.value = 1
    await fetchHistory()

    // Show success toast
    appStore.showSuccess(t('redeem.codeRedeemSuccess'))
  } catch (error: any) {
    errorMessage.value = error.response?.data?.detail || t('redeem.failedToRedeem')

    appStore.showError(t('redeem.redeemFailed'))
  } finally {
    submitting.value = false
  }
}

onMounted(async () => {
  fetchHistory()
  try {
    const settings = await authAPI.getPublicSettings()
    contactInfo.value = settings.contact_info || ''
  } catch (error) {
    console.error('Failed to load contact info:', error)
  }
})
</script>

<style scoped>
.redeem-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 720px;
  margin-inline: auto;
}

.redeem-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}

.redeem-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.redeem-code-field {
  height: 36px;
  font-family: var(--font-mono);
  letter-spacing: 0.04em;
}

.redeem-submit-btn {
  width: 100%;
}

.notice-icon {
  flex: none;
  margin-top: 1px;
}

.notice-title {
  margin-bottom: 2px;
  font-size: var(--fs-13);
  font-weight: var(--fw-semibold);
}

.redeem-rules {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
  padding-left: 18px;
  list-style: disc;
  font-size: var(--fs-12-5);
  color: var(--muted);
}

.redeem-history-card {
  padding: 0;
  overflow: hidden;
}

.redeem-history-card .card-header {
  padding: 16px 20px 12px;
}

.redeem-history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.redeem-history-filter {
  flex: none;
}

.rh-type-cell {
  display: flex;
  align-items: center;
}

.rh-code,
.rh-muted {
  color: var(--muted);
}

.redeem-pagination {
  padding: 10px 20px;
  border-top: 1px solid var(--border);
}

@media (max-width: 640px) {
  .redeem-stats {
    grid-template-columns: minmax(0, 1fr);
  }

  .redeem-history-header {
    flex-direction: column;
    align-items: stretch;
  }

  .redeem-history-filter {
    width: 100%;
  }
}

.fade-enter-active,
.fade-leave-active {
  transition: all 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
