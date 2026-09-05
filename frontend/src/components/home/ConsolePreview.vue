<template>
  <!--
    Illustrative "console preview" window from the landing hero (design 01).
    Purely decorative: the numbers are sample values, so it is aria-hidden.
  -->
  <div class="console-wrap" aria-hidden="true">
    <div class="console-glow"></div>
    <div class="console-window glass-ring">
      <div class="console-bar">
        <span class="console-dots"><i></i><i></i><i></i></span>
        <span class="console-url">
          <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 11h14v10H5zM8 11V7a4 4 0 0 1 8 0v4" /></svg>
          {{ url }}
        </span>
        <span class="console-bar-spacer"></span>
      </div>
      <div class="console-body">
        <div class="console-head">
          <div class="console-uptime">
            <span class="console-label">{{ t('home.console.uptimeLabel') }}</span>
            <span class="console-uptime-value">99.98%</span>
          </div>
          <span class="badge badge-success console-ok"><span class="console-pulse"></span>{{ t('home.console.allOk') }}</span>
        </div>
        <div class="console-chart">
          <svg viewBox="0 0 400 88" preserveAspectRatio="none">
            <path :d="area" class="console-area" />
            <path :d="line" class="console-line" />
          </svg>
          <span class="console-chart-label">{{ t('home.console.rpmLabel') }}</span>
          <span class="console-chart-rpm"><i></i>842 RPM</span>
        </div>
        <div class="console-providers">
          <div v-for="p in providers" :key="p.name" class="glass-inset-muted console-provider">
            <div class="console-provider-top">
              <span class="platform-tile" :style="{ background: platformTileBackground(p.platform) }">
                <PlatformIcon :platform="p.platform" size="sm" />
              </span>
              <span class="console-provider-up">{{ p.up }}</span>
            </div>
            <div class="console-provider-meta">
              <span class="console-provider-name">{{ p.name }}</span>
              <span class="console-provider-models">{{ p.models }}</span>
            </div>
            <div class="console-bars">
              <span v-for="(h, i) in p.bars" :key="i" :style="{ height: h + '%' }"></span>
            </div>
            <span class="console-provider-lat">{{ p.lat }}</span>
          </div>
        </div>
        <div class="log-block console-logs">
          <div v-for="l in logs" :key="l.t" class="console-log" :style="{ opacity: l.op }">
            <span>{{ l.t }}</span>
            <span class="console-log-code">200</span>
            <span class="console-log-model">{{ l.m }}</span>
            <span class="console-log-ms">{{ l.ms }}</span>
          </div>
        </div>
      </div>
    </div>
    <div class="console-pill console-pill-tr">
      <span class="console-pill-icon is-warning">
        <svg width="11" height="11" viewBox="0 0 24 24" fill="currentColor"><path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z" /></svg>
      </span>
      {{ t('home.console.firstToken') }}
    </div>
    <div class="console-pill console-pill-bl">
      <span class="console-pill-icon is-accent">
        <svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6L9 17l-5-5" /></svg>
      </span>
      {{ t('home.console.stickyHit') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import { platformTileBackground } from '@/utils/platformTile'
import type { GroupPlatform } from '@/types'

const props = defineProps<{ apiBaseUrl?: string }>()
const { t } = useI18n()

const url = computed(() => {
  const raw = (props.apiBaseUrl || '').replace(/^https?:\/\//, '').replace(/\/+$/, '')
  return raw ? `${raw}/dashboard` : t('home.console.url')
})

// Deterministic pseudo-random sparkline (same generator as the design canvas).
function spark(seed: number, n: number, w: number, h: number) {
  let x = seed
  const v = Array.from({ length: n }, () => {
    x = (x * 9301 + 49297) % 233280
    return 0.25 + 0.75 * (x / 233280)
  })
  const pts = v.map((val, i) => [i * (w / (n - 1)), h - val * (h - 4)])
  const line = 'M' + pts.map((p) => p[0].toFixed(1) + ',' + p[1].toFixed(1)).join(' L')
  return { line, area: `${line} L${w},${h} L0,${h} Z` }
}

const { line, area } = spark(7, 24, 400, 88)

const providers = [
  { name: 'Claude', platform: 'anthropic' as GroupPlatform, models: 'Opus 4.1 · Sonnet 4.5', lat: '842 ms', up: '99.98%' },
  { name: 'OpenAI', platform: 'openai' as GroupPlatform, models: 'GPT-5 · Codex', lat: '1.1 s', up: '99.95%' },
  { name: 'Gemini', platform: 'gemini' as GroupPlatform, models: '2.5 Pro · Flash', lat: '620 ms', up: '100%' }
].map((p, i) => ({ ...p, bars: [42, 60, 55, 78, 66, 90, 72, 84].map((b) => (b + i * 9) % 100) }))

const logs = [
  { t: '09:42:11', m: 'claude-sonnet-4-5', ms: '812 ms', op: 1 },
  { t: '09:42:10', m: 'gpt-5-codex', ms: '1.1 s', op: 0.8 },
  { t: '09:42:09', m: 'gemini-2.5-pro', ms: '604 ms', op: 0.6 },
  { t: '09:42:07', m: 'claude-opus-4-1', ms: '1.4 s', op: 0.4 }
]
</script>

<style scoped>
.console-wrap {
  position: relative;
}

.console-glow {
  position: absolute;
  inset: -60px -40px -40px;
  background: radial-gradient(circle at 60% 40%, color-mix(in oklch, var(--accent) 26%, transparent), transparent 62%);
  filter: blur(12px);
  pointer-events: none;
}

.console-window {
  position: relative;
  border-radius: 18px;
  overflow: hidden;
  background: color-mix(in oklch, var(--surface) 76%, transparent);
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  box-shadow: var(--shadow), 0 50px 100px -50px color-mix(in oklch, var(--accent) 55%, transparent);
}

.console-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 40px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
}

.console-dots {
  display: flex;
  gap: 6px;
}

.console-dots i {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--surface-tertiary);
}

.console-url {
  margin: 0 auto;
  height: 24px;
  padding: 0 12px;
  border-radius: 7px;
  background: var(--surface-secondary);
  font-size: 11.5px;
  color: var(--muted);
  font-family: var(--font-mono);
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.console-bar-spacer {
  width: 42px;
}

.console-body {
  padding: 16px 18px 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.console-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.console-uptime {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.console-label {
  font-size: 12px;
  line-height: 1.3;
  font-weight: 600;
  color: var(--muted);
}

.console-uptime-value {
  font-family: var(--display);
  font-size: 38px;
  font-weight: 800;
  letter-spacing: -0.04em;
  line-height: 1;
  font-variant-numeric: tabular-nums;
}

.console-ok {
  height: 24px;
  padding: 0 9px;
}

.console-pulse {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--success);
  box-shadow: 0 0 0 3px color-mix(in oklch, var(--success) 25%, transparent);
  animation: s2a-pulse 2s infinite;
}

.console-chart {
  position: relative;
  height: 88px;
}

.console-chart svg {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.console-area {
  fill: color-mix(in oklch, var(--accent) 14%, transparent);
}

.console-line {
  fill: none;
  stroke: var(--accent);
  stroke-width: 2;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.console-chart-label {
  position: absolute;
  top: 0;
  left: 0;
  font-size: 11px;
  line-height: 1.3;
  color: var(--muted);
}

.console-chart-rpm {
  position: absolute;
  top: 0;
  right: 0;
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 600;
  line-height: 1.3;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.console-chart-rpm i {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--accent);
}

.console-providers {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}

.console-provider {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.console-provider-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.platform-tile {
  width: 24px;
  height: 24px;
  border-radius: 7px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: white;
  box-shadow: inset 0 0 0 1px color-mix(in oklch, white 14%, transparent);
  flex: none;
}

.console-provider-up {
  font-size: 11px;
  font-weight: 600;
  color: var(--success-text);
}

.console-provider-meta {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.console-provider-name {
  font-size: 13px;
  line-height: 1.3;
  font-weight: 600;
}

.console-provider-models {
  font-size: 11px;
  line-height: 1.3;
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.console-bars {
  display: flex;
  align-items: flex-end;
  gap: 2px;
  height: 18px;
}

.console-bars span {
  flex: 1;
  border-radius: 2px;
  background: color-mix(in oklch, var(--accent) 45%, transparent);
}

.console-provider-lat {
  font-family: var(--font-mono);
  font-size: 11px;
  line-height: 1.3;
  color: var(--muted);
}

.console-logs {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.console-log {
  display: grid;
  grid-template-columns: 60px 30px 1fr 52px;
  gap: 10px;
  line-height: 1.3;
}

.console-log-code {
  color: var(--success-text);
  font-weight: 600;
}

.console-log-model {
  color: var(--foreground);
}

.console-log-ms {
  text-align: right;
}

.console-pill {
  position: absolute;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 12px 0 8px;
  border-radius: 999px;
  background: color-mix(in oklch, var(--surface) 82%, transparent);
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  box-shadow: var(--shadow);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  z-index: 2;
}

.console-pill-tr {
  top: -16px;
  right: -22px;
}

.console-pill-bl {
  bottom: -14px;
  left: -26px;
}

.console-pill-icon {
  width: 20px;
  height: 20px;
  border-radius: 999px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.console-pill-icon.is-warning {
  background: color-mix(in oklch, var(--warning) 22%, transparent);
  color: var(--warning-text);
}

.console-pill-icon.is-accent {
  background: color-mix(in oklch, var(--accent) 16%, transparent);
  color: var(--accent);
}

@media (max-width: 1100px) {
  .console-pill-tr {
    right: 0;
  }

  .console-pill-bl {
    left: 0;
  }
}

@media (max-width: 480px) {
  .console-providers {
    gap: 6px;
  }

  .console-provider-models {
    display: none;
  }

  .console-uptime-value {
    font-size: 30px;
  }

  .console-glow {
    /* narrow viewports have ~20px horizontal margin around .console-wrap;
       the desktop -40px bleed pushes the decorative glow past the
       viewport edge and forces page-level horizontal scroll. */
    inset: -30px -20px -20px;
  }
}
</style>
