import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import ProfilePasskeyCard from '../ProfilePasskeyCard.vue'
import { resetOverlayLock } from '@/components/ui/overlayLock'

const { listMock, removeMock } = vi.hoisted(() => ({
  listMock: vi.fn(),
  removeMock: vi.fn()
}))

vi.mock('@/api', () => ({
  passkeyAPI: {
    isSupported: () => true,
    list: listMock,
    remove: removeMock
  }
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() })
}))
vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

beforeEach(() => {
  listMock.mockResolvedValue([{
    id: 7,
    name: 'Laptop',
    created_at: '2026-04-20T00:00:00Z',
    last_used_at: null
  }])
  removeMock.mockReset()
})
afterEach(() => {
  resetOverlayLock()
  document.body.innerHTML = ''
})

describe('ProfilePasskeyCard delete confirmation', () => {
  it('requires a password, restores Enter confirmation and prevents duplicate submissions', async () => {
    let complete: () => void = () => {}
    removeMock.mockImplementationOnce(() => new Promise<void>((resolve) => { complete = resolve }))
    const wrapper = mount(ProfilePasskeyCard, {
      attachTo: document.body,
      props: { enabled: true },
      global: { stubs: { Icon: true } }
    })
    await flushPromises()
    await wrapper.get('button.btn-ghost').trigger('click')
    await flushPromises()
    const input = document.querySelector<HTMLInputElement>('#passkey-delete-password')!
    const confirm = document.querySelector<HTMLButtonElement>('.btn-danger')!
    expect(input.autocomplete).toBe('current-password')
    expect(confirm.disabled).toBe(true)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    expect(removeMock).not.toHaveBeenCalled()
    input.value = 'current-secret'
    input.dispatchEvent(new Event('input', { bubbles: true }))
    await flushPromises()
    expect(confirm.disabled).toBe(false)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    await flushPromises()
    expect(removeMock).toHaveBeenCalledWith(7, 'current-secret')
    expect(input.disabled).toBe(true)
    expect(confirm.disabled).toBe(true)
    input.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))
    expect(removeMock).toHaveBeenCalledTimes(1)
    complete()
    await flushPromises()
    expect(wrapper.text()).not.toContain('Laptop')
    wrapper.unmount()
  })
})
