<template>
  <div class="inline-flex flex-col gap-0.5 text-xs font-medium">
    <!-- Row 1: Platform + Type -->
    <div class="inline-flex items-center overflow-hidden rounded-md">
      <span :class="['inline-flex items-center gap-1 px-2 py-1', platformClass]">
        <PlatformIcon :platform="platform" size="xs" />
        <span>{{ platformLabel }}</span>
      </span>
      <span :class="['inline-flex items-center gap-1 px-1.5 py-1', typeClass]">
        <!-- OAuth icon -->
        <svg
          v-if="type === 'oauth'"
          class="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
          />
        </svg>
        <!-- Setup Token icon -->
        <Icon v-else-if="type === 'setup-token'" name="shield" size="xs" />
        <!-- API Key icon -->
        <Icon v-else-if="type === 'service_account'" name="cloud" size="xs" />
        <Icon v-else name="key" size="xs" />
        <span>{{ typeLabel }}</span>
      </span>
      <span
        v-if="organizationRoleLabel"
        class="inline-flex items-center gap-1 bg-indigo-100 px-1.5 py-1 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300"
      >
        <span>{{ organizationRoleLabel }}</span>
      </span>
    </div>
    <!-- Row 2: Plan type + Privacy mode (only if either exists) -->
    <div v-if="planLabel || privacyBadge" class="inline-flex items-center overflow-hidden rounded-md">
      <span v-if="planLabel" :class="['inline-flex items-center gap-1 px-1.5 py-1', planBadgeClass]">
        <GrokFreeIcon
          v-if="isGrokFreePlan"
          data-testid="grok-free-plan-icon"
        />
        <Icon
          v-else-if="grokPlanIconName"
          :name="grokPlanIconName"
          size="xs"
          data-testid="grok-plan-icon"
          aria-hidden="true"
        />
        <span>{{ planLabel }}</span>
      </span>
      <span
        v-if="privacyBadge"
        :class="['inline-flex items-center gap-1 px-1.5 py-1', privacyBadge.class]"
        :title="privacyBadge.title"
      >
        <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" :d="privacyBadge.icon" />
        </svg>
        <span>{{ privacyBadge.label }}</span>
      </span>
    </div>
    <!-- Row 3: Subscription expiration (non-free paid accounts only) -->
    <div v-if="expiresLabel" class="text-[10px] leading-tight text-ink-faint dark:text-dark-500 pl-0.5" :title="subscriptionExpiresAt">
      {{ expiresLabel }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountPlatform, AccountType } from '@/types'
import GrokFreeIcon from './GrokFreeIcon.vue'
import PlatformIcon from './PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

interface Props {
  platform: AccountPlatform
  type: AccountType
  planType?: string
  typeLabelOverride?: string
  privacyMode?: string
  subscriptionExpiresAt?: string
  organizationRole?: string
}

const props = defineProps<Props>()

const normalizeGeminiTier = (
  value?: string
): 'google_one_free' | 'google_ai_pro' | 'google_ai_ultra' | 'aistudio_free' | 'aistudio_paid' | 'gcp_standard' | 'gcp_enterprise' | '' => {
  const normalized = (value || '').trim().toLowerCase()
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
  if (normalized === 'aistudio_paid') return 'aistudio_paid'
  if (normalized === 'aistudio_free') return 'aistudio_free'
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
  if (
    normalized.includes('pro') ||
    normalized.includes('premium')
  ) return 'google_ai_pro'
  if (normalized.includes('paid')) return 'aistudio_paid'
  if (normalized.includes('free')) return 'google_one_free'
  return ''
}

const geminiTier = computed(() => (
  props.platform === 'gemini' ? normalizeGeminiTier(props.planType) : ''
))

const normalizedGrokPlan = computed(() => (
  props.platform === 'grok'
    ? (props.planType || '').trim().toLowerCase().replace(/[\s_-]+/g, '')
    : ''
))

const platformLabel = computed(() => {
  if (props.platform === 'anthropic') return 'Anthropic'
  if (props.platform === 'kiro') return 'Kiro'
  if (props.platform === 'openai') return 'OpenAI'
  if (props.platform === 'sora') return 'Sora'
  if (props.platform === 'antigravity') return 'Antigravity'
  if (props.platform === 'grok') return 'Grok'
  return 'Gemini'
})

