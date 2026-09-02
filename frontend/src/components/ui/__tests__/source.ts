import { readFileSync, readdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const testsDir = dirname(fileURLToPath(import.meta.url))

export const uiDir = resolve(testsDir, '..')
export const styleCss = readFileSync(resolve(testsDir, '../../../style.css'), 'utf8')
export const tokensCss = readFileSync(resolve(testsDir, '../../../styles/tokens.css'), 'utf8')

export function readUi(name: string) {
  return readFileSync(resolve(uiDir, name), 'utf8')
}

export function listUiVueFiles() {
  return readdirSync(uiDir).filter((name) => name.endsWith('.vue'))
}

export const expectedUiFiles = [
  'GlassCard.vue',
  'Button.vue',
  'StatusBadge.vue',
  'FieldLabel.vue',
  'TextInput.vue',
  'UiSelect.vue',
  'ToggleSwitch.vue',
  'Checkbox.vue',
  'SegmentedControl.vue',
  'ProgressBar.vue',
  'PageHeader.vue',
  'StatCard.vue',
  'FilterBar.vue',
  'SettingsSection.vue',
  'SettingRow.vue',
  'EndpointCard.vue',
  'UiModal.vue',
  'UiDrawer.vue',
  'UiPagination.vue',
  'ChipScroller.vue',
  'Fab.vue',
  'MiniStatCard.vue',
  'ListFade.vue'
]
