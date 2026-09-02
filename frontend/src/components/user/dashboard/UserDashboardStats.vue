<template>
 <div class="dash-stats">
 <!-- Row 1: Core Stats -->
 <div class="dash-stat-grid">
 <StatCard
 v-if="!isSimple"
 :label="t('dashboard.balance')"
 :value="`$${formatBalance(balance)}`"
 :sub="t('common.available')"
 class="cursor-pointer"
 @click="emit('balance-history')"
 />
 <StatCard
 :label="t('dashboard.apiKeys')"
 :value="stats?.total_api_keys || 0"
 :sub="`${stats?.active_api_keys || 0} ${t('common.active')}`"
 />
 <StatCard
 :label="t('dashboard.todayRequests')"
 :value="stats?.today_requests || 0"
 :sub="`${t('common.total')}: ${formatNumber(stats?.total_requests || 0)}`"
 />
 <StatCard
 :label="t('dashboard.todayCost')"
 :value="`$${formatCost(stats?.today_actual_cost || 0)}`"
 :sub="`${t('common.total')}: $${formatCost(stats?.total_actual_cost || 0)}`"
 />
 </div>

 <!-- Row 2: Token Stats -->
 <div class="dash-stat-grid">
 <StatCard
 :label="t('dashboard.todayTokens')"
 :value="formatTokens(stats?.today_tokens || 0)"
 :sub="`${t('dashboard.input')}: ${formatTokens(stats?.today_input_tokens || 0)} / ${t('dashboard.output')}: ${formatTokens(stats?.today_output_tokens || 0)}`"
 />
 <StatCard
 :label="t('dashboard.totalTokens')"
 :value="formatTokens(stats?.total_tokens || 0)"
 :sub="`${t('dashboard.input')}: ${formatTokens(stats?.total_input_tokens || 0)} / ${t('dashboard.output')}: ${formatTokens(stats?.total_output_tokens || 0)}`"
 />
 <StatCard :label="t('dashboard.performance')" :value="liveRpmUsed" :sub="liveRpmLabel">
 <template #sparkline>
 <div class="dash-perf">
 <span>{{ formatTokens(stats?.rpm || 0) }} {{ t('dashboard.avgRpm') }}</span>
 <span>{{ currentConcurrency }} {{ t('dashboard.currentConcurrency') }}</span>
 </div>
 </template>
 </StatCard>
 <StatCard
 :label="t('dashboard.avgResponse')"
 :value="formatDuration(stats?.average_duration_ms || 0)"
 :sub="t('dashboard.averageTime')"
 />
 </div>

 <!-- Row 3: Per-platform breakdown -->
 <GlassCard v-if="!isSimple && platformCards.length > 0" padding="md">
 <div class="mb-3 flex items-center justify-between">
 <h3 class="text-sm font-semibold text-foreground">{{ t('dashboard.platformBreakdown') }}</h3>
 <span class="text-xs text-muted">
 {{ t('dashboard.platformCount', { count: sortedPlatforms.length }) }}
 </span>
 </div>
 <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
 <div
 v-for="item in platformCards"
 :key="item.platform"
 :class="[
 'rounded-lg border p-3',
 item.isOther
 ? 'border-dashed border-line bg-surface-2'
 : 'border-line'
 ]"
 >
 <div class="flex items-center justify-between">
 <span class="text-sm font-semibold text-foreground">
 {{ item.isOther ? t('dashboard.platformOther') : platformLabel(item.platform) }}
 </span>
 <span class="font-mono text-sm text-success-text" :title="t('dashboard.actual')">
 ${{ formatCost(item.total_actual_cost) }}
 </span>
 </div>
 <div class="mt-2 space-y-1 text-xs">
 <div class="flex items-center justify-between">
 <span class="text-muted">{{ t('dashboard.todayCost') }}</span>
 <span class="font-mono text-foreground">${{ formatCost(item.today_actual_cost) }}</span>
 </div>
 <div class="flex items-center justify-between">
 <span class="text-muted">{{ t('dashboard.requests') }}</span>
 <span class="font-mono text-foreground">
 {{ item.total_requests > 0 ? formatNumber(item.total_requests) : '-' }}
 </span>
 </div>
 <div class="flex items-center justify-between">
 <span class="text-muted">{{ t('dashboard.tokens') }}</span>
 <span class="font-mono text-foreground">
 {{ item.total_tokens > 0 ? formatTokens(item.total_tokens) : '-' }}
 </span>
 </div>
 </div>

 <div v-if="hasAnyLimit(item.quota) && !item.isOther" class="mt-3 space-y-1.5 border-t border-line pt-2">
 <p class="text-[10px] uppercase tracking-wide text-muted">
 {{ t('dashboard.platformQuota.title') }}
 </p>
 <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
 <div v-if="quotaVal(item.quota, `${w}_limit_usd`) != null" class="space-y-0.5">
 <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
 <div class="flex items-center justify-between text-xs">
 <span class="text-foreground">{{ t(`dashboard.platformQuota.${w}`) }}</span>
 <span class="font-mono text-danger-text">{{ t('dashboard.platformQuota.disabled') }}</span>
 </div>
 <ProgressBar :value="100" />
 </template>
 <template v-else>
 <div class="flex items-center justify-between text-xs">
 <span class="text-foreground">{{ t(`dashboard.platformQuota.${w}`) }}</span>
 <span class="font-mono text-foreground">
 ${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}
 </span>
 </div>
 <ProgressBar :value="calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number)" />
 <p v-if="quotaVal(item.quota, `${w}_window_resets_at`)" class="text-[10px] text-muted">
 {{ t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) }}
 </p>
 </template>
 </div>
 </template>
 </div>
 </div>
 </div>
 </GlassCard>
 </div>
