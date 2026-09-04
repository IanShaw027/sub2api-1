<template>
  <div class="acct-today-stats-cell">
    <!-- Loading state -->
    <div v-if="props.loading && !props.stats" class="acct-today-stats-skeleton">
      <span class="acct-today-stats-skeleton-line acct-today-stats-skeleton-line--primary"></span>
      <span class="acct-today-stats-skeleton-line acct-today-stats-skeleton-line--secondary"></span>
    </div>

    <!-- Error state -->
    <div v-else-if="props.error && !props.stats" class="acct-today-stats-error" :title="props.error">
      {{ props.error }}
    </div>

    <!-- Stats data -->
    <TodayStatsCell
      v-else-if="props.stats"
      :count="props.stats.requests"
      :unit="t('admin.accounts.stats.requestsUnit')"
      :tokens="props.stats.tokens"
      :cost="props.stats.cost"
      :title="props.stats.user_cost != null ? `${t('admin.accounts.stats.userBilledShort')}: ${formatCurrency(props.stats.user_cost)}` : undefined"
    />

    <!-- No data -->
    <div v-else class="acct-today-stats-empty">-</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { WindowStats } from '@/types'
import { formatCurrency } from '@/utils/format'
import TodayStatsCell from '@/components/common/cells/TodayStatsCell.vue'

const props = withDefaults(
  defineProps<{
    stats?: WindowStats | null
    loading?: boolean
    error?: string | null
  }>(),
  {
    stats: null,
    loading: false,
    error: null
  }
)

const { t } = useI18n()
</script>

<style scoped>
.acct-today-stats-cell {
  min-width: 0;
}

.acct-today-stats-skeleton {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.acct-today-stats-skeleton-line {
  display: block;
  height: 10px;
  border-radius: 4px;
  background: color-mix(in oklch, var(--muted) 22%, transparent);
  animation: acct-today-stats-pulse 1.4s ease-in-out infinite;
}

.acct-today-stats-skeleton-line--primary {
  width: 48px;
}

.acct-today-stats-skeleton-line--secondary {
  width: 64px;
}

@keyframes acct-today-stats-pulse {
  0%, 100% {
    opacity: 0.5;
  }
  50% {
    opacity: 1;
  }
}

.acct-today-stats-error {
  font-size: 11.5px;
  color: var(--danger-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.acct-today-stats-empty {
  font-size: 13px;
  color: var(--muted);
}
</style>
