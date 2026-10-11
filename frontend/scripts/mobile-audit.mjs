// Mobile audit: every route at phone and tablet widths, light and dark.
//
// Usage (dev stack running, see `make dev`):
//   node scripts/mobile-audit.mjs <email> [outDir]
// Signs in with the email through the code step (the dev stack's fixed test
// code, read from MOBILE_AUDIT_CODE), then for each route × viewport × theme
// records: horizontal overflow, tap targets under 44 px, inputs under 16 px.
// Writes audit.json and one screenshot per case into outDir.
import { chromium } from 'playwright'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

const BASE = process.env.MOBILE_AUDIT_BASE ?? 'http://localhost:5173'
const CODE = process.env.MOBILE_AUDIT_CODE ?? ''
const [email, outDir = 'mobile-audit-out'] = process.argv.slice(2)
if (!email || !CODE) {
  console.error('usage: MOBILE_AUDIT_CODE=<code> node scripts/mobile-audit.mjs <email> [outDir]')
  process.exit(2)
}
mkdirSync(outDir, { recursive: true })

const VIEWPORTS = [
  { name: '390', width: 390, height: 844, isMobile: true, hasTouch: true },
  { name: '768', width: 768, height: 1024, isMobile: true, hasTouch: true },
]
const THEMES = ['light', 'dark']
// Static routes; the dynamic ones are found by following the first link.
const ROUTES = [
  '/', '/channels', '/documents', '/drive', '/contacts', '/approval', '/settings',
  '/admin', '/admin/users', '/admin/roles',
  '/assets/dashboard', '/assets/list', '/assets/requests', '/assets/types', '/assets/request/new',
  '/workspace-select',
]
const FOLLOW = [
  { from: '/channels', link: 'a[href^="/channels/"]:visible' },
  { from: '/documents', link: 'a[href^="/documents/"]:visible' },
]

/** Runs in the page: what is wrong with this screen at this size. */
function measure() {
  const vw = document.documentElement.clientWidth
  const describe = (el) => {
    const label = el.getAttribute('aria-label') || el.textContent?.trim().slice(0, 40) || ''
    return `${el.tagName.toLowerCase()}${el.id ? '#' + el.id : ''}${label ? ` "${label}"` : ''}`
  }
  const scrollsX = (el) => {
    for (let p = el.parentElement; p; p = p.parentElement) {
      const o = getComputedStyle(p).overflowX
      if (o === 'auto' || o === 'scroll' || o === 'hidden' || o === 'clip') return true
    }
    return false
  }
  const visible = (el) => {
    const r = el.getBoundingClientRect()
    const s = getComputedStyle(el)
    return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none' && s.opacity !== '0'
  }
  const overflow = []
  for (const el of document.body.querySelectorAll('*')) {
    if (!visible(el)) continue
    const r = el.getBoundingClientRect()
    if (r.right > vw + 1 && !scrollsX(el)) overflow.push(`${describe(el)} right=${Math.round(r.right)}`)
  }
  const small = []
  const targets = document.querySelectorAll('button, a[href], [role="button"], [role="tab"], [role="menuitem"], input[type="checkbox"], input[type="radio"], select, summary')
  for (const el of targets) {
    if (!visible(el)) continue
    const r = el.getBoundingClientRect()
    // A text link inside running text is exempt (WCAG 2.5.8 inline exception).
    if (el.tagName === 'A' && getComputedStyle(el).display === 'inline') continue
    // The hit area is the box plus an absolutely placed ::after that reaches past it.
    let w = r.width
    let h = r.height
    const after = getComputedStyle(el, '::after')
    if (after.content !== 'none' && after.position === 'absolute') {
      const out = (v) => Math.max(0, -(parseFloat(v) || 0))
      w += out(after.left) + out(after.right)
      h += out(after.top) + out(after.bottom)
    }
    if (w < 43.5 || h < 43.5) small.push(`${describe(el)} ${Math.round(w)}x${Math.round(h)}`)
  }
  const tinyInputs = []
  for (const el of document.querySelectorAll('input:not([type=checkbox]):not([type=radio]):not([type=hidden]), textarea, select, [contenteditable="true"]')) {
    if (!visible(el)) continue
    const fs = parseFloat(getComputedStyle(el).fontSize)
    if (fs < 16) tinyInputs.push(`${describe(el)} ${fs}px`)
  }
  return {
    pageOverflowX: document.scrollingElement.scrollWidth > vw + 1,
    scrollWidth: document.scrollingElement.scrollWidth,
    overflow: overflow.slice(0, 15),
    smallTargets: small.slice(0, 40),
    smallTargetCount: small.length,
    tinyInputs,
  }
}

