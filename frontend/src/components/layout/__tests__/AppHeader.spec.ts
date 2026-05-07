import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppHeader.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('AppHeader support entry', () => {
  it('passes legacy contact info through to the support button fallback', () => {
    expect(componentSource).toContain('const contactInfo = computed(() => appStore.contactInfo)')
    expect(componentSource).toContain('<SupportQRCodesButton :entries="supportQRCodes" :legacy-contact-info="contactInfo" />')
  })
})
