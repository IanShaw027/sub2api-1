<template>
  <div class="card border border-line p-5 dark:border-dark-700">
    <div class="mb-5 flex items-center justify-between gap-3">
      <div>
        <h3 class="text-base font-semibold text-ink dark:text-white">{{ title }}</h3>
        <p v-if="description" class="mt-1 text-sm text-ink-soft dark:text-ink-soft">{{ description }}</p>
      </div>
      <Icon name="clock" size="md" class="text-ink-faint" />
    </div>

    <div v-if="steps.length" class="space-y-4">
      <div
        v-for="(step, index) in steps"
        :key="step.key"
        class="flex gap-4"
      >
        <div class="flex flex-col items-center">
          <span :class="['mt-1 h-3 w-3 rounded-full', dotClass(step.status)]"></span>
          <span
            v-if="index !== steps.length - 1"
            :class="['mt-2 h-full min-h-8 w-px', lineClass(step.status)]"
          ></span>
        </div>
        <div class="min-w-0 pb-2">
          <div class="flex flex-wrap items-center gap-2">
            <p class="text-sm font-medium text-ink dark:text-white">{{ step.title }}</p>
            <span v-if="step.time" class="text-xs text-ink-soft dark:text-ink-soft">{{ step.time }}</span>
          </div>
          <p v-if="step.description" class="mt-1 text-sm text-ink-body dark:text-ink-soft">{{ step.description }}</p>
        </div>
      </div>
    </div>

    <div v-else class="rounded-card border border-dashed border-line px-4 py-8 text-center text-sm text-ink-soft dark:border-dark-700 dark:text-ink-soft">
      {{ emptyText }}
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

type TimelineStatus = 'done' | 'current' | 'todo' | 'danger'

defineProps<{
  title: string
  description?: string
  emptyText?: string
  steps: Array<{
    key: string
    title: string
    description?: string
    time?: string | null
    status: TimelineStatus
  }>
}>()

function dotClass(status: TimelineStatus): string {
  return {
    done: 'bg-emerald-500',
    current: 'bg-brand-500',
    todo: 'bg-line dark:bg-dark-600',
    danger: 'bg-rose-500'
  }[status]
}

function lineClass(status: TimelineStatus): string {
  return {
    done: 'bg-emerald-200 dark:bg-emerald-900/40',
    current: 'bg-brand-200 dark:bg-brand-900/40',
    todo: 'bg-line dark:bg-dark-700',
    danger: 'bg-rose-200 dark:bg-rose-900/40'
  }[status]
}
</script>
