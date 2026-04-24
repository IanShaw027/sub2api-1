<template>
  <BaseDialog :show="show" :title="t('tickets.create')" width="full" @close="emit('close')">
    <div class="mx-auto min-h-[70vh] max-w-4xl">
      <TicketEditorCard
        :category="category"
        :title="title"
        :payload="payload"
        :submit-label="t('tickets.submit')"
        :submitting="submitting"
        :user-concurrency="userConcurrency"
        :available-groups="availableGroups"
        :user-group-rates="userGroupRates"
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
import type { Group, TicketCategory } from '@/types'

const props = withDefaults(defineProps<{
  show: boolean
  submitting?: boolean
  userConcurrency?: number | null
  availableGroups?: Group[]
  userGroupRates?: Record<number, number>
}>(), {
  submitting: false,
  userConcurrency: null,
  availableGroups: () => [],
  userGroupRates: () => ({}),
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
