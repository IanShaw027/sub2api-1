<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
    <div class="mt-2 grid grid-cols-2 gap-3" data-tour="account-form-type">
      <button
        type="button"
        @click="accountCategory = 'oauth-based'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountCategory === 'oauth-based'
 ? 'border-success bg-[color-mix(in_oklch,var(--success)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--success)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountCategory === 'oauth-based'
 ? 'bg-success text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="key" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">OAuth</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.types.chatgptOauth') }}</span>
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
          <span class="block text-sm font-medium text-foreground">API Key</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.types.responsesApi') }}</span>
        </div>
      </button>
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
