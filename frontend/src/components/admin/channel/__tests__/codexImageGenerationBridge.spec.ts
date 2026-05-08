import { describe, expect, it } from 'vitest'

import {
  codexImageGenerationBridgeModeFromOverride,
  codexImageGenerationBridgeOverrideFromMode,
} from '../codexImageGenerationBridge'

describe('codexImageGenerationBridge helpers', () => {
  it('maps overrides to tri-state modes', () => {
    expect(codexImageGenerationBridgeModeFromOverride(true)).toBe('enabled')
    expect(codexImageGenerationBridgeModeFromOverride(false)).toBe('disabled')
    expect(codexImageGenerationBridgeModeFromOverride(null)).toBe('inherit')
    expect(codexImageGenerationBridgeModeFromOverride(undefined)).toBe('inherit')
  })

  it('maps tri-state modes back to overrides', () => {
    expect(codexImageGenerationBridgeOverrideFromMode('enabled')).toBe(true)
    expect(codexImageGenerationBridgeOverrideFromMode('disabled')).toBe(false)
    expect(codexImageGenerationBridgeOverrideFromMode('inherit')).toBeNull()
  })
})
