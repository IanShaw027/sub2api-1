// Bulk (multi-row) account actions + the export-data flow for AccountsView.vue
// (frontend-health-cleanup 6.4 split): delete/reset-status/refresh-token/toggle-
// schedulable on the current selection, the bulk-edit dialog opener, and JSON export.
import { ref, type Ref } from 'vue'
import { adminAPI } from '@/api/admin'
import { useStepUp, isStepUpBlocked, isStepUpCancelled, stepUpBlockReason } from '@/composables/useStepUp'
import type { Account, AccountPlatform, AccountType } from '@/types'
import type { AccountBulkEditTarget } from './useAccountRowState'

interface UseAccountBulkActionsOptions {
  t: (key: string, params?: any) => string
  appStore: { showError: (message: string) => void; showSuccess: (message: string) => void; showWarning: (message: string) => void }
  accounts: Ref<Account[]>
  selIds: Ref<number[]>
  selPlatforms: Ref<AccountPlatform[]>
  selTypes: Ref<AccountType[]>
  setSelectedIds: (ids: number[]) => void
  clearSelection: () => void
  load: () => Promise<void>
  reload: () => Promise<void>
  buildBulkEditFilterSnapshot: () => Record<string, unknown>
  buildAccountQueryFilters: () => Record<string, unknown>
  showBulkEdit: Ref<boolean>
  bulkEditTarget: Ref<AccountBulkEditTarget | null>
  showImportData: Ref<boolean>
  includeProxyOnExport: Ref<boolean>
  showExportDataDialog: Ref<boolean>
}

