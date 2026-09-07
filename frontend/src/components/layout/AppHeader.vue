<template>
  <header ref="headerRef" class="app-header" :class="{ 'is-compact': compactHeader }" :style="{ '--header-scroll-progress': scrollProgress }">
    <!-- ===== Mobile top bar (<768) — design 08 ===== -->
    <div class="topbar-mobile">
      <router-link :to="homePath" class="brand-mark" :aria-label="siteName">
        <img v-if="settingsLoaded" :src="siteLogo || '/logo.svg'" alt="" />
      </router-link>
      <div class="topbar-mobile-text">
        <span class="topbar-mobile-title">{{ pageTitle }}</span>
        <span class="topbar-mobile-sub">{{ mobileSubtitle }}</span>
      </div>
      <div class="topbar-mobile-actions">
        <div id="topbar-mobile-more"></div>
        <div v-if="user" id="topbar-mobile-bell" class="topbar-mobile-btn"></div>
        <button
          type="button"
          class="topbar-mobile-btn"
          :aria-label="t('common.toggleMenu')"
          :title="t('common.toggleMenu')"
          :aria-expanded="appStore.mobileOpen"
          aria-controls="mobile-drawer"
          @click="toggleMobileSidebar"
        >
          <Icon name="menu" size="sm" :stroke-width="1.9" />
        </button>
      </div>
    </div>

    <!-- ===== Desktop top bar (≥768) — design 03/04/05/06 ===== -->
    <div class="topbar">
      <nav class="topbar-crumbs" aria-label="breadcrumb">
        <router-link v-if="route.path !== breadcrumbRootPath" :to="breadcrumbRootPath" class="topbar-crumb-link">{{ breadcrumbRoot }}</router-link>
        <span v-else>{{ breadcrumbRoot }}</span>
        <template v-for="crumb in parentCrumbs" :key="crumb.path || crumb.label">
          <span class="topbar-crumb-sep" aria-hidden="true">/</span>
          <router-link v-if="crumb.path" :to="crumb.path" class="topbar-crumb-link">{{ crumb.label }}</router-link>
          <span v-else>{{ crumb.label }}</span>
        </template>
        <span class="topbar-crumb-sep" aria-hidden="true">/</span>
        <span class="topbar-crumb-current" aria-current="page">{{ pageTitle }}</span>
      </nav>

      <div class="topbar-actions">
        <!-- Global search (⌘K) -->
        <button
          v-if="user"
          type="button"
          class="topbar-search"
          :aria-label="t('nav.search')"
          :title="t('nav.search')"
          @click="paletteOpen = true"
        >
          <Icon name="search" size="sm" :stroke-width="1.8" />
          <span class="topbar-search-text">{{ t('nav.searchPlaceholder') }}</span>
          <span class="kbd">⌘K</span>
        </button>

        <!-- Balance pill (users) -->
        <div v-if="user && !authStore.isAdmin" class="topbar-balance group relative" tabindex="0">
          <span>{{ t('nav.balance') }}</span>
          <b>{{ formatHeaderMoney(availableBalance) }}</b>
          <span v-if="frozenBalance > 0" class="tag tag-warning">{{ balanceFrozenLabel }}</span>
          <div class="topbar-balance-pop dropdown">
            <div class="topbar-balance-row"><span>{{ balanceAvailableText }}</span><b>{{ formatHeaderMoney(availableBalance) }}</b></div>
            <div class="topbar-balance-row"><span>{{ balanceFrozenText }}</span><b class="text-warning-text">{{ formatHeaderMoney(frozenBalance) }}</b></div>
            <div class="dropdown-divider"></div>
            <div class="topbar-balance-row"><span>{{ balanceTotalText }}</span><b>{{ formatHeaderMoney(totalBalance) }}</b></div>
          </div>
        </div>

        <SubscriptionProgressMini v-if="user" />

        <div v-if="user" class="topbar-bell">
          <Teleport defer to="#topbar-mobile-bell" :disabled="!isMobile">
            <AnnouncementBell />
          </Teleport>
        </div>

        <Teleport defer to="#topbar-mobile-more" :disabled="!isMobile">
        <div ref="moreRef" class="topbar-more" @keydown.esc.stop="closeMore(true)">
          <button
            v-if="compactHeader"
            ref="moreButtonRef"
            type="button"
            class="header-icon-btn"
            :aria-label="t('common.moreActions')"
            :title="t('common.moreActions')"
            :aria-expanded="moreOpen"
            aria-controls="topbar-more-actions"
            @click="toggleMore"
          >
            <Icon name="more" size="sm" />
          </button>
          <div
            id="topbar-more-actions"
            v-show="!compactHeader || moreOpen"
            class="topbar-secondary-actions"
            :class="{ 'dropdown': compactHeader }"
            role="group"
            :aria-label="t('common.moreActions')"
          >
        <a
          v-if="docUrl"
          :href="docUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="header-icon-btn"
          :title="t('nav.docs')"
          :aria-label="t('nav.docs')"
        >
          <Icon name="book" size="sm" :stroke-width="1.8" />
          <span v-if="compactHeader">{{ t('nav.docs') }}</span>
        </a>

        <a
          v-if="downloadToolsUrl"
          :href="downloadToolsUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="header-icon-btn"
          :title="t('common.downloadTools')"
          :aria-label="t('common.downloadTools')"
        >
          <Icon name="download" size="sm" :stroke-width="1.8" />
          <span v-if="compactHeader">{{ t('common.downloadTools') }}</span>
        </a>

        <router-link
          v-if="user && modelPlazaEnabled"
          :to="{ path: '/model-plaza', query: { embedded: '1' } }"
          class="header-icon-btn"
          :title="t('nav.modelPlaza')"
          :aria-label="t('nav.modelPlaza')"
        >
          <Icon name="grid" size="sm" :stroke-width="1.8" />
          <span v-if="compactHeader">{{ t('nav.modelPlaza') }}</span>
        </router-link>

        <SupportQRCodesButton :entries="supportQRCodes" :legacy-contact-info="contactInfo" :menu-mode="compactHeader" @update:open="supportOpen = $event" />

        <LocaleSwitcher :menu-mode="compactHeader" />

        <button
          type="button"
          class="header-icon-btn"
          :title="isDark ? t('nav.lightMode') : t('nav.darkMode')"
          :aria-label="isDark ? t('nav.lightMode') : t('nav.darkMode')"
          @click="toggleTheme"
        >
          <Icon v-if="isDark" name="sun" size="sm" :stroke-width="1.8" />
          <Icon v-else name="moon" size="sm" :stroke-width="1.8" />
          <span v-if="compactHeader">{{ isDark ? t('nav.lightMode') : t('nav.darkMode') }}</span>
        </button>
          </div>
        </div>
        </Teleport>

        <span class="topbar-divider" aria-hidden="true"></span>

        <!-- User pill + menu -->
        <div v-if="user" ref="dropdownRef" class="relative">
          <button ref="userButtonRef" type="button" class="topbar-user" :aria-label="t('common.userMenu')" :aria-expanded="dropdownOpen" aria-controls="topbar-user-actions" @click="toggleDropdown" @keydown.esc.stop="closeDropdown(true)">
            <span class="topbar-avatar avatar-accent">
              <img v-if="avatarUrl" :src="avatarUrl" :alt="displayName" />
              <span v-else>{{ userInitial }}</span>
            </span>
            <span class="topbar-user-name">{{ displayName }}</span>
            <Icon name="chevronDown" size="xs" :stroke-width="2" class="text-muted" />
          </button>

          <transition name="dropdown">
            <div v-if="dropdownOpen" id="topbar-user-actions" class="dropdown topbar-menu" @keydown.esc.stop="closeDropdown(true)">
              <div class="topbar-menu-head">
                <div class="topbar-menu-name">{{ displayName }}</div>
                <div class="topbar-menu-mail">{{ user.email }}</div>
                <span class="tag" :class="authStore.isAdmin ? 'tag-accent' : ''">{{ t('admin.users.roles.' + user.role) }}</span>
              </div>
              <div class="dropdown-divider"></div>
              <router-link to="/profile" class="dropdown-item" @click="closeDropdown">
                <Icon name="user" size="sm" />
                {{ t('nav.profile') }}
              </router-link>
              <router-link to="/keys" class="dropdown-item" @click="closeDropdown">
                <Icon name="key" size="sm" />
                {{ t('nav.apiKeys') }}
              </router-link>
              <a
                v-if="authStore.isAdmin"
                href="https://github.com/Wei-Shaw/sub2api"
                target="_blank"
                rel="noopener noreferrer"
                class="dropdown-item"
                @click="closeDropdown"
              >
                <svg class="h-4 w-4" fill="currentColor" viewBox="0 0 24 24" aria-hidden="true">
                  <path
                    fill-rule="evenodd"
                    clip-rule="evenodd"
                    d="M12 2C6.477 2 2 6.477 2 12c0 4.42 2.865 8.17 6.839 9.49.5.092.682-.217.682-.482 0-.237-.008-.866-.013-1.7-2.782.604-3.369-1.34-3.369-1.34-.454-1.156-1.11-1.464-1.11-1.464-.908-.62.069-.608.069-.608 1.003.07 1.531 1.03 1.531 1.03.892 1.529 2.341 1.087 2.91.831.092-.646.35-1.086.636-1.336-2.22-.253-4.555-1.11-4.555-4.943 0-1.091.39-1.984 1.029-2.683-.103-.253-.446-1.27.098-2.647 0 0 .84-.269 2.75 1.025A9.578 9.578 0 0112 6.836c.85.004 1.705.114 2.504.336 1.909-1.294 2.747-1.025 2.747-1.025.546 1.377.203 2.394.1 2.647.64.699 1.028 1.592 1.028 2.683 0 3.842-2.339 4.687-4.566 4.935.359.309.678.919.678 1.852 0 1.336-.012 2.415-.012 2.743 0 .267.18.578.688.48C19.138 20.167 22 16.418 22 12c0-5.523-4.477-10-10-10z"
                  />
                </svg>
                {{ t('nav.github') }}
              </a>
              <template v-if="contactInfo">
                <div class="dropdown-divider"></div>
                <div class="topbar-menu-contact">
                  <span>{{ t('common.contactSupport') }}</span>
                  <b>{{ contactInfo }}</b>
                </div>
              </template>
              <template v-if="showOnboardingButton">
                <div class="dropdown-divider"></div>
                <button type="button" class="dropdown-item" @click="handleReplayGuide">
                  <Icon name="refresh" size="sm" />
                  {{ $t('onboarding.restartTour') }}
                </button>
              </template>
              <div class="dropdown-divider"></div>
              <button type="button" class="dropdown-item dropdown-item-danger" @click="handleLogout">
                <Icon name="login" size="sm" />
                {{ t('nav.logout') }}
              </button>
            </div>
          </transition>
        </div>
      </div>
    </div>

    <CommandPalette :open="paletteOpen" @close="paletteOpen = false" />
  </header>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore, useAuthStore, useOnboardingStore } from '@/stores'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import SubscriptionProgressMini from '@/components/common/SubscriptionProgressMini.vue'
