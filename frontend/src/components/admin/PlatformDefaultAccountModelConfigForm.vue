<template>
  <div class="space-y-4">
    <!-- Platform tabs -->
    <div class="flex flex-wrap gap-2">
      <button
        v-for="platform in platforms"
        :key="platform"
        type="button"
        :data-testid="`platform-default-tab-${platform}`"
        @click="activePlatform = platform"
        :class="[
          'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
          activePlatform === platform
            ? 'bg-primary-600 text-white'
            : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500'
        ]"
      >
        {{ platformLabel(platform) }}
        <span v-if="platformConfigured(platform)" class="ml-1 inline-block h-2 w-2 rounded-full bg-green-400" />
      </button>
    </div>

    <div class="space-y-5 rounded-lg border border-gray-200 p-4 dark:border-dark-600">
      <!-- Model whitelist -->
      <div>
        <label class="input-label">{{ t('admin.settings.platformDefaults.modelWhitelist') }}</label>
        <ModelWhitelistSelector
          :model-value="current.model_whitelist"
          :platform="activePlatform"
          @update:model-value="updateField('model_whitelist', $event)"
        />
      </div>

      <!-- Model mapping -->
      <ModelMappingEditor
        :model-value="current.model_mapping"
        :label="t('admin.settings.platformDefaults.modelMapping')"
        @update:model-value="updateField('model_mapping', $event)"
      />

      <!-- Compact model mapping -->
      <ModelMappingEditor
        :model-value="current.compact_model_mapping"
        :label="t('admin.settings.platformDefaults.compactModelMapping')"
        @update:model-value="updateField('compact_model_mapping', $event)"
      />

      <!-- Kiro subscription type defaults -->
      <div v-if="activePlatform === 'kiro'" class="border-t border-gray-200 pt-4 dark:border-dark-600">
        <label class="input-label">{{ t('admin.settings.platformDefaults.kiroSubscriptionTypeConfig') }}</label>
        <textarea
          :value="kiroSubscriptionTypeConfigText"
          data-testid="kiro-subscription-type-config"
          rows="8"
          class="input font-mono text-sm"
          spellcheck="false"
          :placeholder="kiroSubscriptionTypeConfigPlaceholder"
          @input="updateKiroSubscriptionTypeConfigText"
        />
        <p class="input-hint">{{ t('admin.settings.platformDefaults.kiroSubscriptionTypeConfigHint') }}</p>
        <p v-if="kiroSubscriptionTypeConfigError" class="mt-1 text-xs text-red-600 dark:text-red-400">
          {{ kiroSubscriptionTypeConfigError }}
        </p>
      </div>

      <!-- Temp unschedulable rules -->
      <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
        <TempUnschedRulesForm
          :enabled="current.temp_unschedulable_enabled"
          :rules="tempUnschedForms"
          @update:enabled="updateField('temp_unschedulable_enabled', $event)"
          @update:rules="updateTempUnschedRules"
        />
      </div>

      <!-- Custom error codes -->
      <div class="border-t border-gray-200 pt-4 dark:border-dark-600">
        <CustomErrorCodesForm
          :enabled="current.custom_error_codes_enabled"
          :codes="current.custom_error_codes"
          @update:enabled="updateField('custom_error_codes_enabled', $event)"
          @update:codes="updateField('custom_error_codes', $event)"
        />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import ModelMappingEditor from '@/components/account/ModelMappingEditor.vue'
import TempUnschedRulesForm from '@/components/account/TempUnschedRulesForm.vue'
import CustomErrorCodesForm from '@/components/account/CustomErrorCodesForm.vue'
import {
  buildTempUnschedRulesValidationPayload,
  loadTempUnschedRules,
  type TempUnschedRuleForm
} from '@/components/account/tempUnschedRules'
import type { DefaultAccountModelConfig } from '@/api/admin/settings'

const props = defineProps<{
  modelValue: Record<string, DefaultAccountModelConfig>
}>()

const emit = defineEmits<{
  'update:modelValue': [value: Record<string, DefaultAccountModelConfig>]
  'validation-error': [value: boolean]
}>()

const { t } = useI18n()

const platforms = ['anthropic', 'openai', 'gemini', 'antigravity', 'kiro', 'grok'] as const

const activePlatform = ref<string>('openai')
const kiroSubscriptionTypeConfigText = ref('{}')
const kiroSubscriptionTypeConfigError = ref('')

const emptyConfig = (): Required<DefaultAccountModelConfig> => ({
  model_whitelist: [],
  model_mapping: {},
  compact_model_mapping: {},
  kiro_subscription_type_model_config: {},
  temp_unschedulable_enabled: false,
  temp_unschedulable_rules: [],
  custom_error_codes_enabled: false,
  custom_error_codes: []
})

const current = computed<Required<DefaultAccountModelConfig>>(() => {
  const cfg = props.modelValue[activePlatform.value]
  return { ...emptyConfig(), ...(cfg ?? {}) }
})

// 临时不可调度规则的持久化形态(payload) <-> 表单形态(逗号字符串 keywords) 的转换
const tempUnschedForms = computed<TempUnschedRuleForm[]>(() =>
  loadTempUnschedRules({ temp_unschedulable_rules: current.value.temp_unschedulable_rules })
)

