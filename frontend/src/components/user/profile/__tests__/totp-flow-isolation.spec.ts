import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { Component } from 'vue'
import TotpSetupModal from '../TotpSetupModal.vue'
import TotpDisableDialog from '../TotpDisableDialog.vue'
import ProfileTotpCard from '../ProfileTotpCard.vue'

const mocks = vi.hoisted(() => ({
  getVerificationMethod: vi.fn(), sendVerifyCode: vi.fn(), initiateSetup: vi.fn(),
  enable: vi.fn(), disable: vi.fn(), showError: vi.fn(), showSuccess: vi.fn(),
  qr: vi.fn(), getStatus: vi.fn(),
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('@/api', () => ({ totpAPI: mocks }))
vi.mock('qrcode', () => ({ default: { toDataURL: mocks.qr } }))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

const setup = {
  secret: 'CURRENT_SECRET', qr_code_url: 'otpauth://totp/example?secret=CURRENT_SECRET', setup_token: 'current-token',
}
const dialogs = [
  { name: 'setup', component: TotpSetupModal },
  { name: 'disable', component: TotpDisableDialog },
] as const

describe('TOTP dialog asynchronous flow isolation', () => {
  let wrapper: VueWrapper | undefined
  let unmounted = false

  beforeEach(() => {
    vi.resetAllMocks()
    unmounted = false
    mocks.getVerificationMethod.mockResolvedValue({ method: 'password' })
    mocks.initiateSetup.mockResolvedValue(setup)
    mocks.qr.mockResolvedValue('data:image/png;base64,CURRENT')
    mocks.getStatus.mockResolvedValue({ feature_enabled: true, enabled: false })
  })

  afterEach(() => {
    if (!unmounted) wrapper?.unmount()
    document.body.innerHTML = ''
    vi.restoreAllMocks()
  })

  async function mountDialog(component: Component) {
    wrapper = mount(component, {
      props: { open: true }, attachTo: document.body,
      global: { stubs: { teleport: true, transition: true } },
    })
    await flushPromises()
    return wrapper
  }

  async function finishFlow(mode: 'reopen' | 'unmount') {
    if (mode === 'unmount') {
      wrapper!.unmount()
      unmounted = true
    } else {
      await wrapper!.setProps({ open: false })
      await wrapper!.setProps({ open: true })
      await flushPromises()
    }
  }

  for (const dialog of dialogs) {
    for (const mode of ['reopen', 'unmount'] as const) {
      it(`${dialog.name}: ignores stale method failure after ${mode}`, async () => {
        const old = deferred<{ method: string }>()
        mocks.getVerificationMethod.mockReturnValueOnce(old.promise)
        const modal = await mountDialog(dialog.component)
        await finishFlow(mode)
        old.reject(new Error('old method failed'))
        await flushPromises()
        expect(modal.emitted('close')).toBeUndefined()
        expect(mocks.showError).not.toHaveBeenCalled()
        if (mode === 'reopen') expect(modal.find('input[type="password"]').exists()).toBe(true)
      })

      it(`${dialog.name}: stale send-code completion creates no timer after ${mode}`, async () => {
        mocks.getVerificationMethod.mockResolvedValue({ method: 'email' })
        const old = deferred<void>()
        mocks.sendVerifyCode.mockReturnValueOnce(old.promise)
        const modal = await mountDialog(dialog.component)
        const timer = vi.spyOn(globalThis, 'setInterval')
        const send = modal.findAll('button').find(button => button.text() === 'profile.totp.sendCode')!
        await send.trigger('click')
        await finishFlow(mode)
        old.resolve()
        await flushPromises()
        expect(timer).not.toHaveBeenCalled()
        expect(mocks.showSuccess).not.toHaveBeenCalled()
        if (mode === 'reopen') {
          const newSend = modal.findAll('button').find(button => button.text() === 'profile.totp.sendCode')!
          expect(newSend.attributes('disabled')).toBeUndefined()
        }
      })
    }
  }

  it.each(['reopen', 'unmount'] as const)('does not publish a cancelled setup result after %s', async (mode) => {
    const old = deferred<typeof setup>()
    mocks.initiateSetup.mockReturnValueOnce(old.promise)
    const modal = await mountDialog(TotpSetupModal)
    await modal.get('input[type="password"]').setValue('password')
    await modal.get('button.btn-primary').trigger('click')
    await finishFlow(mode)
    old.resolve({ ...setup, secret: 'CANCELLED_SECRET' })
    await flushPromises()
    expect(mocks.qr).not.toHaveBeenCalled()
    if (mode === 'reopen') {
      expect(modal.find('input[type="password"]').exists()).toBe(true)
      expect(modal.text()).not.toContain('CANCELLED_SECRET')
      await modal.get('input[type="password"]').setValue('new password')
      expect(modal.get('button.btn-primary').attributes('disabled')).toBeUndefined()
    }
  })

  it('does not replace the current QR image with a late cancelled generation', async () => {
    const oldQr = deferred<string>()
    mocks.qr.mockReturnValueOnce(oldQr.promise)
    const modal = await mountDialog(TotpSetupModal)
    await modal.get('input[type="password"]').setValue('password')
    await modal.get('button.btn-primary').trigger('click')
    await flushPromises()
    await finishFlow('reopen')
    await modal.get('input[type="password"]').setValue('new password')
    await modal.get('button.btn-primary').trigger('click')
    await flushPromises()
    expect(modal.get('img').attributes('src')).toBe('data:image/png;base64,CURRENT')
    oldQr.resolve('data:image/png;base64,CANCELLED')
    await flushPromises()
    expect(modal.get('img').attributes('src')).toBe('data:image/png;base64,CURRENT')
  })

  for (const mode of ['reopen', 'unmount'] as const) {
    it(`does not emit stale enable success after ${mode}`, async () => {
      const old = deferred<void>()
      mocks.enable.mockReturnValueOnce(old.promise)
      const modal = await mountDialog(TotpSetupModal)
      await modal.get('input[type="password"]').setValue('password')
      await modal.get('button.btn-primary').trigger('click')
      await flushPromises()
      await modal.get('button.btn-primary').trigger('click')
      for (const input of modal.findAll('input[maxlength="1"]')) await input.setValue('1')
      await modal.get('form').trigger('submit')
      expect(mocks.enable).toHaveBeenCalledWith({ totp_code: '111111', setup_token: 'current-token' })
      await finishFlow(mode)
      old.resolve()
      await flushPromises()
      expect(modal.emitted('success')).toBeUndefined()
      expect(modal.emitted('changed')?.length ?? 0).toBe(mode === 'reopen' ? 1 : 0)
      expect(mocks.showSuccess).not.toHaveBeenCalled()
    })

    it(`does not emit stale disable success after ${mode}`, async () => {
      const old = deferred<void>()
      mocks.disable.mockReturnValueOnce(old.promise)
      const modal = await mountDialog(TotpDisableDialog)
      await modal.get('input[type="password"]').setValue('password')
      await modal.get('form').trigger('submit')
      expect(mocks.disable).toHaveBeenCalledWith({ password: 'password' })
      await finishFlow(mode)
      old.resolve()
      await flushPromises()
      expect(modal.emitted('success')).toBeUndefined()
      expect(modal.emitted('changed')?.length ?? 0).toBe(mode === 'reopen' ? 1 : 0)
      expect(mocks.showSuccess).not.toHaveBeenCalled()
    })
  }

  it('invalidates immediately when the user closes before the parent updates open', async () => {
    const old = deferred<typeof setup>()
    mocks.initiateSetup.mockReturnValueOnce(old.promise)
    const modal = await mountDialog(TotpSetupModal)
    await modal.get('input[type="password"]').setValue('password')
    await modal.get('button.btn-primary').trigger('click')
    await modal.get('.ui-modal-close').trigger('click')
    expect(modal.emitted('close')).toHaveLength(1)
    old.resolve(setup)
    await flushPromises()
    expect(modal.find('input[type="password"]').exists()).toBe(true)
    expect(mocks.qr).not.toHaveBeenCalled()
  })

  for (const initiallyEnabled of [false, true]) {
    for (const state of ['current', 'closed', 'reopened'] as const) {
      it(`refreshes parent status after ${initiallyEnabled ? 'disable' : 'enable'} in the ${state} wizard`, async () => {
        mocks.getStatus.mockResolvedValue({ feature_enabled: true, enabled: initiallyEnabled })
        const mutation = deferred<void>()
        const mutationMock = initiallyEnabled ? mocks.disable : mocks.enable
        mutationMock.mockReturnValueOnce(mutation.promise)
        wrapper = mount(ProfileTotpCard, {
          attachTo: document.body,
          global: { stubs: { teleport: true, transition: true } },
        })
        await flushPromises()
        await wrapper.get('[role="switch"]').trigger('click')
        await flushPromises()
        const modal = initiallyEnabled
          ? wrapper.getComponent(TotpDisableDialog)
          : wrapper.getComponent(TotpSetupModal)
        await modal.get('input[type="password"]').setValue('password')
        if (!initiallyEnabled) {
          await modal.get('button.btn-primary').trigger('click')
          await flushPromises()
          await modal.get('button.btn-primary').trigger('click')
          for (const input of modal.findAll('input[maxlength="1"]')) await input.setValue('1')
        }
        await modal.get('form').trigger('submit')
        expect(mutationMock).toHaveBeenCalledTimes(1)
        if (state !== 'current') {
          await modal.get('.ui-modal-close').trigger('click')
          await flushPromises()
        }
        if (state === 'reopened') {
          await wrapper.get('[role="switch"]').trigger('click')
          await flushPromises()
          await modal.get('input[type="password"]').setValue('new flow password')
        }
        mocks.getStatus.mockResolvedValue({ feature_enabled: true, enabled: !initiallyEnabled })
        mutation.resolve()
        await flushPromises()
        expect(mocks.getStatus).toHaveBeenCalledTimes(2)
        expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe(String(!initiallyEnabled))
        expect(modal.props('open')).toBe(state === 'reopened')
        expect(modal.emitted('success')?.length ?? 0).toBe(state === 'current' ? 1 : 0)
        expect(mocks.showSuccess).toHaveBeenCalledTimes(state === 'current' ? 1 : 0)
        if (state === 'reopened') {
          expect((modal.get('input[type="password"]').element as HTMLInputElement).value).toBe('new flow password')
        }
      })
    }
  }
})
