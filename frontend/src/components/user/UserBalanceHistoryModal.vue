<template>
  <BaseDialog
    :show="show"
    :title="t('admin.users.balanceHistoryTitle')"
    width="wide"
    :close-on-click-outside="true"
    :z-index="40"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-700">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-full bg-primary-100 dark:bg-primary-900/30">
            <span class="text-lg font-medium text-primary-700 dark:text-primary-300">
              {{ avatarInitial }}
            </span>
          </div>
          <div class="min-w-0 flex-1">
            <p class="truncate font-medium text-gray-900 dark:text-white">{{ email || '-' }}</p>
            <p class="text-xs text-gray-400 dark:text-dark-500">
              {{ t('admin.users.balanceHistory') }}
            </p>
          </div>
          <div class="flex-shrink-0 text-right">
            <p class="text-xs text-gray-500 dark:text-dark-400">{{ t('redeem.currentBalance') }}</p>
            <p class="text-xl font-bold text-gray-900 dark:text-white">
              ${{ balance.toFixed(2) }}
            </p>
          </div>
        </div>
        <div class="mt-2.5 flex items-center justify-end border-t border-gray-200/60 pt-2.5 dark:border-dark-600/60">
          <p class="text-xs text-gray-500 dark:text-dark-400">
            {{ t('admin.users.totalRecharged') }}:
            <span class="font-semibold text-emerald-600 dark:text-emerald-400">${{ totalRecharged.toFixed(2) }}</span>
          </p>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <Select
          v-model="typeFilter"
          :options="typeOptions"
          class="w-56"
          @change="loadHistory(1)"
        />
      </div>

      <div v-if="loading" class="flex justify-center py-8">
        <svg class="h-8 w-8 animate-spin text-primary-500" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      </div>

      <div v-else-if="history.length === 0" class="py-8 text-center">
        <p class="text-sm text-gray-500">{{ t('admin.users.noBalanceHistory') }}</p>
      </div>

      <div v-else class="max-h-[28rem] space-y-3 overflow-y-auto">
        <div
          v-for="item in history"
          :key="item.id"
          class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="flex items-start justify-between gap-4">
            <div class="flex items-start gap-3">
              <div
                :class="[
                  'flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg',
                  getIconBg(item)
                ]"
              >
                <Icon :name="getIconName(item)" size="sm" :class="getIconColor(item)" />
              </div>
              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">
                  {{ getItemTitle(item) }}
                </p>
                <p
                  v-if="item.notes"
                  class="mt-0.5 text-xs text-gray-500 dark:text-dark-400"
                  :title="item.notes"
                >
                  {{ item.notes.length > 60 ? `${item.notes.slice(0, 55)}...` : item.notes }}
                </p>
                <p class="mt-0.5 text-xs text-gray-400 dark:text-dark-500">
                  {{ formatDateTime(item.used_at || item.created_at) }}
                </p>
              </div>
            </div>
            <div class="text-right">
              <p :class="['text-sm font-semibold', getValueColor(item)]">
                {{ formatValue(item) }}
              </p>
              <p
                v-if="isAdminType(item.type)"
                class="text-xs text-gray-400 dark:text-dark-500"
              >
                {{ t('redeem.adminAdjustment') }}
              </p>
              <p
                v-else
                class="font-mono text-xs text-gray-400 dark:text-dark-500"
              >
                {{ item.code.slice(0, 8) }}...
              </p>
            </div>
          </div>
        </div>
      </div>

      <div v-if="totalPages > 1" class="flex items-center justify-center gap-2 pt-2">
        <button
          :disabled="currentPage <= 1"
          class="btn btn-secondary px-3 py-1 text-sm"
          @click="loadHistory(currentPage - 1)"
        >
          {{ t('pagination.previous') }}
        </button>
        <span class="text-sm text-gray-500 dark:text-dark-400">
          {{ currentPage }} / {{ totalPages }}
        </span>
        <button
          :disabled="currentPage >= totalPages"
          class="btn btn-secondary px-3 py-1 text-sm"
          @click="loadHistory(currentPage + 1)"
        >
          {{ t('pagination.next') }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { redeemAPI, type RedeemHistoryItem } from '@/api/redeem'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  show: boolean
  email?: string
  balance: number
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()

const history = ref<RedeemHistoryItem[]>([])
const loading = ref(false)
const currentPage = ref(1)
const total = ref(0)
const totalRecharged = ref(0)
const typeFilter = ref('')
const pageSize = 15

const avatarInitial = computed(() => (props.email?.charAt(0) || 'U').toUpperCase())
const totalPages = computed(() => Math.ceil(total.value / pageSize) || 1)
const typeOptions = computed(() => [
  { value: '', label: t('admin.users.allTypes') },
  { value: 'balance', label: t('admin.users.typeBalance') },
  { value: 'admin_balance', label: t('admin.users.typeAdminBalance') },
  { value: 'concurrency', label: t('admin.users.typeConcurrency') },
  { value: 'admin_concurrency', label: t('admin.users.typeAdminConcurrency') },
  { value: 'subscription', label: t('admin.users.typeSubscription') }
])

watch(
  () => props.show,
  (show) => {
    if (show) {
      typeFilter.value = ''
      loadHistory(1)
    }
  }
)

async function loadHistory(page: number) {
  loading.value = true
  currentPage.value = page
  try {
    const res = await redeemAPI.getHistoryPaginated(page, pageSize, typeFilter.value || undefined)
    history.value = res.items || []
    total.value = res.total || 0
    totalRecharged.value = res.total_recharged || 0
  } catch (error) {
    console.error('Failed to load balance history:', error)
  } finally {
    loading.value = false
  }
}

function isAdminType(type: string) {
  return type === 'admin_balance' || type === 'admin_concurrency'
}

function isBalanceType(type: string) {
  return type === 'balance' || type === 'admin_balance'
}

function isSubscriptionType(type: string) {
  return type === 'subscription'
}

function getIconName(item: RedeemHistoryItem) {
  if (isBalanceType(item.type)) return 'dollar'
  if (isSubscriptionType(item.type)) return 'badge'
  return 'bolt'
}

function getIconBg(item: RedeemHistoryItem) {
  if (isBalanceType(item.type)) {
    return item.value >= 0
      ? 'bg-emerald-100 dark:bg-emerald-900/30'
      : 'bg-red-100 dark:bg-red-900/30'
  }
  if (isSubscriptionType(item.type)) {
    return 'bg-purple-100 dark:bg-purple-900/30'
  }
  return item.value >= 0
    ? 'bg-blue-100 dark:bg-blue-900/30'
    : 'bg-orange-100 dark:bg-orange-900/30'
}

function getIconColor(item: RedeemHistoryItem) {
  if (isBalanceType(item.type)) {
    return item.value >= 0
      ? 'text-emerald-600 dark:text-emerald-400'
      : 'text-red-600 dark:text-red-400'
  }
  if (isSubscriptionType(item.type)) {
    return 'text-purple-600 dark:text-purple-400'
  }
  return item.value >= 0
    ? 'text-blue-600 dark:text-blue-400'
    : 'text-orange-600 dark:text-orange-400'
}

function getValueColor(item: RedeemHistoryItem) {
  if (isSubscriptionType(item.type)) {
    return 'text-purple-600 dark:text-purple-400'
  }
  return item.value >= 0
    ? 'text-emerald-600 dark:text-emerald-400'
    : 'text-red-600 dark:text-red-400'
}

function getItemTitle(item: RedeemHistoryItem) {
  switch (item.type) {
    case 'balance':
      return t('redeem.balanceAddedRedeem')
    case 'admin_balance':
      return item.value >= 0 ? t('redeem.balanceAddedAdmin') : t('redeem.balanceDeductedAdmin')
    case 'concurrency':
      return t('redeem.concurrencyAddedRedeem')
    case 'admin_concurrency':
      return item.value >= 0 ? t('redeem.concurrencyAddedAdmin') : t('redeem.concurrencyReducedAdmin')
    case 'subscription':
      if (item.group?.name) {
        return t('redeem.subscriptionAssignedDesc', { groupName: item.group.name })
      }
      return t('redeem.subscriptionAssigned')
    default:
      return item.type
  }
}

function formatValue(item: RedeemHistoryItem) {
  if (isBalanceType(item.type)) {
    return `${item.value >= 0 ? '+' : ''}$${Math.abs(item.value).toFixed(2)}`
  }
  if (isSubscriptionType(item.type)) {
    return item.validity_days ? t('redeem.subscriptionDays', { days: item.validity_days }) : t('redeem.subscriptionAssigned')
  }
  return `${item.value >= 0 ? '+' : ''}${Math.abs(item.value)}`
}
</script>
