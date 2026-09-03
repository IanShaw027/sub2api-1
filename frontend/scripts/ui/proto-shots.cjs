// Render each artboard of the Claude Design prototype (.dc.html + support.js in one dir) to PNG.
// Usage: node scripts/ui/proto-shots.cjs <design-dir> [glass-light|glass-dark] [out-dir]
// The dir must contain redesign.html (the .dc.html) and support.js; React is fetched from unpkg by support.js.
const WebSocket = require('ws')
const { spawn } = require('child_process')
const fs = require('fs'), os = require('os'), path = require('path'), http = require('http')
const DESIGN = path.resolve(process.argv[2] || '.')
const theme = process.argv[3] || 'glass-light'
const OUT = path.resolve(process.argv[4] || path.join(DESIGN, 'reference', theme)); fs.mkdirSync(OUT, { recursive: true })
// static server for design dir
const mime = { '.html': 'text/html', '.js': 'application/javascript', '.css': 'text/css' }
const srv = http.createServer((req, res) => {
  const p = path.join(DESIGN, decodeURIComponent(req.url.split('?')[0]))
  fs.readFile(p, (e, d) => { if (e) { res.writeHead(404); res.end(); return } res.writeHead(200, { 'Content-Type': mime[path.extname(p)] || 'application/octet-stream' }); res.end(d) })
}).listen(3799)
const port = 9600 + Math.floor(Math.random() * 300)
const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'cdp-'))
const chrome = spawn('google-chrome', ['--headless=new', '--no-sandbox', '--disable-gpu', '--disable-dev-shm-usage', '--hide-scrollbars', `--user-data-dir=${profile}`, '--window-size=1600,1200', `--remote-debugging-port=${port}`, 'about:blank'], { stdio: 'ignore' })
const sleep = ms => new Promise(r => setTimeout(r, ms))
async function getTarget() { for (let i = 0; i < 60; i++) { try { const r = await fetch(`http://127.0.0.1:${port}/json/list`); const l = await r.json(); const t = l.find(x => x.type === 'page'); if (t) return t } catch {} await sleep(250) } throw new Error('no target') }
;(async () => {
  const t = await getTarget()
  const ws = new WebSocket(t.webSocketDebuggerUrl, { perMessageDeflate: false, maxPayload: 512 * 1024 * 1024 })
  let id = 0; const pending = new Map()
  ws.on('message', d => { const m = JSON.parse(d); if (m.id && pending.has(m.id)) { pending.get(m.id)(m); pending.delete(m.id) } })
  await new Promise(r => ws.on('open', r))
  const send = (method, params = {}) => new Promise(res => { const i = ++id; pending.set(i, res); ws.send(JSON.stringify({ id: i, method, params })) })
  await send('Page.enable'); await send('Runtime.enable')
  await send('Emulation.setDeviceMetricsOverride', { width: 1600, height: 1200, deviceScaleFactor: 1, mobile: false })
  await send('Page.navigate', { url: 'http://127.0.0.1:3799/redesign.html' })
  // wait for runtime to render artboards
  let ok = false
  for (let i = 0; i < 80; i++) {
    const r = await send('Runtime.evaluate', { expression: `document.querySelectorAll('[data-screen-label]').length + ':' + document.body.innerText.length`, returnByValue: true })
    const v = r.result?.result?.value || '0:0'
    if (parseInt(v.split(':')[0]) >= 9 && parseInt(v.split(':')[1]) > 2000) { ok = true; break }
    await sleep(500)
  }
  if (!ok) { console.error('artboards did not render'); }
  // apply theme via data-theme root attr
  await send('Runtime.evaluate', { expression: `document.querySelectorAll('[data-theme]').forEach(e=>e.setAttribute('data-theme','${theme}'))` })
  await sleep(2500)
  const list = await send('Runtime.evaluate', { expression: `JSON.stringify([...document.querySelectorAll('[data-screen-label]')].map(e=>{const r=e.getBoundingClientRect();return {label:e.getAttribute('data-screen-label'),x:r.left+scrollX,y:r.top+scrollY,w:r.width,h:r.height}}))`, returnByValue: true })
  const boards = JSON.parse(list.result.result.value)
  console.log(boards.map(b => `${b.label} ${Math.round(b.w)}x${Math.round(b.h)}`).join('\n'))
  const total = await send('Runtime.evaluate', { expression: 'document.documentElement.scrollHeight', returnByValue: true })
  await send('Emulation.setDeviceMetricsOverride', { width: 1600, height: Math.min(total.result.result.value, 16000), deviceScaleFactor: 1, mobile: false })
  await sleep(1500)
  for (const b of boards) {
    const name = b.label.replace(/[^\w一-龥]+/g, '_').slice(0, 40)
    const r = await send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true, clip: { x: b.x, y: b.y, width: b.w, height: b.h, scale: 1 } })
    if (r.result?.data) { fs.writeFileSync(path.join(OUT, name + '.png'), Buffer.from(r.result.data, 'base64')); console.log('saved', name) } else console.error('fail', name, JSON.stringify(r).slice(0, 200))
  }
  ws.close(); chrome.kill(); srv.close(); fs.rmSync(profile, { recursive: true, force: true }); process.exit(0)
})().catch(e => { console.error(e); chrome.kill(); srv.close(); process.exit(1) })
