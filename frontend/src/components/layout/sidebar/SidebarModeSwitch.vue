<template>
  <div class="sidebar-mode-switch" :class="{ 'is-collapsed': collapsed }" role="group" :aria-label="t('nav.navigationMode')">
    <button
      v-for="item in modes"
      :key="item.value"
      type="button"
      :aria-pressed="modelValue === item.value"
      :aria-label="item.label"
      :title="item.label"
      @click="$emit('update:modelValue', item.value)"
    >
      <component :is="item.icon" :size="15" aria-hidden="true" />
      <span v-if="!collapsed">{{ item.label }}</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { ShieldCheck, UserRound } from '@lucide/vue'

defineProps<{ modelValue: 'admin' | 'user'; collapsed?: boolean }>()
defineEmits<{ 'update:modelValue': [value: 'admin' | 'user'] }>()
const { t } = useI18n()
const modes = computed(() => [
  { value: 'admin' as const, label: t('nav.adminView'), icon: ShieldCheck },
  { value: 'user' as const, label: t('nav.userView'), icon: UserRound }
])
</script>

<style scoped>
.sidebar-mode-switch {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 2px;
  flex: none;
  padding: 3px;
  margin: 0 6px 8px;
  border-radius: 8px;
  background: color-mix(in oklch, var(--foreground) 5%, transparent);
}
.sidebar-mode-switch button {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-width: 0;
  height: 32px;
  border: 0;
  border-radius: 6px;
  background: transparent;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background 150ms ease, color 150ms ease;
}
.sidebar-mode-switch button[aria-pressed='true'] {
  background: var(--surface);
  color: var(--info-text);
}
.sidebar-mode-switch button:hover {
  color: var(--foreground);
}
.sidebar-mode-switch.is-collapsed {
  grid-template-columns: 1fr;
  margin-inline: 2px;
}
</style>
