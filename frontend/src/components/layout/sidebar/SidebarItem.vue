<template>
  <div>
    <template v-if="item.children?.length">
      <button
        type="button"
        class="sidebar-item mb-0.5 w-full"
        :class="{
          'sidebar-item-active': isGroupActive && !isExpanded,
          'justify-center px-0': collapsed
        }"
        :title="collapsed ? item.label : undefined"
        :aria-label="collapsed ? item.label : undefined"
        :aria-expanded="collapsed ? undefined : isExpanded"
        :aria-controls="!collapsed && isExpanded ? groupChildrenId : undefined"
        @click="$emit('group-click', item)"
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
      <div
        v-if="!collapsed && isExpanded"
        :id="groupChildrenId"
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
      :id="domId"
      :data-tour="tourAttr"
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
import { computed } from 'vue'
import { sanitizeSvg } from '@/utils/sanitize'
import { itemDomId, itemTourAttr, type NavItem } from './navSections'

const props = defineProps<{
  item: NavItem
  collapsed: boolean
  isActive: boolean
  isGroupActive: boolean
  isExpanded: boolean
  badgeCount: number
  routePath: string
}>()

defineEmits<{
  navigate: [path: string]
  'group-click': [item: NavItem]
}>()

const domId = computed(() => itemDomId(props.item.path))
const tourAttr = computed(() => itemTourAttr(props.item.path))
const groupChildrenId = computed(() => {
  const slug = props.item.path.replace(/^\//, '').replace(/\//g, '-')
  return `sidebar-group-${slug}`
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
</style>