import AnnouncementBell from '@/components/common/AnnouncementBell.vue'
import Icon from '@/components/icons/Icon.vue'
import SupportQRCodesButton from '@/components/common/SupportQRCodesButton.vue'
import CommandPalette from './CommandPalette.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useTheme } from '@/composables/useTheme'
import { useIsMobile } from '@/composables/useIsMobile'
import { parentBreadcrumbs } from './breadcrumbs'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const adminSettingsStore = useAdminSettingsStore()
const onboardingStore = useOnboardingStore()
const { isDark, toggleTheme } = useTheme()
const { isMobile } = useIsMobile()

const user = computed(() => authStore.user)
const dropdownOpen = ref(false)
const paletteOpen = ref(false)
const dropdownRef = ref<HTMLElement | null>(null)
const headerRef = ref<HTMLElement | null>(null)
const scrollProgress = ref(0)
function updateScrollProgress() {
  scrollProgress.value = Math.min(1, Math.max(0, window.scrollY / 48))
}
const userButtonRef = ref<HTMLButtonElement | null>(null)
const moreRef = ref<HTMLElement | null>(null)
const moreButtonRef = ref<HTMLButtonElement | null>(null)
const moreOpen = ref(false)
const supportOpen = ref(false)
const compactHeader = ref(typeof window !== 'undefined' && window.innerWidth < 1280)
let headerObserver: ResizeObserver | undefined
const contactInfo = computed(() => appStore.contactInfo)
const docUrl = computed(() => sanitizeUrl(appStore.docUrl))
const downloadToolsUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.download_tools_url || ''))
const supportQRCodes = computed(() => appStore.cachedPublicSettings?.support_qr_codes || [])
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const avatarUrl = computed(() => user.value?.avatar_url?.trim() || '')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteName = computed(() => appStore.siteName)
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)
const inAdminWorkspace = computed(() => route.path.startsWith('/admin/') || (
  authStore.isAdmin && route.name === 'CustomPage' &&
  adminSettingsStore.customMenuItems.some(item => item.visibility === 'admin' && item.id === route.params.id)
))
const homePath = computed(() => inAdminWorkspace.value ? '/admin/dashboard' : '/dashboard')
const availableBalance = computed(() => Number(user.value?.balance || 0))
const frozenBalance = computed(() => Number(user.value?.frozen_balance || 0))
const totalBalance = computed(() => availableBalance.value + frozenBalance.value)
const balanceAvailableText = computed(() => t('common.availableBalance') === 'common.availableBalance' ? '可用余额' : t('common.availableBalance'))
const balanceFrozenText = computed(() => t('common.frozenBalance') === 'common.frozenBalance' ? '冻结金额' : t('common.frozenBalance'))
const balanceTotalText = computed(() => t('common.totalBalance') === 'common.totalBalance' ? '总余额' : t('common.totalBalance'))
const balanceFrozenLabel = computed(() => `${balanceFrozenText.value} ${formatHeaderMoney(frozenBalance.value)}`)

