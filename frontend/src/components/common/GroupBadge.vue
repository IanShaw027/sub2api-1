<template>
  <span class="group-badge" :style="badgeStyle">
    <!-- Hue dot · 6px -->
    <span class="group-badge-dot" :style="{ background: hueColor }" aria-hidden="true"></span>
    <!-- Group name -->
    <span class="truncate">{{ name }}</span>
    <!-- Right side label -->
    <span v-if="showLabel" :class="labelClass">
      <template v-if="hasCustomRate">
        <!-- 原倍率删除线 + 专属倍率高亮 -->
        <span class="mr-0.5 line-through opacity-50">{{ rateMultiplier }}x</span>
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
// Label style based on type and days remaining
const labelClass = computed(() => {
  const base = 'group-badge-chip'

  if (!isSubscription.value) {
    return base
  }

  // 订阅类型：根据剩余天数提示紧急程度
  if (props.daysRemaining !== null && props.daysRemaining !== undefined) {
    if (props.daysRemaining <= 0 || props.daysRemaining <= 3) {
      return `${base} group-badge-chip-danger`
    }
    if (props.daysRemaining <= 7) {
      return `${base} group-badge-chip-warning`
    }
  }

  return base
})

const peakRateClass = computed(() => 'group-badge-chip group-badge-chip-warning')

/**
 * 分组配色：保留「每个平台一个色相」的逻辑，但统一按原型 07 的公式派生
 * 底色 color-mix(hue 16%) / 文字 color-mix(hue 70%, foreground)。
 */
const PLATFORM_HUE: Record<string, number> = {
  anthropic: 45,
  openai: 160,
  gemini: 262,
  antigravity: 300,
  grok: 285,
  kimi: 350,
  zhipu: 275,
  deepseek: 195,
  composite: 190
}

const hue = computed(() => PLATFORM_HUE[props.platform ?? ''] ?? 300)
const chroma = computed(() => (props.platform === 'grok' ? 0.03 : 0.15))
const hueColor = computed(() => `oklch(68% ${chroma.value} ${hue.value})`)

const badgeStyle = computed(() => ({
  background: `color-mix(in oklch, oklch(68% ${chroma.value} ${hue.value}) ${
    isSubscription.value ? 22 : 16
  }%, transparent)`,
  color: `color-mix(in oklch, oklch(62% ${chroma.value} ${hue.value}) 70%, var(--foreground))`
}))
</script>

<style scoped>
/* 22px 药丸 · currentColor 22% 内描边（原型 07 StatusBadge / 分组标签） */
.group-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 22px;
  max-width: 100%;
  padding: 0 8px;
  border-radius: 999px;
  font-size: 11.5px;
  font-weight: 600;
  line-height: 1;
  white-space: nowrap;
  box-shadow: inset 0 0 0 1px color-mix(in oklch, currentColor 22%, transparent);
}

.group-badge-dot {
  width: 6px;
  height: 6px;
  flex: none;
  border-radius: 999px;
}

.group-badge-chip {
  flex: none;
  padding: 0 4px;
  border-radius: 5px;
  font-size: 10.5px;
  font-weight: 700;
  background: color-mix(in oklch, currentColor 14%, transparent);
}

.group-badge-chip-warning {
  background: color-mix(in oklch, var(--warning) 20%, transparent);
  color: var(--warning-text);
}

.group-badge-chip-danger {
  background: color-mix(in oklch, var(--danger) 16%, transparent);
  color: var(--danger-text);
}
</style>
