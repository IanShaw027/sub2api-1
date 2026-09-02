import { describe, expect, it } from 'vitest'
import { expectedUiFiles, listUiVueFiles, readUi } from './source'

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
