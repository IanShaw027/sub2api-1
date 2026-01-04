<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI } from '@/api/admin/ops'
import type { GroupAvailabilityStatus, GroupAvailabilityConfig, AlertSeverity, ThresholdMode } from '../types'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ElPagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const groups = ref<GroupAvailabilityStatus[]>([])
const updatingId = ref<number | null>(null)

// Search and filter
const searchQuery = ref('')
const monitoringFilter = ref<'all' | 'enabled' | 'disabled'>('all')
const alertFilter = ref<'all' | 'ok' | 'firing'>('all')

// Pagination
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const totalPages = ref(1)

// Custom config dialog
const showCustomConfig = ref(false)
const editingGroup = ref<GroupAvailabilityStatus | null>(null)
const customConfig = ref<GroupAvailabilityConfig | null>(null)

// --- 策略模板定义 (Strategy Templates) ---
interface StrategyTemplate {
  key: string
  emoji: string
  labelKey: string
  descKey: string
  config: {
    threshold_mode: ThresholdMode
    min_available_accounts: number
    min_available_percentage?: number
    severity: AlertSeverity
    cooldown_minutes: number
    notify_email: boolean
  }
}

const strategies: StrategyTemplate[] = [
  {
    key: 'strict',
    emoji: '🛡️',
    labelKey: 'admin.ops.config.strictMode',
    descKey: 'admin.ops.config.strictModeDesc',
    config: { threshold_mode: 'both', min_available_accounts: 5, min_available_percentage: 80, severity: 'critical', cooldown_minutes: 15, notify_email: true }
  },
  {
    key: 'standard',
    emoji: '⚖️',
    labelKey: 'admin.ops.config.standardMode',
    descKey: 'admin.ops.config.standardModeDesc',
    config: { threshold_mode: 'percentage', min_available_accounts: 0, min_available_percentage: 60, severity: 'warning', cooldown_minutes: 30, notify_email: true }
  },
  {
    key: 'loose',
    emoji: '💤',
    labelKey: 'admin.ops.config.looseMode',
    descKey: 'admin.ops.config.looseModeDesc',
    config: { threshold_mode: 'count', min_available_accounts: 1, severity: 'info', cooldown_minutes: 60, notify_email: false }
  },
  {
    key: 'custom',
    emoji: '⚙️',
    labelKey: 'admin.ops.availability.customMode',
    descKey: 'admin.ops.availability.customModeDesc',
    config: { threshold_mode: 'count', min_available_accounts: 2, severity: 'warning', cooldown_minutes: 30, notify_email: true }
  }
]

const customSeverityOptions = computed(() => [
  { value: 'critical', label: t('common.critical') },
  { value: 'warning', label: t('common.warning') },
  { value: 'info', label: t('common.info') }
])

type UpsertPayload = Pick<
  GroupAvailabilityConfig,
  'enabled' | 'threshold_mode' | 'min_available_accounts' | 'min_available_percentage' | 'severity' | 'cooldown_minutes' | 'notify_email'
>

const defaultUpsertPayload: UpsertPayload = {
  enabled: true,
  threshold_mode: 'count',
  min_available_accounts: 3,
  min_available_percentage: 0,
  severity: 'warning',
  cooldown_minutes: 30,
  notify_email: true
}

function buildUpsertPayload(config: GroupAvailabilityConfig | undefined, patch: Partial<UpsertPayload>): UpsertPayload {
  return {
    ...defaultUpsertPayload,
    ...(config
      ? {
          enabled: config.enabled,
          threshold_mode: config.threshold_mode,
          min_available_accounts: config.min_available_accounts,
          min_available_percentage: config.min_available_percentage ?? 0,
          severity: config.severity,
          cooldown_minutes: config.cooldown_minutes,
          notify_email: config.notify_email
        }
      : {}),
    ...patch
  }
}

// 推断当前分组使用的是哪个策略
function detectStrategy(config: GroupAvailabilityConfig | undefined): string {
  if (!config) return 'standard'

  // Check for exact matches with predefined strategies
  for (const strategy of strategies) {
    if (strategy.key === 'custom') continue // Skip custom for exact matching

    const matches =
      config.threshold_mode === strategy.config.threshold_mode &&
      config.min_available_accounts === strategy.config.min_available_accounts &&
      (config.min_available_percentage ?? 0) === (strategy.config.min_available_percentage ?? 0) &&
      config.severity === strategy.config.severity &&
      config.cooldown_minutes === strategy.config.cooldown_minutes &&
      config.notify_email === strategy.config.notify_email

    if (matches) return strategy.key
  }

  // If no exact match, return custom
  return 'custom'
}

