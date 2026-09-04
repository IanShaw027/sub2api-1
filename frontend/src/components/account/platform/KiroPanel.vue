<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
    <div class="mt-2 grid grid-cols-2 gap-3" data-tour="account-form-type">
      <button
        type="button"
        @click="kiroAccountType = 'oauth'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 kiroAccountType === 'oauth'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 kiroAccountType === 'oauth'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="sparkles" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{ t('admin.accounts.types.oauth') }}</span>
          <span class="text-xs text-muted">
            {{ t('admin.accounts.kiro.authorizationDesc') }}
          </span>
        </div>
      </button>

      <button
        type="button"
        @click="kiroAccountType = 'apikey'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 kiroAccountType === 'apikey'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 kiroAccountType === 'apikey'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="key" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">{{ t('admin.accounts.apiKey') }}</span>
          <span class="text-xs text-muted">
            {{ t('admin.accounts.kiro.manualApiKeyDesc') }}
          </span>
        </div>
      </button>
    </div>
  </div>

  <!-- Manual Kiro API Key config -->
  <div v-if="kiroAccountType === 'apikey'" class="space-y-4">
    <p class="rounded-lg border border-[color-mix(in_oklch,var(--accent)_35%,transparent)] bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] p-3 text-sm text-accent">
      {{ t('admin.accounts.kiro.runtimeManagedHint') }}
    </p>
    <div>
      <label class="input-label">{{ t('admin.accounts.apiKeyRequired') }}</label>
      <input
        v-model="apiKeyValue"
        type="password"
        required
        class="input font-mono"
        :placeholder="t('admin.accounts.kiro.apiKeyPlaceholder')"
      />
      <p class="input-hint">{{ t('admin.accounts.kiro.apiKeyHint') }}</p>
    </div>

    <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
      <div>
        <label class="input-label">{{ t('admin.accounts.kiro.regionLabel') }}</label>
        <input
          v-model="region"
          type="text"
          class="input font-mono text-sm"
          :placeholder="t('admin.accounts.kiro.regionPlaceholder')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.kiro.authRegionLabel') }}</label>
        <input
          v-model="authRegion"
          type="text"
          class="input font-mono text-sm"
          :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.kiro.apiRegionLabel') }}</label>
        <input
          v-model="apiRegion"
          type="text"
          class="input font-mono text-sm"
          :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.kiro.profileArnLabel') }}</label>
        <input
          v-model="profileArn"
          type="text"
          class="input font-mono text-sm"
          :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.kiro.machineIdLabel') }}</label>
        <input
          v-model="machineId"
          type="text"
          class="input font-mono text-sm"
          :placeholder="t('admin.accounts.kiro.optionalPlaceholder')"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const kiroAccountType = defineModel<'oauth' | 'apikey'>('kiroAccountType', { required: true })
const apiKeyValue = defineModel<string>('apiKeyValue', { required: true })
const region = defineModel<string>('region', { required: true })
const authRegion = defineModel<string>('authRegion', { required: true })
const apiRegion = defineModel<string>('apiRegion', { required: true })
const profileArn = defineModel<string>('profileArn', { required: true })
const machineId = defineModel<string>('machineId', { required: true })

const { t } = useI18n()
</script>
