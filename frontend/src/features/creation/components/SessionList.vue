<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'
import SegmentedControl from '@/components/ui/SegmentedControl.vue'
import Icon from '@/components/icons/Icon.vue'
import { useCreationStore } from '../stores/creation'
import type { CreationSessionMode } from '../types'

const { t } = useI18n()
const store = useCreationStore()
const props = withDefaults(defineProps<{
  mode?: CreationSessionMode
  search?: string
  embedded?: boolean
}>(), { search: '', embedded: false })
const emit = defineEmits<{ navigated: [] }>()
const activeMode = computed(() => props.mode ?? store.sessionModeFilter)
const modeSessions = computed(() => props.mode
  ? store.sessions.filter(session => session.mode === props.mode)
  : store.visibleSessions,
)
const filteredSessions = computed(() => {
  const query = props.search.trim().toLocaleLowerCase()
  return query ? modeSessions.value.filter(session => `${session.title} ${session.model}`.toLocaleLowerCase().includes(query)) : modeSessions.value
})

const modeOptions = computed(() => [
  { value: 'chat' as CreationSessionMode, label: t('studio.modes.chat') },
  ...(store.hasImageModels || store.sessions.some((session) => session.mode === 'image')
    ? [{ value: 'image' as CreationSessionMode, label: t('studio.modes.image') }]
    : []),
])

async function handleNavigation(action: () => Promise<unknown>) {
  try {
    await action()
  } catch {
    // The store exposes request errors and discards superseded navigation.
  }
}

async function switchMode(mode: CreationSessionMode) {
  await handleNavigation(async () => {
    store.sessionModeFilter = mode
    const sameMode = store.sessions.filter((session) => session.mode === mode)
    // Prefer the active group because selecting a session adopts its group.
    const next = sameMode.find((session) => session.group_id === store.groupId) ?? sameMode[0]
    if (next) {
      await store.selectSession(next.id)
      return
    }
    await store.createSession(mode)
  })
}

async function createForMode() {
  await handleNavigation(async () => {
    await store.createSession(activeMode.value)
    emit('navigated')
  })
}

async function selectSession(id: number) {
  await handleNavigation(async () => {
    await store.selectSession(id)
    emit('navigated')
  })
}

async function removeSession(id: number) {
  await handleNavigation(async () => {
    await store.deleteSession(id)
    if (store.selectedSessionId !== null) return
    if (modeSessions.value.length > 0) {
      await store.selectSession(modeSessions.value[0].id)
    } else {
      await store.createSession(activeMode.value)
    }
  })
}
</script>

<template>
  <div class="studio-session-list" :class="embedded ? 'studio-session-list-embedded' : 'glass-card'">
    <div class="studio-session-list-header">
      <h2 class="text-sm font-semibold text-foreground">{{ t('studio.sessions') }}</h2>
      <SegmentedControl
        v-if="!mode"
        :model-value="store.sessionModeFilter"
        :options="modeOptions"
        @update:model-value="switchMode"
      />
    </div>

    <div class="studio-session-actions">
      <Button variant="secondary" :disabled="store.sessionLoading || !store.groupId" @click="createForMode">
        <Icon name="plus" size="sm" />
        {{ activeMode === 'image' ? t('studio.newImage') : t('studio.newChat') }}
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

    <div v-else-if="filteredSessions.length === 0" class="px-2 py-3 text-xs text-muted">
      {{ t('studio.emptySessions') }}
    </div>

    <ul v-else class="studio-session-items">
      <li
        v-for="session in filteredSessions"
        :key="session.id"
        class="studio-session-item"
        :class="{ 'studio-session-item-active': session.id === store.selectedSessionId }"
      >
        <button type="button" class="studio-session-button" :aria-current="session.id === store.selectedSessionId ? 'page' : undefined" @click="selectSession(session.id)">
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
          <Icon name="trash" size="sm" />
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
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.studio-session-list.studio-session-list-embedded {
  padding: 0;
  height: 100%;
  overflow: hidden;
}

.studio-session-list-embedded .studio-session-items {
  flex: 1;
  min-height: 0;
  max-height: none;
}

.studio-session-list-embedded .studio-session-actions > * {
  width: 100%;
}

.studio-session-list-embedded .studio-session-button {
  min-height: 48px;
}

.studio-session-list-embedded .studio-session-delete {
  width: 36px;
  height: 44px;
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