const typeLabel = computed(() => {
  if (props.typeLabelOverride) {
    return props.typeLabelOverride
  }
  switch (props.type) {
    case 'oauth':
      return t('admin.accounts.types.oauth')
    case 'setup-token':
      return t('admin.accounts.setupToken')
    case 'apikey':
      return t('admin.accounts.apiKey')
    case 'bedrock':
      return t('admin.accounts.bedrockLabel')
    case 'service_account':
      return t('admin.accounts.vertexLabel')
    default:
      return props.type
  }
})

const planLabel = computed(() => {
  if (!props.planType) return ''
  if (props.platform === 'gemini' && geminiTier.value) {
    switch (geminiTier.value) {
      case 'google_one_free':
        return 'Free'
      case 'google_ai_pro':
        return 'Pro'
      case 'google_ai_ultra':
        return 'Ultra'
      case 'aistudio_free':
        return 'AI Studio Free Tier'
      case 'aistudio_paid':
        return 'AI Studio Pay-as-you-go'
      case 'gcp_standard':
        return 'GCP Standard'
      case 'gcp_enterprise':
        return 'GCP Enterprise'
      default:
        break
    }
  }
  const lower = props.planType.toLowerCase()
  if (props.platform === 'grok') {
    switch (normalizedGrokPlan.value) {
      case 'supergrok':
        return 'SuperGrok'
      case 'supergrokheavy':
        return 'SuperGrok Heavy'
      case 'heavy':
        return 'Heavy'
      case 'free':
      case 'basic':
        return 'Grok Free'
      default:
        return props.planType
    }
  }
  switch (lower) {
    case 'plus':
      return 'Plus'
    case 'team':
      return 'Team'
    case 'chatgptpro':
    case 'pro':
      return 'Pro'
    case 'free':
      return 'Free'
    case 'abnormal':
      return t('admin.accounts.subscriptionAbnormal')
    default:
      return props.planType
  }
})

const isGrokFreePlan = computed(() => (
  normalizedGrokPlan.value === 'free' || normalizedGrokPlan.value === 'basic'
))

const grokPlanIconName = computed<'bolt' | null>(() => {
  if (normalizedGrokPlan.value === 'supergrok' || normalizedGrokPlan.value === 'supergrokheavy') {
    return 'bolt'
  }
  return null
})

const platformClass = computed(() => {
  if (props.platform === 'anthropic') {
    return 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400'
  }
  if (props.platform === 'openai') {
    return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
  }
  if (props.platform === 'sora') {
    return 'bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-300'
  }
  if (props.platform === 'kiro') {
    return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-400'
  }
  if (props.platform === 'antigravity') {
    return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-400'
  }
  if (props.platform === 'grok') {
    return 'bg-zinc-100 text-zinc-700 dark:bg-zinc-800 dark:text-zinc-300'
  }
  return 'bg-accent-100 text-accent-700 dark:bg-blue-900/30 dark:text-blue-400'
})

const typeClass = computed(() => {
  if (props.platform === 'anthropic') {
    return 'bg-orange-100 text-orange-600 dark:bg-orange-900/30 dark:text-orange-400'
  }
  if (props.platform === 'openai') {
    return 'bg-emerald-100 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-400'
  }
  if (props.platform === 'sora') {
    return 'bg-rose-100 text-rose-600 dark:bg-rose-900/30 dark:text-rose-300'
  }
  if (props.platform === 'kiro') {
    return 'bg-cyan-100 text-cyan-600 dark:bg-cyan-900/30 dark:text-cyan-400'
  }
  if (props.platform === 'antigravity') {
    return 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-400'
  }
  if (props.platform === 'grok') {
    return 'bg-zinc-100 text-zinc-600 dark:bg-zinc-800 dark:text-zinc-300'
  }
  return 'bg-accent-100 text-accent-600 dark:bg-blue-900/30 dark:text-blue-400'
})

