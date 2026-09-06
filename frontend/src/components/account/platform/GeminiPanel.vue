<template>
  <div>
    <div class="flex items-center justify-between">
      <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
      <button
        type="button"
        @click="showHelpDialog = true"
        class="flex items-center gap-1 rounded px-2 py-1 text-xs text-accent hover:bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]"
      >
        <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 11-18 0 9 9 0 0118 0zm-9 5.25h.008v.008H12v-.008z" />
        </svg>
        {{ t('admin.accounts.gemini.helpButton') }}
      </button>
    </div>
    <div class="mt-2 grid grid-cols-3 gap-3" data-tour="account-form-type">
      <button
        type="button"
        @click="accountCategory = 'oauth-based'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountCategory === 'oauth-based'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountCategory === 'oauth-based'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="key" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">
            {{ t('admin.accounts.gemini.accountType.oauthTitle') }}
          </span>
          <span class="text-xs text-muted">
            {{ t('admin.accounts.gemini.accountType.oauthDesc') }}
          </span>
        </div>
      </button>

      <button
        type="button"
        @click="accountCategory = 'apikey'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountCategory === 'apikey'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountCategory === 'apikey'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <svg
            class="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="1.5"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1721.75 8.25z"
            />
          </svg>
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">
            {{ t('admin.accounts.gemini.accountType.apiKeyTitle') }}
          </span>
          <span class="text-xs text-muted">
            {{ t('admin.accounts.gemini.accountType.apiKeyDesc') }}
          </span>
        </div>
      </button>

      <button
        type="button"
        @click="accountCategory = 'service_account'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountCategory === 'service_account'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountCategory === 'service_account'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="cloud" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">
            Vertex
          </span>
          <span class="text-xs text-muted">
            Service Account
          </span>
        </div>
      </button>
    </div>

    <div
      v-if="accountCategory === 'apikey'"
      class="mt-3 rounded-lg border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] px-3 py-2 text-xs text-accent"
    >
      <p>{{ t('admin.accounts.gemini.accountType.apiKeyNote') }}</p>
      <div class="mt-2 flex flex-wrap gap-2">
        <a
          :href="geminiHelpLinks.apiKey"
          class="font-medium text-accent hover:underline"
          target="_blank"
          rel="noreferrer"
        >
          {{ t('admin.accounts.gemini.accountType.apiKeyLink') }}
        </a>
      </div>
    </div>

    <div
      v-if="accountCategory === 'service_account'"
      class="mt-3 rounded-lg border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] px-3 py-2 text-xs text-accent"
    >
      <p>{{ t('admin.accounts.vertexGeminiHint') }}</p>
    </div>

    <!-- OAuth Type Selection (only show when oauth-based is selected) -->
    <div v-if="accountCategory === 'oauth-based'" class="mt-4">
      <label class="input-label">{{ t('admin.accounts.oauth.gemini.oauthTypeLabel') }}</label>
      <div class="mt-2 grid grid-cols-2 gap-3">
        <!-- Google One OAuth -->
        <button
          type="button"
          @click="selectOAuthType('google_one')"
          :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 geminiOAuthType === 'google_one'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
        >
          <div
            :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 geminiOAuthType === 'google_one'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
          >
            <Icon name="user" size="sm" />
          </div>
          <div class="min-w-0">
            <span class="block text-sm font-medium text-foreground">
              Google One
            </span>
            <span class="text-xs text-muted">
              {{ t('admin.accounts.gemini.oauthType.googleOneDesc') }}
            </span>
            <div class="mt-2 flex flex-wrap gap-1">
              <span
                class="rounded bg-[color-mix(in_oklch,var(--accent)_16%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-accent"
              >
                {{ t('admin.accounts.gemini.oauthType.badges.individuals') }}
              </span>
              <span
                class="rounded bg-[color-mix(in_oklch,var(--success)_16%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-success-text"
              >
                {{ t('admin.accounts.gemini.oauthType.badges.noGcp') }}
              </span>
            </div>
          </div>
        </button>

        <!-- GCP Code Assist OAuth -->
        <button
          type="button"
          @click="selectOAuthType('code_assist')"
          :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 geminiOAuthType === 'code_assist'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_12%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
        >
          <div
            :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 geminiOAuthType === 'code_assist'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
          >
            <Icon name="cloud" size="sm" />
          </div>
          <div class="min-w-0">
            <span class="block text-sm font-medium text-foreground">
              GCP Code Assist
            </span>
            <span class="text-xs text-muted">
              {{ t('admin.accounts.gemini.oauthType.codeAssistDesc') }}
            </span>
            <div class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.gemini.oauthType.codeAssistRequirement') }}
              <a
                :href="geminiHelpLinks.gcpProject"
                class="ml-1 text-accent hover:underline"
                target="_blank"
                rel="noreferrer"
              >
                {{ t('admin.accounts.gemini.oauthType.gcpProjectLink') }}
              </a>
            </div>
            <div class="mt-2 flex flex-wrap gap-1">
              <span
                class="rounded bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-accent"
              >
                {{ t('admin.accounts.gemini.oauthType.badges.enterprise') }}
              </span>
              <span
                class="rounded bg-[color-mix(in_oklch,var(--success)_16%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-success-text"
              >
                {{ t('admin.accounts.gemini.oauthType.badges.highConcurrency') }}
              </span>
            </div>
          </div>
        </button>
      </div>

      <!-- Advanced Options Toggle -->
      <div class="mt-3">
        <button
          type="button"
          @click="showAdvancedOAuth = !showAdvancedOAuth"
          class="flex items-center gap-2 text-sm text-muted hover:text-foreground"
        >
          <svg
            :class="['h-4 w-4 transition-transform', showAdvancedOAuth ? 'rotate-90' : '']"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            stroke-width="2"
          >
            <path stroke-linecap="round" stroke-linejoin="round" d="M9 5l7 7-7 7" />
          </svg>
          <span>
            {{
              showAdvancedOAuth
                ? t('admin.accounts.gemini.oauthType.hideAdvanced')
                : t('admin.accounts.gemini.oauthType.showAdvanced')
            }}
          </span>
        </button>
      </div>

      <!-- Custom OAuth Client (Advanced) -->
      <div v-if="showAdvancedOAuth" class="mt-3 group relative">
        <button
          type="button"
          :disabled="!geminiAIStudioOAuthEnabled"
          @click="selectOAuthType('ai_studio')"
          :class="[
 'flex w-full items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 !geminiAIStudioOAuthEnabled ? 'cursor-not-allowed opacity-60' : '',
 geminiOAuthType === 'ai_studio'
 ? 'border-warning bg-[color-mix(in_oklch,var(--warning)_18%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--warning)_45%,transparent)]'
 ]"
        >
          <div
            :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 geminiOAuthType === 'ai_studio'
 ? 'bg-warning text-white'
 : 'bg-surface-2 text-muted'
 ]"
          >
            <svg
              class="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
              stroke-width="1.5"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09z"
              />
            </svg>
          </div>
          <div class="min-w-0">
            <span class="block text-sm font-medium text-foreground">
              {{ t('admin.accounts.gemini.oauthType.customTitle') }}
            </span>
            <span class="text-xs text-muted">
              {{ t('admin.accounts.gemini.oauthType.customDesc') }}
            </span>
            <div class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.gemini.oauthType.customRequirement') }}
            </div>
            <div class="mt-2 flex flex-wrap gap-1">
              <span
                class="rounded bg-[color-mix(in_oklch,var(--warning)_16%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-warning-text"
              >
                {{ t('admin.accounts.gemini.oauthType.badges.orgManaged') }}
              </span>
              <span
                class="rounded bg-[color-mix(in_oklch,var(--warning)_16%,transparent)] px-2 py-0.5 text-[10px] font-semibold text-warning-text"
              >
                {{ t('admin.accounts.gemini.oauthType.badges.adminRequired') }}
              </span>
            </div>
          </div>
          <span
            v-if="!geminiAIStudioOAuthEnabled"
            class="ml-auto shrink-0 rounded bg-[color-mix(in_oklch,var(--warning)_16%,transparent)] px-2 py-0.5 text-xs text-warning-text"
          >
            {{ t('admin.accounts.oauth.gemini.aiStudioNotConfiguredShort') }}
          </span>
        </button>

        <div
          v-if="!geminiAIStudioOAuthEnabled"
          class="pointer-events-none absolute right-0 top-full z-50 mt-2 w-80 rounded-md border border-[color-mix(in_oklch,var(--warning)_35%,transparent)] bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-3 py-2 text-xs text-warning-text opacity-0 shadow-[var(--shadow-pop)] transition-opacity group-hover:opacity-100"
        >
          {{ t('admin.accounts.oauth.gemini.aiStudioNotConfiguredTip') }}
        </div>
      </div>
    </div>

    <!-- Tier selection (used as fallback when auto-detection is unavailable/fails) -->
    <div v-if="accountCategory !== 'service_account'" class="mt-4">
      <label class="input-label">{{ t('admin.accounts.gemini.tier.label') }}</label>
      <div class="mt-2">
        <select
          v-if="geminiOAuthType === 'google_one'"
          v-model="geminiTierGoogleOne"
          class="input"
        >
          <option value="google_one_free">{{ t('admin.accounts.gemini.tier.googleOne.free') }}</option>
          <option value="google_ai_pro">{{ t('admin.accounts.gemini.tier.googleOne.pro') }}</option>
          <option value="google_ai_ultra">{{ t('admin.accounts.gemini.tier.googleOne.ultra') }}</option>
        </select>

        <select
          v-else-if="geminiOAuthType === 'code_assist'"
          v-model="geminiTierGcp"
          class="input"
        >
          <option value="gcp_standard">{{ t('admin.accounts.gemini.tier.gcp.standard') }}</option>
          <option value="gcp_enterprise">{{ t('admin.accounts.gemini.tier.gcp.enterprise') }}</option>
        </select>

        <select
          v-else
          v-model="geminiTierAIStudio"
          class="input"
        >
          <option value="aistudio_free">{{ t('admin.accounts.gemini.tier.aiStudio.free') }}</option>
          <option value="aistudio_paid">{{ t('admin.accounts.gemini.tier.aiStudio.paid') }}</option>
        </select>
      </div>
      <p class="input-hint">{{ t('admin.accounts.gemini.tier.hint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

type AccountCategory = 'oauth-based' | 'apikey' | 'bedrock' | 'service_account'
type GeminiOAuthType = 'code_assist' | 'google_one' | 'ai_studio'

interface Props {
  geminiAIStudioOAuthEnabled: boolean
  geminiHelpLinks: Record<string, string>
}

const props = defineProps<Props>()

const accountCategory = defineModel<AccountCategory>('accountCategory', { required: true })
const geminiOAuthType = defineModel<GeminiOAuthType>('geminiOauthType', { required: true })
const showAdvancedOAuth = defineModel<boolean>('showAdvancedOauth', { required: true })
const showHelpDialog = defineModel<boolean>('showHelpDialog', { required: true })
const geminiTierGoogleOne = defineModel<'google_one_free' | 'google_ai_pro' | 'google_ai_ultra'>(
  'tierGoogleOne',
  { required: true }
)
const geminiTierGcp = defineModel<'gcp_standard' | 'gcp_enterprise'>('tierGcp', { required: true })
const geminiTierAIStudio = defineModel<'aistudio_free' | 'aistudio_paid'>('tierAiStudio', {
  required: true
})

const { t } = useI18n()
const appStore = useAppStore()

const selectOAuthType = (oauthType: GeminiOAuthType) => {
  if (oauthType === 'ai_studio' && !props.geminiAIStudioOAuthEnabled) {
    appStore.showError(t('admin.accounts.oauth.gemini.aiStudioNotConfigured'))
    return
  }
  geminiOAuthType.value = oauthType
}
</script>
