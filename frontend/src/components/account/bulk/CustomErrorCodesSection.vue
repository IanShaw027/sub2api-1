<template>
  <div class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label
          id="bulk-edit-custom-error-codes-label"
          class="input-label mb-0"
          for="bulk-edit-custom-error-codes-enabled"
        >
          {{ t('admin.accounts.customErrorCodes') }}
        </label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.customErrorCodesHint') }}
        </p>
      </div>
      <input
        v-model="enabled"
        id="bulk-edit-custom-error-codes-enabled"
        type="checkbox"
        aria-controls="bulk-edit-custom-error-codes-body"
        class="rounded border-line text-accent focus:ring-accent"
      />
    </div>

    <div v-if="enabled" id="bulk-edit-custom-error-codes-body" class="space-y-3">
      <div class="rounded-lg bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] p-3">
        <p class="text-xs text-warning-text">
          <Icon name="exclamationTriangle" size="sm" class="mr-1 inline" :stroke-width="2" />
          {{ t('admin.accounts.customErrorCodesWarning') }}
        </p>
      </div>

      <!-- Error Code Buttons -->
      <div class="flex flex-wrap gap-2">
        <button
          v-for="code in commonErrorCodes"
          :key="code.value"
          type="button"
          :class="[
            'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
            selectedCodes.includes(code.value)
              ? 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text ring-1 ring-danger-text'
              : 'bg-surface-2 text-muted hover:bg-surface-3'
          ]"
          @click="toggleErrorCode(code.value)"
        >
          {{ code.value }} {{ code.label }}
        </button>
      </div>

      <!-- Manual input -->
      <div class="flex items-center gap-2">
        <input
          v-model="customErrorCodeInput"
          id="bulk-edit-custom-error-code-input"
          type="number"
          min="100"
          max="599"
          class="input flex-1"
          :placeholder="t('admin.accounts.enterErrorCode')"
          aria-labelledby="bulk-edit-custom-error-codes-label"
          @keyup.enter="addCustomErrorCode"
        />
        <button type="button" class="btn btn-secondary px-3" @click="addCustomErrorCode">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M12 4v16m8-8H4"
            />
          </svg>
        </button>
      </div>

      <!-- Selected codes summary -->
      <div class="flex flex-wrap gap-1.5">
        <span
          v-for="code in [...selectedCodes].sort((a, b) => a - b)"
          :key="code"
          class="inline-flex items-center gap-1 rounded-full bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] px-2.5 py-0.5 text-sm font-medium text-danger-text"
        >
          {{ code }}
          <button
            type="button"
            class="hover:text-danger-text"
            @click="removeErrorCode(code)"
          >
            <Icon name="x" size="xs" class="h-3.5 w-3.5" :stroke-width="2" />
          </button>
        </span>
        <span v-if="selectedCodes.length === 0" class="text-xs text-muted">
          {{ t('admin.accounts.noneSelectedUsesDefault') }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const enabled = defineModel<boolean>('enabled', { required: true })
const selectedCodes = defineModel<number[]>('selectedCodes', { required: true })

const { t } = useI18n()
const appStore = useAppStore()

const customErrorCodeInput = ref<number | null>(null)

// Common HTTP error codes
const commonErrorCodes = [
  { value: 401, label: 'Unauthorized' },
  { value: 403, label: 'Forbidden' },
  { value: 429, label: 'Rate Limit' },
  { value: 500, label: 'Server Error' },
  { value: 502, label: 'Bad Gateway' },
  { value: 503, label: 'Unavailable' },
  { value: 529, label: 'Overloaded' }
]

const toggleErrorCode = (code: number) => {
  const index = selectedCodes.value.indexOf(code)
  if (index === -1) {
    // Adding code - check for 429/529 warning
    if (code === 429) {
      if (!confirm(t('admin.accounts.customErrorCodes429Warning'))) {
        return
      }
    } else if (code === 529) {
      if (!confirm(t('admin.accounts.customErrorCodes529Warning'))) {
        return
      }
    }
    selectedCodes.value.push(code)
  } else {
    selectedCodes.value.splice(index, 1)
  }
}

const addCustomErrorCode = () => {
  const code = customErrorCodeInput.value
  if (code === null || code < 100 || code > 599) {
    appStore.showError(t('admin.accounts.invalidErrorCode'))
    return
  }
  if (selectedCodes.value.includes(code)) {
    appStore.showInfo(t('admin.accounts.errorCodeExists'))
    return
  }
  // Check for 429/529 warning
  if (code === 429) {
    if (!confirm(t('admin.accounts.customErrorCodes429Warning'))) {
      return
    }
  } else if (code === 529) {
    if (!confirm(t('admin.accounts.customErrorCodes529Warning'))) {
      return
    }
  }
  selectedCodes.value.push(code)
  customErrorCodeInput.value = null
}

const removeErrorCode = (code: number) => {
  const index = selectedCodes.value.indexOf(code)
  if (index !== -1) {
    selectedCodes.value.splice(index, 1)
  }
}
</script>
