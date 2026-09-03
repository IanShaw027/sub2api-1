/**
 * WCAG contrast-ratio math for this project's OKLCH design tokens
 * (frontend/src/styles/tokens.css).
 *
 * We deliberately avoid a runtime color library (e.g. `polished`): this
 * project's tokens are authored directly as `oklch(L% C H)` strings and
 * `polished`'s contrast helpers only accept hex/rgb, so using it would still
 * require hand-rolling an OKLCH -> sRGB conversion first. The conversion
 * below implements the CSS Color 4 algorithm (OKLab matrices from
 * Björn Ottosson) end to end, so it is self-contained and has zero
 * dependencies.
 */

import { readFileSync } from 'node:fs'

/** Convert OKLCH (L: 0-1, C, H in degrees) to *linear* sRGB. */
export function oklchToLinearSRGB(L, C, Hdeg) {
  const h = (Hdeg * Math.PI) / 180
  const a = C * Math.cos(h)
  const b = C * Math.sin(h)

  const l_ = L + 0.3963377774 * a + 0.2158037573 * b
  const m_ = L - 0.1055613458 * a - 0.0638541728 * b
  const s_ = L - 0.0894841775 * a - 1.2914855480 * b

  const l = l_ ** 3
  const m = m_ ** 3
  const s = s_ ** 3

  const r = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s
  const g = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s
  const bb = -0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s
  return [r, g, bb]
}

function linearToGamma(c) {
  const clamped = Math.min(1, Math.max(0, c))
  return clamped <= 0.0031308 ? 12.92 * clamped : 1.055 * clamped ** (1 / 2.4) - 0.055
}

function gammaToLinear(c) {
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
}

/** WCAG relative luminance of a (possibly out-of-gamut) linear sRGB triple. */
export function relativeLuminance([r, g, b]) {
  // Clamp through gamma space first, matching how a browser displays an
  // out-of-gamut oklch() color, then re-linearize for the luminance formula.
  const R = gammaToLinear(linearToGamma(r))
  const G = gammaToLinear(linearToGamma(g))
  const B = gammaToLinear(linearToGamma(b))
  return 0.2126 * R + 0.7152 * G + 0.0722 * B
}

/** WCAG 2.x contrast ratio between two OKLCH colors, each as [L, C, H]. */
export function contrastRatio(oklchA, oklchB) {
  const la = relativeLuminance(oklchToLinearSRGB(...oklchA))
  const lb = relativeLuminance(oklchToLinearSRGB(...oklchB))
  const lighter = Math.max(la, lb)
  const darker = Math.min(la, lb)
  return (lighter + 0.05) / (darker + 0.05)
}

/** Parse a `oklch(L% C H)` (or `oklch(L% C H / A)`) string into [L, C, H]. */
export function parseOklch(str) {
  const m = String(str)
    .trim()
    .match(/oklch\(\s*([\d.]+)%\s+([\d.]+)\s+([\d.]+)/)
  if (!m) return null
  return [parseFloat(m[1]) / 100, parseFloat(m[2]), parseFloat(m[3])]
}

/** Extract the `{ ... }` body that follows the first occurrence of `selector` in `source`. */
export function cssBlock(source, selector) {
  const idx = source.indexOf(selector)
  if (idx < 0) throw new Error(`selector not found: ${selector}`)
  const open = source.indexOf('{', idx)
  let depth = 0
  for (let i = open; i < source.length; i++) {
    if (source[i] === '{') depth += 1
    if (source[i] === '}') {
      depth -= 1
      if (depth === 0) return source.slice(open + 1, i)
    }
  }
  throw new Error(`unbalanced braces for selector: ${selector}`)
}

/** Read a `--name: value;` declaration out of a CSS block. Follows one `var(--x)` indirection. */
export function tokenValue(block, name) {
  const match = block.match(new RegExp(`${name}\\s*:\\s*([^;]+);`))
  const raw = match?.[1]?.trim()
  if (!raw) return undefined
  const varMatch = raw.match(/^var\((--[\w-]+)\)$/)
  if (varMatch) return tokenValue(block, varMatch[1])
  return raw
}

/**
 * Load frontend/src/styles/tokens.css and return { light, dark } maps of
 * token name -> parsed [L, C, H], for every oklch() token defined on the
 * `[data-theme='glass-light']` and `[data-theme='glass-dark']` blocks.
 */
export function loadThemeTokens(tokensPath) {
  const source = readFileSync(tokensPath, 'utf8')
  const lightBlock = cssBlock(source, "[data-theme='glass-light']")
  const darkBlock = cssBlock(source, "[data-theme='glass-dark']")

  const names = [
    'canvas',
    'background',
    'foreground',
    'surface',
    'surface-secondary',
    'surface-tertiary',
    'muted',
    'border',
    'accent',
    'success',
    'success-text',
    'warning',
    'warning-text',
    'danger',
    'danger-text'
  ]

  const build = (block) => {
    const out = {}
    for (const name of names) {
      const value = tokenValue(block, `--${name}`)
      const parsed = value ? parseOklch(value) : null
      if (parsed) out[name] = parsed
    }
    return out
  }

  return { light: build(lightBlock), dark: build(darkBlock), source }
}
