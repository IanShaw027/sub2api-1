<template>
  <div>
    <label class="input-label">{{ t('admin.accounts.accountType') }}</label>
    <div class="mt-2 grid grid-cols-2 gap-3">
      <button
        type="button"
        @click="accountType = 'oauth'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountType === 'oauth'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountType === 'oauth'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="key" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">OAuth</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.types.antigravityOauth') }}</span>
        </div>
      </button>

      <button
        type="button"
        @click="accountType = 'upstream'"
        :class="[
 'flex items-center gap-3 rounded-lg border-2 p-3 text-left transition-all',
 accountType === 'upstream'
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_10%,transparent)]'
 : 'border-line hover:border-[color-mix(in_oklch,var(--accent)_45%,transparent)]'
 ]"
      >
        <div
          :class="[
 'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg',
 accountType === 'upstream'
 ? 'bg-accent text-white'
 : 'bg-surface-2 text-muted'
 ]"
        >
          <Icon name="cloud" size="sm" />
        </div>
        <div>
          <span class="block text-sm font-medium text-foreground">API Key</span>
          <span class="text-xs text-muted">{{ t('admin.accounts.types.antigravityApikey') }}</span>
        </div>
      </button>
    </div>
  </div>

  <div v-if="accountType === 'oauth'">
    <label class="input-label">{{ t('admin.accounts.antigravityProjectIdLabel') }}</label>
    <input
      v-model="projectId"
      data-testid="antigravity-project-id-input"
      type="text"
      class="input font-mono"
      :placeholder="t('admin.accounts.antigravityProjectIdPlaceholder')"
    />
    <p class="input-hint">{{ t('admin.accounts.antigravityProjectIdHint') }}</p>
  </div>

  <!-- Upstream config (only for Antigravity upstream type) -->
  <div v-if="accountType === 'upstream'" class="space-y-4">
    <div>
      <label class="input-label">{{ t('admin.accounts.upstream.baseUrl') }}</label>
      <input
        v-model="upstreamBaseUrl"
        type="text"
        required
        class="input"
        placeholder="https://cloudcode-pa.googleapis.com"
      />
      <p class="input-hint">{{ t('admin.accounts.upstream.baseUrlHint') }}</p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.upstream.apiKey') }}</label>
      <input
        v-model="upstreamApiKey"
        type="password"
        required
        class="input font-mono"
        placeholder="sk-..."
      />
      <p class="input-hint">{{ t('admin.accounts.upstream.apiKeyHint') }}</p>
    </div>
    <!-- 上游倍率自动探测：antigravity upstream 也是 API-key 账号 -->
    <div class="flex items-center justify-between gap-4 border-t border-line pt-4">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.upstreamBilling.autoProbe') }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.upstreamBilling.autoProbeHint') }}
        </p>
      </div>
      <Toggle
        v-model="upstreamBillingAutoProbeEnabled"
        data-testid="upstream-billing-auto-probe-antigravity"
        :aria-label="t('admin.accounts.upstreamBilling.autoProbe')"
      />
    </div>
  </div>

  <!-- Antigravity model restriction (applies to OAuth + Upstream) -->
  <!-- Antigravity 只支持模型映射模式，不支持白名单模式 -->
  <div class="border-t border-line pt-4">
    <label class="input-label">{{ t('admin.accounts.modelRestriction') }}</label>

    <!-- Mapping Mode Only (no toggle for Antigravity) -->
    <div>
      <div class="mb-3 rounded-lg bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] p-3">
        <p class="text-xs text-accent">
          {{ t('admin.accounts.mapRequestModels') }}
        </p>
      </div>

      <div v-if="modelMappings.length > 0" class="mb-3 space-y-2">
        <div
          v-for="(mapping, index) in modelMappings"
          :key="index"
          class="space-y-1"
        >
          <div class="flex items-center gap-2">
            <input
              v-model="mapping.from"
              type="text"
              :class="[
 'input flex-1',
 !isValidWildcardPattern(mapping.from) ? 'border-danger-text' : ''
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
        @click="addModelMapping"
        class="mb-3 w-full rounded-lg border-2 border-dashed border-line px-4 py-2 text-muted transition-colors hover:border-line hover:text-foreground"
      >
        <svg class="mr-1 inline h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('admin.accounts.addMapping') }}
      </button>

      <div class="flex flex-wrap gap-2">
        <button
          v-for="preset in presetMappings"
          :key="preset.label"
          type="button"
          @click="addPresetMapping(preset.from, preset.to)"
          :class="['rounded-lg px-3 py-1 text-xs transition-colors', preset.color]"
        >
          + {{ preset.label }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { getPresetMappingsByPlatform, isValidWildcardPattern } from '@/composables/useModelWhitelist'

interface ModelMapping {
  from: string
  to: string
}

const accountType = defineModel<'oauth' | 'upstream'>('accountType', { required: true })
const projectId = defineModel<string>('projectId', { required: true })
const upstreamBaseUrl = defineModel<string>('upstreamBaseUrl', { required: true })
const upstreamApiKey = defineModel<string>('upstreamApiKey', { required: true })
const upstreamBillingAutoProbeEnabled = defineModel<boolean>('upstreamBillingAutoProbeEnabled', {
  required: true
})
const modelMappings = defineModel<ModelMapping[]>('modelMappings', { required: true })

const { t } = useI18n()
const appStore = useAppStore()

const presetMappings = getPresetMappingsByPlatform('antigravity')

const addModelMapping = () => {
  modelMappings.value.push({ from: '', to: '' })
}

const removeModelMapping = (index: number) => {
  modelMappings.value.splice(index, 1)
}

const addPresetMapping = (from: string, to: string) => {
  if (modelMappings.value.some((m) => m.from === from)) {
    appStore.showInfo(t('admin.accounts.mappingExists', { model: from }))
    return
  }
  modelMappings.value.push({ from, to })
}
</script>
