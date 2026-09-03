<template>
 <Teleport to="body">
 <transition name="mobile-drawer-overlay">
 <div
 v-if="mobileOpen"
 class="mobile-drawer-overlay md:hidden"
 @click="close"
 />
 </transition>

 <transition name="mobile-drawer-panel">
 <div
 v-if="mobileOpen"
 id="mobile-drawer"
 ref="drawerPanelRef"
 class="mobile-drawer md:hidden"
 role="dialog"
 aria-modal="true"
 :aria-label="t('common.toggleMenu')"
 >
 <div class="mobile-drawer-header">
 <div
 class="flex h-8 w-8 flex-none items-center justify-center overflow-hidden rounded-full text-xs font-bold"
 style="background: color-mix(in oklch, var(--accent) 18%, transparent); color: var(--accent)"
 >
 <img
 v-if="avatarUrl"
 :src="avatarUrl"
 :alt="displayName"
 class="h-full w-full object-cover"
 >
 <span v-else>{{ userInitial }}</span>
 </div>
 <div class="flex min-w-0 flex-1 flex-col leading-tight">
 <span class="truncate text-[13px] font-semibold text-[var(--foreground)]">{{ user?.email }}</span>
 <span
 class="text-[11px] font-semibold"
 :class="isAdmin ? 'text-[var(--accent)]' : 'text-[var(--muted)]'"
 >{{ roleLabel }}</span>
 </div>
 <button
 ref="closeBtnRef"
 type="button"
 class="mobile-drawer-close"
 :aria-label="t('common.closeMenu')"
 @click="close"
 >
 <Icon name="x" size="sm" />
 </button>
 </div>

 <nav class="mobile-drawer-nav scrollbar-hide">
 <SidebarNavContent
 :sections="sections"
 :collapsed="false"
 :route-path="routePath"
 :is-section-open="isSectionOpen"
 :is-item-active="isItemActive"
 :is-group-active="isGroupActive"
 :is-group-expanded="isGroupExpanded"
 :group-badge="groupBadge"
 :omit-tour-anchors="!tourActive"
 @toggle="$emit('toggle', $event)"
 @navigate="$emit('navigate', $event)"
 @group-click="$emit('group-click', $event)"
 />
 </nav>

 <div class="mobile-drawer-footer">
 <div class="mobile-drawer-footer-row">
 <router-link
 to="/profile"
 class="mobile-drawer-pill"
 @click="close"
 >
 <Icon name="user" size="sm" />
 <span>{{ t('nav.profile') }}</span>
 </router-link>
 <router-link
 to="/keys"
 class="mobile-drawer-pill"
 @click="close"
 >
 <Icon name="key" size="sm" />
 <span>{{ t('nav.apiKeys') }}</span>
 </router-link>
 </div>
 <button
 v-if="showOnboardingButton"
 type="button"
 class="mobile-drawer-pill"
 @click="handleReplayGuide"
 >
 <Icon name="refresh" size="sm" />
 <span>{{ t('onboarding.restartTour') }}</span>
 </button>
 <button
 type="button"
 class="mobile-drawer-pill mobile-drawer-logout"
 :aria-label="t('nav.logout')"
 @click="handleLogout"
 >
 <Icon name="login" size="sm" />
 <span>{{ t('nav.logout') }}</span>
 </button>
 <div class="mobile-drawer-footer-row">
 <button
 type="button"
 class="mobile-drawer-pill"
 :title="isDark ? t('nav.lightMode') : t('nav.darkMode')"
 :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
 @click="toggleTheme"
 >
 <Icon v-if="isDark" name="sun" size="sm" />
 <Icon v-else name="moon" size="sm" />
 <span>{{ isDark ? t('nav.lightMode') : t('nav.darkMode') }}</span>
 </button>
 <button
 type="button"
 class="mobile-drawer-pill"
 :title="localeActionLabel"
 :aria-label="localeActionLabel"
 :disabled="switchingLocale"
 @click="cycleLocale"
 >
 {{ localeCode }}
 </button>
 </div>
 </div>
 </div>
 </transition>
 </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingStore } from '@/stores/onboarding'
