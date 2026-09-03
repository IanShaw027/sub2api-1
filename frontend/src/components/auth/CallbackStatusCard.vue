<template>
  <div class="callback-status-card glass-card glass-ring" data-testid="callback-status-card" :data-status="status">
    <div class="callback-status-icon" :class="`tone-${status}`">
      <span v-if="status === 'loading'" class="callback-status-spinner" aria-hidden="true" />
      <Icon v-else :name="status === 'success' ? 'checkCircle' : 'xCircle'" size="lg" :stroke-width="2" />
    </div>

    <h2 class="callback-status-title">{{ title }}</h2>
    <p v-if="description" class="callback-status-desc">{{ description }}</p>

    <pre v-if="detail" class="code-block callback-status-detail">{{ detail }}</pre>

    <div v-if="$slots.default" class="callback-status-actions">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import Icon from '@/components/icons/Icon.vue'

withDefaults(
  defineProps<{
    status: 'loading' | 'success' | 'error'
    title: string
    description?: string
    detail?: string
  }>(),
  {
    description: undefined,
    detail: undefined
  }
)
</script>

<style scoped>
.callback-status-card {
  width: 100%;
  max-width: 440px;
  margin: 0 auto;
  padding: 36px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 8px;
}

.callback-status-icon {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 6px;
  flex: none;
}

.callback-status-icon.tone-loading {
  background: color-mix(in oklch, var(--accent) 18%, transparent);
  color: var(--accent);
}

.callback-status-icon.tone-success {
  background: color-mix(in oklch, var(--success) 18%, transparent);
  color: var(--success-text);
}

.callback-status-icon.tone-error {
  background: color-mix(in oklch, var(--danger) 18%, transparent);
  color: var(--danger-text);
}

.callback-status-spinner {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 2px solid color-mix(in oklch, var(--accent) 35%, transparent);
  border-top-color: var(--accent);
  animation: spin 0.8s linear infinite;
}

.callback-status-title {
  font-size: 20px;
  font-weight: 800;
  color: var(--foreground);
  line-height: 1.3;
}

.callback-status-desc {
  font-size: 13.5px;
  color: var(--muted);
  line-height: 1.6;
}

.callback-status-detail {
  width: 100%;
  margin-top: 4px;
  text-align: left;
}

.callback-status-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: center;
  gap: 10px;
  margin-top: 10px;
  width: 100%;
}

.callback-status-actions :deep(.ui-btn) {
  min-width: 160px;
}
</style>
