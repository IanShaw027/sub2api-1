<template>
  <button
    type="button"
    @click="toggle"
    class="switch toggle-switch"
    :class="{ 'switch-active': modelValue }"
    role="switch"
    :aria-checked="modelValue"
  >
    <span class="switch-thumb" aria-hidden="true" />
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  emit('update:modelValue', !props.modelValue)
}
</script>

<style scoped>
/* Geometry (36×20, 16px thumb, accent + inset shadow when on) comes from the
   global `.switch` / `.switch-thumb` / `.switch-active` recipe. */
.toggle-switch {
  padding: 0;
  border-width: 1px;
  border-style: solid;
  appearance: none;
}

.toggle-switch:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}
</style>
