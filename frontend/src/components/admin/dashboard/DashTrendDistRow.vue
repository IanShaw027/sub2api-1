<template>
  <div class="dash-row dash-row-trend">
    <section class="glass-card dash-panel">
      <header class="dash-panel-head">
        <div class="dash-panel-heading">
          <span class="dash-panel-title">{{ t('admin.dashboard.requestTrend') }}</span>
          <span class="dash-panel-sub">{{ trendSubtitle }}</span>
        </div>
        <div class="segmented segmented-sm dash-metric-switch" role="radiogroup">
          <button
            v-for="option in trendMetricOptions"
            :key="option.value"
            type="button"
            role="radio"
            class="segmented-item"
            :class="{ 'segmented-item-active': option.value === trendMetric }"
            :aria-checked="option.value === trendMetric"
            @click="trendMetric = option.value"
          >
            {{ option.label }}
          </button>
        </div>
      </header>
      <div v-if="trendBars.length" class="dash-bars">
        <div v-for="bar in trendBars" :key="bar.key" class="dash-bar-col">
          <div class="dash-bar" :title="bar.title" :style="{ height: bar.height, background: bar.fill }"></div>
          <span class="dash-bar-label">{{ bar.label }}</span>
        </div>
      </div>
      <div v-else class="empty-state dash-empty">
        <span class="empty-state-title">{{ t('admin.dashboard.noDataAvailable') }}</span>
      </div>
    </section>

    <section class="glass-card dash-panel">
      <header class="dash-panel-head">
        <span class="dash-panel-title">
          {{ distView === 'models' ? t('admin.dashboard.modelDistribution') : t('admin.dashboard.spendingRankingTitle') }}
        </span>
        <div class="segmented segmented-sm dash-metric-switch" role="radiogroup">
          <button
            type="button"
            role="radio"
            class="segmented-item"
            :class="{ 'segmented-item-active': distView === 'models' }"
            :aria-checked="distView === 'models'"
            @click="distView = 'models'"
          >
            {{ t('admin.dashboard.viewModelDistribution') }}
          </button>
          <button
            type="button"
            role="radio"
            class="segmented-item"
            :class="{ 'segmented-item-active': distView === 'users' }"
            :aria-checked="distView === 'users'"
            @click="distView = 'users'"
          >
            {{ t('admin.dashboard.viewSpendingRanking') }}
          </button>
        </div>
      </header>

      <div v-if="distRowsLoading" class="dash-panel-loading"><LoadingSpinner size="md" /></div>
      <div v-else-if="distRows.length" class="dash-dist">
        <component
          :is="row.userId ? 'button' : 'div'"
          v-for="(row, index) in distRows"
          :key="row.key"
          :type="row.userId ? 'button' : undefined"
          class="dash-dist-row"
          :class="{ 'dash-dist-row-action': !!row.userId }"
          @click="row.userId ? $emit('userRowClick', row.userId) : undefined"
        >
          <span class="dash-dist-tile" :style="{ background: row.tile }">
            <PlatformIcon :platform="row.platform" size="xs" />
          </span>
          <span class="dash-dist-body">
            <span class="dash-dist-line">
              <span class="dash-dist-name">{{ row.name }}</span>
              <span class="dash-dist-meta">${{ row.cost }} · {{ row.pct }}%</span>
            </span>
            <span class="dash-dist-track">
              <span
                class="dash-dist-fill"
                :style="{ width: `${row.pct}%`, opacity: Math.max(0.24, 1 - index * 0.16) }"
              ></span>
            </span>
          </span>
        </component>
      </div>
      <div v-else class="empty-state dash-empty">
        <span class="empty-state-title">{{ t('admin.dashboard.noDataAvailable') }}</span>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
/**
 * Row 3 of the admin dashboard: the request-trend bar chart and the
 * model-distribution / spending-ranking split panel. All derived rows and
 * bars are computed by the view; this component only renders them.
 */
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'

