<template>
 <div class="dash-stats">
 <!-- ============================= StatCard grid ============================= -->
 <div class="dash-stat-grid">
 <StatCard
 :label="t('dashboard.apiKeys')"
 :value="stats?.total_api_keys || 0"
 :sub="`${stats?.active_api_keys || 0} ${t('common.active')}`"
 />
 <StatCard
 :label="t('dashboard.todayRequests')"
 :value="stats?.today_requests || 0"
 :sub="`${t('common.total')}: ${formatNumber(stats?.total_requests || 0)}`"
 >
 <template v-if="requestsSpark" #sparkline>
 <svg viewBox="0 0 100 28" preserveAspectRatio="none" class="dash-spark">
 <path :d="requestsSpark.area" class="dash-spark-area" />
 <path :d="requestsSpark.line" class="dash-spark-line" />
 </svg>
 </template>
 </StatCard>
 <StatCard
 :label="t('dashboard.todayTokens')"
 :value="formatTokens(stats?.today_tokens || 0)"
 :sub="`${t('dashboard.input')}: ${formatTokens(stats?.today_input_tokens || 0)} / ${t('dashboard.output')}: ${formatTokens(stats?.today_output_tokens || 0)}`"
 >
 <template v-if="tokensSpark" #sparkline>
 <svg viewBox="0 0 100 28" preserveAspectRatio="none" class="dash-spark">
 <path :d="tokensSpark.area" class="dash-spark-area" />
 <path :d="tokensSpark.line" class="dash-spark-line" />
 </svg>
 </template>
 </StatCard>
 <StatCard
 :label="t('dashboard.todayCost')"
 :value="`$${formatCost(stats?.today_actual_cost || 0)}`"
 :sub="`${t('common.total')}: $${formatCost(stats?.total_actual_cost || 0)}`"
 >
 <template v-if="costSpark" #sparkline>
 <svg viewBox="0 0 100 28" preserveAspectRatio="none" class="dash-spark">
 <path :d="costSpark.area" class="dash-spark-area" />
 <path :d="costSpark.line" class="dash-spark-line" />
 </svg>
 </template>
 </StatCard>

 <StatCard
 :label="t('dashboard.totalRequests')"
 :value="formatNumber(stats?.total_requests || 0)"
 :sub="t('dashboard.averageTime')"
 :delta="t('common.total')"
 delta-tone="neutral"
 />
 <StatCard
 :label="t('dashboard.totalTokens')"
 :value="formatTokens(stats?.total_tokens || 0)"
 :sub="`${t('dashboard.input')}: ${formatTokens(stats?.total_input_tokens || 0)} / ${t('dashboard.output')}: ${formatTokens(stats?.total_output_tokens || 0)}`"
 :delta="t('common.total')"
 delta-tone="neutral"
 />
 <StatCard
 :label="t('dashboard.totalCost')"
 :value="`$${formatCost(stats?.total_actual_cost || 0)}`"
 :sub="`${t('dashboard.standard')}: $${formatCost(stats?.total_cost || 0)}`"
 :delta="t('common.total')"
 delta-tone="neutral"
 />
 <StatCard
 :label="t('dashboard.avgResponse')"
 :value="formatDuration(stats?.average_duration_ms || 0)"
 :sub="t('dashboard.averageTime')"
 />
 </div>

 <!-- ==================== Platform split — 1fr 1fr 1fr panels ==================== -->
 <section v-if="!isSimple && platformCards.length > 0" class="dash-platform-section">
 <div class="dash-panel-head">
 <div class="dash-panel-heading">
 <span class="dash-panel-title">{{ t('dashboard.platformBreakdown') }}</span>
 <span class="dash-panel-sub">{{ t('dashboard.platformCount', { count: sortedPlatforms.length }) }}</span>
 </div>
 </div>
 <div class="dash-platform-grid">
 <div
 v-for="item in platformCards"
 :key="item.platform"
 :class="['glass-inset', 'dash-platform-tile', item.isOther ? 'dash-platform-tile-other' : '']"
 >
 <div class="dash-platform-tile-top">
 <span class="dash-platform-tile-name">
 <span
 v-if="!item.isOther"
 class="dash-brand-tile"
 :style="{ background: platformTileBackground(item.platform) }"
 >{{ platformInitial(item.platform) }}</span>
 {{ item.isOther ? t('dashboard.platformOther') : platformLabel(item.platform) }}
 </span>
 <span class="dash-platform-tile-cost" :title="t('dashboard.actual')">
 ${{ formatCost(item.total_actual_cost) }}
 </span>
 </div>
 <div class="dash-platform-tile-rows">
 <div class="dash-platform-tile-row">
 <span class="dash-muted">{{ t('dashboard.todayCost') }}</span>
 <span class="dash-mono">${{ formatCost(item.today_actual_cost) }}</span>
 </div>
 <div class="dash-platform-tile-row">
 <span class="dash-muted">{{ t('dashboard.requests') }}</span>
 <span class="dash-mono">
 {{ item.total_requests > 0 ? formatNumber(item.total_requests) : '-' }}
 </span>
 </div>
 <div class="dash-platform-tile-row">
 <span class="dash-muted">{{ t('dashboard.tokens') }}</span>
 <span class="dash-mono">
 {{ item.total_tokens > 0 ? formatTokens(item.total_tokens) : '-' }}
 </span>
 </div>
 </div>

 <div v-if="hasAnyLimit(item.quota) && !item.isOther" class="dash-quota-block">
 <p class="dash-quota-title">{{ t('dashboard.platformQuota.title') }}</p>
 <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
 <div v-if="quotaVal(item.quota, `${w}_limit_usd`) != null" class="dash-quota-row">
 <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
 <div class="dash-platform-tile-row">
 <span class="dash-quota-label">{{ t(`dashboard.platformQuota.${w}`) }}</span>
 <span class="dash-mono dash-quota-disabled">{{ t('dashboard.platformQuota.disabled') }}</span>
 </div>
 <ProgressBar :value="100" />
 </template>
 <template v-else>
 <div class="dash-platform-tile-row">
 <span class="dash-quota-label">{{ t(`dashboard.platformQuota.${w}`) }}</span>
 <span class="dash-mono">
 ${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}
 </span>
 </div>
 <ProgressBar :value="calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number)" />
 <p v-if="quotaVal(item.quota, `${w}_window_resets_at`)" class="dash-quota-reset">
 {{ t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) }}
 </p>
 </template>
 </div>
 </template>
 </div>
 </div>
 </div>
 </section>
 </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import StatCard from '@/components/ui/StatCard.vue'
