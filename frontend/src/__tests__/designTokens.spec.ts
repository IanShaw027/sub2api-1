import { readdirSync, readFileSync, statSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

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
})
