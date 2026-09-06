// Mutually-exclusive header/filter-bar dropdown menus for AccountsView.vue
// (frontend-health-cleanup 6.4 split): import/export, columns, account-tools
// (Teleported, position-calculated) and auto-refresh. Owning all four booleans
// together keeps the "opening one closes the other three" rule in one place.
import { computed, reactive, ref } from 'vue'
import { getFloatingPanelPosition } from '@/utils/floatingPanel'

export function useAccountToolbarMenus() {
  const showImportExportDropdown = ref(false)
  const importExportDropdownRef = ref<HTMLElement | null>(null)

  const showColumnsDropdown = ref(false)
  const columnsDropdownRef = ref<HTMLElement | null>(null)

  const showAutoRefreshDropdown = ref(false)
  const autoRefreshDropdownRef = ref<HTMLElement | null>(null)

  const showAccountToolsDropdown = ref(false)
  const accountToolsDropdownRef = ref<HTMLElement | null>(null)
  const accountToolsTriggerRef = ref<HTMLElement | { $el?: HTMLElement } | null>(null)
  const accountToolsDropdownPosition = reactive({
    top: null as number | null,
    bottom: null as number | null,
    left: 16,
    width: 320,
    maxHeight: 0
  })
  const accountToolsDropdownStyle = computed(() => ({
    top: accountToolsDropdownPosition.top == null ? 'auto' : `${accountToolsDropdownPosition.top}px`,
    bottom: accountToolsDropdownPosition.bottom == null ? 'auto' : `${accountToolsDropdownPosition.bottom}px`,
    left: `${accountToolsDropdownPosition.left}px`,
    width: `${accountToolsDropdownPosition.width}px`
  }))

  const closeAccountToolsDropdown = () => {
    showAccountToolsDropdown.value = false
  }

  const updateAccountToolsDropdownPosition = () => {
    const raw = accountToolsTriggerRef.value as HTMLElement | { $el?: HTMLElement } | null
    const trigger = (raw && '$el' in raw ? raw.$el : raw) as HTMLElement | null | undefined
    if (!trigger || typeof trigger.getBoundingClientRect !== 'function') return

    const position = getFloatingPanelPosition(
      trigger.getBoundingClientRect(),
      document.documentElement.clientWidth || window.innerWidth,
      window.innerHeight
    )
    Object.assign(accountToolsDropdownPosition, position)
  }

  const toggleAccountToolsDropdown = () => {
    const nextVisible = !showAccountToolsDropdown.value
    showAutoRefreshDropdown.value = false
    showImportExportDropdown.value = false
    showColumnsDropdown.value = false
    if (nextVisible) updateAccountToolsDropdownPosition()
    showAccountToolsDropdown.value = nextVisible
  }

  const toggleImportExportDropdown = () => {
    const nextVisible = !showImportExportDropdown.value
    showAccountToolsDropdown.value = false
    showAutoRefreshDropdown.value = false
    showColumnsDropdown.value = false
    showImportExportDropdown.value = nextVisible
  }

  const toggleAutoRefreshDropdown = () => {
    const nextVisible = !showAutoRefreshDropdown.value
    showAccountToolsDropdown.value = false
    showImportExportDropdown.value = false
    showColumnsDropdown.value = false
    showAutoRefreshDropdown.value = nextVisible
  }

  const toggleColumnsDropdown = () => {
    const nextVisible = !showColumnsDropdown.value
    showAccountToolsDropdown.value = false
    showImportExportDropdown.value = false
    showAutoRefreshDropdown.value = false
    showColumnsDropdown.value = nextVisible
  }

  return {
    showImportExportDropdown,
    importExportDropdownRef,
    showColumnsDropdown,
    columnsDropdownRef,
    showAutoRefreshDropdown,
    autoRefreshDropdownRef,
    showAccountToolsDropdown,
    accountToolsDropdownRef,
    accountToolsTriggerRef,
    accountToolsDropdownPosition,
    accountToolsDropdownStyle,
    closeAccountToolsDropdown,
    updateAccountToolsDropdownPosition,
    toggleAccountToolsDropdown,
    toggleImportExportDropdown,
    toggleAutoRefreshDropdown,
    toggleColumnsDropdown
  }
}

export type AccountToolbarMenusState = ReturnType<typeof useAccountToolbarMenus>
