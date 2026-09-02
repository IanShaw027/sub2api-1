<template>
 <div class="flex flex-col gap-1">
 <span
 :class="[
 'inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium',
 statusClass(current, max)
 ]"
 >
 <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
 <path stroke-linecap="round" stroke-linejoin="round" d="M3.75 6A2.25 2.25 0 016 3.75h2.25A2.25 2.25 0 0110.5 6v2.25a2.25 2.25 0 01-2.25 2.25H6a2.25 2.25 0 01-2.25-2.25V6zM3.75 15.75A2.25 2.25 0 016 13.5h2.25a2.25 2.25 0 012.25 2.25V18a2.25 2.25 0 01-2.25 2.25H6A2.25 2.25 0 013.75 18v-2.25zM13.5 6a2.25 2.25 0 012.25-2.25H18A2.25 2.25 0 0120.25 6v2.25A2.25 2.25 0 0118 10.5h-2.25a2.25 2.25 0 01-2.25-2.25V6zM13.5 15.75a2.25 2.25 0 012.25-2.25H18a2.25 2.25 0 012.25 2.25V18A2.25 2.25 0 0118 20.25h-2.25A2.25 2.25 0 0113.5 18v-2.25z" />
 </svg>
 <span class="font-mono">{{ current }}</span>
 <span class="text-muted">/</span>
 <span class="font-mono">{{ max }}</span>
 </span>
 <span
 :class="[
 'inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-medium',
 statusClass(rpmUsed, rpmMax)
 ]"
 >
 <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
 <path stroke-linecap="round" stroke-linejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
 </svg>
 <span class="font-mono">{{ rpmUsed }}</span>
 <span class="text-muted">/</span>
 <span class="font-mono">{{ rpmMax > 0 ? rpmMax : '∞' }}</span>
 </span>
 </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{
 current: number
 max: number
 rpmUsed?: number
 rpmMax?: number
}>(), {
 rpmUsed: 0,
 rpmMax: 0,
})

function statusClass(used: number, limit: number) {
 if (used >= limit && limit > 0) {
 return 'bg-red-100 text-red-700'
 }
 if (used > 0) {
 return 'bg-yellow-100 text-yellow-700'
 }
 return 'bg-surface-2 text-muted'
}
</script>
