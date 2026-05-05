import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const viewPath = resolve(dirname(fileURLToPath(import.meta.url)), '../SettingsView.vue')
const viewSource = readFileSync(viewPath, 'utf8')

describe('SettingsView affiliate feature card structure', () => {
  it('declares the affiliate feature toggle card exactly once and keeps the advanced settings fields in that card', () => {
    const affiliateToggleBindings = viewSource.match(/<Toggle v-model="form\.affiliate_enabled" \/>/g) ?? []

    expect(affiliateToggleBindings).toHaveLength(1)
    expect(viewSource).toContain('form.affiliate_rebate_freeze_hours')
    expect(viewSource).toContain('form.affiliate_rebate_duration_days')
    expect(viewSource).toContain('form.affiliate_rebate_per_invitee_cap')
  })
})
