<template>
  <Teleport to="body">
    <Transition name="cmdk">
      <div v-if="open" class="cmdk-overlay" role="presentation" @click.self="close">
        <div
          ref="panelRef"
          class="cmdk-panel glass-card-solid"
          role="dialog"
          aria-modal="true"
          :aria-label="t('nav.search')"
          @keydown="onKeydown"
        >
          <div class="cmdk-search">
            <Icon name="search" size="sm" class="cmdk-search-icon" />
            <input
              ref="inputRef"
              v-model="query"
              type="text"
              class="cmdk-input"
              :placeholder="t('nav.searchPlaceholder')"
              autocomplete="off"
              spellcheck="false"
            />
            <span class="kbd">Esc</span>
          </div>
          <div class="cmdk-list" role="listbox">
            <template v-if="results.length">
              <p class="dropdown-label">{{ t('nav.pages') }}</p>
              <button
                v-for="(item, index) in results"
                :key="item.path"
                type="button"
                role="option"
                class="cmdk-item"
                :class="{ 'is-active': index === activeIndex }"
                :aria-selected="index === activeIndex"
                @mouseenter="activeIndex = index"
                @click="go(item)"
              >
                <span class="cmdk-item-icon"><Icon name="grid" size="sm" /></span>
                <span class="cmdk-item-label">{{ item.label }}</span>
                <span class="cmdk-item-path">{{ item.path }}</span>
              </button>
            </template>
            <p v-else class="cmdk-empty">{{ t('nav.searchEmpty') }}</p>
          </div>
          <div class="cmdk-footer">{{ t('nav.searchHint') }}</div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useAuthStore } from '@/stores/auth'
import { acquireOverlayLock, releaseOverlayLock } from '@/components/ui/overlayLock'

interface PaletteItem {
  path: string
  label: string
}

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{ close: [] }>()

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const query = ref('')
const activeIndex = ref(0)
const inputRef = ref<HTMLInputElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)

const items = computed<PaletteItem[]>(() => {
  const isAdmin = authStore.isAdmin
  const out: PaletteItem[] = []
  for (const route of router.getRoutes()) {
    const meta = route.meta as Record<string, unknown>
    const titleKey = meta.titleKey as string | undefined
    if (!titleKey || !route.path || route.path.includes(':')) continue
    if (meta.requiresAuth === false) continue
    const admin = route.path.startsWith('/admin')
    if (admin && !isAdmin) continue
    if (route.path === '/setup' || route.path.startsWith('/payment/') || route.path.startsWith('/auth/')) continue
    const label = t(titleKey)
    if (!label || label === titleKey) continue
    out.push({ path: route.path, label })
  }
  return out
})

const results = computed(() => {
  const q = query.value.trim().toLowerCase()
  const list = q
    ? items.value.filter((i) => i.label.toLowerCase().includes(q) || i.path.toLowerCase().includes(q))
    : items.value
  return list.slice(0, 12)
})

function close() {
  emit('close')
}

function go(item: PaletteItem) {
  close()
  void router.push(item.path)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
    return
  }
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    activeIndex.value = Math.min(results.value.length - 1, activeIndex.value + 1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    activeIndex.value = Math.max(0, activeIndex.value - 1)
    return
  }
  if (event.key === 'Enter') {
    const item = results.value[activeIndex.value]
    if (item) {
      event.preventDefault()
      go(item)
    }
  }
}

function onGlobalKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k') {
    event.preventDefault()
    if (props.open) close()
    else emit('close') // no-op; parent toggles via `open`
  }
}

watch(
  () => props.open,
  async (open) => {
    if (open) {
      query.value = ''
      activeIndex.value = 0
      acquireOverlayLock()
      await nextTick()
      inputRef.value?.focus()
    } else {
      releaseOverlayLock()
    }
  }
)

watch(results, () => {
  activeIndex.value = 0
})

onMounted(() => {
  window.addEventListener('keydown', onGlobalKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onGlobalKeydown)
  if (props.open) releaseOverlayLock()
})
</script>

<style scoped>
.cmdk-overlay {
  position: fixed;
  inset: 0;
  z-index: 70;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 12vh 16px 16px;
  background: var(--scrim);
  backdrop-filter: blur(2px);
  -webkit-backdrop-filter: blur(2px);
}

.cmdk-panel {
  width: 100%;
  max-width: 560px;
  border-radius: var(--radius-hero);
  overflow: hidden;
  box-shadow: var(--shadow-pop);
}

.cmdk-search {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 44px;
  padding: 0 14px;
  border-bottom: 1px solid var(--border);
}

.cmdk-search-icon {
  color: var(--muted);
  flex: none;
}

.cmdk-input {
  flex: 1;
  min-width: 0;
  height: 100%;
  border: 0;
  outline: none;
  background: transparent;
  font-size: 14px;
  color: var(--foreground);
}

.cmdk-input::placeholder {
  color: var(--muted);
}

.cmdk-list {
  max-height: 360px;
  overflow-y: auto;
  padding: 6px;
}

.cmdk-item {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  height: 40px;
  padding: 0 10px;
  border-radius: 9px;
  border: 0;
  background: transparent;
  color: var(--foreground);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  text-align: left;
}

.cmdk-item.is-active {
  background: color-mix(in oklch, var(--accent) 10%, transparent);
  color: var(--accent);
}

.cmdk-item-icon {
  display: inline-flex;
  color: var(--muted);
}

.cmdk-item.is-active .cmdk-item-icon {
  color: var(--accent);
}

.cmdk-item-label {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cmdk-item-path {
  font-family: var(--font-mono);
  font-size: 11px;
  color: var(--muted);
}

.cmdk-empty {
  padding: 28px 12px;
  text-align: center;
  font-size: 13px;
  color: var(--muted);
}

.cmdk-footer {
  padding: 8px 14px;
  border-top: 1px solid var(--border);
  font-size: 11.5px;
  color: var(--muted);
}

.cmdk-enter-active,
.cmdk-leave-active {
  transition: opacity 0.15s ease;
}

.cmdk-enter-active .cmdk-panel,
.cmdk-leave-active .cmdk-panel {
  transition: transform 0.15s ease, opacity 0.15s ease;
}

.cmdk-enter-from,
.cmdk-leave-to {
  opacity: 0;
}

.cmdk-enter-from .cmdk-panel,
.cmdk-leave-to .cmdk-panel {
  transform: translateY(-6px) scale(0.98);
  opacity: 0;
}
</style>
