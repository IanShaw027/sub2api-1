<template>
  <div
    class="flex h-full min-h-0 flex-col"
    :class="flat ? '' : 'card bg-white dark:bg-dark-900'"
  >
    <IpGeoBatchToolbar
      v-if="isColumnVisible('client_ip')"
      :ips="rows.map((row) => row.client_ip)"
      @failed="emit('ipGeoBatchFailed')"
    />
    <!-- Loading State -->
    <div v-if="loading" class="flex flex-1 items-center justify-center py-10">
      <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-primary-600"></div>
    </div>

    <!-- Table Container -->
    <div v-else class="flex min-h-0 flex-1 flex-col">
      <div class="min-h-0 flex-1 overflow-auto border-b border-gray-200 dark:border-dark-700">
        <table class="w-full border-separate border-spacing-0">
          <thead class="sticky top-0 z-10 bg-gray-50 dark:bg-dark-800">
            <tr>
              <th
                v-for="column in columns"
                :key="column.key"
                class="border-b border-gray-200 px-4 py-2.5 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:border-dark-700 dark:text-dark-400"
                :class="{
                  'cursor-pointer select-none hover:bg-gray-100 dark:hover:bg-dark-700': column.sortable,
                  'text-right': column.key === 'actions'
                }"
                :aria-sort="column.sortable ? columnAriaSort(column.key) : undefined"
                @click="column.sortable && onSort(column.key)"
              >
                <span class="inline-flex items-center gap-1">
                  {{ column.label }}
                  <span v-if="column.sortable && sortKey === column.key" aria-hidden="true">
                    {{ sortOrder === 'asc' ? '▲' : '▼' }}
                  </span>
                </span>
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-if="rows.length === 0">
              <td :colspan="columns.length" class="py-12 text-center text-sm text-gray-400 dark:text-dark-500">
                {{ t('admin.ops.errorLog.noErrors') }}
              </td>
            </tr>

            <tr
              v-for="log in rows"
              :key="log.id"
              class="group cursor-pointer transition-colors hover:bg-gray-50/80 dark:hover:bg-dark-800/50"
              @click="emit('openErrorDetail', log.id)"
            >
              <!-- Time -->
              <td v-if="isColumnVisible('created_at')" class="whitespace-nowrap px-4 py-2">
                <el-tooltip :content="log.request_id || log.client_request_id" placement="top" :show-after="500">
                  <span class="font-mono text-xs font-medium text-gray-900 dark:text-gray-200">
                    {{ formatDateTime(log.created_at).split(' ')[1] }}
                  </span>
                </el-tooltip>
              </td>

              <!-- Type -->
              <td v-if="isColumnVisible('type')" class="whitespace-nowrap px-4 py-2">
                <span
                  :class="[
                    'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold ring-1 ring-inset',
                    getTypeBadge(log).className
                  ]"
                >
                  {{ getTypeBadge(log).label }}
                </span>
              </td>

              <!-- Category -->
              <td v-if="isColumnVisible('category')" class="whitespace-nowrap px-4 py-2 text-xs text-gray-700 dark:text-gray-300">
                {{ t(`usage.errors.categories.${mapErrorCategory(log.phase, log.type)}`) }}
              </td>

              <!-- Endpoint -->
              <td v-if="isColumnVisible('endpoint')" class="px-4 py-2">
                <div class="max-w-[160px]">
                  <el-tooltip v-if="log.inbound_endpoint" :content="formatEndpointTooltip(log)" placement="top" :show-after="500">
                    <span class="truncate font-mono text-[11px] text-gray-700 dark:text-gray-300">
                      {{ log.inbound_endpoint }}
                    </span>
                  </el-tooltip>
                  <span v-else class="text-xs text-gray-400">-</span>
                </div>
              </td>

              <!-- Platform -->
              <td v-if="isColumnVisible('platform')" class="whitespace-nowrap px-4 py-2">
                <span class="inline-flex items-center rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-bold uppercase text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                  {{ log.platform || '-' }}
                </span>
              </td>

              <!-- Model -->
              <td v-if="isColumnVisible('model')" class="px-4 py-2">
                <div class="max-w-[160px]">
                  <template v-if="hasModelMapping(log)">
                    <el-tooltip :content="modelMappingTooltip(log)" placement="top" :show-after="500">
                      <span class="flex items-center gap-1 truncate font-mono text-[11px] text-gray-700 dark:text-gray-300">
                        <span class="truncate">{{ log.requested_model }}</span>
                        <span class="flex-shrink-0 text-gray-400">→</span>
                        <span class="truncate text-primary-600 dark:text-primary-400">{{ log.upstream_model }}</span>
                      </span>
                    </el-tooltip>
                  </template>
                  <template v-else>
                    <span v-if="displayModel(log)" class="truncate font-mono text-[11px] text-gray-700 dark:text-gray-300" :title="displayModel(log)">
                      {{ displayModel(log) }}
                    </span>
                    <span v-else class="text-xs text-gray-400">-</span>
                  </template>
                </div>
              </td>

              <!-- Group -->
              <td v-if="isColumnVisible('group')" class="px-4 py-2">
                 <el-tooltip v-if="log.group_id" :content="t('admin.ops.errorLog.id') + ' ' + log.group_id" placement="top" :show-after="500">
                  <span class="max-w-[100px] truncate text-xs font-medium text-gray-900 dark:text-gray-200">
                    {{ log.group_name || '-' }}
                  </span>
                </el-tooltip>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>

              <!-- User -->
              <td v-if="isColumnVisible('user')" class="px-4 py-2">
                <el-tooltip v-if="userTooltip(log)" :content="userTooltip(log)" placement="top" :show-after="500">
                  <button
                    v-if="userClickable && effectiveUserID(log) && effectiveUserEmail(log)"
                    type="button"
                    class="max-w-[140px] truncate text-xs font-medium text-primary-600 underline decoration-dashed underline-offset-2 hover:text-primary-700 dark:text-primary-400 dark:hover:text-primary-300"
                    @click.stop="emit('userClick', effectiveUserID(log), effectiveUserEmail(log))"
                  >
                    {{ userLabel(log) }}
                  </button>
                  <span v-else class="max-w-[140px] truncate text-xs font-medium text-gray-900 dark:text-gray-200">
                    {{ userLabel(log) }}
                  </span>
                </el-tooltip>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>

              <!-- API Key -->
              <td v-if="isColumnVisible('api_key')" class="px-4 py-2">
                <el-tooltip v-if="apiKeyTooltip(log)" :content="apiKeyTooltip(log)" placement="top" :show-after="500">
                  <span class="inline-flex max-w-[120px] items-center gap-1 truncate text-xs font-medium text-gray-900 dark:text-gray-200">
                    <span class="truncate">{{ apiKeyLabel(log) }}</span>
                    <span
                      v-if="log.api_key_deleted"
                      class="rounded bg-amber-100 px-1 py-px text-[10px] font-bold text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                    >
                      {{ t('admin.ops.errorLog.keyDeletedBadge') }}
                    </span>
                  </span>
                </el-tooltip>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>

              <!-- Account -->
              <td v-if="isColumnVisible('account')" class="px-4 py-2">
                <el-tooltip v-if="accountTooltip(log)" :content="accountTooltip(log)" placement="top" :show-after="500">
                  <span class="max-w-[120px] truncate text-xs font-medium text-gray-900 dark:text-gray-200">
                    {{ accountLabel(log) }}
                  </span>
                </el-tooltip>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>

              <!-- Status -->
              <td v-if="isColumnVisible('status')" class="whitespace-nowrap px-4 py-2">
                <div class="flex items-center gap-1.5">
                  <span
                    :class="[
                      'inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold ring-1 ring-inset',
                      getStatusClass(log.status_code)
                    ]"
                  >
                    {{ log.status_code }}
                  </span>
                  <span
                    v-if="log.severity"
                    :class="['rounded px-1.5 py-0.5 text-[10px] font-bold', getSeverityClass(log.severity)]"
                  >
                    {{ log.severity }}
                  </span>
                  <span
                    v-if="log.request_type != null && log.request_type > 0"
                    class="rounded bg-gray-100 px-1.5 py-0.5 text-[10px] font-bold text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                  >
                    {{ formatRequestType(log.request_type) }}
                  </span>
                </div>
              </td>

              <!-- Message (Response Content) -->
              <td v-if="isColumnVisible('message')" class="px-4 py-2">
                <div class="max-w-[200px]">
                  <p class="truncate text-[11px] font-medium text-gray-600 dark:text-gray-400" :title="log.message">
                    {{ formatSmartMessage(log.message) || '-' }}
                  </p>
                </div>
              </td>

              <!-- User Agent -->
              <td v-if="isColumnVisible('user_agent')" class="px-4 py-2">
                <span class="block max-w-[240px] truncate text-[11px] text-gray-600 dark:text-gray-400" :title="log.user_agent">
                  {{ log.user_agent || '-' }}
                </span>
              </td>

              <!-- Client IP -->
              <td v-if="isColumnVisible('client_ip')" class="whitespace-nowrap px-4 py-2" @click.stop>
                <template v-if="log.client_ip">
                  <span class="font-mono text-[11px] text-gray-600 dark:text-gray-400">{{ log.client_ip }}</span>
                  <IpGeoCell :ip="log.client_ip" />
                </template>
                <span v-else class="text-xs text-gray-400">-</span>
              </td>

              <!-- Actions -->
              <td v-if="isColumnVisible('actions')" class="whitespace-nowrap px-4 py-2 text-right" @click.stop>
                <div class="flex items-center justify-end gap-3">
                  <button type="button" class="text-primary-600 hover:text-primary-700 dark:text-primary-400 text-xs font-bold" @click="emit('openErrorDetail', log.id)">
                    {{ t('admin.ops.errorLog.details') }}
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- Pagination -->
      <div class="bg-gray-50/50 dark:bg-dark-800/50">
        <Pagination
          v-if="total > 0"
          :total="total"
          :page="page"
          :page-size="pageSize"
          @update:page="emit('update:page', $event)"
          @update:pageSize="emit('update:pageSize', $event)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Pagination from '@/components/common/Pagination.vue'
