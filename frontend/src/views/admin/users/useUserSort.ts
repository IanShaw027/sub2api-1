/** Admin UsersView: server-side column sort state plus the independent,
 * client-side "usage" column sort.
 *
 * DataTable runs in server-side-sort mode — every sortable field triggers a
 * backend query — but the usage columns are batch-fetched asynchronously
 * after the main page load, so they get their own local-sort layer that
 * re-orders the current page's rows in the browser. The one exception is
 * the "last 30 days" usage metric, which the backend can sort natively
 * (`sort_by: 'last_30d_usage'`); that case is treated as a server sort so
 * the client-side re-sort is skipped for it.
 */
import { computed, reactive, ref, type Ref } from 'vue'
import type { AdminUser } from '@/types'
import type { BatchUserUsageStats } from '@/api/admin/dashboard'
import { USAGE_COLUMN_KEYS, USAGE_COLUMN_PLATFORMS } from './useUserColumns'

export const USER_SORT_STORAGE_KEY = 'admin-users-table-sort'

const loadInitialSortState = (): { sort_by: string; sort_order: 'asc' | 'desc' } => {
  const fallback = { sort_by: 'created_at', sort_order: 'desc' as 'asc' | 'desc' }
  const sortable = new Set(['email', 'id', 'username', 'role', 'balance', 'concurrency', 'current_concurrency', 'available_concurrency', 'status', 'last_used_at', 'last_active_at', 'created_at', 'last_30d_usage'])
  try {
    const raw = localStorage.getItem(USER_SORT_STORAGE_KEY)
    if (!raw) return fallback
    const parsed = JSON.parse(raw) as { key?: string; order?: string }
    const key = typeof parsed.key === 'string' ? parsed.key : ''
    if (!sortable.has(key)) return fallback
    return {
      sort_by: key,
      sort_order: parsed.order === 'asc' ? 'asc' : 'desc'
    }
  } catch {
    return fallback
  }
}

type UsageMetric = 'today' | 'total'
type UsageSortState = { key: string; metric: UsageMetric; order: 'asc' | 'desc' } | null
const USAGE_SORT_STORAGE_KEY = 'admin-users-usage-sort'

const loadInitialUsageSort = (): UsageSortState => {
  try {
    const raw = localStorage.getItem(USAGE_SORT_STORAGE_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<{ key: string; metric: string; order: string }>
    if (!parsed.key || !USAGE_COLUMN_KEYS.includes(parsed.key)) return null
    const metric: UsageMetric = parsed.metric === 'total' ? 'total' : 'today'
    const order: 'asc' | 'desc' = parsed.order === 'asc' ? 'asc' : 'desc'
    return { key: parsed.key, metric, order }
  } catch {
    return null
  }
}

export interface UseUserSortCallbacks {
  /** Fired when the "last 30 days" usage sort is (de)activated — needs a server refetch at page 1. */
  onServerUsageSortChanged: () => void
}

export function useUserSort(
  users: Ref<AdminUser[]>,
  usageStats: Ref<Record<string, BatchUserUsageStats>>,
  callbacks: UseUserSortCallbacks
) {
  const sortState = reactive(loadInitialSortState())

  // 用量列前端排序：DataTable 工作在 server-side-sort 模式，所有 sortable
  // 字段都会触发后端查询，而用量列数据是异步批量拉取后再合并到当前页，
  // 因此采用独立的前端排序状态对当前页 users 做本地排序。
  // 排序状态独立于后端 sortState 持久化；缺失数据按 0 处理（desc 沉底、asc 置顶）。
  // 列头排序按钮点击后弹出的"今日/近30天"选择菜单，同时只允许一个列展开。
  const openUsageSortMenu = ref<string | null>(null)
  const usageSort = ref<UsageSortState>(loadInitialUsageSort())

  const persistUsageSort = () => {
    try {
      if (usageSort.value) {
        localStorage.setItem(USAGE_SORT_STORAGE_KEY, JSON.stringify(usageSort.value))
      } else {
        localStorage.removeItem(USAGE_SORT_STORAGE_KEY)
      }
    } catch (e) {
      console.error('Failed to persist usage sort:', e)
    }
  }

  const clearUsageSort = () => {
    if (!usageSort.value) return
    usageSort.value = null
    openUsageSortMenu.value = null
    persistUsageSort()
  }

  const isUsageSortActive = (key: string, metric: UsageMetric) =>
    !!usageSort.value && usageSort.value.key === key && usageSort.value.metric === metric
  const getUsageSortOrder = (key: string, metric: UsageMetric): 'asc' | 'desc' | null =>
    isUsageSortActive(key, metric) ? usageSort.value!.order : null

  // 三态循环：desc → asc → off。选完即关闭菜单（用户大多希望"选中即应用"，
  // 想再切换 order 时重新打开菜单点同一项即可）。
  const isServerLast30dSort = computed(() =>
    usageSort.value?.key === 'usage' && usageSort.value?.metric === 'total'
  )

  const toggleUsageSort = (key: string, metric: UsageMetric) => {
    const cur = usageSort.value
    if (cur && cur.key === key && cur.metric === metric) {
      usageSort.value = cur.order === 'desc' ? { key, metric, order: 'asc' } : null
    } else {
      usageSort.value = { key, metric, order: 'desc' }
    }
    persistUsageSort()
    openUsageSortMenu.value = null
    if (key === 'usage' && metric === 'total') {
      callbacks.onServerUsageSortChanged()
    }
  }

  // 点击图标本身不触发排序，仅开关菜单；首次排序由用户在菜单内选择 metric 触发（默认 desc，详见 toggleUsageSort）。
  const toggleUsageSortMenu = (key: string) => {
    openUsageSortMenu.value = openUsageSortMenu.value === key ? null : key
  }

  const getUsageValue = (userId: number, key: string, metric: UsageMetric): number => {
    const stats = usageStats.value[userId]
    if (!stats) return 0
    const platform = USAGE_COLUMN_PLATFORMS[key]
    if (platform === null) {
      return metric === 'today' ? stats.today_actual_cost ?? 0 : stats.total_actual_cost ?? 0
    }
    const p = stats.by_platform?.find((x) => x.platform === platform)
    if (!p) return 0
    return metric === 'today' ? p.today_actual_cost ?? 0 : p.total_actual_cost ?? 0
  }

  // 在 server-side 排序结果之上叠加用量列的本地排序；无 usageSort 时直接透传原数组。
  // 稳定排序：等值按原 index 保序，避免拉取新用量数据时表行抖动。
  const sortedUsers = computed(() => {
    const s = usageSort.value
    if (!s || isServerLast30dSort.value) return users.value
    return [...users.value]
      .map((row, index) => ({ row, index }))
      .sort((a, b) => {
        const av = getUsageValue(a.row.id, s.key, s.metric)
        const bv = getUsageValue(b.row.id, s.key, s.metric)
        if (av !== bv) return s.order === 'asc' ? av - bv : bv - av
        return a.index - b.index
      })
      .map((x) => x.row)
  })

  const handleSort = (key: string, order: 'asc' | 'desc') => {
    clearUsageSort()
    sortState.sort_by = key
    sortState.sort_order = order
  }

  return {
    sortState,
    usageSort,
    openUsageSortMenu,
    isServerLast30dSort,
    sortedUsers,
    clearUsageSort,
    isUsageSortActive,
    getUsageSortOrder,
    toggleUsageSort,
    toggleUsageSortMenu,
    getUsageValue,
    handleSort
  }
}
