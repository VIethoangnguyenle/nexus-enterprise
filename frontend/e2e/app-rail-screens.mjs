/**
 * Menu chính (app rail) screenshots on the real stack: both states at 1280 and
 * 1440, light and dark, plus the menus, the tooltip and the notification panel.
 * Signs in once with a fresh address (OTP 999999) and never reloads afterwards.
 *
 * Usage: cd frontend && node e2e/app-rail-screens.mjs
 * Output: plans/reports/app-rail-screens/, results in results.json there.
 */
import { chromium } from 'playwright'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const REPO = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..')
const OUT = path.join(REPO, 'plans/reports/app-rail-screens')
const LOG = path.join(REPO, 'plans/reports/progress-app-rail.log')
const BASE = 'http://localhost:5173'
const EMAIL = `rail-${Date.now()}@example.org`
fs.mkdirSync(OUT, { recursive: true })
const log = (l) => { fs.appendFileSync(LOG, `${new Date().toISOString().slice(11, 19)} | ${l}\n`); console.log(l) }

const browser = await chromium.launch()
const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } })
const page = await ctx.newPage()
const errors = []
page.on('console', (m) => { if (m.type() === 'error') errors.push(m.text()) })
page.on('pageerror', (e) => errors.push(String(e)))

await page.goto(`${BASE}/login`)
const field = page.getByLabel('Email hoặc số điện thoại')
await field.waitFor({ state: 'visible', timeout: 15000 })
await field.fill(EMAIL)
await page.getByRole('button', { name: 'Nhận mã đăng nhập' }).click()
const otp = page.getByLabel('Mã xác minh')
await otp.waitFor({ state: 'attached', timeout: 10000 })
await otp.focus()
for (const d of '999999') await page.keyboard.type(d, { delay: 50 })
await page.waitForFunction(() => !location.pathname.startsWith('/login'), null, { timeout: 20000 })
if (new URL(page.url()).pathname.startsWith('/register')) {
  await page.getByLabel('Tên hiển thị').fill('Lê Thị Hoa')
  await page.getByRole('button', { name: /Tiếp tục/ }).click()
  await page.waitForFunction(() => !location.pathname.startsWith('/register'), null, { timeout: 15000 })
}
log(`signed in as ${EMAIL} -> ${page.url()}`)
if (new URL(page.url()).pathname.startsWith('/workspace-select')) {
  await page.locator('[data-ws-row]').first().click()
}
await page.waitForURL((u) => u.pathname.includes('/channels'), { timeout: 20000 }).catch(() => log(`not on /channels: ${page.url()}`))
await page.waitForTimeout(1500)

// The dev-only query devtools button floats over the bottom-left corner, on top of the avatar.
await page.addStyleTag({ content: '.tsqd-parent-container, .tsqd-open-btn-container { display: none !important }' })
const nav = page.getByRole('navigation', { name: 'Điều hướng chính' })
await nav.waitFor({ state: 'visible', timeout: 15000 })
const results = []
// The dev database predates `notifications.entity_type` (GET /api/notifications answers 500 there), so the
// populated panel is shot against a stubbed answer shaped like docs/specs/notifications; the error state is
// shot against the real 500.
const ago = (min) => new Date(Date.now() - min * 60_000).toISOString()
const SAMPLE = [
  { id: 'a1', type: 'approval_approved', read: false, created_at: ago(5), entity_type: 'approval', entity_id: 'r1' },
  { id: 'a2', type: 'asset_assigned', read: false, created_at: ago(95), entity_type: 'asset', entity_id: 'x1' },
  { id: 'a3', type: 'approval_rejected', read: false, created_at: ago(200), entity_type: 'approval', entity_id: 'r2' },
  { id: 'a4', type: 'asset_request_approved', read: true, created_at: ago(26 * 60), entity_type: 'asset_request', entity_id: 'q1' },
  { id: 'a5', type: 'asset_request', read: true, created_at: ago(4 * 24 * 60), entity_type: 'asset_request', entity_id: 'q2' },
  { id: 'a6', type: 'asset_returned', read: true, created_at: ago(6 * 24 * 60), entity_type: 'asset', entity_id: 'x2' },
].map((n) => ({ title: 'x', body: 'x', ...n }))
const stub = (route) => {
  const url = route.request().url()
  if (url.includes('unread-count')) return route.fulfill({ json: { count: 3 } })
  return route.fulfill({ json: { notifications: SAMPLE, total: 31, unread_count: 3 } })
}
const widthOf = async () => Math.round((await nav.boundingBox()).width)
const setTheme = (t) => page.evaluate((t) => { if (t === 'dark') document.documentElement.dataset.theme = 'dark'; else delete document.documentElement.dataset.theme }, t)
const setExpanded = async (want) => {
  const isRail = (await nav.getAttribute('data-rail')) === 'true'
  if (isRail === !want) return
  await page.keyboard.press('Control+Backslash')
  await page.waitForFunction((want) => { const n = document.querySelector('nav[aria-label="Điều hướng chính"]'); return (n.getAttribute('data-rail') === 'true') === !want && !n.hasAttribute('data-swapping') }, want, { timeout: 5000 })
  await page.waitForTimeout(450)
}
const overflowCheck = () => page.evaluate(() => {
  const n = document.querySelector('nav[aria-label="Điều hướng chính"]')
  const labelOverflow = [...n.querySelectorAll('a, button')].filter((e) => e.scrollWidth > e.clientWidth + 1).map((e) => e.textContent?.trim())
  return { pageOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth, labelOverflow }
})

