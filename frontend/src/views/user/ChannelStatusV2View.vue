<template>
  <AppLayout>
    <div class="monitor-page">
      <!-- ============================= 1 · Hero ============================= -->
      <section class="glass-card dash-hero">
        <span class="dash-hero-deco" aria-hidden="true">
          <span class="dash-hero-dots"></span>
          <span class="dash-hero-orb dash-hero-orb-accent"></span>
          <span class="dash-hero-orb dash-hero-orb-success"></span>
        </span>

        <div class="dash-hero-copy">
          <span class="dash-hero-kicker">{{ t('channelMonitorV2.heroKicker') }}</span>
          <h1 class="dash-hero-title">{{ t('channelMonitorV2.title') }}</h1>
          <p class="dash-hero-desc">{{ t('channelMonitorV2.heroDescription') }}</p>
          <div class="dash-hero-actions">
            <button
              type="button"
              class="dash-refresh-btn rounded-[var(--radius-sm)] text-xs font-semibold"
              :disabled="loading"
              :title="t('common.refresh')"
              @click="reload(false)"
            >
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>
      </section>

      <!-- ====================== 2 · KPI StatCard row ====================== -->
      <section
        v-if="snapshot"
        class="monitor-stat-grid"
        :class="showThroughput ? 'monitor-stat-grid-5' : 'monitor-stat-grid-4'"
        :aria-label="t('channelMonitorV2.summaryAria')"
      >
        <StatCard
          :label="t('channelMonitorV2.metrics.successRate')"
          :value="formatPercent(1 - snapshot.metrics.error_rate)"
          :sub="t('channelMonitorV2.metrics.errorRateValue', { value: formatPercent(snapshot.metrics.error_rate) })"
          :delta="healthDeltaLabel(snapshot.health.error_rate)"
          :delta-tone="healthDeltaTone(snapshot.health.error_rate)"
        />
        <StatCard
          :label="t('channelMonitorV2.metrics.ttftP50')"
          :value="formatMs(snapshot.metrics.ttft.p50_ms)"
          :sub="latencyKpiSecondary(snapshot.metrics.ttft)"
          :title="latencyDetail(snapshot.metrics.ttft)"
          :delta="healthDeltaLabel(snapshot.health.ttft)"
          :delta-tone="healthDeltaTone(snapshot.health.ttft)"
        />
        <StatCard
          v-if="showThroughput"
          :label="t('channelMonitorV2.metrics.tps')"
          :value="formatTps(snapshot.metrics.tpm)"
          :sub="t('channelMonitorV2.metrics.tpsDetail')"
          :title="exactTps(snapshot.metrics.tpm)"
        />
        <StatCard
          :label="t('channelMonitorV2.metrics.cacheRate')"
          :value="formatPercent(snapshot.metrics.cache_rate)"
          :sub="t('channelMonitorV2.metrics.cacheDetail')"
          :delta="healthDeltaLabel(snapshot.health.cache || snapshot.health.overall)"
          :delta-tone="healthDeltaTone(snapshot.health.cache || snapshot.health.overall)"
        />
        <StatCard
          v-if="showThroughput"
          :label="t('channelMonitorV2.metrics.rpm')"
          :value="formatRate(snapshot.metrics.rpm)"
          :sub="t('channelMonitorV2.metrics.rpmDetail')"
          :title="exactRate(snapshot.metrics.rpm)"
        />
      </section>
      <section
        v-else-if="loading"
        class="monitor-stat-grid"
        :class="showThroughput ? 'monitor-stat-grid-5' : 'monitor-stat-grid-4'"
        aria-hidden="true"
      >
        <div
          v-for="i in (showThroughput ? 5 : 4)"
          :key="i"
          class="h-[120px] animate-pulse rounded-[var(--radius-card)] bg-surface-2"
        />
      </section>

      <!-- ====================== 3 · Status + filter toolbar ====================== -->
      <MonitorToolbar
        :loading="loading"
        :refreshing="refreshing"
        :has-snapshot="!!snapshot"
        :data-through="snapshot?.coverage.data_through ?? null"
        :coverage-complete="snapshot?.coverage.coverage_complete ?? true"
        :bootstrap-active="bootstrapActive"
        :bootstrap-percent="bootstrapPercent"
        :range="filter.range"
        :ranges="ranges"
        :platforms="filter.platforms"
        :platform-options="platformOptions"
        :group-ids="selectedGroupIds"
        :group-options="groupOptions"
        :models="filter.models"
        :model-options="modelOptions"
        :has-dimension-filter="hasDimensionFilter"
        :matrix-group-by="matrixGroupBy"
        :matrix-group-options="matrixGroupOptions"
        :trend-view="trendView"
        :trend-view-options="trendViewOptions"
        :health-mode="healthMode"
        :health-mode-options="healthModeOptions"
        @clear="clearDimensions"
        @update:range="setRange"
        @update:platforms="filter.platforms = $event"
        @update:group-ids="selectedGroupIds = $event"
        @update:models="filter.models = $event"
        @update:matrix-group-by="matrixGroupBy = $event"
        @update:trend-view="trendView = $event"
        @update:health-mode="healthMode = $event"
      />

      <!-- ====================== 4 · Trend chart / relay pulse matrix ====================== -->
      <div class="relative min-h-[320px]">
        <MonitorTrendChart
          v-if="trendView === 'line'"
          :trend="snapshot?.trend || []"
          :coverage="snapshot?.coverage || null"
          :loading="loading && !snapshot"
        />
        <RelayPulseMatrix
          v-else-if="matrix"
          :rows="matrixRows"
          :coverage="matrix.coverage"
          :health-mode="healthMode"
          :show-throughput="showThroughput"
        />
        <div
          v-else-if="loading"
          class="glass-card flex min-h-[320px] items-center justify-center text-sm text-muted"
        >
          <span class="animate-pulse">{{ t('common.loading') }}</span>
        </div>
      </div>

      <!-- ====================== 5 · Models / errors / users tabs ====================== -->
      <MonitorDataTabs
        :active-tab="activeTab"
        :tabs="tabs"
        :model-rows="modelRows"
        :error-rows="errorRows"
        :user-rows="userRows"
        :show-throughput="showThroughput"
        :is-admin="isAdmin"
        :tab-loading="tabLoading"
        :active-rows-empty="activeRowsEmpty"
        :bootstrap-active="bootstrapActive"
        :expanded-errors="expandedErrors"
        :format-percent="formatPercent"
        :format-ms="formatMs"
        :format-tps="formatTps"
        :exact-tps="exactTps"
        :format-rate="formatRate"
        :latency-detail="latencyDetail"
        :status-dot="statusDot"
        :error-label="errorLabel"
        @update:active-tab="activeTab = $event"
        @drill-model="drillModel"
        @toggle-error="toggleError"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import StatCard from '@/components/ui/StatCard.vue'
