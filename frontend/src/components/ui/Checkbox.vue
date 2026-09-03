<template>
  <label class="ui-checkbox" :class="{ 'is-disabled': disabled }">
    <input
      ref="inputRef"
      v-bind="$attrs"
      type="checkbox"
      class="ui-checkbox-input"
      :checked="modelValue"
      :disabled="disabled"
      @change="onChange"
    >
    <span
      class="ui-checkbox-box"
      :class="{ 'is-checked': modelValue, 'is-indeterminate': indeterminate && !modelValue }"
      aria-hidden="true"
    >
      <svg v-if="modelValue" viewBox="0 0 12 10" class="ui-checkbox-icon">
        <path d="M1 5.2 4.2 8.4 11 1.6" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" />
      </svg>
      <span v-else-if="indeterminate" class="ui-checkbox-dash" />
    </span>
    <span v-if="$slots.default" class="ui-checkbox-label">
      <slot />
    </span>
  </label>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    disabled?: boolean
    indeterminate?: boolean
  }>(),
  {
    disabled: false,
    indeterminate: false
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const inputRef = ref<HTMLInputElement | null>(null)

watch(
  [() => props.indeterminate, inputRef],
  () => {
    if (inputRef.value) inputRef.value.indeterminate = props.indeterminate
  },
  { immediate: true }
)

function onChange(event: Event) {
  const target = event.target as HTMLInputElement
  emit('update:modelValue', target.checked)
}
</script>

<style scoped>
.ui-checkbox {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}

.ui-checkbox.is-disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ui-checkbox-input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}

.ui-checkbox-box {
  width: 16px;
  height: 16px;
  border-radius: 5px;
  border: 1.5px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  box-shadow: var(--field-shadow);
  transition: background 0.15s ease, border-color 0.15s ease;
}

.ui-checkbox-input:checked + .ui-checkbox-box,
.ui-checkbox-box.is-indeterminate {
  background: var(--accent);
  border-color: var(--accent);
}

.ui-checkbox-dash {
  width: 8px;
  height: 1.5px;
  border-radius: 1px;
  background: #fff;
}

.ui-checkbox-input:focus-visible + .ui-checkbox-box {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.ui-checkbox-icon {
  width: 11px;
  height: 9px;
}

.ui-checkbox-label {
  font-size: 13px;
  color: var(--foreground);
}
</style>
