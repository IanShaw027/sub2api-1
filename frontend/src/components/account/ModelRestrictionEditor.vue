<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>

    <div
      v-if="disabledByPassthrough"
      class="mb-3 rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-3"
    >
      <p class="text-xs text-warning-text">
        {{ t('admin.accounts.openai.modelRestrictionDisabledByPassthrough') }}
      </p>
    </div>

    <template v-else>
      <!-- Mode Toggle -->
      <div class="mb-4 flex gap-2">
        <button
          type="button"
          @click="mode = 'whitelist'"
          :class="[
 'flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-all',
 mode === 'whitelist'
 ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
        >
          <svg
            v-if="showIcons"
            class="mr-1.5 inline h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
          {{ t('admin.accounts.modelWhitelist') }}
        </button>
        <button
          type="button"
          @click="mode = 'mapping'"
          :class="[
 'flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-all',
 mode === 'mapping'
 ? 'bg-[color-mix(in_oklch,var(--accent)_16%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
        >
          <svg
            v-if="showIcons"
            class="mr-1.5 inline h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
            />
          </svg>
          {{ t('admin.accounts.modelMapping') }}
        </button>
      </div>

      <!-- Whitelist Mode -->
      <div v-if="mode === 'whitelist'">
        <ModelWhitelistSelector
          v-model="allowedModels"
          :platform="platform"
          :account-id="accountId"
          :sync-credentials="syncCredentials"
          @upstream-synced="emit('upstream-synced')"
        />
        <p class="text-xs text-muted">
          {{ t('admin.accounts.selectedModels', { count: allowedModels.length }) }}
          <span v-if="allowedModels.length === 0">{{ t('admin.accounts.supportsAllModels') }}</span>
        </p>
      </div>

      <!-- Mapping Mode -->
      <div v-else>
        <div class="mb-3 rounded-lg bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] p-3">
          <p class="text-xs text-accent">
            <svg
              v-if="showIcons"
              class="mr-1 inline h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            {{ t('admin.accounts.mapRequestModels') }}
          </p>
        </div>

        <!-- Model Mapping List -->
        <div v-if="modelMappings.length > 0" class="mb-3 space-y-2">
          <div
            v-for="(mapping, index) in modelMappings"
            :key="index"
            class="flex items-center gap-2"
          >
            <input
              v-model="mapping.from"
              type="text"
              class="input flex-1"
              :placeholder="t('admin.accounts.requestModel')"
            />
            <svg class="h-4 w-4 flex-shrink-0 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3" />
            </svg>
            <input
              v-model="mapping.to"
              type="text"
              class="input flex-1"
              :placeholder="t('admin.accounts.actualModel')"
            />
            <button
              type="button"
              @click="removeModelMapping(index)"
              class="rounded-lg p-2 text-danger-text transition-colors hover:bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] hover:text-danger-text"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  stroke-linecap="round"
                  stroke-linejoin="round"
                  stroke-width="2"
                  d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                />
              </svg>
            </button>
          </div>
        </div>

        <button
          type="button"
          @click="addModelMapping"
          class="mb-3 w-full rounded-lg border-2 border-dashed border-line px-4 py-2 text-muted transition-colors hover:border-line hover:text-foreground"
        >
          <svg
            v-if="showIcons"
            class="mr-1 inline h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
          + {{ t('admin.accounts.addMapping') }}
        </button>

        <!-- Quick Add Buttons -->
        <div class="flex flex-wrap gap-2">
          <button
            v-for="preset in presets"
            :key="preset.from"
            type="button"
            @click="addPresetMapping(preset.from, preset.to)"
            :class="[presetButtonClass, preset.color]"
          >
            + {{ preset.label }}
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import { useAppStore } from '@/stores/app'
import type { getPresetMappingsByPlatform } from '@/composables/useModelWhitelist'

interface ModelMapping {
  from: string
  to: string
}

interface SyncCredentials {
  platform: string
  type: string
  base_url?: string
  api_key: string
}

interface Props {
  platform: string
  accountId?: number
  syncCredentials?: SyncCredentials
  disabledByPassthrough?: boolean
  showIcons?: boolean
  presetButtonClass?: string
  presets: ReturnType<typeof getPresetMappingsByPlatform>
}

withDefaults(defineProps<Props>(), {
  accountId: undefined,
  syncCredentials: undefined,
  disabledByPassthrough: false,
  showIcons: false,
  presetButtonClass: 'rounded-lg px-3 py-1 text-xs transition-colors'
})

const emit = defineEmits<{
  'upstream-synced': []
}>()

const mode = defineModel<'whitelist' | 'mapping'>('mode', { required: true })
const allowedModels = defineModel<string[]>('allowedModels', { required: true })
const modelMappings = defineModel<ModelMapping[]>('modelMappings', { required: true })

const { t } = useI18n()
const appStore = useAppStore()

const addModelMapping = () => {
  modelMappings.value.push({ from: '', to: '' })
}

const removeModelMapping = (index: number) => {
  modelMappings.value.splice(index, 1)
}

const addPresetMapping = (from: string, to: string) => {
  const exists = modelMappings.value.some((m) => m.from === from)
  if (exists) {
    appStore.showInfo(t('admin.accounts.mappingExists', { model: from }))
    return
  }
  modelMappings.value.push({ from, to })
}
</script>