// 只在标准模式的管理员下显示新手引导按钮
const showOnboardingButton = computed(() => {
  return !authStore.isSimpleMode && user.value?.role === 'admin'
})

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

const pageTitle = computed(() => {
  // For custom pages, use the menu item's label instead of generic "自定义页面"
  if (route.name === 'CustomPage') {
    const id = route.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) return menuItem.label
  }
  const titleKey = route.meta.titleKey as string
  if (titleKey) {
    return t(titleKey)
  }
  return (route.meta.title as string) || ''
})

const pageDescription = computed(() => {
  const descKey = route.meta.descriptionKey as string
  if (descKey) {
    return t(descKey)
  }
  return (route.meta.description as string) || ''
})

// Intermediate crumbs from nested/parent routes (e.g. 系统设置 / 通用设置)
const parentCrumbs = computed(() => parentBreadcrumbs(router, route, t))

const mobileSubtitle = computed(() => pageDescription.value || user.value?.email || '')

const breadcrumbRoot = computed(() => {
  if (inAdminWorkspace.value) {
    return t('nav.breadcrumbAdmin')
  }
  return t('nav.breadcrumbUser')
})
const breadcrumbRootPath = homePath

function toggleMobileSidebar() {
  appStore.toggleMobileSidebar()
}

async function toggleDropdown() {
  closeMore()
  dropdownOpen.value = !dropdownOpen.value
  if (dropdownOpen.value) {
    await nextTick()
    dropdownRef.value?.querySelector<HTMLElement>('.dropdown-item')?.focus()
  }
}

