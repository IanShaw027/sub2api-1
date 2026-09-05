<template>
  <div class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.headerOverride.title') }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.headerOverride.hint') }}
        </p>
      </div>
      <InlineToggleSwitch v-model="headerOverrideEnabled" />
    </div>

    <div v-if="headerOverrideEnabled" class="space-y-3">
      <div class="rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-3">
        <p class="text-xs text-accent">
          <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
          {{ t('admin.accounts.headerOverride.info') }}
        </p>
      </div>

      <HeaderOverrideEditor
        :rows="headerOverrideRows"
        @update:rows="headerOverrideRows = $event"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared "Header Override" section, used across three call sites (Create's
// apikey-container variant + Create's grok-OAuth-only variant, and Edit's
// single `headerOverrideCapable`-gated variant) with byte-identical inner
// markup — only the outer v-if condition differs per call site, so that
// stays in each host template.
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import InlineToggleSwitch from './InlineToggleSwitch.vue'
import HeaderOverrideEditor from '@/components/account/HeaderOverrideEditor.vue'
import type { HeaderOverrideRow } from '@/components/account/credentialsBuilder'

const { t } = useI18n()

const headerOverrideEnabled = defineModel<boolean>('headerOverrideEnabled', { required: true })
const headerOverrideRows = defineModel<HeaderOverrideRow[]>('headerOverrideRows', { required: true })
</script>
