<template>
  <UiModal :open="open" :title="t('payment.selectPlan')" width="md" @close="$emit('close')">
    <div class="space-y-4">
      <SubscriptionPlanCard v-for="plan in plans" :key="plan.id" :plan="plan" :active-subscriptions="activeSubscriptions" @select="$emit('select', $event)" />
    </div>
  </UiModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import UiModal from '@/components/ui/UiModal.vue'
import SubscriptionPlanCard from '@/components/payment/SubscriptionPlanCard.vue'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'

const { t } = useI18n()

defineProps<{
  open: boolean
  plans: SubscriptionPlan[]
  activeSubscriptions: UserSubscription[]
}>()

defineEmits<{
  close: []
  select: [plan: SubscriptionPlan]
}>()
</script>
