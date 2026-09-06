<template>
  <!-- Temp Unschedulable Rules -->
  <div class="border-t border-line pt-4 space-y-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.tempUnschedulable.title') }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.tempUnschedulable.hint') }}
        </p>
      </div>
      <InlineToggleSwitch v-model="tempUnschedEnabled" />
    </div>

    <div v-if="tempUnschedEnabled" class="space-y-3">
      <div class="rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-3">
        <p class="text-xs text-accent">
          <Icon name="exclamationTriangle" size="sm" class="mr-1 inline" :stroke-width="2" />
          {{ t('admin.accounts.tempUnschedulable.notice') }}
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <button
          v-for="preset in tempUnschedPresets"
          :key="preset.label"
          type="button"
          @click="addTempUnschedRule(preset.rule)"
          class="rounded-lg bg-surface-2 px-3 py-1.5 text-xs font-medium text-muted transition-colors hover:bg-surface-3"
        >
          + {{ preset.label }}
        </button>
      </div>

      <div v-if="tempUnschedRules.length > 0" class="space-y-3">
        <div
          v-for="(rule, index) in tempUnschedRules"
          :key="getTempUnschedRuleKey(rule)"
          class="rounded-lg border border-line p-3"
        >
          <div class="mb-2 flex items-center justify-between">
            <span class="text-xs font-medium text-muted">
              {{ t('admin.accounts.tempUnschedulable.ruleIndex', { index: index + 1 }) }}
            </span>
            <div class="flex items-center gap-2">
              <button
                type="button"
                :disabled="index === 0"
                @click="moveTempUnschedRule(index, -1)"
                class="rounded p-1 text-muted transition-colors hover:text-muted disabled:cursor-not-allowed disabled:opacity-40"
              >
                <Icon name="chevronUp" size="sm" :stroke-width="2" />
              </button>
              <button
                type="button"
                :disabled="index === tempUnschedRules.length - 1"
                @click="moveTempUnschedRule(index, 1)"
                class="rounded p-1 text-muted transition-colors hover:text-muted disabled:cursor-not-allowed disabled:opacity-40"
              >
                <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7" />
                </svg>
              </button>
              <button
                type="button"
                @click="removeTempUnschedRule(index)"
                class="rounded p-1 text-danger-text transition-colors hover:text-danger-text"
              >
                <Icon name="x" size="sm" :stroke-width="2" />
              </button>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.errorCode') }}</label>
              <input
                v-model.number="rule.error_code"
                type="number"
                min="100"
                max="599"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.errorCodePlaceholder')"
              />
            </div>
            <div>
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.durationMinutes') }}</label>
              <input
                v-model.number="rule.duration_minutes"
                type="number"
                min="1"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.durationPlaceholder')"
              />
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.keywords') }}</label>
              <input
                v-model="rule.keywords"
                type="text"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.keywordsPlaceholder')"
              />
              <p class="input-hint">{{ t('admin.accounts.tempUnschedulable.keywordsHint') }}</p>
            </div>
            <div class="sm:col-span-2">
              <label class="input-label">{{ t('admin.accounts.tempUnschedulable.description') }}</label>
              <input
                v-model="rule.description"
                type="text"
                class="input"
                :placeholder="t('admin.accounts.tempUnschedulable.descriptionPlaceholder')"
              />
            </div>
          </div>
        </div>
      </div>

      <button
        type="button"
        @click="addTempUnschedRule()"
        class="w-full rounded-lg border-2 border-dashed border-line px-4 py-2 text-sm text-muted transition-colors hover:border-line hover:text-foreground"
      >
        <svg
          class="mr-1 inline h-4 w-4"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
        </svg>
        {{ t('admin.accounts.tempUnschedulable.addRule') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared between Create/Edit: the temp-unschedulable-rules editor markup,
// which was byte-identical (aside from indentation) between both hosts. All
// state/logic is supplied by the useTempUnschedRules composable (see
// shared/useTempUnschedRules.ts) via defineModel + plain props/functions.
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'
import type { TempUnschedRuleForm } from './useTempUnschedRules'

const { t } = useI18n()

const tempUnschedEnabled = defineModel<boolean>('tempUnschedEnabled', { required: true })
const tempUnschedRules = defineModel<TempUnschedRuleForm[]>('tempUnschedRules', { required: true })

defineProps<{
  tempUnschedPresets: Array<{ label: string; rule: TempUnschedRuleForm }>
  getTempUnschedRuleKey: (rule: TempUnschedRuleForm) => string | number
  addTempUnschedRule: (preset?: TempUnschedRuleForm) => void
  removeTempUnschedRule: (index: number) => void
  moveTempUnschedRule: (index: number, direction: number) => void
}>()
</script>
