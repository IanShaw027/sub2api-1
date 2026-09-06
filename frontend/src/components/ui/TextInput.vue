<template>
  <div class="ui-text-input">
    <FieldLabel v-if="label" :html-for="inputId" :hint="hint" :required="required">
      {{ label }}
    </FieldLabel>
    <input
      :id="inputId"
      ref="inputRef"
      v-bind="$attrs"
      class="field"
      :class="{ 'ui-text-input-error': error }"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      :readonly="readonly"
      :required="required"
      :autocomplete="autocomplete"
      :aria-invalid="error ? true : undefined"
      :aria-describedby="errorDescribedBy"
      @input="onInput"
      @blur="emit('blur', $event)"
      @focus="emit('focus', $event)"
    >
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
import { computed, ref, useId, useSlots } from 'vue'
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
  blur: [event: FocusEvent]
  focus: [event: FocusEvent]
}>()

const slots = useSlots()
const fallbackId = useId()
const inputId = computed(() => props.id ?? `ui-input-${fallbackId}`)
const inputRef = ref<HTMLInputElement | null>(null)
const errorDescribedBy = computed(() =>
  props.error || slots.error ? `${inputId.value}-error` : undefined
)

function onInput(event: Event) {
  const target = event.target as HTMLInputElement
  if (props.type !== 'number') {
    emit('update:modelValue', target.value)
    return
  }
  emit('update:modelValue', target.value === '' ? '' : Number(target.value))
}

defineExpose({ focus: () => inputRef.value?.focus() })
</script>

<style scoped>
.ui-text-input-error {
  border-color: var(--danger);
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
