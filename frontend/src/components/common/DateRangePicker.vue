<template>
 <div class="relative" ref="containerRef">
 <button
 type="button"
 ref="triggerRef"
 :aria-expanded="isOpen"
 aria-haspopup="dialog"
 :aria-controls="isOpen ? popupId : undefined"
 :aria-label="t('dates.selectDateRange')"
 :title="t('dates.selectDateRange')"
 @click="toggle"
 :class="['date-picker-trigger', isOpen && 'date-picker-trigger-open']"
 >
 <span class="date-picker-icon">
 <Icon name="calendar" size="sm" />
 </span>
 <span class="date-picker-value">
 {{ displayValue }}
 </span>
 <span class="date-picker-chevron">
 <Icon
 name="chevronDown"
 size="sm"
 :class="['transition-transform duration-200', isOpen && 'rotate-180']"
 />
 </span>
 </button>

 <Teleport to="body">
 <Transition name="date-picker-dropdown">
 <div v-if="isOpen" :id="popupId" ref="dropdownRef" class="date-picker-dropdown" :style="dropdownStyle" role="dialog" :aria-label="t('dates.selectDateRange')">
 <!-- Quick presets -->
 <div class="date-picker-presets">
 <button
 v-for="preset in presets"
 type="button"
 :key="preset.value"
 @click="selectPreset(preset)"
 :class="['date-picker-preset', isPresetActive(preset) && 'date-picker-preset-active']"
 >
 {{ t(preset.labelKey) }}
 </button>
 </div>

 <div class="date-picker-divider"></div>

 <!-- Custom date range inputs -->
 <div class="date-picker-custom">
 <div class="date-picker-field">
 <label :for="`${popupId}-start`" class="date-picker-label">{{ t('dates.startDate') }}</label>
 <input
 type="date"
 :id="`${popupId}-start`"
 v-model="localStartDate"
 :max="localEndDate || tomorrow"
 class="date-picker-input"
 @change="onDateChange"
 />
 </div>
 <div class="date-picker-separator">
 <Icon name="arrowRight" size="sm" class="text-muted" />
 </div>
 <div class="date-picker-field">
 <label :for="`${popupId}-end`" class="date-picker-label">{{ t('dates.endDate') }}</label>
 <input
 type="date"
 :id="`${popupId}-end`"
 v-model="localEndDate"
 :min="localStartDate"
 :max="tomorrow"
 class="date-picker-input"
 @change="onDateChange"
 />
 </div>
 </div>

 <!-- Apply button -->
 <div class="date-picker-actions">
 <button type="button" @click="apply" class="date-picker-apply" :disabled="!validRange">
 {{ t('dates.apply') }}
 </button>
 </div>
 </div>
 </Transition>
 </Teleport>
 </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick, useId, type CSSProperties } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

interface DatePreset {
 labelKey: string
 value: string
 getRange: () => { start: string; end: string }
}

interface Props {
 startDate: string
 endDate: string
}

