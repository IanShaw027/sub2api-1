<template>
  <!-- Mixed Scheduling (only for antigravity accounts) -->
  <div v-if="show" class="flex items-center gap-2">
    <label class="flex cursor-pointer items-center gap-2">
      <input
        type="checkbox"
        v-model="mixedScheduling"
        class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
      />
      <span class="text-sm font-medium text-foreground">
        {{ t('admin.accounts.mixedScheduling') }}
      </span>
    </label>
    <div class="group relative">
      <span
        class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-surface-3 text-xs text-muted hover:bg-surface-3"
      >
        ?
      </span>
      <!-- Tooltip（向下显示避免被弹窗裁剪） -->
      <div
        class="pointer-events-none absolute left-0 top-full z-[100] mt-1.5 w-72 rounded bg-[var(--code-bg)] px-3 py-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100"
      >
        {{ t('admin.accounts.mixedSchedulingTooltip') }}
        <div
          class="absolute bottom-full left-3 border-4 border-transparent border-b-[var(--code-bg)]"
        ></div>
      </div>
    </div>
  </div>
  <div v-if="show" class="mt-3 flex items-center gap-2">
    <label class="flex cursor-pointer items-center gap-2">
      <input
        type="checkbox"
        v-model="allowOverages"
        class="h-4 w-4 rounded border-line text-accent focus:ring-accent"
      />
      <span class="text-sm font-medium text-foreground">
        {{ t('admin.accounts.allowOverages') }}
      </span>
    </label>
    <div class="group relative">
      <span
        class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full bg-surface-3 text-xs text-muted hover:bg-surface-3"
      >
        ?
      </span>
      <div
        class="pointer-events-none absolute left-0 top-full z-[100] mt-1.5 w-72 rounded bg-[var(--code-bg)] px-3 py-2 text-xs text-white opacity-0 transition-opacity group-hover:opacity-100"
      >
        {{ t('admin.accounts.allowOveragesTooltip') }}
        <div
          class="absolute bottom-full left-3 border-4 border-transparent border-b-[var(--code-bg)]"
        ></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// Antigravity-only "mixed scheduling" + "allow overages" toggle pair, lifted
// verbatim out of the host to shrink it. Both blocks share the same `show`
// gate (form.platform === 'antigravity') which stays a plain prop here.
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  show: boolean
}>()

const mixedScheduling = defineModel<boolean>('mixedScheduling', { required: true })
const allowOverages = defineModel<boolean>('allowOverages', { required: true })
</script>
