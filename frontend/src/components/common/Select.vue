<template>
  <div class="select-root" :class="variant === 'pill' && 'select-root-pill'" ref="containerRef">
    <button
      ref="triggerRef"
      type="button"
      @click="toggle"
      :disabled="disabled"
      :aria-expanded="isOpen"
      :aria-haspopup="true"
      :id="id"
      :aria-label="ariaLabel ?? 'Select option'"
      :aria-describedby="ariaDescribedby"
      :class="[
        'select-trigger',
        variant === 'pill' && 'filter-pill select-trigger-pill',
        isOpen && 'select-trigger-open',
        error && 'select-trigger-error',
        disabled && 'select-trigger-disabled'
      ]"
      @keydown.down.prevent="onTriggerKeyDown"
      @keydown.up.prevent="onTriggerKeyDown"
    >
      <span v-if="variant === 'pill' && pillLabel" class="filter-pill-label">{{ pillLabel }}</span>
      <span class="select-value" :class="variant === 'pill' && 'filter-pill-value'">
        <slot name="selected" :option="selectedOption">
          {{ selectedLabel }}
        </slot>
      </span>
      <span
        v-if="clearable && hasValue && !disabled"
        class="select-clear"
        role="button"
        tabindex="-1"
        aria-label="Clear selection"
        @click.stop="clearSelection"
        @mousedown.stop
        @keydown.enter.stop.prevent="clearSelection"
      >
        <Icon name="x" size="sm" />
      </span>
      <span class="select-icon">
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="2"
          stroke-linecap="round"
          stroke-linejoin="round"
          :class="['transition-transform duration-200', isOpen && 'rotate-180']"
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </span>
    </button>

    <!-- Teleport dropdown to body to escape stacking context -->
    <Teleport to="body">
      <Transition name="select-dropdown">
        <div
          v-if="isOpen"
          ref="dropdownRef"
          class="dropdown select-dropdown-portal"
          :class="[instanceId]"
          :style="dropdownStyle"
          role="listbox"
          @click.stop
          @mousedown.stop
          @keydown="onDropdownKeyDown"
        >
          <!-- Search row · 32px field -->
          <div v-if="isSearchable" class="select-search">
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              stroke-width="1.8"
              stroke-linecap="round"
              stroke-linejoin="round"
              class="select-search-icon"
              aria-hidden="true"
            >
              <path d="M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.35-4.35" />
            </svg>
            <input
              ref="searchInputRef"
              v-model="searchQuery"
              type="text"
              :placeholder="searchPlaceholderText"
              :aria-label="searchPlaceholderText"
              class="select-search-input"
              @click.stop
            />
          </div>

          <!-- Options list -->
          <div class="select-options" ref="optionsListRef">
            <div
              v-for="(option, index) in filteredOptions"
              :key="`${typeof getOptionValue(option)}:${String(getOptionValue(option) ?? '')}`"
              role="option"
              :aria-selected="isSelected(option)"
              :aria-disabled="isOptionDisabled(option)"
              @click.stop="!isOptionDisabled(option) && selectOption(option)"
              @mouseenter="handleOptionMouseEnter(option, index)"
              :class="[
                'select-option',
                isGroupHeaderOption(option) && 'select-option-group',
                isSelected(option) && 'select-option-selected',
                isOptionDisabled(option) && !isGroupHeaderOption(option) && 'select-option-disabled',
                focusedIndex === index && !isGroupHeaderOption(option) && 'select-option-focused'
              ]"
            >
              <slot name="option" :option="option" :selected="isSelected(option)">
                <Icon
                  v-if="option._creatable"
                  name="search"
                  size="sm"
                  class="flex-shrink-0 text-muted"
                />
                <span class="select-option-label" :class="option._creatable && 'italic text-muted'">{{ getOptionLabel(option) }}</span>
                <Icon
                  v-if="isSelected(option)"
                  name="check"
                  size="sm"
                  class="select-option-check"
                  :stroke-width="2.4"
                />
              </slot>
            </div>

            <!-- Empty state -->
            <div v-if="filteredOptions.length === 0" class="select-empty">
              {{ props.loading ? t('common.loading') : emptyTextDisplay }}
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

