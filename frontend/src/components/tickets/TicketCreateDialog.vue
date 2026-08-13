<template>
  <BaseDialog :show="show" :title="t('tickets.create')" width="wide" @close="emit('close')">
    <div class="mx-auto w-full max-w-2xl">
      <TicketEditorCard
        :category="category"
        :title="title"
        :payload="payload"
        :submit-label="t('tickets.submit')"
        :submitting="submitting"
        embedded
        :user-concurrency="userConcurrency"
        :rate-groups="rateGroups"
        @submit="submit"
      />
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TicketEditorCard from './TicketEditorCard.vue'
import type { TicketCategory, TicketRateGroupOption } from '@/types/ticket'

const props = withDefaults(defineProps<{
  show: boolean
  submitting?: boolean
  userConcurrency?: number | null
  rateGroups?: TicketRateGroupOption[]
}>(), {
  submitting: false,
  userConcurrency: null,
  rateGroups: () => [],
})

const emit = defineEmits<{
  close: []
  submit: [form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }]
}>()

const { t } = useI18n()

const category = ref<TicketCategory>('consult')
const title = ref('')
const payload = ref<Record<string, unknown>>({})

watch(() => props.show, (show) => {
  if (show) {
    category.value = 'consult'
    title.value = ''
    payload.value = {}
  }
})

function submit(form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }) {
  emit('submit', form)
}
</script>