import IpGeoCell from '@/components/common/IpGeoCell.vue'
import IpGeoBatchToolbar from '@/components/common/IpGeoBatchToolbar.vue'
import type { OpsErrorLog } from '@/api/admin/ops'
import { getSeverityClass, formatDateTime } from '../utils/opsFormatters'
import { mapErrorCategory } from '@/utils/errorCategory'
import { mapErrorSortKey } from '@/utils/errorBadges'

const { t } = useI18n()

function isUpstreamRow(log: OpsErrorLog): boolean {
  const phase = String(log.phase || '').toLowerCase()
  const owner = String(log.error_owner || '').toLowerCase()
  return phase === 'upstream' && owner === 'provider'
}

function userTooltip(log: OpsErrorLog): string {
  const userID = effectiveUserID(log)
  if (!userID) return ''
  return `${t('admin.ops.errorLog.userId')} ${userID}`
}

function userLabel(log: OpsErrorLog): string {
  return effectiveUserEmail(log) || (effectiveUserID(log) ? String(effectiveUserID(log)) : '-')
}

function effectiveUserID(log: OpsErrorLog): number {
  return log.user_id || log.deleted_key_owner_user_id || 0
}

function effectiveUserEmail(log: OpsErrorLog): string {
  return log.user_email || log.deleted_key_owner_email || ''
}

