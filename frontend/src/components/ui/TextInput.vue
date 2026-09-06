<template>
  <div class="ui-text-input">
    <FieldLabel v-if="label || $slots.label" :html-for="inputId" :hint="hint" :required="required">
      <slot name="label">{{ label }}</slot>
    </FieldLabel>
    <div :class="['ui-text-input-control', $slots.suffix && 'has-suffix']">
      <input
        :id="inputId"
        ref="inputRef"
        v-bind="$attrs"
        class="field"
        :class="{ 'ui-text-input-error': hasError, 'ui-text-input-with-suffix': $slots.suffix }"
        :type="type"
        :value="modelValue"
        :placeholder="placeholder"
        :disabled="disabled"
        :readonly="readonly"
        :required="required"
        :autocomplete="autocomplete"
        :aria-invalid="hasError ? true : ariaInvalid()"
        :aria-describedby="errorDescribedBy()"
        @input="onInput"
        @blur="emit('blur', $event)"
        @focus="emit('focus', $event)"
      >
      <span v-if="$slots.suffix" class="ui-text-input-suffix"><slot name="suffix" /></span>
    </div>
    <p
      v-if="error || $slots.error"
      :id="`${inputId}-error`"
      class="ui-text-input-error-text"
      role="alert"
    >
      <slot name="error">{{ error }}</slot>
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useAttrs, useId, useSlots, type InputHTMLAttributes } from 'vue'
import FieldLabel from './FieldLabel.vue'

defineOptions({ inheritAttrs: false })

const props = withDefaults(
  defineProps<{
    modelValue?: string | number
    label?: string
    hint?: string
    error?: string
    placeholder?: string
    type?: string
    disabled?: boolean
    readonly?: boolean
    required?: boolean
    autocomplete?: string
    id?: string
  }>(),
  {
    modelValue: '',
    type: 'text',
    disabled: false,
    readonly: false,
    required: false
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
  input: [event: Event]
  blur: [event: FocusEvent]
  focus: [event: FocusEvent]
}>()

const slots = useSlots()
const attrs = useAttrs()
const fallbackId = useId()
const inputId = computed(() => props.id ?? `ui-input-${fallbackId}`)
const inputRef = ref<HTMLInputElement | null>(null)
const hasError = computed(() => Boolean(props.error || slots.error))
function ariaInvalid() {
  return attrs['aria-invalid'] as InputHTMLAttributes['aria-invalid']
}
function errorDescribedBy() {
  return [attrs['aria-describedby'], hasError.value ? `${inputId.value}-error` : undefined]
    .filter(Boolean)
    .join(' ') || undefined
}

function onInput(event: Event) {
  const target = event.target as HTMLInputElement
  if (props.type !== 'number') {
    emit('update:modelValue', target.value)
  } else {
    emit('update:modelValue', target.value === '' ? '' : Number(target.value))
  }
  // Native v-model updates the value before user input handlers run.
  emit('input', event)
}

defineExpose({ focus: () => inputRef.value?.focus() })
</script>

<style scoped>
.ui-text-input-error {
  border-color: var(--danger);
}

.ui-text-input-control {
  position: relative;
}

.ui-text-input-control.has-suffix .field {
  padding-right: 42px;
}

.ui-text-input-suffix {
  position: absolute;
  top: 50%;
  right: 8px;
  display: inline-flex;
  align-items: center;
  transform: translateY(-50%);
}

.ui-text-input-error:focus,
.ui-text-input-error:focus-visible {
  border-color: var(--danger);
  box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--danger) 14%, transparent);
}

.ui-text-input-error-text {
  margin-top: 4px;
  font-size: 12px;
  color: var(--danger-text);
}
</style>
