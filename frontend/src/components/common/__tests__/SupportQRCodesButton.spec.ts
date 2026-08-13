import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import SupportQRCodesButton from '@/components/common/SupportQRCodesButton.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        if (key === 'common.contactSupport') return 'Contact Support'
        return key
      },
    }),
  }
})

describe('SupportQRCodesButton', () => {
  it('does not render when there are no valid qr codes or legacy contact info', () => {
    const wrapper = mount(SupportQRCodesButton, {
      props: {
        entries: [{ image_url: '   ', note: 'ignored' }],
        legacyContactInfo: '   ',
      },
      global: {
        stubs: {
          Teleport: false,
          Transition: false,
          Icon: true,
        },
      },
    })

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('rejects javascript urls', () => {
    const wrapper = mount(SupportQRCodesButton, {
      props: {
        entries: [{ image_url: 'javascript:alert(1)', note: 'bad' }],
      },
      global: {
        stubs: {
          Teleport: false,
          Transition: false,
          Icon: true,
        },
      },
    })

    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('renders legacy contact info when qr codes are unavailable', async () => {
    const wrapper = mount(SupportQRCodesButton, {
      props: {
        entries: [],
        legacyContactInfo: '  QQ: 123456789  ',
      },
      attachTo: document.body,
      global: {
        stubs: {
          Transition: false,
          Icon: true,
        },
      },
    })

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(document.body.textContent).toContain('QQ: 123456789')
    expect(document.body.querySelectorAll('img')).toHaveLength(0)

    wrapper.unmount()
  })

  it('opens a dialog and renders qr codes with optional notes', async () => {
    const wrapper = mount(SupportQRCodesButton, {
      props: {
        entries: [
          { image_url: ' https://cdn.example.com/support-1.png ', note: ' Main Support ' },
          { image_url: 'https://cdn.example.com/support-2.png' },
          { image_url: '/api/v1/media/public/12', note: 'Media' },
        ],
      },
      attachTo: document.body,
      global: {
        stubs: {
          Transition: false,
          Icon: true,
        },
      },
    })

    await wrapper.get('button').trigger('click')
    await flushPromises()

    const images = Array.from(document.body.querySelectorAll('img'))
    expect(images).toHaveLength(3)
    expect(images[0]?.getAttribute('src')).toBe('https://cdn.example.com/support-1.png')
    expect(images[1]?.getAttribute('src')).toBe('https://cdn.example.com/support-2.png')
    expect(images[2]?.getAttribute('src')).toBe('/api/v1/media/public/12')
    expect(document.body.textContent).toContain('Main Support')
    expect(document.body.textContent).toContain('Contact Support')

    wrapper.unmount()
  })

  it('prefers qr codes over legacy contact info when both are configured', async () => {
    const wrapper = mount(SupportQRCodesButton, {
      props: {
        entries: [
          { image_url: 'https://cdn.example.com/support-1.png', note: 'Main Support' },
        ],
        legacyContactInfo: 'QQ: 123456789',
      },
      attachTo: document.body,
      global: {
        stubs: {
          Transition: false,
          Icon: true,
        },
      },
    })

    await wrapper.get('button').trigger('click')
    await flushPromises()

    expect(document.body.querySelectorAll('img')).toHaveLength(1)
    expect(document.body.textContent).not.toContain('QQ: 123456789')

    wrapper.unmount()
  })
})
