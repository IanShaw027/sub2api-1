<template>
 <span
 :class="[
 'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium transition-colors',
 badgeClass
 ]"
 >
 <!-- Platform logo -->
 <PlatformIcon v-if="platform" :platform="platform" size="sm" />
 <!-- Group name -->
 <span class="truncate">{{ name }}</span>
 <!-- Right side label -->
 <span v-if="showLabel" :class="labelClass">
 <template v-if="hasCustomRate">
 <!-- 原倍率删除线 + 专属倍率高亮 -->
 <span class="line-through opacity-50 mr-0.5">{{ rateMultiplier }}x</span>
 <span class="font-bold">{{ userRateMultiplier }}x</span>
 </template>
 <template v-else>
 {{ labelText }}
 </template>
 </span>
 <span v-if="hasPeakRate" :class="peakRateClass" :title="peakRateTitle">
 {{ peakRateText }}
 </span>
 </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionType, GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import PlatformIcon from './PlatformIcon.vue'

interface Props {
 name: string
 platform?: GroupPlatform
 subscriptionType?: SubscriptionType
 rateMultiplier?: number
 userRateMultiplier?: number | null // 用户专属倍率
 peakRateEnabled?: boolean
 peakStart?: string
 peakEnd?: string
 peakRateMultiplier?: number
 showRate?: boolean
 daysRemaining?: number | null // 剩余天数（订阅类型时使用）
 /**
 * 订阅分组默认在右侧 label 展示"订阅"或剩余天数；
 * 开启后订阅分组也改为显示倍率（保留订阅主题色 label，配合可用渠道这类
 * 只关心费率、不关心有效期的场景）。
 */
 alwaysShowRate?: boolean
}

const props = withDefaults(defineProps<Props>(), {
 subscriptionType: 'standard',
 showRate: true,
 daysRemaining: null,
 userRateMultiplier: null,
 peakRateEnabled: false,
 alwaysShowRate: false
})

const { t } = useI18n()

const isSubscription = computed(() => props.subscriptionType === 'subscription')

// 是否有专属倍率（且与默认倍率不同）
const hasCustomRate = computed(() => {
 return (
 props.userRateMultiplier !== null &&
 props.userRateMultiplier !== undefined &&
 props.rateMultiplier !== undefined &&
 props.userRateMultiplier !== props.rateMultiplier
 )
})

const appStore = useAppStore()

const hasPeakRate = computed(() => {
 return Boolean(props.showRate && props.peakRateEnabled && props.peakStart && props.peakEnd)
})

const peakRateText = computed(() => {
 return formatPeakRateWindow(
 {
 peak_rate_enabled: props.peakRateEnabled,
 peak_start: props.peakStart,
 peak_end: props.peakEnd,
 peak_rate_multiplier: props.peakRateMultiplier
 },
 serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset)
 )
})

const peakRateTitle = computed(() => {
 return t('common.peakRateTooltip', { window: peakRateText.value })
})

// 是否显示右侧标签
const showLabel = computed(() => {
 if (!props.showRate) return false
 // 订阅类型：显示天数或"订阅"
 if (isSubscription.value) return true
 // 标准类型：显示倍率（包括专属倍率）
 return props.rateMultiplier !== undefined || hasCustomRate.value
})

// Label text
const labelText = computed(() => {
 const rateLabel = props.rateMultiplier !== undefined ? `${props.rateMultiplier}x` : ''
 if (isSubscription.value && !props.alwaysShowRate) {
 // 如果有剩余天数，显示天数
 if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
 if (props.daysRemaining <= 0) {
 return t('admin.users.expired')
 }
 return t('admin.users.daysRemaining', { days: props.daysRemaining })
 }
 // 否则显示"订阅"
 return t('groups.subscription')
 }
 return rateLabel
})

// Label style based on type and days remaining
const labelClass = computed(() => {
 const base = 'px-1.5 py-0.5 rounded text-[10px] font-semibold'

 if (!isSubscription.value) {
 // Standard: subtle background (不再为专属倍率使用不同的背景色)
 return `${base} bg-black/10`
 }

 // 订阅类型：根据剩余天数显示不同颜色
 if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
 if (props.daysRemaining <= 0 || props.daysRemaining <= 3) {
 // 已过期或紧急（<=3天）：红色
 return `${base} bg-red-200/80 text-red-800`
 }
 if (props.daysRemaining <= 7) {
 // 警告（<=7天）：橙色
 return `${base} bg-amber-200/80 text-amber-800`
 }
 }

 // 正常状态或无天数：根据平台显示主题色
 if (props.platform === 'anthropic') {
 return `${base} bg-orange-200/60 text-orange-800`
 }
 if (props.platform === 'openai') {
 return `${base} bg-emerald-200/60 text-emerald-800`
 }
 if (props.platform === 'gemini') {
 return `${base} bg-blue-200/60 text-blue-800`
 }
 if (props.platform === 'antigravity') {
 return `${base} bg-purple-200/60 text-purple-800`
 }
 if (props.platform === 'grok') {
 return `${base} bg-zinc-300/70 text-zinc-800`
 }
 if (props.platform === 'kimi') {
 return `${base} bg-pink-200/60 text-pink-800`
 }
 if (props.platform === 'zhipu') {
 return `${base} bg-indigo-200/60 text-indigo-800`
 }
 if (props.platform === 'deepseek') {
 return `${base} bg-teal-200/60 text-teal-800`
 }
 if (props.platform === 'composite') {
 return `${base} bg-cyan-200/70 text-cyan-900`
 }
 return `${base} bg-violet-200/60 text-violet-800`
})

const peakRateClass = computed(() => {
 return 'px-1.5 py-0.5 rounded text-[10px] font-semibold bg-amber-100 text-amber-700'
})

// Badge color based on platform and subscription type
const badgeClass = computed(() => {
 if (props.platform === 'anthropic') {
 // Claude: orange theme
 return isSubscription.value
 ? 'bg-orange-100 text-orange-700'
 : 'bg-amber-50 text-amber-700'
 } else if (props.platform === 'openai') {
 // OpenAI: green theme
 return isSubscription.value
 ? 'bg-emerald-100 text-emerald-700'
 : 'bg-green-50 text-green-700'
 }
 if (props.platform === 'gemini') {
 return isSubscription.value
 ? 'bg-blue-100 text-blue-700'
 : 'bg-sky-50 text-sky-700'
 }
 if (props.platform === 'antigravity') {
 return isSubscription.value
 ? 'bg-purple-100 text-purple-700'
 : 'bg-fuchsia-50 text-fuchsia-700'
 }
 if (props.platform === 'grok') {
 return isSubscription.value
 ? 'bg-zinc-200 text-zinc-800'
 : 'bg-zinc-100 text-zinc-700'
 }
 if (props.platform === 'kimi') {
 return isSubscription.value
 ? 'bg-pink-100 text-pink-700'
 : 'bg-pink-50 text-pink-700'
 }
 if (props.platform === 'zhipu') {
 return isSubscription.value
 ? 'bg-indigo-100 text-indigo-700'
 : 'bg-indigo-50 text-indigo-700'
 }
 if (props.platform === 'deepseek') {
 return isSubscription.value
 ? 'bg-teal-100 text-teal-700'
 : 'bg-teal-50 text-teal-700'
 }
 if (props.platform === 'composite') {
 return isSubscription.value
 ? 'bg-cyan-100 text-cyan-800'
 : 'bg-cyan-50 text-cyan-800'
 }
 // Fallback: original colors
 return isSubscription.value
 ? 'bg-violet-100 text-violet-700'
 : 'bg-emerald-100 text-emerald-700'
})
</script>
