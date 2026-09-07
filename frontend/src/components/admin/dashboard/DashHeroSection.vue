<template>
  <section class="glass-card dash-hero">
    <span class="dash-hero-ring" aria-hidden="true"></span>

    <div class="dash-hero-copy">
      <span class="dash-hero-kicker">{{ heroKicker }}</span>
      <h1 class="dash-hero-title">
        {{ heroTitleLead }}<span class="dash-hero-accent">{{ formatNumber(stats.normal_accounts) }}</span>{{ t('admin.dashboard.heroTitleTail') }}
      </h1>
      <p class="dash-hero-desc">{{ heroDescription }}</p>
      <div class="dash-hero-actions">
        <Button @click="router.push('/admin/ops')">{{ t('admin.dashboard.viewOpsMonitor') }}</Button>
        <Button variant="secondary" @click="router.push('/admin/accounts')">
          {{ t('admin.dashboard.handleAbnormalAccounts') }}
          <span v-if="abnormalAccounts > 0" class="dash-hero-badge">{{ abnormalAccounts }}</span>
        </Button>
      </div>
    </div>

    <!-- Mobile (<=767px) hero variant: big today-requests figure + 48px bars -->
    <div class="dash-hero-mobile">
      <div class="dash-hero-mobile-top">
        <span class="dash-mini-label">{{ t('admin.dashboard.periodRequests') }}</span>
        <span class="dash-hero-chip" :class="`dash-hero-chip-${serviceTone}`">
          <span class="dash-pulse" :class="`dash-pulse-${serviceTone}`"></span>{{ serviceStatusLabel }}
        </span>
      </div>
      <div class="dash-hero-mobile-value">
        <span class="dash-hero-figure">{{ formatNumber(stats.total_requests) }}</span>
        <span v-if="requestsDelta" class="dash-hero-figure-delta" :class="`dash-delta-${requestsDelta.tone}`">
          {{ requestsDelta.text }}
        </span>
      </div>
      <div class="dash-hero-mobile-bars">
        <span
          v-for="bar in trendBars"
          :key="`m-${bar.key}`"
          :title="bar.title"
          :style="{ height: bar.height, background: bar.fill }"
        ></span>
      </div>
      <div class="dash-hero-mobile-axis">
        <span>{{ trendAxisLabels[0] }}</span>
        <span>{{ trendAxisLabels[1] }}</span>
        <span>{{ trendAxisLabels[2] }}</span>
      </div>
    </div>

    <div class="dash-hero-side">
      <div class="dash-hero-tools">
        <DateRangePicker
          class="dash-daterange"
          v-model:start-date="startDate"
          v-model:end-date="endDate"
          @change="onDateRangeChange"
        />
        <div class="dash-granularity">
          <Select v-model="granularity" :options="granularityOptions" @change="$emit('loadChartData')" />
        </div>
        <Button
          variant="secondary"
          class="dash-refresh"
          :loading="chartsLoading"
          @click="$emit('loadDashboardStats')"
        >
          {{ t('common.refresh') }}
        </Button>
      </div>

      <div class="dash-hero-stats">
        <div class="dash-hero-mini glass-inset">
          <span class="dash-mini-label">{{ t('admin.dashboard.serviceStatus') }}</span>
          <span class="dash-mini-status">
            <span class="dash-pulse" :class="`dash-pulse-${serviceTone}`"></span>{{ serviceStatusLabel }}
          </span>
          <span class="dash-mini-sub">
            {{ formatNumber(stats.normal_accounts) }} / {{ formatNumber(stats.total_accounts) }}
            {{ t('admin.dashboard.accounts') }}
          </span>
        </div>
        <div class="dash-hero-mini glass-inset">
          <span class="dash-mini-label">{{ t('admin.dashboard.realtimeRpm') }}</span>
          <span class="dash-mini-value">{{ formatNumber(stats.rpm) }}</span>
          <span class="dash-mini-sub">{{ t('admin.dashboard.currentSnapshot') }}</span>
        </div>
        <div class="dash-hero-mini glass-inset">
          <span class="dash-mini-label">{{ t('admin.dashboard.realtimeTpm') }}</span>
          <span class="dash-mini-value">{{ formatTokens(stats.tpm) }}</span>
          <span class="dash-mini-sub">
            {{ t('admin.dashboard.avgResponse') }} {{ formatDuration(stats.average_duration_ms) }}
          </span>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
