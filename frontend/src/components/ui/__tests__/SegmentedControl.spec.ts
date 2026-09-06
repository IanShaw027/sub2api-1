import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'
import SegmentedControl from '../SegmentedControl.vue'
import { styleCss } from './source'

describe('SegmentedControl', () => {
  const options = [
    { value: 'all', label: 'All' },
    { value: 'on', label: 'On' },
    { value: 'off', label: 'Off', disabled: true }
  ]

  it('marks the active item and emits changes', async () => {
    const wrapper = mount(SegmentedControl, {
      props: { modelValue: 'all', options }
    })
    expect(wrapper.classes()).toContain('segmented')
    expect(wrapper.find('.segmented-item-active').text()).toBe('All')
    await wrapper.findAll('.segmented-item')[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['on'])
  })

  it('ignores disabled options and uses 36px track', async () => {
    const wrapper = mount(SegmentedControl, {
      props: { modelValue: 'all', options }
    })
    await wrapper.findAll('.segmented-item')[2].trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(styleCss).toMatch(/\.segmented[\s\S]{0,40}?\{[\s\S]*?height:\s*36px/)
  })
})

describe('SegmentedControl keyboard navigation', () => {
  const wrappers: VueWrapper[] = []

  afterEach(() => wrappers.splice(0).forEach((wrapper) => wrapper.unmount()))

  function mountControlled(initial = 'all') {
    const value = ref(initial)
    const wrapper = mount(defineComponent({
      setup: () => () => h(SegmentedControl, {
        modelValue: value.value,
        options: [
          { value: 'all', label: 'All' },
          { value: 'disabled', label: 'Disabled', disabled: true },
          { value: 'chat', label: 'Chat' }
        ],
        'onUpdate:modelValue': (next: string) => { value.value = next }
      })
    }), { attachTo: document.body })
    wrappers.push(wrapper)
    return { value, wrapper, buttons: wrapper.findAll('button') }
  }

  it('moves focus with selection, skips disabled options, and wraps at either end', async () => {
    const { value, buttons } = mountControlled()
    const first = buttons[0].element as HTMLButtonElement
    first.focus()
    await buttons[0].trigger('keydown', { key: 'ArrowRight' })
    await nextTick()
    expect(value.value).toBe('chat')
    expect(document.activeElement).toBe(buttons[2].element)
    expect(buttons[2].attributes('tabindex')).toBe('0')
    expect(buttons[0].attributes('tabindex')).toBe('-1')

    // Native Enter/Space activates the focused button, not the previous choice.
    const activeButton = document.activeElement as HTMLButtonElement
    activeButton.click()
    await nextTick()
    expect(value.value).toBe('chat')

    await buttons[2].trigger('keydown', { key: 'ArrowDown' })
    await nextTick()
    expect(value.value).toBe('all')
    expect(document.activeElement).toBe(first)
    await buttons[0].trigger('keydown', { key: 'ArrowLeft' })
    await nextTick()
    expect(value.value).toBe('chat')
    expect(document.activeElement).toBe(buttons[2].element)
  })

  it.each(['removed', 'disabled'])('retains an enabled Tab entry when selected value is %s', async (initial) => {
    const { value, buttons } = mountControlled(initial)
    expect(buttons.filter((button) => button.attributes('tabindex') === '0')).toHaveLength(1)
    expect(buttons[0].attributes('tabindex')).toBe('0')
    expect(buttons[1].attributes('tabindex')).toBe('-1')
    const first = buttons[0].element as HTMLButtonElement
    first.focus()
    await buttons[0].trigger('keydown', { key: 'ArrowRight' })
    await nextTick()
    expect(value.value).toBe('chat')
    expect(document.activeElement).toBe(buttons[2].element)
  })

  it('supports Home/End and leaves an all-disabled group inert', async () => {
    const { value, buttons } = mountControlled()
    await buttons[0].trigger('keydown', { key: 'End' })
    await nextTick()
    expect(value.value).toBe('chat')
    expect(document.activeElement).toBe(buttons[2].element)
    await buttons[2].trigger('keydown', { key: 'Home' })
    await nextTick()
    expect(value.value).toBe('all')
    expect(document.activeElement).toBe(buttons[0].element)

    const allDisabled = mount(SegmentedControl, {
      props: { modelValue: 'all', options: [{ value: 'all', label: 'All', disabled: true }] }
    })
    wrappers.push(allDisabled)
    expect(allDisabled.get('button').attributes('tabindex')).toBe('-1')
    await allDisabled.get('[role="radiogroup"]').trigger('keydown', { key: 'ArrowRight' })
    expect(allDisabled.emitted('update:modelValue')).toBeUndefined()
  })
})
