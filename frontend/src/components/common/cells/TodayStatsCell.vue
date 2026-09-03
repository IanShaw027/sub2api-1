<template>
  <div class="cell-today-stats">
    <span class="cell-today-stats-primary">
      <slot name="primary">{{ formattedCount }}{{ unit ? ` ${unit}` : '' }}</slot>
    </span>
    <span class="cell-today-stats-secondary">
      <slot name="secondary">{{ formattedTokens }}<template v-if="formattedTokens && formattedCost"> · </template>{{ formattedCost }}</slot>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatNumberLocaleString, formatTokensK, formatCurrency } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    /** Request count for the day. Rendered with locale thousands separators. */
    count?: number | null
    /** Unit label appended after the count, e.g. "次". */
    unit?: string | null
    /** Token volume for the day (raw number, formatted as e.g. "18.2M"). */
    tokens?: number | null
    /** Cost for the day (raw number, formatted as currency). */
    cost?: number | null
    currency?: string
  }>(),
  {
    count: null,
    unit: null,
    tokens: null,
    cost: null,
    currency: 'USD'
  }
)

const formattedCount = computed(() =>
  props.count === null || props.count === undefined ? '0' : formatNumberLocaleString(props.count)
)
const formattedTokens = computed(() =>
  props.tokens === null || props.tokens === undefined ? '' : formatTokensK(props.tokens)
)
const formattedCost = computed(() =>
  props.cost === null || props.cost === undefined ? '' : formatCurrency(props.cost, props.currency)
)
</script>

<style scoped>
.cell-today-stats {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.cell-today-stats-primary {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
}

.cell-today-stats-secondary {
  font-size: 11.5px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
</style>