// Instance ID for unique click-outside detection
const instanceId = `select-${Math.random().toString(36).substring(2, 9)}`

export interface SelectOption {
 value: string | number | boolean | null
 label: string
 disabled?: boolean
 [key: string]: unknown
}

interface Props {
 modelValue: string | number | boolean | null | undefined
 options: SelectOption[] | Array<Record<string, unknown>>
 placeholder?: string
 disabled?: boolean
 error?: boolean
 searchable?: boolean | 'auto'
 searchPlaceholder?: string
 emptyText?: string
 valueKey?: string
 labelKey?: string
 creatable?: boolean
 creatablePrefix?: string
 clearable?: boolean
 id?: string
 ariaLabel?: string
 ariaDescribedby?: string
 /** 触发器外观：field（默认 36px 输入框）| pill（filter-pill 的 label · value） */
 variant?: 'field' | 'pill'
 /** variant="pill" 时显示在数值前的静态标签 */
 pillLabel?: string
 /** 远程搜索模式：输入不在本地过滤 options，而是防抖后 emit('search', query)，由父组件请求数据更新 options */
 remote?: boolean
 /** 远程搜索模式下的加载态：options 为空时下拉显示 loading 文案 */
 loading?: boolean
}

interface Emits {
 (e: 'update:modelValue', value: string | number | boolean | null): void
 (e: 'change', value: string | number | boolean | null, option: SelectOption | null): void
 (e: 'search', query: string): void
}

const props = withDefaults(defineProps<Props>(), {
 disabled: false,
 error: false,
 searchable: 'auto',
 creatable: false,
 creatablePrefix: '',
 clearable: false,
 valueKey: 'value',
 labelKey: 'label',
 remote: false,
 loading: false,
 variant: 'field'
})

const emit = defineEmits<Emits>()

const isOpen = ref(false)
const searchQuery = ref('')
const focusedIndex = ref(-1)
const containerRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const searchInputRef = ref<HTMLInputElement | null>(null)
const dropdownRef = ref<HTMLElement | null>(null)
const optionsListRef = ref<HTMLElement | null>(null)
const dropdownPosition = ref<'bottom' | 'top'>('bottom')
const triggerRect = ref<DOMRect | null>(null)
const dropdownViewportPadding = 8
const dropdownMinimumWidth = 200

// i18n placeholders
const placeholderText = computed(() => props.placeholder ?? t('common.selectOption'))
const searchPlaceholderText = computed(() => props.searchPlaceholder ?? t('common.searchPlaceholder'))
const emptyTextDisplay = computed(() => props.emptyText ?? t('common.noOptionsFound'))

// 远程搜索的防抖间隔（对齐 OpenAIFastPolicyUserSelector 的 300ms 惯例）。
const REMOTE_SEARCH_DEBOUNCE_MS = 300
let remoteSearchTimer: ReturnType<typeof setTimeout> | null = null

const isSearchable = computed(() => {
 // 远程搜索模式始终显示搜索框（选项只是服务端结果的一页）。
 if (props.remote) return true
 if (props.searchable === 'auto') return props.options.length > 5
 return props.searchable
})

// Computed style for teleported dropdown
const dropdownStyle = computed(() => {
 if (!triggerRect.value) return {}

 const rect = triggerRect.value
 const viewportRight = Math.max(dropdownViewportPadding, window.innerWidth - dropdownViewportPadding)
 const left = Math.min(
 Math.max(dropdownViewportPadding, rect.left),
 viewportRight
 )
 const availableWidth = Math.max(0, viewportRight - left)
 const preferredMinWidth = Math.max(dropdownMinimumWidth, rect.width)
 const minWidth = Math.min(preferredMinWidth, availableWidth)
 const style: Record<string, string> = {
 position: 'fixed',
 left: `${left}px`,
 minWidth: `${minWidth}px`,
 maxWidth: `${availableWidth}px`,
 zIndex: '100000020'
 }

 if (dropdownPosition.value === 'top') {
 style.bottom = `${window.innerHeight - rect.top + 4}px`
 } else {
 style.top = `${rect.bottom + 4}px`
 }

 return style
})

const getOptionValue = (option: any): any => {
 if (typeof option === 'object' && option !== null) {
 return option[props.valueKey]
 }
 return option
}

