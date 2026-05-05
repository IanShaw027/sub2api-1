import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const typesPath = resolve(dirname(fileURLToPath(import.meta.url)), '../types/index.ts')
const typesSource = readFileSync(typesPath, 'utf8')

describe('PublicSettings type shape', () => {
  it('declares affiliate_enabled only once', () => {
    const affiliateEnabledFields = typesSource.match(/affiliate_enabled: boolean/g) ?? []

    expect(affiliateEnabledFields).toHaveLength(1)
  })
})