const planBadgeClass = computed(() => {
  if (props.planType && props.planType.toLowerCase() === 'abnormal') {
    return 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400'
  }
  if (props.platform === 'grok' && props.planType) {
    const normalized = props.planType.trim().toLowerCase().replace(/[\s_-]+/g, '')
    if (normalized === 'free' || normalized === 'basic') {
      return 'bg-page text-ink-body dark:bg-dark-700 dark:text-dark-300'
    }
    if (normalized.includes('heavy')) {
      return 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-300'
    }
    if (normalized.includes('supergrok')) {
      return 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300'
    }
  }
  if (props.platform === 'gemini' && geminiTier.value) {
    switch (geminiTier.value) {
      case 'google_one_free':
      case 'aistudio_free':
        return 'bg-page text-ink-body dark:bg-dark-700 dark:text-dark-300'
      case 'google_ai_pro':
      case 'aistudio_paid':
      case 'gcp_standard':
        return 'bg-accent-100 text-accent-600 dark:bg-blue-900/30 dark:text-blue-300'
      case 'google_ai_ultra':
      case 'gcp_enterprise':
        return 'bg-purple-100 text-purple-600 dark:bg-purple-900/30 dark:text-purple-300'
      default:
        break
    }
  }
  return typeClass.value
})

const organizationRoleLabel = computed(() => {
  if (props.platform !== 'openai' || props.type !== 'oauth') return ''
  const planType = (props.planType || '').trim().toLowerCase()
  if (planType !== 'team') return ''
  const normalized = (props.organizationRole || '').trim().toLowerCase()
  if (normalized === 'owner' || normalized === 'admin' || normalized === 'leader') {
    return t('admin.accounts.badges.teamLeader')
  }
  return ''
})

// Subscription expiration label (non-free only)
const expiresLabel = computed(() => {
  if (!props.subscriptionExpiresAt || !props.planType) return ''
  if (props.platform === 'gemini') {
    if (geminiTier.value === 'google_one_free' || geminiTier.value === 'aistudio_free') return ''
  } else if (props.platform === 'grok' && isGrokFreePlan.value) {
    return ''
  } else if (props.planType.toLowerCase() === 'free') {
    return ''
  }
  try {
    const d = new Date(props.subscriptionExpiresAt)
    if (isNaN(d.getTime())) return ''
    const yyyy = d.getFullYear()
    const mm = String(d.getMonth() + 1).padStart(2, '0')
    const dd = String(d.getDate()).padStart(2, '0')
    return `${t('admin.accounts.subscriptionExpires')} ${yyyy}-${mm}-${dd}`
  } catch {
    return ''
  }
})

// Privacy badge — shows different states for OpenAI/Antigravity OAuth privacy setting
const privacyBadge = computed(() => {
  if (props.type !== 'oauth' || !props.privacyMode) return null
  // 支持 OpenAI 和 Antigravity 平台
  if (props.platform !== 'openai' && props.platform !== 'antigravity') return null

  const shieldCheck = 'M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z'
  const shieldX = 'M12 9v3.75m0-10.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285zM12 18h.008v.008H12V18z'
  switch (props.privacyMode) {
    // OpenAI states
    case 'training_off':
      return { label: t('admin.accounts.badges.private'), icon: shieldCheck, title: t('admin.accounts.privacyTrainingOff'), class: 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400' }
    case 'training_set_cf_blocked':
      return { label: t('admin.accounts.badges.cf'), icon: shieldX, title: t('admin.accounts.privacyCfBlocked'), class: 'bg-yellow-100 text-yellow-600 dark:bg-yellow-900/30 dark:text-yellow-400' }
    case 'training_set_failed':
      return { label: t('admin.accounts.badges.fail'), icon: shieldX, title: t('admin.accounts.privacyFailed'), class: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400' }
    // Antigravity states
    case 'privacy_set':
      return { label: t('admin.accounts.badges.private'), icon: shieldCheck, title: t('admin.accounts.privacyAntigravitySet'), class: 'bg-green-100 text-green-600 dark:bg-green-900/30 dark:text-green-400' }
    case 'privacy_set_failed':
      return { label: t('admin.accounts.badges.fail'), icon: shieldX, title: t('admin.accounts.privacyAntigravityFailed'), class: 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400' }
    default:
      return null
  }
})
</script>