const getOptionLabel = (option: any): string => {
 if (typeof option === 'object' && option !== null) {
 return String(option[props.labelKey] ?? '')
 }
 return String(option ?? '')
}

const isOptionDisabled = (option: any): boolean => {
 if (typeof option === 'object' && option !== null) {
 return !!option.disabled
 }
 return false
}

const isGroupHeaderOption = (option: any): boolean => {
 if (typeof option === 'object' && option !== null) {
 return option.kind === 'group'
 }
 return false
}

const selectedOption = computed(() => {
 return props.options.find((opt) => getOptionValue(opt) === props.modelValue) || null
})

const selectedLabel = computed(() => {
 if (selectedOption.value) {
 return getOptionLabel(selectedOption.value)
 }
 // In creatable mode, show the raw value if no matching option
 if (props.creatable && props.modelValue) {
 return String(props.modelValue)
 }
 return placeholderText.value
})

const hasValue = computed(
 () => props.modelValue !== null && props.modelValue !== undefined && props.modelValue !== ''
)

const filteredOptions = computed(() => {
 let opts = props.options as any[]
 // 远程搜索模式不在本地过滤（选项即服务端搜索结果的一页）。
 if (isSearchable.value && searchQuery.value && !props.remote) {
 const query = searchQuery.value.toLowerCase()
 opts = opts.filter((opt) => {
 // Match label
 if (getOptionLabel(opt).toLowerCase().includes(query)) return true
 // Also match description if present
 if (opt.description && String(opt.description).toLowerCase().includes(query)) return true
 return false
 })
 // In creatable mode, always prepend a fuzzy search option
 if (props.creatable && searchQuery.value.trim()) {
 const trimmed = searchQuery.value.trim()
 const prefix = props.creatablePrefix || t('common.search')
 opts = [{ [props.valueKey]: trimmed, [props.labelKey]: `${prefix} "${trimmed}"`, _creatable: true }, ...opts]
 }
 }
 return opts
})

const isSelected = (option: any): boolean => {
 return getOptionValue(option) === props.modelValue
}

const findNextEnabledIndex = (startIndex: number): number => {
 const opts = filteredOptions.value
 if (opts.length === 0) return -1
 for (let offset = 0; offset < opts.length; offset++) {
 const idx = (startIndex + offset) % opts.length
 if (!isOptionDisabled(opts[idx])) return idx
 }
 return -1
}

const findPrevEnabledIndex = (startIndex: number): number => {
 const opts = filteredOptions.value
 if (opts.length === 0) return -1
 for (let offset = 0; offset < opts.length; offset++) {
 const idx = (startIndex - offset + opts.length) % opts.length
 if (!isOptionDisabled(opts[idx])) return idx
 }
 return -1
}

const handleOptionMouseEnter = (option: any, index: number) => {
 if (isOptionDisabled(option) || isGroupHeaderOption(option)) return
 focusedIndex.value = index
}

// Update trigger rect periodically while open to follow scroll/resize
const updateTriggerRect = () => {
 if (containerRef.value) {
 triggerRect.value = containerRef.value.getBoundingClientRect()
 }
}

const calculateDropdownPosition = () => {
 if (!containerRef.value) return
 updateTriggerRect()

 nextTick(() => {
 if (!dropdownRef.value || !triggerRect.value) return
 const dropdownHeight = dropdownRef.value.offsetHeight || 240
 const spaceBelow = window.innerHeight - triggerRect.value.bottom
 const spaceAbove = triggerRect.value.top

 if (spaceBelow < dropdownHeight && spaceAbove > dropdownHeight) {
 dropdownPosition.value = 'top'
 } else {
 dropdownPosition.value = 'bottom'
 }
 })
}

const toggle = () => {
 if (props.disabled) return
 isOpen.value = !isOpen.value
}

