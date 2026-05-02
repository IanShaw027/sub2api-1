<template>
  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
    <div
      v-for="item in items"
      :key="item.key"
      class="card border border-gray-200 p-5 dark:border-dark-700"
    >
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <p class="text-sm font-medium text-gray-500 dark:text-gray-400">{{ item.label }}</p>
          <p class="mt-2 text-2xl font-semibold text-gray-900 dark:text-white">{{ item.value }}</p>
          <p v-if="item.hint" class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ item.hint }}</p>
        </div>
        <div
          :class="[
            'flex h-11 w-11 items-center justify-center rounded-2xl',
            toneClass(item.tone)
          ]"
        >
          <Icon v-if="item.icon" :name="item.icon as any" size="lg" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

type CardTone = 'primary' | 'success' | 'warning' | 'danger' | 'slate'

defineProps<{
  items: Array<{
    key: string
    label: string
    value: string | number
    hint?: string
    icon?: string
    tone?: CardTone
  }>
}>()

function toneClass(tone: CardTone = 'primary'): string {
  return {
    primary: 'bg-primary-50 text-primary-600 dark:bg-primary-900/30 dark:text-primary-200',
    success: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-200',
    warning: 'bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-200',
    danger: 'bg-rose-50 text-rose-600 dark:bg-rose-900/30 dark:text-rose-200',
    slate: 'bg-slate-100 text-slate-600 dark:bg-dark-700 dark:text-slate-200'
  }[tone]
}
</script>
