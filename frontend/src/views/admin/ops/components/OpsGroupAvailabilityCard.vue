<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { opsAPI } from '@/api/admin/ops'
import { adminAPI } from '@/api/admin' // Import adminAPI
import type { GroupAvailabilityStatus, GroupAvailabilityConfig, AlertSeverity } from '../types'
import HelpTooltip from '@/components/common/HelpTooltip.vue'

const { t } = useI18n()
const loading = ref(false)
const groups = ref<GroupAvailabilityStatus[]>([])
const updatingId = ref<number | null>(null)

// --- 策略模板定义 (Strategy Templates) ---
interface StrategyTemplate {
  key: string
  label: string
  desc: string
  config: {
    min_available_accounts: number
    severity: AlertSeverity
    cooldown_minutes: number
    notify_email: boolean
  }
}

const strategies: StrategyTemplate[] = [
  {
    key: 'strict',
    label: '🛡️ 核心保障 (Strict)',
    desc: '敏感度高，只要有账号挂了就报警',
    config: { min_available_accounts: 5, severity: 'critical', cooldown_minutes: 15, notify_email: true }
  },
  {
    key: 'standard',
    label: '⚖️ 标准均衡 (Standard)',
    desc: '平衡策略，可用率过低时报警',
    config: { min_available_accounts: 3, severity: 'warning', cooldown_minutes: 30, notify_email: true }
  },
  {
    key: 'loose',
    label: '💤 宽松模式 (Loose)',
    desc: '仅记录日志，不发送邮件轰炸',
    config: { min_available_accounts: 1, severity: 'info', cooldown_minutes: 60, notify_email: false }
  }
]

// 推断当前分组使用的是哪个策略
function detectStrategy(config: GroupAvailabilityConfig | undefined): string {
  if (!config) return 'standard'
  if (config.severity === 'critical') return 'strict'
  if (config.severity === 'info') return 'loose'
  return 'standard'
}

const fetchData = async () => {
  loading.value = true
  try {
    // 1. Fetch monitoring status
    const statusList = await opsAPI.listGroupAvailabilityStatus()
    
    // 2. Fetch all groups (pagination 1-100 for now, assumes reasonable count)
    const allGroupsRes = await adminAPI.groups.list(1, 100)
    
    // 3. Merge: Ensure all groups are listed
    const merged: GroupAvailabilityStatus[] = allGroupsRes.items.map(g => {
      const status = statusList.find(s => s.group_id === g.id)
      if (status) return status
      
      // Default placeholder for unmonitored groups
      return {
        group_id: g.id,
        group_name: g.name,
        platform: g.platform,
        total_accounts: 0, // Will be updated if backend supports detailed counts in group list
        available_accounts: 0,
        min_available_accounts: 0,
        is_healthy: true, // Default to true visually until monitored
        monitoring_enabled: false,
        config: undefined
      }
    })
    
    // 4. Sort: Monitored first, then by ID
    groups.value = merged.sort((a, b) => {
      if (a.monitoring_enabled === b.monitoring_enabled) return a.group_id - b.group_id
      return a.monitoring_enabled ? -1 : 1
    })

  } catch (err) {
    console.error('Failed to fetch group data', err)
  } finally {
    loading.value = false
  }
}

// 切换监控开关
const toggleMonitoring = async (group: GroupAvailabilityStatus, enabled: boolean) => {
  updatingId.value = group.group_id
  try {
    const currentConfig = group.config || {}
    // If enabling for the first time, use standard strategy defaults
    const defaults = enabled && !group.config ? strategies.find(s => s.key === 'standard')?.config : {}
    
    await opsAPI.updateGroupAvailabilityConfig(group.group_id, {
      ...currentConfig,
      ...defaults,
      group_id: group.group_id,
      enabled: enabled
    })
    
    // Refresh to get full status (including correct accounts count)
    await fetchData()
  } catch (e) {
    console.error('Failed to toggle monitoring', e)
  } finally {
    updatingId.value = null
  }
}

