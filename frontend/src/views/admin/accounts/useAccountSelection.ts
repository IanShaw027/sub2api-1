// Row selection (checkboxes, swipe-select, "select all N results") for AccountsView.vue
// (frontend-health-cleanup 6.4 split).
import { computed, ref, type Ref } from 'vue'
import { useTableSelection } from '@/composables/useTableSelection'
import { useSwipeSelect, type SwipeSelectVirtualContext } from '@/composables/useSwipeSelect'
import { fetchAllAccountIds } from '@/utils/accountSelection'
import { adminAPI } from '@/api/admin'
import type { Account, AccountPlatform, AccountType } from '@/types'
import DataTable from '@/components/common/DataTable.vue'

interface UseAccountSelectionOptions {
  accounts: Ref<Account[]>
  appStore: { showError: (message: string) => void }
  t: (key: string, params?: any) => string
  /** Reactive pagination from useAccountListState — only `.total` is read here. */
  pagination: { total: number }
  /** Snapshot of the current filters/sort, from useAccountListState. */
  buildBulkEditFilterSnapshot: () => Record<string, unknown>
}

/**
 * Wraps the generic useTableSelection composable with the account-specific bits:
 * swipe-to-select over the (possibly virtualized) DataTable, and "select all N
 * results across every page" which requires an extra id-fetching round trip.
 */
export function useAccountSelection(options: UseAccountSelectionOptions) {
  const { accounts, appStore, t, pagination, buildBulkEditFilterSnapshot } = options

  const accountTableRef = ref<HTMLElement | null>(null)
  const dataTableRef = ref<InstanceType<typeof DataTable> | null>(null)

  const selPlatforms = computed<AccountPlatform[]>(() => {
    const platforms = new Set(
      accounts.value
        .filter(a => isSelected(a.id))
        .map(a => a.platform)
    )
    return [...platforms]
  })
  const selTypes = computed<AccountType[]>(() => {
    const types = new Set(
      accounts.value
        .filter(a => isSelected(a.id))
        .map(a => a.type)
    )
    return [...types]
  })

  const {
    selectedSet,
    selectedIds: selIds,
    allVisibleSelected,
    isSelected,
    setSelectedIds,
    select,
    deselect,
    toggle: toggleSel,
    clear: clearSelectedIds,
    removeMany: removeSelectedAccounts,
    toggleVisible,
    selectVisible: selectCurrentPage,
    batchUpdate
  } = useTableSelection<Account>({
    rows: accounts,
    getId: (account) => account.id
  })

  const selectingAllResults = ref(false)
  const selectedAllResultIDs = ref<Set<number> | null>(null)
  const selectionRequestVersion = ref(0)
  const allResultsSelected = computed(() => {
    const snapshot = selectedAllResultIDs.value
    if (!snapshot || snapshot.size === 0 || snapshot.size !== selectedSet.value.size) return false
    return Array.from(snapshot).every(id => selectedSet.value.has(id))
  })

  const clearSelection = () => {
    selectionRequestVersion.value++
    selectingAllResults.value = false
    selectedAllResultIDs.value = null
    clearSelectedIds()
  }

  const selectPage = () => {
    selectCurrentPage()
  }

  const swipeVirtualContext: SwipeSelectVirtualContext = {
    getVirtualizer: () => dataTableRef.value?.virtualizer ?? null,
    getSortedData: () => dataTableRef.value?.sortedData ?? accounts.value,
    getRowId: (row: any) => row.id
  }

  useSwipeSelect(accountTableRef, {
    isSelected,
    select,
    deselect,
    batchUpdate
  }, swipeVirtualContext)

  const toggleSelectAllVisible = (event: Event) => {
    const target = event.target as HTMLInputElement
    toggleVisible(target.checked)
  }

  const handleSelectAllResults = async () => {
    if (selectingAllResults.value || pagination.total === 0) return

    const requestVersion = ++selectionRequestVersion.value
    const filters = buildBulkEditFilterSnapshot()
    selectingAllResults.value = true
    try {
      const ids = await fetchAllAccountIds(
        (page, pageSize, requestFilters) => adminAPI.accounts.list(page, pageSize, requestFilters),
        filters
      )
      if (requestVersion !== selectionRequestVersion.value) return

      setSelectedIds(ids)
      selectedAllResultIDs.value = new Set(ids)
    } catch (error) {
      if (requestVersion !== selectionRequestVersion.value) return
      console.error('Failed to select all account results:', error)
      appStore.showError(t('admin.accounts.bulkActions.selectAllFailed'))
    } finally {
      if (requestVersion === selectionRequestVersion.value) {
        selectingAllResults.value = false
      }
    }
  }

  return {
    accountTableRef,
    dataTableRef,
    selPlatforms,
    selTypes,
    selectedSet,
    selIds,
    allVisibleSelected,
    isSelected,
    setSelectedIds,
    select,
    deselect,
    toggleSel,
    removeSelectedAccounts,
    selectingAllResults,
    allResultsSelected,
    clearSelection,
    selectPage,
    toggleSelectAllVisible,
    handleSelectAllResults
  }
}

export type AccountSelectionState = ReturnType<typeof useAccountSelection>
