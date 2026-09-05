<template>
  <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
    <div>
      <label class="input-label">{{ t('admin.accounts.concurrency') }}</label>
      <input v-model.number="concurrency" type="number" min="1" class="input"
        @input="concurrency = Math.max(1, concurrency || 1)" />
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.loadFactor') }}</label>
      <input v-model.number="loadFactor" type="number" min="1"
        class="input" :placeholder="String(concurrency || 1)"
        @input="loadFactor = (loadFactor &amp;&amp; loadFactor >= 1) ? loadFactor : null" />
      <p class="input-hint">{{ t('admin.accounts.loadFactorHint') }}</p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.priority') }}</label>
      <input
        v-model.number="priority"
        type="number"
        min="1"
        class="input"
        data-tour="account-form-priority"
      />
      <p class="input-hint">{{ t('admin.accounts.priorityHint') }}</p>
    </div>
    <div>
      <label class="input-label">{{ t('admin.accounts.billingRateMultiplier') }}</label>
      <input v-model.number="rateMultiplier" type="number" min="0" step="0.001" class="input" />
      <p class="input-hint">{{ t('admin.accounts.billingRateMultiplierHint') }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
// Concurrency / load factor / priority / rate multiplier fields, lifted
// verbatim out of the host.
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

const concurrency = defineModel<number>('concurrency', { required: true })
const loadFactor = defineModel<number | null>('loadFactor', { required: true })
const priority = defineModel<number>('priority', { required: true })
const rateMultiplier = defineModel<number>('rateMultiplier', { required: true })
</script>
