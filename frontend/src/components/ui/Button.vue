<template>
  <component
    :is="tag"
    :type="tag === 'button' ? nativeType : undefined"
    :to="to"
    :href="href"
    :disabled="isDisabled"
    :aria-busy="loading || undefined"
    :class="buttonClass"
    @click="handleClick"
  >
    <span v-if="loading" class="ui-btn-spinner" aria-hidden="true" />
    <slot />
  </component>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { RouteLocationRaw } from 'vue-router'
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
    size: 'sm',
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
  if (props.to) return 'router-link'
  if (props.href) return 'a'
  return 'button'
})

const buttonClass = computed(() => {
  const classes = ['ui-btn']
  if (props.variant === 'primary') classes.push('btn-glass-primary')
  else if (props.variant === 'secondary') classes.push('btn-glass-secondary')
  else if (props.variant === 'danger') classes.push('ui-btn-danger')
  else if (props.variant === 'ghost') classes.push('ui-btn-ghost')
  else if (props.variant === 'icon') classes.push('ui-btn-icon')

  if (props.size === 'md') classes.push('ui-btn-md')
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
</script>

<style scoped>
.ui-btn {
  text-decoration: none;
}

.ui-btn-md {
  height: 42px;
  padding: 0 18px;
  font-size: 14px;
}

.ui-btn-danger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: var(--radius-btn);
  background: var(--danger);
  color: #fff;
  font-size: 13px;
  font-weight: 600;
  border: 0;
  cursor: pointer;
  transition: filter 0.15s ease, transform 0.1s ease;
}

.ui-btn-md.ui-btn-danger {
  height: 42px;
}

.ui-btn-danger:hover {
  filter: brightness(1.06);
}

.ui-btn-danger:active {
  transform: scale(0.98);
}

.ui-btn-ghost {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 34px;
  padding: 0 12px;
  border-radius: var(--radius-btn);
  background: transparent;
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
  border: 0;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, transform 0.1s ease;
}

.ui-btn-md.ui-btn-ghost {
  height: 42px;
}

.ui-btn-ghost:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.ui-btn-ghost:active {
  transform: scale(0.98);
}

.ui-btn-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  padding: 0;
  border-radius: var(--radius-btn);
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--foreground);
  border: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.15s ease, transform 0.1s ease;
}

.ui-btn-icon:active {
  transform: scale(0.98);
}

.ui-btn-disabled,
.ui-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  pointer-events: none;
}

.ui-btn-spinner {
  width: 14px;
  height: 14px;
  border: 2px solid color-mix(in oklch, currentColor 30%, transparent);
  border-top-color: currentColor;
  border-radius: 50%;
  animation: ui-btn-spin 0.7s linear infinite;
}

@keyframes ui-btn-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