import MonitorToolbar from '@/features/channel-monitor-v2/MonitorToolbar.vue'
import MonitorTrendChart from '@/features/channel-monitor-v2/MonitorTrendChart.vue'
import RelayPulseMatrix from '@/features/channel-monitor-v2/RelayPulseMatrix.vue'
import MonitorDataTabs from '@/features/channel-monitor-v2/MonitorDataTabs.vue'
import { useChannelMonitorV2 } from '@/features/channel-monitor-v2/useChannelMonitorV2'
import type { HealthState } from '@/api/channelMonitorV2'

const { t } = useI18n()

const {
  isAdmin,
  showThroughput,
  ranges,
  tabs,
  matrixGroupOptions,
  healthModeOptions,
  trendViewOptions,
  filter,
  activeTab,
  matrixGroupBy,
  healthMode,
  trendView,
  snapshot,
  matrix,
  modelRows,
  errorRows,
  userRows,
  loading,
  tabLoading,
  refreshing,
  expandedErrors,
  hasDimensionFilter,
  platformOptions,
  groupOptions,
  modelOptions,
  selectedGroupIds,
  activeRowsEmpty,
  bootstrapActive,
  bootstrapPercent,
  matrixRows,
  reload,
  setRange,
  clearDimensions,
  drillModel,
  formatRate,
  exactRate,
  formatTps,
  exactTps,
  formatPercent,
  formatMs,
  latencyDetail,
  latencyKpiSecondary,
  statusDot,
  errorLabel,
  toggleError,
} = useChannelMonitorV2()

/**
 * StatCard only exposes a 2-tone delta pill (up/down + neutral) — there is no
 * distinct "warning" tone. We map the 3-state health score onto it: healthy
 * -> up (success), critical -> down (danger), warning/unknown -> neutral
 * (muted). This loses the amber warning color on the KPI row; the finer-grained
 * per-cell health-score coloring is preserved elsewhere (RelayPulseMatrix dots,
 * models/users table status dots). See deviations report for the suggestion to
 * add a `warn` tone to components/ui/StatCard.vue.
 */
