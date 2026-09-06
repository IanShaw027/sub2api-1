import { readdirSync, readFileSync, statSync } from 'node:fs'
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
})
