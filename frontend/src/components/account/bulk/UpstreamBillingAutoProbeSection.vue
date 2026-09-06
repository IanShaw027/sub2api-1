<template>
  <div v-if="show" class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div class="flex-1 pr-4">
        <label
          id="bulk-edit-upstream-billing-auto-probe-label"
          class="input-label mb-0"
          for="bulk-edit-upstream-billing-auto-probe-enabled"
        >
          {{ t('admin.accounts.upstreamBilling.autoProbe') }}
        </label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.upstreamBilling.autoProbeHint') }}
        </p>
      </div>
      <input
        v-model="enableUpstreamBillingAutoProbe"
        id="bulk-edit-upstream-billing-auto-probe-enabled"
        type="checkbox"
        aria-controls="bulk-edit-upstream-billing-auto-probe"
        class="rounded border-line text-accent focus:ring-accent"
      />
    </div>
    <div
      id="bulk-edit-upstream-billing-auto-probe"
      :class="!enableUpstreamBillingAutoProbe && 'pointer-events-none opacity-50'"
      role="group"
      aria-labelledby="bulk-edit-upstream-billing-auto-probe-label"
    >
      <Select
        v-model="upstreamBillingAutoProbeMode"
        :disabled="!enableUpstreamBillingAutoProbe"
        data-testid="bulk-edit-upstream-billing-auto-probe-select"
        :options="upstreamBillingAutoProbeOptions"
        aria-labelledby="bulk-edit-upstream-billing-auto-probe-label"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'

interface Props {
  show: boolean
  upstreamBillingAutoProbeOptions: SelectOption[]
}

defineProps<Props>()

const enableUpstreamBillingAutoProbe = defineModel<boolean>('enableUpstreamBillingAutoProbe', {
  required: true
})
const upstreamBillingAutoProbeMode = defineModel<string>('upstreamBillingAutoProbeMode', {
  required: true
})

const { t } = useI18n()
</script>
