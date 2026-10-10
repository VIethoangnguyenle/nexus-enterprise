import { chromium, BASE, run, sql, OUT, rec, saveResults } from './lib.mjs'
import fs from 'node:fs'
export { chromium, BASE, run, sql, OUT, rec, saveResults }
export { sleep as _s }
export const UI = `${OUT}/ui`
fs.mkdirSync(UI, { recursive: true })
export const sleep = (ms) => new Promise((r) => setTimeout(r, ms))
let browser
export async function launch() { browser = await chromium.launch(); return browser }
export async function newPage(opts = {}) {
  const ctx = await browser.newContext({ viewport: opts.viewport || { width: 1360, height: 860 }, isMobile: !!opts.mobile, hasTouch: !!opts.mobile })
  const page = await ctx.newPage()
  page.errors = []
  page.on('pageerror', (e) => page.errors.push(e.message))
  page.on('dialog', (d) => { page.errors.push('DIALOG:' + d.message()); d.dismiss() })
  return { ctx, page }
}
export async function shot(page, name) { await page.screenshot({ path: `${UI}/${name}.png`, fullPage: false }).catch(() => {}) }
// step: records PASS/FAIL, screenshot on failure, continues the flow
export async function step(flow, id, fn, page) {
  try {
    const detail = await fn()
    rec(flow, id, true, detail || '')
  } catch (e) {
    const name = `FAIL-${flow}-${id}`.replace(/[^A-Za-z0-9._-]+/g, '_').slice(0, 90)
    if (page) await shot(page, name)
    rec(flow, id, false, (e.message || String(e)).split('\n')[0].slice(0, 280) + (page ? ` [shot ${name}.png url=${page.url()}]` : ''))
  }
}
export async function signInUI(page, email, name) {
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' })
  await page.getByLabel('Email hoặc số điện thoại').fill(email)
  await page.getByRole('button', { name: 'Nhận mã đăng nhập' }).click()
  await page.getByText('Nhập mã 6 số').waitFor({ timeout: 15000 })
  await page.getByLabel('Mã xác minh').fill('999999')
  await page.waitForFunction(() => !location.pathname.startsWith('/login'), null, { timeout: 15000 })
  if (page.url().includes('/register') && name) {
    await page.getByLabel('Tên hiển thị').fill(name)
    await page.getByRole('button', { name: 'Tiếp tục' }).click()
    await page.waitForTimeout(1200)
  }
}
export function verifyEmail(email) { sql(`UPDATE users SET email_verified_at = now() WHERE lower(email) = lower('${email}')`) }
// pick a workspace on /workspace-select (the way a person does), then land on its channels
export async function enterWorkspace(page, wsName) {
  await page.goto(`${BASE}/workspace-select`, { waitUntil: 'networkidle' })
  await page.getByRole('heading', { name: 'Chọn workspace' }).waitFor({ timeout: 10000 })
  await page.getByText(wsName, { exact: false }).first().click()
  await page.waitForURL(/\/channels/, { timeout: 15000 })
  await sleep(500)
  return page.url()
}
export function tokens() { return JSON.parse(fs.readFileSync(`${process.env.SP_DIR}/world-tokens.json`, 'utf8')) }
// chat composer is a Tiptap contenteditable; its placeholder is the aria-label
export const COMPOSER = '[contenteditable="true"][aria-label^="Tin nhắn cho"], [contenteditable="true"][aria-label="Viết tin nhắn"]'
export async function compose(page, text) {
  const c = page.locator(COMPOSER).first()
  await c.waitFor({ timeout: 15000 })
  await c.click()
  await page.keyboard.type(text)
  await page.keyboard.press('Enter')
}