import { useTheme } from '@/composables/useTheme'
import { useIsMobile } from '@/composables/useIsMobile'
import { getLocale, setLocale } from '@/i18n'
import Icon from '@/components/icons/Icon.vue'
import SidebarNavContent from './sidebar/SidebarNavContent.vue'
import type { NavItem, NavSection } from './sidebar/navSections'
import { acquireOverlayLock, releaseOverlayLock } from '@/components/ui/overlayLock'

const FOCUSABLE_SELECTOR = 'a[href], button:not([disabled]), textarea, input, select, [tabindex]:not([tabindex="-1"])'

const props = defineProps<{
 sections: NavSection[]
 routePath: string
 isSectionOpen: (section: NavSection) => boolean
 isItemActive: (item: NavItem) => boolean
 isGroupActive: (item: NavItem) => boolean
 isGroupExpanded: (item: NavItem) => boolean
 groupBadge: (item: NavItem) => number
}>()

defineEmits<{
 toggle: [key: string]
 navigate: [path: string]
 'group-click': [item: NavItem]
}>()

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const onboardingStore = useOnboardingStore()
const { isDark, toggleTheme } = useTheme()
const { isTabletUp } = useIsMobile()

const mobileOpen = computed(() => appStore.mobileOpen)
const user = computed(() => authStore.user)
const isAdmin = computed(() => authStore.isAdmin)
const tourActive = computed(() => onboardingStore.isDriverActive())
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const switchingLocale = ref(false)
const localeTick = ref(0)
const closeBtnRef = ref<HTMLButtonElement | null>(null)
const drawerPanelRef = ref<HTMLElement | null>(null)

const userInitial = computed(() => {
 if (!user.value) return ''
 if (user.value.username) return user.value.username.charAt(0).toUpperCase()
 if (user.value.email) return user.value.email.charAt(0).toUpperCase()
 return ''
})

const displayName = computed(() => {
 if (!user.value) return ''
 return user.value.username || user.value.email?.split('@')[0] || ''
})

const roleLabel = computed(() => {
 if (!user.value) return ''
 if (isAdmin.value) return t('admin.users.roles.admin')
 return t('admin.users.roles.user')
})

const localeCode = computed(() => {
 void localeTick.value
 return getLocale() === 'zh' ? 'ZH' : 'EN'
})

const localeActionLabel = computed(() => {
 void localeTick.value
 const next = getLocale() === 'zh' ? 'English' : '中文'
 return t('common.switchToLocale', { locale: next })
})

const showOnboardingButton = computed(() => {
 return !authStore.isSimpleMode && user.value?.role === 'admin'
})

let previousFocus: HTMLElement | null = null

function close() {
 appStore.setMobileOpen(false)
}

async function handleLogout() {
 close()
 try {
 await authStore.logout()
 } catch (error) {
 console.error('Logout error:', error)
 }
 await router.push('/login')
}

function handleReplayGuide() {
 onboardingStore.replay()
}

function lockBody(lock: boolean) {
 if (lock) {
 acquireOverlayLock()
 return
 }
 releaseOverlayLock()
}

function trapFocus(event: KeyboardEvent) {
 const root = drawerPanelRef.value
 if (!root) return
 const nodes = Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
 (el) => !el.hasAttribute('disabled')
 )
 if (nodes.length === 0) return
 const first = nodes[0]
 const last = nodes[nodes.length - 1]
 if (event.shiftKey && document.activeElement === first) {
 event.preventDefault()
 last.focus()
 } else if (!event.shiftKey && document.activeElement === last) {
 event.preventDefault()
 first.focus()
 }
}

function onKeydown(event: KeyboardEvent) {
 if (!mobileOpen.value) return
 if (tourActive.value) return
 if (event.key === 'Escape') {
 close()
 return
 }
 if (event.key === 'Tab') trapFocus(event)
}

async function cycleLocale() {
 if (switchingLocale.value) return
 switchingLocale.value = true
 try {
 await setLocale(getLocale() === 'en' ? 'zh' : 'en')
 localeTick.value += 1
 } finally {
 switchingLocale.value = false
 }
}

