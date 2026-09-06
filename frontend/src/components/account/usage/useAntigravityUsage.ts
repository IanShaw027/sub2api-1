import { computed, type ComputedRef, type Ref } from 'vue'
import type { Account, AccountUsageInfo } from '@/types'

interface AntigravityUsageResult {
  utilization: number
  resetTime: string | null
}

/** Antigravity OAuth account quota + tier derived state (from load_code_assist extra + antigravity_quota). */
export function useAntigravityUsage(
  account: ComputedRef<Account> | { value: Account },
  usageInfo: Ref<AccountUsageInfo | null> | ComputedRef<AccountUsageInfo | null>
) {
  // ===== Antigravity quota from API (usageInfo.antigravity_quota) =====

  // 检查是否有从 API 获取的配额数据
  const hasAntigravityQuotaFromAPI = computed(() => {
    return usageInfo.value?.antigravity_quota && Object.keys(usageInfo.value.antigravity_quota).length > 0
  })

  // 从 API 配额数据中获取使用率（多模型取最高使用率）
  const getAntigravityUsageFromAPI = (
    modelNames: string[]
  ): AntigravityUsageResult | null => {
    const quota = usageInfo.value?.antigravity_quota
    if (!quota) return null

    let maxUtilization = 0
    let earliestReset: string | null = null

    for (const model of modelNames) {
      const modelQuota = quota[model]
      if (!modelQuota) continue

      if (modelQuota.utilization > maxUtilization) {
        maxUtilization = modelQuota.utilization
      }
      if (modelQuota.reset_time) {
        if (!earliestReset || modelQuota.reset_time < earliestReset) {
          earliestReset = modelQuota.reset_time
        }
      }
    }

    // 如果没有找到任何匹配的模型
    if (maxUtilization === 0 && earliestReset === null) {
      const hasAnyData = modelNames.some((m) => quota[m])
      if (!hasAnyData) return null
    }

    return {
      utilization: maxUtilization,
      resetTime: earliestReset
    }
  }

  // Gemini 3 Pro from API
  const antigravity3ProUsageFromAPI = computed(() =>
    getAntigravityUsageFromAPI(['gemini-3-pro-low', 'gemini-3-pro-high', 'gemini-3-pro-preview'])
  )

  // Gemini 3 Flash from API
  const antigravity3FlashUsageFromAPI = computed(() => getAntigravityUsageFromAPI(['gemini-3-flash']))

  // Gemini Image from API
  const antigravity3ImageUsageFromAPI = computed(() =>
    getAntigravityUsageFromAPI(['gemini-2.5-flash-image', 'gemini-3.1-flash-image', 'gemini-3-pro-image'])
  )

  // Claude from API (all Claude model variants)
  const antigravityClaudeUsageFromAPI = computed(() =>
    getAntigravityUsageFromAPI([
      'claude-fable-5-1',
      'claude-fable-5',
      'claude-sonnet-4-5', 'claude-opus-4-5-thinking',
      'claude-sonnet-4-6', 'claude-opus-4-6', 'claude-opus-4-6-thinking',
      'claude-opus-4-7', 'claude-opus-4-8',
    ])
  )

  const aiCreditsDisplay = computed(() => {
    const credits = usageInfo.value?.ai_credits
    if (!credits || credits.length === 0) return null
    const total = credits.reduce((sum, credit) => sum + (credit.amount ?? 0), 0)
    if (total <= 0) return null
    return total.toFixed(0)
  })

  // Antigravity 账户类型（从 load_code_assist 响应中提取）
  const antigravityTier = computed(() => {
    const extra = account.value.extra as Record<string, unknown> | undefined
    if (!extra) return null

    const loadCodeAssist = extra.load_code_assist as Record<string, unknown> | undefined
    if (!loadCodeAssist) return null

    // 优先取 paidTier，否则取 currentTier
    const paidTier = loadCodeAssist.paidTier as Record<string, unknown> | undefined
    if (paidTier && typeof paidTier.id === 'string') {
      return paidTier.id
    }

    const currentTier = loadCodeAssist.currentTier as Record<string, unknown> | undefined
    if (currentTier && typeof currentTier.id === 'string') {
      return currentTier.id
    }

    return null
  })

  // 检测账户是否有不合格状态（ineligibleTiers）
  const hasIneligibleTiers = computed(() => {
    const extra = account.value.extra as Record<string, unknown> | undefined
    if (!extra) return false

    const loadCodeAssist = extra.load_code_assist as Record<string, unknown> | undefined
    if (!loadCodeAssist) return false

    const ineligibleTiers = loadCodeAssist.ineligibleTiers as unknown[] | undefined
    return Array.isArray(ineligibleTiers) && ineligibleTiers.length > 0
  })

  return {
    hasAntigravityQuotaFromAPI,
    antigravity3ProUsageFromAPI,
    antigravity3FlashUsageFromAPI,
    antigravity3ImageUsageFromAPI,
    antigravityClaudeUsageFromAPI,
    aiCreditsDisplay,
    antigravityTier,
    hasIneligibleTiers
  }
}
