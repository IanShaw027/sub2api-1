<template>
  <div v-if="shouldShowQuota" class="space-y-1">
    <div class="flex items-center gap-1">
      <span :class="['badge rounded px-1.5 py-0.5 text-[10px] font-medium', tierBadgeClass]">
        {{ tierLabel }}
      </span>
      <span class="group relative cursor-help">
        <svg
          class="h-3.5 w-3.5 text-gray-400 hover:text-gray-600 dark:text-gray-500 dark:hover:text-gray-300"
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
          class="pointer-events-none absolute left-0 top-full z-50 mt-1 w-80 whitespace-normal break-words rounded bg-gray-900 px-3 py-2 text-xs leading-relaxed text-white opacity-0 shadow-lg transition-opacity group-hover:opacity-100 dark:bg-gray-700"
        >
          <div class="mb-1 font-semibold">{{ t('admin.accounts.gemini.quotaPolicy.title') }}</div>
          <div class="mb-2 text-gray-300">{{ t('admin.accounts.gemini.quotaPolicy.note') }}</div>
          <div class="space-y-1">
            <div><strong>{{ quotaPolicyChannel }}:</strong></div>
            <div class="pl-2">• {{ quotaPolicyLimits }}</div>
            <div class="mt-2">
              <a
                :href="quotaPolicyDocsUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="text-blue-400 hover:text-blue-300 underline"
              >
                {{ t('admin.accounts.gemini.quotaPolicy.columns.docs') }} →
              </a>
            </div>
          </div>
        </span>
      </span>
    </div>

    <div v-if="detailRows.length" class="space-y-0.5 text-[10px] leading-relaxed text-gray-500 dark:text-gray-400">
      <div
        v-for="row in detailRows"
        :key="row.label"
        class="truncate"
        :title="row.value"
      >
        <span class="text-gray-400 dark:text-gray-500">{{ row.label }}:</span>
        {{ row.value }}
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, GeminiAvailableCredit, GeminiCredentials } from '@/types'

const props = defineProps<{
  account: Account
}>()

const { t } = useI18n()

type DetailRow = {
  label: string
  value: string
}

const geminiCredentials = computed(() => (props.account.credentials || {}) as GeminiCredentials)

const normalizeGeminiOAuthType = (oauthType?: string | null): 'code_assist' | 'google_one' | null => {
  const normalized = (oauthType || '').trim().toLowerCase()
  if (normalized === 'code_assist' || normalized === 'google_one') return normalized
  return null
}

const isCodeAssist = computed(() => {
  const oauthType = normalizeGeminiOAuthType(geminiCredentials.value.oauth_type)
  return oauthType === 'code_assist'
})

const isGoogleOne = computed(() => {
  const oauthType = normalizeGeminiOAuthType(geminiCredentials.value.oauth_type)
  return oauthType === 'google_one'
})

const shouldShowQuota = computed(() => props.account.platform === 'gemini')

const canonicalTier = computed(() => (geminiCredentials.value.tier_id || '').toString().trim().toLowerCase())
const legacyTier = computed(() => (geminiCredentials.value.tier_id || '').toString().trim().toUpperCase())

const tierLabel = computed(() => {
  if (isCodeAssist.value) {
    if (canonicalTier.value === 'gcp_enterprise') return 'GCP Enterprise'
    if (canonicalTier.value === 'gcp_standard') return 'GCP Standard'
    if (legacyTier.value.includes('ULTRA') || legacyTier.value.includes('ENTERPRISE')) return 'GCP Enterprise'
    if (legacyTier.value) return `GCP ${legacyTier.value}`
    return 'GCP'
  }

  if (isGoogleOne.value) {
    if (canonicalTier.value === 'google_ai_ultra') return 'Google AI Ultra'
    if (canonicalTier.value === 'google_ai_pro') return 'Google AI Pro'
    if (canonicalTier.value === 'google_one_free') return 'Google One Free'
    if (legacyTier.value === 'AI_PREMIUM') return 'Google AI Pro'
    if (legacyTier.value === 'GOOGLE_ONE_UNLIMITED') return 'Google AI Ultra'
    if (legacyTier.value) return `Google One ${legacyTier.value}`
    return 'Google One'
  }

  if (canonicalTier.value === 'aistudio_paid') return 'AI Studio Pay-as-you-go'
  if (canonicalTier.value === 'aistudio_free') return 'AI Studio Free Tier'
  return 'AI Studio'
})