function getStrategyLabel(config: GroupAvailabilityConfig | undefined): string {
  const strategyKey = detectStrategy(config)
  const strategy = strategies.find(s => s.key === strategyKey)
  return strategy ? t(strategy.labelKey) : t('admin.ops.availability.customMode')
}

function getStrategyBadgeClass(config: GroupAvailabilityConfig | undefined): string {
  const strategyKey = detectStrategy(config)

  const classMap: Record<string, string> = {
    strict: 'border-red-200 bg-red-50 text-red-700 dark:border-red-800 dark:bg-red-900/20 dark:text-red-400',
    standard: 'border-blue-200 bg-blue-50 text-blue-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-400',
    loose: 'border-gray-200 bg-gray-50 text-gray-700 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-400',
    custom: 'border-purple-200 bg-purple-50 text-purple-700 dark:border-purple-800 dark:bg-purple-900/20 dark:text-purple-400'
  }

  return classMap[strategyKey] || classMap.custom
}

// Check if current config is a preset (not custom)
const isPresetStrategy = computed(() => {
  if (!customConfig.value) return false
  const strategyKey = detectStrategy(customConfig.value)
  return strategyKey !== 'custom'
})

const thresholdModeOptions = computed(() => [
  { value: 'count', label: t('admin.ops.availability.thresholdModes.count') },
  { value: 'percentage', label: t('admin.ops.availability.thresholdModes.percentage') },
  { value: 'both', label: t('admin.ops.availability.thresholdModes.both') }
])

const fetchData = async () => {
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
    totalPages.value = response.total_pages
    currentPage.value = response.page

  } catch (err) {
    console.error('Failed to fetch group data', err)
  } finally {
    loading.value = false
  }
}

const handleSearch = () => {
  currentPage.value = 1
  fetchData()
}

const handlePageChange = (page: number) => {
  currentPage.value = page
  fetchData()
}

const handleSizeChange = (size: number) => {
  pageSize.value = size
  currentPage.value = 1
  fetchData()
}

// 切换监控开关
const toggleMonitoring = async (group: GroupAvailabilityStatus, enabled: boolean) => {
  updatingId.value = group.group_id
  try {
    const payload = buildUpsertPayload(group.config, { enabled })
    await opsAPI.updateGroupAvailabilityConfig(group.group_id, payload)

    // Refresh to get full status (including correct accounts count)
    await fetchData()
    appStore.showSuccess(t('common.success'))
  } catch (e) {
    console.error('Failed to toggle monitoring', e)
    appStore.showError((e as any)?.response?.data?.detail || t('common.error'))
  } finally {
    updatingId.value = null
  }
}

// Open custom config dialog
const openCustomConfig = (group: GroupAvailabilityStatus) => {
  editingGroup.value = group
  const base = buildUpsertPayload(group.config, { enabled: true })
  customConfig.value = {
    group_id: group.group_id,
    ...base
  }
  showCustomConfig.value = true
}

// Apply strategy template
const applyStrategyTemplate = (strategyKey: string) => {
  if (!editingGroup.value || !customConfig.value) return

  const strategy = strategies.find(s => s.key === strategyKey)
  if (!strategy) return

  // Update customConfig with strategy values
  customConfig.value.threshold_mode = strategy.config.threshold_mode
  customConfig.value.min_available_accounts = strategy.config.min_available_accounts
  customConfig.value.min_available_percentage = strategy.config.min_available_percentage ?? 0
  customConfig.value.severity = strategy.config.severity
  customConfig.value.cooldown_minutes = strategy.config.cooldown_minutes
  customConfig.value.notify_email = strategy.config.notify_email
}