function closeDropdown(restoreFocus: unknown = false) {
  dropdownOpen.value = false
  if (restoreFocus === true) userButtonRef.value?.focus()
}

async function toggleMore() {
  closeDropdown()
  moreOpen.value = !moreOpen.value
  if (moreOpen.value) {
    await nextTick()
    moreRef.value?.querySelector<HTMLElement>('.topbar-secondary-actions a, .topbar-secondary-actions button')?.focus()
  }
}

function closeMore(restoreFocus = false) {
  moreOpen.value = false
  if (restoreFocus) moreButtonRef.value?.focus()
}

async function handleLogout() {
  closeDropdown()
  try {
    await authStore.logout()
  } catch (error) {
    // Ignore logout errors - still redirect to login
    console.error('Logout error:', error)
  }
  await router.push('/login')
}

function handleReplayGuide() {
  closeDropdown()
  onboardingStore.replay()
}

function formatHeaderMoney(value: number) {
  if (!Number.isFinite(value)) return '$0.00'
  return `$${value.toFixed(2)}`
}

function handleClickOutside(event: MouseEvent) {
  if (dropdownRef.value && !dropdownRef.value.contains(event.target as Node)) {
    closeDropdown()
  }
  // The support dialog is teleported outside More; keep its return-focus target visible.
  if (!supportOpen.value && moreRef.value && !moreRef.value.contains(event.target as Node)) closeMore()
}

