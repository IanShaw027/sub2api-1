<script setup lang="ts">
import { computed, ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { Download, Ellipsis, GitBranch, History, Pencil, SquarePen, Trash2 } from '@lucide/vue'
import type { LocalChatSession } from '../localChat'
import { localChatMessages } from './localChatMessages'

const props = defineProps<{ sessions: LocalChatSession[]; selectedId: string | null; search?: string; loading?: boolean }>()
const emit = defineEmits<{ action: [action: string, session?: LocalChatSession] }>()
const { t, locale } = useI18n({ useScope: 'local', messages: localChatMessages })
const menu = ref<string | null>(null)
const root = ref<HTMLElement | null>(null)
onClickOutside(root, () => { menu.value = null })
const filtered = computed(() => {
  const query = props.search?.trim().toLowerCase()
  return props.sessions.filter(session => !query || session.title.toLowerCase().includes(query) || session.messages.some(message => message.content.toLowerCase().includes(query)))
})
function action(name: string, session?: LocalChatSession) { menu.value = null; emit('action', name, session) }
function date(value: string) { return new Date(value).toLocaleDateString(locale.value, { month: 'short', day: 'numeric' }) }
</script>

<template>
  <div ref="root" class="local-chat-sessions" @keydown.esc="menu = null">
    <button type="button" class="btn btn-secondary local-chat-new" :disabled="loading" @click="action('new')"><SquarePen :size="16" />{{ t('newChat') }}</button>
    <div v-if="loading" class="local-chat-session-empty" role="status">{{ t('loading') }}</div>
    <div v-else-if="!filtered.length" class="local-chat-session-empty">{{ t('emptyHistory') }}</div>
    <ul v-else class="local-chat-session-list">
      <li v-for="session in filtered" :key="session.id" :class="{ active: session.id === selectedId }">
        <button type="button" class="local-chat-session-select" :aria-current="session.id === selectedId ? 'page' : undefined" @click="action('select', session)"><span class="local-chat-session-title"><GitBranch v-if="session.branchOf" :size="13" :aria-label="t('branch')" />{{ session.title }}</span><span class="local-chat-session-meta">{{ date(session.updatedAt) }}<span>{{ session.model }}</span></span></button>
        <button type="button" class="local-chat-session-more" :aria-label="t('more')" :title="t('more')" :aria-expanded="menu === session.id" @click="menu = menu === session.id ? null : session.id"><Ellipsis :size="16" /></button>
        <div v-if="menu === session.id" class="local-chat-session-menu" @keydown.esc="menu = null"><button type="button" @click="action('rename', session)"><Pencil :size="14" />{{ t('rename') }}</button><button type="button" @click="action('download', session)"><Download :size="14" />{{ t('downloadChat') }}</button><button type="button" class="danger" @click="action('delete', session)"><Trash2 :size="14" />{{ t('remove') }}</button></div>
      </li>
    </ul>
    <button type="button" class="btn btn-ghost local-chat-legacy" @click="action('legacy')"><History :size="15" />{{ t('legacy') }}</button>
  </div>
</template>

<style scoped>
.local-chat-sessions { display: flex; flex: 1; flex-direction: column; min-height: 0; gap: 14px; }
.local-chat-new { width: 100%; flex-shrink: 0; }
.local-chat-session-empty { padding: 12px; font-size: 12px; color: var(--muted); }
.local-chat-session-list { list-style: none; display: flex; flex-direction: column; gap: 5px; flex: 1; min-height: 0; overflow: auto; padding: 0; margin: 0; }
.local-chat-session-list li { position: relative; display: flex; flex-wrap: wrap; align-items: center; border-radius: var(--radius-btn); }
.local-chat-session-list li:hover { background: color-mix(in oklch, var(--foreground) 5%, transparent); }
.local-chat-session-list li.active { background: color-mix(in oklch, var(--accent) 10%, transparent); }
.local-chat-session-list li.active .local-chat-session-title { color: var(--accent); font-weight: var(--fw-semibold); }
.local-chat-session-select { flex: 1; min-width: 0; padding: 10px 8px; background: none; border: 0; text-align: left; color: var(--foreground); }
.local-chat-session-title { display: flex; align-items: center; gap: 5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; }
.local-chat-session-title svg { flex-shrink: 0; }
.local-chat-session-meta { display: flex; align-items: center; gap: 8px; margin-top: 5px; color: var(--muted); font-size: 10px; }
.local-chat-session-meta > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.local-chat-session-more { display: grid; place-items: center; width: 28px; height: 30px; padding: 4px; border: 0; color: var(--muted); background: none; flex-shrink: 0; }
.local-chat-session-menu { width: calc(100% - 8px); margin: 0 4px 5px; border-top: 1px solid var(--border); padding: 5px 0 0; }
.local-chat-session-menu button { display: flex; align-items: center; gap: 8px; width: 100%; padding: 8px; border: 0; border-radius: 4px; background: none; color: var(--foreground); font-size: 12px; text-align: left; }
.local-chat-session-menu button:hover { background: var(--surface-secondary); }
.local-chat-session-menu .danger { color: var(--danger-text); }
.local-chat-legacy { width: 100%; flex-shrink: 0; font-size: var(--fs-12); }
</style>
