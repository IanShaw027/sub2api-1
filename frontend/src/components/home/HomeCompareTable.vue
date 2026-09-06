<template>
  <div class="compare-table glass-card">
    <div class="compare-head">
      <span>{{ t('home.comparison.headers.feature') }}</span>
      <span>{{ t('home.comparison.headers.official') }}</span>
      <span class="text-accent">{{ t('home.comparison.headers.us') }}</span>
    </div>
    <div v-for="row in rows" :key="row.feature" class="compare-row">
      <span class="compare-feature">{{ row.feature }}</span>
      <span class="text-muted">{{ row.official }}</span>
      <span class="compare-us">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="var(--success)" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
        {{ row.us }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

export interface CompareRow {
  feature: string
  official: string
  us: string
}

defineProps<{ rows: CompareRow[] }>()
const { t } = useI18n()
</script>

<style scoped>
.compare-table {
  border-radius: 16px;
  overflow: hidden;
}

.compare-head,
.compare-row {
  display: grid;
  grid-template-columns: 110px 1fr 1fr;
  gap: 16px;
  padding: 12px 22px;
  font-size: 13px;
  align-items: center;
  border-bottom: 1px solid var(--border);
}

.compare-row:last-child {
  border-bottom: 0;
}

.compare-head {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  letter-spacing: 0.06em;
  background: color-mix(in oklch, var(--surface-secondary) 45%, transparent);
}

.compare-feature {
  font-weight: 600;
}

.compare-us {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}

@media (max-width: 767px) {
  .compare-head,
  .compare-row {
    grid-template-columns: 1fr;
    gap: 4px;
    padding: 12px 16px;
  }

  .compare-head span:not(:first-child) {
    display: none;
  }
}
</style>