// The access token lives in memory only (by design), so a reload signs out:
// each context signs in once, then moves between routes client-side.
async function signIn(page) {
  await page.goto(`${BASE}/login`)
  await page.getByLabel('Email hoặc số điện thoại').fill(email)
  await page.getByRole('button', { name: 'Nhận mã đăng nhập' }).click()
  await page.getByRole('heading', { name: 'Nhập mã 6 số' }).waitFor()
  await page.keyboard.type(CODE)
  await page.waitForURL((u) => !u.pathname.startsWith('/login'), { timeout: 15000 })
  await page.waitForLoadState('networkidle').catch(() => {})
}

async function go(page, path) {
  await page.evaluate((p) => {
    window.history.pushState({}, '', p)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }, path)
  await page.waitForLoadState('networkidle').catch(() => {})
  await page.waitForTimeout(500)
}

const browser = await chromium.launch()
const results = []

async function snap(page, vp, theme, route, errors) {
  const m = await page.evaluate(measure)
  const name = `${route.replace(/\W+/g, '_') || 'root'}-${vp.name}-${theme}`
  await page.screenshot({ path: join(outDir, `${name}.png`), fullPage: false })
  results.push({ route, at: new URL(page.url()).pathname, viewport: vp.name, theme, ...m, errors: [...errors] })
  errors.length = 0
}

// One sign-in for the whole run (code requests are rate limited): the same
// page is resized and re-themed between passes.
async function auditAll() {
  const ctx = await browser.newContext({ isMobile: true, hasTouch: true, viewport: { width: 390, height: 844 } })
  const page = await ctx.newPage()
  const errors = []
  page.on('pageerror', (e) => errors.push(String(e)))
  for (const theme of THEMES) {
    await page.emulateMedia({ colorScheme: theme })
    for (const vp of VIEWPORTS) {
      await page.setViewportSize({ width: vp.width, height: vp.height })
      await page.goto(`${BASE}/login`)
      await page.waitForLoadState('networkidle').catch(() => {})
      if (new URL(page.url()).pathname.startsWith('/login')) await snap(page, vp, theme, '/login', errors)
    }
  }
  await page.setViewportSize({ width: 390, height: 844 })
  await signIn(page)
  for (const theme of THEMES) {
    await page.emulateMedia({ colorScheme: theme })
    for (const vp of VIEWPORTS) {
      await page.setViewportSize({ width: vp.width, height: vp.height })
      for (const r of ROUTES) {
        await go(page, r)
        await snap(page, vp, theme, r, errors)
      }
      for (const f of FOLLOW) {
        await go(page, f.from)
        const link = page.locator(f.link).first()
        if (!(await link.count())) continue
        await link.click()
        await page.waitForLoadState('networkidle').catch(() => {})
        await page.waitForTimeout(500)
        await snap(page, vp, theme, f.from + '/:id', errors)
      }
    }
  }
  await ctx.close()
}

await auditAll()
await browser.close()
writeFileSync(join(outDir, 'audit.json'), JSON.stringify(results, null, 2))
const bad = results.filter((r) => r.pageOverflowX || r.overflow.length || r.tinyInputs.length)
console.log(`${results.length} cases; ${bad.length} with overflow or small inputs; small targets in ${results.filter((r) => r.smallTargetCount).length}`)
