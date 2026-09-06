#!/usr/bin/env node
// Estimate the vertical/horizontal offset between reference and actual per horizontal band.
// Usage: node scripts/ui/shift-profile.mjs <reference.png> <actual.png> [--ref-offset 0,32] [--band 60] [--x0 0 --x1 W]
import fs from 'node:fs'
import { PNG } from 'pngjs'
const a = process.argv.slice(2); const files = a.filter(s => !s.startsWith('--') && !/^[\d,]+$/.test(s))
const opt = (k, d) => { const i = a.indexOf(k); return i >= 0 ? a[i + 1] : d }
const [ox, oy] = opt('--ref-offset', '0,0').split(',').map(Number); const band = Number(opt('--band', 60))
const ref = PNG.sync.read(fs.readFileSync(files[0])), act = PNG.sync.read(fs.readFileSync(files[1]))
const x0 = Number(opt('--x0', 0)), x1 = Number(opt('--x1', Math.min(ref.width - ox, act.width)))
const H = Math.min(ref.height - oy, act.height)
const lum = (p, x, y, ow, oh) => { const i = ((y + oh) * p.width + x + ow) * 4; return p.data[i] * 0.3 + p.data[i + 1] * 0.59 + p.data[i + 2] * 0.11 }
const rows = []
for (let y0 = 0; y0 + band <= H; y0 += band) {
  let best = { dy: 0, dx: 0, err: Infinity }, base = 0, ink = 0
  for (let y = y0; y < y0 + band; y++) for (let x = x0; x < x1; x += 2) { const r = lum(ref, x, y, ox, oy); if (r < 200) ink++ }
  for (let dy = -20; dy <= 20; dy++) for (let dx = -16; dx <= 16; dx += 2) {
    let err = 0, n = 0
    for (let y = y0; y < y0 + band; y += 2) { const ay = y + dy; if (ay < 0 || ay >= act.height) continue
      for (let x = x0; x < x1; x += 4) { const ax = x + dx; if (ax < 0 || ax >= act.width) continue; err += Math.abs(lum(ref, x, y, ox, oy) - lum(act, ax, ay, 0, 0)); n++ } }
    err /= n || 1; if (dy === 0 && dx === 0) base = err; if (err < best.err) best = { dy, dx, err }
  }
  rows.push({ y: `${y0}-${y0 + band}`, ink_px: ink, dy: best.dy, dx: best.dx, err0: base.toFixed(1), errBest: best.err.toFixed(1) })
}
console.table(rows.filter(r => r.ink_px > 50))