</template>
<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import StatCard from '@/components/ui/StatCard.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import ProgressBar from '@/components/ui/ProgressBar.vue'
import type { UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

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
 balance: number
 isSimple: boolean
 platformQuotas?: PlatformQuotaItem[] | null
 liveRpmUsed?: number
 liveRpmLimit?: number
 currentConcurrency?: number
}>()
const emit = defineEmits<{
 'balance-history': []
}>()
const { t } = useI18n()

const liveRpmUsed = computed(() => props.liveRpmUsed ?? 0)
const currentConcurrency = computed(() => props.currentConcurrency ?? 0)
const liveRpmLabel = computed(() => {
 const limit = props.liveRpmLimit ?? 0
 return limit > 0 ? `RPM ${liveRpmUsed.value}/${limit}` : t('dashboard.liveRpm')
})

const PLATFORM_LABELS: Record<string, string> = {
 anthropic: 'Claude',
 openai: 'OpenAI',
 gemini: 'Gemini',
 antigravity: 'Antigravity'
}

const platformLabel = (p: string) => PLATFORM_LABELS[p] ?? p

const sortedPlatforms = computed(() => {
 const list = props.stats?.by_platform ?? []
 return [...list].sort((a, b) => b.total_actual_cost - a.total_actual_cost)
})

// 处理"各平台之和 < 总值"的差值：后端按平台聚合时过滤了无法归属平台的行
// （group 与 account 都缺 platform）。这里把差值作为"其他"卡片显式展示，
// 避免 Row 1 总值与 Row 3 平台拆分加总对不上、用户困惑。
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

const formatBalance = (b: number) =>
 new Intl.NumberFormat('en-US', {
 minimumFractionDigits: 2,
 maximumFractionDigits: 2
 }).format(b)

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatTokens = (t: number) => {
 if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
 if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
 return t.toString()
}
const formatDuration = (ms: number) => ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`
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
.dash-perf {
 display: flex;
 flex-direction: column;
 gap: 2px;
 font-size: 10px;
 color: var(--muted);
 width: 100%;
}
@media (max-width: 1100px) {
 .dash-stat-grid {
 grid-template-columns: 1fr 1fr;
 }
}
@media (max-width: 767px) {
 .dash-stat-grid {
 grid-template-columns: 1fr 1fr;
 }
}
</style>
