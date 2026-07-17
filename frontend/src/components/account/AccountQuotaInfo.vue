<template>
  <div v-if="shouldShowQuota" class="space-y-1">
    <div class="flex items-center gap-1">
      <span :class="['badge rounded-control px-1.5 py-0.5 text-[10px] font-medium', tierBadgeClass]">
        {{ tierLabel }}
      </span>
      <span class="group relative cursor-help">
        <svg
          class="h-3.5 w-3.5 text-ink-faint hover:text-ink-soft"
          fill="currentColor"
          viewBox="0 0 20 20"
        >
          <path
            fill-rule="evenodd"
            d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-3a1 1 0 00-.867.5 1 1 0 11-1.731-1A3 3 0 0113 8a3.001 3.001 0 01-2 2.83V11a1 1 0 11-2 0v-1a1 1 0 011-1 1 1 0 100-2zm0 8a1 1 0 100-2 1 1 0 000 2z"
            clip-rule="evenodd"
          />
        </svg>
        <span
          class="pointer-events-none absolute left-0 top-full z-50 mt-1 w-80 whitespace-normal break-words rounded-control bg-ink dark:bg-dark-700 px-3 py-2 text-xs leading-relaxed text-white opacity-0 shadow-md transition-opacity group-hover:opacity-100"
        >
          <div class="mb-1 font-semibold">{{ t('admin.accounts.gemini.quotaPolicy.title') }}</div>
          <div class="mb-2 text-white/80">{{ t('admin.accounts.gemini.quotaPolicy.note') }}</div>
          <div class="space-y-1">
            <div><strong>{{ quotaPolicyChannel }}:</strong></div>
            <div class="pl-2">• {{ quotaPolicyLimits }}</div>
            <div class="mt-2">
              <a
                :href="quotaPolicyDocsUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="text-accent-300 hover:text-accent-100 underline"
              >
                {{ t('admin.accounts.gemini.quotaPolicy.columns.docs') }} →
              </a>
            </div>
          </div>
        </span>
      </span>
    </div>

    <div class="text-xs text-ink-faint">
      <span v-if="!isRateLimited">
        {{ t('admin.accounts.gemini.rateLimit.ok') }}
      </span>
      <span
        v-else
        :class="[
          'font-medium',
          isUrgent
            ? 'text-danger animate-pulse'
            : 'text-warning'
        ]"
      >
        {{ t('admin.accounts.gemini.rateLimit.limited', { time: resetCountdown }) }}
      </span>
    </div>

  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account } from '@/types'
import { collectGeminiTierMetadataSources } from '@/utils/geminiExtra'
import { inferGeminiOAuthType } from '@/utils/geminiOAuthType'

const props = defineProps<{
  account: Account
}>()

const { t } = useI18n()

const translateOrFallback = (
  key: string,
  params: Record<string, string | number> | undefined,
  fallback: string
): string => {
  const translated = params ? t(key, params) : t(key)
  return translated === key ? fallback : translated
}

const geminiCredentials = computed(() => ({
  ...(props.account.credentials || {}),
  ...(props.account.extra || {})
}) as Record<string, unknown>)

const geminiValue = (key: string): string => {
  const value = geminiCredentials.value[key]
  return typeof value === 'string' ? value.trim() : ''
}

const normalizeGeminiCanonicalTier = (
  rawTier: string
): 'google_one_free' | 'google_ai_pro' | 'google_ai_ultra' | 'aistudio_free' | 'aistudio_paid' | 'gcp_standard' | 'gcp_enterprise' | '' => {
  const normalized = rawTier.trim().toLowerCase()
  if (!normalized) return ''
  if (
    normalized === 'google_ai_ultra' ||
    normalized === 'g1-ultra-tier' ||
    normalized === 'google_one_ultra' ||
    normalized === 'google_one_unlimited'
  ) return 'google_ai_ultra'
  if (
    normalized === 'google_ai_pro' ||
    normalized === 'g1-pro-tier' ||
    normalized === 'ai_premium'
  ) return 'google_ai_pro'
  if (
    normalized === 'google_one_free' ||
    normalized === 'google_one_unknown' ||
    normalized === 'free' ||
    normalized === 'free-tier'
  ) return 'google_one_free'
  if (
    normalized === 'aistudio_paid' ||
    normalized.includes('pay-as-you-go') ||
    normalized.includes('ai studio pay') ||
    normalized.includes('aistudio paid')
  ) return 'aistudio_paid'
  if (
    normalized === 'aistudio_free' ||
    normalized.includes('ai studio free') ||
    normalized.includes('aistudio free')
  ) return 'aistudio_free'
  if (
    normalized === 'gcp_standard' ||
    normalized === 'standard' ||
    normalized === 'standard-tier' ||
    normalized === 'pro-tier'
  ) return 'gcp_standard'
  if (
    normalized === 'gcp_enterprise' ||
    normalized === 'enterprise' ||
    normalized === 'ultra-tier'
  ) return 'gcp_enterprise'
  if (normalized.includes('ultra')) return 'google_ai_ultra'
  if (normalized.includes('pro') || normalized.includes('premium')) return 'google_ai_pro'
  if (normalized.includes('paid')) return 'aistudio_paid'
  if (normalized.includes('free')) return 'google_one_free'
  return ''
}

