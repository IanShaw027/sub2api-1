import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import ConfirmDialog from '../ConfirmDialog.vue'
import { resetOverlayLock } from '@/components/ui/overlayLock'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

function mountDialog(props: Record<string, unknown> = {}) {
  return mount(ConfirmDialog, {
    attachTo: document.body,
    props: { show: true, title: 'Delete account', message: 'This cannot be undone.', ...props },
    global: { stubs: { Icon: true } }
  })
}

afterEach(() => {
  resetOverlayLock()
  document.body.innerHTML = ''
})

describe('ConfirmDialog', () => {
  it('renders a 44px danger tone icon and a .btn-danger confirm button for destructive actions', () => {
    const wrapper = mountDialog({ danger: true })
    const icon = document.body.querySelector('.confirm-dialog-icon')
    expect(icon).not.toBeNull()
    expect(icon?.classList.contains('confirm-dialog-icon-danger')).toBe(true)
    const confirmButton = Array.from(document.body.querySelectorAll('button')).find((btn) =>
      btn.textContent?.includes('common.confirm')
    )
    expect(confirmButton?.classList.contains('btn-danger')).toBe(true)
    wrapper.unmount()
  })

  it('supports warning and accent tones explicitly', () => {
    const warning = mountDialog({ tone: 'warning' })
    expect(document.body.querySelector('.confirm-dialog-icon-warning')).not.toBeNull()
    warning.unmount()

    const accent = mountDialog({ tone: 'accent' })
    expect(document.body.querySelector('.confirm-dialog-icon-accent')).not.toBeNull()
    accent.unmount()
  })

  it('emits confirm and cancel', async () => {
    const wrapper = mountDialog()
    const buttons = Array.from(document.body.querySelectorAll('button'))
    const cancelButton = buttons.find((btn) => btn.textContent?.includes('common.cancel'))
    cancelButton?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(wrapper.emitted('cancel')).toBeTruthy()
    wrapper.unmount()
  })
})
