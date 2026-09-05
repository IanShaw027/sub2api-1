<template>
  <div class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.customErrorCodes') }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.customErrorCodesHint') }}
        </p>
      </div>
      <InlineToggleSwitch v-model="customErrorCodesEnabled" />
    </div>

    <div v-if="customErrorCodesEnabled" class="space-y-3">
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
          @click="toggleErrorCode(code.value)"
          :class="[
 'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
 selectedErrorCodes.includes(code.value)
 ? 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text ring-1 ring-danger-text'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
        >
          {{ code.value }} {{ code.label }}
        </button>
      </div>

      <!-- Manual input -->
      <div class="flex items-center gap-2">
        <input
          v-model.number="customErrorCodeInput"
          type="number"
          min="100"
          max="599"
          class="input flex-1"
          :placeholder="t('admin.accounts.enterErrorCode')"
          @keyup.enter="addCustomErrorCode"
        />
        <button type="button" @click="addCustomErrorCode" class="btn btn-secondary px-3">
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
          v-for="code in selectedErrorCodes.sort((a, b) => a - b)"
          :key="code"
          class="inline-flex items-center gap-1 rounded-full bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] px-2.5 py-0.5 text-sm font-medium text-danger-text"
        >
          {{ code }}
          <button
            type="button"
            @click="removeErrorCode(code)"
            class="hover:text-danger-text"
          >
            <Icon name="x" size="sm" :stroke-width="2" />
          </button>
        </span>
        <span v-if="selectedErrorCodes.length === 0" class="text-xs text-muted">
          {{ t('admin.accounts.noneSelectedUsesDefault') }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared "Custom Error Codes" section, byte-identical between
// CreateAccountModal.vue and EditAccountModal.vue. selectedErrorCodes /
// customErrorCodeInput stay defineModel'd since both hosts also read/reset
// them elsewhere (submit payload building, edit-mode hydration, form reset).
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { commonErrorCodes } from '@/composables/useModelWhitelist'
import Icon from '@/components/icons/Icon.vue'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'

const { t } = useI18n()
const appStore = useAppStore()

const customErrorCodesEnabled = defineModel<boolean>('customErrorCodesEnabled', { required: true })
const selectedErrorCodes = defineModel<number[]>('selectedErrorCodes', { required: true })
const customErrorCodeInput = defineModel<number | null>('customErrorCodeInput', { required: true })

const toggleErrorCode = (code: number) => {
  const index = selectedErrorCodes.value.indexOf(code)
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
    selectedErrorCodes.value.push(code)
  } else {
    selectedErrorCodes.value.splice(index, 1)
  }
}

// Add custom error code from input
const addCustomErrorCode = () => {
  const code = customErrorCodeInput.value
  if (code === null || code < 100 || code > 599) {
    appStore.showError(t('admin.accounts.invalidErrorCode'))
    return
  }
  if (selectedErrorCodes.value.includes(code)) {
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
  selectedErrorCodes.value.push(code)
  customErrorCodeInput.value = null
}

// Remove error code
const removeErrorCode = (code: number) => {
  const index = selectedErrorCodes.value.indexOf(code)
  if (index !== -1) {
    selectedErrorCodes.value.splice(index, 1)
  }
}
</script>