interface Emits {
 (e: 'update:startDate', value: string): void
 (e: 'update:endDate', value: string): void
 (e: 'change', range: { startDate: string; endDate: string; preset: string | null }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t, locale } = useI18n()

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const popupId = `date-range-${useId()}`
const dropdownStyle = ref<CSSProperties>({})
const validRange = computed(() => Boolean(localStartDate.value && localEndDate.value && localStartDate.value <= localEndDate.value && localEndDate.value <= tomorrow.value))

function updatePosition() {
 if (!isOpen.value || !triggerRef.value) return
 const rect = triggerRef.value.getBoundingClientRect()
 const padding = 8
 const width = Math.min(360, window.innerWidth - padding * 2)
 const below = window.innerHeight - rect.bottom - padding
 const above = rect.top - padding
 const height = dropdownRef.value?.scrollHeight || 320
 const openAbove = below < height && above > below
 const maxHeight = Math.max(100, (openAbove ? above : below) - padding)
 dropdownStyle.value = {
   width: `${width}px`,
   left: `${Math.max(padding, Math.min(rect.left, window.innerWidth - width - padding))}px`,
   top: openAbove ? undefined : `${Math.max(padding, rect.bottom + padding)}px`,
   bottom: openAbove ? `${Math.max(padding, window.innerHeight - rect.top + padding)}px` : undefined,
   maxHeight: `${Math.min(maxHeight, window.innerHeight - padding * 2)}px`
 }
}
const localStartDate = ref(props.startDate)
const localEndDate = ref(props.endDate)
const activePreset = ref<string | null>('last24Hours')

const today = computed(() => {
 // Use local timezone to avoid UTC timezone issues
 const now = new Date()
 const year = now.getFullYear()
 const month = String(now.getMonth() + 1).padStart(2, '0')
 const day = String(now.getDate()).padStart(2, '0')
 return `${year}-${month}-${day}`
})

// Tomorrow's date - used for max date to handle timezone differences
// When user is in a timezone behind the server, "today" on server might be "tomorrow" locally
const tomorrow = computed(() => {
 const d = new Date()
 d.setDate(d.getDate() + 1)
 return formatDateToString(d)
})

// Helper function to format date to YYYY-MM-DD using local timezone
const formatDateToString = (date: Date): string => {
 const year = date.getFullYear()
 const month = String(date.getMonth() + 1).padStart(2, '0')
 const day = String(date.getDate()).padStart(2, '0')
 return `${year}-${month}-${day}`
}

const presets: DatePreset[] = [
 {
 labelKey: 'dates.today',
 value: 'today',
 getRange: () => {
 const t = today.value
 return { start: t, end: t }
 }
 },
 {
 labelKey: 'dates.yesterday',
 value: 'yesterday',
 getRange: () => {
 const d = new Date()
 d.setDate(d.getDate() - 1)
 const yesterday = formatDateToString(d)
 return { start: yesterday, end: yesterday }
 }
 },
 {
 labelKey: 'dates.last24Hours',
 value: 'last24Hours',
 getRange: () => {
 const end = new Date()
 const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
 return {
 start: formatDateToString(start),
 end: formatDateToString(end)
 }
 }
 },
 {
 labelKey: 'dates.last7Days',
 value: '7days',
 getRange: () => {
 const end = today.value
 const d = new Date()
 d.setDate(d.getDate() - 6)
 const start = formatDateToString(d)
 return { start, end }
 }
 },
 {
 labelKey: 'dates.last14Days',
 value: '14days',
 getRange: () => {
 const end = today.value
 const d = new Date()
 d.setDate(d.getDate() - 13)
 const start = formatDateToString(d)
 return { start, end }
 }
 },
 {
 labelKey: 'dates.last30Days',
 value: '30days',
 getRange: () => {
 const end = today.value
 const d = new Date()
 d.setDate(d.getDate() - 29)
 const start = formatDateToString(d)
 return { start, end }
 }
 },
 {
 labelKey: 'dates.thisMonth',
 value: 'thisMonth',
 getRange: () => {
 const now = new Date()
 const start = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 1))
 return { start, end: today.value }
 }
 },
 {
 labelKey: 'dates.lastMonth',
 value: 'lastMonth',
 getRange: () => {
 const now = new Date()
 const start = formatDateToString(new Date(now.getFullYear(), now.getMonth() - 1, 1))
 const end = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 0))
 return { start, end }
 }
 }
]

const displayValue = computed(() => {
 if (activePreset.value) {
 const preset = presets.find((p) => p.value === activePreset.value)
 if (preset) return t(preset.labelKey)
 }

 if (localStartDate.value && localEndDate.value) {
 if (localStartDate.value === localEndDate.value) {
 return formatDate(localStartDate.value)
 }
 return `${formatDate(localStartDate.value)} - ${formatDate(localEndDate.value)}`
 }

 return t('dates.selectDateRange')
})

const formatDate = (dateStr: string): string => {
 const date = new Date(dateStr + 'T00:00:00')
 const dateLocale = locale.value === 'zh' ? 'zh-CN' : 'en-US'
 return date.toLocaleDateString(dateLocale, { month: 'short', day: 'numeric' })
}

const isPresetActive = (preset: DatePreset): boolean => {
 return activePreset.value === preset.value
}

const selectPreset = (preset: DatePreset) => {
 const range = preset.getRange()
 localStartDate.value = range.start
 localEndDate.value = range.end
 activePreset.value = preset.value
}

const onDateChange = () => {
 // Check if current dates match any preset
 activePreset.value = null
 for (const preset of presets) {
 const range = preset.getRange()
 if (range.start === localStartDate.value && range.end === localEndDate.value) {
 activePreset.value = preset.value
 break
 }
 }
}

const toggle = () => {
 isOpen.value = !isOpen.value
}

const apply = () => {
 if (!validRange.value) return
 emit('update:startDate', localStartDate.value)
 emit('update:endDate', localEndDate.value)
 emit('change', {
 startDate: localStartDate.value,
 endDate: localEndDate.value,
 preset: activePreset.value
 })
 isOpen.value = false
 triggerRef.value?.focus()
}

const handleClickOutside = (event: MouseEvent) => {
 if (!isOpen.value) return
 if (containerRef.value && !containerRef.value.contains(event.target as Node) && !dropdownRef.value?.contains(event.target as Node)) {
 isOpen.value = false
 }
}

