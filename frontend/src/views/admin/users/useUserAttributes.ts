/** Admin UsersView: user attribute *definitions* (the global, enabled attribute
 * schema used to build dynamic columns/filters) — not the per-user values,
 * which are batch-loaded alongside the rest of the page's secondary data
 * (see useUserSecondaryData.ts). */
import { ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { UserAttributeDefinition } from '@/types'

export function useUserAttributes() {
  const attributeDefinitions = ref<UserAttributeDefinition[]>([])

  const loadAttributeDefinitions = async () => {
    try {
      attributeDefinitions.value = await adminAPI.userAttributes.listEnabledDefinitions()
    } catch (e) {
      console.error('Failed to load attribute definitions:', e)
    }
  }

  const getAttributeDefinition = (attrId: number): UserAttributeDefinition | undefined => {
    return attributeDefinitions.value.find(d => d.id === attrId)
  }

  const getAttributeDefinitionName = (attrId: number): string => {
    const def = attributeDefinitions.value.find(d => d.id === attrId)
    return def?.name || String(attrId)
  }

  return {
    attributeDefinitions,
    loadAttributeDefinitions,
    getAttributeDefinition,
    getAttributeDefinitionName
  }
}