function onGlobalKeydown(event: KeyboardEvent) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 'k' && user.value) {
    event.preventDefault()
    paletteOpen.value = !paletteOpen.value
  }
}

onMounted(() => {
  updateScrollProgress()
  window.addEventListener('scroll', updateScrollProgress, { passive: true })
  if (typeof ResizeObserver !== 'undefined' && headerRef.value) {
    headerObserver = new ResizeObserver(([entry]) => {
      if (entry) compactHeader.value = entry.contentRect.width < 1040
    })
    headerObserver.observe(headerRef.value)
  }
  document.addEventListener('click', handleClickOutside)
  window.addEventListener('keydown', onGlobalKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('scroll', updateScrollProgress)
  headerObserver?.disconnect()
  document.removeEventListener('click', handleClickOutside)
  window.removeEventListener('keydown', onGlobalKeydown)
})

watch(() => route.fullPath, () => {
  closeDropdown()
  closeMore()
})

watch(isMobile, () => closeMore())
watch(compactHeader, () => closeMore())
</script>

<style scoped>
.app-header {
  position: sticky;
  top: 0;
  z-index: 30;
  background: transparent;
  isolation: isolate;
}

.app-header::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  pointer-events: none;
  background: var(--surface);
  border-bottom: 1px solid var(--border);
  opacity: var(--header-scroll-progress, 0);
  transition: opacity 120ms ease;
}

/* ---------- Mobile (<768) ---------- */
.topbar-mobile {
  display: flex;
  align-items: center;
  gap: 10px;
  height: 52px;
  padding: 0 16px 0 20px;
  background: color-mix(in oklch, var(--background) 88%, transparent);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
}

.topbar-mobile-text {
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-width: 0;
  flex: 1;
  line-height: 1.1;
}

.topbar-mobile-title {
  font-family: var(--display);
  font-size: 15px;
  font-weight: 800;
  letter-spacing: 0;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-mobile-sub {
  font-size: 11px;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-mobile-actions {
  display: flex;
  gap: 8px;
  flex: none;
}

.topbar-mobile-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  color: var(--foreground);
  box-shadow: inset 0 1px 0 var(--btn-hi), 0 1px 2px color-mix(in oklch, black 6%, transparent);
  cursor: pointer;
  overflow: hidden;
}

/* Teleport target for the notification bell: stays invisible until a page
   mounts a button into it, otherwise it renders as an empty pill. */
#topbar-mobile-bell:empty {
  display: none;
}

#topbar-mobile-more {
  display: contents;
}

.topbar-mobile-btn :deep(button) {
  width: 40px;
  height: 40px;
  border-radius: 12px;
}

/* ---------- Desktop (≥768) · 66px box (60 + 6 top padding, as in the prototype), transparent ---------- */
.topbar {
  display: none;
  height: 66px;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 6px 24px 0 20px;
}

.topbar-crumbs {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
  font-size: 13px;
  color: var(--muted);
  white-space: nowrap;
}

.topbar-crumb-sep {
  opacity: 0.5;
}

.topbar-crumb-link {
  color: var(--muted);
  text-decoration: none;
}

.topbar-crumb-link:hover {
  color: var(--info-text);
  text-decoration: underline;
}

.topbar-crumb-current {
  color: var(--foreground);
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
}

.topbar-actions {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: none;
}

