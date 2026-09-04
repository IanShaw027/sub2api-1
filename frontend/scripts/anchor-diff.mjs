#!/usr/bin/env node
// Functional-anchor diff: makes sure a refactor did not drop interactions/text keys.
// Usage: node scripts/anchor-diff.mjs <file.vue> [--base <rev>] [--scope <dir>]
//   Compares the anchor SET found in <file> at --base (default: 65529a248, pre-redesign)
//   against the anchors found in the working tree across <file> + --scope (default: the
//   file's sibling components directory) so anchors moved into sub-components still count.
//   Anchors: data-tour / data-testid / id= / aria-label / @handler names / v-model targets / t('key').
import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

const ROOT = path.resolve(new URL('..', import.meta.url).pathname)
const REPO = path.resolve(ROOT, '..')
const args = process.argv.slice(2)
const file = args.find((a) => !a.startsWith('--'))
if (!file) { console.error('usage: anchor-diff.mjs <file.vue> [--base rev] [--scope dir ...]'); process.exit(2) }
const opt = (n, d) => { const i = args.indexOf(n); return i >= 0 ? args[i + 1] : d }
const base = opt('--base', '65529a248')
const scopes = args.flatMap((a, i) => (a === '--scope' ? [args[i + 1]] : []))

const RES = {
  'data-tour': /data-tour="([^"]+)"/g,
  'data-testid': /data-testid="([^"]+)"/g,
  id: /\sid="([^"]+)"/g,
  'aria-label': /(?::aria-label|aria-label)="([^"]+)"/g,
  handler: /@[a-zA-Z:.-]+="([A-Za-z_$][\w$]*)/g,
  'v-model': /v-model(?::[\w-]+)?="([^"]+)"/g,
  't(key)': /\bt\(\s*'([^']+)'/g
}
const collect = (text) => {
  const out = {}
  for (const [k, re] of Object.entries(RES)) out[k] = new Set([...text.matchAll(re)].map((m) => m[1].trim()))
  return out
}
const relRepo = path.relative(REPO, path.resolve(file)).split(path.sep).join('/')
let baseText = ''
try { baseText = execFileSync('git', ['show', `${base}:${relRepo}`], { cwd: REPO, encoding: 'utf8', maxBuffer: 1 << 26 }) }
catch { console.error(`cannot read ${relRepo} at ${base}`); process.exit(2) }
const walk = (d, out = []) => { for (const e of fs.readdirSync(d, { withFileTypes: true })) { const p = path.join(d, e.name); e.isDirectory() ? walk(p, out) : /\.(vue|ts)$/.test(e.name) && !/__tests__/.test(p) && out.push(p) } return out }
const nowFiles = [path.resolve(file), ...scopes.flatMap((s) => walk(path.resolve(s)))]
const nowText = nowFiles.map((f) => (fs.existsSync(f) ? fs.readFileSync(f, 'utf8') : '')).join('\n')
const before = collect(baseText), after = collect(nowText)
let lost = 0
for (const k of Object.keys(RES)) {
  const missing = [...before[k]].filter((a) => !after[k].has(a))
  console.log(`${k}: base ${before[k].size} → now ${after[k].size}, lost ${missing.length}`)
  for (const m of missing) console.log(`  - ${m}`)
  lost += missing.length
}
console.log(lost ? `LOST ${lost} anchors` : 'no anchors lost')
process.exit(lost ? 1 : 0)
