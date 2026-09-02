<template>
  <button
    type="button"
    role="switch"
    class="ui-toggle"
    :class="sizeClass"
    :aria-checked="modelValue"
    :disabled="disabled"
    @click="toggle"
  >
    <span class="ui-toggle-thumb" :class="{ 'is-on': modelValue }" />
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ToggleSwitchSize } from './types'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    size?: ToggleSwitchSize
    disabled?: boolean
  }>(),
  {
    size: 'form',
    disabled: false
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const sizeClass = computed(() =>
  props.size === 'compact' ? 'ui-toggle-compact' : 'ui-toggle-form'
)

function toggle() {
  if (props.disabled) return
  emit('update:modelValue', !props.modelValue)
}
</script>

<style scoped>
.ui-toggle {
  position: relative;
  display: inline-flex;
  align-items: center;
  flex: none;
  border: 0;
  padding: 0;
  cursor: pointer;
  border-radius: 999px;
  background: color-mix(in oklch, var(--foreground) 14%, transparent);
  transition: background 0.15s ease;
}

.ui-toggle:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.ui-toggle:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ui-toggle-compact {
  width: 32px;
  height: 18px;
}

.ui-toggle-form {
  width: 36px;
  height: 20px;
}

.ui-toggle[aria-checked='true'] {
  background: var(--accent);
}

.ui-toggle-thumb {
  position: absolute;
  left: 2px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  transition: transform 0.15s ease;
}

.ui-toggle-compact .ui-toggle-thumb {
  width: 14px;
  height: 14px;
}

.ui-toggle-form .ui-toggle-thumb {
  width: 16px;
  height: 16px;
}

.ui-toggle-compact .ui-toggle-thumb.is-on {
  transform: translateX(14px);
}

.ui-toggle-form .ui-toggle-thumb.is-on {
  transform: translateX(16px);
}
</style>
