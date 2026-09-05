import { computed, type Ref } from 'vue'
import type { AccountPlatform } from '@/types'
import { defaultCNBaseUrl, type CnAccountMode, type CnApiProtocol } from '@/components/account/credentialsBuilder'

// Static Gemini help links, used by GeminiHelpDialog / PlatformSelector via
// CreateAccountModal.vue. Moved here (rather than a new file) since it lives
// alongside the other Gemini/platform hint constants extracted in task 16A.
export const geminiHelpLinks = {
  apiKey: 'https://aistudio.google.com/app/apikey',
  aiStudioPricing: 'https://ai.google.dev/pricing',
  gcpProject: 'https://console.cloud.google.com/welcome/new',
  geminiWebActivation: 'https://gemini.google.com/gems/create?hl=en-US&pli=1',
  countryCheck: 'https://policies.google.com/terms',
  countryChange: 'https://policies.google.com/country-association-form'
}

export interface CreateAccountPlatformHintsDeps {
  form: { platform: AccountPlatform }
  accountMode: Ref<CnAccountMode>
  apiProtocol: Ref<CnApiProtocol>
  isCNPlatform: Ref<boolean>
  t: (key: string, params?: Record<string, unknown>) => string
  accountCategory: Ref<'oauth-based' | 'apikey' | 'bedrock' | 'service_account'>
  geminiOAuthType: Ref<'code_assist' | 'google_one' | 'ai_studio'>
  geminiTierGoogleOne: Ref<'google_one_free' | 'google_ai_pro' | 'google_ai_ultra'>
  geminiTierGcp: Ref<'gcp_standard' | 'gcp_enterprise'>
  geminiTierAIStudio: Ref<'aistudio_free' | 'aistudio_paid'>
}

export function useCreateAccountPlatformHints(deps: CreateAccountPlatformHintsDeps) {
  const {
    form,
    accountMode,
    apiProtocol,
    isCNPlatform,
    t,
    accountCategory,
    geminiOAuthType,
    geminiTierGoogleOne,
    geminiTierGcp,
    geminiTierAIStudio
  } = deps

  // Gemini tier selection (used as fallback when auto-detection is unavailable/fails)
  const geminiSelectedTier = computed(() => {
    if (form.platform !== 'gemini') return ''
    if (accountCategory.value === 'apikey') return geminiTierAIStudio.value
    switch (geminiOAuthType.value) {
      case 'google_one':
        return geminiTierGoogleOne.value
      case 'code_assist':
        return geminiTierGcp.value
      default:
        return geminiTierAIStudio.value
    }
  })

  const oauthStepTitle = computed(() => {
    if (form.platform === 'openai') return t('admin.accounts.oauth.openai.title')
    if (form.platform === 'gemini') return t('admin.accounts.oauth.gemini.title')
    if (form.platform === 'antigravity') return t('admin.accounts.oauth.antigravity.title')
    if (form.platform === 'grok') return t('admin.accounts.oauth.grok.title')
    if (form.platform === 'kiro') return t('admin.accounts.kiro.authorizationTitle')
    return t('admin.accounts.oauth.title')
  })

  // Platform-specific hints for API Key type
  const baseUrlHint = computed(() => {
    if (form.platform === 'openai') return t('admin.accounts.openai.baseUrlHint')
    if (form.platform === 'gemini') return t('admin.accounts.gemini.baseUrlHint')
    if (form.platform === 'grok') return ''
    return t('admin.accounts.baseUrlHint')
  })

  const apiKeyHint = computed(() => {
    if (form.platform === 'openai') return t('admin.accounts.openai.apiKeyHint')
    if (form.platform === 'gemini') return t('admin.accounts.gemini.apiKeyHint')
    if (form.platform === 'grok') return ''
    return t('admin.accounts.apiKeyHint')
  })

  // Base URL / API Key 占位符：国产供应商随账号类型变化。
  const apiKeyBaseUrlPlaceholder = computed(() => {
    if (isCNPlatform.value) {
      return defaultCNBaseUrl(form.platform, accountMode.value, apiProtocol.value) || 'https://api.example.com'
    }
    switch (form.platform) {
      case 'openai':
        return 'https://api.openai.com'
      case 'gemini':
        return 'https://generativelanguage.googleapis.com'
      case 'grok':
        return 'https://api.x.ai/v1'
      default:
        return 'https://api.anthropic.com'
    }
  })

  const apiKeyValuePlaceholder = computed(() => {
    switch (form.platform) {
      case 'openai':
        return 'sk-proj-...'
      case 'gemini':
        return 'AIza...'
      case 'grok':
        return 'xai-...'
      case 'kimi':
        return 'sk-...'
      case 'zhipu':
        return '<api-key>.<secret>'
      case 'deepseek':
        return 'sk-...'
      default:
        return 'sk-ant-...'
    }
  })

  return {
    oauthStepTitle,
    baseUrlHint,
    apiKeyHint,
    apiKeyBaseUrlPlaceholder,
    apiKeyValuePlaceholder,
    geminiSelectedTier
  }
}
