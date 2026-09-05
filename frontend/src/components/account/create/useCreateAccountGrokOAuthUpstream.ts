import { ref, type Ref } from 'vue'
import { applyHeaderOverride, validateHeaderOverrideRows, type HeaderOverrideRow } from '@/components/account/credentialsBuilder'

export interface CreateAccountGrokOAuthUpstreamDeps {
  appStore: { showError: (message: string) => void }
  t: (key: string, params?: Record<string, unknown>) => string
  headerOverrideEnabled: Ref<boolean>
  headerOverrideRows: Ref<HeaderOverrideRow[]>
}

export function useCreateAccountGrokOAuthUpstream(deps: CreateAccountGrokOAuthUpstreamDeps) {
  const { appStore, t, headerOverrideEnabled, headerOverrideRows } = deps

  // Grok OAuth：自定义上游地址（base_url 仅改写转发端点，OAuth 授权/刷新不受影响）
  const grokOAuthCustomBaseUrlEnabled = ref(false)
  const grokOAuthBaseUrl = ref('')

  // Grok OAuth 三条创建路径（授权码/RT 批量/SSO 批量）共用的前置校验。
  // 授权码路径必须在兑换 code 之前调用，避免校验失败时白白消耗一次性授权码。
  const validateGrokOAuthUpstreamConfig = (): boolean => {
    if (grokOAuthCustomBaseUrlEnabled.value) {
      const trimmed = grokOAuthBaseUrl.value.trim()
      if (!trimmed) {
        appStore.showError(t('admin.accounts.grokCustomBaseUrl.required'))
        return false
      }
      if (!/^https?:\/\//i.test(trimmed)) {
        appStore.showError(t('admin.accounts.grokCustomBaseUrl.invalid'))
        return false
      }
    }
    if (headerOverrideEnabled.value) {
      const headerError = validateHeaderOverrideRows(headerOverrideRows.value)
      if (headerError) {
        appStore.showError(t(`admin.accounts.headerOverride.${headerError}`))
        return false
      }
    }
    return true
  }

  // 把已通过校验的自定义上游地址与请求头覆写写入 credentials
  const applyGrokOAuthUpstreamConfig = (credentials: Record<string, unknown>) => {
    if (grokOAuthCustomBaseUrlEnabled.value) {
      credentials.base_url = grokOAuthBaseUrl.value.trim()
    }
    applyHeaderOverride(credentials, headerOverrideEnabled.value, headerOverrideRows.value, 'create')
  }

  return {
    grokOAuthCustomBaseUrlEnabled,
    grokOAuthBaseUrl,
    validateGrokOAuthUpstreamConfig,
    applyGrokOAuthUpstreamConfig
  }
}
