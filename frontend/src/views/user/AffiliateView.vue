<template>
 <AppLayout>
 <PageHeader :title="t('affiliate.title')" :description="t('affiliate.description')" />
 <div class="affiliate-page">
 <div v-if="loading" class="flex justify-center py-12">
 <span class="spinner"></span>
 </div>

 <template v-else-if="detail">
 <!-- Rebate stats -->
 <div class="affiliate-stats">
 <StatCard :label="t('affiliate.stats.rebateRate')" :value="`${formattedRebateRate}%`" :sub="t('affiliate.stats.rebateRateHint')" />
 <StatCard :label="t('affiliate.stats.invitedUsers')" :value="formatCount(detail.aff_count)" />
 <StatCard :label="t('affiliate.stats.availableQuota')" :value="formatCurrency(detail.aff_quota)" />
 <StatCard
 :label="t('affiliate.stats.totalQuota')"
 :value="formatCurrency(detail.aff_history_quota)"
 :sub="detail.aff_frozen_quota > 0 ? `${t('affiliate.stats.frozenQuota')}: ${formatCurrency(detail.aff_frozen_quota)}` : undefined"
 />
 </div>

 <!-- Invite code / link -->
 <div class="glass-card">
 <div class="card-header">
 <p class="card-title">{{ t('affiliate.title') }}</p>
 <p class="card-subtitle">{{ t('affiliate.description') }}</p>
 </div>
 <div class="card-body">
 <div class="affiliate-copy-grid">
 <EndpointCard
 :label="t('affiliate.yourCode')"
 :url="detail.aff_code"
 :copy-label="t('affiliate.copyCode')"
 @copy="copyCode"
 />
 <EndpointCard
 :label="t('affiliate.inviteLink')"
 :url="inviteLink"
 :copy-label="t('affiliate.copyLink')"
 @copy="copyInviteLink"
 />
 </div>

 <div class="notice notice-info affiliate-tips">
 <Icon name="infoCircle" size="sm" class="notice-icon" />
 <div class="min-w-0 flex-1">
 <p class="notice-title">{{ t('affiliate.tips.title') }}</p>
 <ul class="affiliate-tips-list">
 <li>{{ t('affiliate.tips.line1') }}</li>
 <li>{{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
 <li>{{ t('affiliate.tips.line3') }}</li>
 <li v-if="detail.aff_frozen_quota > 0">{{ t('affiliate.tips.line4') }}</li>
 </ul>
 </div>
 </div>
 </div>
 </div>

 <!-- Transfer -->
 <div class="glass-card">
 <div class="card-body affiliate-transfer">
 <div class="min-w-0">
 <p class="card-title">{{ t('affiliate.transfer.title') }}</p>
 <p class="card-subtitle mt-1">{{ t('affiliate.transfer.description') }}</p>
 <p v-if="detail.aff_quota <= 0" class="mt-2 text-xs text-warning-text">{{ t('affiliate.transfer.empty') }}</p>
 </div>
 <button type="button" class="btn btn-primary" :disabled="transferring || detail.aff_quota <= 0" @click="showTransferConfirm = true">
 <Icon name="dollar" size="sm" />
 {{ t('affiliate.transfer.button') }}
 </button>
 </div>
 </div>

 <UiModal
 :open="showTransferConfirm"
 :title="t('affiliate.transfer.confirmTitle')"
 width="sm"
 :close-label="t('common.close')"
 :close-on-overlay="!transferring"
 @close="showTransferConfirm = false"
 >
 <p class="text-sm text-muted">
 {{ t('affiliate.transfer.confirmMessage', { amount: formatCurrency(detail.aff_quota) }) }}
 </p>
 <template #footer>
 <div class="flex justify-end gap-3">
 <button type="button" class="btn-secondary" :disabled="transferring" @click="showTransferConfirm = false">
 {{ t('common.cancel') }}
 </button>
 <button type="button" class="btn btn-primary" :disabled="transferring" @click="transferQuota">
 <span v-if="transferring" class="spinner" aria-hidden="true"></span>
 {{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.confirmButton') }}
 </button>
 </div>
 </template>
 </UiModal>

 <!-- Invitees / rebate records -->
 <div class="glass-card affiliate-table-card">
 <div class="card-header">
 <p class="card-title">{{ t('affiliate.invitees.title') }}</p>
 </div>

 <DataTable :columns="inviteeColumns" :data="pagedInvitees" row-key="user_id">
 <template #cell-email="{ row }">
 <span class="text-foreground">{{ row.email || '-' }}</span>
 </template>
 <template #cell-username="{ row }">
 <span class="text-foreground">{{ row.username || '-' }}</span>
 </template>
 <template #cell-total_rebate="{ row }">
 <span class="num font-semibold text-success-text">{{ formatCurrency(row.total_rebate) }}</span>
 </template>
 <template #cell-created_at="{ row }">
 <span class="text-mono rh-muted">{{ formatDateTime(row.created_at) || '-' }}</span>
 </template>
 <template #empty>
 <div class="empty-state">
 <Icon name="users" size="xl" class="empty-state-icon" />
 <p class="empty-state-description">{{ t('affiliate.invitees.empty') }}</p>
 </div>
 </template>
 </DataTable>

 <div v-if="detail.invitees.length > 0" class="affiliate-pagination">
 <UiPagination
 :page="inviteesPage"
 :page-size="inviteesPageSize"
 :total="detail.invitees.length"
 :page-size-options="[10, 20, 50]"
 @update:page="inviteesPage = $event"
 @update:page-size="handleInviteesPageSizeChange"
 />
 </div>
 </div>
 </template>
 </div>
 </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import StatCard from '@/components/ui/StatCard.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import UiModal from '@/components/ui/UiModal.vue'
import EndpointCard from '@/components/ui/EndpointCard.vue'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { UserAffiliateDetail } from '@/types'
import type { Column } from '@/components/common/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const showTransferConfirm = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)

const inviteesPage = ref(1)
const inviteesPageSize = ref(10)

const inviteeColumns = computed<Column[]>(() => [
  { key: 'email', label: t('affiliate.invitees.columns.email') },
  { key: 'username', label: t('affiliate.invitees.columns.username') },
  { key: 'total_rebate', label: t('affiliate.invitees.columns.rebate') },
  { key: 'created_at', label: t('affiliate.invitees.columns.joinedAt') },
])

const pagedInvitees = computed(() => {
  if (!detail.value) return []
  const start = (inviteesPage.value - 1) * inviteesPageSize.value
  return detail.value.invitees.slice(start, start + inviteesPageSize.value)
})

function handleInviteesPageSizeChange(size: number) {
  inviteesPageSize.value = size
  inviteesPage.value = 1
}

const inviteLink = computed(() => {
  if (!detail.value) return ''
  if (typeof window === 'undefined') return `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  return `${window.location.origin}/register?aff=${encodeURIComponent(detail.value.aff_code)}`
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

function formatCount(value: number): string {
  return value.toLocaleString()
}

async function loadAffiliateDetail(silent = false): Promise<void> {
  if (!silent) {
    loading.value = true
  }
  try {
    detail.value = await userAPI.getAffiliateDetail()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.loadFailed')))
  } finally {
    if (!silent) {
      loading.value = false
    }
  }
}

async function copyCode(): Promise<void> {
  if (!detail.value?.aff_code) return
  await copyToClipboard(detail.value.aff_code, t('affiliate.codeCopied'))
}

async function copyInviteLink(): Promise<void> {
  if (!inviteLink.value) return
  await copyToClipboard(inviteLink.value, t('affiliate.linkCopied'))
}

async function transferQuota(): Promise<void> {
  if (!detail.value || detail.value.aff_quota <= 0 || transferring.value) return
  transferring.value = true
  try {
    const resp = await userAPI.transferAffiliateQuota()
    appStore.showSuccess(t('affiliate.transfer.success', { amount: formatCurrency(resp.transferred_quota) }))
    showTransferConfirm.value = false
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser().catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>

<style scoped>
.affiliate-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-width: 1080px;
  margin-inline: auto;
}

.affiliate-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.affiliate-copy-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.affiliate-tips {
  margin-top: 16px;
}

.notice-icon {
  flex: none;
  margin-top: 1px;
}

.notice-title {
  margin-bottom: 4px;
  font-size: var(--fs-13);
  font-weight: var(--fw-semibold);
}

.affiliate-tips-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.affiliate-transfer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.affiliate-table-card {
  padding: 0;
  overflow: hidden;
}

.affiliate-table-card .card-header {
  padding: 16px 20px 12px;
}

.affiliate-pagination {
  padding: 10px 20px;
  border-top: 1px solid var(--border);
}

.rh-muted {
  font-size: var(--fs-12-5);
  color: var(--muted);
}

@media (max-width: 767px) {
  .affiliate-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .affiliate-stats {
    grid-template-columns: minmax(0, 1fr);
  }

  .affiliate-copy-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .affiliate-transfer {
    flex-direction: column;
    align-items: stretch;
  }
}
</style>
