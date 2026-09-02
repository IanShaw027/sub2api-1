import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import EndpointCard from '../EndpointCard.vue'
import { readUi } from './source'

describe('EndpointCard', () => {
  it('renders label, mono URL, copy control, and description', async () => {
    const wrapper = mount(EndpointCard, {
      props: {
        label: 'API 端点 · Anthropic 协议',
        url: 'https://api.sub2api.dev',
        description: 'Claude Code uses /v1/messages',
        badge: '默认'
      }
    })
    expect(wrapper.classes()).toContain('glass-card')
    expect(wrapper.text()).toContain('API 端点 · Anthropic 协议')
    expect(wrapper.find('.ui-endpoint-card-url').text()).toBe('https://api.sub2api.dev')
    expect(wrapper.text()).toContain('Claude Code uses /v1/messages')
    await wrapper.get('.ui-endpoint-card-copy').trigger('click')
    expect(wrapper.emitted('copy')?.[0]).toEqual(['https://api.sub2api.dev'])
  })

  it('uses a 28px copy button', () => {
    expect(readUi('EndpointCard.vue')).toContain('height: 28px')
    expect(readUi('EndpointCard.vue')).toContain('var(--font-mono)')
  })
})
