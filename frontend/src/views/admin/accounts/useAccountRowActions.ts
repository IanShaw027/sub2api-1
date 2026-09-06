// Single-row account actions (edit/test/re-auth/schedule/delete/etc.), the row
// action-menu positioning logic, and the "patch one row back into the list"
// primitive that most of those actions funnel through, for AccountsView.vue
// (frontend-health-cleanup 6.4 split).
import type { Ref } from 'vue'
import { adminAPI } from '@/api/admin'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { proxyExpiryBadgeClass, proxyExpiryLabelKey } from '@/utils/proxyExpiry'
import type { Account, ClaudeModel, Proxy as AccountProxy } from '@/types'
import type { SelectOption } from '@/components/common/Select.vue'
import { accountDisplayEmail } from '../accountRowHelpers'

interface AccountRowMenuState {
  show: boolean
  acc: Account | null
  pos: { top: number; left: number } | null
}

interface UseAccountRowActionsOptions {
  t: (key: string, params?: any) => string
  appStore: { showError: (message: string) => void; showSuccess: (message: string) => void }
  accounts: Ref<Account[]>
  pagination: { page: number; page_size: number; total: number; pages: number }
  hasPendingListSync: Ref<boolean>
  buildAccountQueryFilters: () => {
    platform: string
    type: string
    status: string
    group: string
    privacy_mode: string
    search: string
    sort_by: string
    sort_order: string
  }
  removeSelectedAccounts: (ids: number[]) => void
  reload: () => Promise<void>
  menu: AccountRowMenuState
  /** Relays a refreshed row into any dialog/menu state that's currently holding it. */
  syncAccountRefs: (nextAccount: Account) => void
  enterAutoRefreshSilentWindow: () => void
  updateSchedulableInList: (accountIds: number[], schedulable: boolean) => void
  // Dialog / row-target refs owned by useAccountRowState.
  showEdit: Ref<boolean>
  edAcc: Ref<Account | null>
  showTest: Ref<boolean>
  testingAcc: Ref<Account | null>
  showStats: Ref<boolean>
  statsAcc: Ref<Account | null>
  showReAuth: Ref<boolean>
  reAuthAcc: Ref<Account | null>
  showSchedulePanel: Ref<boolean>
  scheduleAcc: Ref<Account | null>
  scheduleModelOptions: Ref<SelectOption[]>
  showDeviceProfileInspect: Ref<boolean>
  inspectingDeviceAcc: Ref<Account | null>
  showResetDeviceProfileDialog: Ref<boolean>
  resettingDeviceAcc: Ref<Account | null>
  resettingDeviceProfile: Ref<boolean>
  showCreateShadowDialog: Ref<boolean>
  creatingShadowAcc: Ref<Account | null>
  showDeleteDialog: Ref<boolean>
  deletingAcc: Ref<Account | null>
  togglingSchedulable: Ref<number | null>
  showTempUnsched: Ref<boolean>
  tempUnschedAcc: Ref<Account | null>
}

