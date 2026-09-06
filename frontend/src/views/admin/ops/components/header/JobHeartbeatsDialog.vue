<template>
  <BaseDialog :show="show" :title="t('admin.ops.jobs')" width="wide" @close="emit('close')">
    <div v-if="!jobHeartbeats.length" class="text-sm text-muted ">
      {{ t('admin.ops.noData') }}
    </div>
    <div v-else class="space-y-3">
      <div
        v-for="hb in jobHeartbeats"
        :key="hb.job_name"
        class="rounded-xl border border-line bg-surface p-4  "
      >
        <div class="flex items-center justify-between gap-3">
          <div class="truncate text-sm font-semibold text-foreground ">{{ hb.job_name }}</div>
          <div class="flex items-center gap-3 text-xs text-muted ">
            <span v-if="hb.last_duration_ms != null" class="font-mono">{{ hb.last_duration_ms }}ms</span>
            <span>{{ formatTimeShort(hb.updated_at) }}</span>
          </div>
        </div>

        <div class="mt-2 grid grid-cols-1 gap-2 text-xs text-muted  sm:grid-cols-2">
          <div>
            {{ t('admin.ops.lastSuccess') }} <span class="font-mono">{{ formatTimeShort(hb.last_success_at) }}</span>
          </div>
          <div>
            {{ t('admin.ops.lastError') }} <span class="font-mono">{{ formatTimeShort(hb.last_error_at) }}</span>
          </div>
          <div>
            {{ t('admin.ops.result') }} <span class="font-mono">{{ hb.last_result || '-' }}</span>
          </div>
        </div>

        <div
          v-if="hb.last_error"
          class="mt-3 rounded-lg bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] p-2 text-xs text-danger-text  "
        >
          {{ hb.last_error }}
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { OpsJobHeartbeat } from '@/api/admin/ops'

defineProps<{
  show: boolean
  jobHeartbeats: OpsJobHeartbeat[]
  formatTimeShort: (ts?: string | null) => string
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

const { t } = useI18n()
</script>
