<template>
  <div class="dash-row dash-row-split">
    <section class="glass-card dash-panel">
      <header class="dash-panel-head">
        <span class="dash-panel-title">{{ t('admin.dashboard.platformHealth') }}</span>
        <button type="button" class="dash-panel-link" @click="router.push('/admin/accounts')">
          {{ t('admin.dashboard.manageAccounts') }} →
        </button>
      </header>
      <div v-if="platformHealth.length" class="dash-health">
        <div v-for="row in platformHealth" :key="row.platform" class="dash-health-row">
          <span class="dash-health-name">
            <span class="dash-dist-tile" :style="{ background: platformTileBackground(row.platform) }">
              <PlatformIcon :platform="row.platform" size="xs" />
            </span>
            <span class="dash-health-label">{{ platformLabel(row.platform) }}</span>
          </span>
          <span class="dash-health-bar">
            <span class="dash-health-ok" :style="{ width: `${row.okPct}%` }"></span>
            <span class="dash-health-warn" :style="{ width: `${row.limitPct}%` }"></span>
            <span class="dash-health-err" :style="{ width: `${row.errorPct}%` }"></span>
          </span>
          <span class="dash-health-text">{{ row.text }}</span>
        </div>
      </div>
      <div v-else class="empty-state dash-empty">
        <span class="empty-state-title">{{ t('admin.dashboard.noPlatformHealth') }}</span>
        <span class="empty-state-description">{{ t('admin.dashboard.noPlatformHealthDesc') }}</span>
      </div>
    </section>

    <section class="glass-card dash-panel">
      <header class="dash-panel-head">
        <span class="dash-panel-title">{{ t('admin.dashboard.recentEvents') }}</span>
        <button type="button" class="dash-panel-link" @click="router.push('/admin/ops')">
          {{ t('nav.ops') }} →
        </button>
      </header>
      <div v-if="recentEvents.length" class="dash-events">
        <div v-for="event in recentEvents" :key="event.id" class="dash-event">
          <span class="dash-event-time">{{ event.time }}</span>
          <span class="dash-event-dot" :class="`dash-event-dot-${event.tone}`"></span>
          <span class="dash-event-msg" :title="event.message">{{ event.message }}</span>
        </div>
      </div>
      <div v-else class="empty-state dash-empty">
        <span class="empty-state-title">{{ t('admin.dashboard.noEvents') }}</span>
        <span class="empty-state-description">{{ t('admin.dashboard.noEventsDesc') }}</span>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
/**
 * Row 4 of the admin dashboard: platform-health bars and the recent-events
 * feed. Derived rows are computed by the view; this component only renders.
 */
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformLabel, platformTileBackground } from '@/utils/platformTile'
import type { GroupPlatform } from '@/types'

interface HealthRow {
  platform: GroupPlatform
  okPct: number
  limitPct: number
  errorPct: number
  text: string
}

interface EventRow {
  id: number
  time: string
  tone: 'danger' | 'warning' | 'accent' | 'success'
  message: string
}

defineProps<{
  platformHealth: HealthRow[]
  recentEvents: EventRow[]
}>()

const { t } = useI18n()
const router = useRouter()
</script>

<style scoped>
.dash-row {
  display: grid;
  gap: 12px;
}

.dash-row-split {
  grid-template-columns: 1fr 1fr;
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

.dash-panel-title {
  font-size: var(--fs-14);
  line-height: 1.3;
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.dash-panel-link {
  font-size: var(--fs-12-5);
  line-height: 1.3;
  font-weight: var(--fw-semibold);
  color: var(--accent);
  text-decoration: none;
  white-space: nowrap;
  border: 0;
  background: transparent;
  padding: 0;
  cursor: pointer;
  font-family: inherit;
}

.dash-panel-link:hover {
  text-decoration: underline;
}

.dash-empty {
  flex: 1;
  padding: 24px 16px;
}

.dash-row-split .dash-empty {
  min-height: 151px;
}

/* ------------------------------ platform health ------------------------------ */
.dash-health {
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-height: 151px;
  overflow-y: auto;
}

.dash-health-row {
  display: grid;
  grid-template-columns: 120px 1fr 150px;
  align-items: center;
  gap: 14px;
  font-size: var(--fs-12-5);
  line-height: 1.3;
}

.dash-health-name {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
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

.dash-health-name .dash-dist-tile {
  display: inline-flex;
  width: 22px;
  height: 22px;
  border-radius: var(--radius-6);
}

.dash-health-label {
  font-weight: var(--fw-semibold);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.dash-health-bar {
  display: flex;
  height: 8px;
  border-radius: var(--radius-pill);
  overflow: hidden;
  background: var(--surface-tertiary);
  gap: 2px;
}

.dash-health-ok { background: var(--success); }
.dash-health-warn { background: var(--warning); }
.dash-health-err { background: var(--danger); }

.dash-health-text {
  color: var(--muted);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

/* -------------------------------- events -------------------------------- */
.dash-events {
  display: flex;
  flex-direction: column;
  max-height: 151px;
  overflow-y: auto;
}

.dash-event {
  display: grid;
  grid-template-columns: 44px 8px 1fr;
  gap: 10px;
  align-items: center;
  padding: 8px 0;
  border-top: 1px solid var(--border);
  font-size: var(--fs-12-5);
  line-height: 1.3;
}

.dash-event-time {
  font-family: var(--font-mono);
  font-size: var(--fs-11-5);
  line-height: 1.3;
  color: var(--muted);
}

.dash-event-dot {
  width: 8px;
  height: 8px;
  border-radius: var(--radius-pill);
}

.dash-event-dot-danger { background: var(--danger); }
.dash-event-dot-warning { background: var(--warning); }
.dash-event-dot-accent { background: var(--accent); }
.dash-event-dot-success { background: var(--success); }

.dash-event-msg {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* ============================== responsive ============================== */
@media (max-width: 1180px) {
  .dash-row-split {
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
  .dash-health,
  .dash-events,
  .dash-empty {
    margin: 0;
  }
  .dash-health {
    padding: 12px 14px;
  }
  .dash-health-row {
    grid-template-columns: 110px 1fr;
    grid-template-areas: 'name bar' 'text text';
    row-gap: 6px;
  }
  .dash-health-name { grid-area: name; }
  .dash-health-bar { grid-area: bar; }
  .dash-health-text {
    grid-area: text;
    text-align: left;
  }
  .dash-events {
    padding: 0 14px 8px;
  }
  .dash-empty {
    margin: 14px;
  }
}
</style>
