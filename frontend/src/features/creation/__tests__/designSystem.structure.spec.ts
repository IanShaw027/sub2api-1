import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '..')

const vueFiles = [
  'StudioPage.vue',
  'components/SessionList.vue',
  'components/MessageStream.vue',
  'components/MessageContent.vue',
  'components/TokenStats.vue',
  'components/ComposerBar.vue',
  'components/ModelMenu.vue',
  'components/TaskGrid.vue',
  'components/TaskCard.vue',
  'components/PreviewDialog.vue',
]

function read(rel: string) {
  return readFileSync(resolve(root, rel), 'utf8')
}

describe('creation feature design system structure', () => {
  for (const file of vueFiles) {
    it(`${file} avoids legacy gray/dark utility classes`, () => {
      const src = read(file)
      expect(src).not.toMatch(/\bbg-gray-/)
      expect(src).not.toMatch(/\btext-gray-/)
      expect(src).not.toMatch(/\bborder-gray-/)
      expect(src).not.toMatch(/\bdark:/)
    })
  }
})
