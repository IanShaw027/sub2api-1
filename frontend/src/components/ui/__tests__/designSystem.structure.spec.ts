import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { expectedUiFiles, listUiVueFiles, readUi, styleCss, tokensCss } from './source'
import Button from '../Button.vue'
import SegmentedControl from '../SegmentedControl.vue'
import ToggleSwitch from '../ToggleSwitch.vue'
import Checkbox from '../Checkbox.vue'
import StatusBadge from '../StatusBadge.vue'

describe('ui design system structure', () => {
  it('exports the planned component set', () => {
    const files = listUiVueFiles()
    for (const name of expectedUiFiles) {
      expect(files, name).toContain(name)
    }
  })

  it('ui vue files avoid legacy gray utility classes', () => {
    const banned = [/bg-gray-/, /text-gray-/, /border-gray-/, /dark:/]
    for (const file of listUiVueFiles()) {
      const src = readUi(file)
      for (const pattern of banned) {
        expect(src, `${file} matched ${pattern}`).not.toMatch(pattern)
      }
    }
  })

  it('core components use semantic glass classes', () => {
    expect(readUi('GlassCard.vue')).toContain('glass-card')
    expect(readUi('Button.vue')).toContain('btn-glass-primary')
    expect(readUi('Button.vue')).toContain('btn-glass-secondary')
    expect(readUi('StatusBadge.vue')).toContain('badge-tone-')
    expect(readUi('TextInput.vue')).toContain('class="field"')
    expect(readUi('SegmentedControl.vue')).toContain('segmented-item')
    expect(readUi('EndpointCard.vue')).toContain('<GlassCard')
    expect(readUi('SettingsSection.vue')).toContain('<GlassCard')
    expect(readUi('Fab.vue')).toContain('btn-glass-primary')
    expect(readUi('UiModal.vue')).toContain('glass-card-solid')
  })
})

