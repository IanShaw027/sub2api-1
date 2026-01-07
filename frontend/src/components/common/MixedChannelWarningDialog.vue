<template>
  <BaseDialog :show="show" :title="t('admin.accounts.mixedChannelWarning.title')" width="normal" @close="handleCancel">
    <div class="space-y-4">
      <!-- Warning Icon and Message -->
      <div class="flex items-start gap-3">
        <div class="flex-shrink-0">
          <Icon
            name="exclamationTriangle"
            size="lg"
            class="text-amber-500"
          />
        </div>
        <div class="flex-1">
          <p class="text-sm text-gray-700 dark:text-gray-300">
            {{ t('admin.accounts.mixedChannelWarning.description') }}
          </p>
        </div>
      </div>

      <!-- Details Box -->
      <div class="rounded-lg bg-amber-50 p-4 dark:bg-amber-900/20">
        <div class="space-y-2 text-sm">
          <div class="flex justify-between">
            <span class="text-gray-600 dark:text-gray-400">{{ t('admin.accounts.mixedChannelWarning.group') }}:</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ details.group_name }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-gray-600 dark:text-gray-400">{{ t('admin.accounts.mixedChannelWarning.currentPlatform') }}:</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ details.current_platform }}</span>
          </div>
          <div class="flex justify-between">
            <span class="text-gray-600 dark:text-gray-400">{{ t('admin.accounts.mixedChannelWarning.existingPlatform') }}:</span>
            <span class="font-medium text-gray-900 dark:text-white">{{ details.other_platform }}</span>
          </div>
        </div>
      </div>

      <!-- Warning Explanation -->
      <div class="rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-900/10">
        <p class="text-xs leading-relaxed text-amber-800 dark:text-amber-200">
          <Icon name="infoCircle" size="xs" class="mr-1 inline-block" />
          {{ t('admin.accounts.mixedChannelWarning.explanation') }}
        </p>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end space-x-3">
        <button
          @click="handleCancel"
          type="button"
          class="rounded-md border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:border-dark-600 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600 dark:focus:ring-offset-dark-800"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          @click="handleConfirm"
          type="button"
          class="rounded-md bg-amber-600 px-4 py-2 text-sm font-medium text-white hover:bg-amber-700 focus:outline-none focus:ring-2 focus:ring-amber-500 focus:ring-offset-2 dark:focus:ring-offset-dark-800"
        >
          {{ t('admin.accounts.mixedChannelWarning.confirm') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

export interface MixedChannelWarningDetails {
  group_id: number
  group_name: string
  current_platform: string
  other_platform: string
}

interface Props {
  show: boolean
  details: MixedChannelWarningDetails
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

const { t } = useI18n()

defineProps<Props>()
const emit = defineEmits<Emits>()

const handleConfirm = () => {
  emit('confirm')
}

const handleCancel = () => {
  emit('cancel')
}
</script>
