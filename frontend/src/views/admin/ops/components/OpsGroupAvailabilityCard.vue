<script setup lang="ts">
import { ref, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useDebounceFn } from '@vueuse/core'
import { opsAPI } from '@/api/admin/ops'
import type { GroupAvailabilityStatus, GroupAvailabilityConfig, AlertSeverity } from '../types'
import Select from '@/components/common/Select.vue'
import ElPagination from '@/components/common/Pagination.vue'

const { t } = useI18n()
const router = useRouter()

const loading = ref(false)
const groups = ref<GroupAvailabilityStatus[]>([])

// Search and filter
const searchQuery = ref('')
const monitoringFilter = ref<'all' | 'enabled' | 'disabled'>('all')
const alertFilter = ref<'all' | 'ok' | 'firing'>('all')

// Pagination
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)

async function fetchData() {
  loading.value = true
  try {
    const response = await opsAPI.listGroupAvailabilityStatus({
      search: searchQuery.value || undefined,
      monitoring: monitoringFilter.value === 'all' ? undefined : monitoringFilter.value,
      alert: alertFilter.value === 'all' ? undefined : alertFilter.value,
      page: currentPage.value,
      page_size: pageSize.value
    })

    groups.value = response.items
    total.value = response.total
    currentPage.value = response.page
  } catch (err) {
    console.error('[OpsGroupAvailabilityCard] Failed to fetch group availability status', err)
  } finally {
    loading.value = false
  }
}

function goToGroupManagement() {
  router.push('/admin/groups')
}

function formatThresholdSummary(config: GroupAvailabilityConfig): string {
  const mode = config.threshold_mode
  if (mode === 'count') {
    return t('admin.ops.availability.thresholdSummary.count', { n: config.min_available_accounts })
  }
  if (mode === 'percentage') {
    return t('admin.ops.availability.thresholdSummary.percentage', { p: config.min_available_percentage ?? 0 })
  }
  return t('admin.ops.availability.thresholdSummary.both', {
    n: config.min_available_accounts,
    p: config.min_available_percentage ?? 0
  })
}

function formatSeverity(severity: AlertSeverity): string {
  return t(`common.${severity}`)
}

function formatStrategy(group: GroupAvailabilityStatus): { label: string; className: string } {
  if (!group.monitoring_enabled) {
    return {
      label: t('admin.ops.availability.unmonitored'),
      className: 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300'
    }
  }

  if (!group.config) {
    return {
      label: t('admin.ops.availability.notConfigured'),
      className: 'border-amber-200 bg-amber-50 text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200'
    }
  }

  return {
    label: `${formatThresholdSummary(group.config)} · ${formatSeverity(group.config.severity)}`,
    className: 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-900/50 dark:bg-blue-900/20 dark:text-blue-200'
  }
}

const handleSearch = () => {
  currentPage.value = 1
  fetchData()
}

const handleSearchDebounced = useDebounceFn(() => {
  currentPage.value = 1
  fetchData()
}, 400)

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchData()
}

const handleSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  fetchData()
}

onMounted(() => {
  fetchData()
})

watch(searchQuery, () => {
  handleSearchDebounced()
})

watch(monitoringFilter, () => {
  handleSearch()
})

