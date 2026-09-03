#!/usr/bin/env node
// Pixel diff between a reference PNG (prototype artboard) and an actual screenshot.
// Usage: node scripts/ui/pixel-diff.mjs <reference.png> <actual.png> [out-heatmap.png] [--ignore x,y,w,h ...] [--tol 24] [--ref-offset 0,32]
// Ignore rects are in ACTUAL-image coordinates. Prototype artboards rendered by proto-shots.cjs carry a 32px label bar on top: pass --ref-offset 0,32.
// Prints diff percentage (excluding ignored rects) and writes a heatmap (red = differs, faded grey = same).
// Sizes may differ: the comparison runs on the overlapping top-left region and reports the size mismatch.
import fs from 'node:fs'
import path from 'node:path'
import { PNG } from 'pngjs'

const args = process.argv.slice(2)
const files = args.filter((a) => !a.startsWith('--') && !/^\d+,\d+,\d+,\d+$/.test(a))
const [refPath, actPath, outPath = 'pixel-diff.png'] = files
if (!refPath || !actPath) {
  console.error('usage: pixel-diff.mjs <reference.png> <actual.png> [out.png] [--ignore x,y,w,h]... [--tol N]')
  process.exit(2)
}
const ignores = []
let tol = 24
let refOff = [0, 0] // --ref-offset x,y : skip this many reference pixels (e.g. artboard label bar) before comparing
for (let i = 0; i < args.length; i++) {
  if (args[i] === '--ignore') ignores.push(args[++i].split(',').map(Number))
  if (args[i] === '--tol') tol = Number(args[++i])
  if (args[i] === '--ref-offset') refOff = args[++i].split(',').map(Number)
}

const ref = PNG.sync.read(fs.readFileSync(refPath))
const act = PNG.sync.read(fs.readFileSync(actPath))
const w = Math.min(ref.width - refOff[0], act.width)
const h = Math.min(ref.height - refOff[1], act.height)
const out = new PNG({ width: w, height: h })
let compared = 0
let differing = 0
const rowHits = new Array(h).fill(0)
for (let y = 0; y < h; y++) {
  for (let x = 0; x < w; x++) {
    const o = (y * w + x) * 4
    const ri = ((y + refOff[1]) * ref.width + x + refOff[0]) * 4
    const ai = (y * act.width + x) * 4
    const ignored = ignores.some(([ix, iy, iw, ih]) => x >= ix && x < ix + iw && y >= iy && y < iy + ih)
    const d = Math.max(
      Math.abs(ref.data[ri] - act.data[ai]),
      Math.abs(ref.data[ri + 1] - act.data[ai + 1]),
      Math.abs(ref.data[ri + 2] - act.data[ai + 2])
    )
    const grey = Math.round((ref.data[ri] + ref.data[ri + 1] + ref.data[ri + 2]) / 3)
    if (ignored) {
      out.data[o] = grey; out.data[o + 1] = grey; out.data[o + 2] = 255; out.data[o + 3] = 90
      continue
    }
    compared++
    if (d > tol) {
      differing++
      rowHits[y]++
      out.data[o] = 255; out.data[o + 1] = 40; out.data[o + 2] = 40; out.data[o + 3] = 255
    } else {
      out.data[o] = grey; out.data[o + 1] = grey; out.data[o + 2] = grey; out.data[o + 3] = 70
    }
  }
}
fs.mkdirSync(path.dirname(path.resolve(outPath)), { recursive: true })
fs.writeFileSync(outPath, PNG.sync.write(out))

// Summarise the vertical bands that differ most, so agents can map hits to page sections.
const bands = []
let start = -1
for (let y = 0; y <= h; y++) {
  const hot = y < h && rowHits[y] > w * 0.02
  if (hot && start < 0) start = y
  if (!hot && start >= 0) { bands.push([start, y]); start = -1 }
}
const pct = compared ? (differing / compared) * 100 : 0
console.log(JSON.stringify({
  reference: { w: ref.width, h: ref.height },
  actual: { w: act.width, h: act.height },
  compared_region: { w, h },
  size_mismatch: ref.width !== act.width || ref.height !== act.height,
  tolerance: tol,
  ref_offset: refOff,
  ignored_rects: ignores,
  diff_pixels: differing,
  diff_percent: Number(pct.toFixed(3)),
  hot_bands_y: bands.map(([a, b]) => `${a}-${b}`),
  heatmap: path.resolve(outPath)
}, null, 2))