export function useAccountRowActions(options: UseAccountRowActionsOptions) {
  const {
    t,
    appStore,
    accounts,
    pagination,
    hasPendingListSync,
    buildAccountQueryFilters,
    removeSelectedAccounts,
    reload,
    menu,
    syncAccountRefs,
    enterAutoRefreshSilentWindow,
    updateSchedulableInList,
    showEdit,
    edAcc,
    showTest,
    testingAcc,
    showStats,
    statsAcc,
    showReAuth,
    reAuthAcc,
    showSchedulePanel,
    scheduleAcc,
    scheduleModelOptions,
    showDeviceProfileInspect,
    inspectingDeviceAcc,
    showResetDeviceProfileDialog,
    resettingDeviceAcc,
    resettingDeviceProfile,
    showCreateShadowDialog,
    creatingShadowAcc,
    showDeleteDialog,
    deletingAcc,
    togglingSchedulable,
    showTempUnsched,
    tempUnschedAcc
  } = options

  const ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE = 'ungrouped'
  const ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE = '__unset__'

  const accountGroupLabel = (account: Account): string => {
    const groupList = account.groups ?? []
    if (!groupList.length) return t('admin.accounts.ungroupedGroup')
    const [first, ...rest] = groupList
    return rest.length ? `${first.name} +${rest.length}` : first.name
  }

  const accountIdentityTitle = (account: Account): string => {
    const parts = [`#${account.id}`, accountGroupLabel(account)]
    const email = accountDisplayEmail(account)
    if (email) parts.push(email)
    if (account.parent_chatgpt_account_id) parts.push(String(account.parent_chatgpt_account_id))
    return parts.join(' · ')
  }

  const handleEdit = (a: Account) => { edAcc.value = a; showEdit.value = true }

  const openMenu = (a: Account, e: MouseEvent) => {
    menu.acc = a

    const target = e.currentTarget as HTMLElement
    if (target) {
      const rect = target.getBoundingClientRect()
      const menuWidth = 200
      const menuHeight = 360
      const padding = 8
      const viewportWidth = window.innerWidth
      const viewportHeight = window.innerHeight

      let left: number
      let top: number

      if (viewportWidth < 768) {
        // 居中显示,水平位置
        left = Math.max(padding, Math.min(
          rect.left + rect.width / 2 - menuWidth / 2,
          viewportWidth - menuWidth - padding
        ))

        // 优先显示在按钮下方
        top = rect.bottom + 4

        // 如果下方空间不够,显示在上方
        if (top + menuHeight > viewportHeight - padding) {
          top = rect.top - menuHeight - 4
          // 如果上方也不够,就贴在视口顶部
          if (top < padding) {
            top = padding
          }
        }
      } else {
        left = Math.max(padding, Math.min(
          e.clientX - menuWidth,
          viewportWidth - menuWidth - padding
        ))
        top = e.clientY
        if (top + menuHeight > viewportHeight - padding) {
          top = viewportHeight - menuHeight - padding
        }
      }

      menu.pos = { top, left }
    } else {
      menu.pos = { top: e.clientY, left: e.clientX - 200 }
    }

    menu.show = true
  }

  const accountMatchesCurrentFilters = (account: Account) => {
    const filters = buildAccountQueryFilters()
    if (filters.platform && account.platform !== filters.platform) return false
    if (filters.type && account.type !== filters.type) return false
    if (filters.status) {
      const now = Date.now()
      const rateLimitResetAt = account.rate_limit_reset_at ? new Date(account.rate_limit_reset_at).getTime() : Number.NaN
      const isRateLimited = Number.isFinite(rateLimitResetAt) && rateLimitResetAt > now
      const tempUnschedUntil = account.temp_unschedulable_until ? new Date(account.temp_unschedulable_until).getTime() : Number.NaN
      const isTempUnschedulable = Number.isFinite(tempUnschedUntil) && tempUnschedUntil > now

      if (filters.status === 'active') {
        if (account.status !== 'active' || isRateLimited || isTempUnschedulable || !account.schedulable) return false
      } else if (filters.status === 'rate_limited') {
        if (account.status !== 'active' || !isRateLimited || isTempUnschedulable) return false
      } else if (filters.status === 'temp_unschedulable') {
        if (account.status !== 'active' || !isTempUnschedulable) return false
      } else if (filters.status === 'unschedulable') {
        if (account.status !== 'active' || account.schedulable || isRateLimited || isTempUnschedulable) return false
      } else if (account.status !== filters.status) {
        return false
      }
    }
    if (filters.group) {
      const groupIds = account.group_ids ?? account.groups?.map((group) => group.id) ?? []
      if (filters.group === ACCOUNT_UNGROUPED_GROUP_QUERY_VALUE) {
        if (groupIds.length > 0) return false
      } else if (!groupIds.includes(Number(filters.group))) {
        return false
      }
    }
    const privacyMode = typeof account.extra?.privacy_mode === 'string' ? account.extra.privacy_mode : ''
    if (filters.privacy_mode) {
      if (filters.privacy_mode === ACCOUNT_PRIVACY_MODE_UNSET_QUERY_VALUE) {
        if (privacyMode.trim() !== '') return false
      } else if (privacyMode !== filters.privacy_mode) {
        return false
      }
    }
    const search = String(filters.search || '').trim().toLowerCase()
    if (search && !account.name.toLowerCase().includes(search)) return false
    return true
  }

  const mergeRuntimeFields = (oldAccount: Account, updatedAccount: Account): Account => ({
    ...updatedAccount,
    current_concurrency: updatedAccount.current_concurrency ?? oldAccount.current_concurrency,
    current_window_cost: updatedAccount.current_window_cost ?? oldAccount.current_window_cost,
    active_sessions: updatedAccount.active_sessions ?? oldAccount.active_sessions
  })

  const syncPaginationAfterLocalRemoval = () => {
    const nextTotal = Math.max(0, pagination.total - 1)
    pagination.total = nextTotal
    pagination.pages = nextTotal > 0 ? Math.ceil(nextTotal / pagination.page_size) : 0

    const maxPage = Math.max(1, pagination.pages || 1)

    if (pagination.page > maxPage) {
      pagination.page = maxPage
    }
    // 行被本地移除后不立刻全量补页，改为提示用户手动同步。
    hasPendingListSync.value = nextTotal > 0
  }

  const patchAccountInList = (updatedAccount: Account) => {
    const index = accounts.value.findIndex(account => account.id === updatedAccount.id)
    if (index === -1) return
    const mergedAccount = mergeRuntimeFields(accounts.value[index], updatedAccount)
    if (!accountMatchesCurrentFilters(mergedAccount)) {
      accounts.value = accounts.value.filter(account => account.id !== mergedAccount.id)
      syncPaginationAfterLocalRemoval()
      removeSelectedAccounts([mergedAccount.id])
      if (menu.acc?.id === mergedAccount.id) {
        menu.show = false
        menu.acc = null
      }
      return
    }
    const nextAccounts = [...accounts.value]
    nextAccounts[index] = mergedAccount
    accounts.value = nextAccounts
    syncAccountRefs(mergedAccount)
  }

  const handleAccountUpdated = (updatedAccount: Account) => {
    patchAccountInList(updatedAccount)
    enterAutoRefreshSilentWindow()
  }

  const closeTestModal = () => { showTest.value = false; testingAcc.value = null }
  const closeStatsModal = () => { showStats.value = false; statsAcc.value = null }
  const closeReAuthModal = () => { showReAuth.value = false; reAuthAcc.value = null }
  const handleOpenEditorFromReAuth = (a: Account) => {
    closeReAuthModal()
    void handleEdit(a)
  }
  const handleTest = (a: Account) => { testingAcc.value = a; showTest.value = true }
  const handleViewStats = (a: Account) => { statsAcc.value = a; showStats.value = true }
  const handleSchedule = async (a: Account) => {
    scheduleAcc.value = a
    scheduleModelOptions.value = []
    showSchedulePanel.value = true
    try {
      const models = await adminAPI.accounts.getAvailableModels(a.id)
      scheduleModelOptions.value = models.map((m: ClaudeModel) => ({ value: m.id, label: m.display_name || m.id }))
    } catch {
      scheduleModelOptions.value = []
    }
  }
  const closeSchedulePanel = () => { showSchedulePanel.value = false; scheduleAcc.value = null; scheduleModelOptions.value = [] }
  const handleReAuth = (a: Account) => { reAuthAcc.value = a; showReAuth.value = true }
  const duplicatingAccountIDs = new Set<number>()
  const handleDuplicateAccount = async (a: Account) => {
    if (duplicatingAccountIDs.has(a.id)) return
    duplicatingAccountIDs.add(a.id)
    try {
      const duplicate = await adminAPI.accounts.duplicate(a.id)
      appStore.showSuccess(t('admin.accounts.duplicateSuccess', { name: duplicate.name }))
      reload()
    } catch (error: any) {
      console.error('Failed to duplicate account:', error)
      appStore.showError(error?.message || t('admin.accounts.duplicateFailed'))
    } finally {
      duplicatingAccountIDs.delete(a.id)
    }
  }
  const handleRefresh = async (a: Account) => {
    try {
      const updated = await adminAPI.accounts.refreshCredentials(a.id)
      patchAccountInList(updated)
      enterAutoRefreshSilentWindow()
    } catch (error) {
      console.error('Failed to refresh credentials:', error)
    }
  }
  const handleRecoverState = async (a: Account) => {
    try {
      const updated = await adminAPI.accounts.recoverState(a.id)
      patchAccountInList(updated)
      enterAutoRefreshSilentWindow()
      appStore.showSuccess(t('admin.accounts.recoverStateSuccess'))
    } catch (error: any) {
      console.error('Failed to recover account state:', error)
      appStore.showError(error?.message || t('admin.accounts.recoverStateFailed'))
    }
  }
  const handleResetQuota = async (a: Account) => {
    try {
      const updated = await adminAPI.accounts.resetAccountQuota(a.id)
      patchAccountInList(updated)
      enterAutoRefreshSilentWindow()
      appStore.showSuccess(t('common.success'))
    } catch (error) {
      console.error('Failed to reset quota:', error)
    }
  }
  const handleInspectDeviceProfile = (a: Account) => {
    inspectingDeviceAcc.value = a
    showDeviceProfileInspect.value = true
  }
  const closeDeviceProfileInspect = () => {
    showDeviceProfileInspect.value = false
    inspectingDeviceAcc.value = null
  }
  const handleResetDeviceProfile = (a: Account) => {
    resettingDeviceAcc.value = a
    showResetDeviceProfileDialog.value = true
  }
  const confirmResetDeviceProfile = async () => {
    const a = resettingDeviceAcc.value
    if (!a || resettingDeviceProfile.value) return
    resettingDeviceProfile.value = true
    try {
      const updated = await adminAPI.accounts.resetDeviceProfile(a.id)
      patchAccountInList(updated)
      enterAutoRefreshSilentWindow()
      showResetDeviceProfileDialog.value = false
      resettingDeviceAcc.value = null
      appStore.showSuccess(t('admin.accounts.resetDeviceProfileSuccess'))
    } catch (error: unknown) {
      console.error('Failed to reset device profile:', error)
      appStore.showError(extractApiErrorMessage(error, t('admin.accounts.resetDeviceProfileFailed')))
    } finally {
      resettingDeviceProfile.value = false
    }
  }

  const privacyResultMessageKey = (account: Account): { type: 'success' | 'error'; key: string } => {
    const mode = typeof account.extra?.privacy_mode === 'string' ? account.extra.privacy_mode : ''
    if (account.platform === 'openai') {
      switch (mode) {
        case 'training_off':
          return { type: 'success', key: 'admin.accounts.privacyTrainingOff' }
        case 'training_set_cf_blocked':
          return { type: 'error', key: 'admin.accounts.privacyCfBlocked' }
        default:
          return { type: 'error', key: 'admin.accounts.privacyFailed' }
      }
    }
    if (account.platform === 'antigravity') {
      if (mode === 'privacy_set') {
        return { type: 'success', key: 'admin.accounts.privacyAntigravitySet' }
      }
      return { type: 'error', key: 'admin.accounts.privacyAntigravityFailed' }
    }
    return { type: 'error', key: 'admin.accounts.privacyFailed' }
  }

  const handleSetPrivacy = async (a: Account) => {
    try {
      const updated = await adminAPI.accounts.setPrivacy(a.id)
      patchAccountInList(updated)
      enterAutoRefreshSilentWindow()
      const result = privacyResultMessageKey(updated)
      if (result.type === 'success') {
        appStore.showSuccess(t(result.key))
      } else {
        appStore.showError(t(result.key))
      }
    } catch (error: any) {
      console.error('Failed to set privacy:', error)
      appStore.showError(error?.response?.data?.message || t('admin.accounts.privacyFailed'))
    }
  }
  const onRevertFallback = async (a: Account) => {
    try {
      await adminAPI.accounts.revertProxyFallback(a.id)
      appStore.showSuccess(t('admin.accounts.revertProxySuccess'))
      reload()
    } catch (error: any) {
      console.error('Failed to revert proxy fallback:', error)
      appStore.showError(error?.response?.data?.message || t('admin.accounts.revertProxyFailed'))
    }
  }
  const handleCreateSparkShadow = (a: Account) => {
    creatingShadowAcc.value = a
    showCreateShadowDialog.value = true
  }
  const confirmCreateSparkShadow = async () => {
    const a = creatingShadowAcc.value
    if (!a) return
    try {
      await adminAPI.accounts.createSparkShadow(a.id, { name: `${a.name} (Spark)` })
      showCreateShadowDialog.value = false
      creatingShadowAcc.value = null
      appStore.showSuccess(t('admin.accounts.createSparkShadowSuccess'))
      reload()
    } catch (error: any) {
      console.error('Failed to create spark shadow:', error)
      appStore.showError(error?.response?.data?.message || t('admin.accounts.createSparkShadowFailed'))
    }
  }
  const handleDelete = (a: Account) => { deletingAcc.value = a; showDeleteDialog.value = true }
  const confirmDelete = async () => { if (!deletingAcc.value) return; try { await adminAPI.accounts.delete(deletingAcc.value.id); showDeleteDialog.value = false; deletingAcc.value = null; reload() } catch (error) { console.error('Failed to delete account:', error) } }
  const handleToggleSchedulable = async (a: Account) => {
    const nextSchedulable = !a.schedulable
    togglingSchedulable.value = a.id
    try {
      const updated = await adminAPI.accounts.setSchedulable(a.id, nextSchedulable)
      updateSchedulableInList([a.id], updated?.schedulable ?? nextSchedulable)
      enterAutoRefreshSilentWindow()
    } catch (error) {
      console.error('Failed to toggle schedulable:', error)
      appStore.showError(t('admin.accounts.failedToToggleSchedulable'))
    } finally {
      togglingSchedulable.value = null
    }
  }
  const handleShowTempUnsched = (a: Account) => { tempUnschedAcc.value = a; showTempUnsched.value = true }
  const handleTempUnschedReset = async (updated: Account) => {
    showTempUnsched.value = false
    tempUnschedAcc.value = null
    patchAccountInList(updated)
    enterAutoRefreshSilentWindow()
  }
  const formatExpiresAt = (value: number | null) => {
    if (!value) return '-'
    return formatDateTime(
      new Date(value * 1000),
      {
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        hour12: false
      },
      'sv-SE'
    )
  }
  const isExpired = (value: number | null) => {
    if (!value) return false
    return value * 1000 <= Date.now()
  }
  // 所绑定代理的有效期(逻辑同 /admin/proxies,见 utils/proxyExpiry)
  const proxyExpiryBadge = (p: AccountProxy): string => proxyExpiryBadgeClass(p.expires_at, p.status)
  const proxyExpiryText = (p: AccountProxy): string => {
    const { key, params } = proxyExpiryLabelKey(p.expires_at, p.status)
    return params ? t(key, params) : t(key)
  }

  return {
    accountGroupLabel,
    accountIdentityTitle,
    handleEdit,
    openMenu,
    accountMatchesCurrentFilters,
    patchAccountInList,
    handleAccountUpdated,
    closeTestModal,
    closeStatsModal,
    closeReAuthModal,
    handleOpenEditorFromReAuth,
    handleTest,
    handleViewStats,
    handleSchedule,
    closeSchedulePanel,
    handleReAuth,
    handleDuplicateAccount,
    handleRefresh,
    handleRecoverState,
    handleResetQuota,
    handleInspectDeviceProfile,
    closeDeviceProfileInspect,
    handleResetDeviceProfile,
    confirmResetDeviceProfile,
    handleSetPrivacy,
    onRevertFallback,
    handleCreateSparkShadow,
    confirmCreateSparkShadow,
    handleDelete,
    confirmDelete,
    handleToggleSchedulable,
    handleShowTempUnsched,
    handleTempUnschedReset,
    formatExpiresAt,
    isExpired,
    proxyExpiryBadge,
    proxyExpiryText
  }
}

export type AccountRowActionsState = ReturnType<typeof useAccountRowActions>
