<template>
  <BaseDialog
    :show="show"
    :title="t('admin.channelMonitor.adjustAvailability.title')"
    width="normal"
    @close="handleClose"
  >
    <form class="space-y-4" @submit.prevent="submit">
      <div v-if="monitor" class="rounded-xl bg-page p-3 text-sm dark:bg-dark-800">
        <div class="font-medium text-ink dark:text-white">{{ monitor.name }}</div>
        <div class="mt-1 text-xs text-ink-soft">
          {{ monitor.primary_model }} · {{ t('admin.channelMonitor.adjustAvailability.current', { value: formatPct(monitor.availability_7d) }) }}
        </div>
      </div>

      <div>
        <label class="input-label">
          {{ t('admin.channelMonitor.adjustAvailability.target') }}
        </label>
        <div class="flex items-center gap-2">
          <input
            v-model.number="targetPct"
            type="number"
            min="0"
            max="100"
            step="0.01"
            class="input"
            :placeholder="t('admin.channelMonitor.adjustAvailability.placeholder')"
          />
          <span class="text-sm text-ink-soft">%</span>
        </div>
        <p class="mt-2 text-xs text-ink-soft">
          {{ t('admin.channelMonitor.adjustAvailability.hint') }}
        </p>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-2">
        <button type="button" class="btn btn-secondary" :disabled="submitting" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="submitting || !monitor" @click="submit">
          {{ submitting ? t('common.submitting') : t('admin.channelMonitor.adjustAvailability.submit') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  AvailabilityAdjustResult,
  ChannelMonitor,
} from '@/api/admin/channelMonitor'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'

const props = defineProps<{
  show: boolean
  monitor: ChannelMonitor | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'adjusted', result: AvailabilityAdjustResult): void
}>()

const { t } = useI18n()
const appStore = useAppStore()
const targetPct = ref<number | null>(null)
const submitting = ref(false)

function formatPct(value: number): string {
  return `${Number(value || 0).toFixed(2)}%`
}

watch(
  () => [props.show, props.monitor?.id] as const,
  () => {
    if (props.show && props.monitor) {
      targetPct.value = Number(props.monitor.availability_7d || 0)
    }
  },
  { immediate: true }
)

function handleClose() {
  if (submitting.value) return
  emit('close')
}

async function submit() {
  if (!props.monitor || submitting.value) return
  const value = Number(targetPct.value)
  if (!Number.isFinite(value) || value < 0 || value > 100) {
    appStore.showError(t('admin.channelMonitor.adjustAvailability.invalid'))
    return
  }

  submitting.value = true
  try {
    const result = await adminAPI.channelMonitor.adjustAvailability7d(props.monitor.id, {
      availability_pct: value,
    })
    emit('adjusted', result)
  } catch (err: unknown) {
    appStore.showError(extractApiErrorMessage(err, t('admin.channelMonitor.adjustAvailability.failed')))
  } finally {
    submitting.value = false
  }
}
</script>
