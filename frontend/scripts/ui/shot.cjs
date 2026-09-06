// Usage: UI_SHOTS_CHROME=/path/to/chromium node scripts/ui/shot.cjs <name> <route> [w] [h] [role=admin|user|guest] [theme] [locale] [full]
// Requires: mock backend (scripts/mock/server.js, :8091) + `VITE_DEV_PROXY_TARGET=http://127.0.0.1:8091 vite --port 3777`.
// Output dir: $UI_SHOTS_DIR (default frontend/.shots). Dev server: $UI_SHOTS_BASE (default http://127.0.0.1:3777).
const WebSocket = require('ws')
const { spawn } = require('child_process')
const fs = require('fs')
const os = require('os')
const path = require('path')
const { execFileSync } = require('child_process')

const [name, route, w = '1440', h = '1000', role = 'admin', theme = 'light', locale = 'zh', full = '0'] = process.argv.slice(2)
const OUT_DIR = process.env.UI_SHOTS_DIR || path.join(__dirname, '..', '..', '.shots')
const DEV = process.env.UI_SHOTS_BASE || 'http://127.0.0.1:3777'
const OUT = path.join(OUT_DIR, name + '.png')
fs.mkdirSync(path.dirname(OUT), { recursive: true })
const port = 9500 + Math.floor(Math.random() * 400)
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'cdp-'))
function findChrome() {
  if (process.env.UI_SHOTS_CHROME) return process.env.UI_SHOTS_CHROME
  const candidates = process.platform === 'darwin'
    ? [
        '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
        '/Applications/Chromium.app/Contents/MacOS/Chromium',
        path.join(os.homedir(), 'Applications/Google Chrome.app/Contents/MacOS/Google Chrome'),
      ]
    : process.platform === 'win32'
      ? [process.env.PROGRAMFILES && path.join(process.env.PROGRAMFILES, 'Google/Chrome/Application/chrome.exe')]
      : ['google-chrome', 'chromium', 'chromium-browser']
  for (const candidate of candidates.filter(Boolean)) {
    if (path.isAbsolute(candidate) && fs.existsSync(candidate)) return candidate
    if (!path.isAbsolute(candidate)) {
      try {
        return execFileSync('which', [candidate], { encoding: 'utf8' }).trim()
      } catch { /* try the next candidate */ }
    }
  }
  throw new Error('No Chromium-based browser found. Set UI_SHOTS_CHROME to a browser executable path.')
}

const chrome = spawn(findChrome(), ['--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage',
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
    } catch { /* Chrome's debugging endpoint is not ready yet. */ }
    await sleep(250)
  }
  throw new Error('no target')
}

