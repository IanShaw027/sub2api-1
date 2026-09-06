<template>
  <div v-if="show">
    <!-- OpenAI OAuth WS mode -->
    <div class="border-t border-line pt-4">
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-openai-ws-mode-label"
          class="input-label mb-0"
          for="bulk-edit-openai-ws-mode-enabled"
        >
          {{ t('admin.accounts.openai.wsMode') }}
        </label>
        <input
          v-model="enableOpenAIWSMode"
          id="bulk-edit-openai-ws-mode-enabled"
          type="checkbox"
          aria-controls="bulk-edit-openai-ws-mode"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <div
        id="bulk-edit-openai-ws-mode"
        :class="!enableOpenAIWSMode && 'pointer-events-none opacity-50'"
      >
        <p class="mb-3 text-xs text-muted">
          {{ t('admin.accounts.openai.wsModeDesc') }}
        </p>
        <p class="mb-3 text-xs text-muted">
          {{ t(openAIWSModeConcurrencyHintKey) }}
        </p>
        <Select
          v-model="openaiOAuthResponsesWebSocketV2Mode"
          data-testid="bulk-edit-openai-ws-mode-select"
          :options="openAIWSModeOptions"
          aria-labelledby="bulk-edit-openai-ws-mode-label"
        />
      </div>
    </div>

    <!-- OpenAI OAuth Codex CLI only -->
    <div class="border-t border-line pt-4">
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-openai-codex-cli-only-label"
          class="input-label mb-0"
          for="bulk-edit-openai-codex-cli-only-enabled"
        >
          {{ t('admin.accounts.openai.codexCLIOnly') }}
        </label>
        <input
          v-model="enableCodexCLIOnly"
          id="bulk-edit-openai-codex-cli-only-enabled"
          type="checkbox"
          aria-controls="bulk-edit-openai-codex-cli-only"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <div
        id="bulk-edit-openai-codex-cli-only"
        :class="!enableCodexCLIOnly && 'pointer-events-none opacity-50'"
      >
        <p class="mb-3 text-xs text-muted">
          {{ t('admin.accounts.openai.codexCLIOnlyDesc') }}
        </p>
        <button
          id="bulk-edit-openai-codex-cli-only-toggle"
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
            codexCLIOnlyEnabled ? 'bg-accent' : 'bg-surface-3'
          ]"
          @click="codexCLIOnlyEnabled = !codexCLIOnlyEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
              codexCLIOnlyEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
    </div>

    <!-- OpenAI OAuth: Codex app-server -->
    <div class="border-t border-line pt-4">
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-openai-codex-app-server-label"
          class="input-label mb-0"
          for="bulk-edit-openai-codex-app-server-enabled"
        >
          {{ t('admin.accounts.openai.codexCLIOnlyAppServer') }}
        </label>
        <input
          v-model="enableCodexCLIOnlyAppServer"
          id="bulk-edit-openai-codex-app-server-enabled"
          type="checkbox"
          aria-controls="bulk-edit-openai-codex-app-server"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <div
        id="bulk-edit-openai-codex-app-server"
        :class="!enableCodexCLIOnlyAppServer && 'pointer-events-none opacity-50'"
      >
        <p class="mb-3 text-xs text-muted">
          {{ t('admin.accounts.openai.codexCLIOnlyAppServerDesc') }}
        </p>
        <button
          id="bulk-edit-openai-codex-app-server-toggle"
          type="button"
          :class="[
            'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
            codexCLIOnlyAppServerEnabled ? 'bg-accent' : 'bg-surface-3'
          ]"
          @click="codexCLIOnlyAppServerEnabled = !codexCLIOnlyAppServerEnabled"
        >
          <span
            :class="[
              'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
              codexCLIOnlyAppServerEnabled ? 'translate-x-5' : 'translate-x-0'
            ]"
          />
        </button>
      </div>
    </div>

    <!-- Codex 指纹收敛模式（仅 OpenAI OAuth） -->
    <div class="border-t border-line pt-4">
      <div class="mb-3 flex items-center justify-between">
        <label class="input-label mb-0">{{ t('admin.accounts.openai.codexFingerprintMode') }}</label>
        <input
          id="bulk-edit-openai-codex-fingerprint-mode-enabled"
          v-model="enableCodexFingerprintMode"
          type="checkbox"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <div :class="!enableCodexFingerprintMode && 'pointer-events-none opacity-50'">
        <p class="mb-2 text-xs text-muted">
          {{ t('admin.accounts.openai.codexFingerprintModeDesc') }}
        </p>
        <Select
          v-model="codexFingerprintMode"
          data-testid="bulk-codex-fingerprint-mode-select"
          :options="codexFingerprintModeOptions"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import type { OpenAIWSMode } from '@/utils/openaiWsMode'

interface Props {
  show: boolean
  openAIWSModeOptions: SelectOption[]
  openAIWSModeConcurrencyHintKey: string
  codexFingerprintModeOptions: SelectOption[]
}

defineProps<Props>()

const enableOpenAIWSMode = defineModel<boolean>('enableOpenAIWSMode', { required: true })
const openaiOAuthResponsesWebSocketV2Mode = defineModel<OpenAIWSMode>(
  'openaiOAuthResponsesWebSocketV2Mode',
  { required: true }
)
const enableCodexCLIOnly = defineModel<boolean>('enableCodexCLIOnly', { required: true })
const codexCLIOnlyEnabled = defineModel<boolean>('codexCLIOnlyEnabled', { required: true })
const enableCodexCLIOnlyAppServer = defineModel<boolean>('enableCodexCLIOnlyAppServer', {
  required: true
})
const codexCLIOnlyAppServerEnabled = defineModel<boolean>('codexCLIOnlyAppServerEnabled', {
  required: true
})
const enableCodexFingerprintMode = defineModel<boolean>('enableCodexFingerprintMode', {
  required: true
})
const codexFingerprintMode = defineModel<string>('codexFingerprintMode', { required: true })

const { t } = useI18n()
</script>
