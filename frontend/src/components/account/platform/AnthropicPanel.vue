<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
    <div class="mt-2 grid grid-cols-2 gap-3 sm:grid-cols-4" data-tour="account-form-type">
      <button
        type="button"
        @click="accountCategory = 'oauth-based'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountCategory === 'oauth-based'
 ? 'border-warning bg-[color-mix(in_oklch,var(--warning)_18%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--warning)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountCategory === 'oauth-based'
 ? 'bg-warning text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="sparkles" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{
            t('admin.accounts.claudeCode')
          }}</span>
          <span class="text-xs text-muted">{{
            t('admin.accounts.oauthSetupToken')
          }}</span>
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
          <Icon name="key" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{
            t('admin.accounts.claudeConsole')
          }}</span>
          <span class="text-xs text-muted">{{
            t('admin.accounts.apiKey')
          }}</span>
        </div>
      </button>

      <button
        type="button"
        @click="accountCategory = 'bedrock'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountCategory === 'bedrock'
 ? 'border-warning bg-[color-mix(in_oklch,var(--warning)_18%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--warning)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountCategory === 'bedrock'
 ? 'bg-warning text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="cloud" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{
            t('admin.accounts.bedrockLabel')
          }}</span>
          <span class="text-xs text-muted">{{
            t('admin.accounts.bedrockDesc')
          }}</span>
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
          <span class="block text-sm font-medium text-foreground">Vertex</span>
          <span class="text-xs text-muted">Service Account</span>
        </div>
      </button>
    </div>

    <div
      v-if="accountCategory === 'service_account'"
      class="mt-3 rounded-lg border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] px-3 py-2 text-xs text-accent"
    >
      <p>{{ t('admin.accounts.vertexAnthropicHint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

type AccountCategory = 'oauth-based' | 'apikey' | 'bedrock' | 'service_account'

const accountCategory = defineModel<AccountCategory>('accountCategory', { required: true })

const { t } = useI18n()
</script>
