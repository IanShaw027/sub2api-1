<template>
 <SidebarSection
 v-for="(section, index) in sections"
 :key="section.key"
 :section="section"
 :open="isSectionOpen(section)"
 :collapsed="collapsed"
 :is-first="index === 0"
 :route-path="routePath"
 :is-item-active="isItemActive"
 :is-group-active="isGroupActive"
 :is-group-expanded="isGroupExpanded"
 :group-badge="groupBadge"
 :omit-tour-anchors="omitTourAnchors"
 @toggle="$emit('toggle', $event)"
 @navigate="$emit('navigate', $event)"
 @group-click="$emit('group-click', $event)"
 />
</template>

<script setup lang="ts">
import SidebarSection from './SidebarSection.vue'
import type { NavItem, NavSection } from './navSections'

withDefaults(defineProps<{
 sections: NavSection[]
 collapsed: boolean
 routePath: string
 isSectionOpen: (section: NavSection) => boolean
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
</script>
