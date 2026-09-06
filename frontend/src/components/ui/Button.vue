<template>
  <component
    :is="tag"
    :type="tag === 'button' ? nativeType : undefined"
    :to="to"
    :href="href"
    :disabled="tag === 'button' ? isDisabled : undefined"
    :aria-disabled="isDisabled || undefined"
    :tabindex="isDisabled && tag !== 'button' ? -1 : undefined"
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
  if (props.to) return 'router-link'
  if (props.href) return 'a'
  return 'button'
})

const buttonClass = computed(() => {
  const classes = ['ui-btn']
  if (props.variant === 'primary') classes.push('btn-glass-primary')
  else if (props.variant === 'secondary') classes.push('btn-glass-secondary')
  else if (props.variant === 'danger') classes.push('ui-btn-danger')
  else if (props.variant === 'success') classes.push('ui-btn-success')
  else if (props.variant === 'warning') classes.push('ui-btn-warning')
  else if (props.variant === 'ghost') classes.push('ui-btn-ghost')
  else if (props.variant === 'icon') classes.push('ui-btn-icon')

  if (props.size === 'lg') classes.push('ui-btn-lg')
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
</script>

<style scoped>
.ui-btn {
  text-decoration: none;
}

.ui-btn-lg,
.ui-btn-lg.btn-glass-primary,
.ui-btn-lg.btn-glass-secondary {
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
  background: color-mix(in oklch, var(--danger) 12%, transparent);
  color: var(--danger-text);
  font-size: 13px;
  font-weight: 600;
  border: 0;
  cursor: pointer;
  transition: filter 0.15s ease, transform 0.1s ease, background 0.15s ease;
}

.ui-btn-lg.ui-btn-danger {
  height: 42px;
}

.ui-btn-danger:hover {
  filter: brightness(1.06);
}

.ui-btn-danger:active {
  transform: scale(0.98);
}

.ui-btn-success,
.ui-btn-warning {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: var(--radius-btn);
  font-size: 13px;
  font-weight: 600;
  border: 0;
  cursor: pointer;
  transition: filter 0.15s ease, transform 0.1s ease, background 0.15s ease;
}

.ui-btn-success {
  background: color-mix(in oklch, var(--success) 16%, transparent);
  color: var(--success-text);
}

.ui-btn-warning {
  background: color-mix(in oklch, var(--warning) 18%, transparent);
  color: var(--warning-text);
}

.ui-btn-lg.ui-btn-success,
.ui-btn-lg.ui-btn-warning {
  height: 42px;
}

.ui-btn-success:hover {
  filter: brightness(1.06);
}

.ui-btn-warning:hover {
  filter: brightness(1.06);
}

.ui-btn-success:active,
.ui-btn-warning:active {
  transform: scale(0.98);
}

/* xs (26px r8) / sm (32px r9) sizes for variants not backed by the
   global `.btn` selector (danger / success / warning / ghost / icon). */
.ui-btn-danger.btn-xs,
.ui-btn-success.btn-xs,
.ui-btn-warning.btn-xs,
.ui-btn-ghost.btn-xs {
  height: 26px;
  padding: 0 9px;
  border-radius: 8px;
  font-size: 12px;
}

.ui-btn-danger.btn-sm,
.ui-btn-success.btn-sm,
.ui-btn-warning.btn-sm,
.ui-btn-ghost.btn-sm {
  height: 32px;
  padding: 0 12px;
  border-radius: 9px;
  font-size: 12.5px;
}

.ui-btn-icon.btn-xs {
  width: 28px;
  height: 28px;
  border-radius: 8px;
}

.ui-btn-icon.btn-sm {
  width: 32px;
  height: 32px;
  border-radius: 9px;
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

.ui-btn-lg.ui-btn-ghost {
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
  width: 34px;
  height: 34px;
  padding: 0;
  border-radius: var(--radius-btn);
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--muted);
  border: 1px solid var(--border);
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, transform 0.1s ease;
}

.ui-btn-lg.ui-btn-icon {
  width: 42px;
  height: 42px;
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