const handleEscape = (event: KeyboardEvent) => {
 if (event.key === 'Escape' && isOpen.value) {
 isOpen.value = false
 triggerRef.value?.focus()
 }
}

// Sync local state with props
watch(
 () => props.startDate,
 (val) => {
 localStartDate.value = val
 onDateChange()
 }
)

watch(
 () => props.endDate,
 (val) => {
 localEndDate.value = val
 onDateChange()
 }
)

onMounted(() => {
 window.addEventListener('resize', updatePosition)
 window.addEventListener('scroll', updatePosition, true)
 document.addEventListener('click', handleClickOutside)
 document.addEventListener('keydown', handleEscape)
 // Initialize active preset detection
 onDateChange()
})

onUnmounted(() => {
 window.removeEventListener('resize', updatePosition)
 window.removeEventListener('scroll', updatePosition, true)
 document.removeEventListener('click', handleClickOutside)
 document.removeEventListener('keydown', handleEscape)
})

watch(isOpen, async open => {
 if (!open) return
 updatePosition()
 await nextTick()
 updatePosition()
 dropdownRef.value?.querySelector<HTMLButtonElement>('button')?.focus()
})
</script>

<style scoped>
.date-picker-trigger {
 display: flex;
 align-items: center;
 gap: 8px;
 height: 36px;
 padding: 0 12px;
 border-radius: var(--radius-field);
 background: color-mix(in oklch, var(--surface) 85%, transparent);
 border: 1px solid var(--border);
 box-shadow: var(--field-shadow);
 color: var(--foreground);
 font-size: 13px;
 transition: border-color 0.15s ease, box-shadow 0.15s ease;
 cursor: pointer;
}

.date-picker-trigger:focus,
.date-picker-trigger:focus-visible,
.date-picker-trigger-open {
 outline: none;
 border-color: var(--accent);
 box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent);
}

.date-picker-icon {
 @apply text-muted;
}

.date-picker-value {
 @apply font-medium;
}

.date-picker-chevron {
 @apply text-muted;
}

.date-picker-dropdown {
 position: fixed;
 z-index: 100000020;
 padding: 6px;
 border-radius: 12px;
 background: color-mix(in oklch, var(--surface) 92%, transparent);
 border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
 box-shadow: var(--shadow-pop);
 backdrop-filter: blur(20px);
 -webkit-backdrop-filter: blur(20px);
 overflow-y: auto;
 overscroll-behavior: contain;
}

.date-picker-presets {
 @apply grid grid-cols-2 gap-1 p-2;
}

.date-picker-preset {
 @apply rounded-md px-3 py-1.5 text-xs font-medium;
 @apply text-muted;
 @apply hover:bg-surface-2;
 @apply transition-colors duration-150;
}

.date-picker-preset-active {
 @apply bg-[color-mix(in_oklch,var(--accent)_16%,transparent)];
 color: var(--info-text);
}

.date-picker-divider {
 @apply border-t border-line;
}

.date-picker-custom {
 @apply flex items-end gap-2 p-3;
}

.date-picker-field {
 @apply min-w-0 flex-1;
}

.date-picker-label {
 @apply mb-1 block text-xs font-medium text-muted;
}

.date-picker-input {
 @apply w-full rounded-md px-2 py-1.5 text-sm;
 @apply bg-surface-2;
 @apply border border-line;
 @apply text-foreground;
 @apply focus:border-accent focus:outline-none focus:ring-2 focus:ring-[color-mix(in_oklch,var(--accent)_30%,transparent)];
}

.date-picker-input::-webkit-calendar-picker-indicator {
 @apply cursor-pointer opacity-60 hover:opacity-100;
 filter: invert(0.5);
}

:global([data-theme='glass-dark']) .date-picker-input::-webkit-calendar-picker-indicator {
 filter: none;
}

.date-picker-separator {
 @apply flex items-center justify-center pb-1;
}

.date-picker-actions {
 @apply flex justify-end p-2 pt-0;
}

.date-picker-apply {
 @apply rounded-lg px-4 py-1.5 text-sm font-medium;
 @apply bg-accent text-white;
 @apply hover:opacity-90;
 @apply transition-colors duration-150;
}

/* Dropdown animation */
.date-picker-dropdown-enter-active,
.date-picker-dropdown-leave-active {
 transition: all 0.2s ease;
}

.date-picker-dropdown-enter-from,
.date-picker-dropdown-leave-to {
 opacity: 0;
 transform: translateY(-8px);
}
</style>