watch(isOpen, (open) => {
 if (open) {
 calculateDropdownPosition()
 // Reset focused index to current selection or first item
 if (filteredOptions.value.length === 0) {
 focusedIndex.value = -1
 } else {
 const selectedIdx = filteredOptions.value.findIndex(isSelected)
 const initialIdx = selectedIdx >= 0 ? selectedIdx : 0
 focusedIndex.value = isOptionDisabled(filteredOptions.value[initialIdx])
 ? findNextEnabledIndex(initialIdx + 1)
 : initialIdx
 }

 if (isSearchable.value) {
 nextTick(() => searchInputRef.value?.focus())
 }
 // Add scroll listener to update position
 window.addEventListener('scroll', updateTriggerRect, { capture: true, passive: true })
 window.addEventListener('resize', calculateDropdownPosition)
 } else {
 searchQuery.value = ''
 focusedIndex.value = -1
 // 关闭时取消仍在排队的远程搜索（避免关闭后尾随 emit 一次 search(''))。
 if (remoteSearchTimer) {
 clearTimeout(remoteSearchTimer)
 remoteSearchTimer = null
 }
 window.removeEventListener('scroll', updateTriggerRect, { capture: true })
 window.removeEventListener('resize', calculateDropdownPosition)
 }
})

// 远程搜索：输入防抖后交给父组件请求（!isOpen 抑制关闭重置 searchQuery 触发的空 query）。
watch(searchQuery, (query) => {
 if (!props.remote || !isOpen.value) return
 if (remoteSearchTimer) clearTimeout(remoteSearchTimer)
 remoteSearchTimer = setTimeout(() => {
 remoteSearchTimer = null
 emit('search', query.trim())
 }, REMOTE_SEARCH_DEBOUNCE_MS)
})

const selectOption = (option: any) => {
 const value = getOptionValue(option) ?? null
 emit('update:modelValue', value)
 emit('change', value, option)
 isOpen.value = false
 triggerRef.value?.focus()
}

const clearSelection = () => {
 if (props.disabled) return
 emit('update:modelValue', null)
 emit('change', null, null)
}

// Keyboards
const onTriggerKeyDown = () => {
 if (!isOpen.value) {
 isOpen.value = true
 }
}

const onDropdownKeyDown = (e: KeyboardEvent) => {
 switch (e.key) {
 case 'ArrowDown':
 e.preventDefault()
 focusedIndex.value = findNextEnabledIndex(focusedIndex.value + 1)
 if (focusedIndex.value >= 0) scrollToFocused()
 break
 case 'ArrowUp':
 e.preventDefault()
 focusedIndex.value = findPrevEnabledIndex(focusedIndex.value - 1)
 if (focusedIndex.value >= 0) scrollToFocused()
 break
 case 'Enter':
 e.preventDefault()
 if (focusedIndex.value >= 0 && focusedIndex.value < filteredOptions.value.length) {
 const opt = filteredOptions.value[focusedIndex.value]
 if (!isOptionDisabled(opt)) selectOption(opt)
 }
 break
 case 'Escape':
 e.preventDefault()
 e.stopImmediatePropagation()
 isOpen.value = false
 triggerRef.value?.focus()
 break
 case 'Tab':
 isOpen.value = false
 break
 }
}

const scrollToFocused = () => {
 nextTick(() => {
 const list = optionsListRef.value
 if (!list) return
 const focusedEl = list.children[focusedIndex.value] as HTMLElement
 if (!focusedEl) return

 if (focusedEl.offsetTop < list.scrollTop) {
 list.scrollTop = focusedEl.offsetTop
 } else if (focusedEl.offsetTop + focusedEl.offsetHeight > list.scrollTop + list.offsetHeight) {
 list.scrollTop = focusedEl.offsetTop + focusedEl.offsetHeight - list.offsetHeight
 }
 })
}

const handleClickOutside = (event: MouseEvent) => {
 const target = event.target as HTMLElement
 // Check if click is inside THIS specific instance's dropdown or trigger
 const isInDropdown = !!target.closest(`.${instanceId}`)
 const isInTrigger = containerRef.value?.contains(target)

 if (!isInDropdown && !isInTrigger && isOpen.value) {
 isOpen.value = false
 }
}

onMounted(() => {
 document.addEventListener('click', handleClickOutside)
})

onUnmounted(() => {
 document.removeEventListener('click', handleClickOutside)
 window.removeEventListener('scroll', updateTriggerRect, { capture: true })
 window.removeEventListener('resize', calculateDropdownPosition)
 if (remoteSearchTimer) {
 clearTimeout(remoteSearchTimer)
 remoteSearchTimer = null
 }
})
</script>

