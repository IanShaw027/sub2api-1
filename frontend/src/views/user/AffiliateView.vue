<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex justify-center py-12">
        <div
          class="h-8 w-8 animate-spin rounded-full border-2 border-primary-500 border-t-transparent"
        ></div>
      </div>

      <template v-else-if="detail">
        <div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
          <div class="card p-5">
            <p class="flex items-center gap-1.5 text-sm text-gray-500 dark:text-dark-400">
              <Icon name="dollar" size="sm" class="text-primary-500" />
              {{ t('affiliate.stats.rebateRate') }}
            </p>
            <p class="mt-2 text-2xl font-semibold text-primary-600 dark:text-primary-400">
              {{ formattedRebateRate }}<span class="ml-0.5 text-base font-medium">%</span>
            </p>
            <p class="mt-1 text-xs text-gray-400 dark:text-dark-500">
              {{ t('affiliate.stats.rebateRateHint') }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.invitedUsers') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCount(detail.invited_count ?? detail.aff_count) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.rebatedInvitees') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCount(detail.rebated_invitee_count || 0) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.remainingSlots') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ detail.remaining_rebate_slots == null ? t('affiliate.stats.unlimited') : formatCount(detail.remaining_rebate_slots) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.availableQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-emerald-600 dark:text-emerald-400">
              {{ formatCurrency(detail.aff_quota) }}
            </p>
          </div>
          <div class="card p-5">
            <p class="text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.stats.totalQuota') }}</p>
            <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">
              {{ formatCurrency(detail.aff_history_quota) }}
            </p>
            <p v-if="detail.aff_frozen_quota > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
              {{ t('affiliate.stats.frozenQuota') }}: {{ formatCurrency(detail.aff_frozen_quota) }}
            </p>
          </div>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.title') }}</h3>
          <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.description') }}</p>

          <div class="mt-5 grid gap-4 md:grid-cols-2">
            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.yourCode') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm font-semibold text-gray-900 dark:text-white">{{ detail.aff_code }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyCode">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyCode') }}</span>
                </button>
              </div>
            </div>

            <div class="space-y-2">
              <p class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('affiliate.inviteLink') }}</p>
              <div class="flex items-center gap-2 rounded-xl border border-gray-200 bg-gray-50 px-3 py-2 dark:border-dark-700 dark:bg-dark-900">
                <code class="flex-1 truncate text-sm text-gray-700 dark:text-gray-300">{{ inviteLink }}</code>
                <button class="btn btn-secondary btn-sm" @click="copyInviteLink">
                  <Icon name="copy" size="sm" />
                  <span>{{ t('affiliate.copyLink') }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="mt-5 rounded-xl border border-primary-200 bg-primary-50 p-4 dark:border-primary-900/40 dark:bg-primary-900/20">
            <p class="text-sm font-medium text-primary-800 dark:text-primary-200">{{ t('affiliate.policy.title') }}</p>
            <p v-if="policyText" class="mt-2 text-sm leading-6 text-primary-700 dark:text-primary-300">
              {{ policyText }}
            </p>
            <ul class="mt-2 space-y-1 text-sm text-primary-700 dark:text-primary-300">
              <li>{{ t('affiliate.policy.rate', { rate: formatPercent(detail.policy.rebate_rate) }) }}</li>
              <li>{{ detail.policy.rebate_cap > 0 ? t('affiliate.policy.capLimited', { amount: formatCurrency(detail.policy.rebate_cap) }) : t('affiliate.policy.capUnlimited') }}</li>
              <li>{{ detail.policy.invitee_limit > 0 ? t('affiliate.policy.inviteeLimited', { count: detail.policy.invitee_limit }) : t('affiliate.policy.inviteeUnlimited') }}</li>
              <li v-if="detail.policy.signup_bonus > 0">{{ t('affiliate.policy.signupBonus', { amount: formatCurrency(detail.policy.signup_bonus) }) }}</li>
              <li>1. {{ t('affiliate.tips.line1') }}</li>
              <li>2. {{ t('affiliate.tips.line2', { rate: `${formattedRebateRate}%` }) }}</li>
              <li>3. {{ t('affiliate.tips.line3') }}</li>
              <li v-if="detail.aff_frozen_quota > 0">4. {{ t('affiliate.tips.line4') }}</li>
            </ul>
            <div class="mt-4 rounded-lg border border-primary-200/70 bg-white/70 p-3 text-xs leading-5 text-primary-700 dark:border-primary-900/30 dark:bg-dark-950/30 dark:text-primary-200">
              <p class="font-medium">{{ t('affiliate.tips.title') }}</p>
              <ul class="mt-1 space-y-1">
                <li>1. {{ t('affiliate.tips.line1') }}</li>
                <li>2. {{ t('affiliate.tips.line2', { rate: formatPercent(detail.policy.rebate_rate) }) }}</li>
                <li>3. {{ t('affiliate.tips.line3') }}</li>
              </ul>
            </div>
          </div>
        </div>

        <div class="card p-6">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.transfer.title') }}</h3>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ t('affiliate.transfer.description') }}</p>
            </div>
            <button
              class="btn btn-primary"
              :disabled="transferring || detail.aff_quota <= 0"
              @click="transferQuota"
            >
              <Icon v-if="transferring" name="refresh" size="sm" class="animate-spin" />
              <Icon v-else name="dollar" size="sm" />
              <span>{{ transferring ? t('affiliate.transfer.transferring') : t('affiliate.transfer.button') }}</span>
            </button>
          </div>
          <p v-if="detail.aff_quota <= 0" class="mt-3 text-sm text-amber-600 dark:text-amber-400">
            {{ t('affiliate.transfer.empty') }}
          </p>
        </div>

        <div class="card p-6">
          <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.invitees.title') }}</h3>
          <div v-if="detail.invitees.length === 0" class="mt-4 rounded-xl border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400">
            {{ t('affiliate.invitees.empty') }}
          </div>
          <div v-else class="mt-4 overflow-x-auto">
            <table class="w-full min-w-[560px] text-left text-sm">
              <thead>
                <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.email') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.username') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.joinedAt') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.consumed') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.rebate') }}</th>
                  <th class="px-3 py-2 font-medium">{{ t('affiliate.invitees.columns.details') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="item in detail.invitees"
                  :key="item.user_id"
                  class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
                >
                  <td class="px-3 py-3 text-gray-900 dark:text-white">{{ item.email || '-' }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ item.username || '-' }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</td>
                  <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatCurrency(item.total_consumed || 0) }}</td>
                  <td class="px-3 py-3 text-right font-medium text-emerald-600 dark:text-emerald-400">{{ formatCurrency(item.total_rebate || 0) }}</td>
                  <td class="px-3 py-3">
                    <button class="btn btn-secondary btn-sm" @click="openLedger(item.user_id)">
                      {{ t('affiliate.invitees.viewDetails') }}
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div v-if="ledgerOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4" @click.self="closeLedger">
          <div class="max-h-[85vh] w-full max-w-3xl overflow-y-auto rounded-2xl bg-white p-6 shadow-xl dark:bg-dark-900">
            <div class="mb-4 flex items-center justify-between">
              <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('affiliate.ledger.title') }}</h3>
              <button class="btn btn-secondary btn-sm" @click="closeLedger">{{ t('common.close') }}</button>
            </div>
            <div v-if="ledgerLoading" class="py-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
            <div v-else-if="ledgerItems.length === 0" class="py-8 text-center text-sm text-gray-500">{{ t('affiliate.ledger.empty') }}</div>
            <div v-else class="overflow-x-auto">
              <table class="w-full min-w-[620px] text-left text-sm">
                <thead>
                  <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
                    <th class="px-3 py-2">{{ t('affiliate.ledger.columns.time') }}</th>
                    <th class="px-3 py-2">{{ t('affiliate.ledger.columns.consumed') }}</th>
                    <th class="px-3 py-2">{{ t('affiliate.ledger.columns.rate') }}</th>
                    <th class="px-3 py-2">{{ t('affiliate.ledger.columns.rebate') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="item in ledgerItems" :key="item.id" class="border-b border-gray-100 last:border-0 dark:border-dark-800">
                    <td class="px-3 py-3">{{ formatDateTime(item.created_at) }}</td>
                    <td class="px-3 py-3">{{ formatCurrency(item.base_amount) }}</td>
                    <td class="px-3 py-3">{{ item.rebate_rate }}%</td>
                    <td class="px-3 py-3">{{ formatCurrency(item.amount) }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
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
import Icon from '@/components/icons/Icon.vue'
import userAPI from '@/api/user'
import type { AffiliateLedgerEntry, UserAffiliateDetail } from '@/types'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useClipboard } from '@/composables/useClipboard'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import { buildAppAbsoluteUrl, buildAppPath } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const { copyToClipboard } = useClipboard()

const loading = ref(true)
const transferring = ref(false)
const detail = ref<UserAffiliateDetail | null>(null)
const ledgerOpen = ref(false)
const ledgerLoading = ref(false)
const ledgerItems = ref<AffiliateLedgerEntry[]>([])
let activeLedgerRequestID = 0

const inviteLink = computed(() => {
  if (!detail.value) return ''
  const registerTarget = `/register?aff=${encodeURIComponent(detail.value.aff_code)}`
  const registerPath = buildAppPath(registerTarget)
  if (typeof window === 'undefined') return registerPath
  return buildAppAbsoluteUrl(registerTarget, window.location.origin)
})

// Rebate rate is a percentage in the range [0, 100]; backend already clamps it.
// We trim trailing zeros (e.g. 20.00 → "20", 12.50 → "12.5") for a cleaner UI.
const formattedRebateRate = computed(() => {
  const v = detail.value?.effective_rebate_rate_percent ?? 0
  const rounded = Math.round(v * 100) / 100
  return Number.isInteger(rounded) ? String(rounded) : rounded.toString()
})

const policyText = computed(() => detail.value?.policy.policy_text?.trim() || '')

function formatCount(value: number): string {
  return value.toLocaleString()
}

function formatPercent(value: number): string {
  return `${Number(value || 0).toFixed(2)}%`
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
    await Promise.all([
      loadAffiliateDetail(true),
      authStore.refreshUser({ touchActive: true }).catch(() => undefined),
    ])
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('affiliate.transferFailed')))
  } finally {
    transferring.value = false
  }
}

async function openLedger(inviteeId: number): Promise<void> {
  const requestID = ++activeLedgerRequestID
  ledgerOpen.value = true
  ledgerLoading.value = true
  ledgerItems.value = []
  try {
    const items = await userAPI.getAffiliateInviteeLedger(inviteeId)
    if (requestID !== activeLedgerRequestID) {
      return
    }
    ledgerItems.value = items
  } catch (error) {
    if (requestID !== activeLedgerRequestID) {
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('affiliate.ledger.loadFailed')))
  } finally {
    if (requestID === activeLedgerRequestID) {
      ledgerLoading.value = false
    }
  }
}

function closeLedger(): void {
  activeLedgerRequestID++
  ledgerOpen.value = false
  ledgerLoading.value = false
}

onMounted(() => {
  void loadAffiliateDetail()
})
</script>
