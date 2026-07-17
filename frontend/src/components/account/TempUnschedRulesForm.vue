<template>
  <div class="space-y-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.tempUnschedulable.title') }}</label>
        <p class="mt-1 text-xs text-ink-soft dark:text-ink-soft">
          {{ t('admin.accounts.tempUnschedulable.hint') }}
        </p>
      </div>
      <Toggle :model-value="enabled" @update:model-value="updateEnabled" />
    </div>

    <div v-if="enabled" class="space-y-3">
      <div class="rounded-control bg-accent-50 p-3 dark:bg-accent-900/20">
        <p class="text-xs text-accent-700 dark:text-accent-400">
          <Icon name="exclamationTriangle" size="sm" class="mr-1 inline" :stroke-width="2" />
          {{ t('admin.accounts.tempUnschedulable.notice') }}
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <button
          v-for="preset in presets"
          :key="preset.label"
          type="button"
          @click="addRule(preset.rule)"
          class="rounded-control bg-page px-3 py-1.5 text-xs font-medium text-ink-body transition-colors hover:bg-line dark:bg-dark-600 dark:text-ink-body dark:hover:bg-dark-500"
        >
          + {{ preset.label }}
        </button>
      </div>

      <div v-if="rules.length > 0" class="space-y-3">
        <div
          v-for="(rule, index) in rules"
          :key="getRuleKey(rule)"
          class="rounded-control border border-line p-3 dark:border-dark-600"
        >
          <div class="mb-2 flex items-center justify-between">
            <span class="text-xs font-medium text-ink-soft dark:text-ink-soft">
              {{ t('admin.accounts.tempUnschedulable.ruleIndex', { index: index + 1 }) }}
            </span>
            <div class="flex items-center gap-2">
              <button
                type="button"
                :disabled="index === 0"
                @click="moveRule(index, -1)"
                class="rounded p-1 text-ink-faint transition-colors hover:text-ink-body disabled:cursor-not-allowed disabled:opacity-40 dark:hover:text-ink"
              >
                <Icon name="chevronUp" size="sm" :stroke-width="2" />
              </button>
              <button
                type="button"
                :disabled="index === rules.length - 1"
                @click="moveRule(index, 1)"
                class="rounded p-1 text-ink-faint transition-colors hover:text-ink-body disabled:cursor-not-allowed disabled:opacity-40 dark:hover:text-ink"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </button>
              <button
                type="button"
                @click="removeRule(index)"
                class="rounded p-1 text-danger transition-colors hover:text-danger/80"
              >
                <Icon name="x" size="sm" :stroke-width="2" />
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.errorCode') }}</label>
              <input
                :value="rule.error_code ?? ''"
                type="number"
                min="100"
                max="599"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.errorCodePlaceholder')"
                @input="updateRuleField(index, 'error_code', numberInputValue($event))"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.durationMinutes') }}</label>
              <input
                :value="rule.duration_minutes ?? ''"
                type="number"
                min="1"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.durationPlaceholder')"
                @input="updateRuleField(index, 'duration_minutes', numberInputValue($event))"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.keywords') }}</label>
              <input
                :value="rule.keywords"
                type="text"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.keywordsPlaceholder')"
                @input="updateRuleField(index, 'keywords', textInputValue($event))"
              />
              <p class="input-hint">{{ t('admin.accounts.tempUnschedulable.keywordsHint') }}</p>
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.description') }}</label>
              <input
                :value="rule.description"
                type="text"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.descriptionPlaceholder')"
                @input="updateRuleField(index, 'description', textInputValue($event))"
              />
            </div>
          </div>
        </div>
      </div>

      <button
        type="button"
        @click="addRule()"
        class="w-full rounded-control border-2 border-dashed border-line px-4 py-2 text-sm text-ink-body transition-colors hover:border-ink-faint hover:text-ink-body dark:border-dark-500 dark:text-ink-soft dark:hover:border-dark-400 dark:hover:text-ink-faint"
      >
        <svg class="mr-1 inline h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('admin.accounts.tempUnschedulable.addRule') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { createStableObjectKeyResolver } from '@/utils/stableObjectKey'
