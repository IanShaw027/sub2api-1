#!/usr/bin/env node
/**
 * Validate WCAG contrast ratios for the glass design tokens
 * (frontend/src/styles/tokens.css), for both the light and dark themes.
 *
 * Usage: node scripts/check-contrast.js
 * Exits non-zero if any critical pair fails its threshold.
 */

import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { compositeTint, contrastRatio, contrastRatioLinear, loadThemeTokens, oklchToLinearSRGB } from './contrast-utils.js'

const here = dirname(fileURLToPath(import.meta.url))
const tokensPath = resolve(here, '../src/styles/tokens.css')

// [foreground token, background token, minimum ratio, description]
// Text pairs use the WCAG AA "normal text" threshold (4.5:1).
// Non-text (borders / focus indicators / UI component boundaries) use the
// WCAG AA "non-text contrast" threshold (3:1), per SC 1.4.11.
const TEXT_PAIRS = [
  ['foreground', 'canvas', 4.5, 'body text on page canvas'],
  ['foreground', 'background', 4.5, 'body text on page background'],
  ['foreground', 'surface', 4.5, 'body text on card surface (.studio-message, GlassCard)'],
  ['foreground', 'surface-secondary', 4.5, 'body text on secondary surface'],
  ['muted', 'canvas', 4.5, 'muted text (text-muted) on canvas'],
  ['muted', 'background', 4.5, 'muted text (text-muted) on background'],
  ['muted', 'surface', 4.5, 'muted text (text-muted) on card surface'],
  ['danger-text', 'canvas', 4.5, 'error text (text-danger) on canvas'],
  ['danger-text', 'background', 4.5, 'error text (text-danger) on background'],
  ['danger-text', 'surface', 4.5, 'error text (text-danger) on card surface']
]

const NON_TEXT_PAIRS = [
  ['border', 'canvas', 3, 'border vs page canvas'],
  ['border', 'background', 3, 'border vs page background (ComposerBar input, .field, .input)'],
  ['border', 'surface', 3, 'border vs card surface (GlassCard, SessionList item)'],
  ['accent', 'canvas', 3, 'focus ring / accent vs canvas'],
  ['accent', 'background', 3, 'focus ring / accent vs background']
]

// Badge tones (src/style.css `.badge-*`): text token on `color-mix(tone alpha%, transparent)`
// composited over each surface the badge can sit on. Badges are 11–12px text → 4.5:1.
// [text token, tint tone token, tint alpha, description]
const BADGE_TONES = [
  ['info-text', 'accent', 0.12, 'badge-info / badge-primary'],
  ['success-text', 'success', 0.16, 'badge-success'],
  ['warning-text', 'warning', 0.18, 'badge-warning'],
  ['danger-text', 'danger', 0.14, 'badge-danger (danger-text on danger 14%)'],
  ['muted', null, 0, 'badge-muted (muted on surface-secondary)']
]
const BADGE_BACKDROPS = ['surface', 'background', 'canvas']

// Tooltip (`.tooltip-bubble`): --background text on --foreground bubble.
const TOOLTIP_PAIRS = [['background', 'foreground', 4.5, 'tooltip text on tooltip bubble']]

function runBadges(tokens) {
  let failures = 0
  for (const [textTok, toneTok, alpha, desc] of BADGE_TONES) {
    for (const backdrop of BADGE_BACKDROPS) {
      const text = tokens[textTok]
      const back = toneTok ? tokens[backdrop] : tokens['surface-secondary']
      if (!text || !back || (toneTok && !tokens[toneTok])) {
        console.log(`  SKIP  badge ${desc} (token not defined)`)
        continue
      }
      const bg = toneTok ? compositeTint(tokens[toneTok], alpha, back) : oklchToLinearSRGB(...back)
      const ratio = contrastRatioLinear(oklchToLinearSRGB(...text), bg)
      const pass = ratio >= 4.5
      if (!pass) failures += 1
      console.log(
        `  ${pass ? 'PASS' : 'FAIL'}  ${`badge ${desc}`.padEnd(46)} on --${backdrop.padEnd(12)} = ${fmt(ratio).padEnd(8)} (min 4.5:1)`
      )
      if (!toneTok) break
    }
  }
  return failures
}

function fmt(n) {
  return `${n.toFixed(2)}:1`
}

function runTheme(name, tokens) {
  console.log(`\n${name}`)
  console.log('-'.repeat(name.length))
  let failures = 0
  const rows = [
    ...TEXT_PAIRS.map((p) => [...p, 'text']),
    ...TOOLTIP_PAIRS.map((p) => [...p, 'text']),
    ...NON_TEXT_PAIRS.map((p) => [...p, 'non-text'])
  ]

  for (const [fg, bg, min, desc] of rows) {
    const fgColor = tokens[fg]
    const bgColor = tokens[bg]
    if (!fgColor || !bgColor) {
      console.log(`  SKIP  --${fg} vs --${bg} (token not defined)`)
      continue
    }
    const ratio = contrastRatio(fgColor, bgColor)
    const pass = ratio >= min
    if (!pass) failures += 1
    const status = pass ? 'PASS' : 'FAIL'
    console.log(
      `  ${status}  --${fg.padEnd(18)} vs --${bg.padEnd(18)} = ${fmt(ratio).padEnd(8)} (min ${min}:1)  ${desc}`
    )
  }
  failures += runBadges(tokens)
  return failures
}

const { light, dark } = loadThemeTokens(tokensPath)

let totalFailures = 0
totalFailures += runTheme('glass-dark', dark)
totalFailures += runTheme('glass-light', light)

console.log()
if (totalFailures > 0) {
  console.error(`✗ ${totalFailures} contrast check(s) failed.`)
  process.exit(1)
} else {
  console.log('✓ All contrast checks passed.')
}
