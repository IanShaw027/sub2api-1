import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'

import {
  DESKTOP_UP_MEDIA_QUERY,
  TABLET_UP_MEDIA_QUERY,
  useIsMobile
} from '../useIsMobile'

const originalMatchMedia = window.matchMedia

type Viewport = 'mobile' | 'tablet' | 'desktop'

function mockViewport(initial: Viewport) {
  const states: Record<Viewport, { tabletUp: boolean; desktop: boolean }> = {
    mobile: { tabletUp: false, desktop: false },
    tablet: { tabletUp: true, desktop: false },
    desktop: { tabletUp: true, desktop: true }
  }
  let current = states[initial]
  const listeners = new Map<string, Set<(event: MediaQueryListEvent) => void>>()

  function matchesFor(query: string) {
    if (query.includes('min-width: 1024px')) return current.desktop
    if (query.includes('min-width: 768px')) return current.tabletUp
    return false
  }

  window.matchMedia = ((query: string) => {
    if (!listeners.has(query)) listeners.set(query, new Set())
    const set = listeners.get(query)!
    return {
      get matches() {
        return matchesFor(query)
      },
      media: query,
      onchange: null,
      addListener: (cb: (event: MediaQueryListEvent) => void) => set.add(cb),
      removeListener: (cb: (event: MediaQueryListEvent) => void) => set.delete(cb),
      addEventListener: (_event: string, cb: (event: MediaQueryListEvent) => void) => set.add(cb),
      removeEventListener: (_event: string, cb: (event: MediaQueryListEvent) => void) => set.delete(cb),
      dispatchEvent: () => true
    }
  }) as unknown as typeof window.matchMedia

  function setViewport(next: Viewport) {
    current = states[next]
    for (const [query, set] of listeners) {
      const event = { matches: matchesFor(query), media: query } as MediaQueryListEvent
      set.forEach((cb) => cb(event))
    }
  }

  return { setViewport }
}

function mountComposable() {
  let api!: ReturnType<typeof useIsMobile>
  const wrapper = mount(
    defineComponent({
      setup() {
        api = useIsMobile()
        return () => null
      }
    })
  )
  return { wrapper, api }
}

describe('useIsMobile', () => {
  afterEach(() => {
    window.matchMedia = originalMatchMedia
  })

  it('reports mobile below 768px', () => {
    mockViewport('mobile')
    const { wrapper, api } = mountComposable()
    expect(api.isMobile.value).toBe(true)
    expect(api.isTablet.value).toBe(false)
    expect(api.isDesktop.value).toBe(false)
    expect(api.isTabletUp.value).toBe(false)
    wrapper.unmount()
  })

  it('reports tablet between 768 and 1023', () => {
    mockViewport('tablet')
    const { wrapper, api } = mountComposable()
    expect(api.isMobile.value).toBe(false)
    expect(api.isTablet.value).toBe(true)
    expect(api.isDesktop.value).toBe(false)
    expect(api.isTabletUp.value).toBe(true)
    wrapper.unmount()
  })

  it('reports desktop at 1024 and up', () => {
    mockViewport('desktop')
    const { wrapper, api } = mountComposable()
    expect(api.isMobile.value).toBe(false)
    expect(api.isTablet.value).toBe(false)
    expect(api.isDesktop.value).toBe(true)
    expect(api.isTabletUp.value).toBe(true)
    wrapper.unmount()
  })

  it('updates flags when matchMedia change fires', () => {
    const { setViewport } = mockViewport('mobile')
    const { wrapper, api } = mountComposable()
    expect(api.isMobile.value).toBe(true)

    setViewport('tablet')
    expect(api.isMobile.value).toBe(false)
    expect(api.isTablet.value).toBe(true)
    expect(api.isDesktop.value).toBe(false)

    setViewport('desktop')
    expect(api.isTablet.value).toBe(false)
    expect(api.isDesktop.value).toBe(true)
    wrapper.unmount()
  })

  it('exports the Tailwind md/lg media queries', () => {
    expect(TABLET_UP_MEDIA_QUERY).toBe('(min-width: 768px)')
    expect(DESKTOP_UP_MEDIA_QUERY).toBe('(min-width: 1024px)')
  })
})
