<template>
  <div class="sidebar-section" :data-section="section.key">
    <button
      v-if="!collapsed"
      type="button"
      class="sidebar-section-title"
      :aria-expanded="open"
      :aria-controls="sectionItemsId"
      @click="$emit('toggle', section.key)"
    >
      <span>{{ t(section.titleKey) }}</span>
      <span class="flex items-center gap-1.5">
        <span
          v-if="!open"
          class="inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-[var(--surface-secondary)] px-1 text-[10px] font-semibold"
        >{{ section.items.length }}</span>
        <svg
          class="h-3 w-3 transition-transform duration-150"
          :class="open ? 'rotate-0' : '-rotate-90'"
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
    <div v-else-if="!isFirst" class="sidebar-section-divider" />
    <div
      :id="sectionItemsId"
      class="sidebar-section-items"
      :class="{ hidden: !open && !collapsed }"
      :aria-hidden="!open && !collapsed ? 'true' : 'false'"
    >
      <SidebarItem
        v-for="item in section.items"
        :key="item.path"
        :item="item"
        :collapsed="collapsed"
        :is-active="isItemActive(item)"
        :is-group-active="isGroupActive(item)"
        :is-expanded="isGroupExpanded(item)"
        :badge-count="groupBadge(item)"
        :route-path="routePath"
        :omit-tour-anchors="omitTourAnchors"
        @navigate="$emit('navigate', $event)"
        @group-click="$emit('group-click', $event)"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import SidebarItem from './SidebarItem.vue'
import type { NavItem, NavSection } from './navSections'

const props = withDefaults(defineProps<{
  section: NavSection
  open: boolean
  collapsed: boolean
  isFirst: boolean
  routePath: string
  isItemActive: (item: NavItem) => boolean
  isGroupActive: (item: NavItem) => boolean
  isGroupExpanded: (item: NavItem) => boolean
  groupBadge: (item: NavItem) => number
  omitTourAnchors?: boolean
}>(), {
  omitTourAnchors: false
})

defineEmits<{
  toggle: [key: string]
  navigate: [path: string]
  'group-click': [item: NavItem]
}>()

const { t } = useI18n()

const sectionItemsId = computed(() =>
  props.omitTourAnchors ? undefined : `sidebar-section-${props.section.key}`
)
</script>
