/** Admin UsersView: the "secondary" per-row data that's batch-fetched after
 * the main user list request resolves — usage stats, custom attribute
 * values, and per-platform balance quotas. Kept separate from the main list
 * load so the table can render before these heavier, optional columns
 * finish loading. */
import { ref, type Ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { AdminUser, UserAttributeDefinition } from '@/types'
import type { BatchUserUsageStats } from '@/api/admin/dashboard'
import type { PlatformQuotaItem } from '@/api/admin/users'

export interface UseUserSecondaryDataFlags {
  hasVisibleUsageColumn: Ref<boolean>
  hasVisibleAttributeColumns: Ref<boolean>
  hasVisiblePlatformQuotaColumn: Ref<boolean>
}

export function useUserSecondaryData(
  users: Ref<AdminUser[]>,
  attributeDefinitions: Ref<UserAttributeDefinition[]>,
  flags: UseUserSecondaryDataFlags
) {
  const usageStats = ref<Record<string, BatchUserUsageStats>>({})
  const userAttributeValues = ref<Record<number, Record<number, string>>>({})
  const platformQuotaStats = ref<Record<number, PlatformQuotaItem[]>>({})

  const getPlatformUsage = (userId: number, platform: string) =>
    usageStats.value[userId]?.by_platform?.find((p) => p.platform === platform)

  // Get formatted attribute value for display in table
  const getAttributeValue = (userId: number, attrId: number): string => {
    const userAttrs = userAttributeValues.value[userId]
    if (!userAttrs) return '-'
    const value = userAttrs[attrId]
    if (!value) return '-'

    // Find definition for this attribute
    const def = attributeDefinitions.value.find(d => d.id === attrId)
    if (!def) return value

    // Format based on type
    if (def.type === 'multi_select' && value) {
      try {
        const arr = JSON.parse(value)
        if (Array.isArray(arr)) {
          // Map values to labels
          return arr.map(v => {
            const opt = def.options?.find(o => o.value === v)
            return opt?.label || v
          }).join(', ')
        }
      } catch {
        return value
      }
    }

    if (def.type === 'select' && value && def.options) {
      const opt = def.options.find(o => o.value === value)
      return opt?.label || value
    }

    return value
  }

  let secondaryDataSeq = 0

  const loadUsersSecondaryData = async (
    userIds: number[],
    signal?: AbortSignal,
    expectedSeq?: number
  ) => {
    if (userIds.length === 0) return

    const tasks: Promise<void>[] = []

    if (flags.hasVisibleUsageColumn.value) {
      tasks.push(
        (async () => {
          try {
            const usageResponse = await adminAPI.dashboard.getBatchUsersUsage(userIds)
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            usageStats.value = usageResponse.stats
          } catch (e) {
            if (signal?.aborted) return
            console.error('Failed to load usage stats:', e)
          }
        })()
      )
    }

    if (attributeDefinitions.value.length > 0 && flags.hasVisibleAttributeColumns.value) {
      tasks.push(
        (async () => {
          try {
            const attrResponse = await adminAPI.userAttributes.getBatchUserAttributes(userIds)
            if (signal?.aborted) return
            if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
            userAttributeValues.value = attrResponse.attributes
          } catch (e) {
            if (signal?.aborted) return
            console.error('Failed to load user attribute values:', e)
          }
        })()
      )
    }

    if (flags.hasVisiblePlatformQuotaColumn.value) {
      tasks.push(
        (async () => {
          try {
            // 无批量端点：对当前页用户逐个拉取，分块并发（每批 6），批间检查中止条件，避免大 pageSize 时请求洪峰
            const CHUNK = 6
            for (let i = 0; i < userIds.length; i += CHUNK) {
              if (signal?.aborted) return
              if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
              const chunk = userIds.slice(i, i + CHUNK)
              const results = await Promise.allSettled(
                chunk.map((id) => adminAPI.users.getPlatformQuotas(id))
              )
              if (signal?.aborted) return
              if (typeof expectedSeq === 'number' && expectedSeq !== secondaryDataSeq) return
              const merged = { ...platformQuotaStats.value }
              results.forEach((r, idx) => {
                if (r.status === 'fulfilled') {
                  merged[chunk[idx]] = r.value.platform_quotas || []
                }
              })
              platformQuotaStats.value = merged
            }
          } catch (e) {
            if (signal?.aborted) return
            console.error('Failed to load platform quotas:', e)
          }
        })()
      )
    }

    if (tasks.length > 0) {
      await Promise.allSettled(tasks)
    }
  }

  const refreshCurrentPageSecondaryData = () => {
    const userIds = users.value.map((u) => u.id)
    if (userIds.length === 0) return
    const seq = ++secondaryDataSeq
    void loadUsersSecondaryData(userIds, undefined, seq)
  }

  /** Bump the sequence number and return it, for callers (loadUsers) that
   * schedule a deferred secondary-data load themselves. */
  const nextSecondaryDataSeq = () => ++secondaryDataSeq
  const isCurrentSecondaryDataSeq = (seq: number) => seq === secondaryDataSeq
  const resetSecondaryData = () => {
    usageStats.value = {}
    userAttributeValues.value = {}
    platformQuotaStats.value = {}
  }

  return {
    usageStats,
    userAttributeValues,
    platformQuotaStats,
    getPlatformUsage,
    getAttributeValue,
    loadUsersSecondaryData,
    refreshCurrentPageSecondaryData,
    nextSecondaryDataSeq,
    isCurrentSecondaryDataSeq,
    resetSecondaryData
  }
}
