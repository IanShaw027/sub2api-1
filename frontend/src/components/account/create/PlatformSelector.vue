<template>
  <!-- Platform Selection - Segmented Control Style -->
  <div>
    <label class="input-label">{{ t('admin.accounts.platform') }}</label>
    <div class="mt-2 flex flex-wrap rounded-lg bg-surface-2 p-1" data-tour="account-form-platform">
      <button
        type="button"
        @click="platform = 'anthropic'"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'anthropic'
 ? 'bg-surface text-warning-text shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
      >
        <Icon name="sparkles" size="sm" />
        Anthropic
      </button>
      <button
        type="button"
        @click="platform = 'openai'"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'openai'
 ? 'bg-surface text-success-600 shadow-sm'
 : 'text-muted hover:text-foreground'
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
            d="M3.75 13.5l10.5-11.25L12 10.5h8.25L9.75 21.75 12 13.5H3.75z"
          />
        </svg>
        OpenAI
      </button>
      <button
        type="button"
        @click="platform = 'gemini'"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'gemini'
 ? 'bg-surface text-accent shadow-sm'
 : 'text-muted hover:text-foreground'
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
            d="M12 2l1.5 6.5L20 10l-6.5 1.5L12 18l-1.5-6.5L4 10l6.5-1.5L12 2z"
          />
        </svg>
        Gemini
      </button>
      <button
        type="button"
        @click="platform = 'antigravity'"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'antigravity'
 ? 'bg-surface text-accent-600 shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
      >
        <Icon name="cloud" size="sm" />
        Antigravity
      </button>
      <button
        type="button"
        @click="platform = 'grok'"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'grok'
 ? 'bg-surface text-foreground shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
      >
        <PlatformIcon platform="grok" size="sm" />
        Grok
      </button>
      <button
        type="button"
        @click="platform = 'kiro'"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'kiro'
 ? 'bg-surface text-accent-600 shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
      >
        <PlatformIcon platform="kiro" size="md" />
        Kiro
      </button>
    </div>
    <!-- CN providers row: Kimi / Zhipu GLM / DeepSeek -->
    <div class="mt-2 flex flex-wrap rounded-lg bg-surface-2 p-1">
      <button
        type="button"
        @click="emit('selectCnPlatform', 'kimi')"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'kimi'
 ? 'bg-surface text-accent-600 shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
      >
        <PlatformIcon platform="kimi" size="sm" />
        Kimi
      </button>
      <button
        type="button"
        @click="emit('selectCnPlatform', 'zhipu')"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'zhipu'
 ? 'bg-surface text-accent-600 shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
      >
        <PlatformIcon platform="zhipu" size="sm" />
        Zhipu GLM
      </button>
      <button
        type="button"
        @click="emit('selectCnPlatform', 'deepseek')"
        :class="[
 'flex flex-1 items-center justify-center gap-2 rounded-md px-4 py-2.5 text-sm font-medium transition-all',
 platform === 'deepseek'
 ? 'bg-surface text-success-600 shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
      >
        <PlatformIcon platform="deepseek" size="sm" />
        DeepSeek
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
// Create-only: the platform segmented-control + CN-providers row. The
// segmented buttons write straight to the `platform` defineModel (host binds
// v-model:platform="form.platform"), matching the original inline
// `form.platform = 'x'` assignments. The CN provider buttons instead emit
// `selectCnPlatform`, since selecting one of those (selectCNPlatform in the
// host) has several additional side effects (form.type, accountCategory,
// apiProtocol, accountMode, apiKeyBaseUrl, resetAdaptiveBaseUrls) that must
// stay owned by the host, not be duplicated here.
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { AccountPlatform } from '@/types'

const { t } = useI18n()

const platform = defineModel<AccountPlatform>('platform', { required: true })

const emit = defineEmits<{
  (e: 'selectCnPlatform', platform: 'kimi' | 'zhipu' | 'deepseek'): void
}>()
</script>
