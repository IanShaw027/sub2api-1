// Requires the local mock preview and PLAYWRIGHT_MODULE when Playwright is not installed locally.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const fs = require('node:fs/promises')
const path = require('node:path')

async function main() {
  const base = process.env.UI_SHOTS_BASE || 'http://127.0.0.1:3777'
  const output = path.resolve(__dirname, '../../../output/playwright/full-route-audit')
  await fs.mkdir(output, { recursive: true })
  let routes = (await fs.readFile(path.join(__dirname, 'routes.txt'), 'utf8')).trim().split('\n').map(line => {
    const [, role, route] = line.split('|')
    return { role, route }
  })
  for (const route of ['/studio/chat', '/studio/image', '/studio/video', '/studio/voice', '/studio/gallery']) {
    routes.push({ role: 'user', route })
  }
  if (process.env.UI_AUDIT_ROUTES) routes = routes.filter(item => process.env.UI_AUDIT_ROUTES.split(',').includes(item.route))
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const results = process.env.UI_AUDIT_ROUTES ? JSON.parse(await fs.readFile(path.join(output, '../full-route-audit.json'), 'utf8')).filter(item => !routes.some(route => route.route === item.route)) : []
  try {
    for (const width of [1440, 390]) {
      const context = await browser.newContext({ viewport: { width, height: width === 390 ? 844 : 1000 } })
      const page = await context.newPage()
      let errors = []
      page.on('pageerror', error => errors.push(error.message))
      let previousRole
      for (const { role, route } of routes) {
        errors = []
        if (role !== previousRole) {
          await page.goto(`${base}/setup/seed?role=${role}&theme=light&locale=zh&to=%2F__blank`)
          await page.waitForTimeout(800)
          previousRole = role
        }
        try {
          await page.goto(base + route)
          await page.waitForTimeout(1100)
          await page.evaluate(() => document.fonts.ready)
          const state = await page.evaluate(() => {
            const visible = el => { const r = el.getBoundingClientRect(); const s = getComputedStyle(el); return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none' }
            const descriptor = el => `${el.tagName.toLowerCase()}.${String(el.className).slice(0, 120)}`
            const buttons = [...document.querySelectorAll('button')].filter(visible)
            return {
              actualRoute: location.pathname,
              scrollWidth: document.documentElement.scrollWidth,
              headings: [...document.querySelectorAll('h1,h2')].map(el => el.textContent.trim()),
              bodyLength: document.body.innerText.length,
              overlay: [...document.querySelectorAll('vite-error-overlay,vite-plugin-checker-error-overlay')].some(el => visible(el) || [...(el.shadowRoot?.querySelectorAll('.window,.backdrop') || [])].some(visible)),
              missingIconLabels: buttons.filter(el => !el.innerText.trim() && el.querySelector('svg,img') && !el.getAttribute('aria-label') && !el.getAttribute('title') && !el.getAttribute('aria-labelledby')).map(descriptor),
              missingIconTooltips: buttons.filter(el => !el.innerText.trim() && el.querySelector('svg,img') && el.getAttribute('aria-label') && !el.getAttribute('title') && !el.getAttribute('aria-describedby') && !el.closest('.help-tooltip-wrapper,.tooltip-container')).map(el => ({ element: descriptor(el), label: el.getAttribute('aria-label') })),
              wideElements: [...document.querySelectorAll('main *')].filter(visible).filter(el => { const r = el.getBoundingClientRect(); return r.right > innerWidth + 2 && !el.closest('table,[class*="overflow-x"],.table-scroll-container') }).slice(0, 12).map(descriptor),
              tables: [...document.querySelectorAll('table')].filter(visible).map(el => ({ width: Math.round(el.getBoundingClientRect().width), cells: el.querySelectorAll('td').length })),
              toasts: [...document.querySelectorAll('.toast-error .toast-message')].map(el => el.textContent.trim())
            }
          })
          const filename = `${width}-${route.split('?')[0].replace(/[^a-z0-9]+/gi, '-') || 'home'}.png`
          await page.screenshot({ path: path.join(output, filename), fullPage: true })
          const external = /\/auth\/|\/payment\/|\/custom\/|^\/setup$/.test(route)
          results.push({ route, role, width, ...state, errors: [...errors], external, screenshot: filename })
          console.log(JSON.stringify({ route, width, overflow: state.scrollWidth > width, errors: errors.length, iconLabels: state.missingIconLabels.length, tooltips: state.missingIconTooltips.length, actualRoute: state.actualRoute }))
        } catch (error) {
          results.push({ route, role, width, error: error.message })
        }
        await fs.writeFile(path.join(output, '../full-route-audit.json'), JSON.stringify(results, null, 2))
      }
      await context.close()
    }
  } finally {
    await browser.close()
  }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
