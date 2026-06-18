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
      approved: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200',
      rejected: 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200',
      pending: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200'
    },
    visibility: {
      public: 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200',
      private: 'bg-slate-100 text-slate-700 dark:bg-dark-700 dark:text-slate-200',
      force_private: 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200'
    },
    governance: {
      online: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200',
      disabled: 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200',
      force_private: 'bg-orange-50 text-orange-700 dark:bg-orange-900/30 dark:text-orange-200',
      draft: 'bg-slate-100 text-slate-700 dark:bg-dark-700 dark:text-slate-200'
    },
    runtime: {
      healthy: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200',
      warning: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200',
      critical: 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
    },
    settlement: {
      settled: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200',
      ready: 'bg-sky-50 text-sky-700 dark:bg-sky-900/30 dark:text-sky-200',
      pending: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200',
      frozen: 'bg-slate-100 text-slate-700 dark:bg-dark-700 dark:text-slate-200',
      rejected: 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
    }
  }

  return variants[props.mode][value] || 'bg-slate-100 text-slate-700 dark:bg-dark-700 dark:text-slate-200'
})
</script>