function healthDeltaLabel(state?: HealthState) {
  if (!state) return undefined
  return t(`channelMonitorV2.healthState.${state}`)
}
function healthDeltaTone(state?: HealthState): 'up' | 'down' | 'neutral' {
  if (state === 'healthy') return 'up'
  if (state === 'critical') return 'down'
  return 'neutral'
}
</script>

<style scoped>
.monitor-page { display: flex; flex-direction: column; gap: 16px; padding-bottom: 48px; }

/* ================================= hero =================================
   Mirrors views/admin/DashboardView.vue / views/user/DashboardView.vue's hero
   recipe (glass-ui-redesign convention: each page inlines its own copy of
   `.dash-hero*` rather than a shared layout component). Simplified to a
   single-column copy since this page's KPIs live in their own StatCard row
   instead of a hero side-panel. */
.dash-hero {
  position: relative;
  border-radius: var(--radius-hero);
  padding: 26px 30px;
  display: grid;
  grid-template-columns: 1fr;
  flex: none;
  min-height: 219px;
}
.dash-hero-deco { position: absolute; inset: 0; border-radius: inherit; overflow: hidden; pointer-events: none; }
.dash-hero-dots {
  position: absolute;
  inset: 0;
  left: 45%;
  background: radial-gradient(color-mix(in oklch, var(--foreground) 7%, transparent) 1px, transparent 1.3px) 0 0 / 16px 16px;
  mask-image: linear-gradient(90deg, transparent, black 45%);
  -webkit-mask-image: linear-gradient(90deg, transparent, black 45%);
}
.dash-hero-orb { position: absolute; border-radius: 50%; }
.dash-hero-orb-accent {
  right: -90px;
  top: -140px;
  width: 380px;
  height: 380px;
  background: radial-gradient(circle at 35% 35%, color-mix(in oklch, var(--accent) 60%, white) 0%, color-mix(in oklch, var(--accent) 30%, transparent) 42%, transparent 70%);
}
.dash-hero-orb-success {
  right: 220px;
  bottom: -160px;
  width: 260px;
  height: 260px;
  background: radial-gradient(circle at 50% 50%, color-mix(in oklch, var(--success) 30%, transparent) 0%, transparent 65%);
}
.dash-hero-copy { position: relative; display: flex; flex-direction: column; justify-content: flex-start; gap: 10px; }
.dash-hero-kicker { font-size: 12.5px; color: var(--muted); line-height: 1.3; margin-bottom: 3px; }
.dash-hero-title { margin: 0; font-family: var(--display); font-size: 30px; font-weight: 800; letter-spacing: -0.03em; line-height: 1.15; }
.dash-hero-desc { margin: 0; font-size: 13.5px; color: var(--muted); max-width: 540px; line-height: 1.55; }
.dash-hero-actions { display: flex; gap: 8px; margin-top: 6px; }
.dash-refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 10px;
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--foreground);
  border: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease, transform 0.1s ease;
  flex: none;
  white-space: nowrap;
}
.dash-refresh-btn:hover:not(:disabled) { background: var(--surface); border-color: color-mix(in oklch, var(--foreground) 18%, transparent); }
.dash-refresh-btn:active:not(:disabled) { transform: scale(0.98); }
.dash-refresh-btn:disabled { opacity: 0.6; cursor: not-allowed; }

/* ============================= KPI StatCard grid ============================= */
.monitor-stat-grid { display: grid; gap: 12px; }
.monitor-stat-grid-4 { grid-template-columns: repeat(4, minmax(0, 1fr)); }
.monitor-stat-grid-5 { grid-template-columns: repeat(5, minmax(0, 1fr)); }
@media (max-width: 1200px) {
  .monitor-stat-grid-5 { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (max-width: 1100px) {
  .monitor-stat-grid-4, .monitor-stat-grid-5 { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
@media (max-width: 767px) {
  .monitor-page { gap: 14px; }
  .dash-hero { padding: 18px 18px 16px; border-radius: 18px; }
  .dash-hero-title { font-size: 22px; }
}
@media (max-width: 480px) {
  .monitor-stat-grid-4, .monitor-stat-grid-5 { grid-template-columns: 1fr; }
}
</style>
