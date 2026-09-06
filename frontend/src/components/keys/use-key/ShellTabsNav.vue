<template>
 <div class="overflow-x-auto border-b border-line">
 <nav class="-mb-px flex min-w-max gap-4" aria-label="Tabs">
 <button
 v-for="tab in tabs"
 :key="tab.id"
 type="button"
 @click="activeTab = tab.id"
 :class="[
 'whitespace-nowrap py-2.5 px-1 border-b-2 font-medium text-sm transition-colors',
 activeTab === tab.id
 ? 'border-accent text-accent'
 : 'border-transparent text-muted hover:text-foreground hover:border-line'
 ]"
 >
 <span class="flex items-center gap-2">
 <component :is="tab.icon" class="w-4 h-4" />
 {{ tab.label }}
 </span>
 </button>
 </nav>
 </div>
</template>

<script setup lang="ts">
import type { TabConfig } from './types'

defineProps<{
 tabs: TabConfig[]
}>()

const activeTab = defineModel<string>({ required: true })
</script>
