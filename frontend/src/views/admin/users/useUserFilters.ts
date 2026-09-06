/** Admin UsersView: filter state (role/status/group/api-key-group + custom
 * attribute filters), the "visible filters" toggle set, and the derived
 * filter-option lists. Persisted to localStorage like the column settings. */
import { computed, reactive, ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminGroup, UserAttributeDefinition } from '@/types'
import type { SelectOption } from '@/components/common/Select.vue'
import { buildApiKeyGroupFilterOptions } from '../apiKeyGroupFilterOptions'

export interface UseUserFiltersCallbacks {
  loadAllGroups: () => void
  loadAllGroupsForApiKeyFilter: () => void
  /** Used by toggleBuiltInFilter/toggleAttributeFilter: reset to page 1, then reload. */
  resetPageAndReload: () => void
  /** Used by applyFilter (select/search-triggered changes keep the current page). */
  reload: () => void
}

export function useUserFilters(
  attributeDefinitions: Ref<UserAttributeDefinition[]>,
  allGroups: Ref<AdminGroup[]>,
  allGroupsForApiKeyFilter: Ref<AdminGroup[]>,
  callbacks: UseUserFiltersCallbacks
) {
  const { t } = useI18n()

  // Filter values (role, status, and custom attributes)
  const filters = reactive({
    role: '',
    status: '',
    group: '',  // group name for fuzzy match, '' = all
    apiKeyGroup: null as number | null  // group id bound to the user's API keys, null = all
  })
  const activeAttributeFilters = reactive<Record<number, string>>({})

  // Visible filters tracking (which filters are shown in the UI)
  // Keys: 'role', 'status', 'attr_${id}'
  const visibleFilters = reactive<Set<string>>(new Set())

  // Dropdown states
  const showFilterDropdown = ref(false)

  // Dropdown refs for click outside detection
  const filterDropdownRef = ref<HTMLElement | null>(null)

  // localStorage keys
  const FILTER_VALUES_KEY = 'user-filter-values'
  const VISIBLE_FILTERS_KEY = 'user-visible-filters'

  // All filterable attribute definitions (enabled attributes)
  const filterableAttributes = computed(() =>
    attributeDefinitions.value.filter(def => def.enabled)
  )

  // Built-in filter definitions
  const builtInFilters = computed(() => [
    { key: 'role', name: t('admin.users.columns.role'), type: 'select' as const },
    { key: 'status', name: t('admin.users.columns.status'), type: 'select' as const },
    { key: 'group', name: t('admin.users.authorizedGroupFilter'), type: 'select' as const },
    { key: 'apiKeyGroup', name: t('admin.users.apiKeyGroupFilter'), type: 'select' as const }
  ])

  // Group filter options: "All Groups" + active exclusive groups (value = group name for fuzzy match)
  const groupFilterOptions = computed(() => {
    const options: { value: string; label: string }[] = [
      { value: '', label: t('admin.users.allAuthorizedGroups') }
    ]
    for (const g of allGroups.value) {
      if (g.status !== 'active' || !g.is_exclusive || g.subscription_type !== 'standard') continue
      options.push({ value: g.name, label: g.name })
    }
    return options
  })

  // API Key group filter options: "All" + groups partitioned by type (value = group id).
  // Uses allGroupsForApiKeyFilter which includes disabled groups.
  const apiKeyGroupFilterOptions = computed(() =>
    buildApiKeyGroupFilterOptions(allGroupsForApiKeyFilter.value, {
      all: t('admin.users.allApiKeyGroups'),
      exclusive: t('admin.users.apiKeyGroupExclusive'),
      public: t('admin.users.apiKeyGroupPublic'),
      subscription: t('admin.users.apiKeyGroupSubscription'),
      disabled: t('admin.users.apiKeyGroupDisabled'),
    }) as SelectOption[]
  )

  // Save filters to localStorage
  const saveFiltersToStorage = () => {
    try {
      // Save visible filters
      localStorage.setItem(VISIBLE_FILTERS_KEY, JSON.stringify([...visibleFilters]))
      // Save filter values
      const values = {
        role: filters.role,
        status: filters.status,
        group: filters.group,
        apiKeyGroup: filters.apiKeyGroup,
        attributes: activeAttributeFilters
      }
      localStorage.setItem(FILTER_VALUES_KEY, JSON.stringify(values))
    } catch (e) {
      console.error('Failed to save filters:', e)
    }
  }

  // Load saved filters from localStorage
  const loadSavedFilters = () => {
    try {
      // Load visible filters
      const savedVisible = localStorage.getItem(VISIBLE_FILTERS_KEY)
      if (savedVisible) {
        const parsed = JSON.parse(savedVisible) as string[]
        parsed.forEach(key => visibleFilters.add(key))
      }
      // Load filter values
      const savedValues = localStorage.getItem(FILTER_VALUES_KEY)
      if (savedValues) {
        const parsed = JSON.parse(savedValues)
        if (parsed.role) filters.role = parsed.role
        if (parsed.status) filters.status = parsed.status
        if (parsed.group) filters.group = parsed.group
        if (typeof parsed.apiKeyGroup === 'number') filters.apiKeyGroup = parsed.apiKeyGroup
        if (parsed.attributes) {
          Object.assign(activeAttributeFilters, parsed.attributes)
        }
      }
    } catch (e) {
      console.error('Failed to load saved filters:', e)
    }
  }

  // Toggle a built-in filter (role/status)
  const toggleBuiltInFilter = (key: string) => {
    if (visibleFilters.has(key)) {
      visibleFilters.delete(key)
      if (key === 'role') filters.role = ''
      if (key === 'status') filters.status = ''
      if (key === 'group') filters.group = ''
      if (key === 'apiKeyGroup') filters.apiKeyGroup = null
    } else {
      visibleFilters.add(key)
      if (key === 'group') callbacks.loadAllGroups()
      if (key === 'apiKeyGroup') callbacks.loadAllGroupsForApiKeyFilter()
    }
    saveFiltersToStorage()
    callbacks.resetPageAndReload()
  }

  // Toggle a custom attribute filter
  const toggleAttributeFilter = (attr: UserAttributeDefinition) => {
    const key = `attr_${attr.id}`
    if (visibleFilters.has(key)) {
      visibleFilters.delete(key)
      delete activeAttributeFilters[attr.id]
    } else {
      visibleFilters.add(key)
      activeAttributeFilters[attr.id] = ''
    }
    saveFiltersToStorage()
    callbacks.resetPageAndReload()
  }

  const updateAttributeFilter = (attrId: number, value: string) => {
    activeAttributeFilters[attrId] = value
  }

  // Apply filter and save to localStorage
  const applyFilter = () => {
    saveFiltersToStorage()
    callbacks.reload()
  }

  return {
    filters,
    activeAttributeFilters,
    visibleFilters,
    showFilterDropdown,
    filterDropdownRef,
    filterableAttributes,
    builtInFilters,
    groupFilterOptions,
    apiKeyGroupFilterOptions,
    loadSavedFilters,
    saveFiltersToStorage,
    toggleBuiltInFilter,
    toggleAttributeFilter,
    updateAttributeFilter,
    applyFilter
  }
}
