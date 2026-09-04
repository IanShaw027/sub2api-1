<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
    <div class="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2" data-tour="account-form-type">
      <button
        type="button"
        @click="accountCategory = 'oauth-based'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountCategory === 'oauth-based'
 ? 'border-foreground bg-surface-2'
 : 'border-line hover:border-muted'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountCategory === 'oauth-based'
 ? 'bg-foreground text-background'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <PlatformIcon platform="grok" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">OAuth</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.types.grokOauth') }}</span>
        </div>
      </button>

      <button
        type="button"
        data-testid="grok-account-type-api-key"
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
          <span class="block text-sm font-medium text-foreground">API Key</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.types.responsesApi') }}</span>
        </div>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'

type AccountCategory = 'oauth-based' | 'apikey' | 'bedrock' | 'service_account'

const accountCategory = defineModel<AccountCategory>('accountCategory', { required: true })

const { t } = useI18n()
</script>
