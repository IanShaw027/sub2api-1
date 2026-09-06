/** Admin UsersView: the shared groups list backing the "groups" table cell,
 * the "authorised group" filter, and the API-key-group filter. */
import { ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { AdminGroup, AdminUser } from '@/types'

export function useUserGroupsData() {
  // Groups data for the groups column and the existing "authorised group" filter (active only)
  const allGroups = ref<AdminGroup[]>([])
  const loadAllGroups = async () => {
    if (allGroups.value.length > 0) return
    try {
      allGroups.value = await adminAPI.groups.getAll()
    } catch (e) {
      console.error('Failed to load groups:', e)
    }
  }

  // Groups for the API Key group filter — includes disabled groups so admins can
  // filter users whose keys are still bound to a now-disabled group.
  const allGroupsForApiKeyFilter = ref<AdminGroup[]>([])
  const loadAllGroupsForApiKeyFilter = async () => {
    if (allGroupsForApiKeyFilter.value.length > 0) return
    try {
      allGroupsForApiKeyFilter.value = await adminAPI.groups.getAllIncludingInactive()
    } catch (e) {
      console.error('Failed to load groups for API key filter:', e)
    }
  }

  // Resolve user's accessible groups: exclusive groups first, then public groups
  const getUserGroups = (user: AdminUser) => {
    const exclusive: AdminGroup[] = []
    const publicGroups: AdminGroup[] = []
    for (const g of allGroups.value) {
      if (g.status !== 'active' || g.subscription_type !== 'standard') continue
      if (g.is_exclusive) {
        if (user.allowed_groups?.includes(g.id)) {
          exclusive.push(g)
        }
      } else {
        publicGroups.push(g)
      }
    }
    return { exclusive, publicGroups }
  }

  // 计算剩余天数
  const getDaysRemaining = (expiresAt: string): number => {
    const now = new Date()
    const expires = new Date(expiresAt)
    const diffMs = expires.getTime() - now.getTime()
    return Math.ceil(diffMs / (1000 * 60 * 60 * 24))
  }

  return {
    allGroups,
    loadAllGroups,
    allGroupsForApiKeyFilter,
    loadAllGroupsForApiKeyFilter,
    getUserGroups,
    getDaysRemaining
  }
}
