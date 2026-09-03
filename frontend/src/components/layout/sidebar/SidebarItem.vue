<template>
 <div>
 <template v-if="item.children?.length">
 <div class="sidebar-group-root">
 <button
 ref="groupButtonRef"
 type="button"
 class="sidebar-item mb-0.5 w-full"
 :class="{
 'sidebar-item-active': isGroupActive && !isExpanded,
 'justify-center px-0': collapsed
 }"
 :title="collapsed ? item.label : undefined"
 :aria-label="collapsed ? item.label : undefined"
 :aria-expanded="collapsed ? (isTablet && flyoutOpen ? true : undefined) : isExpanded"
 :aria-controls="!omitTourAnchors && !collapsed && isExpanded ? groupChildrenId : undefined"
 @click="onGroupClick"
 >
 <span class="sidebar-item-icon">
 <component :is="item.icon" class="h-[17px] w-[17px] flex-shrink-0" />
 <span
 v-if="collapsed && badgeCount > 0"
 class="absolute -right-[3px] -top-[2px] h-[7px] w-[7px] rounded-full border-2 border-[var(--background)] bg-[var(--danger)]"
 />
 </span>
 <span
 v-if="!collapsed"
 class="flex min-w-0 flex-1 items-center gap-2 overflow-hidden"
 >
 <span class="min-w-0 flex-1 truncate">{{ item.label }}</span>
 <span
 v-if="badgeCount && !isExpanded"
 class="inline-flex h-2 w-2 flex-shrink-0 rounded-full bg-[var(--danger)]"
 />
 <svg
 class="h-[13px] w-[13px] flex-none text-[var(--muted)] transition-transform duration-150"
 :class="isExpanded ? 'rotate-180' : ''"
 viewBox="0 0 24 24"
 fill="none"
 stroke="currentColor"
 stroke-width="2"
 stroke-linecap="round"
 stroke-linejoin="round"
 >
 <path d="m6 9 6 6 6-6" />
 </svg>
 </span>
 </button>
 <Teleport to="body">
 <div
 v-if="collapsed && isTablet && flyoutOpen"
 ref="flyoutRef"
 class="sidebar-group-flyout"
 role="menu"
 :aria-label="item.label"
 :style="flyoutStyle"
 @click.stop
 >
 <router-link
 v-for="child in item.children"
 :key="child.path"
 :to="child.path"
 class="sidebar-group-flyout-item"
 role="menuitem"
 :class="{ 'sidebar-item-active': routePath === child.path }"
 :aria-current="routePath === child.path ? 'page' : undefined"
 @click="onFlyoutNavigate(child.path)"
 >
 <span v-if="child.icon" class="sidebar-item-icon">
 <component :is="child.icon" class="h-4 w-4" />
 </span>
 <span class="min-w-0 flex-1 truncate">{{ child.label }}</span>
 </router-link>
 </div>
 </Teleport>
 </div>
 <div
 v-if="!collapsed && isExpanded"
 :id="omitTourAnchors ? undefined : groupChildrenId"
 class="mb-1 ml-[17px] flex flex-col gap-px border-l border-[var(--border)] pl-[9px]"
 >
 <router-link
 v-for="child in item.children"
 :key="child.path"
 :to="child.path"
 class="sidebar-item text-[12.5px]"
 :class="{ 'sidebar-item-active': routePath === child.path }"
 :aria-current="routePath === child.path ? 'page' : undefined"
 @click="$emit('navigate', child.path)"
 >
 <span v-if="child.icon" class="sidebar-item-icon">
 <component :is="child.icon" class="h-4 w-4" />
 </span>
 <span class="min-w-0 flex-1 truncate">{{ child.label }}</span>
 <span
 v-if="child.badge"
 class="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-[var(--danger)] px-[5px] text-[10.5px] font-bold text-white"
 >{{ child.badge > 99 ? '99+' : child.badge }}</span>
 </router-link>
 </div>
 </template>
 <router-link
 v-else
 :to="item.path"
 class="sidebar-item mb-0.5"
 :class="{
 'sidebar-item-active': isActive,
 'justify-center px-0': collapsed
 }"
 :title="collapsed ? item.label : undefined"
 :aria-label="collapsed ? item.label : undefined"
 :aria-current="isActive ? 'page' : undefined"
 :id="omitTourAnchors ? undefined : domId"
 :data-tour="omitTourAnchors ? undefined : tourAttr"
 @click="$emit('navigate', item.path)"
 >
 <span class="sidebar-item-icon">
 <span
 v-if="item.iconSvg"
 class="sidebar-svg-icon h-[17px] w-[17px] flex-shrink-0"
 v-html="sanitizeSvg(item.iconSvg)"
 />
 <component v-else :is="item.icon" class="h-[17px] w-[17px] flex-shrink-0" />
 <span
 v-if="collapsed && item.badge"
 class="absolute -right-[3px] -top-[2px] h-[7px] w-[7px] rounded-full border-2 border-[var(--background)] bg-[var(--danger)]"
 />
 </span>
 <span v-if="!collapsed" class="min-w-0 flex-1 truncate">{{ item.label }}</span>
 <span
 v-if="!collapsed && item.badge"
 class="inline-flex h-[18px] min-w-[18px] items-center justify-center rounded-full bg-[var(--danger)] px-[5px] text-[10.5px] font-bold text-white"
 >{{ item.badge > 99 ? '99+' : item.badge }}</span>
 </router-link>
 </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { sanitizeSvg } from '@/utils/sanitize'
