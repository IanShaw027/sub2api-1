import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import TicketConversationPane from '../TicketConversationPane.vue'

vi.mock('@/utils/format', () => ({
  formatDateTime: (value: string) => value,
}))

describe('TicketConversationPane', () => {
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
    expect(wrapper.emitted('reply')).toEqual([['first reply']])

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
