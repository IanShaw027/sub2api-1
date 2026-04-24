<template>
  <div class="space-y-4 rounded-2xl border bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
    <div class="grid gap-4 md:grid-cols-2">
      <div>
        <label class="input-label">{{ t('tickets.fields.category') }}</label>
        <select v-model="localCategory" class="input" :disabled="disableCategory">
          <option v-for="option in ticketCategoryOptions" :key="option.value" :value="option.value">
            {{ t(option.labelKey) }}
          </option>
        </select>
      </div>
      <div>
        <label class="input-label">{{ t('tickets.fields.title') }}</label>
        <input v-model="localTitle" class="input" />
      </div>
    </div>

    <TicketCategoryForm :category="localCategory" :model-value="localPayload" @update:model-value="localPayload = $event" />

    <div class="flex justify-end gap-3">
      <button v-if="showCancel" class="btn btn-secondary" @click="$emit('cancel')">{{ t('common.cancel') }}</button>
      <button class="btn btn-primary" :disabled="submitting" @click="submit">
        {{ submitting ? t('common.submitting') : submitLabel }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TicketCategory } from '@/types'
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
}>(), {
  disableCategory: false,
  submitting: false,
  showCancel: false,
})

const emit = defineEmits<{
  submit: [{ category: TicketCategory; title: string; form_payload: Record<string, unknown> }]
  cancel: []
}>()

const { t } = useI18n()
const localCategory = ref<TicketCategory>(props.category)
const localTitle = ref(props.title)
const localPayload = ref<Record<string, unknown>>({ ...props.payload })

watch(() => props.category, (value) => { localCategory.value = value })
watch(() => props.title, (value) => { localTitle.value = value })
watch(() => props.payload, (value) => { localPayload.value = { ...value } }, { deep: true })

function submit() {
  emit('submit', {
    category: localCategory.value,
    title: localTitle.value.trim(),
    form_payload: localPayload.value,
  })
}
</script>
