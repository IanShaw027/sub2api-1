<template>
  <div v-if="show">
    <!-- OpenAI API Key endpoint capabilities -->
    <div class="border-t border-line pt-4">
      <div class="mb-3 flex items-center justify-between gap-4">
        <div class="flex-1">
          <label
            id="bulk-edit-openai-endpoint-capabilities-label"
            class="input-label mb-0"
            for="bulk-edit-openai-endpoint-capabilities-enabled"
          >
            {{ t('admin.accounts.openai.endpointCapabilities') }}
          </label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.endpointCapabilitiesDesc') }}
          </p>
        </div>
        <input
          v-model="enableOpenAIEndpointCapabilities"
          id="bulk-edit-openai-endpoint-capabilities-enabled"
          type="checkbox"
          aria-controls="bulk-edit-openai-endpoint-capabilities-body"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <div
        id="bulk-edit-openai-endpoint-capabilities-body"
        :class="!enableOpenAIEndpointCapabilities && 'pointer-events-none opacity-50'"
        role="group"
        aria-labelledby="bulk-edit-openai-endpoint-capabilities-label"
      >
        <div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
          <label
            v-for="option in openAIEndpointCapabilityOptions"
            :key="option.value"
            class="flex cursor-pointer items-center gap-2 rounded-lg border border-line px-3 py-2 text-sm"
          >
            <input
              type="checkbox"
              :disabled="!enableOpenAIEndpointCapabilities"
              class="rounded border-line text-accent focus:ring-accent"
              :data-testid="`bulk-edit-openai-endpoint-capability-${option.value}`"
              :checked="openAIEndpointCapabilities.includes(option.value)"
              @change="emit('toggle-capability', option.value, $event)"
            />
            <span class="text-foreground">{{ option.label }}</span>
          </label>
        </div>
      </div>
    </div>

    <!-- OpenAI API Key Responses route -->
    <div class="border-t border-line pt-4">
      <div class="mb-3 flex items-center justify-between gap-4">
        <div class="flex-1">
          <label
            id="bulk-edit-openai-responses-mode-label"
            class="input-label mb-0"
            for="bulk-edit-openai-responses-mode-enabled"
          >
            {{ t('admin.accounts.openai.responsesMode') }}
          </label>
          <p class="mt-1 text-xs text-muted">
            {{ t('admin.accounts.openai.responsesModeDesc') }}
          </p>
        </div>
        <input
          v-model="enableOpenAIResponsesMode"
          id="bulk-edit-openai-responses-mode-enabled"
          type="checkbox"
          aria-controls="bulk-edit-openai-responses-mode-body"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <div
        id="bulk-edit-openai-responses-mode-body"
        :class="!enableOpenAIResponsesMode && 'pointer-events-none opacity-50'"
        role="group"
        aria-labelledby="bulk-edit-openai-responses-mode-label"
      >
        <Select
          v-model="openAIResponsesMode"
          :disabled="!enableOpenAIResponsesMode || !openAIResponsesModeApplicable"
          data-testid="bulk-edit-openai-responses-mode-select"
          :options="openAIResponsesModeOptions"
          aria-labelledby="bulk-edit-openai-responses-mode-label"
        />
        <p
          v-if="enableOpenAIEndpointCapabilities && !openAITextGenerationCapabilityEnabled"
          class="mt-2 rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-3 py-2 text-xs text-warning-text"
          data-testid="bulk-edit-openai-responses-mode-not-applicable"
        >
          {{ t('admin.accounts.openai.responsesModeTextDisabledHint') }}
        </p>
      </div>
    </div>

    <!-- OpenAI API Key WS mode -->
    <div class="border-t border-line pt-4">
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-openai-apikey-ws-mode-label"
          class="input-label mb-0"
          for="bulk-edit-openai-apikey-ws-mode-enabled"
        >
          {{ t('admin.accounts.openai.wsMode') }}
        </label>
        <input
          v-model="enableOpenAIAPIKeyWSMode"
          id="bulk-edit-openai-apikey-ws-mode-enabled"
          type="checkbox"
          aria-controls="bulk-edit-openai-apikey-ws-mode"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <div
        id="bulk-edit-openai-apikey-ws-mode"
        :class="!enableOpenAIAPIKeyWSMode && 'pointer-events-none opacity-50'"
      >
        <p class="mb-3 text-xs text-muted">
          {{ t('admin.accounts.openai.wsModeDesc') }}
        </p>
        <p class="mb-3 text-xs text-muted">
          {{ t(openAIAPIKeyWSModeConcurrencyHintKey) }}
        </p>
        <Select
          v-model="openaiAPIKeyResponsesWebSocketV2Mode"
          data-testid="bulk-edit-openai-apikey-ws-mode-select"
          :options="openAIWSModeOptions"
          aria-labelledby="bulk-edit-openai-apikey-ws-mode-label"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import type { OpenAIEndpointCapability, OpenAIResponsesMode } from '@/types'
import type { OpenAIWSMode } from '@/utils/openaiWsMode'

interface Props {
  show: boolean
  openAIEndpointCapabilityOptions: Array<{ value: OpenAIEndpointCapability; label: string }>
  openAIResponsesModeOptions: SelectOption[]
  openAIResponsesModeApplicable: boolean
  openAITextGenerationCapabilityEnabled: boolean
  openAIWSModeOptions: SelectOption[]
  openAIAPIKeyWSModeConcurrencyHintKey: string
}

defineProps<Props>()

const emit = defineEmits<{
  'toggle-capability': [value: OpenAIEndpointCapability, event: Event]
}>()

const enableOpenAIEndpointCapabilities = defineModel<boolean>('enableOpenAIEndpointCapabilities', {
  required: true
})
const openAIEndpointCapabilities = defineModel<OpenAIEndpointCapability[]>(
  'openAIEndpointCapabilities',
  { required: true }
)
const enableOpenAIResponsesMode = defineModel<boolean>('enableOpenAIResponsesMode', {
  required: true
})
const openAIResponsesMode = defineModel<OpenAIResponsesMode>('openAIResponsesMode', {
  required: true
})
const enableOpenAIAPIKeyWSMode = defineModel<boolean>('enableOpenAIAPIKeyWSMode', {
  required: true
})
const openaiAPIKeyResponsesWebSocketV2Mode = defineModel<OpenAIWSMode>(
  'openaiAPIKeyResponsesWebSocketV2Mode',
  { required: true }
)

const { t } = useI18n()
</script>
