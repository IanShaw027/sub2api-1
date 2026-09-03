import { computed, readonly, ref } from 'vue'

export type ThemePreference = 'light' | 'dark' | 'system'
export type AccentName = 'blue' | 'sky' | 'indigo' | 'teal' | 'violet'
export type GlassTheme = 'glass-light' | 'glass-dark'

const THEME_KEY = 'theme'
const ACCENT_KEY = 'accent'
const ACCENTS: readonly AccentName[] = ['blue', 'sky', 'indigo', 'teal', 'violet']

const preference = ref<ThemePreference>('system')
const accent = ref<AccentName>('blue')
const generation = ref(0)

let mediaQuery: MediaQueryList | null = null
let initialized = false

function readStorage(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function writeStorage(key: string, value: string): void {
  try {
    localStorage.setItem(key, value)
  } catch {
    // ignore quota / private-mode failures
  }
}

function readPreference(): ThemePreference {
  const saved = readStorage(THEME_KEY)
  if (saved === 'light' || saved === 'dark' || saved === 'system') {
    return saved
  }
  return 'system'
}

function readAccent(): AccentName {
  const saved = readStorage(ACCENT_KEY)
  if (saved && ACCENTS.includes(saved as AccentName)) {
    return saved as AccentName
  }
  return 'blue'
}

function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function resolveIsDark(pref: ThemePreference): boolean {
  if (pref === 'dark') return true
  if (pref === 'light') return false
  return systemPrefersDark()
}

function applyToDocument(): void {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  const dark = resolveIsDark(preference.value)
  root.dataset.theme = dark ? 'glass-dark' : 'glass-light'
  root.dataset.accent = accent.value
  root.classList.toggle('dark', dark)
  root.style.colorScheme = dark ? 'dark' : 'light'
  generation.value += 1
}

function onSystemChange(): void {
  if (preference.value !== 'system') return
  applyToDocument()
}

function ensureMediaListener(): void {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
  if (mediaQuery) return
  mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
  if (typeof mediaQuery.addEventListener === 'function') {
    mediaQuery.addEventListener('change', onSystemChange)
    return
  }
  if (typeof mediaQuery.addListener === 'function') {
    mediaQuery.addListener(onSystemChange)
  }
}

function detachMediaListener(): void {
  if (!mediaQuery) return
  if (typeof mediaQuery.removeEventListener === 'function') {
    mediaQuery.removeEventListener('change', onSystemChange)
  } else if (typeof mediaQuery.removeListener === 'function') {
    mediaQuery.removeListener(onSystemChange)
  }
  mediaQuery = null
}

export function initTheme(): void {
  preference.value = readPreference()
  accent.value = readAccent()
  applyToDocument()
  detachMediaListener()
  ensureMediaListener()
  initialized = true
}

export function setTheme(next: ThemePreference): void {
  preference.value = next
  writeStorage(THEME_KEY, next)
  applyToDocument()
}

export function setAccent(next: AccentName): void {
  accent.value = next
  writeStorage(ACCENT_KEY, next)
  applyToDocument()
}

export function toggleDark(): void {
  setTheme(resolveIsDark(preference.value) ? 'light' : 'dark')
}

/**
 * Single source of truth for `data-theme`, `data-accent`, and the `dark` class.
 * `main.ts` calls `initTheme()` before mount; components use this composable.
 */
export function useTheme() {
  if (!initialized) {
    initTheme()
  }

  const isDark = computed(() => {
    void generation.value
    return resolveIsDark(preference.value)
  })

  const resolvedTheme = computed<GlassTheme>(() => (isDark.value ? 'glass-dark' : 'glass-light'))

  return {
    preference: readonly(preference),
    accent: readonly(accent),
    isDark,
    resolvedTheme,
    setTheme,
    setAccent,
    toggleDark,
    toggleTheme: toggleDark
  }
}
