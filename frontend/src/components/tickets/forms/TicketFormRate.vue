<template>
  <div class="space-y-4">
    <div>
      <label class="input-label">{{ t('tickets.fields.targetGroups') }}</label>
      <div class="space-y-2 rounded-xl border border-line p-3 dark:border-dark-700">
        <label
          v-for="group in rateEligibleGroups"
          :key="group.id"
          class="flex items-start gap-3 rounded-lg px-3 py-2 transition-colors"
          :class="readonly ? 'cursor-default' : 'cursor-pointer hover:bg-page dark:hover:bg-dark-700/40'"
        >
          <input
            type="checkbox"
            class="mt-1"
            :disabled="readonly"
            :checked="selectedGroupIds.includes(group.id)"
            @change="toggleGroup(group, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span class="font-medium text-ink dark:text-white">{{ group.name }}</span>
              <span class="text-xs text-ink-soft dark:text-dark-400">
                {{ t('tickets.fields.baseRate') }}: {{ formatMultiplier(group.rate_multiplier) }}
              </span>
              <span class="text-xs text-ink-soft dark:text-dark-400">
                {{ t('tickets.fields.specialRate') }}: {{ formatMultiplier(userGroupRates[group.id]) }}
              </span>
              <span class="text-xs text-accent-600 dark:text-blue-300">
                {{ t('tickets.fields.effectiveRate') }}: {{ formatMultiplier(effectiveRate(group.id, group.rate_multiplier)) }}
              </span>
            </div>
          </div>
        </label>
        <p v-if="rateEligibleGroups.length === 0" class="text-sm text-ink-soft dark:text-dark-400">
          {{ t('tickets.emptyAvailableGroups') }}
        </p>
      </div>
    </div>

    <div v-if="selectedSummaries.length > 0" class="space-y-2 rounded-xl border border-line p-3 dark:border-dark-700">
      <p class="text-sm font-medium text-ink dark:text-white">{{ t('tickets.fields.currentRate') }}</p>
      <div
        v-for="summary in selectedSummaries"
        :key="summary.group_id"
        class="grid gap-2 rounded-lg bg-page px-3 py-2 text-sm dark:bg-dark-700/40 md:grid-cols-[minmax(0,1.1fr)_repeat(3,minmax(0,0.7fr))]"
      >
        <div class="font-medium text-ink dark:text-white">{{ summary.group_name }}</div>
        <div class="text-ink-soft dark:text-dark-400">{{ t('tickets.fields.baseRate') }}: {{ formatMultiplier(summary.base_rate) }}</div>
        <div class="text-ink-soft dark:text-dark-400">{{ t('tickets.fields.specialRate') }}: {{ formatMultiplier(summary.special_rate) }}</div>
        <div class="text-accent-600 dark:text-blue-300">{{ t('tickets.fields.effectiveRate') }}: {{ formatMultiplier(summary.effective_rate) }}</div>
      </div>
    </div>

    <div v-if="legacyCurrentRate" class="rounded-xl border border-line p-3 text-sm dark:border-dark-700">
      <p class="text-xs font-medium uppercase tracking-wide text-ink-soft dark:text-dark-400">{{ t('tickets.fields.currentRate') }}</p>
      <p class="mt-1 font-medium text-ink dark:text-white">{{ legacyCurrentRate }}</p>
    </div>

    <div v-if="legacyTargetScope" class="rounded-xl border border-line p-3 text-sm dark:border-dark-700">
      <p class="text-xs font-medium uppercase tracking-wide text-ink-soft dark:text-dark-400">{{ t('tickets.fields.targetScope') }}</p>
      <p class="mt-1 font-medium text-ink dark:text-white">{{ legacyTargetScope }}</p>
    </div>

    <div>
      <div>
        <label class="input-label">{{ t('tickets.fields.targetRate') }}</label>
        <input :value="stringValue('target_rate')" :readonly="readonly" class="input" @input="updateField('target_rate', ($event.target as HTMLInputElement).value)" />
      </div>
    </div>

    <div>
      <label class="input-label">{{ t('tickets.fields.usageScenario') }}</label>
      <textarea :value="stringValue('usage_scenario')" :readonly="readonly" class="input min-h-[120px]" @input="updateField('usage_scenario', ($event.target as HTMLTextAreaElement).value)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group } from '@/types'

interface RateSummary {
  group_id: number
  group_name: string
  base_rate: number
  special_rate: number | null
  effective_rate: number
}

const props = withDefaults(defineProps<{
  modelValue: Record<string, unknown>
  readonly?: boolean
  availableGroups?: Group[]
  userGroupRates?: Record<number, number>
}>(), {
  readonly: false,
  availableGroups: () => [],
  userGroupRates: () => ({}),
})