/**
 * Dashboard hero: greeting/status copy, the date-range + granularity tools,
 * and the 3-tile realtime mini-stats. Extracted from DashboardView so the
 * view file stays under the size gate; all derived data is computed by the
 * view and passed in as props.
 */
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import type { DashboardStats } from '@/types'
import Button from '@/components/ui/Button.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import { formatDuration, formatNumber, formatTokens } from './useDashboardFormat'

interface TrendBar {
  key: string
  height: string
  label: string
  title: string
  fill: string
}

defineProps<{
  stats: DashboardStats
  heroKicker: string
  heroTitleLead: string
  heroDescription: string
  abnormalAccounts: number
  serviceTone: 'success' | 'warning' | 'danger'
  serviceStatusLabel: string
  requestsDelta: { text: string; tone: 'up' | 'down' } | null
  trendBars: TrendBar[]
  trendAxisLabels: string[]
  granularityOptions: { value: string; label: string }[]
  chartsLoading: boolean
  onDateRangeChange: (range: { startDate: string; endDate: string; preset: string | null }) => void
}>()

defineEmits<{
  loadChartData: []
  loadDashboardStats: []
}>()

const granularity = defineModel<'day' | 'hour'>('granularity', { required: true })
const startDate = defineModel<string>('startDate', { required: true })
const endDate = defineModel<string>('endDate', { required: true })

const { t } = useI18n()
const router = useRouter()
</script>

<style scoped>
.dash-hero {
  position: relative;
  border-radius: var(--radius-hero);
  padding: 26px 30px;
  display: grid;
  grid-template-columns: 1.25fr 1fr;
  gap: 24px;
  flex: none;
}

.dash-hero-deco {
  position: absolute;
  inset: 0;
  border-radius: inherit;
  overflow: hidden;
  pointer-events: none;
}

.dash-hero-dots {
  position: absolute;
  inset: 0;
  left: 45%;
  background: radial-gradient(color-mix(in oklch, var(--foreground) 7%, transparent) 1px, transparent 1.3px) 0 0 / 16px 16px;
  mask-image: linear-gradient(90deg, transparent, black 45%);
  -webkit-mask-image: linear-gradient(90deg, transparent, black 45%);
}

.dash-hero-orb {
  position: absolute;
  border-radius: var(--radius-circle);
}

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

/* gradient hairline ring — mobile hero variant only */
.dash-hero-ring {
  display: none;
  position: absolute;
  inset: 0;
  border-radius: inherit;
  padding: 1px;
  background: linear-gradient(135deg, color-mix(in oklch, var(--accent) 55%, transparent), transparent 35%, transparent 65%, color-mix(in oklch, var(--success) 45%, transparent));
  -webkit-mask: linear-gradient(white 0 0) content-box, linear-gradient(white 0 0);
  -webkit-mask-composite: xor;
  mask-composite: exclude;
  pointer-events: none;
}

.dash-hero-copy {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 10px;
}

.dash-hero-kicker {
  font-size: var(--fs-12-5);
  color: var(--muted);
  line-height: 1.3;
  margin-bottom: 3px;
}

.dash-hero-title {
  margin: 0;
  font-family: var(--display);
  font-size: var(--fs-30);
  font-weight: var(--fw-extrabold);
  letter-spacing: -0.03em;
  line-height: 1.15;
}

.dash-hero-accent {
  color: var(--accent);
}

.dash-hero-desc {
  margin: 0;
  font-size: var(--fs-13-5);
  color: var(--muted);
  max-width: 540px;
  line-height: 1.55;
}

.dash-hero-actions {
  display: flex;
  gap: 8px;
  margin-top: 6px;
}

