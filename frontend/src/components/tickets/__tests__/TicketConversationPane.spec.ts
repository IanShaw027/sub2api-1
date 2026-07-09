import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import TicketConversationPane from '../TicketConversationPane.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      if (key === 'common.submit') return 'Shared Submit'
      if (key === 'common.submitting') return 'Shared Submitting'
      return key
    },
  }),
}))

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value,
}))

describe('TicketConversationPane', () => {
  it('uses shared common i18n labels for default submit and sending copy', () => {
    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
      },
    })

    expect(wrapper.get('button').text()).toBe('Shared Submit')

    const sendingWrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        sending: true,
        replyContent: 'draft',
      },
    })

    expect(sendingWrapper.get('button').text()).toBe('Shared Submitting')
  })

  it('prefers explicit submit and sending label props over shared defaults', async () => {
    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        submitText: 'Custom Submit',
        sendingText: 'Custom Sending',
      },
    })

    expect(wrapper.get('button').text()).toBe('Custom Submit')

    await wrapper.setProps({
      sending: true,
      replyContent: 'draft',
    })

    expect(wrapper.get('button').text()).toBe('Custom Sending')
  })

  it('does not render unsafe avatar snapshot URLs in the conversation', () => {
    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [{
          id: 1,
          ticket_id: 1,
          sender_role: 'admin',
          sender_name_snapshot: 'Admin',
          sender_avatar_snapshot: 'javascript:alert(1)',
          message_type: 'message',
          content: 'hello',
          created_at: '2026-07-08T00:00:00Z',
        }],
      },
    })

    expect(wrapper.find('img[alt="Admin"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('A')
  })

  it('submits on Enter but ignores Shift+Enter and IME composition Enter', async () => {
    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
      },
    })

    const textarea = wrapper.get('textarea')

    await textarea.setValue('first reply')
    await textarea.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('reply')).toEqual([['first reply', undefined]])

    await textarea.setValue('line break')
    await textarea.trigger('keydown', { key: 'Enter', shiftKey: true })
    expect(wrapper.emitted('reply')).toHaveLength(1)

    await textarea.setValue('ime draft')
    await textarea.trigger('compositionstart')
    await textarea.trigger('keydown', { key: 'Enter' })
    await textarea.trigger('compositionend')
    expect(wrapper.emitted('reply')).toHaveLength(1)

    await textarea.setValue('native composing')
    await textarea.trigger('keydown', { key: 'Enter', isComposing: true })
    expect(wrapper.emitted('reply')).toHaveLength(1)
  })

  it('does not submit on Enter while sending or while an attachment upload is in progress', async () => {
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    let uploadResolver!: (value: { id: number; public_url: string; mime_type: string }) => void
    const pendingUpload = new Promise<{ id: number; public_url: string; mime_type: string }>((resolve) => {
      uploadResolver = resolve
    })
    URL.createObjectURL = vi.fn(() => 'blob:pending-ticket-image') as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as typeof URL.revokeObjectURL

    const uploadFn = vi.fn(() => pendingUpload)
    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const textarea = wrapper.get('textarea')
    await textarea.setValue('draft reply')

    await wrapper.setProps({ sending: true })
    await textarea.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('reply')).toBeUndefined()

    await wrapper.setProps({ sending: false })
    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['image'], 'ticket.png', { type: 'image/png' })],
      configurable: true,
    })
    await fileInput.trigger('change')
    await Promise.resolve()

    await textarea.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('reply')).toBeUndefined()

    uploadResolver({
      id: 9,
      public_url: 'https://example.com/ticket.png',
      mime_type: 'image/png',
    })
    await flushPromises()

    URL.createObjectURL = originalCreateObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL
  })

  it('clears the draft and pending attachments when switching to another ticket', async () => {
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    const revokeObjectURL = vi.fn()
    URL.createObjectURL = vi.fn(() => 'blob:ticket-preview') as typeof URL.createObjectURL
    URL.revokeObjectURL = revokeObjectURL as typeof URL.revokeObjectURL

    const uploadFn = vi.fn().mockResolvedValue({
      id: 12,
      public_url: 'https://example.com/ticket.png',
      mime_type: 'image/png',
    })
    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const textarea = wrapper.get('textarea')
    await textarea.setValue('draft reply')

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['image'], 'ticket.png', { type: 'image/png' })],
      configurable: true,
    })
    await fileInput.trigger('change')
    await flushPromises()

    expect(wrapper.html()).toContain('blob:ticket-preview')
    expect((textarea.element as HTMLTextAreaElement).value).toBe('draft reply')

    await wrapper.setProps({ ticketId: 2 })
    await flushPromises()

    expect((textarea.element as HTMLTextAreaElement).value).toBe('')
    expect(wrapper.html()).not.toContain('blob:ticket-preview')
    expect(revokeObjectURL).toHaveBeenCalledWith('blob:ticket-preview')

    URL.createObjectURL = originalCreateObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL
  })

  it('stops queueing the remaining attachment uploads after switching tickets', async () => {
    let uploadResolver!: (value: { id: number; public_url: string; mime_type: string }) => void
    const firstUpload = new Promise<{ id: number; public_url: string; mime_type: string }>((resolve) => {
      uploadResolver = resolve
    })

    const uploadFn = vi
      .fn()
      .mockReturnValueOnce(firstUpload)
      .mockResolvedValueOnce({
        id: 22,
        public_url: 'https://example.com/two.png',
        mime_type: 'image/png',
      })

    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [
        new File(['one'], 'one.png', { type: 'image/png' }),
        new File(['two'], 'two.png', { type: 'image/png' }),
      ],
      configurable: true,
    })

    const uploadPromise = fileInput.trigger('change')
    await Promise.resolve()
    await wrapper.setProps({ ticketId: 2 })
    expect(wrapper.get('input[type="file"]').attributes('disabled')).toBeUndefined()
    uploadResolver({
      id: 11,
      public_url: 'https://example.com/one.png',
      mime_type: 'image/png',
    })
    await uploadPromise
    await flushPromises()

    expect(uploadFn).toHaveBeenCalledTimes(1)
    expect(uploadFn.mock.calls[0]?.[1]).toBe(1)
    expect(wrapper.html()).not.toContain('https://example.com/one.png')
  })

  it('keeps the new ticket upload lock while an old upload finally resolves after a ticket switch', async () => {
    let firstUploadResolver!: (value: { id: number; public_url: string; mime_type: string }) => void
    let secondUploadResolver!: (value: { id: number; public_url: string; mime_type: string }) => void

    const uploadFn = vi
      .fn()
      .mockReturnValueOnce(new Promise<{ id: number; public_url: string; mime_type: string }>((resolve) => {
        firstUploadResolver = resolve
      }))
      .mockReturnValueOnce(new Promise<{ id: number; public_url: string; mime_type: string }>((resolve) => {
        secondUploadResolver = resolve
      }))

    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['one'], 'one.png', { type: 'image/png' })],
      configurable: true,
    })

    await fileInput.trigger('change')
    await Promise.resolve()
    await wrapper.setProps({ ticketId: 2 })
    expect(wrapper.get('input[type="file"]').attributes('disabled')).toBeUndefined()

    Object.defineProperty(wrapper.get('input[type="file"]').element, 'files', {
      value: [new File(['two'], 'two.png', { type: 'image/png' })],
      configurable: true,
    })
    await wrapper.get('input[type="file"]').trigger('change')
    await Promise.resolve()

    firstUploadResolver({
      id: 11,
      public_url: 'https://example.com/one.png',
      mime_type: 'image/png',
    })
    await flushPromises()

    expect(wrapper.get('input[type="file"]').attributes('disabled')).toBeDefined()

    secondUploadResolver({
      id: 12,
      public_url: 'https://example.com/two.png',
      mime_type: 'image/png',
    })
    await flushPromises()
  })

  it('drops stale upload results when switching A -> B -> A before the original upload resolves', async () => {
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    const createObjectURL = vi.fn(() => 'blob:stale-ticket-upload')
    URL.createObjectURL = createObjectURL as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as typeof URL.revokeObjectURL

    let uploadResolver!: (value: { id: number; public_url: string; mime_type: string }) => void
    const uploadFn = vi.fn().mockReturnValueOnce(
      new Promise<{ id: number; public_url: string; mime_type: string }>((resolve) => {
        uploadResolver = resolve
      }),
    )

    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['one'], 'one.png', { type: 'image/png' })],
      configurable: true,
    })

    await fileInput.trigger('change')
    await Promise.resolve()
    await wrapper.setProps({ ticketId: 2 })
    await wrapper.setProps({ ticketId: 1 })
    uploadResolver({
      id: 31,
      public_url: 'https://example.com/one.png',
      mime_type: 'image/png',
    })
    await flushPromises()

    expect(createObjectURL).not.toHaveBeenCalled()
    expect(wrapper.html()).not.toContain('https://example.com/one.png')

    URL.createObjectURL = originalCreateObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL
  })

  it('does not emit upload errors from an old ticket after switching tickets', async () => {
    let rejectUpload!: (reason?: unknown) => void
    const uploadFn = vi.fn().mockReturnValueOnce(new Promise((_, reject) => {
      rejectUpload = reject
    }))

    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['one'], 'one.png', { type: 'image/png' })],
      configurable: true,
    })

    const uploadPromise = fileInput.trigger('change')
    await Promise.resolve()
    await wrapper.setProps({ ticketId: 2 })

    rejectUpload(new Error('stale upload failed'))
    await uploadPromise
    await flushPromises()

    expect(wrapper.emitted('upload-error')).toBeUndefined()
  })

  it('uses a local image preview when the uploaded image has no generated thumbnail', async () => {
    const originalCreateObjectURL = URL.createObjectURL
    const originalRevokeObjectURL = URL.revokeObjectURL
    URL.createObjectURL = vi.fn(() => 'blob:ticket-image-thumbnail-fallback') as typeof URL.createObjectURL
    URL.revokeObjectURL = vi.fn() as typeof URL.revokeObjectURL

    const uploadFn = vi.fn().mockResolvedValue({
      id: 26,
      public_url: 'https://example.com/ticket.png',
      mime_type: 'image/png',
      original_file_name: 'ticket.png',
      size_bytes: 5,
    })

    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['image'], 'ticket.png', { type: 'image/png' })],
      configurable: true,
    })

    await fileInput.trigger('change')
    await flushPromises()

    expect(URL.createObjectURL).toHaveBeenCalledTimes(1)
    expect(wrapper.html()).toContain('blob:ticket-image-thumbnail-fallback')

    wrapper.unmount()
    URL.createObjectURL = originalCreateObjectURL
    URL.revokeObjectURL = originalRevokeObjectURL
  })

  it('uploads non-image attachments and emits their media ids on reply', async () => {
    const originalCreateObjectURL = URL.createObjectURL
    URL.createObjectURL = vi.fn(() => 'blob:should-not-be-used') as typeof URL.createObjectURL

    const uploadFn = vi.fn().mockResolvedValue({
      id: 27,
      url: 'https://example.com/notes.txt',
      mime_type: 'text/plain',
    })

    const wrapper = mount(TicketConversationPane, {
      props: {
        title: 'Conversation',
        emptyText: 'Empty',
        messages: [],
        uploadFn,
        ticketId: 1,
      },
    })

    const fileInput = wrapper.get('input[type="file"]')
    Object.defineProperty(fileInput.element, 'files', {
      value: [new File(['hello'], 'notes.txt', { type: 'text/plain' })],
      configurable: true,
    })

    expect(fileInput.attributes('accept')).toBeUndefined()

    await fileInput.trigger('change')
    await flushPromises()

    expect(uploadFn).toHaveBeenCalledTimes(1)
    expect(URL.createObjectURL).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('txt')
    expect(wrapper.get('button.btn-primary').attributes('disabled')).toBeUndefined()

    await wrapper.get('button.btn-primary').trigger('click')

    expect(wrapper.emitted('reply')).toEqual([['', [{ media_id: 27 }]]])

    URL.createObjectURL = originalCreateObjectURL
  })
})
