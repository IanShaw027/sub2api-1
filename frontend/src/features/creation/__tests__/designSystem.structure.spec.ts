import { readdirSync, readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const root = resolve(__dirname, '..')

const vueFiles = [
  'StudioPage.vue',
  ...readdirSync(resolve(root, 'components')).filter(file => file.endsWith('.vue')).map(file => `components/${file}`),
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
      expect(src).not.toMatch(/\bdark:[^\s'"`]/)
      expect(src).not.toMatch(/var\(--(?:shadow-card|surface-[23]|on-accent)\b/)
    })
  }
  it('uses the shared application shell and does not define an independent navigation bar', () => {
    const page = read('StudioPage.vue')
    expect(page).toContain('<AppLayout fill-height>')
    expect(page).toContain('<PageHeader')
    expect(page).not.toContain('creation-header')
    expect(page).not.toContain('creation-modes')
  })
})
