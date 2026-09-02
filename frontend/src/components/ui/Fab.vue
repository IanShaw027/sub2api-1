<template>
  <button type="button" class="ui-fab btn-glass-primary" :aria-label="label" @click="$emit('click', $event)">
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
.ui-fab.btn-glass-primary {
  position: fixed;
  right: 16px;
  bottom: calc(28px + env(safe-area-inset-bottom, 0px));
  z-index: 40;
  min-width: 52px;
  height: 52px;
  padding: 0 20px;
  border-radius: 16px;
  font-size: 15px;
  box-shadow: 0 16px 32px -12px var(--accent);
}

.ui-fab:active {
  transform: scale(0.98);
}

.ui-fab:hover {
  filter: brightness(1.06);
}
</style>
