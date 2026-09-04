#!/usr/bin/env node
// zh/en locale key parity + duplicate-key check. Usage: node scripts/i18n-diff.mjs
// Loads the real locale modules through vite-node so nested objects are compared exactly.
import { spawnSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
const ROOT = path.resolve(new URL('..', import.meta.url).pathname)
const tmp = path.join(ROOT, 'node_modules', '.cache', 'i18n-diff.ts')
fs.mkdirSync(path.dirname(tmp), { recursive: true })
fs.writeFileSync(tmp, `
import zhMod from '${ROOT}/src/i18n/locales/zh/index'
import enMod from '${ROOT}/src/i18n/locales/en/index'
const zh:any = (zhMod as any).default ?? zhMod, en:any = (enMod as any).default ?? enMod
function flat(o:any,p=''):string[]{return Object.entries(o).flatMap(([k,v])=>v&&typeof v==='object'?flat(v,p+k+'.'):[p+k])}
const z=new Set(flat(zh)), e=new Set(flat(en))
const zo=[...z].filter(k=>!e.has(k)), eo=[...e].filter(k=>!z.has(k))
console.log(JSON.stringify({zh:z.size,en:e.size,zhOnly:zo,enOnly:eo}))
`)
const r = spawnSync(path.join(ROOT, 'node_modules', '.bin', 'vite-node'), [tmp], { cwd: ROOT, encoding: 'utf8' })
if (r.status !== 0) { console.error(r.stderr || r.stdout); process.exit(2) }
const out = JSON.parse(r.stdout.trim().split('\n').pop())
// duplicate keys inside one object literal: vue-tsc reports TS1117; here a cheap textual check per file
let dups = 0
function scanDups(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name)
    if (e.isDirectory()) scanDups(p)
    else if (e.name.endsWith('.ts')) {
      const stack = [new Map()]
      fs.readFileSync(p, 'utf8').split('\n').forEach((line, i) => {
        const m = line.match(/^\s*['"]?([A-Za-z0-9_-]+)['"]?\s*:\s*(\{)?/)
        const opens = (line.match(/\{/g) || []).length, closes = (line.match(/\}/g) || []).length
        if (m) {
          const cur = stack[stack.length - 1]
          if (cur.has(m[1])) { dups++; console.log(`duplicate key ${m[1]} at ${path.relative(ROOT, p)}:${i + 1} (first at line ${cur.get(m[1])})`) }
          cur.set(m[1], i + 1)
        }
        for (let k = 0; k < opens; k++) stack.push(new Map())
        for (let k = 0; k < closes; k++) if (stack.length > 1) stack.pop()
      })
    }
  }
}
scanDups(path.join(ROOT, 'src/i18n/locales'))
console.log(`zh: ${out.zh}, en: ${out.en}`)
console.log(`zh-only: ${out.zhOnly.length}, en-only: ${out.enOnly.length}, duplicates: ${dups}`)
for (const k of out.zhOnly.slice(0, 50)) console.log('  zh-only ' + k)
for (const k of out.enOnly.slice(0, 50)) console.log('  en-only ' + k)
process.exit(out.zhOnly.length || out.enOnly.length || dups ? 1 : 0)
