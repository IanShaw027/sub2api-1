<template>
  <span
    :class="[
      'inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold',
      toneClass
    ]"
  >
    {{ displayLabel }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

type BadgeMode = 'review' | 'visibility' | 'governance' | 'runtime' | 'settlement'

const props = withDefaults(defineProps<{
  status?: string | null
  label?: string | null
  mode?: BadgeMode
}>(), {
  status: '',
  label: '',
  mode: 'review'
})

const displayLabel = computed(() => {
  if (props.label && props.label.trim()) return props.label
  if (!props.status) return t('common.unknown')
  return props.status.replace(/[_-]/g, ' ')
})

const toneClass = computed(() => {
  const value = String(props.status || '').trim().toLowerCase()

  const variants: Record<BadgeMode, Record<string, string>> = {
    review: {
      approved: 'bg-success-soft text-success dark:bg-emerald-900/30 dark:text-emerald-200',
      rejected: 'bg-danger-soft text-danger dark:bg-rose-900/30 dark:text-rose-200',
      pending: 'bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-200'
    },
    visibility: {
      public: 'bg-accent-50 text-accent-700 dark:bg-accent-900/30 dark:text-accent-200',
      private: 'bg-page text-ink-soft dark:bg-dark-700 dark:text-slate-200',
      force_private: 'bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-200'
    },
    governance: {
      online: 'bg-success-soft text-success dark:bg-emerald-900/30 dark:text-emerald-200',
      disabled: 'bg-danger-soft text-danger dark:bg-rose-900/30 dark:text-rose-200',
      force_private: 'bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-200',
      draft: 'bg-page text-ink-soft dark:bg-dark-700 dark:text-slate-200'
    },
    runtime: {
      healthy: 'bg-success-soft text-success dark:bg-emerald-900/30 dark:text-emerald-200',
      warning: 'bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-200',
      critical: 'bg-danger-soft text-danger dark:bg-rose-900/30 dark:text-rose-200'
    },
    settlement: {
      settled: 'bg-success-soft text-success dark:bg-emerald-900/30 dark:text-emerald-200',
      ready: 'bg-accent-50 text-accent-700 dark:bg-accent-900/30 dark:text-accent-200',
      pending: 'bg-warning-soft text-warning dark:bg-amber-900/30 dark:text-amber-200',
      frozen: 'bg-page text-ink-soft dark:bg-dark-700 dark:text-slate-200',
      rejected: 'bg-danger-soft text-danger dark:bg-rose-900/30 dark:text-rose-200'
    }
  }

  return variants[props.mode][value] || 'bg-page text-ink-soft dark:bg-dark-700 dark:text-slate-200'
})
</script>
