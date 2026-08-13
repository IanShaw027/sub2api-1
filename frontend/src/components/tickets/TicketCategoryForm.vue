<template>
  <component
    :is="currentComponent"
    :model-value="modelValue"
    :readonly="readonly"
    :user-concurrency="userConcurrency"
    :rate-groups="rateGroups"
    @update:model-value="emit('update:modelValue', $event)"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { TicketCategory, TicketRateGroupOption } from '@/types/ticket'
import TicketFormConsult from './forms/TicketFormConsult.vue'
import TicketFormRefund from './forms/TicketFormRefund.vue'
import TicketFormConcurrency from './forms/TicketFormConcurrency.vue'
import TicketFormRate from './forms/TicketFormRate.vue'
import TicketFormOther from './forms/TicketFormOther.vue'

const props = defineProps<{
  category: TicketCategory
  modelValue: Record<string, unknown>
  readonly?: boolean
  userConcurrency?: number | null
  rateGroups?: TicketRateGroupOption[]
}>()
const emit = defineEmits<{ 'update:modelValue': [value: Record<string, unknown>] }>()

const currentComponent = computed(() => {
  switch (props.category) {
    case 'consult':
      return TicketFormConsult
    case 'refund':
      return TicketFormRefund
    case 'concurrency_apply':
      return TicketFormConcurrency
    case 'rate_apply':
      return TicketFormRate
    default:
      return TicketFormOther
  }
})
</script>
