<template>
 <div class="flex min-w-0 flex-1 items-start justify-between gap-3">
 <!-- Left: name + description -->
 <div
 class="flex min-w-0 flex-1 flex-col items-start"
 :title="description || undefined"
 >
 <!-- Row 1: platform badge (name bold) -->
 <GroupBadge
 :name="name"
 :platform="platform"
 :subscription-type="subscriptionType"
 :show-rate="false"
 class="groupOptionItemBadge"
 />
 <!-- Row 2: description with top spacing -->
 <span
 v-if="description"
 class="mt-1.5 w-full whitespace-pre-line [overflow-wrap:anywhere] text-left text-[11.5px] leading-relaxed text-muted line-clamp-3"
 >
 {{ description }}
 </span>
 </div>

 <!-- Right: rate pill + checkmark (vertically centered to first row) -->
 <div class="flex shrink-0 items-center gap-2 pt-0.5">
 <div class="flex shrink-0 flex-col items-end gap-1">
 <!-- Rate pill (platform color) -->
 <span v-if="rateMultiplier !== undefined" :class="['badge', ratePillClass]">
 <template v-if="hasCustomRate">
 <span class="mr-1 line-through opacity-50">{{ rateMultiplier }}x</span>
 <span class="font-bold">{{ userRateMultiplier }}x</span>
 </template>
 <template v-else>
 {{ rateMultiplier }}x {{ t('admin.groups.rateLabel') }}
 </template>
 </span>
 <span
 v-if="hasPeakRate"
 class="badge badge-tone-warning"
 :title="peakRateTitle"
 >
 {{ peakRateText }}
 </span>
 </div>
 <!-- Checkmark -->
 <svg
 v-if="showCheckmark && selected"
 class="h-3.5 w-3.5 shrink-0 text-accent"
 fill="none"
 stroke="currentColor"
 viewBox="0 0 24 24"
 stroke-width="2"
 >
 <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
 </svg>
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import GroupBadge from './GroupBadge.vue'
import type { SubscriptionType, GroupPlatform } from '@/types'
import { useAppStore } from '@/stores/app'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'

const { t } = useI18n()

interface Props {
 name: string
 platform: GroupPlatform
 subscriptionType?: SubscriptionType
 rateMultiplier?: number
 userRateMultiplier?: number | null
 peakRateEnabled?: boolean
 peakStart?: string
 peakEnd?: string
 peakRateMultiplier?: number
 description?: string | null
 selected?: boolean
 showCheckmark?: boolean
}

const props = withDefaults(defineProps<Props>(), {
 subscriptionType: 'standard',
 selected: false,
 showCheckmark: true,
 userRateMultiplier: null,
 peakRateEnabled: false
})

// Whether user has a custom rate different from default
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
 return Boolean(props.peakRateEnabled && props.peakStart && props.peakEnd)
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

// Rate pill color matches platform badge color
const ratePillClass = computed(() => {
 switch (props.platform) {
 case 'anthropic':
 return 'badge-tone-warning'
 case 'openai':
 return 'badge-tone-success'
 case 'gemini':
 return 'badge-tone-accent'
 default: // antigravity and others
 return 'badge-tone-muted'
 }
})
</script>

<style scoped>
/* Bold the group name inside GroupBadge when used in dropdown option */
.groupOptionItemBadge :deep(span.truncate) {
 font-weight: 600;
}
</style>
