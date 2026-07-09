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

describe('AppHeader document links', () => {
  it('uses separate settings for help docs and tool downloads', () => {
    expect(componentSource).toContain("cachedPublicSettings?.doc_url")
    expect(componentSource).toContain("cachedPublicSettings?.download_tools_url")
    expect(componentSource).toContain('const downloadToolsUrl = computed(() => (appStore.cachedPublicSettings?.download_tools_url || appStore.downloadToolsUrl || \'\').trim())')
    expect(componentSource).not.toContain('`${base}/cli`')
  })
})

describe('AppHeader avatar rendering', () => {
  it('sanitizes avatar URLs and adds privacy-preserving image attributes', () => {
    expect(componentSource).toContain("import { safeImageUrl } from '@/utils/safeImageUrl'")
    expect(componentSource).toContain("const avatarUrl = computed(() => safeImageUrl(user.value?.avatar_url))")
    expect(componentSource).toContain('referrerpolicy="no-referrer"')
    expect(componentSource).toContain('loading="lazy"')
  })
})
