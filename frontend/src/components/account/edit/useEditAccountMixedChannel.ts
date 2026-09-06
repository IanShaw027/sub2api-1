import { computed, ref, type Ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { Account, CheckMixedChannelResponse } from '@/types'

export interface EditAccountMixedChannelDeps {
  props: { account: Account | null }
  form: { group_ids: number[] }
  appStore: { showError: (message: string) => void; showSuccess: (message: string) => void }
  t: (key: string, params?: Record<string, unknown>) => string
  emit: ((event: 'close') => void) & ((event: 'updated', account: Account) => void)
  submitting: Ref<boolean>
}

export function useEditAccountMixedChannel(deps: EditAccountMixedChannelDeps) {
  const { props, form, appStore, t, emit, submitting } = deps

  const showMixedChannelWarning = ref(false)
  const mixedChannelWarningDetails = ref<{ groupName: string; currentPlatform: string; otherPlatform: string } | null>(
    null
  )
  const mixedChannelWarningRawMessage = ref('')
  const mixedChannelWarningAction = ref<(() => Promise<void>) | null>(null)
  const antigravityMixedChannelConfirmed = ref(false)

  const needsMixedChannelCheck = () => props.account?.platform === 'antigravity' || props.account?.platform === 'anthropic'

  const buildMixedChannelDetails = (resp?: CheckMixedChannelResponse) => {
    const details = resp?.details
    if (!details) {
      return null
    }
    return {
      groupName: details.group_name || 'Unknown',
      currentPlatform: details.current_platform || 'Unknown',
      otherPlatform: details.other_platform || 'Unknown'
    }
  }

  const clearMixedChannelDialog = () => {
    showMixedChannelWarning.value = false
    mixedChannelWarningDetails.value = null
    mixedChannelWarningRawMessage.value = ''
    mixedChannelWarningAction.value = null
  }

  const openMixedChannelDialog = (opts: {
    response?: CheckMixedChannelResponse
    message?: string
    onConfirm: () => Promise<void>
  }) => {
    mixedChannelWarningDetails.value = buildMixedChannelDetails(opts.response)
    mixedChannelWarningRawMessage.value =
      opts.message || opts.response?.message || t('admin.accounts.failedToUpdate')
    mixedChannelWarningAction.value = opts.onConfirm
    showMixedChannelWarning.value = true
  }

  const withAntigravityConfirmFlag = (payload: Record<string, unknown>) => {
    if (needsMixedChannelCheck() && antigravityMixedChannelConfirmed.value) {
      return {
        ...payload,
        confirm_mixed_channel_risk: true
      }
    }
    const cloned = { ...payload }
    delete cloned.confirm_mixed_channel_risk
    return cloned
  }

  const ensureAntigravityMixedChannelConfirmed = async (onConfirm: () => Promise<void>): Promise<boolean> => {
    if (!needsMixedChannelCheck()) {
      return true
    }
    if (antigravityMixedChannelConfirmed.value) {
      return true
    }
    if (!props.account) {
      return false
    }

    try {
      const result = await adminAPI.accounts.checkMixedChannelRisk({
        platform: props.account.platform,
        group_ids: form.group_ids,
        account_id: props.account.id
      })
      if (!result.has_risk) {
        return true
      }
      openMixedChannelDialog({
        response: result,
        onConfirm: async () => {
          antigravityMixedChannelConfirmed.value = true
          await onConfirm()
        }
      })
      return false
    } catch (error: any) {
      appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
      return false
    }
  }

  const handleClose = () => {
    antigravityMixedChannelConfirmed.value = false
    clearMixedChannelDialog()
    emit('close')
  }

  const submitUpdateAccount = async (accountID: number, updatePayload: Record<string, unknown>) => {
    submitting.value = true
    try {
      const updatedAccount = await adminAPI.accounts.update(accountID, withAntigravityConfirmFlag(updatePayload))
      appStore.showSuccess(t('admin.accounts.accountUpdated'))
      emit('updated', updatedAccount)
      handleClose()
    } catch (error: any) {
      if (error.status === 409 && error.error === 'mixed_channel_warning' && needsMixedChannelCheck()) {
        openMixedChannelDialog({
          message: error.message,
          onConfirm: async () => {
            antigravityMixedChannelConfirmed.value = true
            await submitUpdateAccount(accountID, updatePayload)
          }
        })
        return
      }
      appStore.showError(error.message || t('admin.accounts.failedToUpdate'))
    } finally {
      submitting.value = false
    }
  }

  const handleMixedChannelConfirm = async () => {
    const action = mixedChannelWarningAction.value
    if (!action) {
      clearMixedChannelDialog()
      return
    }
    clearMixedChannelDialog()
    submitting.value = true
    try {
      await action()
    } finally {
      submitting.value = false
    }
  }

  const handleMixedChannelCancel = () => {
    clearMixedChannelDialog()
  }

  const mixedChannelWarningMessageText = computed(() => {
    if (mixedChannelWarningDetails.value) {
      return t('admin.accounts.mixedChannelWarning', mixedChannelWarningDetails.value)
    }
    return mixedChannelWarningRawMessage.value
  })

  return {
    showMixedChannelWarning,
    mixedChannelWarningDetails,
    mixedChannelWarningRawMessage,
    mixedChannelWarningAction,
    antigravityMixedChannelConfirmed,
    needsMixedChannelCheck,
    buildMixedChannelDetails,
    clearMixedChannelDialog,
    handleClose,
    openMixedChannelDialog,
    withAntigravityConfirmFlag,
    ensureAntigravityMixedChannelConfirmed,
    submitUpdateAccount,
    handleMixedChannelConfirm,
    handleMixedChannelCancel,
    mixedChannelWarningMessageText
  }
}
