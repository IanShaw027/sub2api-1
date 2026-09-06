<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import { useCreationStore } from '../stores/creation'
import type { CreationSessionMode } from '../types'

const { t } = useI18n()
const store = useCreationStore()

const modeOptions = computed(() => [
  { value: 'chat' as CreationSessionMode, label: t('studio.modes.chat') },
  ...(store.hasImageModels || store.sessions.some((session) => session.mode === 'image')
    ? [{ value: 'image' as CreationSessionMode, label: t('studio.modes.image') }]
    : []),
])

async function switchMode(mode: CreationSessionMode) {
  store.sessionModeFilter = mode
  const next = store.sessions.find((session) => session.mode === mode)
  if (next) {
    await store.selectSession(next.id)
    return
  }
  await store.createSession(mode)
}

async function createForMode() {
  await store.createSession(store.sessionModeFilter)
}

async function selectSession(id: number) {
  await store.selectSession(id)
}

async function removeSession(id: number) {
  await store.deleteSession(id)
  if (store.selectedSessionId !== null) return
  if (store.visibleSessions.length > 0) {
    await store.selectSession(store.visibleSessions[0].id)
  } else {
    await store.createSession(store.sessionModeFilter)
  }
}
</script>

<template>
  <GlassCard class="studio-session-list" padding="sm">
    <div class="studio-session-list-header">
      <h2 class="text-sm font-semibold text-foreground">{{ t('studio.sessions') }}</h2>
      <SegmentedControl
        :model-value="store.sessionModeFilter"
        :options="modeOptions"
        @update:model-value="switchMode"
      />
    </div>

    <div class="studio-session-actions">
      <Button variant="secondary" size="sm" @click="createForMode">
        {{ store.sessionModeFilter === 'image' ? t('studio.newImage') : t('studio.newChat') }}
      </Button>
    </div>

    <div v-if="store.sessionsLoading" class="px-2 py-3 text-xs text-muted">
      {{ t('common.loading') }}
    </div>

    <div v-else-if="store.visibleSessions.length === 0" class="px-2 py-3 text-xs text-muted">
      {{ t('studio.emptySessions') }}
    </div>

    <ul v-else class="studio-session-items">
      <li
        v-for="session in store.visibleSessions"
        :key="session.id"
        class="studio-session-item"
        :class="{ 'studio-session-item-active': session.id === store.selectedSessionId }"
      >
        <button type="button" class="studio-session-button" @click="selectSession(session.id)">
          <span class="studio-session-title">{{ session.title || t('studio.newChat') }}</span>
          <span class="studio-session-meta text-muted">{{ session.model }}</span>
        </button>
        <button
          type="button"
          class="studio-session-delete text-muted"
          :title="t('studio.deleteSession')"
          @click="removeSession(session.id)"
        >
          ×
        </button>
      </li>
    </ul>
  </GlassCard>
</template>

<style scoped>
.studio-session-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
}

.studio-session-list-header {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.studio-session-actions {
  display: flex;
}

.studio-session-items {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
  overflow: auto;
  max-height: min(52vh, 560px);
}

.studio-session-item {
  display: flex;
  align-items: center;
  gap: 4px;
  border: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
  border-radius: 12px;
  background: color-mix(in oklch, var(--surface) 80%, transparent);
}

.studio-session-item-active {
  border-color: color-mix(in oklch, var(--accent) 45%, var(--border));
  background: color-mix(in oklch, var(--accent) 8%, var(--surface));
}

.studio-session-button {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 10px 12px;
  text-align: left;
  background: transparent;
  border: none;
  color: var(--foreground);
}

.studio-session-title {
  font-size: 13px;
  font-weight: 600;
}

.studio-session-meta {
  font-size: 11px;
}

.studio-session-delete {
  width: 32px;
  height: 32px;
  border: none;
  background: transparent;
  font-size: 18px;
  line-height: 1;
}

@media (max-width: 767px) {
  .studio-session-items {
    max-height: 180px;
  }
}
</style>
