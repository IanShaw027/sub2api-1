import { spawnSync } from 'node:child_process'
import { mkdtempSync, readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import postcss from 'postcss'
import tailwindcss from 'tailwindcss'
import tailwindConfig from '../../tailwind.config.js'

const srcDir = resolve(dirname(fileURLToPath(import.meta.url)), '..')
const tokensPath = resolve(srcDir, 'styles/tokens.css')
const indexHtmlPath = resolve(srcDir, '../index.html')
const tokensSource = readFileSync(tokensPath, 'utf8')

const REQUIRED_TOKENS = [
  '--canvas',
  '--background',
  '--foreground',
  '--surface',
  '--surface-secondary',
  '--surface-tertiary',
  '--muted',
  '--border',
  '--accent',
  '--success',
  '--success-text',
  '--warning',
  '--warning-text',
  '--danger',
  '--danger-text',
  '--code-bg',
  '--shadow',
  '--shadow-hover',
  '--btn-hi',
  '--field-shadow',
  '--display',
  '--radius-sm',
  '--radius-btn',
  '--radius-field',
  '--radius-card',
  '--radius-hero'
]

function cssBlock(source: string, selector: string): string {
  const idx = source.indexOf(selector)
  expect(idx).toBeGreaterThanOrEqual(0)
  const open = source.indexOf('{', idx)
  expect(open).toBeGreaterThan(idx)
  let depth = 0
  for (let i = open; i < source.length; i++) {
    if (source[i] === '{') depth += 1
    if (source[i] === '}') {
      depth -= 1
      if (depth === 0) return source.slice(open + 1, i)
    }
  }
  return ''
}

function tokenValue(block: string, name: string): string | undefined {
  const match = block.match(new RegExp(`${name}\\s*:\\s*([^;]+);`))
  return match?.[1]?.trim()
}

function collectSourceFiles(dir: string, acc: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    const stat = statSync(full)
    if (stat.isDirectory()) {
      if (entry === 'node_modules' || entry === 'dist') continue
      collectSourceFiles(full, acc)
    } else if (/\.(vue|ts|js|css|html)$/.test(entry)) {
      acc.push(full)
    }
  }
  return acc
}

