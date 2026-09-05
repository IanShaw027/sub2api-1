<template>
  <div class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.poolMode') }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.poolModeHint') }}
        </p>
      </div>
      <InlineToggleSwitch v-model="poolModeEnabled" />
    </div>
    <div v-if="poolModeEnabled" class="rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-3">
      <p class="text-xs text-accent">
        <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
        {{ t('admin.accounts.poolModeInfo') }}
      </p>
    </div>
    <div v-if="poolModeEnabled" class="mt-3">
      <label class="input-label">{{ t('admin.accounts.poolModeRetryCount') }}</label>
      <input
        v-model.number="poolModeRetryCount"
        type="number"
        min="0"
        :max="MAX_POOL_MODE_RETRY_COUNT"
        step="1"
        class="input"
      />
      <p class="mt-1 text-xs text-muted">
        {{
          t('admin.accounts.poolModeRetryCountHint', {
            default: DEFAULT_POOL_MODE_RETRY_COUNT,
            max: MAX_POOL_MODE_RETRY_COUNT
          })
        }}
      </p>
    </div>
    <div v-if="poolModeEnabled" class="mt-3">
      <label class="input-label">{{ t('admin.accounts.poolModeRetryStatusCodes') }}</label>
      <input
        v-model="poolModeRetryStatusCodesInput"
        type="text"
        class="input"
        :placeholder="DEFAULT_POOL_MODE_RETRY_STATUS_CODES.join(', ')"
      />
      <p class="mt-1 text-xs text-muted">
        {{ t('admin.accounts.poolModeRetryStatusCodesHint', { default: DEFAULT_POOL_MODE_RETRY_STATUS_CODES.join(', ') }) }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared "Pool Mode" section, used 4x across CreateAccountModal.vue (normal +
// Bedrock variants) and EditAccountModal.vue (normal + Bedrock variants) —
// all four call sites were verified byte-identical except for the leading
// HTML comment text, which stays in each host template around this tag.
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'

const { t } = useI18n()

const DEFAULT_POOL_MODE_RETRY_COUNT = 3
const MAX_POOL_MODE_RETRY_COUNT = 10
const DEFAULT_POOL_MODE_RETRY_STATUS_CODES = [401, 403, 429]

const poolModeEnabled = defineModel<boolean>('poolModeEnabled', { required: true })
const poolModeRetryCount = defineModel<number>('poolModeRetryCount', { required: true })
const poolModeRetryStatusCodesInput = defineModel<string>('poolModeRetryStatusCodesInput', { required: true })
</script>
