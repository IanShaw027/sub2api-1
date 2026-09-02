import { createPinia, setActivePinia } from 'pinia'
import { describe, expect, it } from 'vitest'

import { useOnboardingStore } from '../onboarding'

describe('onboarding store driverActive', () => {
  it('tracks tour activity reactively instead of driver.isActive()', () => {
    setActivePinia(createPinia())
    const store = useOnboardingStore()

    expect(store.isDriverActive()).toBe(false)
    store.setDriverInstance({} as never)
    expect(store.isDriverActive()).toBe(false)

    store.setDriverActive(true)
    expect(store.isDriverActive()).toBe(true)
    expect(store.driverActive).toBe(true)

    store.setDriverInstance(null)
    expect(store.isDriverActive()).toBe(false)
    expect(store.driverActive).toBe(false)
  })
})