.dash-hero-badge {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: var(--radius-pill);
  background: var(--danger);
  color: var(--on-tone);
  font-size: var(--fs-10-5);
  line-height: 1.3;
  font-weight: var(--fw-bold);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.dash-hero-side {
  position: relative;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 12px;
  min-width: 0;
}

.dash-hero-tools {
  position: static;
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  flex-wrap: wrap;
}

.dash-daterange :deep(.date-picker-trigger) {
  height: 28px;
  padding: 0 10px;
  gap: 6px;
  font-size: var(--fs-12);
}

.dash-granularity {
  width: 88px;
}

.dash-granularity :deep(.select-trigger) {
  height: 28px;
  padding: 0 8px 0 10px;
  gap: 6px;
  font-size: var(--fs-12);
}

.dash-hero-tools .dash-refresh {
  height: 28px;
  padding: 0 10px;
  font-size: var(--fs-12);
}

.dash-hero-stats {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 10px;
}

.dash-hero-mini {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 8px;
  min-width: 0;
  min-height: 111px;
}

.dash-mini-label {
  font-size: var(--fs-11-5);
  color: var(--muted);
  font-weight: var(--fw-semibold);
  line-height: 1.3;
}

.dash-mini-status {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: var(--fs-17);
  font-weight: var(--fw-bold);
  line-height: 1.3;
}

.dash-mini-value {
  font-family: var(--display);
  font-size: var(--fs-24);
  font-weight: var(--fw-extrabold);
  letter-spacing: -0.03em;
  font-variant-numeric: tabular-nums;
  line-height: 1;
}

.dash-mini-sub {
  font-size: var(--fs-11-5);
  color: var(--muted);
  line-height: 1.3;
}

.dash-delta-up {
  color: var(--success-text);
  font-weight: var(--fw-semibold);
}

.dash-delta-down {
  color: var(--danger-text);
  font-weight: var(--fw-semibold);
}

.dash-pulse {
  width: 8px;
  height: 8px;
  border-radius: var(--radius-pill);
  flex: none;
  animation: dash-pulse 2s infinite;
}

.dash-pulse-success {
  background: var(--success);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--success) 25%, transparent);
}

.dash-pulse-warning {
  background: var(--warning);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--warning) 25%, transparent);
}

.dash-pulse-danger {
  background: var(--danger);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--danger) 25%, transparent);
}

@keyframes dash-pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.45;
  }
}

/* --------------------------- mobile hero variant --------------------------- */
.dash-hero-mobile {
  display: none;
  position: relative;
  flex-direction: column;
  gap: 12px;
}

.dash-hero-mobile-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.dash-hero-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  padding: 0 8px;
  border-radius: var(--radius-pill);
  font-size: var(--fs-11-5);
  line-height: 1.3;
  font-weight: var(--fw-semibold);
  box-shadow: inset 0 0 0 1px color-mix(in oklch, currentColor 22%, transparent);
}

.dash-hero-chip-success {
  background: color-mix(in oklch, var(--success) 16%, transparent);
  color: var(--success-text);
}

.dash-hero-chip-warning {
  background: color-mix(in oklch, var(--warning) 18%, transparent);
  color: var(--warning-text);
}

.dash-hero-chip-danger {
  background: color-mix(in oklch, var(--danger) 14%, transparent);
  color: var(--danger-text);
}

.dash-hero-mobile-value {
  display: flex;
  align-items: flex-end;
  gap: 10px;
}

.dash-hero-figure {
  font-family: var(--display);
  font-size: var(--fs-40);
  font-weight: var(--fw-extrabold);
  letter-spacing: -0.04em;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.dash-hero-figure-delta {
  font-size: var(--fs-12);
  line-height: 1.3;
  font-weight: var(--fw-semibold);
  padding: 3px 7px;
  border-radius: var(--radius-6);
  margin-bottom: 4px;
  background: var(--surface-secondary);
}

.dash-hero-figure-delta.dash-delta-up {
  background: color-mix(in oklch, var(--success) 16%, transparent);
}

.dash-hero-figure-delta.dash-delta-down {
  background: color-mix(in oklch, var(--danger) 14%, transparent);
}

.dash-hero-mobile-bars {
  display: flex;
  align-items: flex-end;
  gap: 3px;
  height: 48px;
}

.dash-hero-mobile-bars span {
  flex: 1;
  border-radius: var(--radius-bar-sm);
}

.dash-hero-mobile-axis {
  display: flex;
  justify-content: space-between;
  font-size: var(--fs-11);
  line-height: 1.3;
  color: var(--muted);
  font-family: var(--font-mono);
}

/* ============================== responsive ============================== */
@media (max-width: 1180px) {
  .dash-hero {
    grid-template-columns: 1fr;
  }
  .dash-hero-tools {
    position: static;
    justify-content: flex-start;
  }
  .dash-hero-mini {
    min-height: 0;
  }
}

@media (max-width: 767px) {
  .dash-hero {
    padding: 18px 18px 16px;
    border-radius: var(--radius-18);
    gap: 12px;
  }
  .dash-hero-copy,
  .dash-hero-stats {
    display: none;
  }
  .dash-hero-mobile {
    display: flex;
  }
  .dash-hero-ring {
    display: block;
  }
  .dash-hero-dots {
    display: none;
  }
  .dash-hero-orb-accent {
    right: -70px;
    top: -90px;
    width: 240px;
    height: 240px;
  }
  .dash-hero-orb-success {
    display: none;
  }
  .dash-hero-tools {
    flex-wrap: wrap;
    justify-content: flex-start;
  }
}
</style>
