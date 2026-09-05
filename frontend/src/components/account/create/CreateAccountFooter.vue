<template>
  <div v-if="step === 1" class="flex justify-end gap-3">
    <button @click="emit('close')" type="button" class="btn btn-secondary">
      {{ t('common.cancel') }}
    </button>
    <button
      type="submit"
      form="create-account-form"
      :disabled="submitting"
      class="btn btn-primary"
      data-tour="account-form-submit"
    >
      <svg
        v-if="submitting"
        class="-ml-1 mr-2 h-4 w-4 animate-spin"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        ></circle>
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
      {{
        isOAuthFlow
          ? t('common.next')
          : submitting
            ? t('admin.accounts.creating')
            : t('common.create')
      }}
    </button>
  </div>
  <div v-else class="flex justify-between gap-3">
    <button type="button" class="btn btn-secondary" @click="emit('back')">
      {{ t('common.back') }}
    </button>
    <button
      v-if="platform !== 'kiro' && isManualInputMethod"
      type="button"
      :disabled="!canExchangeCode"
      class="btn btn-primary"
      @click="emit('exchangeCode')"
    >
      <svg
        v-if="currentOAuthLoading"
        class="-ml-1 mr-2 h-4 w-4 animate-spin"
        fill="none"
        viewBox="0 0 24 24"
      >
        <circle
          class="opacity-25"
          cx="12"
          cy="12"
          r="10"
          stroke="currentColor"
          stroke-width="4"
        ></circle>
        <path
          class="opacity-75"
          fill="currentColor"
          d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
      {{
        currentOAuthLoading
          ? t('admin.accounts.oauth.verifying')
          : t('admin.accounts.oauth.completeAuth')
      }}
    </button>
  </div>
</template>

<script setup lang="ts">
// BaseDialog footer for CreateAccountModal, lifted verbatim out of the host.
// Handler wiring stays as emits so the host keeps full control of behavior.
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  step: number
  submitting: boolean
  isOAuthFlow: boolean
  platform: string
  isManualInputMethod: boolean
  canExchangeCode: boolean
  currentOAuthLoading: boolean
}>()

const emit = defineEmits<{
  close: []
  back: []
  exchangeCode: []
}>()
</script>
