<template>
  <div class="space-y-4">
    <!-- Auth Mode Radio -->
    <div>
      <label class="input-label">{{ t('admin.accounts.bedrockAuthMode') }}</label>
      <div class="mt-2 flex gap-4">
        <label class="flex cursor-pointer items-center">
          <input
            v-model="bedrockAuthMode"
            type="radio"
            value="sigv4"
            class="mr-2 text-accent focus:ring-accent"
          />
          <span class="text-sm text-foreground">{{ t('admin.accounts.bedrockAuthModeSigv4') }}</span>
        </label>
        <label class="flex cursor-pointer items-center">
          <input
            v-model="bedrockAuthMode"
            type="radio"
            value="apikey"
            class="mr-2 text-accent focus:ring-accent"
          />
          <span class="text-sm text-foreground">{{ t('admin.accounts.bedrockAuthModeApikey') }}</span>
        </label>
      </div>
    </div>

    <!-- SigV4 fields -->
    <template v-if="bedrockAuthMode === 'sigv4'">
      <div>
        <label class="input-label">{{ t('admin.accounts.bedrockAccessKeyId') }}</label>
        <input
          v-model="bedrockAccessKeyId"
          type="text"
          required
          class="input font-mono"
          placeholder="AKIA..."
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.bedrockSecretAccessKey') }}</label>
        <input
          v-model="bedrockSecretAccessKey"
          type="password"
          required
          class="input font-mono"
        />
      </div>
      <div>
        <label class="input-label">{{ t('admin.accounts.bedrockSessionToken') }}</label>
        <input
          v-model="bedrockSessionToken"
          type="password"
          class="input font-mono"
        />
        <p class="input-hint">{{ t('admin.accounts.bedrockSessionTokenHint') }}</p>
      </div>
    </template>

    <!-- API Key field -->
    <div v-if="bedrockAuthMode === 'apikey'">
      <label class="input-label">{{ t('admin.accounts.bedrockApiKeyInput') }}</label>
      <input
        v-model="bedrockApiKeyValue"
        type="password"
        required
        class="input font-mono"
      />
    </div>

    <!-- Shared: Region -->
    <div>
      <label class="input-label">{{ t('admin.accounts.bedrockRegion') }}</label>
      <select v-model="bedrockRegion" class="input">
        <optgroup label="US">
          <option value="us-east-1">us-east-1 (N. Virginia)</option>
          <option value="us-east-2">us-east-2 (Ohio)</option>
          <option value="us-west-1">us-west-1 (N. California)</option>
          <option value="us-west-2">us-west-2 (Oregon)</option>
          <option value="us-gov-east-1">us-gov-east-1 (GovCloud US-East)</option>
          <option value="us-gov-west-1">us-gov-west-1 (GovCloud US-West)</option>
        </optgroup>
        <optgroup label="Europe">
          <option value="eu-west-1">eu-west-1 (Ireland)</option>
          <option value="eu-west-2">eu-west-2 (London)</option>
          <option value="eu-west-3">eu-west-3 (Paris)</option>
          <option value="eu-central-1">eu-central-1 (Frankfurt)</option>
          <option value="eu-central-2">eu-central-2 (Zurich)</option>
          <option value="eu-south-1">eu-south-1 (Milan)</option>
          <option value="eu-south-2">eu-south-2 (Spain)</option>
          <option value="eu-north-1">eu-north-1 (Stockholm)</option>
        </optgroup>
        <optgroup label="Asia Pacific">
          <option value="ap-northeast-1">ap-northeast-1 (Tokyo)</option>
          <option value="ap-northeast-2">ap-northeast-2 (Seoul)</option>
          <option value="ap-northeast-3">ap-northeast-3 (Osaka)</option>
          <option value="ap-south-1">ap-south-1 (Mumbai)</option>
          <option value="ap-south-2">ap-south-2 (Hyderabad)</option>
          <option value="ap-southeast-1">ap-southeast-1 (Singapore)</option>
          <option value="ap-southeast-2">ap-southeast-2 (Sydney)</option>
        </optgroup>
        <optgroup label="Canada">
          <option value="ca-central-1">ca-central-1 (Canada)</option>
        </optgroup>
        <optgroup label="South America">
          <option value="sa-east-1">sa-east-1 (São Paulo)</option>
        </optgroup>
      </select>
      <p class="input-hint">{{ t('admin.accounts.bedrockRegionHint') }}</p>
    </div>

    <!-- Shared: Force Global -->
    <div>
      <label class="flex items-center gap-2 cursor-pointer">
        <input
          v-model="bedrockForceGlobal"
          type="checkbox"
          class="rounded border-line text-accent focus:ring-accent"
        />
        <span class="text-sm text-foreground">{{ t('admin.accounts.bedrockForceGlobal') }}</span>
      </label>
      <p class="input-hint mt-1">{{ t('admin.accounts.bedrockForceGlobalHint') }}</p>
    </div>

    <!-- Model Restriction Section for Bedrock -->
    <div class="border-t border-line pt-4">
      <ModelRestrictionEditor
        v-model:mode="modelRestrictionMode"
        v-model:allowed-models="allowedModels"
        v-model:model-mappings="modelMappings"
        platform="anthropic"
        :sync-credentials="syncPreviewCredentials"
        :presets="bedrockPresets"
        @upstream-synced="emit('upstream-synced')"
      />
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
import ModelRestrictionEditor from '@/components/account/ModelRestrictionEditor.vue'
import PoolModeSection from '@/components/account/shared/PoolModeSection.vue'
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
  syncPreviewCredentials?: SyncCredentials
  bedrockPresets: ReturnType<typeof getPresetMappingsByPlatform>
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'upstream-synced'): void
}>()

const bedrockAuthMode = defineModel<'sigv4' | 'apikey'>('bedrockAuthMode', { required: true })
const bedrockAccessKeyId = defineModel<string>('bedrockAccessKeyId', { required: true })
const bedrockSecretAccessKey = defineModel<string>('bedrockSecretAccessKey', { required: true })
const bedrockSessionToken = defineModel<string>('bedrockSessionToken', { required: true })
const bedrockApiKeyValue = defineModel<string>('bedrockApiKeyValue', { required: true })
const bedrockRegion = defineModel<string>('bedrockRegion', { required: true })
const bedrockForceGlobal = defineModel<boolean>('bedrockForceGlobal', { required: true })
const modelRestrictionMode = defineModel<'whitelist' | 'mapping'>('modelRestrictionMode', { required: true })
const allowedModels = defineModel<string[]>('allowedModels', { required: true })
const modelMappings = defineModel<ModelMapping[]>('modelMappings', { required: true })
const poolModeEnabled = defineModel<boolean>('poolModeEnabled', { required: true })
const poolModeRetryCount = defineModel<number>('poolModeRetryCount', { required: true })
const poolModeRetryStatusCodesInput = defineModel<string>('poolModeRetryStatusCodesInput', { required: true })

const { t } = useI18n()
</script>
