import { computed, onBeforeUnmount, ref } from 'vue'

/** Tablet and up — matches Tailwind `md` (768px). */
export const TABLET_UP_MEDIA_QUERY = '(min-width: 768px)'

/** Desktop and up — matches Tailwind `lg` (1024px). */
export const DESKTOP_UP_MEDIA_QUERY = '(min-width: 1024px)'

function subscribeMql(query: string, onChange: (matches: boolean) => void): () => void {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return () => {}
  }
  const mql = window.matchMedia(query)
  const sync = () => onChange(mql.matches)
  sync()
  if (typeof mql.addEventListener === 'function') {
    mql.addEventListener('change', sync)
    return () => mql.removeEventListener('change', sync)
  }
  if (typeof mql.addListener === 'function') {
    mql.addListener(sync)
    return () => mql.removeListener(sync)
  }
  return () => {}
}

/**
 * Viewport flags for the mobile shell.
 * Defaults to desktop so tests (matchMedia mock returns `matches: true`)
 * and SSR keep the rail/header chrome, not the drawer.
 *
 * Reads `matchMedia` synchronously in setup to avoid a collapsed/expanded flash.
 *
 * - `isMobile`: &lt;768
 * - `isTablet`: 768–1023
 * - `isDesktop`: ≥1024
 */
export function useIsMobile() {
  const isTabletUp = ref(true)
  const isDesktop = ref(true)
  let unbindTablet: (() => void) | null = null
  let unbindDesktop: (() => void) | null = null

  if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
    unbindTablet = subscribeMql(TABLET_UP_MEDIA_QUERY, (matches) => {
      isTabletUp.value = matches
    })
    unbindDesktop = subscribeMql(DESKTOP_UP_MEDIA_QUERY, (matches) => {
      isDesktop.value = matches
    })
  }

  onBeforeUnmount(() => {
    unbindTablet?.()
    unbindDesktop?.()
    unbindTablet = null
    unbindDesktop = null
  })

  return {
    isTabletUp: computed(() => isTabletUp.value),
    isDesktop: computed(() => isDesktop.value),
    isTablet: computed(() => isTabletUp.value && !isDesktop.value),
    isMobile: computed(() => !isTabletUp.value)
  }
}
