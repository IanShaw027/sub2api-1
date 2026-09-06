<template>
      <!-- API Key fields (only for apikey type) -->
      <div v-if="account?.type === 'apikey' && account?.platform !== 'kiro'" class="space-y-4">
        <div v-if="!isCNApiKeyAccount || editApiProtocol !== 'adaptive'">
          <label class="input-label">{{ t('admin.accounts.baseUrl') }}</label>
          <input
            v-model="editBaseUrl"
            type="text"
            class="input"
            :placeholder="
              account?.platform === 'openai'
                ? 'https://api.openai.com'
                : account?.platform === 'gemini'
                  ? 'https://generativelanguage.googleapis.com'
                  : account?.platform === 'antigravity'
                    ? 'https://cloudcode-pa.googleapis.com'
                    : account?.platform === 'grok'
                      ? 'https://api.x.ai/v1'
                      : 'https://api.anthropic.com'
            "
          />
          <p v-if="baseUrlHint" class="input-hint">{{ baseUrlHint }}</p>
          <GrokBaseUrlPresets
            v-if="account?.platform === 'grok'"
            class="mt-2"
            @select="editBaseUrl = $event"
          />
          <CnBaseUrlPresets
            v-if="isCNApiKeyAccount"
            class="mt-2"
            :platform="cnPresetPlatform"
            :mode="editAccountMode"
            :protocol="editApiProtocol"
            :current-url="editBaseUrl"
            @select="onCnPresetSelect"
          />
        </div>
        <div v-else>
          <label class="input-label">{{ t('admin.accounts.cnProviders.apiProtocol.endpoints') }}</label>
          <div class="mt-2 space-y-3">
            <div v-for="item in editAdaptiveProtocolOptions" :key="item.value">
              <label class="mb-1 block text-xs font-medium text-muted">
                {{ t(`admin.accounts.cnProviders.apiProtocol.${item.labelKey}`) }}
              </label>
              <input v-model="editAdaptiveBaseUrls[item.value]" type="text" class="input" />
            </div>
          </div>
          <p v-if="!cnSupportsNativeResponses(account?.platform)" class="input-hint">
            {{ t('admin.accounts.cnProviders.apiProtocol.responsesFallbackDesc') }}
          </p>
        </div>
        <!-- Account Mode Selection (CN providers) -->
        <div v-if="isCNApiKeyAccount">
          <label class="input-label">{{ t('admin.accounts.cnProviders.accountMode.title') }}</label>
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              v-for="opt in cnAccountModeOptions"
              :key="opt.value"
              type="button"
              :class="[
 'rounded-lg border-2 px-3 py-1.5 text-xs transition-all',
 editAccountMode === opt.value
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] font-medium text-accent'
 : 'border-line text-foreground hover:border-line'
 ]"
              @click="editAccountMode = opt.value"
            >
              {{ t(`admin.accounts.cnProviders.accountMode.${opt.labelKey}`) }}
            </button>
          </div>
          <p class="input-hint">{{ t(`admin.accounts.cnProviders.accountMode.${editAccountMode}Desc`) }}</p>
        </div>
        <!-- API Protocol Selection (CN providers) -->
        <div v-if="isCNApiKeyAccount">
          <label class="input-label">{{ t('admin.accounts.cnProviders.apiProtocol.title') }}</label>
          <div class="mt-2 flex flex-wrap gap-2">
            <button
              v-for="opt in cnProtocolOptions"
              :key="opt.value"
              type="button"
              :class="[
 'rounded-lg border-2 px-3 py-1.5 text-xs transition-all',
 editApiProtocol === opt.value
 ? 'border-accent bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] font-medium text-accent'
 : 'border-line text-foreground hover:border-line'
 ]"
              @click="editApiProtocol = opt.value"
            >
              {{ t(`admin.accounts.cnProviders.apiProtocol.${opt.labelKey}`) }}
            </button>
          </div>
          <p class="input-hint">{{ t(`admin.accounts.cnProviders.apiProtocol.${cnProtocolDescKey}Desc`) }}</p>
        </div>
        <!-- Zhipu 团队版 Coding Plan：组织/项目 ID（可选，填写后用量查询走团队版端点） -->
        <div v-if="account?.platform === 'zhipu' && editAccountMode === 'coding'">
          <div class="flex items-center">
            <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.title') }}</label>
            <HelpTooltip trigger="click" width-class="w-80">
              <p class="mb-1 font-medium">{{ t('admin.accounts.cnProviders.zhipuTeam.help.title') }}</p>
              <ol class="list-decimal space-y-1 pl-4">
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step1') }}</li>
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step2') }}</li>
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step3') }}</li>
                <li>{{ t('admin.accounts.cnProviders.zhipuTeam.help.step4') }}</li>
              </ol>
              <p class="mt-2 break-all rounded bg-black/20 p-1.5 font-mono text-[11px] leading-relaxed">
                {{ t('admin.accounts.cnProviders.zhipuTeam.help.example') }}
              </p>
            </HelpTooltip>
          </div>
          <div class="mt-2 grid gap-4 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.organization') }}</label>
              <input v-model="editZhipuOrganization" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.organizationPlaceholder')" />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.cnProviders.zhipuTeam.project') }}</label>
              <input v-model="editZhipuProject" type="text" class="input" :placeholder="t('admin.accounts.cnProviders.zhipuTeam.projectPlaceholder')" />
            </div>
          </div>
          <p class="input-hint mt-2">{{ t('admin.accounts.cnProviders.zhipuTeam.hint') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('admin.accounts.apiKey') }}</label>
          <input
            v-model="editApiKey"
            type="password"
            class="input font-mono"
            autocomplete="new-password"
            data-1p-ignore
            data-lpignore="true"
            data-bwignore="true"
            :placeholder="
              account?.platform === 'openai'
                ? 'sk-proj-...'
                : account?.platform === 'gemini'
                  ? 'AIza...'
                  : account?.platform === 'antigravity'
                    ? 'sk-...'
                    : account?.platform === 'grok'
                      ? 'xai-...'
                      : 'sk-ant-...'
            "
          />
          <p class="input-hint">{{ t('admin.accounts.leaveEmptyToKeep') }}</p>
        </div>

        <!-- Model Restriction Section (不适用于 Antigravity) -->
        <div v-if="account?.platform !== 'antigravity'" class="border-t border-line pt-4">
          <ModelRestrictionEditor
            v-model:mode="modelRestrictionMode"
            v-model:allowed-models="allowedModels"
            v-model:model-mappings="modelMappings"
            :platform="account?.platform || 'anthropic'"
            :account-id="account?.id"
            :disabled-by-passthrough="isOpenAIModelRestrictionDisabled"
            :show-icons="true"
            :presets="presetMappings"
          />
        </div>

        <!-- Pool Mode Section -->
        <PoolModeSection
          v-model:pool-mode-enabled="poolModeEnabled"
          v-model:pool-mode-retry-count="poolModeRetryCount"
          v-model:pool-mode-retry-status-codes-input="poolModeRetryStatusCodesInput"
        />

        <!-- Custom Error Codes Section -->
        <CustomErrorCodesSection
          v-model:custom-error-codes-enabled="customErrorCodesEnabled"
          v-model:selected-error-codes="selectedErrorCodes"
          v-model:custom-error-code-input="customErrorCodeInput"
        />

      </div>

      <!-- Grok OAuth client-tool prompt cache opt-in -->
      <div
        v-if="account?.platform === 'grok' && account?.type === 'oauth'"
        class="border-t border-line pt-4"
      >
        <div class="flex items-center justify-between gap-4">
          <div class="min-w-0">
            <label class="input-label mb-0">{{ t('admin.accounts.grokClientToolCache.title') }}</label>
            <p class="mt-1 text-xs text-muted">
              {{ t('admin.accounts.grokClientToolCache.hint') }}
            </p>
          </div>
          <Toggle
            v-model="grokClientToolCacheEnabled"
            data-testid="grok-client-tool-cache-toggle"
            :aria-label="t('admin.accounts.grokClientToolCache.title')"
          />
        </div>
      </div>

      <!-- Grok OAuth Custom Upstream URL (仅改写转发端点，OAuth 授权/刷新不受影响) -->
      <GrokCustomBaseUrlSection
        v-if="account?.platform === 'grok' && account?.type === 'oauth'"
        v-model:grokOAuthCustomBaseUrlEnabled="grokOAuthCustomBaseUrlEnabled"
        v-model:grokOAuthBaseUrl="grokOAuthBaseUrl"
      />

      <!-- Header Override Section (eligible API-key platforms + grok OAuth) -->
      <HeaderOverrideSection
        v-if="headerOverrideCapable"
        v-model:headerOverrideEnabled="headerOverrideEnabled"
        v-model:headerOverrideRows="headerOverrideRows"
      />

      <!-- OpenAI/Grok OAuth Model Mapping (OAuth 类型没有 apikey 容器，需要独立的模型映射区域) -->
      <div
        v-if="(account?.platform === 'openai' || account?.platform === 'grok') && account?.type === 'oauth'"
        class="border-t border-line pt-4"
      >
        <ModelRestrictionEditor
          v-model:mode="modelRestrictionMode"
          v-model:allowed-models="allowedModels"
          v-model:model-mappings="modelMappings"
          :platform="account?.platform || 'anthropic'"
          :account-id="account?.id"
          :disabled-by-passthrough="isOpenAIModelRestrictionDisabled"
          :presets="presetMappings"
        />
      </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Toggle from '@/components/common/Toggle.vue'
