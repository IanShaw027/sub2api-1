<template>
  <!-- 账户类型徽章 -->
  <div v-if="antigravityTierLabel" class="mb-1 flex items-center gap-1">
    <span
      :class="[
        'inline-block rounded px-1.5 py-0.5 text-[10px] font-medium',
        antigravityTierClass
      ]"
    >
      {{ antigravityTierLabel }}
    </span>
    <!-- 不合格账户警告图标 -->
    <span
      v-if="hasIneligibleTiers"
      class="group relative cursor-help"
    >
      <svg
        class="h-3.5 w-3.5 text-danger-500"
        fill="currentColor"
        viewBox="0 0 20 20"
      >
        <path
          fill-rule="evenodd"
          d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
          clip-rule="evenodd"
        />
      </svg>
      <span
        class="pointer-events-none absolute left-0 top-full z-50 mt-1 w-80 whitespace-normal break-words rounded bg-[var(--code-bg)] px-3 py-2 text-xs leading-relaxed text-white opacity-0 shadow-[var(--shadow-pop)] transition-opacity group-hover:opacity-100"
      >
        {{ t('admin.accounts.ineligibleWarning') }}
      </span>
    </span>
  </div>

  <!-- Forbidden state (403) -->
  <div v-if="isForbidden" class="space-y-1">
    <span
      :class="[
        'inline-block rounded px-1.5 py-0.5 text-[10px] font-medium',
        forbiddenBadgeClass
      ]"
    >
      {{ forbiddenLabel }}
    </span>
    <div v-if="validationURL" class="flex items-center gap-1">
      <a
        :href="validationURL"
        target="_blank"
        rel="noopener noreferrer"
        class="text-[10px] text-accent hover:text-accent-800 hover:underline"
        :title="t('admin.accounts.openVerification')"
      >
        {{ t('admin.accounts.openVerification') }}
      </a>
      <button
        type="button"
        class="text-[10px] text-muted hover:text-foreground"
        :title="t('admin.accounts.copyLink')"
        @click="copyValidationURL"
      >
        {{ linkCopied ? t('admin.accounts.linkCopied') : t('admin.accounts.copyLink') }}
      </button>
    </div>
  </div>

  <!-- Needs reauth (401) -->
  <div v-else-if="needsReauth" class="space-y-1">
    <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text">
      {{ t('admin.accounts.needsReauth') }}
    </span>
  </div>

  <!-- Degraded error (non-403, non-401) -->
  <div v-else-if="usageInfo?.error" class="space-y-1">
    <span class="inline-block rounded px-1.5 py-0.5 text-[10px] font-medium bg-warning-100 text-warning-700">
      {{ usageErrorLabel }}
    </span>
  </div>

  <!-- Loading state -->
  <div v-else-if="loading" class="space-y-1.5">
    <div class="flex items-center gap-1">
      <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
      <div class="h-1.5 w-8 animate-pulse rounded-full bg-surface-3"></div>
      <div class="h-3 w-[32px] animate-pulse rounded bg-surface-3"></div>
    </div>
  </div>

  <!-- Error state -->
  <div v-else-if="error" class="text-xs text-danger-500">
    {{ error }}
  </div>

  <!-- Usage data from API -->
  <div v-else-if="hasAntigravityQuotaFromAPI" class="space-y-1">
    <!-- Gemini 3 Pro -->
    <UsageProgressBar
      v-if="antigravity3ProUsageFromAPI !== null"
      :label="t('admin.accounts.usageWindow.gemini3Pro')"
      :utilization="antigravity3ProUsageFromAPI.utilization"
      :resets-at="antigravity3ProUsageFromAPI.resetTime"
      color="indigo"
    />

    <!-- Gemini 3 Flash -->
    <UsageProgressBar
      v-if="antigravity3FlashUsageFromAPI !== null"
      :label="t('admin.accounts.usageWindow.gemini3Flash')"
      :utilization="antigravity3FlashUsageFromAPI.utilization"
      :resets-at="antigravity3FlashUsageFromAPI.resetTime"
      color="emerald"
    />

    <!-- Gemini 3 Image -->
    <UsageProgressBar
      v-if="antigravity3ImageUsageFromAPI !== null"
      :label="t('admin.accounts.usageWindow.gemini3Image')"
      :utilization="antigravity3ImageUsageFromAPI.utilization"
      :resets-at="antigravity3ImageUsageFromAPI.resetTime"
      color="purple"
    />

    <!-- Claude -->
    <UsageProgressBar
      v-if="antigravityClaudeUsageFromAPI !== null"
      :label="t('admin.accounts.usageWindow.claude')"
      :utilization="antigravityClaudeUsageFromAPI.utilization"
      :resets-at="antigravityClaudeUsageFromAPI.resetTime"
      color="amber"
    />

    <div v-if="aiCreditsDisplay" class="mt-1 text-[10px] text-muted">
      💳 {{ t('admin.accounts.aiCreditsBalance') }}: {{ aiCreditsDisplay }}
    </div>
  </div>
  <div v-else-if="aiCreditsDisplay" class="text-[10px] text-muted">
    💳 {{ t('admin.accounts.aiCreditsBalance') }}: {{ aiCreditsDisplay }}
  </div>
  <div v-else class="text-xs text-muted">-</div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Account, AccountUsageInfo } from '@/types'
import UsageProgressBar from '../UsageProgressBar.vue'
import { useAntigravityUsage } from './useAntigravityUsage'
import { useUsageErrorState } from './useUsageErrorState'

const props = defineProps<{
  account: Account
  usageInfo: AccountUsageInfo | null
  loading: boolean
  error: string | null
}>()

const { t } = useI18n()

const account = computed(() => props.account)
const usageInfo = computed(() => props.usageInfo)

const {
  hasAntigravityQuotaFromAPI,
  antigravity3ProUsageFromAPI,
  antigravity3FlashUsageFromAPI,
  antigravity3ImageUsageFromAPI,
  antigravityClaudeUsageFromAPI,
  aiCreditsDisplay,
  antigravityTier,
  hasIneligibleTiers
} = useAntigravityUsage(account, usageInfo)

const { isForbidden, forbiddenLabel, forbiddenBadgeClass, validationURL, needsReauth, usageErrorLabel } =
  useUsageErrorState(usageInfo)

// 账户类型显示标签
const antigravityTierLabel = computed(() => {
  switch (antigravityTier.value) {
    case 'free-tier':
      return t('admin.accounts.tier.free')
    case 'g1-pro-tier':
      return t('admin.accounts.tier.pro')
    case 'g1-ultra-tier':
      return t('admin.accounts.tier.ultra')
    default:
      return null
  }
})

// 账户类型徽章样式
const antigravityTierClass = computed(() => {
  switch (antigravityTier.value) {
    case 'free-tier':
      return 'bg-surface-2 text-muted'
    case 'g1-pro-tier':
      return 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
    case 'g1-ultra-tier':
      return 'bg-accent-100 text-accent-600'
    default:
      return ''
  }
})

const linkCopied = ref(false)
const copyValidationURL = async () => {
  if (!validationURL.value) return
  try {
    await navigator.clipboard.writeText(validationURL.value)
    linkCopied.value = true
    setTimeout(() => { linkCopied.value = false }, 2000)
  } catch {
    // fallback: ignore
  }
}
</script>
