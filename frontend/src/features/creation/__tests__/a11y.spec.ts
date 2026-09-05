import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) =>
        key.replace(/\{(\w+)\}/g, (_match, token) => String(params?.[token] ?? `{${token}}`)),
    }),
  }
})

import MessageStream from '../components/MessageStream.vue'
import ComposerBar from '../components/ComposerBar.vue'
import PreviewDialog from '../components/PreviewDialog.vue'
import { useCreationStore } from '../stores/creation'
import { resetOverlayLock } from '@/components/ui/overlayLock'

const stubs = {
  Icon: true,
}

describe('creation studio accessibility', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    resetOverlayLock()
    document.body.innerHTML = ''
  })

  describe('MessageStream', () => {
    it('exposes a live log region and labels each message', async () => {
      const store = useCreationStore()
      store.messages = [
        { id: 1, session_id: 1, role: 'user', content: 'hello', created_at: '2026-01-01T00:00:00Z' },
        { id: 2, session_id: 1, role: 'assistant', content: 'hi there', created_at: '2026-01-01T00:01:00Z' },
      ]

      const wrapper = mount(MessageStream, { global: { stubs } })

      const log = wrapper.get('.studio-message-stream')
      expect(log.attributes('role')).toBe('log')
      expect(log.attributes('aria-live')).toBe('polite')
      expect(log.attributes('aria-label')).toBe('studio.a11y.messageLog')

      const articles = wrapper.findAll('article')
      expect(articles).toHaveLength(2)
      expect(articles[0].attributes('aria-label')).toContain('studio.a11y.userMessage')
      expect(articles[1].attributes('aria-label')).toContain('studio.a11y.assistantMessage')
    })

    it('marks the stream as busy and announces the loading state', async () => {
      const store = useCreationStore()
      store.messagesLoading = true

      const wrapper = mount(MessageStream, { global: { stubs } })

      expect(wrapper.get('.studio-message-stream').attributes('aria-busy')).toBe('true')
      const status = wrapper.get('[role="status"]')
      expect(status.attributes('aria-live')).toBe('polite')
    })

    it('marks the stream as busy while a response is streaming', async () => {
      const store = useCreationStore()
      store.streaming = true
      store.streamingContent = 'partial'

      const wrapper = mount(MessageStream, { global: { stubs } })

      expect(wrapper.get('.studio-message-stream').attributes('aria-busy')).toBe('true')
      const streamingArticle = wrapper.get('.studio-message-assistant')
      expect(streamingArticle.attributes('aria-busy')).toBe('true')
      expect(streamingArticle.attributes('aria-label')).toBe('studio.a11y.assistantStreaming')
    })
  })

  describe('ComposerBar', () => {
    it('labels the textarea input', () => {
      const wrapper = mount(ComposerBar, { global: { stubs } })
      const textarea = wrapper.get('textarea')
      expect(textarea.attributes('aria-label')).toBe('studio.a11y.composerInput')
    })

    it('marks the send button busy while streaming', async () => {
      const store = useCreationStore()
      store.streaming = true

      const wrapper = mount(ComposerBar, { global: { stubs } })
      await nextTick()

      const button = wrapper.get('.studio-composer-send')
      expect(button.attributes('aria-busy')).toBe('true')
    })

    it('announces the send error as an assertive alert', async () => {
      const store = useCreationStore()
      store.error = 'Failed to send message.'

      const wrapper = mount(ComposerBar, { global: { stubs } })
      await nextTick()

      const alert = wrapper.get('[role="alert"]')
      expect(alert.attributes('aria-live')).toBe('assertive')
      expect(alert.text()).toBe('Failed to send message.')
    })
  })

  describe('PreviewDialog', () => {
    function mountDialog(open: boolean) {
      return mount(PreviewDialog, {
        attachTo: document.body,
        props: { open, imageUrl: 'https://cdn.example/preview.png' },
        global: {
          stubs: {
            ...stubs,
            Transition: { props: ['name'], template: '<slot />' },
          },
        },
      })
    }

    it('renders as an accessible modal dialog with a labeled close button', async () => {
      const wrapper = mountDialog(true)
      await flushPromises()
      await nextTick()

      const dialog = document.body.querySelector('[role="dialog"]')
      expect(dialog).not.toBeNull()
      expect(dialog?.getAttribute('aria-modal')).toBe('true')

      const closeButton = document.body.querySelector('.ui-modal-close')
      expect(closeButton?.getAttribute('aria-label')).toBe('studio.a11y.closePreview')

      wrapper.unmount()
    })

    it('traps focus inside the dialog while open', async () => {
      const wrapper = mountDialog(true)
      await flushPromises()
      await nextTick()

      const dialog = document.body.querySelector('[role="dialog"]') as HTMLElement
      expect(dialog).not.toBeNull()

      const closeButton = document.body.querySelector('.ui-modal-close') as HTMLElement
      expect(document.activeElement).toBe(closeButton)

      window.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }),
      )
      await nextTick()
      expect(document.activeElement).toBe(closeButton)

      wrapper.unmount()
    })

    it('closes on Escape', async () => {
      const wrapper = mountDialog(true)
      await flushPromises()
      await nextTick()

      window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
      await nextTick()

      expect(wrapper.emitted('close')).toBeTruthy()
      wrapper.unmount()
    })
  })
})
