<template>
  <HelpTooltip class="dash-breakdown-trigger" width-class="w-56">
    <template #trigger>
      <span :aria-label="t('admin.dashboard.expandDetails')" :title="t('admin.dashboard.expandDetails')" tabindex="0">
        <DashSparkline v-if="path?.line" :path="path" />
        <Icon v-else name="infoCircle" size="sm" />
      </span>
    </template>
    <div class="dash-breakdown">
      <div v-for="item in items" :key="item.key" class="dash-breakdown-row">
        <span>{{ item.label }}</span>
        <span class="dash-breakdown-value" :class="item.textClass">{{ unit }}{{ format(item.value) }}</span>
      </div>
    </div>
  </HelpTooltip>
</template>

<script setup lang="ts">
/**
 * Stat-card sparkline that doubles as the trigger for a cost-split tooltip,
 * so the card keeps the prototype's shape while retaining the breakdown data.
 */
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'
import { useI18n } from 'vue-i18n'
import DashSparkline from './DashSparkline.vue'
import type { BreakdownItem, SparkPath } from './types'

withDefaults(defineProps<{
  path?: SparkPath
  items: BreakdownItem[]
  format: (value: number) => string
  unit?: string
}>(), { unit: '$' })
const { t } = useI18n()
</script>

<style scoped>
.dash-breakdown-trigger {
  margin-left: 0 !important;
  width: 100%;
  height: 100%;
}

.dash-breakdown {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dash-breakdown-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.dash-breakdown-value {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.dash-tone-success { color: var(--success-text); }
.dash-tone-warning { color: var(--warning-text); }
.dash-tone-muted { color: var(--muted); }
.dash-tone-accent { color: var(--accent); }
.dash-tone-danger { color: var(--danger-text); }
</style>
