// Usage: node scripts/ui/shot.cjs <name> <route> [w] [h] [role=admin|user|guest] [theme] [locale] [full]
// Requires: mock backend (scripts/mock/server.js, :8091) + `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091 vite --port 3777`.
// Output dir: $UI_SHOTS_DIR (default frontend/.shots). Dev server: $UI_SHOTS_BASE (default http://127.0.0.1:3777).
const WebSocket = require('ws')
const { spawn } = require('child_process')
const fs = require('fs')
const os = require('os')
const path = require('path')

const [name, route, w = '1440', h = '1000', role = 'admin', theme = 'light', locale = 'zh', full = '0'] = process.argv.slice(2)
const OUT_DIR = process.env.UI_SHOTS_DIR || path.join(__dirname, '..', '..', '.shots')
const DEV = process.env.UI_SHOTS_BASE || 'http://127.0.0.1:3777'
const OUT = path.join(OUT_DIR, name + '.png')
fs.mkdirSync(path.dirname(OUT), { recursive: true })
const port = 9500 + Math.floor(Math.random() * 400)
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'cdp-'))
const chrome = spawn('google-chrome', ['--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage',
  '--hide-scrollbars', `--user-data-dir=${profile}`, `--window-size=${w},${h}`, `--remote-debugging-port=${port}`, 'about:blank'],
  { stdio: 'ignore' })

const sleep = ms => new Promise(r => setTimeout(r, ms))
async function getTarget() {
  for (let i = 0; i < 60; i++) {
    try {
      const r = await fetch(`http://127.0.0.1:${port}/json/list`)
      const list = await r.json()
      const t = list.find(x => x.type === 'page')
      if (t) return t
    } catch {}
    await sleep(250)
  }
  throw new Error('no target')
}

;(async () => {
  const t = await getTarget()
  const ws = new WebSocket(t.webSocketDebuggerUrl, { perMessageDeflate: false, maxPayload: 512 * 1024 * 1024 })
  let id = 0
  const pending = new Map()
  ws.on('message', d => {
    const m = JSON.parse(d)
    if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id) }
  })
  await new Promise(r => ws.on('open', r))
  const send = (method, params = {}) => new Promise(res => { const i = ++id; pending.set(i, res); ws.send(JSON.stringify({ id: i, method, params })) })

  await send('Page.enable')
  await send('Runtime.enable')
  await send('Emulation.setDeviceMetricsOverride', { width: +w, height: +h, deviceScaleFactor: 1, mobile: +w < 500 })
  const seed = `${DEV}/setup/seed?role=${role}&theme=${theme}&locale=${locale}&to=%2F__blank`
  await send('Page.navigate', { url: seed })
  await sleep(1500)
  await send('Page.navigate', { url: DEV + route })
  await sleep(6000)
  // kill dev overlays
  await send('Runtime.evaluate', { expression: `
    document.querySelectorAll('vite-plugin-checker-error-overlay,vite-error-overlay').forEach(e=>e.remove());
    const s=document.createElement('style');s.textContent='vite-plugin-checker-error-overlay,vite-error-overlay{display:none!important}';document.head.appendChild(s);
  ` })
  await sleep(600)
  // Optional probe: UI_SHOTS_PROBE='sel1|sel2' prints top/left/height of the first match of each selector.
  if (process.env.UI_SHOTS_PROBE) {
    const sels = JSON.stringify(process.env.UI_SHOTS_PROBE.split('|'))
    const pr = await send('Runtime.evaluate', { returnByValue: true, expression: `(${sels}).map(s=>{const e=document.querySelector(s);if(!e)return s+': (none)';const r=e.getBoundingClientRect();return s+': top='+Math.round(r.top)+' left='+Math.round(r.left)+' h='+Math.round(r.height)+' w='+Math.round(r.width)}).join(' ; ') + ' ; scrollWidth=' + document.documentElement.scrollWidth` })
    const v = pr.result && pr.result.result && pr.result.result.value; console.log(v !== undefined ? v : JSON.stringify(pr).slice(0, 600))
  }
  const params = { format: 'png' }
  if (full === '1') params.captureBeyondViewport = true
  const r = await send('Page.captureScreenshot', params)
  if (!r.result || !r.result.data) { console.error('capture failed', JSON.stringify(r).slice(0, 400)); process.exit(1) }
  fs.writeFileSync(OUT, Buffer.from(r.result.data, 'base64'))
  ws.close(); chrome.kill()
  try { fs.rmSync(profile, { recursive: true, force: true }) } catch {}
  console.log(OUT)
  process.exit(0)
})().catch(e => { console.error(e); chrome.kill(); process.exit(1) })