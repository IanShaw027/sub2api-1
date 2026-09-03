import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import CallbackStatusCard from '@/components/auth/CallbackStatusCard.vue'

describe('CallbackStatusCard', () => {
  it('renders a loading state with a spinner and no icon', () => {
    const wrapper = mount(CallbackStatusCard, {
      props: {
        status: 'loading',
        title: 'Signing you in',
        description: 'Please wait a moment'
      }
    })

    expect(wrapper.attributes('data-status')).toBe('loading')
    expect(wrapper.find('.callback-status-spinner').exists()).toBe(true)
    expect(wrapper.find('svg').exists()).toBe(false)
    expect(wrapper.find('.callback-status-title').text()).toBe('Signing you in')
    expect(wrapper.find('.callback-status-desc').text()).toBe('Please wait a moment')
    expect(wrapper.find('.code-block').exists()).toBe(false)
  })

  it('renders a success state with a check icon', () => {
    const wrapper = mount(CallbackStatusCard, {
      props: {
        status: 'success',
        title: 'Login successful'
      }
    })

    expect(wrapper.attributes('data-status')).toBe('success')
    expect(wrapper.find('.callback-status-spinner').exists()).toBe(false)
    expect(wrapper.find('svg').exists()).toBe(true)
    expect(wrapper.find('.callback-status-desc').exists()).toBe(false)
  })

  it('renders an error state with the detail in a code block and slotted actions', () => {
    const wrapper = mount(CallbackStatusCard, {
      props: {
        status: 'error',
        title: 'Login failed',
        description: 'access_denied',
        detail: 'error=access_denied&state=abc123'
      },
      slots: {
        default: '<button type="button" data-testid="retry">Retry</button>'
      }
    })

    expect(wrapper.attributes('data-status')).toBe('error')
    expect(wrapper.find('.callback-status-desc').text()).toBe('access_denied')
    expect(wrapper.find('.code-block').text()).toBe('error=access_denied&state=abc123')
    expect(wrapper.find('[data-testid="retry"]').exists()).toBe(true)
  })

  it('omits the actions wrapper when no default slot content is provided', () => {
    const wrapper = mount(CallbackStatusCard, {
      props: { status: 'loading', title: 'Loading' }
    })

    expect(wrapper.find('.callback-status-actions').exists()).toBe(false)
  })
})
