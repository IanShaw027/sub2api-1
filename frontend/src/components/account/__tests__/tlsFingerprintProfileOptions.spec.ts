import { describe, expect, it } from 'vitest'
import {
  formatTLSFingerprintProfileOptionLabel,
  getSelectableTLSFingerprintProfiles
} from '../tlsFingerprintProfileOptions'

describe('tlsFingerprintProfileOptions', () => {
  const profiles = [
    { id: 1, name: 'Shared Node', platform: '' },
    { id: 2, name: 'OpenAI Codex CLI', platform: 'openai' },
    { id: 3, name: 'Kiro Desktop', platform: 'kiro' },
    { id: 4, name: 'Anthropic Claude Code', platform: 'anthropic' }
  ]

  it('keeps only shared and same-platform profiles for new selections', () => {
    const selectable = getSelectableTLSFingerprintProfiles(profiles, 'openai', null)

    expect(selectable.map((profile) => profile.id)).toEqual([1, 2])
    expect(selectable.every((profile) => !profile.isPlatformMismatch)).toBe(true)
  })

  it('keeps the currently selected cross-platform profile visible as a mismatch', () => {
    const selectable = getSelectableTLSFingerprintProfiles(profiles, 'openai', 3)

    expect(selectable.map((profile) => profile.id)).toEqual([1, 2, 3])
    expect(selectable.find((profile) => profile.id === 3)?.isPlatformMismatch).toBe(true)
  })

  it('labels shared, platform-specific, and mismatched profiles explicitly', () => {
    const selectable = getSelectableTLSFingerprintProfiles(profiles, 'openai', 3)

    expect(formatTLSFingerprintProfileOptionLabel(selectable[0], {
      shared: 'shared',
      mismatch: 'platform mismatch'
    })).toBe('[shared] Shared Node')
    expect(formatTLSFingerprintProfileOptionLabel(selectable[1], {
      shared: 'shared',
      mismatch: 'platform mismatch'
    })).toBe('[openai] OpenAI Codex CLI')
    expect(formatTLSFingerprintProfileOptionLabel(selectable[2], {
      shared: 'shared',
      mismatch: 'platform mismatch'
    })).toBe('[kiro] Kiro Desktop (platform mismatch)')
  })
})
