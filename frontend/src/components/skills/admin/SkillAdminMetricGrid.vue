<template>
  <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
    <div
      v-for="item in items"
      :key="item.key"
      class="card border border-line p-5 dark:border-dark-700"
    >
      <div class="flex items-start justify-between gap-4">
        <div class="min-w-0">
          <p class="text-sm font-medium text-ink-soft dark:text-ink-soft">{{ item.label }}</p>
          <p class="mt-2 text-2xl font-semibold text-ink dark:text-white">{{ item.value }}</p>
          <p v-if="item.hint" class="mt-2 text-xs text-ink-soft dark:text-ink-soft">{{ item.hint }}</p>
        </div>
        <div
          :class="[
            'flex h-11 w-11 items-center justify-center rounded-card',
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
    primary: 'bg-brand-50 text-brand-600 dark:bg-brand-900/30 dark:text-brand-200',
    success: 'bg-emerald-50 text-emerald-600 dark:bg-emerald-900/30 dark:text-emerald-200',
    warning: 'bg-amber-50 text-amber-600 dark:bg-amber-900/30 dark:text-amber-200',
    danger: 'bg-rose-50 text-rose-600 dark:bg-rose-900/30 dark:text-rose-200',
    slate: 'bg-page text-ink-body dark:bg-dark-700 dark:text-slate-200'
  }[tone]
}
</script>
