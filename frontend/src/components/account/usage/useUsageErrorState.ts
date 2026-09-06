import { computed, type ComputedRef, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountUsageInfo } from '@/types'

/**
 * Shared derived state for the "forbidden / needs-reauth / degraded" account usage
 * banners. Used by the Antigravity, Grok and Kiro usage blocks, each of which passes
 * in its own `usageInfo` ref/computed (the same underlying data as the parent cell).
 */
export function useUsageErrorState(usageInfo: Ref<AccountUsageInfo | null> | ComputedRef<AccountUsageInfo | null>) {
  const { t } = useI18n()

  const isForbidden = computed(() => !!usageInfo.value?.is_forbidden)
  const forbiddenType = computed(() => usageInfo.value?.forbidden_type || 'forbidden')
  const validationURL = computed(() => usageInfo.value?.validation_url || '')

  // 需要重新授权（401）
  const needsReauth = computed(() => !!usageInfo.value?.needs_reauth)

  // 降级错误标签（rate_limited / network_error）
  const usageErrorLabel = computed(() => {
    const code = usageInfo.value?.error_code
    if (code === 'rate_limited') return t('admin.accounts.rateLimited')
    return t('admin.accounts.usageError')
  })

  const forbiddenLabel = computed(() => {
    switch (forbiddenType.value) {
      case 'validation':
        return t('admin.accounts.forbiddenValidation')
      case 'violation':
        return t('admin.accounts.forbiddenViolation')
      default:
        return t('admin.accounts.forbidden')
    }
  })

  const forbiddenBadgeClass = computed(() => {
    if (forbiddenType.value === 'validation') {
      return 'bg-warning-100 text-warning-700'
    }
    return 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text'
  })

  return {
    isForbidden,
    forbiddenType,
    validationURL,
    needsReauth,
    usageErrorLabel,
    forbiddenLabel,
    forbiddenBadgeClass
  }
}
