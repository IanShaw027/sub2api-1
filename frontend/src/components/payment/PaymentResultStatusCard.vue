<template>
  <div class="result-status glass-card glass-ring">
    <div class="result-status-icon" :class="`tone-${tone}`">
      <span v-if="tone === 'loading'" class="result-status-spinner" aria-hidden="true" />
      <svg v-else-if="tone === 'success'" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M5 13l4 4L19 7" />
      </svg>
      <svg v-else class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
      </svg>
    </div>
    <h1 class="result-status-title">{{ title }}</h1>
    <p v-if="description" class="result-status-desc">{{ description }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  tone: 'loading' | 'success' | 'error'
  title: string
  description?: string
}>()
</script>

<style scoped>
.result-status {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 36px;
  gap: 8px;
}

.result-status-icon {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 6px;
  flex: none;
}

.result-status-icon.tone-loading {
  background: color-mix(in oklch, var(--accent) 18%, transparent);
  color: var(--accent);
}

.result-status-icon.tone-success {
  background: color-mix(in oklch, var(--success) 18%, transparent);
  color: var(--success-text);
}

.result-status-icon.tone-error {
  background: color-mix(in oklch, var(--danger) 18%, transparent);
  color: var(--danger-text);
}

.result-status-spinner {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 2px solid color-mix(in oklch, var(--accent) 35%, transparent);
  border-top-color: var(--accent);
  animation: result-status-spin 0.8s linear infinite;
}

@keyframes result-status-spin {
  to {
    transform: rotate(360deg);
  }
}

.result-status-title {
  font-size: 20px;
  font-weight: 800;
  color: var(--foreground);
  line-height: 1.3;
}

.result-status-desc {
  font-size: 13.5px;
  color: var(--muted);
  line-height: 1.6;
}
</style>