import PoolModeSection from '@/components/account/shared/PoolModeSection.vue'
import CustomErrorCodesSection from '@/components/account/shared/CustomErrorCodesSection.vue'
import HeaderOverrideSection from '@/components/account/shared/HeaderOverrideSection.vue'
import GrokCustomBaseUrlSection from '@/components/account/shared/GrokCustomBaseUrlSection.vue'
import ModelRestrictionEditor from '@/components/account/ModelRestrictionEditor.vue'
import GrokBaseUrlPresets from '@/components/account/GrokBaseUrlPresets.vue'
import CnBaseUrlPresets from '@/components/account/CnBaseUrlPresets.vue'
import { isHeaderOverrideCapable, type CnAccountMode, type CnApiProtocol, type CnNativeApiProtocol } from '@/components/account/credentialsBuilder'
import { getPresetMappingsByPlatform } from '@/composables/useModelWhitelist'
import type { Account } from '@/types'

interface ModelMapping {
  from: string
  to: string
}

interface Props {
  account: Account | null
  isCNApiKeyAccount: boolean
  baseUrlHint: string
  editAdaptiveProtocolOptions: Array<{ value: CnNativeApiProtocol; labelKey: string }>
  cnPresetPlatform: 'kimi' | 'zhipu' | 'deepseek'
  onCnPresetSelect: (preset: { mode: CnAccountMode; protocol: CnApiProtocol; url: string }) => void
  cnSupportsNativeResponses: (platform: string) => boolean
  cnAccountModeOptions: Array<{ value: CnAccountMode; labelKey: 'payg' | 'coding' }>
  cnProtocolOptions: Array<{ value: CnApiProtocol; labelKey: string }>
  cnProtocolDescKey: string
  isOpenAIModelRestrictionDisabled: boolean
  presetMappings: ReturnType<typeof getPresetMappingsByPlatform>
}