export function useAccountBulkActions(options: UseAccountBulkActionsOptions) {
  const {
    t,
    appStore,
    accounts,
    selIds,
    selPlatforms,
    selTypes,
    setSelectedIds,
    clearSelection,
    load,
    reload,
    buildBulkEditFilterSnapshot,
    buildAccountQueryFilters,
    showBulkEdit,
    bulkEditTarget,
    showImportData,
    includeProxyOnExport,
    showExportDataDialog
  } = options

  const handleBulkDelete = async () => {
    const accountIds = [...selIds.value]
    if (!confirm(t('admin.accounts.bulkActions.confirmDelete', { count: accountIds.length }))) return
    try {
      const result = await adminAPI.accounts.batchDelete(accountIds)
      if (result.failed > 0) {
        appStore.showError(t('admin.accounts.bulkActions.partialSuccess', {
          success: result.success,
          failed: result.failed
        }))
        setSelectedIds(result.failed_ids?.length ? result.failed_ids : accountIds)
      } else {
        appStore.showSuccess(t('admin.accounts.bulkActions.deleteSuccess', { count: result.success }))
        clearSelection()
      }
      await reload()
    } catch (error) {
      console.error('Failed to bulk delete accounts:', error)
      appStore.showError(String(error))
    }
  }

  const handleBulkResetStatus = async () => {
    if (!confirm(t('common.confirm'))) return
    try {
      const result = await adminAPI.accounts.batchClearError(selIds.value)
      if (result.failed > 0) {
        appStore.showError(t('admin.accounts.bulkActions.partialSuccess', { success: result.success, failed: result.failed }))
      } else {
        appStore.showSuccess(t('admin.accounts.bulkActions.resetStatusSuccess', { count: result.success }))
        clearSelection()
      }
      reload()
    } catch (error) {
      console.error('Failed to bulk reset status:', error)
      appStore.showError(String(error))
    }
  }

  const handleBulkRefreshToken = async () => {
    if (!confirm(t('common.confirm'))) return
    try {
      const result = await adminAPI.accounts.batchRefresh(selIds.value)
      if (result.failed > 0) {
        appStore.showError(t('admin.accounts.bulkActions.partialSuccess', { success: result.success, failed: result.failed }))
      } else {
        appStore.showSuccess(t('admin.accounts.bulkActions.refreshTokenSuccess', { count: result.success }))
        clearSelection()
      }
      reload()
    } catch (error) {
      console.error('Failed to bulk refresh token:', error)
      appStore.showError(String(error))
    }
  }

  const updateSchedulableInList = (accountIds: number[], schedulable: boolean) => {
    if (accountIds.length === 0) return
    const idSet = new Set(accountIds)
    accounts.value = accounts.value.map((account) => (idSet.has(account.id) ? { ...account, schedulable } : account))
  }

  const normalizeBulkSchedulableResult = (
    result: {
      success?: number
      failed?: number
      success_ids?: number[]
      failed_ids?: number[]
      results?: Array<{ account_id: number; success: boolean }>
    },
    accountIds: number[]
  ) => {
    const responseSuccessIds = Array.isArray(result.success_ids) ? result.success_ids : []
    const responseFailedIds = Array.isArray(result.failed_ids) ? result.failed_ids : []
    if (responseSuccessIds.length > 0 || responseFailedIds.length > 0) {
      return {
        successIds: responseSuccessIds,
        failedIds: responseFailedIds,
        successCount: typeof result.success === 'number' ? result.success : responseSuccessIds.length,
        failedCount: typeof result.failed === 'number' ? result.failed : responseFailedIds.length,
        hasIds: true,
        hasCounts: true
      }
    }

    const results = Array.isArray(result.results) ? result.results : []
    if (results.length > 0) {
      const successIds = results.filter(item => item.success).map(item => item.account_id)
      const failedIds = results.filter(item => !item.success).map(item => item.account_id)
      return {
        successIds,
        failedIds,
        successCount: typeof result.success === 'number' ? result.success : successIds.length,
        failedCount: typeof result.failed === 'number' ? result.failed : failedIds.length,
        hasIds: true,
        hasCounts: true
      }
    }

    const hasExplicitCounts = typeof result.success === 'number' || typeof result.failed === 'number'
    const successCount = typeof result.success === 'number' ? result.success : 0
    const failedCount = typeof result.failed === 'number' ? result.failed : 0
    if (hasExplicitCounts && failedCount === 0 && successCount === accountIds.length && accountIds.length > 0) {
      return {
        successIds: accountIds,
        failedIds: [],
        successCount,
        failedCount,
        hasIds: true,
        hasCounts: true
      }
    }

    return {
      successIds: [],
      failedIds: [],
      successCount,
      failedCount,
      hasIds: false,
      hasCounts: hasExplicitCounts
    }
  }

  const handleBulkToggleSchedulable = async (schedulable: boolean) => {
    const accountIds = [...selIds.value]
    try {
      const result = await adminAPI.accounts.bulkUpdate(accountIds, { schedulable })
      const { successIds, failedIds, successCount, failedCount, hasIds, hasCounts } = normalizeBulkSchedulableResult(result, accountIds)
      if (!hasIds && !hasCounts) {
        appStore.showError(t('admin.accounts.bulkSchedulableResultUnknown'))
        setSelectedIds(accountIds)
        load().catch((error) => {
          console.error('Failed to refresh accounts:', error)
        })
        return
      }
      if (successIds.length > 0) {
        updateSchedulableInList(successIds, schedulable)
      }
      if (successCount > 0 && failedCount === 0) {
        const message = schedulable
          ? t('admin.accounts.bulkSchedulableEnabled', { count: successCount })
          : t('admin.accounts.bulkSchedulableDisabled', { count: successCount })
        appStore.showSuccess(message)
      }
      if (failedCount > 0) {
        const message = hasCounts || hasIds
          ? t('admin.accounts.bulkSchedulablePartial', { success: successCount, failed: failedCount })
          : t('admin.accounts.bulkSchedulableResultUnknown')
        appStore.showError(message)
        setSelectedIds(failedIds.length > 0 ? failedIds : accountIds)
      } else {
        if (hasIds) clearSelection()
        else setSelectedIds(accountIds)
      }
    } catch (error) {
      console.error('Failed to bulk toggle schedulable:', error)
      appStore.showError(t('common.error'))
    }
  }

  const collectSelectionMetadata = (rows: Account[]) => {
    const selectedPlatforms = Array.from(new Set(rows.map(account => account.platform)))
    const selectedTypes = Array.from(new Set(rows.map(account => account.type)))
    return { selectedPlatforms, selectedTypes }
  }

  const openBulkEditSelected = () => {
    bulkEditTarget.value = {
      mode: 'selected',
      accountIds: [...selIds.value],
      selectedPlatforms: [...selPlatforms.value],
      selectedTypes: [...selTypes.value]
    }
    showBulkEdit.value = true
  }

  const openBulkEditFiltered = async () => {
    const filters = buildBulkEditFilterSnapshot()
    const preview = await adminAPI.accounts.list(1, 100, filters)
    const { selectedPlatforms, selectedTypes } = collectSelectionMetadata(preview.items)
    bulkEditTarget.value = {
      mode: 'filtered',
      filters,
      previewCount: preview.total,
      selectedPlatforms,
      selectedTypes
    }
    showBulkEdit.value = true
  }

  const handleBulkUpdated = () => {
    showBulkEdit.value = false
    bulkEditTarget.value = null
    clearSelection()
    reload()
  }

  const handleDataImported = () => {
    showImportData.value = false
    reload()
  }

  const exportingData = ref(false)
  const accountExportStepUp = useStepUp()

  const formatExportTimestamp = () => {
    const now = new Date()
    const pad2 = (value: number) => String(value).padStart(2, '0')
    return `${now.getFullYear()}${pad2(now.getMonth() + 1)}${pad2(now.getDate())}${pad2(now.getHours())}${pad2(now.getMinutes())}${pad2(now.getSeconds())}`
  }

  const openExportDataDialog = () => {
    includeProxyOnExport.value = true
    showExportDataDialog.value = true
  }

  const handleExportData = async () => {
    if (exportingData.value) return
    exportingData.value = true
    try {
      const dataPayload = await accountExportStepUp.run(() => adminAPI.accounts.exportData(
        selIds.value.length > 0
          ? { ids: selIds.value, includeProxies: includeProxyOnExport.value }
          : {
              includeProxies: includeProxyOnExport.value,
              filters: buildAccountQueryFilters()
            }
      ))
      const timestamp = formatExportTimestamp()
      const filename = `sub2api-account-${timestamp}.json`
      const blob = new Blob([JSON.stringify(dataPayload, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = filename
      link.click()
      URL.revokeObjectURL(url)
      // spark 影子账号被后端排除出备份(其凭据透传母账号、调度配置不可经凭据型导入重建);
      // 跳过非零时明确提示用户,避免「下载成功但少了账号」的静默丢失。
      if (dataPayload.skipped_shadows && dataPayload.skipped_shadows > 0) {
        appStore.showWarning(t('admin.accounts.dataExportedSkippedShadows', { count: dataPayload.skipped_shadows }))
      } else {
        appStore.showSuccess(t('admin.accounts.dataExported'))
      }
    } catch (error: any) {
      if (isStepUpCancelled(error)) {
        // 用户主动取消 step-up 验证，静默返回，不弹错误提示。
      } else if (isStepUpBlocked(error)) {
        appStore.showError(
          stepUpBlockReason(error) === 'STEP_UP_ADMIN_API_KEY_FORBIDDEN'
            ? t('stepUp.adminApiKeyForbidden')
            : t('stepUp.notEnabled')
        )
      } else {
        appStore.showError(error?.message || t('admin.accounts.dataExportFailed'))
      }
    } finally {
      exportingData.value = false
      showExportDataDialog.value = false
    }
  }

  return {
    handleBulkDelete,
    handleBulkResetStatus,
    handleBulkRefreshToken,
    updateSchedulableInList,
    handleBulkToggleSchedulable,
    openBulkEditSelected,
    openBulkEditFiltered,
    handleBulkUpdated,
    handleDataImported,
    exportingData,
    accountExportStepUp,
    openExportDataDialog,
    handleExportData
  }
}

export type AccountBulkActionsState = ReturnType<typeof useAccountBulkActions>