watch(alertFilter, () => {
  handleSearch()
})
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
      <div class="flex items-center gap-4">
        <div class="flex h-10 w-10 items-center justify-center rounded-2xl bg-green-50 dark:bg-green-900/20">
          <svg class="h-6 w-6 text-green-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <div>
          <h3 class="text-base font-black text-gray-900 dark:text-white">
            {{ t('admin.ops.availability.title') }}
          </h3>
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
            {{ t('admin.ops.availability.recentCount', { n: total }) }}
          </p>
        </div>
      </div>

      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
        <button
          @click="goToGroupManagement"
          class="inline-flex items-center justify-center gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2 text-xs font-bold text-gray-700 shadow-sm transition-all hover:border-blue-300 hover:bg-blue-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-blue-900/20"
        >
          {{ t('admin.ops.availability.goToGroupManagement') }}
        </button>

        <button
          @click="fetchData"
          :disabled="loading"
          class="inline-flex items-center justify-center gap-2 rounded-xl border border-gray-200 bg-white px-4 py-2 text-xs font-bold text-gray-700 shadow-sm transition-all hover:border-blue-300 hover:bg-blue-50 disabled:opacity-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-blue-900/20"
        >
          <svg class="h-4 w-4" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <div class="lg:col-span-1">
        <div class="group relative">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
            <svg class="h-4 w-4 text-gray-400 transition-colors group-focus-within:text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
          <input
            v-model="searchQuery"
            @keyup.enter="handleSearch"
            type="text"
            :placeholder="t('admin.ops.availability.searchPlaceholder')"
            class="w-full rounded-2xl border-gray-200 bg-gray-50/50 py-2.5 pl-10 pr-4 text-sm font-medium text-gray-700 transition-all focus:border-blue-500 focus:bg-white focus:ring-4 focus:ring-blue-500/10 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:focus:bg-dark-800"
          />
        </div>
      </div>

      <Select
        v-model="monitoringFilter"
        :options="[
          { value: 'all', label: t('admin.ops.availability.filters.allMonitoring') },
          { value: 'enabled', label: t('admin.ops.availability.filters.monitoringEnabled') },
          { value: 'disabled', label: t('admin.ops.availability.filters.monitoringDisabled') }
        ]"
        @change="handleSearch"
      />

      <Select
        v-model="alertFilter"
        :options="[
          { value: 'all', label: t('admin.ops.availability.filters.allAlerts') },
          { value: 'ok', label: t('admin.ops.availability.filters.alertOk') },
          { value: 'firing', label: t('admin.ops.availability.filters.alertFiring') }
        ]"
        @change="handleSearch"
      />

      <button
        @click="handleSearch"
        :disabled="loading"
        class="rounded-2xl bg-blue-500 px-4 py-2.5 text-sm font-bold text-white transition-all hover:bg-blue-600 focus:ring-4 focus:ring-blue-500/20 disabled:opacity-50 dark:bg-blue-600 dark:hover:bg-blue-700"
      >
        {{ t('common.search') }}
      </button>
    </div>

    <div class="relative overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
      <div v-if="loading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/60 backdrop-blur-sm dark:bg-dark-900/60">
        <div class="flex flex-col items-center gap-3">
          <div class="relative">
            <div class="h-10 w-10 rounded-full border-4 border-gray-200 dark:border-dark-700"></div>
            <div class="absolute top-0 h-10 w-10 animate-spin rounded-full border-4 border-blue-500 border-t-transparent"></div>
          </div>
          <span class="text-xs font-black uppercase tracking-widest text-gray-500 dark:text-dark-400">{{ t('admin.ops.availability.loading') }}</span>
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <caption class="sr-only">{{ t('admin.ops.availability.title') }}</caption>
          <thead>
            <tr class="bg-gray-50/50 dark:bg-dark-800/50">
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.availability.groupName') }}
              </th>
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-center text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.availability.waterLevel') }}
              </th>
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-center text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.availability.monitoringSwitch') }}
              </th>
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-right text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.availability.alertStrategy') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-if="groups.length === 0 && !loading">
              <td colspan="4" class="py-24 text-center">
                <div class="flex flex-col items-center gap-3">
                  <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-50 text-gray-300 dark:bg-dark-800">
                    <svg class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
                    </svg>
                  </div>
                  <p class="text-sm font-bold text-gray-400 dark:text-dark-500">{{ t('admin.ops.availability.noData') }}</p>
                </div>
              </td>
            </tr>
            <tr
              v-for="group in groups"
              :key="group.group_id"
              class="group transition-all duration-200 hover:bg-gray-50/80 dark:hover:bg-dark-800/50"
            >
              <td class="px-6 py-4">
                <div class="flex items-center gap-3">
                  <span
                    class="h-2.5 w-2.5 flex-shrink-0 rounded-full"
                    :class="[
                      !group.monitoring_enabled ? 'bg-gray-300' : group.is_healthy ? 'bg-green-500' : 'animate-pulse bg-red-500'
                    ]"
                  ></span>
                  <span class="text-sm font-bold text-gray-900 dark:text-white">
                    {{ group.group_name }}
                  </span>
                </div>
              </td>

              <td class="px-6 py-4">
                <div class="flex flex-col items-center gap-2">
                  <div class="text-sm font-black text-gray-700 dark:text-gray-300">
                    {{ group.available_accounts }} <span class="text-gray-400">/ {{ group.total_accounts }}</span>
                  </div>
                  <div class="h-2 w-24 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600">
                    <div
                      class="h-full rounded-full transition-all"
                      :class="group.is_healthy ? 'bg-green-500' : 'bg-red-500'"
                      :style="{ width: `${(group.available_accounts / Math.max(group.total_accounts, 1)) * 100}%` }"
                    ></div>
                  </div>
                </div>
              </td>

              <td class="px-6 py-4">
                <div class="flex justify-center">
                  <span
                    class="inline-flex items-center rounded-full border px-3 py-1 text-xs font-bold"
                    :class="group.monitoring_enabled
                      ? 'border-green-200 bg-green-50 text-green-700 dark:border-green-900/50 dark:bg-green-900/20 dark:text-green-200'
                      : 'border-gray-200 bg-gray-50 text-gray-700 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300'"
                  >
                    {{ group.monitoring_enabled ? t('admin.ops.availability.filters.monitoringEnabled') : t('admin.ops.availability.filters.monitoringDisabled') }}
                  </span>
                </div>
              </td>

              <td class="px-6 py-4">
                <div class="flex items-center justify-end">
                  <span class="inline-flex items-center rounded-xl border px-3 py-2 text-xs font-bold" :class="formatStrategy(group).className">
                    {{ formatStrategy(group).label }}
                  </span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div class="mt-6 flex items-center justify-between border-t border-gray-100 pt-6 dark:border-dark-700">
      <div class="text-xs font-medium text-gray-500 dark:text-dark-400">
        {{ t('pagination.showing') }} {{ (currentPage - 1) * pageSize + 1 }} {{ t('pagination.to') }} {{ Math.min(currentPage * pageSize, total) }} {{ t('pagination.of') }} {{ total }}
      </div>
      <ElPagination
        :total="total"
        :page="currentPage"
        :page-size="pageSize"
        @update:page="handlePageChange"
        @update:pageSize="handleSizeChange"
      />
    </div>
  </div>
</template>