const props = defineProps<Props>()

const editApiProtocol = defineModel<CnApiProtocol>('editApiProtocol', { required: true })
const editBaseUrl = defineModel<string>('editBaseUrl', { required: true })
const editAccountMode = defineModel<CnAccountMode>('editAccountMode', { required: true })
const editAdaptiveBaseUrls = defineModel<Record<CnNativeApiProtocol, string>>('editAdaptiveBaseUrls', { required: true })
const editZhipuOrganization = defineModel<string>('editZhipuOrganization', { required: true })
const editZhipuProject = defineModel<string>('editZhipuProject', { required: true })
const editApiKey = defineModel<string>('editApiKey', { required: true })
const modelRestrictionMode = defineModel<'whitelist' | 'mapping'>('modelRestrictionMode', { required: true })
const allowedModels = defineModel<string[]>('allowedModels', { required: true })
const modelMappings = defineModel<ModelMapping[]>('modelMappings', { required: true })
const poolModeEnabled = defineModel<boolean>('poolModeEnabled', { required: true })
const poolModeRetryCount = defineModel<number>('poolModeRetryCount', { required: true })
const poolModeRetryStatusCodesInput = defineModel<string>('poolModeRetryStatusCodesInput', { required: true })
const customErrorCodesEnabled = defineModel<boolean>('customErrorCodesEnabled', { required: true })
const selectedErrorCodes = defineModel<number[]>('selectedErrorCodes', { required: true })
const grokClientToolCacheEnabled = defineModel<boolean>('grokClientToolCacheEnabled', { required: true })
const grokOAuthCustomBaseUrlEnabled = defineModel<boolean>('grokOAuthCustomBaseUrlEnabled', { required: true })
const grokOAuthBaseUrl = defineModel<string>('grokOAuthBaseUrl', { required: true })
const headerOverrideEnabled = defineModel<boolean>('headerOverrideEnabled', { required: true })
const headerOverrideRows = defineModel<import('@/components/account/credentialsBuilder').HeaderOverrideRow[]>('headerOverrideRows', { required: true })

// Local-only state: not read anywhere else in the host component.
const customErrorCodeInput = ref<number | null>(null)

const headerOverrideCapable = computed(
  () => !!props.account && isHeaderOverrideCapable(props.account.platform, props.account.type)
)

const { t } = useI18n()
</script>
