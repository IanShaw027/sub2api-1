<template>
  <div class="cell-platform">
    <span
      class="cell-platform-tile"
      :style="{ background: platformTileBackground(platform) }"
    >
      <PlatformIcon :platform="(platform as GroupPlatform)" size="sm" class="cell-platform-icon" />
    </span>
    <span class="cell-platform-name">{{ label ?? platformLabel(platform) }}</span>
  </div>
</template>

<script setup lang="ts">
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import type { GroupPlatform } from '@/types'
import { platformTileBackground, platformLabel } from '@/utils/platformTile'

withDefaults(
  defineProps<{
    platform: string
    /** Override the auto-resolved platform label. */
    label?: string | null
  }>(),
  {
    label: null
  }
)
</script>

<style scoped>
.cell-platform {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.cell-platform-tile {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.14);
  color: #fff;
}

.cell-platform-icon {
  color: #fff;
}

.cell-platform-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