import type { TempUnschedRuleForm } from './tempUnschedRules'

interface TempUnschedPreset {
  label: string
  rule: {
    error_code: number
    keywords: string
    duration_minutes: number
    description: string
  }
}

const props = defineProps<{
  enabled: boolean
  rules: TempUnschedRuleForm[]
  presets?: TempUnschedPreset[]
}>()

const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:rules': [value: TempUnschedRuleForm[]]
}>()

const { t } = useI18n()

const presets = computed<TempUnschedPreset[]>(() => props.presets ?? defaultPresets())

const getRuleKey = createStableObjectKeyResolver<TempUnschedRuleForm>('temp-unsched-rule-form')

const updateEnabled = (value: boolean) => {
  emit('update:enabled', value)
}

const addRule = (preset?: TempUnschedPreset['rule']) => {
  const next = [...props.rules]
  if (preset) {
    next.push({
      error_code: preset.error_code,
      keywords: preset.keywords,
      duration_minutes: preset.duration_minutes,
      description: preset.description
    })
  } else {
    next.push({ error_code: null, keywords: '', duration_minutes: 30, description: '' })
  }
  emit('update:rules', next)
}

const removeRule = (index: number) => {
  const next = [...props.rules]
  next.splice(index, 1)
  emit('update:rules', next)
}

const moveRule = (index: number, direction: number) => {
  const target = index + direction
  if (target < 0 || target >= props.rules.length) return
  const next = [...props.rules]
  const current = next[index]
  next[index] = next[target]
  next[target] = current
  emit('update:rules', next)
}

const cloneRules = (): TempUnschedRuleForm[] => props.rules.map((rule) => ({ ...rule }))

const updateRuleField = <K extends keyof TempUnschedRuleForm>(
  index: number,
  key: K,
  value: TempUnschedRuleForm[K]
) => {
  const next = cloneRules()
  if (!next[index]) return
  next[index] = { ...next[index], [key]: value }
  emit('update:rules', next)
}

const numberInputValue = (event: Event): number | null => {
  const raw = (event.target as HTMLInputElement).value
  if (raw.trim() === '') return null
  const value = Number(raw)
  return Number.isFinite(value) ? value : null
}

const textInputValue = (event: Event): string => (event.target as HTMLInputElement).value

function defaultPresets(): TempUnschedPreset[] {
  return [
    {
      label: t('admin.accounts.tempUnschedulable.presets.overloadLabel'),
      rule: { error_code: 529, keywords: 'overloaded, too many', duration_minutes: 60, description: t('admin.accounts.tempUnschedulable.presets.overloadDesc') }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.rateLimitLabel'),
      rule: { error_code: 429, keywords: 'rate limit, too many requests', duration_minutes: 10, description: t('admin.accounts.tempUnschedulable.presets.rateLimitDesc') }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.unavailableLabel'),
      rule: { error_code: 503, keywords: 'unavailable, maintenance', duration_minutes: 30, description: t('admin.accounts.tempUnschedulable.presets.unavailableDesc') }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamUnavailableLabel'),
      rule: { error_code: 502, keywords: 'Upstream service temporarily unavailable', duration_minutes: 10, description: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamUnavailableDesc') }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamFailedLabel'),
      rule: { error_code: 502, keywords: 'Upstream request failed', duration_minutes: 10, description: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamFailedDesc') }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamForbiddenLabel'),
      rule: { error_code: 502, keywords: 'Upstream access forbidden', duration_minutes: 10, description: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamForbiddenDesc') }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.openaiServiceUnavailableLabel'),
      rule: { error_code: 503, keywords: 'Service temporarily unavailable, overloaded', duration_minutes: 10, description: t('admin.accounts.tempUnschedulable.presets.openaiServiceUnavailableDesc') }
    },
    {
      label: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamConnLabel'),
      rule: { error_code: 500, keywords: 'upstream connection failed, Upstream transport error', duration_minutes: 10, description: t('admin.accounts.tempUnschedulable.presets.openaiUpstreamConnDesc') }
    }
  ]
}
</script>