watch(
 mobileOpen,
 async (open, wasOpen) => {
 if (open) {
 previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
 lockBody(true)
 if (typeof window !== 'undefined') {
 window.addEventListener('keydown', onKeydown)
 }
 if (!tourActive.value) {
 await nextTick()
 closeBtnRef.value?.focus()
 }
 return
 }
 if (wasOpen === true) {
 lockBody(false)
 if (typeof window !== 'undefined') {
 window.removeEventListener('keydown', onKeydown)
 }
 previousFocus?.focus?.()
 previousFocus = null
 }
 },
 { immediate: true }
)

watch(isTabletUp, (up) => {
 if (up && mobileOpen.value) {
 close()
 }
})

watch(
 () => props.routePath,
 () => {
 if (mobileOpen.value && !tourActive.value) close()
 }
)

onBeforeUnmount(() => {
 window.removeEventListener('keydown', onKeydown)
 if (mobileOpen.value) {
 lockBody(false)
 }
})
</script>

<style scoped>
.mobile-drawer-overlay {
 position: fixed;
 inset: 0;
 z-index: 45;
 background: rgba(0, 0, 0, 0.28);
 backdrop-filter: blur(2px);
 -webkit-backdrop-filter: blur(2px);
}

.mobile-drawer {
 position: fixed;
 top: calc(56px + env(safe-area-inset-top, 0px));
 right: 0;
 bottom: 0;
 z-index: 50;
 display: flex;
 flex-direction: column;
 width: 300px;
 box-sizing: border-box;
 padding: 14px 14px 24px;
 background: color-mix(in oklch, var(--background) 88%, transparent);
 backdrop-filter: blur(28px);
 -webkit-backdrop-filter: blur(28px);
 border-left: 1px solid var(--border);
 box-shadow: -30px 0 60px -30px rgba(0, 0, 0, 0.5);
 color: var(--foreground);
}

.mobile-drawer-header {
 display: flex;
 align-items: center;
 gap: 10px;
 margin-bottom: 16px;
 flex: none;
}

.mobile-drawer-close {
 display: inline-flex;
 align-items: center;
 justify-content: center;
 width: 36px;
 height: 36px;
 flex: none;
 border-radius: 10px;
 color: var(--muted);
 background: transparent;
 border: 1px solid transparent;
 transition: background 0.15s ease, color 0.15s ease;
}

.mobile-drawer-close:hover {
 background: color-mix(in oklch, var(--foreground) 6%, transparent);
 color: var(--foreground);
}

.mobile-drawer-nav {
 display: flex;
 flex-direction: column;
 gap: 1px;
 flex: 1;
 min-height: 0;
 overflow-y: auto;
 scrollbar-width: none;
}

.mobile-drawer-footer {
 display: flex;
 flex-direction: column;
 gap: 8px;
 flex: none;
 margin-top: 16px;
}

.mobile-drawer-footer-row {
 display: flex;
 gap: 8px;
}

.mobile-drawer-pill {
 display: inline-flex;
 align-items: center;
 justify-content: center;
 gap: 8px;
 height: 40px;
 min-height: 40px;
 flex: none;
 border-radius: 12px;
 font-size: 13px;
 font-weight: 600;
 color: var(--foreground);
 background: color-mix(in oklch, var(--foreground) 6%, transparent);
 border: 1px solid var(--border);
}

.mobile-drawer-footer-row > .mobile-drawer-pill {
 flex: 1;
}

.mobile-drawer-logout {
 color: var(--danger-text);
}

.mobile-drawer-overlay-enter-active,
.mobile-drawer-overlay-leave-active {
 transition: opacity 0.2s ease;
}

.mobile-drawer-overlay-enter-from,
.mobile-drawer-overlay-leave-to {
 opacity: 0;
}

.mobile-drawer-panel-enter-active,
.mobile-drawer-panel-leave-active {
 transition: transform 0.25s ease;
}

.mobile-drawer-panel-enter-from,
.mobile-drawer-panel-leave-to {
 transform: translateX(100%);
}
</style>
