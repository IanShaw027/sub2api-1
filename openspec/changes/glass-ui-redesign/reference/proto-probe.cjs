// node proto-probe.cjs  -> prints element rects relative to each artboard (03/04/05/06)
const WebSocket = require('/home/box/code/sub2api/frontend/node_modules/ws')
const { spawn } = require('child_process')
const fs = require('fs'), os = require('os'), path = require('path'), http = require('http')
const S = '/tmp/claude-1000/-home-box-code-sub2api/42a7ff81-4e53-45f5-b619-8dc39e8731a3/scratchpad'
const mime = { '.html': 'text/html', '.js': 'application/javascript' }
const srv = http.createServer((req, res) => { const p = path.join(S, 'design', decodeURIComponent(req.url.split('?')[0])); fs.readFile(p, (e, d) => { if (e) { res.writeHead(404); res.end(); return } res.writeHead(200, { 'Content-Type': mime[path.extname(p)] || 'application/octet-stream' }); res.end(d) }) }).listen(3799)
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
  for (let i = 0; i < 80; i++) { const r = await send('Runtime.evaluate', { expression: `document.querySelectorAll('[data-screen-label]').length + ':' + document.body.innerText.length`, returnByValue: true }); const v = r.result?.result?.value || '0:0'; if (parseInt(v.split(':')[0]) >= 9 && parseInt(v.split(':')[1]) > 2000) break; await sleep(500) }
  await sleep(2500)
  const expr = process.argv[2] || `
  (()=>{const out=[];for(const sec of document.querySelectorAll('[data-screen-label]')){const label=sec.getAttribute('data-screen-label');if(!/^0[3456]/.test(label))continue;
    const board=sec.children[1];if(!board)continue;const b=board.getBoundingClientRect();const rel=e=>{const r=e.getBoundingClientRect();return Math.round(r.top-b.top)+'..'+Math.round(r.bottom-b.top)+' x'+Math.round(r.left-b.left)+' w'+Math.round(r.width)};
    const col=board.querySelector(':scope > div:nth-child(2)');const header=col&&col.querySelector(':scope > header');const main=col&&col.querySelector(':scope > main');
    out.push('== '+label+' board '+Math.round(b.width)+'x'+Math.round(b.height));
    if(header){out.push(' header '+rel(header)+' crumbs '+rel(header.children[0])+' right '+rel(header.children[1]));}
    if(main){out.push(' main '+rel(main));let k=0;for(const c of main.children){if(k++>5)break;const h1=c.querySelector('h1');out.push('  child'+k+' '+rel(c)+(h1?' h1 '+rel(h1)+' lh='+getComputedStyle(h1).lineHeight+' font='+getComputedStyle(h1).fontFamily.slice(0,20):'')+(h1&&h1.nextElementSibling?' p '+rel(h1.nextElementSibling)+' lh='+getComputedStyle(h1.nextElementSibling).lineHeight:''))}}
  }return out.join('\\n')})()`
  const r = await send('Runtime.evaluate', { expression: expr, returnByValue: true })
  console.log(r.result?.result?.value ?? JSON.stringify(r).slice(0, 800))
  ws.close(); chrome.kill(); srv.close(); try { fs.rmSync(profile, { recursive: true, force: true }) } catch {} process.exit(0)
})().catch(e => { console.error(e); chrome.kill(); srv.close(); process.exit(1) })
