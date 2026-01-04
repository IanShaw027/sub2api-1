<script setup lang="ts">
import { getSeverityClass, truncateMessage, formatDateTime } from '../utils/opsFormatters'
import type { ErrorFilters } from '../types'
import type { OpsErrorLog, OpsPlatform, OpsSeverity } from '@/api/admin/ops'
import ElPagination from '@/components/common/Pagination.vue'

interface Props {
  errorLogs: OpsErrorLog[]
  errorLogsTotal: number
  errorLogsLoading: boolean
  filters: ErrorFilters
  page?: number
  pageSize?: number
}

interface Emits {
  (e: 'update:filters', value: ErrorFilters): void
  (e: 'update:page', value: number): void
  (e: 'update:pageSize', value: number): void
  (e: 'openErrorDetail', log: OpsErrorLog): void
}

const props = withDefaults(defineProps<Props>(), {
  page: 1,
  pageSize: 50
})
const emit = defineEmits<Emits>()

const platformOptions: OpsPlatform[] = ['openai', 'anthropic', 'gemini', 'antigravity']
const statusCodeOptions = [400, 401, 403, 404, 429, 500, 502, 503, 504]
const severityOptions: OpsSeverity[] = ['P0', 'P1', 'P2', 'P3']

function updateFilter(key: keyof ErrorFilters, value: any) {
  emit('update:filters', { ...props.filters, [key]: value })
}

function handlePageChange(page: number) {
  emit('update:page', page)
}

function handleSizeChange(pageSize: number) {
  emit('update:pageSize', pageSize)
}
</script>

<template>
  <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-6 flex items-center justify-between">
      <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">错误日志查询</h3>
      <span class="text-xs font-medium text-gray-500">共 {{ errorLogsTotal }} 条记录</span>
    </div>

    <!-- Filters Bar -->
    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
      <!-- Platform Multi-Select -->
      <div>
        <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">平台</label>
        <select
          :value="filters.platforms"
          @change="updateFilter('platforms', Array.from(($event.target as HTMLSelectElement).selectedOptions, opt => opt.value))"
          multiple
          class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
          style="height: 38px; overflow: auto;"
        >
          <option v-for="platform in platformOptions" :key="platform" :value="platform">
            {{ platform }}
          </option>
        </select>
      </div>

      <!-- Status Code Multi-Select -->
      <div>
        <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">错误码</label>
        <select
          :value="filters.statusCodes"
          @change="updateFilter('statusCodes', Array.from(($event.target as HTMLSelectElement).selectedOptions, opt => Number(opt.value)))"
          multiple
          class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
          style="height: 38px; overflow: auto;"
        >
          <option v-for="code in statusCodeOptions" :key="code" :value="code">
            {{ code }}
          </option>
        </select>
      </div>

      <!-- Severity Select -->
      <div>
        <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">严重级别</label>
        <select
          :value="filters.severity"
          @change="updateFilter('severity', ($event.target as HTMLSelectElement).value)"
          class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
        >
          <option value="">全部</option>
          <option v-for="sev in severityOptions" :key="sev" :value="sev">
            {{ sev }}
          </option>
        </select>
      </div>

      <!-- IP Address Input -->
      <div>
        <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">IP地址</label>
        <input
          :value="filters.clientIp"
          @input="updateFilter('clientIp', ($event.target as HTMLInputElement).value)"
          type="text"
          placeholder="搜索IP"
          class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 placeholder-gray-400 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
        />
      </div>

      <!-- Search Input -->
      <div>
        <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">搜索</label>
        <input
          :value="filters.searchText"
          @input="updateFilter('searchText', ($event.target as HTMLInputElement).value)"
          type="text"
          placeholder="request_id / 错误信息"
          class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 placeholder-gray-400 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
        />
      </div>
    </div>

    <!-- Error Logs Table -->
    <div class="overflow-x-auto">
      <div v-if="errorLogsLoading" class="flex items-center justify-center py-12">
        <div class="text-sm font-medium text-gray-400">加载中...</div>
      </div>
      <div v-else-if="errorLogs.length === 0" class="flex items-center justify-center py-12">
        <div class="text-sm font-medium text-gray-400">当前筛选条件下无错误记录</div>
      </div>
      <table v-else class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
        <thead class="bg-gray-50 dark:bg-dark-900">
          <tr>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">时间</th>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">Request ID</th>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">平台</th>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">错误码</th>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">严重级别</th>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">延迟(ms)</th>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">错误信息</th>
            <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">客户端IP</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
          <tr
            v-for="log in errorLogs"
            :key="log.id"
            class="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/50"
            @click="emit('openErrorDetail', log)"
          >
            <td class="whitespace-nowrap px-3 py-3 text-xs text-gray-900 dark:text-gray-300">
              {{ formatDateTime(log.created_at) }}
            </td>
            <td class="px-3 py-3 text-xs font-mono text-gray-700 dark:text-gray-400">
              <div class="max-w-[120px] truncate" :title="log.request_id">{{ log.request_id }}</div>
            </td>
            <td class="whitespace-nowrap px-3 py-3">
              <span class="rounded-full bg-blue-100 px-2 py-1 text-xs font-bold text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
                {{ log.platform }}
              </span>
            </td>
            <td class="whitespace-nowrap px-3 py-3">
              <span class="rounded-full bg-gray-100 px-2 py-1 text-xs font-bold text-gray-800 dark:bg-gray-900/30 dark:text-gray-400">
                {{ log.status_code }}
              </span>
            </td>
            <td class="whitespace-nowrap px-3 py-3">
              <span class="rounded-full px-2 py-1 text-xs font-bold" :class="getSeverityClass(log.severity)">
                {{ log.severity }}
              </span>
            </td>
            <td class="whitespace-nowrap px-3 py-3 text-xs text-gray-700 dark:text-gray-400">
              {{ log.latency_ms !== null ? log.latency_ms.toFixed(0) : '--' }}
            </td>
            <td class="px-3 py-3 text-xs text-gray-700 dark:text-gray-400">
              <div class="max-w-[300px]" :title="log.message">{{ truncateMessage(log.message) }}</div>
            </td>
            <td class="whitespace-nowrap px-3 py-3 text-xs font-mono text-gray-600 dark:text-gray-500">
              {{ log.client_ip || '--' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <el-pagination
      v-if="!errorLogsLoading && errorLogsTotal > 0"
      class="mt-4"
      :total="errorLogsTotal"
      :page="page"
      :page-size="pageSize"
      @update:page="handlePageChange"
      @update:pageSize="handleSizeChange"
    />
  </div>
</template>

<style scoped>
/* Custom select styling */
select {
  appearance: none;
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 0.5rem center;
  background-size: 1.5em 1.5em;
}
</style>
