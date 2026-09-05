import { computed, watch, type Ref } from 'vue'
import {
  cnSupportsNativeResponses,
  defaultCNAdaptiveBaseUrls,
  defaultCNBaseUrl,
  type CnAccountMode,
  type CnApiProtocol,
  type CnNativeApiProtocol
} from '@/components/account/credentialsBuilder'
import { buildModelMappingObject } from '@/composables/useModelWhitelist'
import type { AccountPlatform, AccountType } from '@/types'

interface ModelMapping {
  from: string
  to: string
}

export interface CreateAccountCnPlatformDeps {
  form: { platform: AccountPlatform; type: AccountType }
  accountCategory: Ref<'oauth-based' | 'apikey' | 'bedrock' | 'service_account'>
  accountMode: Ref<CnAccountMode>
  apiProtocol: Ref<CnApiProtocol>
  apiKeyBaseUrl: Ref<string>
  adaptiveBaseUrls: Ref<Record<CnNativeApiProtocol, string>>
  apiKeyValue: Ref<string>
  modelRestrictionMode: Ref<'whitelist' | 'mapping'>
  allowedModels: Ref<string[]>
  modelMappings: Ref<ModelMapping[]>
}

export function useCreateAccountCnPlatform(deps: CreateAccountCnPlatformDeps) {
  const {
    form,
    accountCategory,
    accountMode,
    apiProtocol,
    apiKeyBaseUrl,
    adaptiveBaseUrls,
    apiKeyValue,
    modelRestrictionMode,
    allowedModels,
    modelMappings
  } = deps

  const isCNPlatform = computed(
    () => form.platform === 'kimi' || form.platform === 'zhipu' || form.platform === 'deepseek'
  )
  // CnBaseUrlPresets 的 platform prop 是平台字面量联合类型，模板里不能写
  // `as` 断言（其中的 `|` 会被 eslint 误判为 Vue2 filter 语法），经此 computed 传递。
  const cnPresetPlatform = computed<'kimi' | 'zhipu' | 'deepseek'>(() => {
    if (form.platform === 'kimi' || form.platform === 'zhipu' || form.platform === 'deepseek') {
      return form.platform
    }
    return 'kimi'
  })
  // 当前平台可选的协议档（responses 仅 deepseek / kimi）。
  const cnProtocolOptions = computed<Array<{ value: CnApiProtocol; labelKey: string }>>(() => {
    const opts: Array<{ value: CnApiProtocol; labelKey: string }> = [
      { value: 'adaptive', labelKey: 'adaptive' },
      { value: 'chat_completions', labelKey: 'chatCompletions' },
      { value: 'anthropic', labelKey: 'anthropic' }
    ]
    if (cnSupportsNativeResponses(form.platform)) {
      opts.push({ value: 'responses', labelKey: 'responses' })
    }
    return opts
  })
  const cnAdaptiveProtocolOptions = computed<Array<{ value: CnNativeApiProtocol; labelKey: string }>>(() => {
    const opts: Array<{ value: CnNativeApiProtocol; labelKey: string }> = [
      { value: 'chat_completions', labelKey: 'chatCompletions' },
      { value: 'anthropic', labelKey: 'anthropic' }
    ]
    if (cnSupportsNativeResponses(form.platform)) opts.push({ value: 'responses', labelKey: 'responses' })
    return opts
  })

  function resetAdaptiveBaseUrls(platform: 'kimi' | 'zhipu' | 'deepseek', mode: CnAccountMode) {
    adaptiveBaseUrls.value = defaultCNAdaptiveBaseUrls(platform, mode)
  }
  // 当前选中平台的品牌色（选中卡片描边 / 图标底色），与 platformColors 取色一致。
  const cnAccentActiveClass = computed(() => {
    switch (form.platform) {
      case 'kimi':
        return 'border-accent-500 bg-accent-50'
      case 'zhipu':
        return 'border-accent-500 bg-accent-50'
      case 'deepseek':
        return 'border-success-500 bg-success-50'
      default:
        return 'border-accent bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]'
    }
  })
  const cnAccentIconClass = computed(() => {
    switch (form.platform) {
      case 'kimi':
        return 'bg-accent-500 text-white'
      case 'zhipu':
        return 'bg-accent-500 text-white'
      case 'deepseek':
        return 'bg-success-500 text-white'
      default:
        return 'bg-accent text-white'
    }
  })
  // 切换国产供应商平台：强制 apikey 类型，deepseek 无 coding 套餐故锁定 payg，
  // 协议回落 adaptive，并把 base url 重置为该平台默认端点。
  function selectCNPlatform(platform: 'kimi' | 'zhipu' | 'deepseek') {
    form.platform = platform
    form.type = 'apikey'
    accountCategory.value = 'apikey'
    apiProtocol.value = 'adaptive'
    if (platform === 'deepseek') {
      accountMode.value = 'payg'
    }
    apiKeyBaseUrl.value = defaultCNBaseUrl(platform, accountMode.value, apiProtocol.value)
    resetAdaptiveBaseUrls(platform, accountMode.value)
  }
  // 账号类型 / 协议变更时同步默认 base url。
  watch(accountMode, (mode, previousMode) => {
    if (!isCNPlatform.value) return
    if (apiProtocol.value === 'adaptive') {
      const previousDefaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, previousMode)
      const nextDefaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, mode)
      for (const item of cnAdaptiveProtocolOptions.value) {
        if (!adaptiveBaseUrls.value[item.value] || adaptiveBaseUrls.value[item.value] === previousDefaults[item.value]) {
          adaptiveBaseUrls.value[item.value] = nextDefaults[item.value]
        }
      }
      apiKeyBaseUrl.value = adaptiveBaseUrls.value.chat_completions
      return
    }
    apiKeyBaseUrl.value = defaultCNBaseUrl(form.platform, mode, apiProtocol.value)
  })
  watch(apiProtocol, (protocol) => {
    if (!isCNPlatform.value) return
    if (protocol === 'adaptive') {
      const defaults = defaultCNAdaptiveBaseUrls(cnPresetPlatform.value, accountMode.value)
      for (const item of cnAdaptiveProtocolOptions.value) {
        if (!adaptiveBaseUrls.value[item.value]) adaptiveBaseUrls.value[item.value] = defaults[item.value]
      }
      apiKeyBaseUrl.value = adaptiveBaseUrls.value.chat_completions
      return
    }
    apiKeyBaseUrl.value = defaultCNBaseUrl(form.platform, accountMode.value, protocol)
  })
  // 点击预设端点：同时回填 base url、账号类型与协议。
  function onCnPresetSelect(preset: { mode: CnAccountMode; protocol: CnApiProtocol; url: string }) {
    accountMode.value = preset.mode
    apiProtocol.value = preset.protocol
    apiKeyBaseUrl.value = preset.url
  }

  const syncPreviewCredentials = computed(() => {
    if (!apiKeyValue.value) return undefined
    const baseUrl = isCNPlatform.value && apiProtocol.value === 'adaptive'
      ? adaptiveBaseUrls.value.chat_completions.trim() || apiKeyBaseUrl.value.trim()
      : apiKeyBaseUrl.value.trim()
    const modelMapping = buildModelMappingObject(
      modelRestrictionMode.value,
      allowedModels.value,
      modelMappings.value
    )
    return {
      platform: form.platform,
      type: form.type,
      base_url: baseUrl || undefined,
      api_key: apiKeyValue.value,
      ...(modelMapping ? { model_mapping: modelMapping } : {})
    }
  })

  return {
    isCNPlatform,
    cnPresetPlatform,
    cnProtocolOptions,
    cnAdaptiveProtocolOptions,
    cnAccentActiveClass,
    cnAccentIconClass,
    selectCNPlatform,
    onCnPresetSelect,
    syncPreviewCredentials
  }
}
