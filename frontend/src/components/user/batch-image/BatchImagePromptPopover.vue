<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="batch-prompt-popover fixed z-[9999] rounded-lg border border-line bg-surface p-3 text-sm text-foreground shadow-xl ring-1 ring-black/5 "
      :style="style"
      @mouseenter="$emit('cancel-close')"
      @mouseleave="$emit('schedule-close')"
    >
      <div class="mb-2 flex items-center justify-between gap-3">
        <span class="text-xs font-medium text-muted">{{ t('batchImage.promptPopover.title') }}</span>
        <button
          type="button"
          class="rounded-md px-2 py-1 text-xs font-medium text-accent transition-colors hover:bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] focus:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklch,var(--accent)_30%,transparent)]"
          @click="$emit('copy')"
        >
          {{ t('common.copy') }}
        </button>
      </div>
      <p class="max-h-48 overflow-y-auto whitespace-pre-wrap break-words leading-6 selection:bg-[color-mix(in_oklch,var(--accent)_18%,transparent)] selection:text-foreground">
        {{ text }}
      </p>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

defineProps<{
  visible: boolean
  text: string
  style: Record<string, string>
}>()

defineEmits<{
  'cancel-close': []
  'schedule-close': []
  copy: []
}>()

const { t } = useI18n()
</script>

<style scoped>
.batch-prompt-popover {
  user-select: text;
}

.batch-prompt-popover p {
  scrollbar-width: thin;
}
</style>
