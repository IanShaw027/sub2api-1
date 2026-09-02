import { nextTick } from 'vue'
import { sectionKeyForSelector } from '@/constants/sidebar'
import { useAppStore } from '@/stores/app'

/**
 * Force-open the sidebar section that owns a tour target so driver.js can
 * measure a non-zero rect. Prefers the live DOM `data-section` (role-aware)
 * and falls back to `sectionKeyForSelector` when the element is not mounted.
 * Uses a transient store flag — does not persist the user's section preference.
 */
export async function ensureSidebarSectionForSelector(selector: string): Promise<void> {
  const el = document.querySelector(selector)
  const key =
    el?.closest('[data-section]')?.getAttribute('data-section') ??
    sectionKeyForSelector(selector)
  if (!key) return
  useAppStore().forceOpenSidebarSection(key)
  await nextTick()
}
