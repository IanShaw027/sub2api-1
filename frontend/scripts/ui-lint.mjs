#!/usr/bin/env node
// Glass UI static consistency scan (openspec glass-ui-quality-gates).
// Usage: node scripts/ui-lint.mjs [--scoped] [--json] [paths...]
//   default : legacy class patterns + colour literals outside the whitelist
//   --scoped: additionally scan views/** <style scoped> for non-layout declarations
// Exit code 1 when any hit is found.
import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(new URL('..', import.meta.url).pathname)
const SRC = path.join(ROOT, 'src')
const args = process.argv.slice(2)
const scoped = args.includes('--scoped')
const asJson = args.includes('--json')
const targets = args.filter((a) => !a.startsWith('--'))

const LEGACY = [
  /\bdark:/, /\bbg-white\b/, /\bborder-gray-/, /\btext-gray-/, /\bbg-gray-/,
  /\brounded-2xl\b/, /\brounded-3xl\b/, /\bshadow-lg\b/, /\bshadow-md\b/, /\bshadow-xl\b/, /\bshadow-2xl\b/,
  /\btext-lg font-semibold\b/, /fonts\.googleapis\.com/, /fonts\.gstatic\.com/
]
const COLOR = /#(?:[0-9a-fA-F]{3,4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})\b|\brgba?\(|\bhsla?\(/g
const COLOR_WHITELIST = [
  /^style\.css$/, /^styles\/tokens\.css$/, /^utils\/platformTile\.ts$/, /^components\/icons\//,
  /^components\/common\/PlatformIcon\.vue$/, /^components\/common\/ModelIcon\.vue$/, /^i18n\/locales\//,
  /^components\/payment\/.*Brand/, /^components\/auth\/.*(Brand|OAuth|LinuxDo|WeChat)/i
]
const SCOPED_WHITELIST = [/^views\/HomeView\.vue$/, /^views\/auth\//]
const SCOPED_PROPS = /^\s*(color|background(?:-color)?|border-radius|box-shadow|font-size|font-weight)\s*:/

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) { if (e.name !== '__tests__' && e.name !== 'node_modules') walk(p, out) }
    else if (/\.(vue|ts|css)$/.test(e.name) && !/\.(spec|test)\.ts$/.test(e.name)) out.push(p)
  }
  return out
}
const files = targets.length ? targets.map((t) => path.resolve(t)) : walk(SRC)
const hits = { legacy: [], color: [], scoped: [] }
const rel = (f) => path.relative(SRC, f).split(path.sep).join('/')

for (const file of files) {
  const r = rel(file)
  const text = fs.readFileSync(file, 'utf8')
  const lines = text.split('\n')
  lines.forEach((line, i) => {
    for (const re of LEGACY) { const m = line.match(re); if (m) hits.legacy.push({ file: r, line: i + 1, match: m[0] }) }
    if (!COLOR_WHITELIST.some((w) => w.test(r))) {
      const trimmed = line.replace(/\/\/.*$/, '').replace(/\/\*.*?\*\//g, '').replace(/<!--.*?-->/g, '')
      for (const m of trimmed.matchAll(COLOR)) {
        // ignore url()/id anchors like href="#foo" and CSS custom-property fallbacks that reference tokens
        const before = trimmed.slice(Math.max(0, m.index - 6), m.index)
        if (/href="$|url\($|["'#]$/.test(before) && m[0].startsWith('#') && /^#[a-z]/i.test(m[0]) && !/^#[0-9a-f]{3,8}$/i.test(m[0])) continue
        hits.color.push({ file: r, line: i + 1, match: m[0] })
      }
    }
  })
  if (scoped && r.startsWith('views/') && !SCOPED_WHITELIST.some((w) => w.test(r))) {
    const styleRe = /<style[^>]*scoped[^>]*>([\s\S]*?)<\/style>/g
    for (const m of text.matchAll(styleRe)) {
      const offset = text.slice(0, m.index).split('\n').length
      m[1].split('\n').forEach((line, i) => {
        if (SCOPED_PROPS.test(line) && !/var\(--/.test(line) && !/inherit|transparent|currentColor|none|unset/.test(line)) {
          hits.scoped.push({ file: r, line: offset + i, match: line.trim() })
        }
      })
    }
  }
}

const total = hits.legacy.length + hits.color.length + hits.scoped.length
if (asJson) console.log(JSON.stringify({ total, ...hits }, null, 2))
else {
  for (const [k, list] of Object.entries(hits)) {
    console.log(`${k}: ${list.length}`)
    for (const h of list.slice(0, 200)) console.log(`  ${h.file}:${h.line}  ${h.match}`)
    if (list.length > 200) console.log(`  … ${list.length - 200} more`)
  }
  console.log(`total: ${total}`)
}
process.exitCode = total ? 1 : 0
