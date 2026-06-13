<template>
  <div>
    <div class="mb-3 flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{ t('admin.accounts.customErrorCodes') }}</label>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.customErrorCodesHint') }}
        </p>
      </div>
      <Toggle :model-value="enabled" @update:model-value="updateEnabled" />
    </div>

    <div v-if="enabled" class="space-y-3">
      <div class="rounded-lg bg-amber-50 p-3 dark:bg-amber-900/20">
        <p class="text-xs text-amber-700 dark:text-amber-400">
          <Icon name="exclamationTriangle" size="sm" class="mr-1 inline" :stroke-width="2" />
          {{ t('admin.accounts.customErrorCodesWarning') }}
        </p>
      </div>

      <div class="flex flex-wrap gap-2">
        <button
          v-for="code in commonErrorCodes"
          :key="code.value"
          type="button"
          @click="toggleErrorCode(code.value)"
          :class="[
            'rounded-lg px-3 py-1.5 text-sm font-medium transition-colors',
            codes.includes(code.value)
              ? 'bg-red-100 text-red-700 ring-1 ring-red-500 dark:bg-red-900/30 dark:text-red-400'
              : 'bg-gray-100 text-gray-600 hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-400 dark:hover:bg-dark-500'
          ]"
        >
          {{ code.value }} {{ code.label }}
        </button>
      </div>

      <div class="flex items-center gap-2">
        <input
          v-model.number="manualInput"
          type="number"
          min="100"
          max="599"
          class="input flex-1"
          :placeholder="t('admin.accounts.enterErrorCode')"
          @keyup.enter="addManualCode"
        />
        <button type="button" @click="addManualCode" class="btn btn-secondary px-3">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4" />
          </svg>
        </button>
      </div>

      <div class="flex flex-wrap gap-1.5">
        <span
          v-for="code in sortedCodes"
          :key="code"
          class="inline-flex items-center gap-1 rounded-full bg-red-100 px-2.5 py-0.5 text-sm font-medium text-red-700 dark:bg-red-900/30 dark:text-red-400"
        >
          {{ code }}
          <button
            type="button"
            @click="removeErrorCode(code)"
            class="hover:text-red-900 dark:hover:text-red-300"
          >
            <Icon name="x" size="sm" :stroke-width="2" />
          </button>
        </span>
        <span v-if="codes.length === 0" class="text-xs text-gray-400">
          {{ t('admin.accounts.noneSelectedUsesDefault') }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { useAppStore } from '@/stores/app'
import { commonErrorCodes } from '@/composables/useModelWhitelist'
import { isValidCustomErrorCode } from './customErrorCodes'

const props = defineProps<{
  enabled: boolean
  codes: number[]
}>()

const emit = defineEmits<{
  'update:enabled': [value: boolean]
  'update:codes': [value: number[]]
}>()

const { t } = useI18n()
const appStore = useAppStore()

const manualInput = ref<number | null>(null)

const sortedCodes = computed(() => [...props.codes].sort((a, b) => a - b))

const updateEnabled = (value: boolean) => {
  emit('update:enabled', value)
}

// 429/529 命中后会停止账号调度，加码前二次确认（与账号编辑页一致）。
const confirmDangerousCode = (code: number): boolean => {
  if (code === 429) {
    return confirm(t('admin.accounts.customErrorCodes429Warning'))
  }
  if (code === 529) {
    return confirm(t('admin.accounts.customErrorCodes529Warning'))
  }
  return true
}

const toggleErrorCode = (code: number) => {
  if (props.codes.includes(code)) {
    emit('update:codes', props.codes.filter((c) => c !== code))
    return
  }
  if (!confirmDangerousCode(code)) return
  emit('update:codes', [...props.codes, code])
}

const addManualCode = () => {
  const code = manualInput.value
  if (!isValidCustomErrorCode(code)) {
    appStore.showError(t('admin.accounts.invalidErrorCode'))
    return
  }
  if (props.codes.includes(code)) {
    appStore.showInfo(t('admin.accounts.errorCodeExists'))
    return
  }
  if (!confirmDangerousCode(code)) return
  emit('update:codes', [...props.codes, code])
  manualInput.value = null
}

const removeErrorCode = (code: number) => {
  emit('update:codes', props.codes.filter((c) => c !== code))
}
</script>