const tierBadgeClass = computed(() => {
  if (isCodeAssist.value) {
    if (canonicalTier.value === 'gcp_enterprise' || legacyTier.value.includes('ULTRA') || legacyTier.value.includes('ENTERPRISE')) {
      return 'bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-300'
    }
    return 'bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300'
  }

  if (isGoogleOne.value) {
    if (canonicalTier.value === 'google_ai_ultra' || legacyTier.value === 'GOOGLE_ONE_UNLIMITED') {
      return 'bg-purple-100 text-purple-600 dark:bg-purple-900/40 dark:text-purple-300'
    }
    if (canonicalTier.value === 'google_ai_pro' || legacyTier.value === 'AI_PREMIUM') {
      return 'bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300'
    }
    return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  }

  if (canonicalTier.value === 'aistudio_paid') return 'bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300'
  if (canonicalTier.value === 'aistudio_free') return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
  return 'bg-blue-100 text-blue-600 dark:bg-blue-900/40 dark:text-blue-300'
})

const quotaPolicyChannel = computed(() => {
  if (isCodeAssist.value) {
    return t('admin.accounts.gemini.quotaPolicy.rows.gcp.channel')
  }
  if (isGoogleOne.value) {
    return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.channel')
  }
  return t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.channel')
})

const quotaPolicyLimits = computed(() => {
  if (isCodeAssist.value) {
    return canonicalTier.value === 'gcp_enterprise'
      ? t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsEnterprise')
      : t('admin.accounts.gemini.quotaPolicy.rows.gcp.limitsStandard')
  }

  if (isGoogleOne.value) {
    if (canonicalTier.value === 'google_ai_ultra') return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsUltra')
    if (canonicalTier.value === 'google_ai_pro') return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsPro')
    return t('admin.accounts.gemini.quotaPolicy.rows.googleOne.limitsFree')
  }

  return canonicalTier.value === 'aistudio_paid'
    ? t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsPaid')
    : t('admin.accounts.gemini.quotaPolicy.rows.aiStudio.limitsFree')
})

const quotaPolicyDocsUrl = computed(() => {
  if (isCodeAssist.value || isGoogleOne.value) {
    return 'https://developers.google.com/gemini-code-assist/resources/code_assist_quota'
  }
  return 'https://ai.google.dev/gemini-api/docs/rate-limits'
})

const scopeLabelMap: Record<string, string> = {
  openid: 'OpenID',
  email: 'Email',
  profile: 'Profile',
  'https://www.googleapis.com/auth/userinfo.email': 'UserInfo Email',
  'https://www.googleapis.com/auth/userinfo.profile': 'UserInfo Profile',
  'https://www.googleapis.com/auth/cloud-platform': 'Cloud Platform'
}

const scopeSummary = computed(() => {
  const scope = (geminiCredentials.value.scope || '').trim()
  if (!scope) return ''
  const labels = scope
    .split(/\s+/)
    .map(value => value.trim())
    .filter(Boolean)
    .map(value => scopeLabelMap[value] || value)
  return labels.join(', ')
})

const planSummary = computed(() => {
  const planName = (geminiCredentials.value.plan_name || '').trim()
  if (planName) return planName
  const paidTierName = (geminiCredentials.value.gemini_paid_tier_name || '').trim()
  if (paidTierName) return paidTierName
  const currentTierName = (geminiCredentials.value.gemini_current_tier_name || '').trim()
  if (currentTierName) return currentTierName
  return ''
})

const creditsSummary = computed(() => {
  const credits = geminiCredentials.value.gemini_available_credits
  if (!Array.isArray(credits) || credits.length === 0) return ''
  return credits
    .map((credit: GeminiAvailableCredit) => {
      const type = (credit.creditType || '').trim()
      const amount = (credit.creditAmount || '').trim()
      if (!type && !amount) return ''
      const typeLabel = type === 'GOOGLE_ONE_AI' ? 'Google One AI' : type || t('admin.accounts.gemini.details.credits')
      return amount ? `${typeLabel} ${amount}` : typeLabel
    })
    .filter(Boolean)
    .join(' / ')
})

const projectId = computed(() => (geminiCredentials.value.project_id || '').trim())
const email = computed(() => (geminiCredentials.value.email || '').trim())

const detailRows = computed<DetailRow[]>(() => {
  const rows: DetailRow[] = []

  if (planSummary.value) {
    rows.push({ label: t('admin.accounts.gemini.details.subscription'), value: planSummary.value })
  }
  if (email.value) {
    rows.push({ label: t('common.email'), value: email.value })
  }
  if (projectId.value) {
    rows.push({ label: t('admin.accounts.oauth.gemini.projectIdLabel'), value: projectId.value })
  }
  if (scopeSummary.value) {
    rows.push({ label: t('admin.accounts.gemini.details.scope'), value: scopeSummary.value })
  }
  if (creditsSummary.value) {
    rows.push({ label: t('admin.accounts.gemini.details.credits'), value: creditsSummary.value })
  }

  return rows
})
</script>
