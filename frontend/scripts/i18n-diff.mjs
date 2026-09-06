#!/usr/bin/env node
// zh/en locale key parity + duplicate-key check. Usage: node scripts/i18n-diff.mjs
// Loads the real locale modules through Vite so nested objects are compared exactly.
import { createServer } from 'vite'
import { createServer as createHttpServer } from 'node:http'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
const ROOT = fileURLToPath(new URL('..', import.meta.url))
const server = await createServer({
  root: ROOT,
  configFile: false,
  cacheDir: path.join(ROOT, 'node_modules/.cache/i18n-diff-vite'),
  optimizeDeps: { noDiscovery: true, include: [] },
  server: { middlewareMode: true, hmr: { server: createHttpServer() } },
  appType: 'custom'
})
let out
try {
  const zh = (await server.ssrLoadModule('/src/i18n/locales/zh/index.ts')).default
  const en = (await server.ssrLoadModule('/src/i18n/locales/en/index.ts')).default
  const flat = (o, p = '') => Object.entries(o).flatMap(([k, v]) =>
    v && typeof v === 'object' ? flat(v, p + k + '.') : [p + k])
  const z = new Set(flat(zh)), e = new Set(flat(en))
  out = { zh: z.size, en: e.size, zhOnly: [...z].filter(k => !e.has(k)), enOnly: [...e].filter(k => !z.has(k)) }
} finally {
  await server.close()
}
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
