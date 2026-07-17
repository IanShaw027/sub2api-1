<template>
  <div class="mt-5 grid grid-cols-2 gap-2">
    <div
      class="rounded-xl border border-line/70 bg-page/80 p-3 dark:border-dark-700/50 dark:bg-dark-900/40"
    >
      <div
        class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-ink-faint"
      >
        <Icon :name="primaryIcon" size="xs" />
        <span>{{ primaryLabel }}</span>
      </div>
      <div class="mt-1.5 font-mono text-lg font-bold tabular-nums text-ink">
        {{ primaryValue }}<span class="ml-0.5 text-xs font-normal text-ink-faint">{{ primaryUnit }}</span>
      </div>
    </div>
    <div
      class="rounded-xl border border-line/70 bg-page/80 p-3 dark:border-dark-700/50 dark:bg-dark-900/40"
    >
      <div
        class="flex items-center gap-1.5 text-[10px] font-semibold uppercase tracking-wider text-ink-faint"
      >
        <Icon :name="secondaryIcon" size="xs" />
        <span>{{ secondaryLabel }}</span>
      </div>
      <div class="mt-1.5 font-mono text-lg font-bold tabular-nums text-ink">
        {{ secondaryValue }}<span class="ml-0.5 text-xs font-normal text-ink-faint">{{ secondaryUnit }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  primaryLabel: string
  primaryValue: string
  primaryUnit: string
  primaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
  secondaryLabel: string
  secondaryValue: string
  secondaryUnit: string
  secondaryIcon: 'bolt' | 'globe' | 'clock' | 'link'
}>()
</script>