const platformLabel = (platform: string): string => {
  const key = `admin.settings.platformDefaults.platform.${platform}`
  const label = t(key)
  return label === key ? platform : label
}

const platformConfigured = (platform: string): boolean => {
  const cfg = props.modelValue[platform]
  if (!cfg) return false
  return (
    (cfg.model_whitelist?.length ?? 0) > 0 ||
    Object.keys(cfg.model_mapping ?? {}).length > 0 ||
    Object.keys(cfg.compact_model_mapping ?? {}).length > 0 ||
    (cfg.temp_unschedulable_rules?.length ?? 0) > 0 ||
    cfg.temp_unschedulable_enabled === true ||
    (cfg.custom_error_codes?.length ?? 0) > 0 ||
    cfg.custom_error_codes_enabled === true ||
    Object.keys(cfg.kiro_subscription_type_model_config ?? {}).length > 0
  )
}

const formatJson = (value: unknown): string => JSON.stringify(value ?? {}, null, 2)

const kiroSubscriptionTypeConfigPlaceholder = formatJson({
  pro: {
    model_whitelist: ['claude-sonnet-4-6'],
    model_mapping: {
      'claude-sonnet-*': 'claude-sonnet-4.6'
    },
    compact_model_mapping: {
      'claude-sonnet-*': 'claude-haiku-4.5'
    }
  }
})

const syncKiroSubscriptionTypeConfigText = () => {
  if (activePlatform.value !== 'kiro') return
  if (kiroSubscriptionTypeConfigError.value) return
  kiroSubscriptionTypeConfigText.value = formatJson(current.value.kiro_subscription_type_model_config ?? {})
  kiroSubscriptionTypeConfigError.value = ''
}

const commitPlatform = (next: Required<DefaultAccountModelConfig>) => {
  const out: Record<string, DefaultAccountModelConfig> = { ...props.modelValue }
  // 去除空字段，保持 payload 干净（后端 normalize 也会丢弃空配置）
  const cleaned: DefaultAccountModelConfig = {}
  if (next.model_whitelist.length > 0) cleaned.model_whitelist = next.model_whitelist
  if (Object.keys(next.model_mapping).length > 0) cleaned.model_mapping = next.model_mapping
  if (Object.keys(next.compact_model_mapping).length > 0) cleaned.compact_model_mapping = next.compact_model_mapping
  const kiroSubscriptionConfig = next.kiro_subscription_type_model_config
  if (
    kiroSubscriptionConfig &&
    !Array.isArray(kiroSubscriptionConfig) &&
    typeof kiroSubscriptionConfig === 'object' &&
    Object.keys(kiroSubscriptionConfig).length > 0
  ) {
    cleaned.kiro_subscription_type_model_config = kiroSubscriptionConfig as Record<string, DefaultAccountModelConfig>
  }
  if (next.temp_unschedulable_rules.length > 0) cleaned.temp_unschedulable_rules = next.temp_unschedulable_rules
  if (next.temp_unschedulable_enabled) cleaned.temp_unschedulable_enabled = true
  if (next.custom_error_codes.length > 0) cleaned.custom_error_codes = next.custom_error_codes
  if (next.custom_error_codes_enabled) cleaned.custom_error_codes_enabled = true

  if (Object.keys(cleaned).length === 0) {
    delete out[activePlatform.value]
  } else {
    out[activePlatform.value] = cleaned
  }
  emit('update:modelValue', out)
}

const updateField = <K extends keyof Required<DefaultAccountModelConfig>>(
  key: K,
  value: Required<DefaultAccountModelConfig>[K]
) => {
  commitPlatform({ ...current.value, [key]: value })
}

const updateTempUnschedRules = (forms: TempUnschedRuleForm[]) => {
  commitPlatform({ ...current.value, temp_unschedulable_rules: buildTempUnschedRulesValidationPayload(forms) })
}

const updateKiroSubscriptionTypeConfigText = (event: Event) => {
  const text = (event.target as HTMLTextAreaElement).value
  kiroSubscriptionTypeConfigText.value = text
  const trimmed = text.trim()
  if (!trimmed) {
    kiroSubscriptionTypeConfigError.value = ''
    emit('validation-error', false)
    updateField('kiro_subscription_type_model_config', {})
    return
  }

  try {
    const parsed = JSON.parse(trimmed)
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') {
      throw new Error('root must be an object')
    }
    kiroSubscriptionTypeConfigError.value = ''
    emit('validation-error', false)
    updateField(
      'kiro_subscription_type_model_config',
      parsed as Record<string, DefaultAccountModelConfig>
    )
  } catch (_error) {
    kiroSubscriptionTypeConfigError.value = t('admin.settings.platformDefaults.kiroSubscriptionTypeConfigInvalid')
    emit('validation-error', true)
  }
}

watch(
  () => [
    activePlatform.value,
    current.value.kiro_subscription_type_model_config
  ] as const,
  syncKiroSubscriptionTypeConfigText,
  { immediate: true, deep: true }
)
</script>
