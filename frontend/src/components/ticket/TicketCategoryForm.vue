<template>
  <div class="space-y-3">
    <div v-if="category === 'consult'">
      <label class="input-label">{{ t('tickets.form.question') }}</label>
      <textarea v-model="form.question" rows="4" class="input mt-1 w-full" :disabled="disabled" />
    </div>
    <div v-else-if="category === 'refund'" class="space-y-3">
      <div>
        <label class="input-label">{{ t('tickets.form.orderNo') }}</label>
        <input v-model="form.order_no" class="input mt-1 w-full" :disabled="disabled" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.form.refundAmount') }}</label>
        <input v-model="form.refund_amount" class="input mt-1 w-full" :disabled="disabled" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.form.reason') }}</label>
        <textarea v-model="form.reason" rows="3" class="input mt-1 w-full" :disabled="disabled" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.form.evidence') }}</label>
        <textarea v-model="form.evidence" rows="2" class="input mt-1 w-full" :disabled="disabled" />
      </div>
    </div>
    <div v-else-if="category === 'concurrency_apply'" class="space-y-3">
      <div>
        <label class="input-label">{{ t('tickets.form.currentConcurrency') }}</label>
        <input v-model="form.current_concurrency" class="input mt-1 w-full" :disabled="disabled" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.form.targetConcurrency') }}</label>
        <input v-model="form.target_concurrency" class="input mt-1 w-full" :disabled="disabled" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.form.usageScenario') }}</label>
        <textarea v-model="form.usage_scenario" rows="3" class="input mt-1 w-full" :disabled="disabled" />
      </div>
    </div>
    <div v-else-if="category === 'rate_apply'" class="space-y-3">
      <p class="text-sm text-gray-500">{{ t('tickets.form.rateHint') }}</p>
      <label v-for="group in rateGroups" :key="group.group_id" class="flex items-center gap-2 text-sm">
        <input v-model="selectedGroupIds" type="checkbox" :value="group.group_id" :disabled="disabled" />
        <span>{{ group.name }} · {{ t('tickets.form.baseRate') }} {{ group.base_rate_multiplier }} · {{ t('tickets.form.effectiveRate') }} {{ group.effective_rate }}</span>
      </label>
      <div>
        <label class="input-label">{{ t('tickets.form.targetRate') }}</label>
        <input v-model="form.target_rate" class="input mt-1 w-full" :disabled="disabled" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.form.usageScenario') }}</label>
        <textarea v-model="form.usage_scenario" rows="3" class="input mt-1 w-full" :disabled="disabled" />
      </div>
    </div>
    <div v-else>
      <label class="input-label">{{ t('tickets.form.details') }}</label>
      <textarea v-model="form.details" rows="4" class="input mt-1 w-full" :disabled="disabled" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { TicketCategory, TicketRateGroupOption } from '@/types/ticket'

defineProps<{
  category: TicketCategory
  disabled?: boolean
  rateGroups?: TicketRateGroupOption[]
}>()

const form = defineModel<Record<string, string>>('form', { required: true })
const selectedGroupIds = defineModel<number[]>('selectedGroupIds', { default: () => [] })
const { t } = useI18n()
</script>
