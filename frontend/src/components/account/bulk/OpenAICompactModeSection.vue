<template>
  <div v-if="show" class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div class="flex-1 pr-4">
        <label
          id="bulk-edit-openai-compact-mode-label"
          class="input-label mb-0"
          for="bulk-edit-openai-compact-mode-enabled"
        >
          {{ t('admin.accounts.openai.compactMode') }}
        </label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.openai.compactModeDesc') }}
        </p>
      </div>
      <input
        v-model="enableOpenAICompactMode"
        id="bulk-edit-openai-compact-mode-enabled"
        type="checkbox"
        aria-controls="bulk-edit-openai-compact-mode"
        class="rounded border-line text-accent focus:ring-accent"
      />
    </div>
    <div
      id="bulk-edit-openai-compact-mode"
      :class="!enableOpenAICompactMode && 'pointer-events-none opacity-50'"
    >
      <Select
        v-model="openAICompactMode"
        data-testid="bulk-edit-openai-compact-mode-select"
        :options="openAICompactModeOptions"
        aria-labelledby="bulk-edit-openai-compact-mode-label"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'
import type { OpenAICompactMode } from '@/types'

interface Props {
  show: boolean
  openAICompactModeOptions: SelectOption[]
}

defineProps<Props>()

const enableOpenAICompactMode = defineModel<boolean>('enableOpenAICompactMode', { required: true })
const openAICompactMode = defineModel<OpenAICompactMode>('openAICompactMode', { required: true })

const { t } = useI18n()
</script>
