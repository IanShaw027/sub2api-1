<template>
  <button
    type="button"
    :role="switchRole ? 'switch' : undefined"
    :aria-checked="switchRole ? modelValue : undefined"
    @click="onClick"
    :class="[
 'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
 modelValue ? 'bg-accent' : 'bg-surface-3'
 ]"
  >
    <span
      :class="[
        'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
        modelValue ? 'translate-x-5' : 'translate-x-0'
      ]"
    />
  </button>
</template>

<script setup lang="ts">
// Shared visual/behavioral primitive for the many inline "flag toggle" rows
// duplicated across CreateAccountModal.vue and EditAccountModal.vue. Markup
// and Tailwind classes are copied verbatim from the original inline buttons
// so rendered DOM/pixels are unchanged.
//
// `data-testid` is intentionally NOT a declared prop: passing it as a plain
// attribute on <InlineToggleSwitch data-testid="..." /> lets Vue's default
// attribute fallthrough forward it verbatim onto the root <button>, which
// keeps the literal `data-testid="..."` string in the host file's source
// (important for anchor-diff / raw-source-based tests) while still rendering
// identical DOM to the original inline markup.
const modelValue = defineModel<boolean>({ required: true })

const props = withDefaults(
  defineProps<{
    /** When true, also renders role="switch" + :aria-checked (a few call sites only). */
    switchRole?: boolean
    /**
     * When false, clicking only emits `click` and does not flip modelValue itself —
     * used where the host component owns a custom handler with extra side effects.
     */
    autoToggle?: boolean
  }>(),
  { autoToggle: true }
)

const emit = defineEmits<{ (e: 'click'): void }>()

function onClick() {
  emit('click')
  if (props.autoToggle) {
    modelValue.value = !modelValue.value
  }
}
</script>
