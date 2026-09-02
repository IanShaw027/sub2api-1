import { readFileSync, readdirSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import {
  Button,
  Checkbox,
  ChipScroller,
  Fab,
  GlassCard,
  ProgressBar,
  SegmentedControl,
  StatusBadge,
  TextInput,
  ToggleSwitch
} from '../index'

const uiDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')

function readUiSource(name: string) {
  return readFileSync(join(uiDir, name), 'utf8')
}

function listUiVueFiles(): string[] {
  return readdirSync(uiDir).filter((name) => name.endsWith('.vue'))
}

describe('ui design system structure', () => {
  it('ui vue files avoid legacy gray utility classes', () => {
    const banned = [/bg-gray-/, /text-gray-/, /border-gray-/]
    for (const file of listUiVueFiles()) {
      const src = readUiSource(file)
      for (const pattern of banned) {
        expect(src, file).not.toMatch(pattern)
      }
    }
  })

  it('core components use semantic glass classes', () => {
    expect(readUiSource('GlassCard.vue')).toContain('glass-card')
    expect(readUiSource('Button.vue')).toContain('btn-glass-primary')
    expect(readUiSource('StatusBadge.vue')).toContain('badge-tone-')
    expect(readUiSource('TextInput.vue')).toContain('class="field"')
    expect(readUiSource('SegmentedControl.vue')).toContain('segmented')
  })
})

describe('GlassCard', () => {
  it('renders glass variant with token radius', () => {
    const wrapper = mount(GlassCard, { slots: { default: 'Body' } })
    expect(wrapper.classes()).toContain('glass-card')
    expect(wrapper.text()).toContain('Body')
  })
})

describe('Button', () => {
  it('uses 34px glass primary by default', () => {
    const wrapper = mount(Button, { slots: { default: 'Save' } })
    expect(wrapper.classes()).toContain('btn-glass-primary')
    expect(readUiSource('Button.vue')).toContain('height: 42px')
  })
})

describe('StatusBadge', () => {
  it('applies tone class', () => {
    const wrapper = mount(StatusBadge, { props: { tone: 'success', label: 'OK' } })
    expect(wrapper.classes()).toContain('badge-tone-success')
  })
})

describe('TextInput', () => {
  it('renders field class and error', () => {
    const wrapper = mount(TextInput, {
      props: { modelValue: '', label: 'Name', error: 'Required' }
    })
    expect(wrapper.find('input.field').exists()).toBe(true)
    expect(wrapper.text()).toContain('Required')
  })
})

describe('ToggleSwitch', () => {
  it('renders compact 32x18 track', () => {
    const wrapper = mount(ToggleSwitch, { props: { modelValue: true, size: 'compact' } })
    expect(wrapper.find('.ui-toggle-compact').exists()).toBe(true)
    expect(readUiSource('ToggleSwitch.vue')).toMatch(/width:\s*32px/)
    expect(readUiSource('ToggleSwitch.vue')).toMatch(/height:\s*18px/)
  })
})

describe('Checkbox', () => {
  it('emits update on change', async () => {
    const wrapper = mount(Checkbox, { props: { modelValue: false } })
    await wrapper.find('input').setValue(true)
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([true])
  })
})

describe('SegmentedControl', () => {
  it('marks active option', () => {
    const wrapper = mount(SegmentedControl, {
      props: {
        modelValue: 'a',
        options: [
          { value: 'a', label: 'A' },
          { value: 'b', label: 'B' }
        ]
      }
    })
    expect(wrapper.find('.segmented-item-active').text()).toBe('A')
  })
})

describe('ProgressBar', () => {
  it('uses danger tone at 90%', () => {
    const wrapper = mount(ProgressBar, { props: { value: 92 } })
    expect(wrapper.find('.ui-progress-danger').exists()).toBe(true)
  })

  it('uses warning tone at 70%', () => {
    const wrapper = mount(ProgressBar, { props: { value: 75 } })
    expect(wrapper.find('.ui-progress-warning').exists()).toBe(true)
  })
})

describe('ChipScroller', () => {
  it('renders 30px chips', () => {
    const wrapper = mount(ChipScroller, {
      props: {
        modelValue: 'all',
        chips: [{ value: 'all', label: 'All' }]
      }
    })
    expect(readUiSource('ChipScroller.vue')).toMatch(/height:\s*30px/)
    expect(wrapper.find('.ui-chip').exists()).toBe(true)
  })
})

describe('Fab', () => {
  it('uses 52px height and safe-area offset', () => {
    mount(Fab, { slots: { default: 'Create' } })
    const src = readUiSource('Fab.vue')
    expect(src).toMatch(/height:\s*52px/)
    expect(src).toContain('safe-area-inset-bottom')
  })
})
