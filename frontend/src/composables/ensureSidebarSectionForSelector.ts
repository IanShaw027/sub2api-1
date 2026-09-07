import { nextTick } from 'vue'
import { sectionKeyForSelector } from '@/constants/sidebar'
import { TABLET_UP_MEDIA_QUERY } from '@/composables/useIsMobile'
import { useAppStore } from '@/stores/app'
import { useOnboardingStore } from '@/stores/onboarding'

function isMobileViewport(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return false
  return !window.matchMedia(TABLET_UP_MEDIA_QUERY).matches
}

/**
 * Force-open the sidebar section that owns a tour target so driver.js can
 * measure a non-zero rect. Prefers the live DOM `data-section` (role-aware)
 * and falls back to `sectionKeyForSelector` when the element is not mounted.
 * Uses a transient store flag — does not persist the user's section preference.
 *
 * On mobile the rail is inert/off-screen, so the tour targets live in the
 * drawer: open it first so selectors resolve to clickable nodes.
 */
export async function ensureSidebarSectionForSelector(selector: string): Promise<void> {
  const appStore = useAppStore()
  const targetSection = sectionKeyForSelector(selector)
  if (targetSection) {
    useOnboardingStore().setSidebarMode(targetSection === 'workspace' ? 'user' : 'admin')
    await nextTick()
  }
  if (isMobileViewport()) {
    appStore.setMobileOpen(true)
    await nextTick()
  }
  const el = document.querySelector(selector)
  const key =
    el?.closest('[data-section]')?.getAttribute('data-section') ??
    sectionKeyForSelector(selector)
  if (!key) return
  appStore.forceOpenSidebarSection(key)
  await nextTick()
}
