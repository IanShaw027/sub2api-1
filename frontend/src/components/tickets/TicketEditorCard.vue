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
          <input v-model="localTitle" class="input" maxlength="80" />
        </div>
      </div>
    </div>

    <div :class="bodyClass">
      <TicketCategoryForm
        :category="localCategory"
        :model-value="localPayload"
        :user-concurrency="userConcurrency"
        :rate-groups="rateGroups"
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
import type { TicketCategory, TicketRateGroupOption } from '@/types/ticket'
import Select from '@/components/common/Select.vue'
import { sanitizeTicketPayload, ticketCategoryOptions } from '@/utils/tickets'
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
  rateGroups?: TicketRateGroupOption[]
}>(), {
  disableCategory: false,
  submitting: false,
  showCancel: false,
  embedded: false,
  userConcurrency: null,
  rateGroups: () => [],
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
    : 'flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border bg-white dark:border-dark-700 dark:bg-dark-800',
)
const headerClass = computed(() =>
  props.embedded
    ? 'space-y-4'
    : 'border-b border-gray-100 px-5 py-5 dark:border-dark-700',
)
const bodyClass = computed(() =>
  props.embedded
    ? 'space-y-5'
    : 'min-h-0 flex-1 overflow-y-auto px-5 py-5',
)
const footerClass = computed(() =>
  props.embedded
    ? 'pt-2'
    : 'border-t border-gray-100 px-5 py-4 dark:border-dark-700',
)

watch(() => props.category, (value) => { localCategory.value = value })
watch(() => props.title, (value) => { localTitle.value = value })
watch(() => props.payload, (value) => {
  localPayload.value = { ...value }
}, { deep: true })

function submit() {
  emit('submit', {
    category: localCategory.value,
    title: localTitle.value.trim(),
    form_payload: sanitizeTicketPayload(localCategory.value, localPayload.value),
  })
}
</script>
