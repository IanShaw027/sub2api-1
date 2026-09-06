<template>
  <div class="space-y-4">
    <!-- SigV4 fields -->
    <template v-if="!isBedrockApiKeyMode">
      <div>
        <label class="input-label">{{ t('admin.accounts.bedrockAccessKeyId') }}</label>
        <input
          v-model="editBedrockAccessKeyId"
          type="text"
          class="input font-mono"
          placeholder="AKIA..."
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.bedrockSecretAccessKey') }}</label>
        <input
          v-model="editBedrockSecretAccessKey"
          type="password"
          class="input font-mono"
          :placeholder="t('admin.accounts.bedrockSecretKeyLeaveEmpty')"
        />
        <p class="input-hint">{{ t('admin.accounts.bedrockSecretKeyLeaveEmpty') }}</p>
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.bedrockSessionToken') }}</label>
        <input
          v-model="editBedrockSessionToken"
          type="password"
          class="input font-mono"
          :placeholder="t('admin.accounts.bedrockSecretKeyLeaveEmpty')"
        />
        <p class="input-hint">{{ t('admin.accounts.bedrockSessionTokenHint') }}</p>
      </div>
    </template>

    <!-- API Key field -->
    <div v-if="isBedrockApiKeyMode">
      <label class="input-label">{{ t('admin.accounts.bedrockApiKeyInput') }}</label>
      <input
        v-model="editBedrockApiKeyValue"
        type="password"
        class="input font-mono"
        :placeholder="t('admin.accounts.bedrockApiKeyLeaveEmpty')"
      />
      <p class="input-hint">{{ t('admin.accounts.bedrockApiKeyLeaveEmpty') }}</p>
    </div>

    <!-- Shared: Region -->
    <div>
      <label class="input-label">{{ t('admin.accounts.bedrockRegion') }}</label>
      <input
        v-model="editBedrockRegion"
        type="text"
        class="input"
        placeholder="us-east-1"
      />
      <p class="input-hint">{{ t('admin.accounts.bedrockRegionHint') }}</p>
    </div>

    <!-- Shared: Force Global -->
    <div>
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          v-model="editBedrockForceGlobal"
          type="checkbox"
          class="rounded border-line text-accent focus:ring-accent"
        />
        <span class="text-sm text-foreground">{{ t('admin.accounts.bedrockForceGlobal') }}</span>
      </label>
      <p class="input-hint mt-1">{{ t('admin.accounts.bedrockForceGlobalHint') }}</p>
    </div>

    <!-- Model Restriction for Bedrock -->
    <div class="border-t border-line pt-4">
      <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>

      <!-- Mode Toggle -->
      <div class="mb-4 flex gap-2">
        <button
          type="button"
          @click="modelRestrictionMode = 'whitelist'"
          :class="[
 'flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-all',
 modelRestrictionMode === 'whitelist'
 ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
        >
          {{ t('admin.accounts.modelWhitelist') }}
        </button>
        <button
          type="button"
          @click="modelRestrictionMode = 'mapping'"
          :class="[
 'flex-1 rounded-lg px-4 py-2 text-sm font-medium transition-all',
 modelRestrictionMode === 'mapping'
 ? 'bg-[color-mix(in_oklch,var(--accent)_16%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
        >
          {{ t('admin.accounts.modelMapping') }}
        </button>
      </div>

      <!-- Whitelist Mode -->
      <div v-if="modelRestrictionMode === 'whitelist'">
        <ModelWhitelistSelector v-model="allowedModels" platform="anthropic" />
        <p class="text-xs text-muted">
          {{ t('admin.accounts.selectedModels', { count: allowedModels.length }) }}
          <span v-if="allowedModels.length === 0 && modelMappings.length === 0">{{ t('admin.accounts.supportsAllModels') }}</span>
        </p>
      </div>

      <!-- Mapping Mode -->
      <div v-else class="space-y-3">
        <div v-for="(mapping, index) in modelMappings" :key="getModelMappingKey(mapping)" class="flex items-center gap-2">
          <input v-model="mapping.from" type="text" class="input flex-1" :placeholder="t('admin.accounts.fromModel')" />
          <span class="text-muted">→</span>
          <input v-model="mapping.to" type="text" class="input flex-1" :placeholder="t('admin.accounts.toModel')" />
          <button type="button" @click="modelMappings.splice(index, 1)" class="text-danger-text hover:text-danger-text">
            <Icon name="trash" size="sm" />
          </button>
        </div>
        <button type="button" @click="modelMappings.push({ from: '', to: '' })" class="btn btn-secondary text-sm">
          + {{ t('admin.accounts.addMapping') }}
        </button>
        <!-- Bedrock Preset Mappings -->
        <div class="flex flex-wrap gap-2">
          <button
            v-for="preset in bedrockPresets"
            :key="preset.from"
            type="button"
            @click="modelMappings.push({ from: preset.from, to: preset.to })"
            :class="['rounded-lg px-3 py-1 text-xs transition-colors', preset.color]"
          >
            + {{ preset.label }}
          </button>
        </div>
      </div>
    </div>

    <!-- Pool Mode Section for Bedrock -->
    <PoolModeSection
      v-model:pool-mode-enabled="poolModeEnabled"
      v-model:pool-mode-retry-count="poolModeRetryCount"
      v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import PoolModeSection from '@/components/account/shared/PoolModeSection.vue'
import type { getPresetMappingsByPlatform } from '@/composables/useModelWhitelist'

interface ModelMapping {
  from: string
  to: string
}

interface Props {
  isBedrockApiKeyMode: boolean
  bedrockPresets: ReturnType<typeof getPresetMappingsByPlatform>
  getModelMappingKey: (mapping: ModelMapping) => string | number
}

defineProps<Props>()

const editBedrockAccessKeyId = defineModel<string>('editBedrockAccessKeyId', { required: true })
const editBedrockSecretAccessKey = defineModel<string>('editBedrockSecretAccessKey', { required: true })
const editBedrockSessionToken = defineModel<string>('editBedrockSessionToken', { required: true })
const editBedrockApiKeyValue = defineModel<string>('editBedrockApiKeyValue', { required: true })
const editBedrockRegion = defineModel<string>('editBedrockRegion', { required: true })
const editBedrockForceGlobal = defineModel<boolean>('editBedrockForceGlobal', { required: true })
const modelRestrictionMode = defineModel<'whitelist' | 'mapping'>('modelRestrictionMode', { required: true })
const allowedModels = defineModel<string[]>('allowedModels', { required: true })
const modelMappings = defineModel<ModelMapping[]>('modelMappings', { required: true })
const poolModeEnabled = defineModel<boolean>('poolModeEnabled', { required: true })
const poolModeRetryCount = defineModel<number>('poolModeRetryCount', { required: true })
const poolModeRetryStatusCodesInput = defineModel<string>('poolModeRetryStatusCodesInput', { required: true })

const { t } = useI18n()
</script>
