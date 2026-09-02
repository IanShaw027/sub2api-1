import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { initTheme, setAccent, setTheme, useTheme } from '../useTheme'

const originalMatchMedia = window.matchMedia

function mockMatchMedia(matches: boolean) {
  const listeners: Array<(event?: MediaQueryListEvent) => void> = []
  const media = {
    matches,
    media: '(prefers-color-scheme: dark)',
    onchange: null,
    addListener: (cb: () => void) => listeners.push(cb),
    removeListener: vi.fn(),
    addEventListener: (_event: string, cb: (event: MediaQueryListEvent) => void) => listeners.push(cb),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
    emit() {
      listeners.forEach((listener) => listener({ matches: media.matches } as MediaQueryListEvent))
    }
  }
  window.matchMedia = (() => media) as unknown as typeof window.matchMedia
  return media
}

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    delete document.documentElement.dataset.theme
    delete document.documentElement.dataset.accent
    mockMatchMedia(false)
    initTheme()
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    delete document.documentElement.dataset.theme
    delete document.documentElement.dataset.accent
  })

  it('sets both html.dark and data-theme for explicit dark', () => {
    localStorage.setItem('theme', 'dark')
    initTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.dataset.theme).toBe('glass-dark')
    expect(document.documentElement.dataset.accent).toBe('blue')
  })

  it('sets glass-light without the dark class for explicit light', () => {
    localStorage.setItem('theme', 'light')
    initTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.dataset.theme).toBe('glass-light')
  })

  it('honors system preference when theme is system or unset', () => {
    mockMatchMedia(true)
    initTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.dataset.theme).toBe('glass-dark')

    setTheme('system')
    mockMatchMedia(false)
    initTheme()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.dataset.theme).toBe('glass-light')
  })

  it('persists accent on html.dataset.accent', () => {
    setAccent('teal')
    expect(localStorage.getItem('accent')).toBe('teal')
    expect(document.documentElement.dataset.accent).toBe('teal')

    document.documentElement.dataset.accent = 'blue'
    initTheme()
    expect(document.documentElement.dataset.accent).toBe('teal')
  })

  it('toggleDark writes an explicit light/dark preference', () => {
    const { toggleDark, isDark } = useTheme()
    setTheme('light')
    expect(isDark.value).toBe(false)

    toggleDark()
    expect(localStorage.getItem('theme')).toBe('dark')
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.dataset.theme).toBe('glass-dark')
  })

  it('survives localStorage read and write failures', () => {
    const getItem = vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('denied')
    })
    expect(() => initTheme()).not.toThrow()
    expect(document.documentElement.dataset.theme).toBe('glass-light')
    getItem.mockRestore()

    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota')
    })
    expect(() => setTheme('dark')).not.toThrow()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.dataset.theme).toBe('glass-dark')
    setItem.mockRestore()
  })

  it('treats garbage theme values as system', () => {
    localStorage.setItem('theme', 'neon-disco')
    localStorage.setItem('accent', 'hotpink')
    initTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.dataset.theme).toBe('glass-light')
    expect(document.documentElement.dataset.accent).toBe('blue')
  })

  it('flips html.dark and data-theme when the system preference changes', () => {
    const media = mockMatchMedia(false)
    localStorage.setItem('theme', 'system')
    initTheme()

    expect(document.documentElement.classList.contains('dark')).toBe(false)
    expect(document.documentElement.dataset.theme).toBe('glass-light')

    media.matches = true
    media.emit()

    expect(document.documentElement.classList.contains('dark')).toBe(true)
    expect(document.documentElement.dataset.theme).toBe('glass-dark')
  })
})