describe('design tokens', () => {
  it('defines required token names on glass-light and glass-dark', () => {
    expect(tokensSource).toContain(':root')
    expect(tokensSource).toContain('[data-theme=\'glass-light\']')
    expect(tokensSource).toContain('[data-theme=\'glass-dark\']')

    for (const token of REQUIRED_TOKENS) {
      expect(tokensSource).toContain(token)
    }
  })

  it('defines accent variants', () => {
    for (const accent of ['sky', 'indigo', 'teal', 'violet']) {
      expect(tokensSource).toContain(`[data-accent='${accent}']`)
    }
  })

  it('defines --accent, --surface, and --border in both theme blocks', () => {
    const light = cssBlock(tokensSource, "[data-theme='glass-light']")
    const dark = cssBlock(tokensSource, "[data-theme='glass-dark']")
    for (const token of ['--accent', '--surface', '--border']) {
      expect(light).toContain(`${token}:`)
      expect(dark).toContain(`${token}:`)
    }
  })

  it('defines --success, --warning, and --danger in both theme blocks', () => {
    const light = cssBlock(tokensSource, "[data-theme='glass-light']")
    const dark = cssBlock(tokensSource, "[data-theme='glass-dark']")
    for (const token of ['--success', '--warning', '--danger']) {
      expect(tokenValue(light, token)).toBeTruthy()
      expect(tokenValue(dark, token)).toBeTruthy()
      expect(tokenValue(dark, token)).not.toMatch(/^var\(/)
    }
  })

  it('overrides --accent differently across at least two data-accent themes', () => {
    const sky = tokenValue(cssBlock(tokensSource, "[data-accent='sky']"), '--accent')
    const indigo = tokenValue(cssBlock(tokensSource, "[data-accent='indigo']"), '--accent')
    expect(sky).toBeTruthy()
    expect(indigo).toBeTruthy()
    expect(sky).not.toBe(indigo)
  })

  it('matches the prototype oklch value for every data-accent override', () => {
    const ACCENT_OVERRIDES: Record<string, string> = {
      sky: 'oklch(58.76% 0.1389 241.98)',
      indigo: 'oklch(58.54% 0.2041 277.12)',
      teal: 'oklch(60.02% 0.1039 184.73)',
      violet: 'oklch(60.56% 0.219 292.72)'
    }
    for (const [accent, expected] of Object.entries(ACCENT_OVERRIDES)) {
      const block = cssBlock(tokensSource, `[data-accent='${accent}']`)
      expect(tokenValue(block, '--accent'), accent).toBe(expected)
    }
  })

  it('matches the prototype light-theme oklch values (documented deviations excepted)', () => {
    const light = cssBlock(tokensSource, "[data-theme='glass-light']")
    // NOTE: --muted is intentionally 50% L, not the prototype's 55.17% L — see
    // deviations.md ("--muted 亮色"): 55.17% fails 4.5:1 against --surface-secondary
    // / --surface-tertiary. 50% is the minimum safe value.
    const LIGHT_TOKENS: Record<string, string> = {
      '--background': 'oklch(97.02% 0.0015 262.89)',
      '--foreground': 'oklch(21% 0.012 262)',
      '--surface': 'oklch(100% 0 0)',
      '--border': 'oklch(90% 0.003 259.82)',
      '--muted': 'oklch(50% 0.006 259.82)',
      '--accent': 'oklch(62.31% 0.1881 259.82)',
      '--success': 'oklch(73.29% 0.1946 151.55)',
      '--warning': 'oklch(78.19% 0.1593 73.04)',
      '--danger': 'oklch(65.32% 0.2342 26.45)'
    }
    for (const [token, expected] of Object.entries(LIGHT_TOKENS)) {
      expect(tokenValue(light, token), token).toBe(expected)
    }
  })

  it('matches the prototype dark-theme oklch values (documented deviations excepted)', () => {
    const dark = cssBlock(tokensSource, "[data-theme='glass-dark']")
    // Decorative borders follow the prototype; stronger control boundaries use
    // --border-strong and are covered by the contrast suite.
    // --accent/--success/--warning/--danger are redeclared identically to
    // light here: the prototype marks them "同" (same as light).
    const DARK_TOKENS: Record<string, string> = {
      '--background': 'oklch(12% 0.0015 262.89)',
      '--foreground': 'oklch(95% 0.004 262)',
      '--surface': 'oklch(21.03% 0.003 262.89)',
      '--border': 'oklch(28% 0.003 259.82)',
      '--muted': 'oklch(70.5% 0.006 259.82)',
      '--accent': 'oklch(62.31% 0.1881 259.82)',
      '--success': 'oklch(73.29% 0.1946 151.55)',
      '--warning': 'oklch(78.19% 0.1593 73.04)',
      '--danger': 'oklch(65.32% 0.2342 26.45)'
    }
    for (const [token, expected] of Object.entries(DARK_TOKENS)) {
      expect(tokenValue(dark, token), token).toBe(expected)
    }
  })

  it('derives dark --success-text and --warning-text from --success/--warning, and pins --danger-text', () => {
    const dark = cssBlock(tokensSource, "[data-theme='glass-dark']")
    expect(tokenValue(dark, '--success-text')).toBe('var(--success)')
    expect(tokenValue(dark, '--warning-text')).toBe('var(--warning)')
    expect(tokenValue(dark, '--danger-text')).toBe('oklch(72% 0.2 26.45)')
  })

  it('does not reference Google Fonts CSS in src or index.html', () => {
    const googleFontsHost = ['fonts', 'googleapis', 'com'].join('.')
    const files = [...collectSourceFiles(srcDir), indexHtmlPath]
    for (const file of files) {
      const source = readFileSync(file, 'utf8')
      expect(source, file).not.toContain(googleFontsHost)
    }
  })

  it('does not use the retired dark class selector', () => {
    for (const file of collectSourceFiles(srcDir)) {
      const source = readFileSync(file, 'utf8')
      expect(source, file).not.toMatch(/(?:^|[\s,{])(?:html|:root)?\.dark(?:[\s>:{.,#]|$)/m)
    }
  })

  it('does not use Tailwind variants inside @apply', () => {
    for (const file of collectSourceFiles(srcDir)) {
      const source = readFileSync(file, 'utf8')
      expect(source, file).not.toMatch(/@apply[^;\n]*\b(?:supports-|dark:)/)
    }
  })

  it('generates real CSS for semantic opacity utilities and table selection', async () => {
    const classes = [
      'bg-surface/95', 'bg-surface-2/80', 'text-background/70', 'border-line/50',
      'ring-background/10', 'bg-accent/60', 'shadow-accent/25', 'bg-danger/20',
      'hover:bg-surface/10', 'bg-[color-mix(in_oklch,var(--accent)_4.8%,transparent)]'
    ]
    const result = await postcss([tailwindcss({
      ...tailwindConfig,
      content: [{ raw: classes.join(' ') }]
    })]).process('@tailwind utilities;', { from: undefined })
    const rules = new Map<string, string>()
    result.root.walkRules((rule) => { rules.set(rule.selector, rule.toString()) })
    for (const className of classes) {
      if (className.startsWith('bg-[')) continue
      const escaped = className.replace(/([^a-zA-Z0-9_-])/g, '\\$1')
      const selector = `.${escaped}${className.startsWith('hover:') ? ':hover' : ''}`
      expect(rules.has(selector), className).toBe(true)
    }
    expect(rules.get('.bg-surface\\/95')).toContain('var(--surface) calc(0.95 * 100%)')
    expect(rules.get('.border-line\\/50')).toContain('var(--border) calc(0.5 * 100%)')
    const backgrounds: string[] = []
    result.root.walkDecls('background-color', (declaration) => { backgrounds.push(declaration.value) })
    expect(backgrounds).toContain('color-mix(in oklch,var(--accent) 4.8%,transparent)')
  })

  it('does not append unsupported opacity to arbitrary CSS-variable colors', () => {
    for (const file of collectSourceFiles(srcDir)) {
      const source = readFileSync(file, 'utf8')
      expect(source, file).not.toMatch(
        /\b(?:bg|text|border|ring|shadow)-\[(?:var\(|color-mix\()[^\]\n]+\]\/[\d[.]/,
      )
    }
  })

  it('authorizes the prepaint theme script with the backend CSP nonce placeholder', () => {
    const html = new DOMParser().parseFromString(readFileSync(indexHtmlPath, 'utf8'), 'text/html')
    const themeScript = Array.from(html.querySelectorAll('script')).find((script) =>
      script.textContent?.includes('root.dataset.theme')
    )
    expect(themeScript?.getAttribute('nonce')).toBe('__CSP_NONCE_VALUE__')
  })
  it('passes scripts/ui-lint.mjs --scoped --palette on the whole tree (task 15.1)', () => {
    // legacy classes, raw colour literals, raw Tailwind palette classes and non-layout declarations
    // in views/** scoped styles — the same gate as `npm run lint:ui`, so a stray `text-red-500`
    // fails the unit suite instead of only the manual lint step.
    const r = spawnSync(process.execPath, [resolve(srcDir, '../scripts/ui-lint.mjs'), '--scoped', '--palette'], {
      cwd: resolve(srcDir, '..'),
      encoding: 'utf8',
    })
    expect(r.stdout + r.stderr).toMatch(/total: 0/)
    expect(r.status).toBe(0)
  })

  it('distinguishes dark utility classes from theme configuration keys', () => {
    const directory = mkdtempSync(join(tmpdir(), 'sub2api-ui-lint-'))
    const fixture = join(directory, 'theme.vue')
    try {
      for (const [source, expected] of [
        ["const themes = { light: 'github-light', dark: 'github-dark' }", 0],
        ['<div class="dark:bg-surface"></div>', 1],
      ] as const) {
        writeFileSync(fixture, source)
        const result = spawnSync(process.execPath, [resolve(srcDir, '../scripts/ui-lint.mjs'), fixture], { encoding: 'utf8' })
        expect(result.status, result.stdout + result.stderr).toBe(expected)
      }
    } finally {
      rmSync(directory, { recursive: true, force: true })
    }
  })
})