const geminiTierMetadataSources = computed(() => collectGeminiTierMetadataSources(
  (props.account.credentials || {}) as Record<string, unknown>,
  (props.account.extra || {}) as Record<string, unknown>
))

const resolveGeminiPlanBucket = (rawTier: string): 'free' | 'pro' | 'ultra' | 'unknown' => {
  const normalized = rawTier.trim().toLowerCase()
  if (!normalized) return 'unknown'
  if (normalized.includes('ultra')) return 'ultra'
  if (normalized.includes('pro') || normalized.includes('premium')) return 'pro'
  if (normalized.includes('free') || normalized === 'standard-tier') return 'free'
  return 'unknown'
}

const now = ref(new Date())
let timer: ReturnType<typeof setInterval> | null = null

const shouldShowQuota = computed(() => props.account.platform === 'gemini')
const isVertexServiceAccount = computed(() => props.account.platform === 'gemini' && props.account.type === 'service_account')

const isRateLimited = computed(() => {
  if (!props.account.rate_limit_reset_at) return false
  const resetTime = Date.parse(props.account.rate_limit_reset_at)
  if (Number.isNaN(resetTime)) return false
  return resetTime > now.value.getTime()
})

const resetCountdown = computed(() => {
  if (!props.account.rate_limit_reset_at) return ''
  const resetTime = Date.parse(props.account.rate_limit_reset_at)
  if (Number.isNaN(resetTime)) return '-'

  const diffMs = resetTime - now.value.getTime()
  if (diffMs <= 0) return t('admin.accounts.gemini.rateLimit.now')

  const diffSeconds = Math.floor(diffMs / 1000)
  const diffMinutes = Math.floor(diffSeconds / 60)
  const diffHours = Math.floor(diffMinutes / 60)

  if (diffMinutes < 1) {
    return translateOrFallback(
      'admin.accounts.gemini.rateLimit.secondsShort',
      { n: diffSeconds },
      `${diffSeconds}s`
    )
  }
  if (diffHours < 1) {
    const secs = diffSeconds % 60
    return translateOrFallback(
      'admin.accounts.gemini.rateLimit.minutesSeconds',
      { m: diffMinutes, s: secs },
      `${diffMinutes}m ${secs}s`
    )
  }
  const mins = diffMinutes % 60
  return t('common.time.countdown.hoursMinutes', { h: diffHours, m: mins })
})

const isUrgent = computed(() => {
  if (!props.account.rate_limit_reset_at) return false
  const resetTime = Date.parse(props.account.rate_limit_reset_at)
  if (Number.isNaN(resetTime)) return false
  const diffMs = resetTime - now.value.getTime()
  return diffMs > 0 && diffMs < 60000
})

const canonicalTier = computed(() => {
  return geminiTierMetadataSources.value
    .map((value) => normalizeGeminiCanonicalTier(value))
    .find((value) => value.length > 0) || ''
})
const legacyTier = computed(() => {
  const tier =
    geminiValue('tier_id') ||
    geminiValue('gemini_current_tier_id') ||
    geminiValue('gemini_paid_tier_id')
  return tier.toUpperCase()
})

const planTierSource = computed(() => {
  return geminiTierMetadataSources.value[0] || ''
})

const googleOnePlanBucket = computed(() => resolveGeminiPlanBucket(planTierSource.value))
const codeAssistPlanBucket = computed(() => {
  const source = (planTierSource.value || canonicalTier.value || legacyTier.value).trim().toLowerCase()
  if (source.includes('standard')) return 'standard'
  if (source.includes('enterprise')) return 'enterprise'
  if (source === 'standard-tier' || source === 'pro-tier') return 'standard'
  if (source === 'ultra-tier') return 'enterprise'
  return 'enterprise'
})
const inferredGeminiOAuthType = computed(() =>
  inferGeminiOAuthType(
    (props.account.credentials || {}) as Record<string, unknown>,
    (props.account.extra || {}) as Record<string, unknown>,
    ''
  )
)
const isCodeAssist = computed(() => {
  return inferredGeminiOAuthType.value === 'code_assist'
})

const isGoogleOne = computed(() => {
  return inferredGeminiOAuthType.value === 'google_one'
})

