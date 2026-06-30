import { describe, expect, it } from 'vitest'
import {
  formatTLSFingerprintProfileOptionLabel,
  getSelectableTLSFingerprintProfiles,
  getTLSFingerprintProfilesForDimension
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



  it('keeps OS-only binding slots restricted to profiles without client_type unless selected', () => {
    const dimensionProfiles = [
      { id: 1, name: 'Windows OS-only', platform: 'openai', os: 'windows', client_type: '' },
      { id: 2, name: 'Windows Codex', platform: 'openai', os: 'windows', client_type: 'codex' },
      { id: 3, name: 'Windows Shared Client', platform: 'openai', os: 'windows', client_type: null },
      { id: 4, name: 'macOS OS-only', platform: 'openai', os: 'macos', client_type: '' }
    ]

    expect(getTLSFingerprintProfilesForDimension(
      dimensionProfiles,
      { platform: 'openai', os: 'windows', clientType: '' },
      null
    ).map((profile) => profile.id)).toEqual([1, 3])
    expect(getTLSFingerprintProfilesForDimension(
      dimensionProfiles,
      { platform: 'openai', os: 'windows' },
      null
    ).map((profile) => profile.id)).toEqual([1, 3])
    expect(getTLSFingerprintProfilesForDimension(
      dimensionProfiles,
      { platform: 'openai', os: 'windows', clientType: '' },
      2
    ).map((profile) => profile.id)).toEqual([1, 2, 3])
  })

  it('matches backend transport semantics: empty binding context keeps transport-specific profiles', () => {
    const transportProfiles = [
      { id: 1, name: 'Shared Any Transport', platform: 'openai', transport: '' },
      { id: 2, name: 'HTTP/2 Only', platform: 'openai', transport: 'h2' },
      { id: 3, name: 'WebSocket Only', platform: 'openai', transport: 'websocket-h2' }
    ]

    expect(getTLSFingerprintProfilesForDimension(
      transportProfiles,
      { platform: 'openai', os: 'windows' },
      null
    ).map((profile) => profile.id)).toEqual([1, 2, 3])
    expect(getTLSFingerprintProfilesForDimension(
      transportProfiles,
      { platform: 'openai', os: 'windows', transport: 'websocket' },
      null
    ).map((profile) => profile.id)).toEqual([1, 3])
    expect(getTLSFingerprintProfilesForDimension(
      transportProfiles,
      { platform: 'openai', os: 'windows' },
      2
    ).map((profile) => profile.id)).toEqual([1, 2, 3])

    expect(getSelectableTLSFingerprintProfiles(
      transportProfiles,
      'openai',
      null
    ).map((profile) => profile.id)).toEqual([1, 2, 3])
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