// 切换策略模板
const changeStrategy = async (group: GroupAvailabilityStatus, strategyKey: string) => {
  const strategy = strategies.find(s => s.key === strategyKey)
  if (!strategy) return

  updatingId.value = group.group_id
  try {
    const currentConfig = group.config || {}
    await opsAPI.updateGroupAvailabilityConfig(group.group_id, {
      ...currentConfig,
      group_id: group.group_id,
      enabled: true,
      ...strategy.config
    })
    await fetchData()
  } catch (e) {
    console.error('Failed to change strategy', e)
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
    <!-- Header -->
    <div class="mb-4 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">
          {{ t('admin.ops.availability.title') }}
        </h3>
        <HelpTooltip content="监控各分组的账号存活情况。如果是 VIP 专用分组，建议开启「严格模式」。" />
      </div>
      
      <button
        @click="fetchData"
        :disabled="loading"
        class="text-xs text-blue-600 hover:text-blue-700 dark:text-blue-400"
      >
        {{ t('common.refresh') }}
      </button>
    </div>

    <!-- Content -->
    <div v-if="loading && groups.length === 0" class="flex h-32 items-center justify-center text-sm text-gray-400">
      <div class="animate-pulse">{{ t('admin.ops.availability.loading') }}</div>
    </div>

    <div v-else class="space-y-1">
      <!-- List Header -->
      <div class="grid grid-cols-12 gap-4 border-b border-gray-100 pb-2 text-xs font-medium text-gray-500 dark:border-dark-700 dark:text-gray-400 px-2">
        <div class="col-span-3">分组名称</div>
        <div class="col-span-3 text-center">水位 (可用/总数)</div>
        <div class="col-span-3 text-center">监控开关</div>
        <div class="col-span-3 text-right">报警策略</div>
      </div>

      <!-- Group Items -->
      <div 
        v-for="group in groups" 
        :key="group.group_id"
        class="grid grid-cols-12 items-center gap-4 rounded-lg px-2 py-3 transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/50"
      >
        <!-- 1. Name & Status Dot -->
        <div class="col-span-3 flex items-center gap-2 overflow-hidden">
          <span 
            class="h-2 w-2 flex-shrink-0 rounded-full"
            :class="[
              !group.monitoring_enabled ? 'bg-gray-300' :
              group.is_healthy ? 'bg-green-500' : 'animate-pulse bg-red-500'
            ]"
          ></span>
          <span class="truncate text-sm font-medium text-gray-900 dark:text-white" :title="group.group_name">
            {{ group.group_name }}
          </span>
        </div>

        <!-- 2. Water Level -->
        <div class="col-span-3 flex flex-col items-center justify-center">
          <div class="text-xs font-medium text-gray-700 dark:text-gray-300">
            {{ group.available_accounts }} <span class="text-gray-400">/ {{ group.total_accounts }}</span>
          </div>
          <!-- Mini Progress Bar -->
          <div class="mt-1 h-1 w-16 overflow-hidden rounded-full bg-gray-100 dark:bg-dark-600">
            <div 
              class="h-full rounded-full transition-all"
              :class="group.is_healthy ? 'bg-green-500' : 'bg-red-500'"
              :style="{ width: `${(group.available_accounts / Math.max(group.total_accounts, 1)) * 100}%` }"
            ></div>
          </div>
        </div>

        <!-- 3. Monitoring Switch -->
        <div class="col-span-3 flex justify-center">
          <button
            @click="toggleMonitoring(group, !group.monitoring_enabled)"
            :disabled="updatingId === group.group_id"
            class="relative inline-flex h-5 w-9 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
            :class="[group.monitoring_enabled ? 'bg-blue-600' : 'bg-gray-200 dark:bg-dark-600']"
          >
            <span
              class="pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow ring-0 transition duration-200 ease-in-out"
              :class="[group.monitoring_enabled ? 'translate-x-4' : 'translate-x-0']"
            ></span>
          </button>
        </div>

        <!-- 4. Strategy Selector -->
        <div class="col-span-3 flex justify-end">
          <select
            :disabled="!group.monitoring_enabled || updatingId === group.group_id"
            :value="detectStrategy(group.config)"
            @change="(e) => changeStrategy(group, (e.target as HTMLSelectElement).value)"
            class="block w-full max-w-[140px] rounded-md border-0 bg-transparent py-1 pl-2 pr-7 text-xs font-medium text-gray-600 ring-1 ring-inset ring-gray-300 focus:ring-2 focus:ring-blue-600 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-300 dark:ring-dark-600 sm:text-xs sm:leading-6"
          >
            <option v-for="s in strategies" :key="s.key" :value="s.key">
              {{ s.label }}
            </option>
          </select>
        </div>
      </div>
      
      <!-- Empty State -->
      <div v-if="groups.length === 0" class="py-4 text-center text-xs text-gray-400">
        {{ t('admin.ops.availability.noData') }}
      </div>
    </div>
  </div>
</template>