const emit = defineEmits<{ 'update:modelValue': [value: Record<string, unknown>] }>()
const { t } = useI18n()
const rateEligibleGroups = computed(() =>
  props.availableGroups.filter((group) => group.subscription_type === 'standard'),
)
const visibleGroupIDs = computed(() => new Set(rateEligibleGroups.value.map((group) => group.id)))

function normalizeGroupIDs(raw: unknown) {
  if (!Array.isArray(raw)) return []
  return Array.from(new Set(
    raw
      .map((item) => Number(item))
      .filter((item) => Number.isFinite(item) && item > 0),
  ))
}

const selectedGroupIds = computed<number[]>(() => {
  return normalizeGroupIDs(props.modelValue?.group_ids)
    .filter((item) => visibleGroupIDs.value.has(item))
})

function buildSummaries(groupIDs: number[]): RateSummary[] {
  return rateEligibleGroups.value
    .filter((group) => groupIDs.includes(group.id))
    .map((group) => ({
      group_id: group.id,
      group_name: group.name,
      base_rate: group.rate_multiplier,
      special_rate: props.userGroupRates[group.id] ?? null,
      effective_rate: effectiveRate(group.id, group.rate_multiplier),
    }))
}

const selectedSummaries = computed<RateSummary[]>(() => buildSummaries(selectedGroupIds.value))

const legacyCurrentRate = computed(() => {
  const value = String(props.modelValue?.current_rate ?? '').trim()
  return value
})

const legacyTargetScope = computed(() => {
  const value = String(props.modelValue?.target_scope ?? '').trim()
  return value
})

function stringValue(key: string) {
  return String(props.modelValue?.[key] ?? '')
}

function updateField(key: string, value: string) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

function formatMultiplier(value: number | null | undefined) {
  if (value == null || Number.isNaN(Number(value))) {
    return t('tickets.fields.none')
  }
  return `${Number(value)}x`
}

function effectiveRate(groupID: number, baseRate: number) {
  const specialRate = props.userGroupRates[groupID]
  return specialRate ?? baseRate
}

function normalizeSummaries(raw: unknown): RateSummary[] {
  if (!Array.isArray(raw)) return []
  return raw
    .map((item) => {
      const record = item as Record<string, unknown>
      const groupID = Number(record.group_id)
      const baseRate = Number(record.base_rate)
      const effectiveRateValue = Number(record.effective_rate)
      const specialRateValue = record.special_rate
      return {
        group_id: groupID,
        group_name: String(record.group_name ?? ''),
        base_rate: Number.isFinite(baseRate) ? baseRate : 0,
        special_rate: specialRateValue == null || specialRateValue === '' ? null : Number(specialRateValue),
        effective_rate: Number.isFinite(effectiveRateValue) ? effectiveRateValue : 0,
      }
    })
    .filter((item) => item.group_id > 0 && visibleGroupIDs.value.has(item.group_id))
}

function summariesEqual(left: RateSummary[], right: RateSummary[]) {
  if (left.length !== right.length) return false
  return left.every((item, index) => {
    const other = right[index]
    return item.group_id === other.group_id
      && item.group_name === other.group_name
      && item.base_rate === other.base_rate
      && item.special_rate === other.special_rate
      && item.effective_rate === other.effective_rate
  })
}

function syncVisibleSelections() {
  const nextIDs = selectedGroupIds.value
  const currentIDs = normalizeGroupIDs(props.modelValue?.group_ids)
  const nextSummaries = buildSummaries(nextIDs)
  const currentSummaries = normalizeSummaries(props.modelValue?.current_group_rates)

  if (rateEligibleGroups.value.length === 0 && (currentIDs.length > 0 || currentSummaries.length > 0)) {
    return
  }

  if (currentIDs.length === nextIDs.length
    && currentIDs.every((id, index) => id === nextIDs[index])
    && summariesEqual(currentSummaries, nextSummaries)) {
    return
  }

  emit('update:modelValue', {
    ...props.modelValue,
    group_ids: nextIDs,
    current_group_rates: nextSummaries,
  })
}

function toggleGroup(group: Group, checked: boolean) {
  const nextIDs = checked
    ? Array.from(new Set([...selectedGroupIds.value, group.id]))
    : selectedGroupIds.value.filter((id) => id !== group.id)

  emit('update:modelValue', {
    ...props.modelValue,
    group_ids: nextIDs,
    current_group_rates: buildSummaries(nextIDs),
  })
}

watch(
  () => [props.modelValue?.group_ids, props.modelValue?.current_group_rates, props.availableGroups, props.userGroupRates],
  () => {
    syncVisibleSelections()
  },
  { deep: true, immediate: true },
)
</script>