function apiKeyTooltip(log: OpsErrorLog): string {
  if (!log.api_key_id) return ''
  return `${t('admin.ops.errorLog.apiKeyId')} ${log.api_key_id}`
}

function apiKeyLabel(log: OpsErrorLog): string {
  return log.api_key_name || (log.api_key_id ? String(log.api_key_id) : '-')
}

function accountTooltip(log: OpsErrorLog): string {
  if (!log.account_id) return ''
  return `${t('admin.ops.errorLog.accountId')} ${log.account_id}`
}

function accountLabel(log: OpsErrorLog): string {
  return log.account_name || (log.account_id ? String(log.account_id) : '-')
}

function formatEndpointTooltip(log: OpsErrorLog): string {
  const parts: string[] = []
  if (log.inbound_endpoint) parts.push(`Inbound: ${log.inbound_endpoint}`)
  if (log.upstream_endpoint) parts.push(`Upstream: ${log.upstream_endpoint}`)
  return parts.join('\n') || ''
}

function hasModelMapping(log: OpsErrorLog): boolean {
  const requested = String(log.requested_model || '').trim()
  const upstream = String(log.upstream_model || '').trim()
  return !!requested && !!upstream && requested !== upstream
}

function modelMappingTooltip(log: OpsErrorLog): string {
  const requested = String(log.requested_model || '').trim()
  const upstream = String(log.upstream_model || '').trim()
  if (!requested && !upstream) return ''
  if (requested && upstream) return `${requested} → ${upstream}`
  return upstream || requested
}

