<template>
  <component
    :is="tag"
    :type="tag === 'button' ? nativeType : undefined"
    v-bind="to ? { to } : href ? { href } : {}"
    :disabled="tag === 'button' ? isDisabled : undefined"
    :aria-disabled="isDisabled || undefined"
    :tabindex="isDisabled && tag !== 'button' ? -1 : undefined"
    :aria-busy="loading || undefined"
    :class="buttonClass"
    @click.capture="guardNavigation"
    @click="handleClick"
  >
    <span v-if="loading" class="ui-btn-spinner" aria-hidden="true" />
    <slot />
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
import { RouterLink } from 'vue-router'
import type { ButtonSize, ButtonVariant } from './types'

const props = withDefaults(
  defineProps<{
    variant?: ButtonVariant
    size?: ButtonSize
    loading?: boolean
    disabled?: boolean
    nativeType?: 'button' | 'submit' | 'reset'
    to?: RouteLocationRaw
    href?: string
  }>(),
  {
    variant: 'primary',
    size: 'md',
    loading: false,
    disabled: false,
    nativeType: 'button'
  }
)

const emit = defineEmits<{
  click: [event: MouseEvent]
}>()

const isDisabled = computed(() => props.disabled || props.loading)

const tag = computed(() => {
  if (props.to) return RouterLink
  if (props.href) return 'a'
  return 'button'
})

const buttonClass = computed(() => {
  const classes = ['btn', 'ui-btn']
  if (props.variant === 'primary') classes.push('btn-glass-primary')
  else if (props.variant === 'secondary') classes.push('btn-glass-secondary')
  else if (props.variant === 'danger') classes.push('btn-danger', 'ui-btn-danger')
  else if (props.variant === 'success') classes.push('btn-success', 'ui-btn-success')
  else if (props.variant === 'warning') classes.push('btn-warning', 'ui-btn-warning')
  else if (props.variant === 'ghost') classes.push('btn-ghost', 'ui-btn-ghost')
  else if (props.variant === 'icon') classes.push('btn-icon', 'btn-secondary', 'ui-btn-icon')

  if (props.size === 'lg') classes.push('btn-lg', 'ui-btn-lg')
  else if (props.size === 'sm') classes.push('btn-sm')
  else if (props.size === 'xs') classes.push('btn-xs')
  if (isDisabled.value) classes.push('ui-btn-disabled')
  return classes
})

function handleClick(event: MouseEvent) {
  if (isDisabled.value) {
    event.preventDefault()
    return
  }
  emit('click', event)
}

function guardNavigation(event: MouseEvent) {
  // RouterLink handles its own click before fallthrough listeners run.
  if (isDisabled.value) event.preventDefault()
}
</script>
