<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'
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
  const sameMode = store.sessions.filter((session) => session.mode === mode)
  // Prefer a session that already belongs to the active group so switching modes never
  // silently swaps the selected group (selectSession adopts the session's group_id).
  const next = sameMode.find((session) => session.group_id === store.groupId) ?? sameMode[0]
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
  if (store.visibleSessions.length > 0) {
    await store.selectSession(store.visibleSessions[0].id)
  } else {
    await store.createSession(store.sessionModeFilter)
  }
}
</script>

<template>
  <div class="glass-card studio-session-list">
    <div class="studio-session-list-header">
      <h2 class="text-sm font-semibold text-foreground">{{ t('studio.sessions') }}</h2>
      <SegmentedControl
        :model-value="store.sessionModeFilter"
        :options="modeOptions"
        @update:model-value="switchMode"
      />
    </div>

    <div class="studio-session-actions">
      <Button variant="secondary" @click="createForMode">
        {{ store.sessionModeFilter === 'image' ? t('studio.newImage') : t('studio.newChat') }}
      </Button>
    </div>

    <div
      v-if="store.sessionsLoading"
      class="px-2 py-3 text-xs text-muted"
      role="status"
      aria-live="polite"
      aria-busy="true"
      :aria-label="t('studio.a11y.loadingSessions')"
    >
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
          :aria-label="t('studio.deleteSession')"
          @click="removeSession(session.id)"
        >
          ×
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.studio-session-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 0;
  padding: 12px;
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

/* Desktop 3-column layout: match the main/controls columns' locked height
   (viewport minus header + page padding) and scroll the session list
   internally instead of clamping to a vh fraction. */
@media (min-width: 1101px) {
  .studio-session-list {
    height: calc(100vh - 146px - 24px);
    overflow-y: auto;
  }

  .studio-session-items {
    flex: 1;
    min-height: 0;
    max-height: none;
  }
}

.studio-session-item {
  display: flex;
  align-items: center;
  gap: 2px;
  border-radius: 9px;
  transition: background 0.15s ease;
}

.studio-session-item:hover {
  background: color-mix(in oklch, var(--foreground) 5%, transparent);
}

.studio-session-item-active,
.studio-session-item-active:hover {
  background: var(--surface-secondary);
}

.studio-session-button {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 1px;
  min-height: 36px;
  padding: 4px 10px;
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 9px;
  color: var(--foreground);
}

.studio-session-title {
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.studio-session-item-active .studio-session-title {
  color: var(--accent);
  font-weight: 600;
}

.studio-session-meta {
  font-size: 11px;
  font-family: var(--font-mono);
}

.studio-session-delete {
  width: 28px;
  height: 28px;
  flex: none;
  border: none;
  border-radius: 8px;
  background: transparent;
  font-size: 16px;
  line-height: 1;
}

.studio-session-delete:hover {
  background: color-mix(in oklch, var(--foreground) 8%, transparent);
  color: var(--foreground);
}

@media (max-width: 767px) {
  .studio-session-items {
    max-height: 180px;
  }
}
</style>