function displayModel(log: OpsErrorLog): string {
  const upstream = String(log.upstream_model || '').trim()
  if (upstream) return upstream
  const requested = String(log.requested_model || '').trim()
  if (requested) return requested
  return String(log.model || '').trim()
}

function formatRequestType(type: number | null | undefined): string {
  switch (type) {
    case 1: return t('admin.ops.errorLog.requestTypeSync')
    case 2: return t('admin.ops.errorLog.requestTypeStream')
    case 3: return t('admin.ops.errorLog.requestTypeWs')
    case 4: return t('admin.ops.errorLog.requestTypeCyber')
    case 5: return t('admin.ops.errorLog.requestTypeImageWebBridge')
    case 6: return t('admin.ops.errorLog.requestTypeCyber')
    case 7: return t('admin.ops.errorLog.requestTypeVideo')
    case 8: return t('admin.ops.errorLog.requestTypeImage')
    default: return t('common.unknown')
  }
}

function getTypeBadge(log: OpsErrorLog): { label: string; className: string } {
  const phase = String(log.phase || '').toLowerCase()
  const owner = String(log.error_owner || '').toLowerCase()

  if (isUpstreamRow(log)) {
    return { label: t('admin.ops.errorLog.typeUpstream'), className: 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-400 dark:ring-red-500/30' }
  }
  if (owner === 'account') {
    return { label: t('admin.ops.errorLog.typeAccount'), className: 'bg-cyan-50 text-cyan-700 ring-cyan-600/20 dark:bg-cyan-900/30 dark:text-cyan-300 dark:ring-cyan-500/30' }
  }
  if (phase === 'request' && owner === 'client') {
    return { label: t('admin.ops.errorLog.typeRequest'), className: 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-400 dark:ring-amber-500/30' }
  }
  if (phase === 'auth' && owner === 'client') {
    return { label: t('admin.ops.errorLog.typeAuth'), className: 'bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900/30 dark:text-blue-400 dark:ring-blue-500/30' }
  }
  if (phase === 'routing' && owner === 'platform') {
    return { label: t('admin.ops.errorLog.typeRouting'), className: 'bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900/30 dark:text-purple-400 dark:ring-purple-500/30' }
  }
  if (phase === 'internal' && owner === 'platform') {
    return { label: t('admin.ops.errorLog.typeInternal'), className: 'bg-gray-100 text-gray-800 ring-gray-600/20 dark:bg-dark-700 dark:text-gray-200 dark:ring-dark-500/40' }
  }

    const fallback = phase || owner || t('common.unknown')
    return { label: fallback, className: 'bg-gray-50 text-gray-700 ring-gray-600/10 dark:bg-dark-900 dark:text-gray-300 dark:ring-dark-700' }
}

interface Props {
  rows: OpsErrorLog[]
  total: number
  loading: boolean
  page: number
  pageSize: number
  userClickable?: boolean
  visibleColumnKeys?: string[]
  flat?: boolean
}

interface Emits {
  (e: 'openErrorDetail', id: number): void
  (e: 'update:page', value: number): void
  (e: 'update:pageSize', value: number): void
  (e: 'ipGeoBatchFailed'): void
  (e: 'sort', sortBy: string, sortOrder: 'asc' | 'desc'): void
  (e: 'userClick', userId: number, email?: string): void
}

const props = withDefaults(defineProps<Props>(), {
  userClickable: false,
  flat: false
})
const emit = defineEmits<Emits>()

const allColumns = computed(() => [
  { key: 'created_at', label: t('admin.ops.errorLog.time'), sortable: true },
  { key: 'type', label: t('admin.ops.errorLog.type'), sortable: false },
  { key: 'category', label: t('usage.errors.category'), sortable: false },
  { key: 'endpoint', label: t('admin.ops.errorLog.endpoint'), sortable: false },
  { key: 'platform', label: t('admin.ops.errorLog.platform'), sortable: false },
  { key: 'model', label: t('admin.ops.errorLog.model'), sortable: true },
  { key: 'group', label: t('admin.ops.errorLog.group'), sortable: false },
  { key: 'user', label: t('admin.ops.errorLog.user'), sortable: false },
  { key: 'api_key', label: t('admin.ops.errorLog.apiKey'), sortable: false },
  { key: 'account', label: t('admin.ops.errorLog.account'), sortable: false },
  { key: 'status', label: t('admin.ops.errorLog.status'), sortable: true },
  { key: 'message', label: t('admin.ops.errorLog.message'), sortable: false },
  { key: 'user_agent', label: t('usage.userAgent'), sortable: false },
  { key: 'client_ip', label: t('admin.ops.errorLog.ip'), sortable: false },
  { key: 'actions', label: t('admin.ops.errorLog.action'), sortable: false }
])

const columns = computed(() =>
  props.visibleColumnKeys
    ? allColumns.value.filter((column) => props.visibleColumnKeys!.includes(column.key))
    : allColumns.value
)
const isColumnVisible = (key: string) => columns.value.some((column) => column.key === key)
const sortKey = ref('created_at')
const sortOrder = ref<'asc' | 'desc'>('desc')

function onSort(key: string) {
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortOrder.value = 'asc'
  }
  emit('sort', mapErrorSortKey(key), sortOrder.value)
}

function columnAriaSort(key: string): 'ascending' | 'descending' | 'none' {
  if (sortKey.value !== key) return 'none'
  return sortOrder.value === 'asc' ? 'ascending' : 'descending'
}

function getStatusClass(code: number): string {
  if (code >= 500) return 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-400 dark:ring-red-500/30'
  if (code === 429) return 'bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900/30 dark:text-purple-400 dark:ring-purple-500/30'
  if (code >= 400) return 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-400 dark:ring-amber-500/30'
  return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/30 dark:text-gray-400 dark:ring-gray-500/30'
}

function formatSmartMessage(msg: string): string {
  if (!msg) return ''

  if (msg.startsWith('{') || msg.startsWith('[')) {
    try {
      const obj = JSON.parse(msg)
      if (obj?.error?.message) return String(obj.error.message)
      if (obj?.message) return String(obj.message)
      if (obj?.detail) return String(obj.detail)
      if (typeof obj === 'object') return JSON.stringify(obj).substring(0, 150)
    } catch {
      // ignore parse error
    }
  }

  if (msg.includes('context deadline exceeded')) return t('admin.ops.errorLog.commonErrors.contextDeadlineExceeded')
  if (msg.includes('connection refused')) return t('admin.ops.errorLog.commonErrors.connectionRefused')
  if (msg.toLowerCase().includes('rate limit')) return t('admin.ops.errorLog.commonErrors.rateLimit')

  return msg.length > 200 ? msg.substring(0, 200) + '...' : msg

}
</script>
