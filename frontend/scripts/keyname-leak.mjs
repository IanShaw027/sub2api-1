#!/usr/bin/env node
// keyname-leak.mjs — task 15.5: walk every route in a headless Chrome and flag raw i18n key
// names that leaked into rendered text (e.g. "dashboard.stats.title" showing instead of a label).
//
// Usage: node scripts/keyname-leak.mjs [--routes scripts/ui/routes.txt] [--theme glass-light]
//        [--locale zh] [--width 1440] [--only slug,slug] [--base http://127.0.0.1:3777]
// Route list lines are `<route pattern>|<role>|<actual path>` (see scripts/ui/routes.txt).
// Requires the mock backend (:8091) and Vite (:3777, VITE_DEV_PROXY_TARGET set) to be running.
// Exit code 1 when any leak is found, so it can serve as a gate.

import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import WebSocket from 'ws'

const here = path.dirname(fileURLToPath(import.meta.url))
const args = process.argv.slice(2)
const opt = (name, dflt) => {
  const i = args.indexOf(name)
  return i >= 0 ? args[i + 1] : dflt
}
const ROUTES_FILE = opt('--routes', path.join(here, 'ui', 'routes.txt'))
const THEME = opt('--theme', 'glass-light')
const LOCALE = opt('--locale', 'zh')
const WIDTH = Number(opt('--width', '1440'))
const BASE = opt('--base', process.env.UI_SHOTS_BASE || 'http://127.0.0.1:3777')
const ONLY = opt('--only', '')
  .split(',')
  .map(s => s.trim())
  .filter(Boolean)

// Top-level namespaces of the merged locale object: every raw key starts with one of these.
function loadNamespaces() {
  const dir = path.join(here, '..', 'src', 'i18n', 'locales', LOCALE)
  const ns = new Set()
  for (const f of fs.readdirSync(dir)) {
    // a sub-directory (e.g. `admin/`) is mounted as one nested namespace in index.ts
    if (fs.statSync(path.join(dir, f)).isDirectory()) { ns.add(f); continue }
    if (!f.endsWith('.ts') || f === 'index.ts') continue
    const src = fs.readFileSync(path.join(dir, f), 'utf8')
    for (const m of src.matchAll(/^ {2}([A-Za-z][A-Za-z0-9_]*): \{/gm)) ns.add(m[1])
  }
  return [...ns].sort()
}

const namespaces = loadNamespaces()
// key = ns.segment.segment… ; require ≥ 2 dots so plain "admin.users" style prose does not match,
// and reject anything that looks like a URL/path/file (preceded by "/" or ":" or followed by "/").
const keyRe = new RegExp(`(?<![\\w/:.@-])(?:${namespaces.join('|')})(?:\\.[A-Za-z0-9_]+){2,}(?![\\w/-])`, 'g')

function parseRoutes() {
  return fs
    .readFileSync(ROUTES_FILE, 'utf8')
    .split('\n')
    .map(l => l.trim())
    .filter(l => l && !l.startsWith('#'))
    .map(l => {
      const [pattern, role, actual] = l.split('|').map(s => s.trim())
      const slug = (actual || pattern).replace(/^\//, '').replace(/[^A-Za-z0-9]+/g, '-').replace(/-+$/, '') || 'root'
      return { pattern, role: role || 'guest', path: actual || pattern, slug }
    })
    .filter(r => ONLY.length === 0 || ONLY.includes(r.slug))
}

const sleep = ms => new Promise(r => setTimeout(r, ms))

async function launchChrome() {
  const port = 9900 + Math.floor(Math.random() * 90)
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'cdp-leak-'))
  const chrome = spawn(
    'google-chrome',
    ['--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage', `--remote-debugging-port=${port}`, `--user-data-dir=${profile}`, 'about:blank'],
    { stdio: 'ignore' }
  )
  let target
  for (let i = 0; i < 60 && !target; i++) {
    try {
      const list = await (await fetch(`http://127.0.0.1:${port}/json/list`)).json()
      target = list.find(t => t.type === 'page')
    } catch {
      // devtools endpoint not up yet
    }
    if (!target) await sleep(250)
  }
  if (!target) throw new Error('chrome did not expose a page target')
  const ws = new WebSocket(target.webSocketDebuggerUrl, { perMessageDeflate: false, maxPayload: 64 * 1024 * 1024 })
  let id = 0
  const pending = new Map()
  ws.on('message', d => {
    const m = JSON.parse(d)
    if (m.id && pending.has(m.id)) {
      pending.get(m.id)(m)
      pending.delete(m.id)
    }
  })
  await new Promise(r => ws.on('open', r))
  const send = (method, params = {}) =>
    new Promise(res => {
      const i = ++id
      pending.set(i, res)
      ws.send(JSON.stringify({ id: i, method, params }))
    })
  const close = () => {
    try { ws.close() } catch { /* already closed */ }
    chrome.kill()
    try { fs.rmSync(profile, { recursive: true, force: true }) } catch { /* best effort */ }
  }
  return { send, close }
}

async function main() {
  const routes = parseRoutes()
  console.log(`keyname-leak: ${routes.length} routes, ${namespaces.length} namespaces, theme=${THEME} locale=${LOCALE}`)
  const { send, close } = await launchChrome()
  await send('Page.enable')
  await send('Runtime.enable')
  await send('Emulation.setDeviceMetricsOverride', { width: WIDTH, height: 900, deviceScaleFactor: 1, mobile: WIDTH < 500 })

  let leaks = 0
  let lastRole = null
  for (const r of routes) {
    if (r.role !== lastRole) {
      await send('Page.navigate', { url: `${BASE}/setup/seed?role=${r.role}&theme=${THEME}&locale=${LOCALE}&to=%2F__blank` })
      await sleep(1500)
      lastRole = r.role
    }
    await send('Page.navigate', { url: BASE + r.path })
    await sleep(4000)
    const ev = await send('Runtime.evaluate', {
      returnByValue: true,
      expression: `(() => {
        document.querySelectorAll('vite-plugin-checker-error-overlay,vite-error-overlay').forEach(e => e.remove())
        const attrs = []
        for (const el of document.querySelectorAll('[title],[placeholder],[aria-label]')) {
          for (const a of ['title','placeholder','aria-label']) { const v = el.getAttribute(a); if (v) attrs.push(v) }
        }
        return { text: document.body.innerText, attrs: attrs.join('\\n'), title: document.title }
      })()`
    })
    const v = ev.result && ev.result.result && ev.result.result.value
    if (!v) {
      console.log(`  ?  ${r.path}  (no text)`)
      continue
    }
    const hits = new Set()
    for (const src of [v.text, v.attrs, v.title]) for (const m of src.matchAll(keyRe)) hits.add(m[0])
    if (hits.size) {
      leaks += hits.size
      console.log(`  ✗  ${r.path}  ${[...hits].join(', ')}`)
    } else {
      console.log(`  ✓  ${r.path}`)
    }
  }
  close()
  console.log(`\nleaks: ${leaks}`)
  process.exit(leaks ? 1 : 0)
}

main().catch(e => {
  console.error(e)
  process.exit(2)
})