// Save custom config
const saveCustomConfig = async () => {
  if (!editingGroup.value || !customConfig.value) return

  updatingId.value = editingGroup.value.group_id
  try {
    const payload = buildUpsertPayload(editingGroup.value.config, {
      enabled: true,
      threshold_mode: customConfig.value.threshold_mode,
      min_available_accounts: customConfig.value.min_available_accounts,
      min_available_percentage: customConfig.value.min_available_percentage,
      severity: customConfig.value.severity,
      cooldown_minutes: customConfig.value.cooldown_minutes,
      notify_email: customConfig.value.notify_email
    })
    await opsAPI.updateGroupAvailabilityConfig(editingGroup.value.group_id, payload)
    await fetchData()
    showCustomConfig.value = false
    appStore.showSuccess(t('common.save'))
  } catch (e) {
    console.error('Failed to save custom config', e)
    appStore.showError((e as any)?.response?.data?.detail || t('common.error'))
  } finally {
    updatingId.value = null
  }
}

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <!-- Header Section -->
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

      <button
        @click="fetchData"
        :disabled="loading"
        class="inline-flex items-center gap-2 rounded-xl px-4 py-2 text-xs font-bold transition-all border shadow-sm bg-white border-gray-200 text-gray-700 hover:border-blue-300 hover:bg-blue-50 dark:bg-dark-800 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-blue-900/20"
      >
        <svg class="h-4 w-4" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        {{ t('common.refresh') }}
      </button>
    </div>

    <!-- Filters Bar -->
    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
      <!-- Search Input -->
      <div class="lg:col-span-1">
        <div class="relative group">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
            <svg class="h-4 w-4 text-gray-400 group-focus-within:text-blue-500 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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

      <!-- Monitoring Filter -->
      <Select
        v-model="monitoringFilter"
        :options="[
          { value: 'all', label: t('admin.ops.availability.filters.allMonitoring') },
          { value: 'enabled', label: t('admin.ops.availability.filters.monitoringEnabled') },
          { value: 'disabled', label: t('admin.ops.availability.filters.monitoringDisabled') }
        ]"
        @change="handleSearch"
      />

      <!-- Alert Filter -->
      <Select
        v-model="alertFilter"
        :options="[
          { value: 'all', label: t('admin.ops.availability.filters.allAlerts') },
          { value: 'ok', label: t('admin.ops.availability.filters.alertOk') },
          { value: 'firing', label: t('admin.ops.availability.filters.alertFiring') }
        ]"
        @change="handleSearch"
      />

      <!-- Search Button -->
      <button
        @click="handleSearch"
        :disabled="loading"
        class="rounded-2xl bg-blue-500 px-4 py-2.5 text-sm font-bold text-white transition-all hover:bg-blue-600 focus:ring-4 focus:ring-blue-500/20 disabled:opacity-50 dark:bg-blue-600 dark:hover:bg-blue-700"
      >
        {{ t('common.search') }}
      </button>
    </div>

    <!-- Table Area -->
    <div class="relative overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900 shadow-sm">
      <!-- Loading Overlay -->
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
              <!-- Group Name & Status -->
              <td class="px-6 py-4">
                <div class="flex items-center gap-3">
                  <span
                    class="h-2.5 w-2.5 flex-shrink-0 rounded-full"
                    :class="[
                      !group.monitoring_enabled ? 'bg-gray-300' :
                      group.is_healthy ? 'bg-green-500' : 'animate-pulse bg-red-500'
                    ]"
                  ></span>
                  <span class="text-sm font-bold text-gray-900 dark:text-white">
                    {{ group.group_name }}
                  </span>
                </div>
              </td>

              <!-- Water Level -->
              <td class="px-6 py-4">
                <div class="flex flex-col items-center gap-2">
                  <div class="text-sm font-black text-gray-700 dark:text-gray-300">
                    {{ group.available_accounts }} <span class="text-gray-400">/ {{ group.total_accounts }}</span>
                  </div>
                  <!-- Progress Bar -->
                  <div class="h-2 w-24 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600">
                    <div
                      class="h-full rounded-full transition-all"
                      :class="group.is_healthy ? 'bg-green-500' : 'bg-red-500'"
                      :style="{ width: `${(group.available_accounts / Math.max(group.total_accounts, 1)) * 100}%` }"
                    ></div>
                  </div>
                </div>
              </td>

              <!-- Monitoring Switch -->
              <td class="px-6 py-4">
                <div class="flex justify-center">
                  <button
                    @click="toggleMonitoring(group, !group.monitoring_enabled)"
                    :disabled="updatingId === group.group_id"
                    class="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
                    :class="[group.monitoring_enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-dark-600']"
                  >
                    <span
                      class="pointer-events-none inline-block h-5 w-5 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
                      :class="[group.monitoring_enabled ? 'translate-x-5' : 'translate-x-0']"
                    ></span>
                  </button>
                </div>
              </td>

              <!-- Strategy Badge -->
              <td class="px-6 py-4">
                <div class="flex items-center justify-end">
                  <button
                    @click="openCustomConfig(group)"
                    :disabled="updatingId === group.group_id"
                    class="group/badge inline-flex items-center gap-2 rounded-xl border px-3 py-2 text-xs font-bold transition-all hover:border-blue-400 hover:bg-blue-50 disabled:opacity-50 dark:hover:bg-blue-900/20"
                    :class="getStrategyBadgeClass(group.config)"
                  >
                    <span>{{ getStrategyLabel(group.config) }}</span>
                    <svg class="h-3.5 w-3.5 opacity-60 transition-transform group-hover/badge:translate-x-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                    </svg>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Pagination -->
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

  <!-- Custom Config Dialog -->
  <BaseDialog :show="showCustomConfig" :title="t('admin.ops.availability.customConfigTitle')" width="normal" @close="showCustomConfig = false">
    <div v-if="customConfig" class="space-y-6">
      <!-- Group Info -->
      <div class="rounded-xl border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800">
        <div class="flex items-center gap-3">
          <div class="flex h-10 w-10 items-center justify-center rounded-xl bg-white dark:bg-dark-900">
            <svg class="h-5 w-5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
            </svg>
          </div>
          <div>
            <div class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.availability.groupName') }}</div>
            <div class="text-sm font-bold text-gray-900 dark:text-white">{{ editingGroup?.group_name }}</div>
          </div>
        </div>
      </div>

      <!-- Strategy Templates -->
      <div>
        <label class="mb-3 block text-sm font-bold text-gray-700 dark:text-gray-300">{{ t('admin.ops.availability.strategyTemplates') }}</label>
        <div class="grid grid-cols-2 gap-3">
          <button
            v-for="strategy in strategies"
            :key="strategy.key"
            @click="applyStrategyTemplate(strategy.key)"
            type="button"
            class="group relative flex flex-col items-start gap-2 rounded-xl border-2 p-4 text-left transition-all hover:border-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/10"
            :class="detectStrategy(customConfig) === strategy.key
              ? 'border-blue-500 bg-blue-50 dark:border-blue-600 dark:bg-blue-900/20'
              : 'border-gray-200 dark:border-dark-700'"
          >
            <span class="text-sm font-bold text-gray-900 dark:text-white">{{ t(strategy.labelKey) }}</span>
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t(strategy.descKey) }}</p>
            <div v-if="detectStrategy(customConfig) === strategy.key" class="absolute right-3 top-3">
              <svg class="h-5 w-5 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
              </svg>
            </div>
          </button>
        </div>
      </div>

      <!-- Divider -->
      <div class="relative">
        <div class="absolute inset-0 flex items-center">
          <div class="w-full border-t border-gray-200 dark:border-dark-700"></div>
        </div>
        <div class="relative flex justify-center">
          <span class="bg-white px-3 text-xs font-bold uppercase tracking-wider text-gray-500 dark:bg-dark-900 dark:text-gray-400">
            {{ isPresetStrategy ? t('admin.ops.availability.presetSettings') : t('admin.ops.availability.advancedSettings') }}
          </span>
        </div>
      </div>

      <!-- Advanced Settings -->
      <div class="space-y-4">
        <div v-if="isPresetStrategy" class="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-800 dark:bg-blue-900/20">
          <div class="flex items-start gap-3">
            <svg class="h-5 w-5 flex-shrink-0 text-blue-500 mt-0.5" fill="currentColor" viewBox="0 0 20 20">
              <path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" />
            </svg>
            <div class="flex-1">
              <p class="text-xs font-bold text-blue-700 dark:text-blue-400">{{ t('admin.ops.availability.presetStrategyInfo') }}</p>
              <p class="mt-1 text-xs text-blue-600 dark:text-blue-500">{{ t('admin.ops.availability.presetStrategyHint') }}</p>
            </div>
          </div>
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('admin.ops.availability.thresholdMode') }}</label>
          <Select
            v-model="customConfig.threshold_mode"
            :options="thresholdModeOptions"
            :disabled="isPresetStrategy"
          />
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('admin.ops.config.minAvailableAccounts') }}</label>
          <input
            v-model.number="customConfig.min_available_accounts"
            type="number"
            min="0"
            class="input"
            :disabled="isPresetStrategy"
          />
        </div>

        <div v-if="customConfig.threshold_mode === 'percentage' || customConfig.threshold_mode === 'both'">
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('admin.ops.availability.minAvailablePercentage') }}</label>
          <input
            v-model.number="customConfig.min_available_percentage"
            type="number"
            min="0"
            max="100"
            step="0.1"
            class="input"
            :disabled="isPresetStrategy"
          />
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('admin.ops.availability.severity') }}</label>
          <Select
            v-model="customConfig.severity"
            :options="customSeverityOptions"
            :disabled="isPresetStrategy"
          />
        </div>

        <div>
          <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">{{ t('admin.ops.availability.cooldownMinutes') }}</label>
          <input
            v-model.number="customConfig.cooldown_minutes"
            type="number"
            min="0"
            class="input"
            :disabled="isPresetStrategy"
          />
        </div>

        <div>
          <label class="flex items-center gap-2">
            <input
              v-model="customConfig.notify_email"
              type="checkbox"
              class="h-4 w-4 rounded border-gray-300"
              :disabled="isPresetStrategy"
            />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.ops.availability.notifyEmail') }}</span>
          </label>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button @click="showCustomConfig = false" class="btn btn-secondary">{{ t('common.cancel') }}</button>
        <button @click="saveCustomConfig" class="btn btn-primary">{{ t('common.save') }}</button>
      </div>
    </template>
  </BaseDialog>
</template>