import ProgressBar from '@/components/ui/ProgressBar.vue'
import { platformTileBackground, platformLabel as tilePlatformLabel } from '@/utils/platformTile'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem, TrendDataPoint } from '@/types'

interface FusedPlatformCard {
 platform: string
 total_actual_cost: number
 today_actual_cost: number
 total_requests: number
 total_tokens: number
 isOther?: boolean
 quota?: PlatformQuotaItem
}

const props = defineProps<{
 stats: UserStatsType
 isSimple: boolean
 platformQuotas?: PlatformQuotaItem[] | null
 trend?: TrendDataPoint[]
}>()
const { t } = useI18n()

const PLATFORM_LABELS: Record<string, string> = {
 anthropic: 'Claude',
 openai: 'OpenAI',
 gemini: 'Gemini',
 antigravity: 'Antigravity'
}

const platformLabel = (p: string) => PLATFORM_LABELS[p] ?? tilePlatformLabel(p) ?? p
const platformInitial = (p: string) => platformLabel(p).slice(0, 1).toUpperCase()

const sortedPlatforms = computed(() => {
 const list = props.stats?.by_platform ?? []
 return [...list].sort((a, b) => b.total_actual_cost - a.total_actual_cost)
})

// 处理"各平台之和 < 总值"的差值：后端按平台聚合时过滤了无法归属平台的行
// （group 与 account 都缺 platform）。这里把差值作为"其他"卡片显式展示，
// 避免 StatCard 总值与平台拆分加总对不上、用户困惑。
const OTHER_THRESHOLD = 0.0001
const platformCards = computed<FusedPlatformCard[]>(() => {
 // 建立 by_platform Map
 const byPlat = new Map<string, (typeof sortedPlatforms.value)[number]>()
 for (const item of props.stats?.by_platform ?? []) byPlat.set(item.platform, item)

 // 建立 quota Map
 const byQuota = new Map<string, PlatformQuotaItem>()
 for (const q of props.platformQuotas ?? []) byQuota.set(q.platform, q)

 // union 平台集合。后端 by_platform / quota 接口均不会返回 platform='__other__'，
 // 无需显式排除；__other__ 由下方差值补差逻辑单独追加。
 const platforms = new Set<string>([...byPlat.keys(), ...byQuota.keys()])

 const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'kiro']
 const cards: FusedPlatformCard[] = []

 for (const p of platforms) {
 const stat = byPlat.get(p)
 cards.push({
 platform: p,
 total_actual_cost: stat?.total_actual_cost ?? 0,
 today_actual_cost: stat?.today_actual_cost ?? 0,
 total_requests: stat?.total_requests ?? 0,
 total_tokens: stat?.total_tokens ?? 0,
 quota: byQuota.get(p),
 })
 }

 // 排序：按 PLATFORM_ORDER，未知平台按名称排序
 cards.sort((a, b) => {
 const ai = PLATFORM_ORDER.indexOf(a.platform)
 const bi = PLATFORM_ORDER.indexOf(b.platform)
 if (ai === -1 && bi === -1) return a.platform.localeCompare(b.platform)
 if (ai === -1) return 1
 if (bi === -1) return -1
 return ai - bi
 })

 // __other__ 补差逻辑：只对 by_platform 有 usage 数据的总和计算
 const total = props.stats?.total_actual_cost ?? 0
 const today = props.stats?.today_actual_cost ?? 0
 const sumTotal = cards.reduce((s, c) => s + c.total_actual_cost, 0)
 const sumToday = cards.reduce((s, c) => s + c.today_actual_cost, 0)
 const diffTotal = Math.max(0, total - sumTotal)
 const diffToday = Math.max(0, today - sumToday)

 if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
 cards.push({
 platform: '__other__',
 total_actual_cost: diffTotal,
 today_actual_cost: diffToday,
 total_requests: 0,
 total_tokens: 0,
 isOther: true,
 })
 }

 return cards
})

