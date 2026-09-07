const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright')
const assert = require('node:assert/strict')
const fs = require('node:fs/promises')
const path = require('node:path')

async function main() {
  const browser = await chromium.launch({ channel: 'chrome', headless: true })
  const output = path.resolve(__dirname, '../../../output/playwright/dashboard-review')
  await fs.mkdir(output, { recursive: true })
  const checks = []
  try {
    for (const width of [1440, 390]) {
      const page = await browser.newPage({ viewport: { width, height: width === 390 ? 844 : 1000 } })
      const requests = []
      const errors = []
      page.on('request', request => { if (request.url().includes('/api/')) requests.push(request.url()) })
      page.on('pageerror', error => errors.push(error.message))
      await page.goto('http://127.0.0.1:3777/setup/seed?role=admin&theme=light&locale=zh&to=%2Fadmin%2Fdashboard')
      await page.locator('.distribution-toggle').first().waitFor()
      await page.screenshot({ path: path.join(output, `${width}-dashboard.png`), fullPage: true })
      await page.locator('.date-picker-trigger').click()
      const dialog = page.locator('.date-picker-dropdown')
      await dialog.waitFor()
      await page.waitForTimeout(250)
      const rect = await dialog.boundingBox()
      assert(rect && rect.x >= 0 && rect.y >= 0 && rect.x + rect.width <= width)
      await page.screenshot({ path: path.join(output, `${width}-date-picker.png`) })
      await dialog.getByRole('button', { name: '近 7 天', exact: true }).click()
      requests.length = 0
      await dialog.getByRole('button', { name: '应用', exact: true }).click()
      await page.waitForTimeout(900)
      const urls = requests.map(value => new URL(value))
      const snapshot = urls.find(url => url.pathname.endsWith('/dashboard/snapshot-v2') && url.searchParams.get('include_trend') === 'true')
      assert(snapshot, 'date selection must reload range stats and trend')
      const start = snapshot.searchParams.get('start_date')
      const end = snapshot.searchParams.get('end_date')
      for (const endpoint of ['users-trend', 'users-ranking']) {
        assert(urls.some(url => url.pathname.endsWith(endpoint) && url.searchParams.get('start_date') === start && url.searchParams.get('end_date') === end), endpoint)
      }
      assert(urls.some(url => url.pathname.endsWith('/ops/errors') && url.searchParams.has('start_time')), 'events date scope')
      await page.locator('.distribution-toggle').first().click()
      await page.waitForTimeout(400)
      assert(requests.some(url => url.includes('/user-breakdown?') && new URL(url).searchParams.has('model')))
      await page.screenshot({ path: path.join(output, `${width}-model-users.png`), fullPage: true })
      await page.getByRole('radio', { name: '用户消费榜', exact: true }).click()
      await page.locator('.distribution-toggle').first().click()
      await page.waitForTimeout(400)
      assert(requests.some(url => url.includes('/dashboard/models?') && new URL(url).searchParams.has('user_id')))
      await page.screenshot({ path: path.join(output, `${width}-user-models.png`), fullPage: true })
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), width)
      assert.deepEqual(errors, [])
      checks.push({ width, start, end, requests, errors, datePopup: rect })
      await page.close()
    }
    await fs.writeFile(path.join(output, 'checks.json'), JSON.stringify(checks, null, 2))
    console.log('Dashboard range, bidirectional expansion and popup checks passed at 1440px and 390px')
  } finally { await browser.close() }
}
main().catch(error => { console.error(error); process.exitCode = 1 })