.topbar-more {
  position: relative;
  display: inline-flex;
  align-items: center;
}

.topbar-secondary-actions {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.topbar-secondary-actions.dropdown {
  position: absolute;
  right: 0;
  top: calc(100% + 6px);
  z-index: 40;
  min-width: 176px;
  padding: 6px;
  flex-direction: column;
  align-items: stretch;
}

.topbar-secondary-actions.dropdown > * {
  width: 100%;
  justify-content: flex-start;
}

.topbar-secondary-actions.dropdown > .header-icon-btn {
  gap: 10px;
  padding-inline: 10px;
  font-size: 13px;
  white-space: nowrap;
}

.topbar-search {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  width: 230px;
  padding: 0 12px;
  border-radius: var(--radius-field);
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  color: var(--muted);
  font-size: 13px;
  cursor: text;
  text-align: left;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.topbar-search:hover {
  background: var(--surface);
  border-color: color-mix(in oklch, var(--foreground) 18%, transparent);
}

.topbar-search-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-balance {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 12px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  font-size: 12.5px;
  color: var(--muted);
  white-space: nowrap;
}

.topbar-balance > b {
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
  font-weight: 600;
}

.topbar-balance-pop {
  display: none;
  right: 0;
  top: calc(100% + 6px);
  width: 208px;
  font-size: 12px;
}

.topbar-balance:hover .topbar-balance-pop {
  display: block;
}

.topbar-balance:focus-within .topbar-balance-pop {
  display: block;
}

.topbar-balance-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  color: var(--muted);
}

.topbar-balance-row b {
  color: var(--foreground);
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.topbar-bell :deep(button) {
  width: 34px;
  height: 34px;
  border-radius: 10px;
}

.topbar-divider {
  width: 1px;
  height: 20px;
  background: var(--border);
  margin: 0 4px;
}

.topbar-user {
  display: flex;
  align-items: center;
  gap: 8px;
  height: 34px;
  padding: 0 10px 0 4px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  cursor: pointer;
  transition: background 0.15s ease, border-color 0.15s ease;
}

.topbar-user:hover {
  background: var(--surface);
  border-color: color-mix(in oklch, var(--foreground) 18%, transparent);
}

.topbar-avatar {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  border-radius: 999px;
  overflow: hidden;
  background: color-mix(in oklch, var(--accent) 18%, transparent);
  color: var(--accent);
  font-size: 12px;
  font-weight: 700;
  flex: none;
}

.topbar-avatar img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.topbar-user-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  max-width: 140px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.topbar-menu {
  right: 0;
  top: calc(100% + 8px);
  width: 232px;
}

.topbar-menu-head {
  padding: 8px 10px 6px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.topbar-menu-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.topbar-menu-mail {
  font-size: 12px;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 4px;
}

.topbar-menu-contact {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 6px 10px;
  font-size: 12px;
  color: var(--muted);
}

.topbar-menu-contact b {
  color: var(--foreground);
  font-weight: 500;
  overflow-wrap: anywhere;
}

.dropdown-enter-active,
.dropdown-leave-active {
  transition: opacity 0.15s ease, transform 0.15s ease;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  transform: scale(0.96) translateY(-4px);
}

@media (min-width: 768px) {
  .topbar-mobile {
    display: none;
  }

  .topbar {
    display: flex;
  }
}

.is-compact {
  .topbar {
    height: 66px;
    min-height: 66px;
    flex-wrap: nowrap;
    gap: 8px;
    padding-top: 6px;
    padding-bottom: 0;
  }

  .topbar-crumbs {
    flex: 1;
    min-width: 0;
    overflow: hidden;
  }

  .topbar-crumbs > :not(.topbar-crumb-current) {
    display: none;
  }

  .topbar-actions {
    flex: none;
    min-width: 0;
    flex-wrap: nowrap;
    gap: 4px;
  }

  .topbar-search {
    width: 44px;
    padding: 0;
    justify-content: center;
  }

  .topbar-search-text,
  .topbar-search .kbd,
  .topbar-user-name {
    display: none;
  }

  .topbar-user {
    padding: 0 4px;
  }
}
</style>