// Quota helpers

type QuotaWindow = 'daily' | 'weekly' | 'monthly'
type QuotaField = `${QuotaWindow}_limit_usd` | `${QuotaWindow}_usage_usd` | `${QuotaWindow}_window_resets_at`

function quotaVal(q: PlatformQuotaItem | undefined, key: QuotaField): PlatformQuotaItem[QuotaField] {
 return q?.[key]
}

function hasAnyLimit(q: PlatformQuotaItem | undefined): boolean {
 if (!q) return false
 return q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null
}

function calcPercent(usage: number, limit: number): number {
 if (!limit || limit <= 0) return 0
 return Math.min(100, Math.max(0, Math.round((usage / limit) * 100)))
}

// 与 formatBalance 一致使用 Intl.NumberFormat 做半偶舍入，避免 toFixed 在不同 JS 引擎
// 下偶发截断而非四舍五入（与后端展示精度不一致）。
const usdFormatter = new Intl.NumberFormat('en-US', {
 minimumFractionDigits: 2,
 maximumFractionDigits: 2,
})
function formatUsd(n: number): string {
 if (!Number.isFinite(n)) return '0.00'
 return usdFormatter.format(n)
}

function formatResetTime(iso: string | null | undefined): string {
 if (!iso) return ''
 const d = new Date(iso)
 if (Number.isNaN(d.getTime())) return iso
 return d.toLocaleString(undefined, {
 month: 'numeric',
 day: 'numeric',
 hour: '2-digit',
 minute: '2-digit',
 hour12: false,
 })
}

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatTokens = (t: number) => {
 if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
 if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
 return t.toString()
}
const formatDuration = (ms: number) => ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`

// ---- Sparklines: derived from the already-loaded trend series, never faked ----
function buildSparkline(values: number[]): { area: string; line: string } | null {
 if (!values || values.length < 2) return null
 const w = 100
 const h = 28
 const max = Math.max(...values)
 const min = Math.min(...values)
 const range = max - min || 1
 const stepX = w / (values.length - 1)
 const points = values.map((v, i) => {
 const x = i * stepX
 const y = h - ((v - min) / range) * h
 return `${x.toFixed(2)},${y.toFixed(2)}`
 })
 const line = `M${points.join(' L')}`
 const area = `${line} L${w},${h} L0,${h} Z`
 return { area, line }
}

const requestsSpark = computed(() => buildSparkline((props.trend ?? []).map((d) => d.requests)))
const tokensSpark = computed(() => buildSparkline((props.trend ?? []).map((d) => d.total_tokens)))
const costSpark = computed(() => buildSparkline((props.trend ?? []).map((d) => d.actual_cost)))
</script>
<style scoped>
.dash-stats {
 display: flex;
 flex-direction: column;
 gap: 12px;
}
.dash-stat-grid {
 display: grid;
 grid-template-columns: repeat(4, minmax(0, 1fr));
 gap: 12px;
}
.dash-stat-grid :deep(.ui-stat-card-sparkline > div) {
 margin-left: 0;
 width: 100%;
 height: 100%;
}
.dash-spark {
 width: 96px;
 height: 28px;
 flex: none;
 overflow: visible;
}
.dash-spark-area {
 fill: color-mix(in oklch, var(--accent) 14%, transparent);
}
.dash-spark-line {
 fill: none;
 stroke: var(--accent);
 stroke-width: 1.6;
 stroke-linecap: round;
 stroke-linejoin: round;
}

/* ==================== Platform split (1fr 1fr 1fr glass-inset panels) ==================== */
.dash-platform-section {
 display: flex;
 flex-direction: column;
 gap: 12px;
}
.dash-panel-head {
 display: flex;
 align-items: center;
 justify-content: space-between;
 gap: 12px;
}
.dash-panel-heading {
 display: flex;
 flex-direction: column;
 gap: 2px;
 min-width: 0;
}
.dash-panel-title {
 font-size: 14px;
 line-height: 1.3;
 font-weight: 600;
 color: var(--foreground);
}
.dash-panel-sub {
 font-size: 12px;
 line-height: 1.3;
 color: var(--muted);
}
.dash-platform-grid {
 display: grid;
 grid-template-columns: repeat(3, minmax(0, 1fr));
 gap: 12px;
}
.dash-platform-tile-other {
 border-style: dashed;
 background: var(--surface-secondary);
}
.dash-platform-tile-top {
 display: flex;
 align-items: center;
 justify-content: space-between;
 gap: 8px;
}
.dash-platform-tile-name {
 display: inline-flex;
 align-items: center;
 gap: 8px;
 font-size: 13px;
 font-weight: 600;
 color: var(--foreground);
}
.dash-brand-tile {
 display: inline-flex;
 align-items: center;
 justify-content: center;
 width: 20px;
 height: 20px;
 border-radius: 6px;
 color: #fff;
 font-size: 10.5px;
 font-weight: 700;
 box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.14);
 flex: none;
}
.dash-platform-tile-cost {
 font-family: var(--font-mono);
 font-size: 12.5px;
 color: var(--success-text);
}
.dash-platform-tile-rows {
 margin-top: 8px;
 display: flex;
 flex-direction: column;
 gap: 4px;
}
.dash-platform-tile-row {
 display: flex;
 align-items: center;
 justify-content: space-between;
 font-size: 12px;
}
.dash-muted {
 color: var(--muted);
}
.dash-mono {
 font-family: var(--font-mono);
 color: var(--foreground);
 font-variant-numeric: tabular-nums;
}
.dash-quota-block {
 margin-top: 10px;
 padding-top: 8px;
 border-top: 1px solid var(--border);
 display: flex;
 flex-direction: column;
 gap: 6px;
}
.dash-quota-title {
 margin: 0;
 font-size: 10px;
 font-weight: 600;
 text-transform: uppercase;
 letter-spacing: 0.06em;
 color: var(--muted);
}
.dash-quota-row {
 display: flex;
 flex-direction: column;
 gap: 3px;
}
.dash-quota-label {
 font-size: 12px;
 color: var(--foreground);
}
.dash-quota-disabled {
 color: var(--danger-text) !important;
}
.dash-quota-reset {
 margin: 0;
 font-size: 10px;
 color: var(--muted);
}
@media (max-width: 1100px) {
 .dash-stat-grid {
 grid-template-columns: 1fr 1fr;
 }
 .dash-platform-grid {
 grid-template-columns: 1fr 1fr;
 }
}
@media (max-width: 767px) {
 .dash-stat-grid {
 grid-template-columns: 1fr 1fr;
 }
 .dash-platform-grid {
 grid-template-columns: 1fr;
 }
}
@media (max-width: 480px) {
 .dash-stat-grid {
 grid-template-columns: 1fr;
 }
}
</style>