(async () => {
  const t = await getTarget()
  const ws = new WebSocket(t.webSocketDebuggerUrl, { perMessageDeflate: false, maxPayload: 512 * 1024 * 1024 })
  let id = 0
  const pending = new Map()
  const pageErrors = []
  ws.on('message', d => {
    const m = JSON.parse(d)
    if (m.method === 'Runtime.exceptionThrown') pageErrors.push(m.params.exceptionDetails.exception?.description || m.params.exceptionDetails.text)
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
  let ready = false
  const readySelector = process.env.UI_SHOTS_READY_SELECTOR || (route.split('?')[0] === '/admin/settings' ? '#settings-form' : 'h1,h2')
  for (let attempt = 0; attempt < 60; attempt++) {
    const state = await send('Runtime.evaluate', { returnByValue: true, expression: `location.pathname === ${JSON.stringify(route.split('?')[0])} && document.querySelectorAll(${JSON.stringify(readySelector)}).length > 0` })
    if (state.result?.result?.value) { ready = true; break }
    await sleep(500)
  }
  if (!ready) throw new Error('Route content did not become ready within 30 seconds')
  await send('Runtime.evaluate', { expression: 'document.fonts.ready', awaitPromise: true })
  await sleep(2000)
  // Full-page captures must visit lazy/off-screen charts before snapshotting.
  if (full === '1') {
    await send('Runtime.evaluate', { expression: `(async () => {
      for (const canvas of document.querySelectorAll('canvas')) {
        canvas.scrollIntoView({ block: 'center', behavior: 'instant' });
        await new Promise(resolve => setTimeout(resolve, 1500));
      }
      window.scrollTo({ top: 0, behavior: 'instant' });
    })()`, awaitPromise: true })
    await sleep(400)
  }
  const diagnostics = await send('Runtime.evaluate', { returnByValue: true, expression: `({
    route: location.pathname,
    theme: document.documentElement.dataset.theme,
    viewport: { width: innerWidth, height: innerHeight },
    scrollWidth: document.documentElement.scrollWidth,
    overlays: Array.from(document.querySelectorAll('vite-plugin-checker-error-overlay,vite-error-overlay')).map(e => ({
      tag: e.tagName,
      text: Array.from((e.shadowRoot || e).querySelectorAll('.message-body,.message,.file,pre')).map(n => n.textContent).join(' ').slice(-12000),
      visible: [e, ...Array.from(e.shadowRoot?.querySelectorAll('.window,.backdrop') || [])].some(n => {
        const box = n.getBoundingClientRect(), style = getComputedStyle(n)
        return box.width > 0 && box.height > 0 && style.visibility !== 'hidden' && style.display !== 'none' && style.opacity !== '0'
      })
    })),
    headings: Array.from(document.querySelectorAll('h1,h2')).map(e => e.textContent.trim()),
    errorToasts: Array.from(document.querySelectorAll('.toast-error .toast-message')).map(e => e.textContent.trim()),
    brokenImages: Array.from(document.images).filter(e => !e.complete || !e.naturalWidth).map(e => e.src)
  })` })
  const diagnostic = diagnostics.result?.result?.value
  if (!diagnostic) throw new Error('Page diagnostics failed')
  diagnostic.pageErrors = pageErrors
  fs.writeFileSync(OUT.replace(/\.png$/, '.json'), JSON.stringify(diagnostic, null, 2) + '\n')
  if (diagnostic.route !== route.split('?')[0]) throw new Error(`Unexpected route: ${diagnostic.route}`)
  await sleep(600)
  // Optional probe: UI_SHOTS_PROBE='sel1|sel2' prints top/left/height of the first match of each selector.
  // UI_SHOTS_PROBE_ALL=1 prints a height histogram over every match instead (row-height gates).
  if (process.env.UI_SHOTS_PROBE && process.env.UI_SHOTS_PROBE_ALL) {
    const sels = JSON.stringify(process.env.UI_SHOTS_PROBE.split('|'))
    const pr = await send('Runtime.evaluate', { returnByValue: true, expression: `(${sels}).map(s=>{const hs={};document.querySelectorAll(s).forEach(e=>{const h=Math.round(e.getBoundingClientRect().height);hs[h]=(hs[h]||0)+1});return s+': '+Object.entries(hs).sort((a,b)=>a[0]-b[0]).map(([h,n])=>h+'x'+n).join(' ')}).join(' ; ')` })
    console.log(pr.result.result.value)
  } else if (process.env.UI_SHOTS_PROBE) {
    const sels = JSON.stringify(process.env.UI_SHOTS_PROBE.split('|'))
    const pr = await send('Runtime.evaluate', { returnByValue: true, expression: `(${sels}).map(s=>{const e=document.querySelector(s);if(!e)return s+': (none)';const r=e.getBoundingClientRect();return s+': top='+Math.round(r.top)+' left='+Math.round(r.left)+' h='+Math.round(r.height)+' w='+Math.round(r.width)}).join(' ; ') + ' ; scrollWidth=' + document.documentElement.scrollWidth` })
    const v = pr.result && pr.result.result && pr.result.result.value; console.log(v !== undefined ? v : JSON.stringify(pr).slice(0, 600))
  }
  const params = { format: 'png' }
  if (full === '1') params.captureBeyondViewport = true
  const r = await send('Page.captureScreenshot', params)
  if (!r.result || !r.result.data) { console.error('capture failed', JSON.stringify(r).slice(0, 400)); process.exit(1) }
  fs.writeFileSync(OUT, Buffer.from(r.result.data, 'base64'))
  if (diagnostic.overlays.some(e => e.visible)) throw new Error('Visible Vite error overlay present; screenshot saved for diagnosis')
  if (pageErrors.length) throw new Error('Page runtime exception; screenshot saved for diagnosis')
  if (diagnostic.errorToasts.length) throw new Error('Unexpected error toast; screenshot saved for diagnosis')
  ws.close(); chrome.kill()
  try { fs.rmSync(profile, { recursive: true, force: true }) } catch { /* Chrome may still hold the temporary profile. */ }
  console.log(OUT)
  process.exit(0)
})().catch(e => { console.error(e); chrome.kill(); process.exit(1) })
