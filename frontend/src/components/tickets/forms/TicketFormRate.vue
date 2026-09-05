<template>
  <div class="space-y-4">
    <div>
      <label class="input-label">{{ t('tickets.fields.targetGroups') }}</label>
      <div class="space-y-2 rounded-xl border border-line p-3">
        <label
          v-for="group in displayGroups"
          :key="group.group_id"
          class="flex items-start gap-3 rounded-lg px-3 py-2 transition-colors"
          :class="readonly ? 'cursor-default' : 'cursor-pointer hover:bg-surface-2'"
        >
          <input
            type="checkbox"
            class="mt-1"
            :disabled="readonly"
            :checked="selectedGroupIds.includes(group.group_id)"
            @change="toggleGroup(group.group_id, ($event.target as HTMLInputElement).checked)"
          />
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span class="font-medium text-foreground">{{ group.name }}</span>
              <span class="text-xs text-muted">
                {{ t('tickets.fields.baseRate') }}: {{ formatMultiplier(group.base_rate_multiplier) }}
              </span>
              <span v-if="group.user_rate_multiplier != null" class="text-xs text-muted">
                {{ t('tickets.fields.specialRate') }}: {{ formatMultiplier(group.user_rate_multiplier) }}
              </span>
              <span class="text-xs text-accent-600">
                {{ t('tickets.fields.effectiveRate') }}: {{ formatMultiplier(group.effective_rate) }}
              </span>
            </div>
          </div>
        </label>
        <p v-if="displayGroups.length === 0" class="text-sm text-muted">
          {{ t('tickets.emptyAvailableGroups') }}
        </p>
      </div>
    </div>

    <div v-if="legacyCurrentRate" class="rounded-xl border border-line p-3 text-sm">
      <p class="text-xs font-medium uppercase tracking-wide text-muted">{{ t('tickets.fields.currentRate') }}</p>
      <p class="mt-1 font-medium text-foreground">{{ legacyCurrentRate }}</p>
    </div>

    <div v-if="legacyTargetScope" class="rounded-xl border border-line p-3 text-sm">
      <p class="text-xs font-medium uppercase tracking-wide text-muted">{{ t('tickets.fields.targetScope') }}</p>
      <p class="mt-1 font-medium text-foreground">{{ legacyTargetScope }}</p>
    </div>

    <TextInput
      :label="t('tickets.fields.targetRate')"
      :model-value="stringValue('target_rate')"
      :readonly="readonly"
      @update:model-value="updateField('target_rate', String($event))"
    />

    <TextArea
      :label="t('tickets.fields.usageScenario')"
      :model-value="stringValue('usage_scenario')"
      :readonly="readonly"
      :rows="5"
      @update:model-value="updateField('usage_scenario', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import TextInput from '@/components/ui/TextInput.vue'
import TextArea from '@/components/common/TextArea.vue'
import type { TicketRateGroupOption } from '@/types/ticket'

const props = withDefaults(defineProps<{
  modelValue: Record<string, unknown>
  readonly?: boolean
  rateGroups?: TicketRateGroupOption[]
}>(), {
  readonly: false,
  rateGroups: () => [],
})

const emit = defineEmits<{ 'update:modelValue': [value: Record<string, unknown>] }>()
const { t } = useI18n()

function normalizeGroupIDs(raw: unknown) {
  if (!Array.isArray(raw)) return []
  return Array.from(new Set(
    raw
      .map((item) => Number(item))
      .filter((item) => Number.isFinite(item) && item > 0),
  ))
}

function snapshotsFromPayload(): TicketRateGroupOption[] {
  const raw = props.modelValue?.group_snapshots
  if (!Array.isArray(raw)) return []
  return raw
    .map((item) => {
      const record = item as Record<string, unknown>
      const groupID = Number(record.group_id)
      const baseRate = Number(record.base_rate_multiplier ?? record.base_rate)
      const effectiveRate = Number(record.effective_rate)
      const specialRate = record.user_rate_multiplier ?? record.special_rate
      return {
        group_id: groupID,
        name: String(record.name ?? record.group_name ?? ''),
        base_rate_multiplier: Number.isFinite(baseRate) ? baseRate : 0,
        user_rate_multiplier: specialRate == null || specialRate === '' ? undefined : Number(specialRate),
        effective_rate: Number.isFinite(effectiveRate) ? effectiveRate : 0,
      }
    })
    .filter((item) => item.group_id > 0)
}

const displayGroups = computed(() => {
  if (props.readonly) {
    const snapshots = snapshotsFromPayload()
    if (snapshots.length > 0) return snapshots
  }
  return props.rateGroups
})

const selectedGroupIds = computed<number[]>(() => normalizeGroupIDs(props.modelValue?.group_ids))

const legacyCurrentRate = computed(() => String(props.modelValue?.current_rate ?? '').trim())
const legacyTargetScope = computed(() => String(props.modelValue?.target_scope ?? '').trim())

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

function toggleGroup(groupId: number, checked: boolean) {
  if (props.readonly) return
  const nextIDs = checked
    ? Array.from(new Set([...selectedGroupIds.value, groupId]))
    : selectedGroupIds.value.filter((id) => id !== groupId)
  emit('update:modelValue', {
    ...props.modelValue,
    group_ids: nextIDs,
  })
}
</script>
