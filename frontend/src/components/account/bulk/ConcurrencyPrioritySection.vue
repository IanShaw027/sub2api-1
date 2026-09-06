<template>
  <div class="grid grid-cols-2 gap-4 border-t border-line pt-4 lg:grid-cols-4">
    <div>
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-concurrency-label"
          class="input-label mb-0"
          for="bulk-edit-concurrency-enabled"
        >
          {{ t('admin.accounts.concurrency') }}
        </label>
        <input
          v-model="enableConcurrency"
          id="bulk-edit-concurrency-enabled"
          type="checkbox"
          aria-controls="bulk-edit-concurrency"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <input
        :value="concurrency"
        id="bulk-edit-concurrency"
        type="number"
        min="1"
        :disabled="!enableConcurrency"
        class="input"
        :class="!enableConcurrency && 'cursor-not-allowed opacity-50'"
        aria-labelledby="bulk-edit-concurrency-label"
        @input="updateConcurrency"
      />
    </div>
    <div>
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-load-factor-label"
          class="input-label mb-0"
          for="bulk-edit-load-factor-enabled"
        >
          {{ t('admin.accounts.loadFactor') }}
        </label>
        <input
          v-model="enableLoadFactor"
          id="bulk-edit-load-factor-enabled"
          type="checkbox"
          aria-controls="bulk-edit-load-factor"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <input
        :value="loadFactor"
        id="bulk-edit-load-factor"
        type="number"
        min="1"
        :disabled="!enableLoadFactor"
        class="input"
        :class="!enableLoadFactor && 'cursor-not-allowed opacity-50'"
        aria-labelledby="bulk-edit-load-factor-label"
        @input="updateLoadFactor"
      />
      <p class="input-hint">{{ t('admin.accounts.loadFactorHint') }}</p>
    </div>
    <div>
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-priority-label"
          class="input-label mb-0"
          for="bulk-edit-priority-enabled"
        >
          {{ t('admin.accounts.priority') }}
        </label>
        <input
          v-model="enablePriority"
          id="bulk-edit-priority-enabled"
          type="checkbox"
          aria-controls="bulk-edit-priority"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <input
        v-model.number="priority"
        id="bulk-edit-priority"
        type="number"
        min="1"
        :disabled="!enablePriority"
        class="input"
        :class="!enablePriority && 'cursor-not-allowed opacity-50'"
        aria-labelledby="bulk-edit-priority-label"
      />
    </div>
    <div>
      <div class="mb-3 flex items-center justify-between">
        <label
          id="bulk-edit-rate-multiplier-label"
          class="input-label mb-0"
          for="bulk-edit-rate-multiplier-enabled"
        >
          {{ t('admin.accounts.billingRateMultiplier') }}
        </label>
        <input
          v-model="enableRateMultiplier"
          id="bulk-edit-rate-multiplier-enabled"
          type="checkbox"
          aria-controls="bulk-edit-rate-multiplier"
          class="rounded border-line text-accent focus:ring-accent"
        />
      </div>
      <input
        v-model.number="rateMultiplier"
        id="bulk-edit-rate-multiplier"
        type="number"
        min="0"
        step="0.01"
        :disabled="!enableRateMultiplier"
        class="input"
        :class="!enableRateMultiplier && 'cursor-not-allowed opacity-50'"
        aria-labelledby="bulk-edit-rate-multiplier-label"
      />
      <p class="input-hint">{{ t('admin.accounts.billingRateMultiplierHint') }}</p>
      <p
        v-if="enableRateMultiplier"
        class="mt-2 flex items-start gap-1 text-xs text-warning-text"
        data-testid="bulk-rate-sync-warning"
      >
        <Icon name="exclamationTriangle" size="xs" class="mt-0.5 flex-shrink-0" />
        <span>{{ t('admin.accounts.bulkEdit.rateSyncWarning') }}</span>
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const enableConcurrency = defineModel<boolean>('enableConcurrency', { required: true })
const concurrency = defineModel<number | null>('concurrency', { required: true })
const enableLoadFactor = defineModel<boolean>('enableLoadFactor', { required: true })
const loadFactor = defineModel<number | null>('loadFactor', { required: true })
const enablePriority = defineModel<boolean>('enablePriority', { required: true })
const priority = defineModel<number | null>('priority', { required: true })
const enableRateMultiplier = defineModel<boolean>('enableRateMultiplier', { required: true })
const rateMultiplier = defineModel<number | null>('rateMultiplier', { required: true })

const updateConcurrency = (event: Event) => {
  const input = event.target as HTMLInputElement
  const value = Math.max(1, Number(input.value) || 1)
  input.value = String(value)
  concurrency.value = value
}

const updateLoadFactor = (event: Event) => {
  const input = event.target as HTMLInputElement
  const value = Number(input.value)
  const normalized = value >= 1 ? value : null
  input.value = normalized === null ? '' : String(normalized)
  loadFactor.value = normalized
}

const { t } = useI18n()
</script>
