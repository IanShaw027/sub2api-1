import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
// contrast-utils.js lives under frontend/scripts, outside src/ — it has no
// project dependencies so importing it directly from a spec is safe.
import { contrastRatio, loadThemeTokens } from '../../scripts/contrast-utils.js'

const tokensPath = resolve(dirname(fileURLToPath(import.meta.url)), '../styles/tokens.css')
const { light, dark } = loadThemeTokens(tokensPath)

const AA_TEXT = 4.5
const AA_NON_TEXT = 3

describe('color contrast (WCAG AA)', () => {
  describe('glass-dark theme', () => {
    it('foreground text meets 4.5:1 on canvas, background, and surfaces', () => {
      expect(contrastRatio(dark.foreground, dark.canvas)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(dark.foreground, dark.background)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(dark.foreground, dark.surface)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(dark.foreground, dark['surface-secondary'])).toBeGreaterThanOrEqual(
        AA_TEXT
      )
    })

    it('text-muted meets 4.5:1 on canvas, background, and surface', () => {
      expect(contrastRatio(dark.muted, dark.canvas)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(dark.muted, dark.background)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(dark.muted, dark.surface)).toBeGreaterThanOrEqual(AA_TEXT)
    })

    it('text-danger meets 4.5:1 on canvas, background, and surface', () => {
      expect(contrastRatio(dark['danger-text'], dark.canvas)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(dark['danger-text'], dark.background)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(dark['danger-text'], dark.surface)).toBeGreaterThanOrEqual(AA_TEXT)
    })

    it('borders (ComposerBar input, .field, GlassCard, SessionList) meet 3:1 against their backgrounds', () => {
      expect(contrastRatio(dark['border-strong'], dark.canvas)).toBeGreaterThanOrEqual(AA_NON_TEXT)
      expect(contrastRatio(dark['border-strong'], dark.background)).toBeGreaterThanOrEqual(AA_NON_TEXT)
      expect(contrastRatio(dark['border-strong'], dark.surface)).toBeGreaterThanOrEqual(AA_NON_TEXT)
    })

    it('the accent focus ring meets 3:1 against canvas and background', () => {
      expect(contrastRatio(dark.accent, dark.canvas)).toBeGreaterThanOrEqual(AA_NON_TEXT)
      expect(contrastRatio(dark.accent, dark.background)).toBeGreaterThanOrEqual(AA_NON_TEXT)
    })
  })

  describe('glass-light theme', () => {
    it('foreground text meets 4.5:1 on canvas, background, and surfaces', () => {
      expect(contrastRatio(light.foreground, light.canvas)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(light.foreground, light.background)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(light.foreground, light.surface)).toBeGreaterThanOrEqual(AA_TEXT)
    })

    it('text-muted meets 4.5:1 on canvas, background, and surface', () => {
      expect(contrastRatio(light.muted, light.canvas)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(light.muted, light.background)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(light.muted, light.surface)).toBeGreaterThanOrEqual(AA_TEXT)
    })

    it('text-danger meets 4.5:1 on canvas, background, and surface', () => {
      expect(contrastRatio(light['danger-text'], light.canvas)).toBeGreaterThanOrEqual(AA_TEXT)
      expect(contrastRatio(light['danger-text'], light.background)).toBeGreaterThanOrEqual(
        AA_TEXT
      )
      expect(contrastRatio(light['danger-text'], light.surface)).toBeGreaterThanOrEqual(AA_TEXT)
    })
  })

  // Known gaps, tracked so a future token change doesn't silently regress
  // further and so they show up in an intentional, documented place rather
  // than as a surprise contrast-script failure. These were out of scope for
  // the dark-mode contrast pass (see frontend/scripts/check-contrast.js) and
  // need a light-theme design decision (raising --border and/or --accent
  // lightness) before they can be tightened to AA.
  describe('glass-light theme (known gaps, not yet AA)', () => {
    it('--border against canvas is still below the 3:1 non-text minimum', () => {
      const ratio = contrastRatio(light.border, light.canvas)
      expect(ratio).toBeLessThan(AA_NON_TEXT)
    })

    it('--accent against canvas is just below the 3:1 non-text minimum', () => {
      const ratio = contrastRatio(light.accent, light.canvas)
      expect(ratio).toBeLessThan(AA_NON_TEXT)
      expect(ratio).toBeGreaterThan(2.5)
    })
  })
})
