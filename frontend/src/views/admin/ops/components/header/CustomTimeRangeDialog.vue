<template>
  <!-- Custom Time Range Dialog -->
  <BaseDialog :show="show" :title="t('admin.ops.timeRange.custom')" width="narrow" @close="handleCustomTimeRangeCancel">
    <div class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-foreground  mb-1">
          {{ t('admin.ops.customTimeRange.startTime') }}
        </label>
        <input
          v-model="customStartTimeInput"
          type="datetime-local"
          class="w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-foreground focus:border-accent-500 focus:outline-none focus:ring-1 focus:ring-accent-500   "
        />
      </div>
      <div>
        <label class="block text-sm font-medium text-foreground  mb-1">
          {{ t('admin.ops.customTimeRange.endTime') }}
        </label>
        <input
          v-model="customEndTimeInput"
          type="datetime-local"
          class="w-full rounded-lg border border-line bg-surface px-3 py-2 text-sm text-foreground focus:border-accent-500 focus:outline-none focus:ring-1 focus:ring-accent-500   "
        />
      </div>
      <div class="flex justify-end gap-3 pt-2">
        <button
          type="button"
          class="rounded-lg bg-surface-2 px-4 py-2 text-sm font-medium text-foreground hover:bg-surface-3   "
          @click="handleCustomTimeRangeCancel"
        >
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="rounded-lg bg-accent-500 px-4 py-2 text-sm font-medium text-white hover:bg-accent-600"
          @click="handleCustomTimeRangeConfirm"
        >
          {{ t('common.confirm') }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'

defineProps<{
  show: boolean
}>()

const customStartTimeInput = defineModel<string>('startTime', { required: true })
const customEndTimeInput = defineModel<string>('endTime', { required: true })

const emit = defineEmits<{
  (e: 'confirm'): void
  (e: 'cancel'): void
}>()

const { t } = useI18n()

function handleCustomTimeRangeConfirm() {
  emit('confirm')
}

function handleCustomTimeRangeCancel() {
  emit('cancel')
}
</script>
