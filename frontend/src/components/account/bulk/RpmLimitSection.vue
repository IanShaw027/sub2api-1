<template>
  <div v-if="show" class="border-t border-line pt-4">
    <div class="mb-3 flex items-center justify-between">
      <label
        id="bulk-edit-rpm-limit-label"
        class="input-label mb-0"
        for="bulk-edit-rpm-limit-enabled"
      >
        {{ t('admin.accounts.quotaControl.rpmLimit.label') }}
      </label>
      <input
        v-model="enableRpmLimit"
        id="bulk-edit-rpm-limit-enabled"
        type="checkbox"
        aria-controls="bulk-edit-rpm-limit-body"
        class="rounded border-line text-accent focus:ring-accent"
      />
    </div>

    <div
      id="bulk-edit-rpm-limit-body"
      :class="!enableRpmLimit && 'pointer-events-none opacity-50'"
      role="group"
      aria-labelledby="bulk-edit-rpm-limit-label"
    >
      <div class="mb-3 flex items-center justify-between">
        <span class="text-sm text-foreground">{{ t('admin.accounts.quotaControl.rpmLimit.hint') }}</span>
        <ToggleSwitch v-model="rpmLimitEnabled" />
      </div>

      <div v-if="rpmLimitEnabled" class="space-y-3">
        <div>
          <label class="input-label text-xs">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpm') }}</label>
          <input
            v-model.number="bulkBaseRpm"
            type="number"
            min="1"
            max="1000"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.rpmLimit.baseRpmPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.baseRpmHint') }}</p>
        </div>

        <div>
          <label class="input-label text-xs">{{ t('admin.accounts.quotaControl.rpmLimit.strategy') }}</label>
          <div class="flex gap-2">
            <button
              type="button"
              @click="bulkRpmStrategy = 'tiered'"
              :class="[
 'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
 bulkRpmStrategy === 'tiered'
 ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
            >
              {{ t('admin.accounts.quotaControl.rpmLimit.strategyTiered') }}
            </button>
            <button
              type="button"
              @click="bulkRpmStrategy = 'sticky_exempt'"
              :class="[
 'flex-1 rounded-lg px-3 py-2 text-sm font-medium transition-all',
 bulkRpmStrategy === 'sticky_exempt'
 ? 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent'
 : 'bg-surface-2 text-muted hover:bg-surface-3'
 ]"
            >
              {{ t('admin.accounts.quotaControl.rpmLimit.strategyStickyExempt') }}
            </button>
          </div>
        </div>

        <div v-if="bulkRpmStrategy === 'tiered'">
          <label class="input-label text-xs">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBuffer') }}</label>
          <input
            v-model.number="bulkRpmStickyBuffer"
            type="number"
            min="1"
            step="1"
            class="input"
            :placeholder="t('admin.accounts.quotaControl.rpmLimit.stickyBufferPlaceholder')"
          />
          <p class="input-hint">{{ t('admin.accounts.quotaControl.rpmLimit.stickyBufferHint') }}</p>
        </div>
      </div>
    </div>

    <!-- 用户消息限速模式（独立于 RPM 开关，始终可见） -->
    <div class="mt-4">
      <label class="input-label">{{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueue') }}</label>
      <p class="mt-1 text-xs text-muted mb-2">
        {{ t('admin.accounts.quotaControl.rpmLimit.userMsgQueueHint') }}
      </p>
      <div class="flex space-x-2">
        <button
          type="button"
          v-for="opt in umqModeOptions"
          :key="opt.value"
          @click="userMsgQueueMode = userMsgQueueMode === opt.value ? null : opt.value"
          :class="[
 'px-3 py-1.5 text-sm rounded-md border transition-colors',
 userMsgQueueMode === opt.value
 ? 'bg-accent text-white border-accent'
 : 'bg-surface text-foreground border-line hover:bg-surface-2'
 ]"
        >
          {{ opt.label }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'

interface UmqModeOption {
  value: string
  label: string
}

interface Props {
  show: boolean
  umqModeOptions: UmqModeOption[]
}

defineProps<Props>()

const enableRpmLimit = defineModel<boolean>('enableRpmLimit', { required: true })
const rpmLimitEnabled = defineModel<boolean>('rpmLimitEnabled', { required: true })
const bulkBaseRpm = defineModel<number | null>('bulkBaseRpm', { required: true })
const bulkRpmStrategy = defineModel<'tiered' | 'sticky_exempt'>('bulkRpmStrategy', { required: true })
const bulkRpmStickyBuffer = defineModel<number | null>('bulkRpmStickyBuffer', { required: true })
const userMsgQueueMode = defineModel<string | null>('userMsgQueueMode', { required: true })

const { t } = useI18n()
</script>
