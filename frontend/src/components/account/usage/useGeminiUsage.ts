import { computed, type ComputedRef, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountUsageInfo, GeminiCredentials, WindowStats } from '@/types'

/** Gemini platform tier/auth-type label + daily quota bar derived state. */
export function useGeminiUsage(
  account: ComputedRef<Account> | { value: Account },
  usageInfo: Ref<AccountUsageInfo | null> | ComputedRef<AccountUsageInfo | null>
) {
  const { t } = useI18n()

  const showGeminiTodayStats = computed(() => {
    return account.value.platform === 'gemini' && account.value.type === 'service_account'
  })

  const geminiUsageAvailable = computed(() => {
    return (
      !!usageInfo.value?.gemini_shared_daily ||
      !!usageInfo.value?.gemini_pro_daily ||
      !!usageInfo.value?.gemini_flash_daily ||
      !!usageInfo.value?.gemini_shared_minute ||
      !!usageInfo.value?.gemini_pro_minute ||
      !!usageInfo.value?.gemini_flash_minute
    )
  })

  // Gemini 账户类型（从 credentials 中提取）
  const geminiTier = computed(() => {
    if (account.value.platform !== 'gemini') return null
    const creds = account.value.credentials as GeminiCredentials | undefined
    return creds?.tier_id || null
  })

  const geminiOAuthType = computed(() => {
    if (account.value.platform !== 'gemini') return null
    const creds = account.value.credentials as GeminiCredentials | undefined
    return (creds?.oauth_type || '').trim() || null
  })

  // Gemini 是否为 Code Assist OAuth
  const isGeminiCodeAssist = computed(() => {
    if (account.value.platform !== 'gemini') return false
    const creds = account.value.credentials as GeminiCredentials | undefined
    return creds?.oauth_type === 'code_assist' || (!creds?.oauth_type && !!creds?.project_id)
  })

  const geminiChannelShort = computed((): 'ai studio' | 'gcp' | 'google one' | 'client' | null => {
    if (account.value.platform !== 'gemini') return null

    // API Key accounts are AI Studio.
    if (account.value.type === 'apikey') return 'ai studio'

    if (geminiOAuthType.value === 'google_one') return 'google one'
    if (isGeminiCodeAssist.value) return 'gcp'
    if (geminiOAuthType.value === 'ai_studio') return 'client'

    // Fallback (unknown legacy data): treat as AI Studio.
    return 'ai studio'
  })

  const geminiUserLevel = computed((): string | null => {
    if (account.value.platform !== 'gemini') return null

    const tier = (geminiTier.value || '').toString().trim()
    const tierLower = tier.toLowerCase()
    const tierUpper = tier.toUpperCase()

    // Google One: free / pro / ultra
    if (geminiOAuthType.value === 'google_one') {
      if (tierLower === 'google_one_free') return 'free'
      if (tierLower === 'google_ai_pro') return 'pro'
      if (tierLower === 'google_ai_ultra') return 'ultra'

      // Backward compatibility (legacy tier markers)
      if (tierUpper === 'AI_PREMIUM' || tierUpper === 'GOOGLE_ONE_STANDARD') return 'pro'
      if (tierUpper === 'GOOGLE_ONE_UNLIMITED') return 'ultra'
      if (tierUpper === 'FREE' || tierUpper === 'GOOGLE_ONE_BASIC' || tierUpper === 'GOOGLE_ONE_UNKNOWN' || tierUpper === '') return 'free'

      return null
    }

    // GCP Code Assist: standard / enterprise
    if (isGeminiCodeAssist.value) {
      if (tierLower === 'gcp_enterprise') return 'enterprise'
      if (tierLower === 'gcp_standard') return 'standard'

      // Backward compatibility
      if (tierUpper.includes('ULTRA') || tierUpper.includes('ENTERPRISE')) return 'enterprise'
      return 'standard'
    }

    // AI Studio (API Key) and Client OAuth: free / paid
    if (account.value.type === 'apikey' || geminiOAuthType.value === 'ai_studio') {
      if (tierLower === 'aistudio_paid') return 'paid'
      if (tierLower === 'aistudio_free') return 'free'

      // Backward compatibility
      if (tierUpper.includes('PAID') || tierUpper.includes('PAYG') || tierUpper.includes('PAY')) return 'paid'
      if (tierUpper.includes('FREE')) return 'free'
      if (account.value.type === 'apikey') return 'free'
      return null
    }

    return null
  })

  // Gemini 认证类型（按要求：授权方式简称 + 用户等级）
  const geminiAuthTypeLabel = computed(() => {
    if (account.value.platform !== 'gemini') return null
    if (!geminiChannelShort.value) return null
    return geminiUserLevel.value ? `${geminiChannelShort.value} ${geminiUserLevel.value}` : geminiChannelShort.value
  })

  // Gemini 账户类型徽章样式（统一样式）
  const geminiTierClass = computed(() => {
    // Use channel+level to choose a stable color without depending on raw tier_id variants.
    const channel = geminiChannelShort.value
    const level = geminiUserLevel.value

    if (channel === 'client' || channel === 'ai studio') {
      return 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
    }

    if (channel === 'google one') {
      if (level === 'ultra') return 'bg-accent-100 text-accent-600'
      if (level === 'pro') return 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
      return 'bg-surface-2 text-muted'
    }

    if (channel === 'gcp') {
      if (level === 'enterprise') return 'bg-accent-100 text-accent-600'
      return 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
    }

    return ''
  })

  // Gemini 配额政策信息
  const geminiQuotaPolicyChannel = computed(() => {
    if (geminiOAuthType.value === 'google_one') {
      return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.channel')
    }
    if (isGeminiCodeAssist.value) {
      return t('admin.accounts.gemini.quotaPolicy.rows.gcp.channel')
    }
    return t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.channel')
  })

  const geminiQuotaPolicyLimits = computed(() => {
    const tierLower = (geminiTier.value || '').toString().trim().toLowerCase()

    if (geminiOAuthType.value === 'google_one') {
      if (tierLower === 'google_ai_ultra' || geminiUserLevel.value === 'ultra') {
        return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsUltra')
      }
      if (tierLower === 'google_ai_pro' || geminiUserLevel.value === 'pro') {
        return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsPro')
      }
      return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsFree')
    }

    if (isGeminiCodeAssist.value) {
      if (tierLower === 'gcp_enterprise' || geminiUserLevel.value === 'enterprise') {
        return t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsEnterprise')
      }
      return t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsStandard')
    }

    // AI Studio (API Key / custom OAuth)
    if (tierLower === 'aistudio_paid' || geminiUserLevel.value === 'paid') {
      return t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsPaid')
    }
    return t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsFree')
  })

  const geminiQuotaPolicyDocsUrl = computed(() => {
    if (geminiOAuthType.value === 'google_one' || isGeminiCodeAssist.value) {
      return 'https://developers.google.com/gemini-code-assist/resources/quotas'
    }
    return 'https://ai.google.dev/pricing'
  })

  const geminiUsesSharedDaily = computed(() => {
    if (account.value.platform !== 'gemini') return false
    // Per requirement: Google One & GCP are shared RPD pools (no per-model breakdown).
    return (
      !!usageInfo.value?.gemini_shared_daily ||
      !!usageInfo.value?.gemini_shared_minute ||
      geminiOAuthType.value === 'google_one' ||
      isGeminiCodeAssist.value
    )
  })

  const geminiUsageBars = computed(() => {
    if (account.value.platform !== 'gemini') return []
    if (!usageInfo.value) return []

    const bars: Array<{
      key: string
      label: string
      utilization: number
      resetsAt: string | null
      windowStats?: WindowStats | null
      color: 'indigo' | 'emerald'
    }> = []

    if (geminiUsesSharedDaily.value) {
      const sharedDaily = usageInfo.value.gemini_shared_daily
      if (sharedDaily) {
        bars.push({
          key: 'shared_daily',
          label: '1d',
          utilization: sharedDaily.utilization,
          resetsAt: sharedDaily.resets_at,
          windowStats: sharedDaily.window_stats,
          color: 'indigo'
        })
      }
      return bars
    }

    const pro = usageInfo.value.gemini_pro_daily
    if (pro) {
      bars.push({
        key: 'pro_daily',
        label: 'pro',
        utilization: pro.utilization,
        resetsAt: pro.resets_at,
        windowStats: pro.window_stats,
        color: 'indigo'
        })
    }

    const flash = usageInfo.value.gemini_flash_daily
    if (flash) {
      bars.push({
        key: 'flash_daily',
        label: 'flash',
        utilization: flash.utilization,
        resetsAt: flash.resets_at,
        windowStats: flash.window_stats,
        color: 'emerald'
      })
    }

    return bars
  })

  return {
    showGeminiTodayStats,
    geminiUsageAvailable,
    geminiAuthTypeLabel,
    geminiTierClass,
    geminiQuotaPolicyChannel,
    geminiQuotaPolicyLimits,
    geminiQuotaPolicyDocsUrl,
    geminiUsageBars
  }
}