const tierLabel = computed(() => {
  if (isVertexServiceAccount.value) {
    return translateOrFallback('admin.accounts.gemini.tier.vertex', undefined, 'Vertex AI')
  }

  if (isCodeAssist.value) {
    return codeAssistPlanBucket.value === 'standard'
      ? translateOrFallback('admin.accounts.gemini.tier.gcp.standard', undefined, 'GCP Standard')
      : translateOrFallback('admin.accounts.gemini.tier.gcp.enterprise', undefined, 'GCP Enterprise')
  }

  if (isGoogleOne.value) {
    if (googleOnePlanBucket.value === 'ultra') {
      return translateOrFallback('admin.accounts.gemini.tier.googleOne.ultra', undefined, 'Google One Ultra')
    }
    if (googleOnePlanBucket.value === 'pro') {
      return translateOrFallback('admin.accounts.gemini.tier.googleOne.pro', undefined, 'Google One Pro')
    }
    if (googleOnePlanBucket.value === 'free') {
      return translateOrFallback('admin.accounts.gemini.tier.googleOne.free', undefined, 'Google One Free')
    }
    if (legacyTier.value === 'AI_PREMIUM') {
      return translateOrFallback('admin.accounts.gemini.tier.googleOne.pro', undefined, 'Google One Pro')
    }
    if (legacyTier.value === 'GOOGLE_ONE_UNLIMITED') {
      return translateOrFallback('admin.accounts.gemini.tier.googleOne.ultra', undefined, 'Google One Ultra')
    }
    return translateOrFallback('admin.accounts.oauth.gemini.googleOneTitle', undefined, 'Google One')
  }

  if (canonicalTier.value === 'aistudio_paid') {
    return translateOrFallback('admin.accounts.gemini.tier.aiStudio.paid', undefined, 'AI Studio Pay-as-you-go')
  }
  if (canonicalTier.value === 'aistudio_free') {
    return translateOrFallback('admin.accounts.gemini.tier.aiStudio.free', undefined, 'AI Studio Free Tier')
  }
  return translateOrFallback('admin.features.aiStudio.title', undefined, 'AI Studio')
})

const tierBadgeClass = computed(() => {
  if (isVertexServiceAccount.value) {
    return 'bg-brand-50 text-brand-700 dark:bg-brand-900/40 dark:text-brand-300'
  }

  if (isCodeAssist.value) {
    return codeAssistPlanBucket.value === 'standard'
      ? 'bg-accent-50 text-accent-600 dark:bg-accent-900/40 dark:text-accent-300'
      : 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
  }

  if (isGoogleOne.value) {
    if (googleOnePlanBucket.value === 'ultra' || legacyTier.value === 'GOOGLE_ONE_UNLIMITED') {
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
    }
    if (googleOnePlanBucket.value === 'pro' || legacyTier.value === 'AI_PREMIUM') {
      return 'bg-accent-50 text-accent-600 dark:bg-accent-900/40 dark:text-accent-300'
    }
    return 'bg-page text-ink-soft dark:bg-dark-700 dark:text-dark-300'
  }

  if (canonicalTier.value === 'aistudio_paid') return 'bg-accent-50 text-accent-600 dark:bg-accent-900/40 dark:text-accent-300'
  if (canonicalTier.value === 'aistudio_free') return 'bg-page text-ink-soft dark:bg-dark-700 dark:text-dark-300'
  return 'bg-accent-50 text-accent-600 dark:bg-accent-900/40 dark:text-accent-300'
})

const quotaPolicyChannel = computed(() => {
  if (isVertexServiceAccount.value) {
    return t('admin.accounts.gemini.quotaPolicy.rows.vertex.channel')
  }
  if (isCodeAssist.value) {
    return t('admin.accounts.gemini.quotaPolicy.rows.gcp.channel')
  }
  if (isGoogleOne.value) {
    return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.channel')
  }
  return t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.channel')
})

const quotaPolicyLimits = computed(() => {
  if (isVertexServiceAccount.value) {
    return t('admin.accounts.gemini.quotaPolicy.rows.vertex.limits')
  }

  if (isCodeAssist.value) {
    return codeAssistPlanBucket.value === 'standard'
      ? t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsStandard')
      : t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsEnterprise')
  }

  if (isGoogleOne.value) {
    if (googleOnePlanBucket.value === 'ultra') return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsUltra')
    if (googleOnePlanBucket.value === 'pro') return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsPro')
    return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsFree')
  }

  return canonicalTier.value === 'aistudio_paid'
    ? t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsPaid')
    : t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsFree')
})

const quotaPolicyDocsUrl = computed(() => {
  if (isVertexServiceAccount.value) {
    return 'https://cloud.google.com/vertex-ai/generative-ai/docs/quotas'
  }
  if (isCodeAssist.value || isGoogleOne.value) {
    return 'https://developers.google.com/gemini-code-assist/resources/code_assist_quota'
  }
  return 'https://ai.google.dev/gemini-api/docs/rate-limits'
})

watch(
  () => isRateLimited.value,
  (limited) => {
    if (limited && !timer) {
      timer = setInterval(() => {
        now.value = new Date()
      }, 1000)
    } else if (!limited && timer) {
      clearInterval(timer)
      timer = null
    }
  },
  { immediate: true }
)

onUnmounted(() => {
  if (timer) {
    clearInterval(timer)
    timer = null
  }
})
</script>
