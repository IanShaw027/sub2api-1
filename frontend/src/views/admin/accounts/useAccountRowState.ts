// Dialog / row-menu state for AccountsView.vue (frontend-health-cleanup 6.4 split).
// Pure state container: refs + reactive menu object only, no handlers. Handlers that
// mutate this state live in useAccountRowActions.ts / useAccountBulkActions.ts so this
// file stays a small, easy-to-scan list of "what dialog is open, for which account".
import { computed, reactive, ref } from 'vue'
import type { Account, AccountPlatform, AccountType } from '@/types'
import type { SelectOption } from '@/components/common/Select.vue'
import type { AccountSortOrder } from './useAccountListState'

export type AccountBulkEditTarget =
  | {
      mode: 'selected'
      accountIds: number[]
      selectedPlatforms: AccountPlatform[]
      selectedTypes: AccountType[]
    }
  | {
      mode: 'filtered'
      filters: {
        platform?: string
        type?: string
        status?: string
        group?: string
        search?: string
        privacy_mode?: string
        sort_by?: string
        sort_order?: AccountSortOrder
      }
      previewCount: number
      selectedPlatforms: AccountPlatform[]
      selectedTypes: AccountType[]
    }

export function useAccountRowState() {
  const showCreate = ref(false)
  const showEdit = ref(false)
  const showSync = ref(false)
  const showImportData = ref(false)
  const showExportDataDialog = ref(false)
  const includeProxyOnExport = ref(true)
  const showBulkEdit = ref(false)
  const bulkEditTarget = ref<AccountBulkEditTarget | null>(null)
  const showTempUnsched = ref(false)
  const showDeleteDialog = ref(false)
  const showResetDeviceProfileDialog = ref(false)
  const showDeviceProfileInspect = ref(false)
  const resettingDeviceProfile = ref(false)
  const showCreateShadowDialog = ref(false)
  const showReAuth = ref(false)
  const showTest = ref(false)
  const showStats = ref(false)
  const showErrorPassthrough = ref(false)
  const showTLSFingerprintProfiles = ref(false)
  const showTLSFingerprintRouters = ref(false)
  const showCapacityForecast = ref(false)
  const showCyberEvents = ref(false)
  const showCyberErrorDetail = ref(false)
  const cyberEventsAcc = ref<Account | null>(null)
  const cyberErrorID = ref<number | null>(null)
  const edAcc = ref<Account | null>(null)
  const tempUnschedAcc = ref<Account | null>(null)
  const deletingAcc = ref<Account | null>(null)
  const resettingDeviceAcc = ref<Account | null>(null)
  const inspectingDeviceAcc = ref<Account | null>(null)
  const creatingShadowAcc = ref<Account | null>(null)
  const reAuthAcc = ref<Account | null>(null)
  const testingAcc = ref<Account | null>(null)
  const statsAcc = ref<Account | null>(null)
  const showSchedulePanel = ref(false)
  const scheduleAcc = ref<Account | null>(null)
  const scheduleModelOptions = ref<SelectOption[]>([])
  const togglingSchedulable = ref<number | null>(null)
  const menu = reactive<{ show: boolean; acc: Account | null; pos: { top: number; left: number } | null }>({
    show: false,
    acc: null,
    pos: null
  })

  // Whether any full-screen dialog/panel is open — used to pause auto-refresh /
  // upstream-billing polling while the user is looking at a modal.
  const isAnyModalOpen = computed(() => {
    return (
      showCreate.value ||
      showEdit.value ||
      showSync.value ||
      showImportData.value ||
      showExportDataDialog.value ||
      showBulkEdit.value ||
      showTempUnsched.value ||
      showDeleteDialog.value ||
      showResetDeviceProfileDialog.value ||
      showDeviceProfileInspect.value ||
      showReAuth.value ||
      showTest.value ||
      showStats.value ||
      showSchedulePanel.value ||
      showErrorPassthrough.value ||
      showTLSFingerprintProfiles.value ||
      showTLSFingerprintRouters.value ||
      showCapacityForecast.value
    )
  })

  return {
    showCreate,
    showEdit,
    showSync,
    showImportData,
    showExportDataDialog,
    includeProxyOnExport,
    showBulkEdit,
    bulkEditTarget,
    showTempUnsched,
    showDeleteDialog,
    showResetDeviceProfileDialog,
    showDeviceProfileInspect,
    resettingDeviceProfile,
    showCreateShadowDialog,
    showReAuth,
    showTest,
    showStats,
    showErrorPassthrough,
    showTLSFingerprintProfiles,
    showTLSFingerprintRouters,
    showCapacityForecast,
    showCyberEvents,
    showCyberErrorDetail,
    cyberEventsAcc,
    cyberErrorID,
    edAcc,
    tempUnschedAcc,
    deletingAcc,
    resettingDeviceAcc,
    inspectingDeviceAcc,
    creatingShadowAcc,
    reAuthAcc,
    testingAcc,
    statsAcc,
    showSchedulePanel,
    scheduleAcc,
    scheduleModelOptions,
    togglingSchedulable,
    menu,
    isAnyModalOpen
  }
}

export type AccountRowState = ReturnType<typeof useAccountRowState>