import { useIsMobile } from '@/composables/useIsMobile'
import { itemDomId, itemTourAttr, type NavItem } from './navSections'

const props = defineProps<{
 item: NavItem
 collapsed: boolean
 isActive: boolean
 isGroupActive: boolean
 isExpanded: boolean
 badgeCount: number
 routePath: string
 omitTourAnchors?: boolean
}>()

const emit = defineEmits<{
 navigate: [path: string]
 'group-click': [item: NavItem]
}>()

const { isTablet } = useIsMobile()
const flyoutOpen = ref(false)
const groupButtonRef = ref<HTMLElement | null>(null)
const flyoutRef = ref<HTMLElement | null>(null)
const flyoutStyle = ref<Record<string, string>>({})

const domId = computed(() => itemDomId(props.item.path))
const tourAttr = computed(() => itemTourAttr(props.item.path))
const groupChildrenId = computed(() => {
 const slug = props.item.path.replace(/^\//, '').replace(/\//g, '-')
 return `sidebar-group-${slug}`
})

function updateFlyoutPosition() {
 const trigger = groupButtonRef.value
 if (!trigger) return
 const rect = trigger.getBoundingClientRect()
 const padding = 8
 const left = rect.right + padding
 let top = rect.top
 const flyout = flyoutRef.value
 if (flyout) {
 const height = flyout.offsetHeight
 if (top + height > window.innerHeight - padding) {
 top = Math.max(padding, window.innerHeight - height - padding)
 }
 }
 flyoutStyle.value = {
 position: 'fixed',
 top: `${top}px`,
 left: `${left}px`,
 zIndex: '55'
 }
}

function closeFlyout() {
 flyoutOpen.value = false
}

function onGroupClick() {
 if (props.collapsed && isTablet.value && props.item.children?.length) {
 flyoutOpen.value = !flyoutOpen.value
 if (flyoutOpen.value) {
 updateFlyoutPosition()
 void nextTick(updateFlyoutPosition)
 }
 return
 }
 emit('group-click', props.item)
}

function onFlyoutNavigate(path: string) {
 closeFlyout()
 emit('navigate', path)
}

function onDocumentClick(event: MouseEvent) {
 if (!flyoutOpen.value) return
 const target = event.target as Node | null
 if (groupButtonRef.value?.contains(target)) return
 if (flyoutRef.value?.contains(target)) return
 closeFlyout()
}

function onDocumentKeydown(event: KeyboardEvent) {
 if (!flyoutOpen.value) return
 if (event.key === 'Escape') closeFlyout()
}

watch(() => props.routePath, closeFlyout)
watch([() => props.collapsed, isTablet], () => {
 if (!props.collapsed || !isTablet.value) closeFlyout()
})

watch(flyoutOpen, (open) => {
 if (open) {
 window.addEventListener('resize', updateFlyoutPosition)
 window.addEventListener('scroll', updateFlyoutPosition, true)
 } else {
 window.removeEventListener('resize', updateFlyoutPosition)
 window.removeEventListener('scroll', updateFlyoutPosition, true)
 }
})

onMounted(() => {
 document.addEventListener('click', onDocumentClick)
 document.addEventListener('keydown', onDocumentKeydown)
})

onBeforeUnmount(() => {
 document.removeEventListener('click', onDocumentClick)
 document.removeEventListener('keydown', onDocumentKeydown)
 window.removeEventListener('resize', updateFlyoutPosition)
 window.removeEventListener('scroll', updateFlyoutPosition, true)
})
</script>

<style scoped>
.sidebar-svg-icon {
 color: currentColor;
}

.sidebar-svg-icon :deep(svg) {
 display: block;
 width: 17px;
 height: 17px;
}

.sidebar-group-root {
 position: relative;
}

.sidebar-group-flyout {
 position: fixed;
 z-index: 55;
 min-width: 200px;
 padding: 6px;
 border-radius: 12px;
 background: color-mix(in oklch, var(--background) 92%, transparent);
 border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
 box-shadow: var(--shadow-hover);
 backdrop-filter: blur(20px);
}

.sidebar-group-flyout-item {
 display: flex;
 align-items: center;
 gap: 8px;
 height: 36px;
 padding: 0 10px;
 border-radius: 9px;
 font-size: 13px;
 font-weight: 500;
 color: var(--foreground);
 text-decoration: none;
}

.sidebar-group-flyout-item:hover {
 background: color-mix(in oklch, var(--foreground) 6%, transparent);
}
</style>
