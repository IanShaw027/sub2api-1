<template>
  <div :class="containerClass">
    <div :class="headerClass">
      <div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <div>
          <label class="input-label">{{ t('tickets.fields.category') }}</label>
          <Select
            v-model="localCategory"
            :options="categoryOptions"
            :searchable="false"
            :disabled="disableCategory"
          />
        </div>
        <div>
          <label class="input-label">{{ t('tickets.fields.title') }}</label>
          <input v-model="localTitle" class="input" />
        </div>
      </div>
    </div>

    <div :class="bodyClass">
      <TicketCategoryForm
        :category="localCategory"
        :model-value="localPayload"
        :user-concurrency="userConcurrency"
        :available-groups="availableGroups"
        :user-group-rates="userGroupRates"
        @update:model-value="localPayload = $event"
      />
    </div>

    <div :class="footerClass">
      <div class="flex justify-end gap-3">
        <button v-if="showCancel" class="btn btn-secondary" @click="$emit('cancel')">{{ t('common.cancel') }}</button>
        <button class="btn btn-primary" :disabled="submitting" @click="submit">
          {{ submitting ? t('common.submitting') : submitLabel }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Group, TicketCategory } from '@/types'
import Select from '@/components/common/Select.vue'
import { ticketCategoryOptions } from '@/utils/tickets'
import TicketCategoryForm from './TicketCategoryForm.vue'

const props = withDefaults(defineProps<{
  category: TicketCategory
  title: string
  payload: Record<string, unknown>
  disableCategory?: boolean
  submitting?: boolean
  submitLabel: string
  showCancel?: boolean
  embedded?: boolean
  userConcurrency?: number | null
  availableGroups?: Group[]
  userGroupRates?: Record<number, number>
}>(), {
  disableCategory: false,
  submitting: false,
  showCancel: false,
  embedded: false,
  userConcurrency: null,
  availableGroups: () => [],
  userGroupRates: () => ({}),
})

const emit = defineEmits<{
  submit: [{ category: TicketCategory; title: string; form_payload: Record<string, unknown> }]
  cancel: []
}>()

const { t } = useI18n()
const localCategory = ref<TicketCategory>(props.category)
const localTitle = ref(props.title)
const localPayload = ref<Record<string, unknown>>({ ...props.payload })
const categoryOptions = computed(() =>
  ticketCategoryOptions.map((option) => ({
    value: option.value,
    label: t(option.labelKey),
  })),
)
const containerClass = computed(() =>
  props.embedded
    ? 'space-y-5'
    : 'flex h-full min-h-0 flex-col overflow-hidden rounded-card border border-line bg-card dark:border-dark-700 dark:bg-dark-800',
)
const headerClass = computed(() =>
  props.embedded
    ? 'space-y-4'
    : 'border-b border-line px-5 py-5 dark:border-dark-700',
)
const bodyClass = computed(() =>
  props.embedded
    ? 'space-y-5'
    : 'min-h-0 flex-1 overflow-y-auto px-5 py-5',
)
const footerClass = computed(() =>
  props.embedded
    ? 'pt-2'
    : 'border-t border-line px-5 py-4 dark:border-dark-700',
)

watch(() => props.category, (value) => { localCategory.value = value })
watch(() => props.title, (value) => { localTitle.value = value })
watch(() => props.payload, (value) => {
  localPayload.value = { ...value }
}, { deep: true })

function submit() {
  const nextPayload: Record<string, unknown> = { ...localPayload.value }

  if (localCategory.value === 'rate_apply') {
    const visibleGroupIDs = new Set(
      props.availableGroups
        .filter((group) => group.subscription_type === 'standard')
        .map((group) => group.id),
    )
    const groupIDs = Array.isArray(nextPayload.group_ids)
      ? Array.from(new Set(
        nextPayload.group_ids
          .map((item) => Number(item))
          .filter((item) => Number.isFinite(item) && item > 0 && visibleGroupIDs.has(item)),
      ))
      : []

    nextPayload.group_ids = groupIDs
    nextPayload.current_group_rates = props.availableGroups
      .filter((group) => group.subscription_type === 'standard' && groupIDs.includes(group.id))
      .map((group) => ({
        group_id: group.id,
        group_name: group.name,
        base_rate: group.rate_multiplier,
        special_rate: props.userGroupRates[group.id] ?? null,
        effective_rate: props.userGroupRates[group.id] ?? group.rate_multiplier,
      }))
  }

  emit('submit', {
    category: localCategory.value,
    title: localTitle.value.trim(),
    form_payload: nextPayload,
  })
}
</script>