<style scoped>
.select-root {
  position: relative;
}

.select-root-pill {
  display: inline-block;
  width: max-content;
}

/* Trigger · `.field` recipe (36px, radius 12, glass 85%, field-shadow). */
.select-trigger {
  display: flex;
  width: 100%;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 36px;
  padding: 0 10px 0 12px;
  border-radius: var(--radius-field);
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  box-shadow: var(--field-shadow);
  color: var(--foreground);
  font-size: 13px;
  cursor: pointer;
  text-align: left;
  transition: border-color 0.15s ease, box-shadow 0.15s ease, background 0.15s ease;
}

.select-trigger:focus-visible,
.select-trigger-open {
  outline: none;
  border-color: var(--accent);
  box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent);
}

/* Pill variant · `label · value` (geometry from the global `.filter-pill`). */
.select-trigger.select-trigger-pill {
  width: max-content;
  justify-content: flex-start;
}

.select-trigger-error {
  border-color: var(--danger);
}

.select-trigger-error.select-trigger-open,
.select-trigger-error:focus-visible {
  border-color: var(--danger);
  box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--danger) 18%, transparent);
}

.select-trigger-disabled {
  cursor: not-allowed;
  background: var(--surface-secondary);
  opacity: 0.7;
}

.select-value {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.select-trigger-pill .select-value {
  flex: none;
}

.select-icon {
  flex: none;
  display: inline-flex;
  align-items: center;
  color: var(--muted);
}

.select-clear {
  display: inline-flex;
  flex: none;
  cursor: pointer;
  align-items: center;
  justify-content: center;
  color: var(--muted);
  transition: color 0.15s ease;
}

.select-clear:hover {
  color: var(--foreground);
}
</style>

<style>
/* Panel geometry (radius 12, padding 6, surface 92% + blur 20, shadow-pop)
   comes from the global `.dropdown` recipe; only the list internals live here. */
.dropdown.select-dropdown-portal {
  width: max-content;
  min-width: 200px;
  overflow: hidden;
  pointer-events: auto !important;
  animation: none;
}

.select-dropdown-portal .select-search {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  margin-bottom: 4px;
  padding: 0 10px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  box-shadow: var(--field-shadow);
}

.select-dropdown-portal .select-search-icon {
  flex: none;
  color: var(--muted);
}

.select-dropdown-portal .select-search-input {
  flex: 1 1 auto;
  min-width: 0;
  border: 0;
  background: transparent;
  font-size: 13px;
  color: var(--foreground);
  outline: none;
}

.select-dropdown-portal .select-search-input::placeholder {
  color: var(--muted);
}

.select-dropdown-portal .select-options {
  max-height: 320px;
  overflow-y: auto;
  outline: none;
}

/* Option · 36px, radius 9 */
.select-dropdown-portal .select-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: 36px;
  padding: 0 10px;
  border-radius: 9px;
  font-size: 13px;
  font-weight: 500;
  color: var(--foreground);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease;
  pointer-events: auto !important;
}

.select-dropdown-portal .select-option:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
}

.select-dropdown-portal .select-option-selected {
  background: color-mix(in oklch, var(--accent) 10%, transparent);
  color: var(--accent);
}

.select-dropdown-portal .select-option-check {
  flex: none;
  width: 14px;
  height: 14px;
  color: var(--accent);
}

.select-dropdown-portal .select-option-focused {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
}

.select-dropdown-portal .select-option-selected.select-option-focused {
  background: color-mix(in oklch, var(--accent) 14%, transparent);
}

.select-dropdown-portal .select-option-disabled {
  cursor: not-allowed;
  opacity: 0.4;
}

.select-dropdown-portal .select-option-group {
  height: 26px;
  cursor: default;
  user-select: none;
  background: transparent;
  font-size: 10px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--muted);
}

.select-dropdown-portal .select-option-group:hover {
  background: transparent;
}

.select-dropdown-portal .select-option-label {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left;
}

.select-dropdown-portal .select-empty {
  padding: 20px 10px;
  text-align: center;
  font-size: 12.5px;
  color: var(--muted);
}

.select-dropdown-enter-active,
.select-dropdown-leave-active {
  transition: all 0.15s ease;
}

.select-dropdown-enter-from,
.select-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