// default state on a fresh device: the rail
results.push({ check: 'default is rail', rail: (await nav.getAttribute('data-rail')) === 'true', width: await widthOf() })
log(`default: ${JSON.stringify(results.at(-1))}`)

for (const [w, h] of [[1280, 800], [1440, 900]]) {
  await page.setViewportSize({ width: w, height: h })
  for (const theme of ['light', 'dark']) {
    await setTheme(theme)
    for (const state of ['rail', 'expanded']) {
      await setExpanded(state === 'expanded')
      await page.mouse.move(700, 450)
      const file = `${w}-${theme}-${state}.png`
      await page.screenshot({ path: path.join(OUT, file) })
      const rec = { file, navWidth: await widthOf(), ...(await overflowCheck()) }
      results.push(rec)
      log(`shot ${file} ${JSON.stringify(rec)}`)
    }
  }
}

// menus, tooltip and the panel at 1440 light + dark
await page.setViewportSize({ width: 1440, height: 900 })
for (const theme of ['light', 'dark']) {
  await setTheme(theme)
  await setExpanded(false)
  await page.getByRole('button', { name: /^Đổi workspace, đang ở/ }).click()
  await page.waitForTimeout(400)
  await page.screenshot({ path: path.join(OUT, `1440-${theme}-rail-workspace-menu.png`) })
  await page.keyboard.press('Escape')
  await page.getByRole('button', { name: /^Tài khoản: / }).click()
  await page.waitForTimeout(400)
  await page.screenshot({ path: path.join(OUT, `1440-${theme}-rail-account-menu.png`) })
  await page.keyboard.press('Escape')
  await page.waitForTimeout(200)
  await page.getByRole('button', { name: 'Mở rộng menu chính' }).focus()
  await page.keyboard.press('Shift+Tab')
  await page.keyboard.press('Tab')
  await page.waitForTimeout(400)
  await page.screenshot({ path: path.join(OUT, `1440-${theme}-rail-toggle-tooltip.png`), clip: { x: 0, y: 0, width: 420, height: 200 } })
  await page.route(/\/api\/notifications(\?|\/unread-count)/, stub)
  await page.getByRole('button', { name: /^Thông báo/ }).click()
  await page.getByRole('dialog', { name: 'Thông báo' }).waitFor({ state: 'visible', timeout: 8000 })
  await page.getByText('Đã hiện 6 trong 31').waitFor({ timeout: 8000 })
  await page.waitForTimeout(500)
  await page.screenshot({ path: path.join(OUT, `1440-${theme}-rail-notifications.png`) })
  await page.keyboard.press('Escape')
  await page.waitForTimeout(400)
  await page.unroute(/\/api\/notifications(\?|\/unread-count)/, stub)
  log(`menus ${theme} done`)
}
await setTheme('light')
await setExpanded(true)
await page.route(/\/api\/notifications(\?|\/unread-count)/, stub)
await page.getByRole('button', { name: /^Thông báo/ }).click()
await page.getByRole('dialog', { name: 'Thông báo' }).waitFor({ state: 'visible', timeout: 8000 })
await page.getByText('Đã hiện 6 trong 31').waitFor({ timeout: 8000 })
await page.waitForTimeout(500)
await page.screenshot({ path: path.join(OUT, '1440-light-expanded-notifications.png') })
await page.keyboard.press('Escape')
await page.waitForTimeout(300)
await page.unroute(/\/api\/notifications(\?|\/unread-count)/, stub)
// the real 500 of this dev database: the error state with its retry
await page.getByRole('button', { name: /^Thông báo/ }).click()
await page.getByText('Không tải được thông báo').waitFor({ timeout: 15000 })
await page.waitForTimeout(300)
await page.screenshot({ path: path.join(OUT, '1440-light-expanded-notifications-error.png') })
await page.keyboard.press('Escape')
await page.getByRole('button', { name: /^Đổi workspace|Khối|Không gian/ }).first().click().catch(() => {})
await page.waitForTimeout(400)
await page.screenshot({ path: path.join(OUT, '1440-light-expanded-workspace-menu.png') })

results.push({ check: 'console errors', errors })
fs.writeFileSync(path.join(OUT, 'results.json'), JSON.stringify(results, null, 2))
log(`console errors: ${errors.length}`)
await browser.close()
