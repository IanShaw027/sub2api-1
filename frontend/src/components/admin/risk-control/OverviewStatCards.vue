<template>
  <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
    <GlassCard
      v-for="item in items"
      :key="item.key"
      padding="sm"
    >
      <div class="flex min-w-0 items-center gap-3">
        <div class="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg" :class="item.iconClass">
          <Icon :name="item.icon" size="sm" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="flex min-w-0 items-center justify-between gap-2">
            <p class="truncate text-xs font-medium text-muted">{{ item.label }}</p>
            <span
              v-if="item.badge"
              class="inline-flex flex-shrink-0 items-center rounded-full px-2 py-0.5 text-xs font-medium"
              :class="item.badgeClass"
            >
              {{ item.badge }}
            </span>
          </div>
          <div class="mt-1 flex min-w-0 items-baseline gap-2">
            <p class="truncate text-xl font-semibold leading-7 text-foreground">{{ item.value }}</p>
            <p v-if="item.meta" class="truncate text-xs text-muted">{{ item.meta }}</p>
          </div>
        </div>
      </div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import GlassCard from '@/components/ui/GlassCard.vue'
import Icon from '@/components/icons/Icon.vue'
import type { OverviewItem } from './types'

defineProps<{
  items: OverviewItem[]
}>()
</script>
