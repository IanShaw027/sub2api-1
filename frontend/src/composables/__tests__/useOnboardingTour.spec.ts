import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { Driver } from 'driver.js'

import {
  moveNextSafely,
  resetMoveNextInFlight
} from '@/composables/useOnboardingTour'
import { useAppStore } from '@/stores/app'

function stubRect(el: Element, height = 20): void {
  vi.spyOn(el, 'getBoundingClientRect').mockReturnValue({
    height,
    width: 80,
    top: 0,
    left: 0,
    bottom: height,
    right: 80,
    x: 0,
    y: 0,
    toJSON: () => ({})
  })
}

function createDriver(options?: {
  index?: number
  nextSelector?: string
}): { driver: Driver; moveNext: ReturnType<typeof vi.fn> } {
  let index = options?.index ?? 0
  const moveNext = vi.fn(() => {
    index += 1
  })
  const nextSelector = options?.nextSelector ?? '#tour-next'
  const driver = {
    isActive: () => true,
    getActiveIndex: () => index,
    getConfig: () => ({
      steps: [{ element: '#tour-current' }, { element: nextSelector }]
    }),
    moveNext
  } as unknown as Driver
  return { driver, moveNext }
}

describe('moveNextSafely', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMoveNextInFlight()
    vi.spyOn(console, 'warn').mockImplementation(() => undefined)
  })

  afterEach(() => {
    resetMoveNextInFlight()
    vi.restoreAllMocks()
    document.body.innerHTML = ''
  })

  it('advances only once when two callers race during ensureElement', async () => {
    const { driver, moveNext } = createDriver()

    await Promise.all([moveNextSafely(driver, 0), moveNextSafely(driver, 0)])

    expect(moveNext).toHaveBeenCalledTimes(1)
  })

  it('allows a later call after the in-flight advance finishes', async () => {
    const { driver, moveNext } = createDriver()

    await moveNextSafely(driver, 0)
    await moveNextSafely(driver, 0)

    expect(moveNext).toHaveBeenCalledTimes(2)
  })

  it('still calls moveNext when the next element never appears', async () => {
    const { driver, moveNext } = createDriver({ nextSelector: '#missing-tour-target' })

    await moveNextSafely(driver, 0)

    expect(console.warn).toHaveBeenCalledWith(
      expect.stringContaining('Next step element not found: #missing-tour-target')
    )
    expect(moveNext).toHaveBeenCalledTimes(1)
  })

  it('does not advance if another path already changed the active index', async () => {
    const { driver, moveNext } = createDriver()
    vi.spyOn(driver, 'getActiveIndex')
      .mockReturnValueOnce(0)
      .mockReturnValueOnce(1)

    await moveNextSafely(driver, 0)

    expect(moveNext).not.toHaveBeenCalled()
  })

  it('force-opens the next sidebar section and waits for a measurable rect before moveNext', async () => {
    const section = document.createElement('div')
    section.setAttribute('data-section', 'myAccount')
    const target = document.createElement('a')
    target.setAttribute('data-tour', 'sidebar-my-keys')
    section.appendChild(target)
    document.body.appendChild(section)
    stubRect(target)
    const order: string[] = []
    target.scrollIntoView = () => {
      order.push('scroll')
    }

    const { driver, moveNext } = createDriver({
      nextSelector: '[data-tour="sidebar-my-keys"]'
    })
    moveNext.mockImplementation(() => {
      order.push('moveNext')
    })

    const appStore = useAppStore()
    await moveNextSafely(driver)

    expect(appStore.sidebarSectionsForceOpen.myAccount).toBe(true)
    expect(appStore.sidebarSectionsOpen.myAccount).toBeUndefined()
    expect(order).toEqual(['scroll', 'moveNext'])
    expect(moveNext).toHaveBeenCalledTimes(1)
  })
})
