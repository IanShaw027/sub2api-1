<template>
      <!-- Kiro API Key fields -->
      <div
        v-if="account?.platform === 'kiro' && account?.type === 'apikey'"
        class="space-y-4 rounded-lg border border-accent-200 bg-accent-50/60 p-4"
      >
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.accounts.apiKey') }}</label>
            <input
              v-model="editApiKey"
              type="password"
              class="input font-mono"
              autocomplete="new-password"
              data-1p-ignore
              data-lpignore="true"
              data-bwignore="true"
              :placeholder="t('admin.accounts.kiro.apiKeyPlaceholder')"
            />
            <p class="input-hint">{{ t('admin.accounts.leaveEmptyToKeep') }}</p>
          </div>
        </div>
      </div>

      <div
        v-if="account?.platform === 'kiro' && account?.type === 'oauth'"
        class="space-y-4 rounded-lg border border-accent-200 bg-accent-50/60 p-4"
      >
        <div class="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <label class="input-label">{{ t('admin.accounts.kiro.profileArnLabel') }}</label>
            <p class="input-hint">{{ t('admin.accounts.kiro.runtimeManagedHint') }}</p>
          </div>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="discoveringKiroProfiles"
            @click="discoverKiroProfilesForEdit"
          >
            {{ discoveringKiroProfiles ? t('admin.accounts.oauth.generating') : t('admin.accounts.kiro.discoverProfiles') }}
          </button>
        </div>
        <select
          v-if="kiroProfileSelectOptions.length > 0"
          v-model="selectedKiroProfileArnChoice"
          class="input"
        >
          <option :value="kiroProfileChoiceKeep">{{ t('admin.accounts.leaveEmptyToKeep') }}</option>
          <option :value="kiroProfileChoiceAuto">{{ t('admin.accounts.kiro.profileStateAuto') }}</option>
          <option
            v-for="profile in kiroProfileSelectOptions"
            :key="profile.arn"
            :value="profile.arn"
          >
            {{ profile.name }}
          </option>
        </select>
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <span
            class="inline-flex rounded px-1.5 py-0.5 font-medium"
            :class="kiroProfileStatusBadgeClass"
          >
            {{ kiroProfileStatusLabel }}
          </span>
          <span
            v-if="kiroProfilePendingHint"
            class="text-accent-700"
          >
            {{ kiroProfilePendingHint }}
          </span>
        </div>
        <p v-if="currentKiroProfileArn" class="text-xs text-muted">
          {{ t('admin.accounts.kiro.profileArnLabel') }}: {{ currentKiroProfileArn }}
        </p>
        <KiroDiagnosticChips
          :credentials="account?.credentials || {}"
          :extra="account?.extra || {}"
          :include-profile-mode="false"
          chip-class="inline-flex rounded bg-surface/80 px-2 py-1 text-xs text-accent-800"
        />
      </div>

      <div
        v-if="account?.platform === 'kiro'"
        class="border-t border-line pt-4"
      >
        <ModelRestrictionEditor
          v-model:mode="modelRestrictionMode"
          v-model:allowed-models="allowedModels"
          v-model:model-mappings="modelMappings"
          platform="kiro"
          :account-id="account?.id"
          preset-button-class="rounded-lg px-3 py-2 text-sm font-medium transition-colors"
          :presets="presetMappings"
        />
      </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import KiroDiagnosticChips from '@/components/account/KiroDiagnosticChips.vue'
import ModelRestrictionEditor from '@/components/account/ModelRestrictionEditor.vue'
import type { getPresetMappingsByPlatform } from '@/composables/useModelWhitelist'
import type { Account } from '@/types'

interface ModelMapping {
  from: string
  to: string
}

interface Props {
  account: Account | null
  discoveringKiroProfiles: boolean
  discoverKiroProfilesForEdit: () => void | Promise<void>
  kiroProfileSelectOptions: Array<{ arn: string; name: string }>
  kiroProfileChoiceKeep: string
  kiroProfileChoiceAuto: string
  kiroProfileStatusBadgeClass: string
  kiroProfileStatusLabel: string
  kiroProfilePendingHint: string
  currentKiroProfileArn: string
  presetMappings: ReturnType<typeof getPresetMappingsByPlatform>
}

defineProps<Props>()

const editApiKey = defineModel<string>('editApiKey', { required: true })
const selectedKiroProfileArnChoice = defineModel<string>('selectedKiroProfileArnChoice', { required: true })
const modelRestrictionMode = defineModel<'whitelist' | 'mapping'>('modelRestrictionMode', { required: true })
const allowedModels = defineModel<string[]>('allowedModels', { required: true })
const modelMappings = defineModel<ModelMapping[]>('modelMappings', { required: true })

const { t } = useI18n()
</script>
