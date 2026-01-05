<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { opsAPI, type AccountStatusSummary } from '@/api/admin/ops'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const accounts = ref<AccountStatusSummary[]>([])

const search = ref('')
const sortKey = ref<'error_1h' | 'error_24h' | 'timeout_1h' | 'rate_limit_1h'>('error_1h')

const sortOptions = computed(() => [
  { value: 'error_1h', label: t('admin.ops.accountStatus.sort.error1h') },
  { value: 'error_24h', label: t('admin.ops.accountStatus.sort.error24h') },
  { value: 'timeout_1h', label: t('admin.ops.accountStatus.sort.timeout1h') },
  { value: 'rate_limit_1h', label: t('admin.ops.accountStatus.sort.rateLimit1h') }
])

function safeInt(n: unknown): number {
  return typeof n === 'number' && Number.isFinite(n) ? n : 0
}

async function load() {
  loading.value = true
  try {
    const res = await opsAPI.getAllAccountStatus()
    accounts.value = res.accounts || []
  } catch (err: any) {
    console.error('[OpsAccountStatusCard] Failed to load account status', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.accountStatus.loadFailed'))
    accounts.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
})

const filtered = computed(() => {
  const q = search.value.trim()
  if (!q) return accounts.value
  return accounts.value.filter(a => String(a.account_id).includes(q))
})

const sorted = computed(() => {
  const rows = filtered.value
  const key = sortKey.value
  const score = (a: AccountStatusSummary): number => {
    if (key === 'error_1h') return safeInt(a.stats_1h?.error_count)
    if (key === 'error_24h') return safeInt(a.stats_24h?.error_count)
    if (key === 'timeout_1h') return safeInt(a.stats_1h?.timeout_count)
    return safeInt(a.stats_1h?.rate_limit_count)
  }
  return [...rows].sort((a, b) => score(b) - score(a))
})

// Keep this cheap: show the top N; full browsing belongs to Accounts view.
const topN = computed(() => sorted.value.slice(0, 50))
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.accountStatus.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.accountStatus.description') }}</p>
      </div>

      <div class="flex items-center gap-2">
        <input
          v-model="search"
          type="text"
          class="h-8 w-[140px] rounded-lg border border-gray-200 bg-white px-3 text-xs text-gray-700 outline-none ring-0 focus:border-primary-400 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200"
          :placeholder="t('admin.ops.accountStatus.searchPlaceholder')"
        />
        <select
          v-model="sortKey"
          class="h-8 w-[160px] rounded-lg border border-gray-200 bg-white px-2 text-xs text-gray-700 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-200"
        >
          <option v-for="opt in sortOptions" :key="opt.value" :value="opt.value">{{ opt.label }}</option>
        </select>
        <button
          class="flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
          :disabled="loading"
          @click="load"
        >
          <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('admin.ops.accountStatus.loading') }}
    </div>

    <div v-else-if="topN.length === 0" class="rounded-xl border border-dashed border-gray-200 p-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
      {{ t('admin.ops.accountStatus.empty') }}
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-900">
            <tr>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.accountStatus.table.accountId') }}
              </th>
              <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.accountStatus.table.errors1h') }}
              </th>
              <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.accountStatus.table.timeouts1h') }}
              </th>
              <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.accountStatus.table.rateLimits1h') }}
              </th>
              <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.accountStatus.table.errors24h') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
            <tr v-for="row in topN" :key="row.account_id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
              <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-gray-700 dark:text-gray-200">
                {{ row.account_id }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-right text-xs font-bold text-gray-900 dark:text-white">
                {{ row.stats_1h?.error_count ?? 0 }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-right text-xs text-gray-600 dark:text-gray-300">
                {{ row.stats_1h?.timeout_count ?? 0 }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-right text-xs text-gray-600 dark:text-gray-300">
                {{ row.stats_1h?.rate_limit_count ?? 0 }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-right text-xs text-gray-600 dark:text-gray-300">
                {{ row.stats_24h?.error_count ?? 0 }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="border-t border-gray-200 px-4 py-3 text-[11px] text-gray-400 dark:border-dark-700">
        {{ t('admin.ops.accountStatus.footer', { shown: topN.length, total: accounts.length }) }}
      </div>
    </div>
  </div>
</template>

