<template>
 <div class="rounded-lg border border-line p-3">
 <div class="mb-2">
 <p class="text-sm font-medium text-foreground">
 {{ t('keys.useKeyModal.openai.authModeTitle') }}
 </p>
 <p class="mt-0.5 text-xs text-muted">
 {{ t('keys.useKeyModal.openai.authModeDescription') }}
 </p>
 </div>
 <div
 class="grid grid-cols-2 gap-1 rounded-lg bg-surface-2 p-1"
 role="radiogroup"
 :aria-label="t('keys.useKeyModal.openai.authModeTitle')"
 >
 <button
 type="button"
 role="radio"
 data-testid="codex-auth-mode-legacy"
 :aria-checked="codexAuthMode === 'legacy'"
 :class="[
 'rounded-md px-3 py-2 text-sm font-medium transition-colors',
 codexAuthMode === 'legacy'
 ? 'bg-surface text-accent shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
 @click="codexAuthMode = 'legacy'"
 >
 {{ t('keys.useKeyModal.openai.authModeLegacy') }}
 </button>
 <button
 type="button"
 role="radio"
 data-testid="codex-auth-mode-api-key"
 :aria-checked="codexAuthMode === 'api-key'"
 :class="[
 'rounded-md px-3 py-2 text-sm font-medium transition-colors',
 codexAuthMode === 'api-key'
 ? 'bg-surface text-accent shadow-sm'
 : 'text-muted hover:text-foreground'
 ]"
 @click="codexAuthMode = 'api-key'"
 >
 {{ t('keys.useKeyModal.openai.authModeApiKey') }}
 </button>
 </div>
 <div
 v-if="codexAuthMode === 'api-key'"
 data-testid="codex-api-key-restart-notice"
 class="mt-3 flex items-start gap-2 border-l-2 border-warning-400 bg-warning-50 px-3 py-2 text-xs leading-5 text-warning-800"
 >
 <Icon name="exclamationCircle" size="sm" class="mt-0.5 flex-shrink-0" />
 <p>{{ t('keys.useKeyModal.openai.authModeApiKeyRestartNotice') }}</p>
 </div>
 </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { CodexAuthMode } from './types'

const { t } = useI18n()

const codexAuthMode = defineModel<CodexAuthMode>({ required: true })
</script>
