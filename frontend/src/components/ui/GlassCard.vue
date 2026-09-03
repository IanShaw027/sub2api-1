<template>
  <div :class="rootClass">
    <div v-if="$slots.header" class="ui-glass-card-header">
      <slot name="header" />
    </div>
    <div :class="bodyClass">
      <slot />
    </div>
    <div v-if="$slots.footer" class="ui-glass-card-footer">
      <slot name="footer" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, useSlots } from 'vue'
import type { GlassCardPadding, GlassCardVariant } from './types'

const slots = useSlots()

const props = withDefaults(
  defineProps<{
    variant?: GlassCardVariant
    hover?: boolean
    padding?: GlassCardPadding
  }>(),
  {
    variant: 'glass',
    hover: false,
    padding: 'md'
  }
)

const variantClass = computed(() => {
  switch (props.variant) {
    case 'solid':
      return 'glass-card-solid'
    case 'transparent':
      return 'ui-glass-card-transparent'
    default:
      return 'glass-card'
  }
})

const paddingClass = computed(() => {
  switch (props.padding) {
    case 'sm':
      return 'ui-glass-card-pad-sm'
    case 'lg':
      return 'ui-glass-card-pad-lg'
    default:
      return 'ui-glass-card-pad-md'
  }
})

const rootClass = computed(() => [
  variantClass.value,
  paddingClass.value,
  props.hover ? 'glass-card-hover' : null
])

const bodyClass = computed(() => (slots.header || slots.footer ? 'ui-glass-card-body' : null))
</script>

<style scoped>
.ui-glass-card-transparent {
  background: transparent;
  border: 1px solid color-mix(in oklch, var(--border) 55%, transparent);
  border-radius: var(--radius-card);
}

.ui-glass-card-pad-sm {
  padding: 12px;
}

.ui-glass-card-pad-md {
  padding: 16px;
}

.ui-glass-card-pad-lg {
  padding: 20px;
}

.ui-glass-card-header {
  margin: -16px -16px 0;
  padding: 16px 16px 12px;
  border-bottom: 1px solid var(--border);
}

.ui-glass-card-pad-sm .ui-glass-card-header {
  margin: -12px -12px 0;
  padding: 10px 12px;
}

.ui-glass-card-pad-lg .ui-glass-card-header {
  margin: -20px -20px 0;
  padding: 16px 20px 12px;
}

.ui-glass-card-body {
  padding-top: 14px;
}

.ui-glass-card-footer {
  margin: 0 -16px -16px;
  padding: 12px 16px;
  border-top: 1px solid var(--border);
}

.ui-glass-card-pad-sm .ui-glass-card-footer {
  margin: 0 -12px -12px;
  padding: 10px 12px;
}

.ui-glass-card-pad-lg .ui-glass-card-footer {
  margin: 0 -20px -20px;
  padding: 14px 20px;
}
</style>
