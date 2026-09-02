<template>
  <button type="button" class="ui-fab" :aria-label="label" @click="$emit('click', $event)">
    <slot />
  </button>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    label?: string
  }>(),
  {
    label: 'Action'
  }
)

defineEmits<{ click: [event: MouseEvent] }>()
</script>

<style scoped>
.ui-fab {
  position: fixed;
  right: 16px;
  bottom: calc(28px + env(safe-area-inset-bottom, 0px));
  z-index: 40;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-width: 52px;
  height: 52px;
  padding: 0 18px;
  border: 0;
  border-radius: 16px;
  background: var(--accent);
  color: #fff;
  font-size: 14px;
  font-weight: 700;
  box-shadow: 0 12px 28px -10px var(--accent);
  cursor: pointer;
  transition: transform 0.1s ease, filter 0.15s ease;
}

.ui-fab:active {
  transform: scale(0.98);
}

.ui-fab:hover {
  filter: brightness(1.06);
}
</style>