interface TrendBar {
  key: string
  height: string
  label: string
  title: string
  fill: string
}

interface DistRow {
  key: string
  name: string
  cost: string
  pct: number
  platform: GroupPlatform
  tile: string
  userId?: number
}

defineProps<{
  trendBars: TrendBar[]
  trendSubtitle: string
  trendMetricOptions: { value: 'requests' | 'tokens' | 'cost'; label: string }[]
  distRowsLoading: boolean
  distRows: DistRow[]
}>()

defineEmits<{
  userRowClick: [userId: number]
}>()

const trendMetric = defineModel<'requests' | 'tokens' | 'cost'>('trendMetric', { required: true })
const distView = defineModel<'models' | 'users'>('distView', { required: true })

const { t } = useI18n()
</script>

<style scoped>
.dash-row {
  display: grid;
  gap: 12px;
}

.dash-row-trend {
  grid-template-columns: 2fr 1fr;
}

.dash-panel {
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.dash-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 28px;
}

.dash-panel-heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.dash-panel-title {
  font-size: var(--fs-14);
  line-height: 1.3;
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.dash-panel-sub {
  font-size: var(--fs-12);
  line-height: 1.3;
  color: var(--muted);
}

.dash-panel-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 120px;
}

.dash-empty {
  flex: 1;
  padding: 24px 16px;
}

.dash-metric-switch {
  flex: none;
}

/* ------------------------------- bar chart ------------------------------- */
.dash-bars {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 150px;
  padding-top: 6px;
  margin-top: 2px;
}

.dash-bar-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  height: 100%;
  justify-content: flex-end;
}

.dash-bar {
  width: 100%;
  border-radius: var(--radius-bar);
}

.dash-bar-label {
  font-size: var(--fs-10-5);
  line-height: 1.3;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}

/* ---------------------------- model distribution ---------------------------- */
.dash-dist {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.dash-dist-row {
  display: flex;
  align-items: center;
  gap: 10px;
  border: 0;
  background: transparent;
  padding: 0;
  width: 100%;
  text-align: left;
  color: inherit;
  font: inherit;
}

.dash-dist-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.dash-dist-row-action {
  cursor: pointer;
}

.dash-dist-line {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: var(--fs-12-5);
  line-height: 1.3;
}

.dash-dist-tile {
  display: none;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: var(--radius-7);
  color: var(--on-tone);
  flex: none;
  box-shadow: inset 0 0 0 1px var(--ring-on-tone);
}

.dash-dist-name {
  font-family: var(--font-mono);
  font-size: var(--fs-12);
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
  min-width: 0;
}

.dash-dist-meta {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
  flex: none;
}

.dash-dist-track {
  display: block;
  height: 6px;
  border-radius: var(--radius-pill);
  background: var(--surface-tertiary);
  overflow: hidden;
}

.dash-dist-fill {
  display: block;
  height: 100%;
  border-radius: var(--radius-pill);
  background: linear-gradient(90deg, color-mix(in oklch, var(--accent) 70%, white), var(--accent));
}

/* ============================== responsive ============================== */
@media (max-width: 1180px) {
  .dash-row-trend {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 767px) {
  .dash-panel {
    padding: 0;
    gap: 0;
  }
  .dash-panel-head {
    padding: 12px 14px 10px;
    border-bottom: 1px solid var(--border);
  }
  .dash-panel-title {
    font-size: var(--fs-13);
  }
  .dash-dist,
  .dash-bars,
  .dash-empty,
  .dash-panel-loading {
    margin: 0;
  }
  .dash-bars {
    padding: 12px 14px 14px;
    height: 160px;
  }
  .dash-bar-label {
    font-size: var(--fs-9-5);
  }
  .dash-dist {
    gap: 0;
  }
  .dash-dist-row {
    flex-direction: row;
    align-items: center;
    gap: 10px;
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
  }
  .dash-dist-tile {
    display: inline-flex;
  }
  .dash-empty {
    margin: 14px;
  }
}
</style>
