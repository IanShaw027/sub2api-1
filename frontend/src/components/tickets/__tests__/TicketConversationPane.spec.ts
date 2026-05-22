import { mount } from '@vue/test-utils'
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
})
