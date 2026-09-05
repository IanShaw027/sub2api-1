import { computed, ref, type Ref } from 'vue'
import { adminAPI } from '@/api/admin'
import type { AccountPlatform, CheckMixedChannelResponse, CreateAccountRequest } from '@/types'

export interface CreateAccountMixedChannelDeps {
  form: { platform: AccountPlatform; group_ids: number[] }
  appStore: { showError: (message: string) => void; showWarning: (message: string) => void; showSuccess: (message: string) => void }
  t: (key: string, params?: Record<string, unknown>) => string
  emit: ((event: 'created') => void) & ((event: 'close') => void)
  submitting: Ref<boolean>
  upstreamModelsPreviewed: Ref<boolean>
  kiroOAuth: { cancelDeviceAuthorization: () => void }
  supportsTLSFingerprint: (platform?: string | null) => boolean
  applyTLSFingerprintToExtra: (extra: Record<string, unknown>) => void
  mixedScheduling: Ref<boolean>
  allowOverages: Ref<boolean>
}

export function useCreateAccountMixedChannel(deps: CreateAccountMixedChannelDeps) {
  const {
    form,
    appStore,
    t,
    emit,
    submitting,
    upstreamModelsPreviewed,
    kiroOAuth,
    supportsTLSFingerprint,
    applyTLSFingerprintToExtra,
    mixedScheduling,
    allowOverages
  } = deps

  const showMixedChannelWarning = ref(false)
  const mixedChannelWarningDetails = ref<{ groupName: string; currentPlatform: string; otherPlatform: string } | null>(
    null
  )
  const mixedChannelWarningRawMessage = ref('')
  const mixedChannelWarningAction = ref<(() => Promise<void>) | null>(null)
  const antigravityMixedChannelConfirmed = ref(false)

  const needsMixedChannelCheck = (platform: AccountPlatform) => platform === 'antigravity' || platform === 'anthropic'

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

  const handleClose = () => {
    kiroOAuth.cancelDeviceAuthorization()
    antigravityMixedChannelConfirmed.value = false
    clearMixedChannelDialog()
    emit('close')
  }

  const openMixedChannelDialog = (opts: {
    response?: CheckMixedChannelResponse
    message?: string
    onConfirm: () => Promise<void>
  }) => {
    mixedChannelWarningDetails.value = buildMixedChannelDetails(opts.response)
    mixedChannelWarningRawMessage.value =
      opts.message || opts.response?.message || t('admin.accounts.failedToCreate')
    mixedChannelWarningAction.value = opts.onConfirm
    showMixedChannelWarning.value = true
  }

  const withAntigravityConfirmFlag = (payload: CreateAccountRequest): CreateAccountRequest => {
    if (needsMixedChannelCheck(payload.platform) && antigravityMixedChannelConfirmed.value) {
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
    if (!needsMixedChannelCheck(form.platform)) {
      return true
    }
    if (antigravityMixedChannelConfirmed.value) {
      return true
    }

    try {
      const result = await adminAPI.accounts.checkMixedChannelRisk({
        platform: form.platform,
        group_ids: form.group_ids
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
      appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.accounts.failedToCreate'))
      return false
    }
  }

  const submitCreateAccount = async (payload: CreateAccountRequest) => {
    submitting.value = true
    try {
      const account = await adminAPI.accounts.create(withAntigravityConfirmFlag(payload))
      const modelMapping = payload.credentials.model_mapping
      const hasConcreteMappedTarget = payload.type === 'apikey' &&
        typeof modelMapping === 'object' &&
        modelMapping !== null &&
        Object.values(modelMapping).some((target) =>
          typeof target === 'string' && target.trim() !== '' && !target.includes('*')
        )
      if (upstreamModelsPreviewed.value || hasConcreteMappedTarget) {
        try {
          const result = await adminAPI.accounts.syncUpstreamModels(account.id)
          if (result.warnings?.some(warning => warning.code === 'upstream_model_metadata_incomplete')) {
            appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataIncomplete'))
          }
        } catch {
          appStore.showWarning(t('admin.accounts.syncUpstreamModelsFailed'))
        }
      }
      if (
        payload.type === 'apikey' &&
        payload.upstream_billing_probe_enabled === true
      ) {
        try {
          await adminAPI.accounts.probeUpstreamBilling(account.id)
        } catch {
          appStore.showWarning(t('admin.accounts.upstreamBilling.probeFailed'))
        }
      }
      appStore.showSuccess(t('admin.accounts.accountCreated'))
      emit('created')
      handleClose()
    } catch (error: any) {
      if (error.response?.status === 409 && error.response?.data?.error === 'mixed_channel_warning' && needsMixedChannelCheck(form.platform)) {
        openMixedChannelDialog({
          message: error.response?.data?.message,
          onConfirm: async () => {
            antigravityMixedChannelConfirmed.value = true
            await submitCreateAccount(payload)
          }
        })
        return
      }
      appStore.showError(error.response?.data?.message || error.response?.data?.detail || t('admin.accounts.failedToCreate'))
    } finally {
      submitting.value = false
    }
  }

  function buildAntigravityExtra(): Record<string, unknown> | undefined {
    const extra: Record<string, unknown> = {}
    if (mixedScheduling.value) extra.mixed_scheduling = true
    if (allowOverages.value) extra.allow_overages = true
    return Object.keys(extra).length > 0 ? extra : undefined
  }

  // Helper function to create account with mixed channel warning handling
  const doCreateAccount = async (payload: CreateAccountRequest) => {
    if (supportsTLSFingerprint(payload.platform)) {
      const extra: Record<string, unknown> = { ...(payload.extra || {}) }
      applyTLSFingerprintToExtra(extra)
      payload.extra = extra
    }
    const canContinue = await ensureAntigravityMixedChannelConfirmed(async () => {
      await submitCreateAccount(payload)
    })
    if (!canContinue) {
      return
    }
    await submitCreateAccount(payload)
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
    submitCreateAccount,
    buildAntigravityExtra,
    doCreateAccount,
    handleMixedChannelConfirm,
    handleMixedChannelCancel,
    mixedChannelWarningMessageText
  }
}
