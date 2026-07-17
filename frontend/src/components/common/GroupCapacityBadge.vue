<template>
  <div class="flex flex-col gap-1">
    <!-- 并发槽位 -->
    <div class="flex items-center gap-1">
      <span
        title="当前分组真实并发"
        :class="[
          'inline-flex min-w-[48px] items-center justify-center gap-1 rounded-md px-1.5 py-0.5 text-[10px] font-medium',
          capacityClass(concurrencyUsed, concurrencyMax)
        ]"
      >
        <svg class="h-2.5 w-2.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
        </svg>
        <span class="font-mono">{{ concurrencyUsed }}</span>
        <template v-if="concurrencyMax > 0">
          <span class="text-ink-faint dark:text-dark-500">/</span>
          <span class="font-mono">{{ concurrencyMax }}</span>
        </template>
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  concurrencyUsed: number
  concurrencyMax: number
}

withDefaults(defineProps<Props>(), {
  concurrencyUsed: 0,
  concurrencyMax: 0
})

function capacityClass(used: number, max: number): string {
  if (max > 0 && used >= max) {
    return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
  }
  if (used > 0) {
    return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  }
  return 'bg-line text-ink-body dark:bg-dark-800 dark:text-dark-400'
}
</script>
