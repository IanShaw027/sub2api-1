import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import SetupWizardView from '../SetupWizardView.vue'

const configureSetupBootstrapSecretFromLocation = vi.fn()
const testDatabase = vi.fn()
const testRedis = vi.fn()
const install = vi.fn()
const buildGatewayUrl = vi.fn((path: string) => `http://localhost${path}`)

vi.mock('@/api/setup', () => ({
  configureSetupBootstrapSecretFromLocation: (...args: unknown[]) =>
    configureSetupBootstrapSecretFromLocation(...args),
  testDatabase: (...args: unknown[]) => testDatabase(...args),
  testRedis: (...args: unknown[]) => testRedis(...args),
  install: (...args: unknown[]) => install(...args)
}))

vi.mock('@/api/client', () => ({
  buildGatewayUrl: (...args: unknown[]) => buildGatewayUrl(...args)
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const globalStubs = {
  Icon: true,
  UiSelect: true
}

async function fillDatabaseStep(wrapper: ReturnType<typeof mount>) {
  testDatabase.mockResolvedValueOnce({})
  await wrapper.findAllComponents({ name: 'Button' })[0].trigger('click')
  await flushPromises()
}

describe('SetupWizardView', () => {
  beforeEach(() => {
    configureSetupBootstrapSecretFromLocation.mockReset()
    testDatabase.mockReset()
    testRedis.mockReset()
    install.mockReset()
    buildGatewayUrl.mockClear()
  })

  it('captures the bootstrap secret from the location fragment on mount', () => {
    mount(SetupWizardView, { global: { stubs: globalStubs } })
    expect(configureSetupBootstrapSecretFromLocation).toHaveBeenCalled()
  })

  it('renders the database step first with the test-connection button disabled from proceeding', () => {
    const wrapper = mount(SetupWizardView, { global: { stubs: globalStubs } })

    expect(wrapper.text()).toContain('setup.database.title')
    const nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    expect(nextButton?.attributes('disabled')).toBeDefined()
  })

  it('enables moving to the redis step once the database connection test succeeds', async () => {
    testDatabase.mockResolvedValueOnce({})
    const wrapper = mount(SetupWizardView, { global: { stubs: globalStubs } })

    const testButton = wrapper.findAllComponents({ name: 'Button' })[0]
    await testButton.trigger('click')
    await flushPromises()

    expect(testDatabase).toHaveBeenCalled()
    expect(wrapper.text()).toContain('setup.status.success')

    const nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    expect(nextButton?.attributes('disabled')).toBeUndefined()

    await nextButton?.trigger('click')
    expect(wrapper.text()).toContain('setup.redis.title')
  })

  it('surfaces a connection error message when the database test rejects', async () => {
    testDatabase.mockRejectedValueOnce({ message: 'db down' })
    const wrapper = mount(SetupWizardView, { global: { stubs: globalStubs } })

    const testButton = wrapper.findAllComponents({ name: 'Button' })[0]
    await testButton.trigger('click')
    await flushPromises()

    expect(wrapper.find('.notice-danger').exists()).toBe(true)
    expect(wrapper.text()).toContain('db down')
  })

  it('gates the admin step on matching, sufficiently long passwords before allowing completion', async () => {
    const wrapper = mount(SetupWizardView, { global: { stubs: globalStubs } })

    await fillDatabaseStep(wrapper)
    let nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    await nextButton?.trigger('click')

    testRedis.mockResolvedValueOnce({})
    const redisTestButton = wrapper.findAllComponents({ name: 'Button' })[0]
    await redisTestButton.trigger('click')
    await flushPromises()
    nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    await nextButton?.trigger('click')

    expect(wrapper.text()).toContain('setup.admin.title')
    nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    expect(nextButton?.attributes('disabled')).toBeDefined()

    const inputs = wrapper.findAll('input')
    const emailInput = inputs.find((i) => i.attributes('type') === 'email')
    const passwordInputs = inputs.filter((i) => i.attributes('type') === 'password')
    await emailInput?.setValue('admin@example.com')
    await passwordInputs[0]?.setValue('supersecret')
    await passwordInputs[1]?.setValue('supersecret')

    nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    expect(nextButton?.attributes('disabled')).toBeUndefined()
  })

  it('runs the install request and shows a success notice on the final step', async () => {
    const wrapper = mount(SetupWizardView, { global: { stubs: globalStubs } })

    await fillDatabaseStep(wrapper)
    let nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    await nextButton?.trigger('click')

    testRedis.mockResolvedValueOnce({})
    await wrapper.findAllComponents({ name: 'Button' })[0].trigger('click')
    await flushPromises()
    nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    await nextButton?.trigger('click')

    const inputs = wrapper.findAll('input')
    const emailInput = inputs.find((i) => i.attributes('type') === 'email')
    const passwordInputs = inputs.filter((i) => i.attributes('type') === 'password')
    await emailInput?.setValue('admin@example.com')
    await passwordInputs[0]?.setValue('supersecret')
    await passwordInputs[1]?.setValue('supersecret')
    nextButton = wrapper.findAllComponents({ name: 'Button' }).find((b) => b.text().includes('common.next'))
    await nextButton?.trigger('click')

    expect(wrapper.text()).toContain('setup.ready.title')

    install.mockResolvedValueOnce({})
    // The post-install restart poll uses real setTimeout/fetch; drive it with fake
    // timers and a stubbed fetch so the test doesn't wait out the real ~60s poll loop.
    buildGatewayUrl.mockReturnValue('http://localhost/setup/status')
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, json: async () => ({ data: { needs_setup: true } }) })
    )
    vi.useFakeTimers()

    const completeButton = wrapper
      .findAllComponents({ name: 'Button' })
      .find((b) => b.text().includes('setup.status.completeInstallation'))
    await completeButton?.trigger('click')
    await flushPromises()
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(install).toHaveBeenCalled()
    expect(wrapper.find('.notice-success').exists()).toBe(true)
    expect(wrapper.text()).toContain('setup.status.completed')

    vi.useRealTimers()
    vi.unstubAllGlobals()
  })
})
