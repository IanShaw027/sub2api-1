<template>
  <div class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.grokCustomBaseUrl.title') }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.grokCustomBaseUrl.hint') }}
        </p>
      </div>
      <InlineToggleSwitch v-model="grokOAuthCustomBaseUrlEnabled" data-testid="grok-custom-base-url-toggle" />
    </div>
    <div v-if="grokOAuthCustomBaseUrlEnabled" class="space-y-2">
      <input
        v-model="grokOAuthBaseUrl"
        type="text"
        class="input"
        data-testid="grok-custom-base-url-input"
        :placeholder="t('admin.accounts.grokCustomBaseUrl.placeholder')"
      />
      <GrokBaseUrlPresets @select="grokOAuthBaseUrl = $event" />
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared "Grok OAuth Custom Upstream URL" section, used in both hosts with
// byte-identical inner markup — only the outer v-if condition (isOAuthFlow
// vs. account.type === 'oauth') differs, so that stays in each host template.
import { useI18n } from 'vue-i18n'
import InlineToggleSwitch from './InlineToggleSwitch.vue'
import GrokBaseUrlPresets from '@/components/account/GrokBaseUrlPresets.vue'

const { t } = useI18n()

const grokOAuthCustomBaseUrlEnabled = defineModel<boolean>('grokOAuthCustomBaseUrlEnabled', { required: true })
const grokOAuthBaseUrl = defineModel<string>('grokOAuthBaseUrl', { required: true })
</script>
