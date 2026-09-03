<template>
  <Teleport to="body">
    <div
      class="pointer-events-none fixed right-4 top-4 z-[9999] space-y-3"
      aria-live="polite"
      aria-atomic="true"
    >
      <TransitionGroup
        enter-active-class="transition ease-out duration-300"
        enter-from-class="opacity-0 translate-x-full"
        enter-to-class="opacity-100 translate-x-0"
        leave-active-class="transition ease-in duration-200"
        leave-from-class="opacity-100 translate-x-0"
        leave-to-class="opacity-0 translate-x-full"
      >
        <div
          v-for="toast in toasts"
          :key="toast.id"
          :class="['toast-item pointer-events-auto', `toast-${toast.type}`]"
        >
          <div class="toast-item-body">
            <!-- Tone icon · 22px circle (16% tint) -->
            <span class="toast-icon" aria-hidden="true">
              <Icon :name="getToastIconName(toast.type)" size="xs" :stroke-width="2.4" />
            </span>

            <!-- Content -->
            <div class="min-w-0 flex-1">
              <p v-if="toast.title" class="toast-title">
                {{ toast.title }}
              </p>
              <p :class="['toast-message', toast.title ? null : 'toast-message-solo']">
                {{ toast.message }}
              </p>
            </div>

            <!-- Close button · 14px glyph in a 28px hit area -->
            <button
              type="button"
              @click="removeToast(toast.id)"
              class="toast-close"
              aria-label="Close notification"
            >
              <Icon name="x" size="sm" :stroke-width="2" />
            </button>
          </div>

          <!-- Progress bar · 2px, tone coloured -->
          <div v-if="toast.duration" class="toast-progress-track">
            <div
              class="toast-progress"
              :style="{ animationDuration: `${toast.duration}ms` }"
            ></div>
          </div>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'

const appStore = useAppStore()

const toasts = computed(() => appStore.toasts)

const getToastIconName = (type: string): 'checkCircle' | 'xCircle' | 'exclamationTriangle' | 'infoCircle' => {
  switch (type) {
    case 'success':
      return 'checkCircle'
    case 'error':
      return 'xCircle'
    case 'warning':
      return 'exclamationTriangle'
    case 'info':
    default:
      return 'infoCircle'
  }
}

const removeToast = (id: string) => {
  appStore.hideToast(id)
}
</script>

<style scoped>
/* Card · 12px 14px, radius 12, surface + border + shadow-hover (prototype 07). */
.toast-item {
  position: relative;
  min-width: 320px;
  max-width: 420px;
  overflow: hidden;
  border-radius: 12px;
  background: var(--surface);
  border: 1px solid var(--border);
  box-shadow: var(--shadow-hover);
}

.toast-item-body {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 14px;
}

.toast-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  flex: none;
  border-radius: 999px;
  background: var(--surface-secondary);
  color: var(--muted);
}

.toast-success .toast-icon {
  background: color-mix(in oklch, var(--success) 16%, transparent);
  color: var(--success-text);
}

.toast-error .toast-icon {
  background: color-mix(in oklch, var(--danger) 14%, transparent);
  color: var(--danger-text);
}

.toast-warning .toast-icon {
  background: color-mix(in oklch, var(--warning) 18%, transparent);
  color: var(--warning-text);
}

.toast-info .toast-icon {
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
}

.toast-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.toast-message {
  font-size: 12px;
  line-height: 1.5;
  color: var(--muted);
}

.toast-message-solo {
  font-size: 13px;
  color: var(--foreground);
}

.toast-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  margin: -4px -6px -4px 0;
  flex: none;
  border-radius: 8px;
  color: var(--muted);
  transition: background 0.15s ease, color 0.15s ease;
}

.toast-close svg {
  width: 14px;
  height: 14px;
}

.toast-close:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.toast-progress-track {
  height: 2px;
  background: var(--surface-secondary);
}

.toast-progress {
  width: 100%;
  height: 100%;
  background: var(--muted);
  animation-name: toast-progress-shrink;
  animation-timing-function: linear;
  animation-fill-mode: forwards;
}

.toast-success .toast-progress {
  background: var(--success);
}

.toast-error .toast-progress {
  background: var(--danger);
}

.toast-warning .toast-progress {
  background: var(--warning);
}

.toast-info .toast-progress {
  background: var(--accent);
}

@keyframes toast-progress-shrink {
  from {
    width: 100%;
  }
  to {
    width: 0%;
  }
}
</style>