describe('surfaces', () => {
  it('exposes the card family recipe (glass-card, glass-ring, glass-inset, card-*)', () => {
    expect(styleCss).toMatch(/\.glass-card,[\s\S]*?\.card,[\s\S]*?\.card-glass\s*\{/)
    expect(styleCss).toContain('.glass-card-flat {')
    expect(styleCss).toContain('.glass-ring {')
    expect(styleCss).toContain('.glass-inset {')
    expect(styleCss).toMatch(/\.card-header,\s*\n\s*\.glass-card-header\s*\{/)
    expect(styleCss).toContain('.card-title {')
    expect(styleCss).toContain('.card-subtitle {')
    expect(styleCss).toMatch(/\.card-body,\s*\n\s*\.glass-card-body\s*\{/)
    expect(styleCss).toContain('.card-footer {')
  })

  it('exposes the summary/filter row layout classes so pages stop hand-rolling them', () => {
    expect(styleCss).toContain('.summary-row {')
    expect(styleCss).toContain('.filter-row {')
    expect(styleCss).toContain('.filter-search {')
    expect(styleCss).toContain('.filter-count {')
  })

  it('summary-chip is a 64px glass chip with a tone dot and tabular value', () => {
    expect(styleCss).toMatch(/\.summary-chip\s*\{[^}]*height:\s*64px/)
    expect(styleCss).toMatch(/\.summary-chip-dot\s*\{[^}]*width:\s*6px/)
    expect(styleCss).toMatch(/\.summary-chip-value\s*\{[^}]*font-weight:\s*800/)
  })

  it('modal/dialog surfaces use the foreground scrim, fixed width scale, and a mobile bottom sheet', () => {
    expect(styleCss).toMatch(/\.modal-overlay,[\s\S]*?background:\s*color-mix\(in oklch, var\(--foreground\) 40%/)
    expect(readUi('UiModal.vue')).toContain("sm: '440px'")
    expect(readUi('UiModal.vue')).toContain("xl: '960px'")
    expect(styleCss).toMatch(/max-width:\s*767px\)\s*\{\s*\n\s*\.modal-overlay/)
  })

  it('feedback surfaces (toast/notice/empty-state/skeleton/spinner) are defined once globally', () => {
    expect(styleCss).toContain('.toast {')
    expect(styleCss).toContain('.notice-danger {')
    expect(styleCss).toContain('.empty-state {')
    expect(styleCss).toContain('.skeleton {')
    expect(styleCss).toContain('.spinner {')
    expect(styleCss).toContain('.tooltip-bubble {')
  })
})

describe('controls', () => {
  it('button sizes match the spec (xs 26 r8 · sm 32 r9 · default 34 r10 · md 42)', () => {
    expect(styleCss).toMatch(/\.btn-xs\s*{\s*height:\s*26px;\s*padding:\s*0 9px;\s*border-radius:\s*8px/)
    expect(styleCss).toMatch(/\.btn-sm\s*{\s*height:\s*32px;[\s\S]*?border-radius:\s*9px/)
    expect(styleCss).toMatch(/\.btn-glass-primary[\s\S]*?height:\s*34px/)
    expect(readUi('Button.vue')).toContain('height: 42px')
    expect(readUi('Button.vue')).toContain("classes.push('ui-btn-success')")
    expect(readUi('Button.vue')).toContain("classes.push('ui-btn-warning')")
    expect(readUi('Button.vue')).toContain("classes.push('btn-xs')")
    expect(readUi('Button.vue')).toContain("classes.push('btn-sm')")
    expect(mount(Button, { props: { size: 'xs' } }).classes()).toContain('btn-xs')
    expect(mount(Button, { props: { size: 'sm' } }).classes()).toContain('btn-sm')
    expect(mount(Button, { props: { variant: 'success' } }).classes()).toContain('ui-btn-success')
    expect(mount(Button, { props: { variant: 'warning' } }).classes()).toContain('ui-btn-warning')
  })

  it('icon buttons are 34×34 r10 (header) and 28×28 r8 (table)', () => {
    expect(styleCss).toMatch(/\.icon-btn\s*{[\s\S]*?width:\s*28px;\s*height:\s*28px;[\s\S]*?border-radius:\s*8px/)
    expect(styleCss).toMatch(/\.header-icon-btn\s*{[\s\S]*?width:\s*34px;\s*height:\s*34px;[\s\S]*?border-radius:\s*10px/)
    expect(tokensCss).toContain('--radius-btn: 10px')
  })

  it('field / textarea / input-lg geometry matches the spec (36 r12, textarea min 96 padding 10 12, lg 40)', () => {
    expect(styleCss).toMatch(/\.field\s*{[\s\S]*?height:\s*36px;[\s\S]*?border-radius:\s*var\(--radius-field\)/)
    expect(tokensCss).toContain('--radius-field: 12px')
    expect(styleCss).toMatch(/textarea\.input,\s*\n\s*textarea\.field\s*{[\s\S]*?min-height:\s*96px;\s*\n\s*padding:\s*10px 12px/)
    expect(styleCss).toMatch(/\.input-lg,\s*\n\s*\.field-lg\s*{\s*height:\s*40px/)
  })

  it('focus / error rings use accent 18% and danger 14%', () => {
    expect(styleCss).toMatch(/\.field:focus,[\s\S]*?color-mix\(in oklch, var\(--accent\) 18%, transparent\)/)
    expect(styleCss).toMatch(/\.input-error:focus,\s*\n\s*\.field-error:focus\s*{[\s\S]*?color-mix\(in oklch, var\(--danger\) 14%, transparent\)/)
  })

  it('filter pill is a 36px r12 label/value pill with an active accent state', () => {
    expect(styleCss).toMatch(/\.filter-pill\s*{[\s\S]*?height:\s*36px;[\s\S]*?border-radius:\s*var\(--radius-field\)/)
    expect(styleCss).toMatch(/\.filter-pill\.is-active\s*{\s*border-color:\s*var\(--accent\);\s*\n\s*background:\s*color-mix\(in oklch, var\(--accent\) 10%, transparent\)/)
  })

  it('dropdown panel is r12 p6 surface 92% blur 20 with 36px r9 items and 11/600 uppercase group labels', () => {
    expect(styleCss).toMatch(/\.dropdown,\s*\n\s*\.popover\s*{[\s\S]*?padding:\s*6px;\s*\n\s*border-radius:\s*12px;\s*\n\s*background:\s*color-mix\(in oklch, var\(--surface\) 92%/)
    expect(styleCss).toMatch(/backdrop-filter:\s*blur\(20px\)/)
    expect(styleCss).toMatch(/\.dropdown-item\s*{[\s\S]*?height:\s*36px;[\s\S]*?border-radius:\s*9px/)
    expect(styleCss).toMatch(/\.dropdown-label\s*{[\s\S]*?font-size:\s*11px;[\s\S]*?letter-spacing:\s*0\.06em/)
  })

  it('segmented control is 36 (sm 30) r11 p3 with keyboard arrow navigation', () => {
    expect(styleCss).toMatch(/\.segmented,\s*\n\s*\.tabs\s*{[\s\S]*?height:\s*36px;\s*\n\s*padding:\s*3px;[\s\S]*?border-radius:\s*11px/)
    expect(styleCss).toMatch(/\.segmented-sm\s*{\s*height:\s*30px/)
    expect(readUi('SegmentedControl.vue')).toContain('ArrowLeft')
    expect(readUi('SegmentedControl.vue')).toContain('ArrowRight')

    const wrapper = mount(SegmentedControl, {
      props: {
        modelValue: 'a',
        options: [
          { value: 'a', label: 'A' },
          { value: 'b', label: 'B' }
        ]
      }
    })
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('toggle switch is 36×20 (compact 32×18) with a var(--thumb) thumb', () => {
    expect(styleCss).toMatch(/\.switch-compact\s*{\s*width:\s*32px;\s*\n\s*height:\s*18px/)
    expect(readUi('ToggleSwitch.vue')).toContain('background: var(--thumb)')

    const form = mount(ToggleSwitch, { props: { modelValue: false } })
    expect(form.classes()).toContain('ui-toggle-form')

    const compact = mount(ToggleSwitch, { props: { modelValue: false, size: 'compact' } })
    expect(compact.classes()).toContain('ui-toggle-compact')
  })

  it('checkbox is 16px r5 1.5px border with an indeterminate dash', () => {
    expect(readUi('Checkbox.vue')).toMatch(/\.ui-checkbox-box\s*{\s*width:\s*16px;\s*\n\s*height:\s*16px;\s*\n\s*border-radius:\s*5px;\s*\n\s*border:\s*1\.5px solid/)
    const wrapper = mount(Checkbox, { props: { modelValue: false, indeterminate: true } })
    expect(wrapper.find('.ui-checkbox-dash').exists()).toBe(true)
  })

  it('badges are 22px r999 with five tones and a pulsing live dot', () => {
    expect(styleCss).toMatch(/\.badge,\s*\n\s*\.badge-tone-success,[\s\S]*?height:\s*22px;\s*\n\s*padding:\s*0 8px;\s*\n\s*border-radius:\s*999px/)
    expect(styleCss).toContain('@keyframes s2a-pulse')
    const wrapper = mount(StatusBadge, { props: { tone: 'success', dot: true, pulse: true } })
    expect(wrapper.find('.ui-status-badge-dot-live').exists()).toBe(true)
  })

  it('tag / count-badge geometry matches the spec', () => {
    expect(styleCss).toMatch(/\.tag\s*{[\s\S]*?padding:\s*3px 7px;\s*\n\s*border-radius:\s*6px/)
    expect(styleCss).toMatch(/\.count-badge\s*{[\s\S]*?height:\s*18px;[\s\S]*?font-size:\s*10\.5px;\s*\n\s*font-weight:\s*700/)
  })
})
