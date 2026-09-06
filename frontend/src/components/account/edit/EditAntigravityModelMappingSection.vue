<template>
      <div v-if="account?.platform === 'antigravity'" class="border-t border-line pt-4">
        <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>

        <!-- Mapping Mode Only (no toggle for Antigravity) -->
        <div>
          <div class="mb-3 rounded-lg bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] p-3">
            <p class="text-xs text-accent">{{ t('admin.accounts.mapRequestModels') }}</p>
          </div>

          <div class="mb-3 flex flex-wrap gap-2">
            <button
              type="button"
              @click="syncAntigravityUpstreamModels"
              :disabled="isSyncingAntigravityUpstream || !account?.id"
              class="rounded-lg border border-success px-3 py-1.5 text-sm text-success-text hover:bg-[color-mix(in_oklch,var(--success)_16%,transparent)] disabled:cursor-not-allowed disabled:opacity-60"
            >
              {{ isSyncingAntigravityUpstream ? t('admin.accounts.syncUpstreamModelsLoading') : t('admin.accounts.syncUpstreamModels') }}
            </button>
          </div>

          <div v-if="antigravityModelMappings.length > 0" class="mb-3 space-y-2">
            <div
              v-for="(mapping, index) in antigravityModelMappings"
              :key="getAntigravityModelMappingKey(mapping)"
              class="space-y-1"
            >
              <div class="flex items-center gap-2">
                <input
                  v-model="mapping.from"
                  type="text"
                  :class="[
 'input flex-1',
 !isValidWildcardPattern(mapping.from) ? 'border-danger-text' : '',
 mapping.to.includes('*') ? '' : ''
 ]"
                  :placeholder="t('admin.accounts.requestModel')"
                />
                <svg class="h-4 w-4 flex-shrink-0 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3" />
                </svg>
                <input
                  v-model="mapping.to"
                  type="text"
                  :class="[
 'input flex-1',
 mapping.to.includes('*') ? 'border-danger-text' : ''
 ]"
                  :placeholder="t('admin.accounts.actualModel')"
                />
                <button
                  type="button"
                  @click="removeAntigravityModelMapping(index)"
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
              <!-- 校验错误提示 -->
              <p v-if="!isValidWildcardPattern(mapping.from)" class="text-xs text-danger-text">
                {{ t('admin.accounts.wildcardOnlyAtEnd') }}
              </p>
              <p v-if="mapping.to.includes('*')" class="text-xs text-danger-text">
                {{ t('admin.accounts.targetNoWildcard') }}
              </p>
            </div>
          </div>

          <button
            type="button"
            @click="addAntigravityModelMapping"
            class="mb-3 w-full rounded-lg border-2 border-dashed border-line px-4 py-2 text-muted transition-colors hover:border-line hover:text-foreground"
          >
            <svg class="mr-1 inline h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
            </svg>
            {{ t('admin.accounts.addMapping') }}
          </button>

          <div class="flex flex-wrap gap-2">
            <button
              v-for="preset in antigravityPresetMappings"
              :key="preset.label"
              type="button"
              @click="addAntigravityPresetMapping(preset.from, preset.to)"
              :class="['rounded-lg px-3 py-1 text-xs transition-colors', preset.color]"
            >
              + {{ preset.label }}
            </button>
          </div>
        </div>
      </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'
import { getPresetMappingsByPlatform, isValidWildcardPattern } from '@/composables/useModelWhitelist'
import type { Account } from '@/types'

interface ModelMapping {
  from: string
  to: string
}

interface Props {
  account: Account | null
}

const props = defineProps<Props>()

const antigravityModelMappings = defineModel<ModelMapping[]>('antigravityModelMappings', { required: true })

const { t } = useI18n()
const appStore = useAppStore()

const isSyncingAntigravityUpstream = ref(false)
const antigravityPresetMappings = computed(() => getPresetMappingsByPlatform('antigravity'))
const getAntigravityModelMappingKey = createStableObjectKeyResolver<ModelMapping>('edit-antigravity-model-mapping')

const addAntigravityModelMapping = () => {
  antigravityModelMappings.value.push({ from: '', to: '' })
}

const removeAntigravityModelMapping = (index: number) => {
  antigravityModelMappings.value.splice(index, 1)
}

const addAntigravityPresetMapping = (from: string, to: string) => {
  const exists = antigravityModelMappings.value.some((m) => m.from === from)
  if (exists) {
    appStore.showInfo(t('admin.accounts.mappingExists', { model: from }))
    return
  }
  antigravityModelMappings.value.push({ from, to })
}

const syncAntigravityUpstreamModels = async () => {
  if (!props.account?.id || isSyncingAntigravityUpstream.value) return

  isSyncingAntigravityUpstream.value = true
  try {
    const result = await adminAPI.accounts.syncUpstreamModels(props.account.id)
    const upstreamModels = result.models.map((model) => model.trim()).filter(Boolean)
    if (upstreamModels.length === 0) {
      appStore.showInfo(t('admin.accounts.syncUpstreamModelsEmpty'))
      return
    }

    let addedCount = 0
    for (const model of upstreamModels) {
      const exists = antigravityModelMappings.value.some((mapping) => mapping.from === model)
      if (!exists) {
        antigravityModelMappings.value.push({ from: model, to: model })
        addedCount += 1
      }
    }

    if (result.warnings?.some((warning) => warning.code === 'upstream_model_metadata_incomplete')) {
      appStore.showWarning(t('admin.accounts.syncUpstreamModelsMetadataIncomplete'))
      return
    }
    if (addedCount > 0) {
      appStore.showSuccess(t('admin.accounts.syncUpstreamModelsSuccess', { count: addedCount, total: upstreamModels.length }))
    } else {
      appStore.showInfo(t('admin.accounts.syncUpstreamModelsNoChanges', { count: upstreamModels.length }))
    }
  } catch (error) {
    const message = error instanceof Error ? error.message : t('admin.accounts.syncUpstreamModelsFailed')
    appStore.showError(t('admin.accounts.syncUpstreamModelsError', { message }))
  } finally {
    isSyncingAntigravityUpstream.value = false
  }
}
</script>
