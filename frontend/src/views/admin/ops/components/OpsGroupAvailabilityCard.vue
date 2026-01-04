<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { opsAPI } from '@/api/admin/ops'
import type { GroupAvailabilityStatus } from '../types'
import OpsConfigDialog from './OpsConfigDialog.vue'
import { formatDistanceToNow } from 'date-fns'
import { zhCN } from 'date-fns/locale'

const { t, locale } = useI18n()
const router = useRouter()
const loading = ref(false)
const groups = ref<GroupAvailabilityStatus[]>([])
const showConfigDialog = ref(false)
const focusGroupId = ref<number | null>(null)

const isMonitoringEnabled = (group: GroupAvailabilityStatus) => {
  return group.monitoring_enabled ?? group.config?.enabled ?? false
}

const fetchData = async () => {
  loading.value = true
  try {
    groups.value = await opsAPI.listGroupAvailabilityStatus()
  } catch (err) {
    console.error('Failed to fetch group availability status', err)
  } finally {
    loading.value = false
  }
}

const getStatusType = (group: GroupAvailabilityStatus) => {
  if (!isMonitoringEnabled(group)) return 'info'
  return group.is_healthy ? 'success' : 'danger'
}

const getStatusText = (group: GroupAvailabilityStatus) => {
  if (!isMonitoringEnabled(group)) return t('admin.ops.availability.unmonitored')
  return group.is_healthy ? t('admin.ops.availability.healthy') : t('admin.ops.availability.alert')
}

const getProgressColor = (group: GroupAvailabilityStatus) => {
  if (!isMonitoringEnabled(group)) return '#9ca3af'
  return group.is_healthy ? '#10b981' : '#ef4444'
}

const availabilityPercentage = (group: GroupAvailabilityStatus) => {
  if (group.total_accounts === 0) return 0
  return Math.round((group.available_accounts / group.total_accounts) * 100)
}

const formatLastAlert = (group: GroupAvailabilityStatus) => {
  const timestamp = group.last_alert_at || group.config?.updated_at
  if (!timestamp) return '-'
  try {
    return formatDistanceToNow(new Date(timestamp), {
      addSuffix: true,
      locale: locale.value === 'zh' ? zhCN : undefined
    })
  } catch {
    return '-'
  }
}

function openGroupAvailabilityConfig(groupId?: number) {
  focusGroupId.value = groupId ?? null
  showConfigDialog.value = true
}

function closeGroupAvailabilityConfig() {
  showConfigDialog.value = false
  focusGroupId.value = null
  fetchData()
}

const hasData = computed(() => groups.value.length > 0)

onMounted(() => {
  fetchData()
})
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-center justify-between">
      <h3 class="flex items-center gap-2 text-sm font-bold text-gray-900 dark:text-white">
        <svg class="h-4 w-4 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
        </svg>
        {{ t('admin.ops.availability.title') }}
      </h3>
      <button
        @click="fetchData"
        :disabled="loading"
        class="flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
      >
        <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
        {{ t('admin.ops.availability.refresh') }}
      </button>
    </div>

    <div v-if="loading && !hasData" class="flex h-48 items-center justify-center text-sm text-gray-400">
      <div class="animate-pulse">{{ t('admin.ops.availability.loading') }}</div>
    </div>

    <div v-else-if="!hasData" class="flex h-48 flex-col items-center justify-center gap-3 text-sm text-gray-400">
      <svg class="h-12 w-12 text-gray-300 dark:text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
      </svg>
      <p class="font-medium">{{ t('admin.ops.availability.noData') }}</p>
      <button
        @click="router.push('/admin/groups')"
        class="mt-2 rounded-lg bg-blue-500 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-blue-600 dark:bg-blue-600 dark:hover:bg-blue-700"
      >
        前往分组管理
      </button>
    </div>

    <el-table v-else :data="groups" stripe class="w-full">
      <el-table-column :label="t('admin.ops.availability.groupName')" min-width="150">
        <template #default="{ row }">
          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-900 dark:text-white">{{ row.group_name }}</span>
            <el-tag size="small" type="info">{{ row.platform }}</el-tag>
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('admin.ops.availability.availableAccounts')" min-width="200">
        <template #default="{ row }">
          <div class="space-y-1">
            <div class="flex items-center justify-between text-xs">
              <span class="text-gray-600 dark:text-gray-400">
                {{ row.available_accounts }} / {{ row.total_accounts }}
              </span>
              <span class="font-medium text-gray-900 dark:text-white">
                {{ availabilityPercentage(row) }}%
              </span>
            </div>
            <el-progress
              :percentage="availabilityPercentage(row)"
              :color="getProgressColor(row)"
              :show-text="false"
              :stroke-width="6"
            />
          </div>
        </template>
      </el-table-column>

      <el-table-column :label="t('admin.ops.availability.status')" width="100">
        <template #default="{ row }">
          <el-tag :type="getStatusType(row)" size="small">
            {{ getStatusText(row) }}
          </el-tag>
        </template>
      </el-table-column>

      <el-table-column :label="t('admin.ops.availability.lastUpdate')" width="120">
        <template #default="{ row }">
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ formatLastAlert(row) }}
          </span>
        </template>
      </el-table-column>

      <el-table-column :label="t('admin.ops.availability.actions')" width="100" align="center">
        <template #default="{ row }">
          <button
            @click="openGroupAvailabilityConfig(row.group_id)"
            class="text-xs font-medium text-blue-600 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
          >
            {{ t('admin.ops.availability.config') }}
          </button>
        </template>
      </el-table-column>
    </el-table>

    <OpsConfigDialog
      :show="showConfigDialog"
      :focus-group-id="focusGroupId"
      @close="closeGroupAvailabilityConfig"
    />
  </div>
</template>
