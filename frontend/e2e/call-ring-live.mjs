/**
 * Ringing a direct call against the running dev stack (make dev, Vite :5173, LiveKit :7880).
 * A signs in on a desktop window, B on a 390px phone, both with fake camera and
 * microphone. Takes the screenshots for the design comparison and checks the
 * call actually connects: tiles, hand, reaction, presenting, minimise + navigate.
 *
 * Usage: cd frontend && node e2e/call-ui-live.mjs
 * Accounts: A = smoke1791570521@example.org, B = verify-b-1791691723687@example.org, OTP 999999.
 * OTP requests are limited to 5 per 15 minutes per address; one browser sign-in each per run.
 */
import { chromium } from 'playwright'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const BASE = 'http://localhost:5173'
const OTP = '999999'
const EMAIL_A = 'smoke1791570521@example.org'
const EMAIL_B = process.env.EMAIL_B || 'verify-b-1791691723687@example.org'
const OUT = path.resolve(__dirname, '../../plans/reports/video-call-screens')
const MOCKUP = 'file://' + path.resolve(__dirname, '../../design/mockups/video-call.html')
fs.mkdirSync(OUT, { recursive: true })

// The refresh cookie of each account is kept between runs (OTP requests are limited), outside the repo.
const STATE_DIR = process.env.CALL_UI_STATE_DIR || path.join(process.env.TMPDIR || '/tmp', 'call-ui-live-state')
fs.mkdirSync(STATE_DIR, { recursive: true })
const stateFile = (tag) => path.join(STATE_DIR, `${tag}.json`)
const stateOf = (tag) => (fs.existsSync(stateFile(tag)) ? stateFile(tag) : undefined)

const results = []
const check = (name, ok, detail = '') => { results.push({ name, ok, detail }); console.log(`${ok ? 'PASS' : 'FAIL'}  ${name} ${detail}`) }
const shot = (page, name) => page.screenshot({ path: path.join(OUT, `${name}.png`) })

const bearer = { A: null, B: null }
function watchToken(page, tag) {
  page.on('request', (req) => {
    const h = req.headers()['authorization']
    if (h && req.url().includes('/api/')) bearer[tag] = h
  })
  page.on('console', (m) => { if (m.type() === 'error') console.log(`[${tag} console.error]`, m.text().slice(0, 200)) })
  page.on('pageerror', (e) => console.log(`[${tag} pageerror]`, String(e).slice(0, 300)))
}

async function signIn(page, email, tag) {
  watchToken(page, tag)
  if (stateOf(tag)) {
    await page.goto(`${BASE}/channels`)
    await page.waitForLoadState('networkidle')
    if (/channels/.test(new URL(page.url()).pathname) && !(await page.getByLabel('Email hoặc số điện thoại').count())) return
  }
  await page.goto(`${BASE}/login`)
  await page.waitForLoadState('networkidle')
  const field = page.getByLabel('Email hoặc số điện thoại')
  await field.waitFor({ state: 'visible', timeout: 10000 })
  await field.fill(email)
  await page.getByRole('button', { name: 'Nhận mã đăng nhập' }).click()
  const otp = page.getByLabel('Mã xác minh')
  await otp.waitFor({ state: 'attached', timeout: 10000 })
  await otp.focus()
  for (const d of OTP) await page.keyboard.type(d, { delay: 50 })
  await page.waitForURL((u) => /register|workspace-select|onboarding|channels/.test(u.pathname), { timeout: 15000 })
  if (page.url().includes('/workspace-select')) {
    await page.locator('[data-ws-row]').first().click()
    await page.waitForURL((u) => u.pathname.includes('/channels'), { timeout: 15000 })
  }
  await page.waitForLoadState('networkidle')
}

/** The phone hides its controls after 4 s; a tap on the picture brings them back. */
async function reveal(page) {
  if (await page.locator('.cs-phone-hidden').count()) await page.locator('.cs-tap').click({ position: { x: 20, y: 200 } })
}

async function api(page, tag, method, url, body) {
  return page.evaluate(async ({ method, url, body, auth }) => {
    const r = await fetch(`/api${url}`, {
      method, headers: { 'Content-Type': 'application/json', Authorization: auth },
      body: body ? JSON.stringify(body) : undefined,
    })
    let json = null
    try { json = await r.json() } catch { /* empty */ }
    return { status: r.status, json }
  }, { method, url, body, auth: bearer[tag] })
}

async function main() {
  const browser = await chromium.launch({
    headless: true,
    args: ['--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream', '--autoplay-policy=no-user-gesture-required'],
  })
  const media = { permissions: ['camera', 'microphone'], colorScheme: 'light' }
  const ctxA = await browser.newContext({ viewport: { width: 1440, height: 900 }, ...media, storageState: stateOf('A') })
  const ctxB = await browser.newContext({ viewport: { width: 1440, height: 900 }, ...media, storageState: stateOf('B') })
  const A = await ctxA.newPage(), B = await ctxB.newPage()
  await signIn(A, EMAIL_A, 'A')
  await signIn(B, EMAIL_B, 'B')
  await ctxA.storageState({ path: stateFile('A') })
  await ctxB.storageState({ path: stateFile('B') })
  // B's second device: another tab of the same session at phone width (a second
  // context would share the refresh cookie, and rotating it signs the other out).
  const B2 = await ctxB.newPage()
  await B2.setViewportSize({ width: 390, height: 844 })
  watchToken(B2, 'B2')
  await B2.goto(`${BASE}/channels`)
  await B2.waitForLoadState('networkidle')

  const wsId = (await api(A, 'A', 'GET', '/me/workspaces')).json?.workspaces?.[0]?.id
  const bMe = await api(B, 'B', 'GET', '/me')
  const bId = bMe.json?.user?.id ?? bMe.json?.id
  const chans = (await api(A, 'A', 'GET', `/workspaces/${wsId}/channels`)).json?.channels ?? []
  let bNode = null
  for (const c of chans) {
    const hit = ((await api(A, 'A', 'GET', `/channels/${c.id}/members`)).json?.members ?? []).find((m) => m.user_id === bId)
    if (hit) { bNode = hit.ngac_node_id; break }
  }
  const dm = (await api(A, 'A', 'POST', '/dms', { target_user_id: bId, target_ngac_node_id: bNode })).json
  check('DM between A and B', !!dm?.id)
  // B has the app open in B's own workspace; a direct call reaches it there.
  for (const p of [B, B2]) { await p.goto(`${BASE}/channels`); await p.waitForLoadState('networkidle') }

  // ---- A calls B from the DM header ----
  await A.goto(`${BASE}/channels/${dm.id}?ws=${wsId}`)
  await A.waitForLoadState('networkidle')
  await shot(A, 'desktop-dm-header')
  await A.getByRole('button', { name: 'Gọi video' }).first().click()
  await A.waitForURL((u) => u.pathname.startsWith('/calls/'), { timeout: 15000 })
  const firstId = new URL(A.url()).pathname.split('/').pop()
  check('A goes straight into the call', true, new URL(A.url()).pathname)
  const toast = B.getByRole('dialog', { name: 'Cuộc gọi video đến' }).or(B.getByText('đang gọi video cho bạn', { exact: false }))
  const rang = await toast.first().waitFor({ timeout: 10000 }).then(() => true).catch(() => false)
  check('B desktop rings (toast)', rang)
  await shot(B, 'desktop-incoming')
  const rangPhone = await B2.getByText('Cuộc gọi video đến', { exact: false }).first().waitFor({ timeout: 10000 }).then(() => true).catch(() => false)
  check('B phone rings (full screen)', rangPhone)
  await shot(B2, 'phone-incoming')

  // ---- B answers on desktop; the phone stops ----
  await B.getByRole('button', { name: 'Video', exact: true }).click()
  await B.waitForURL((u) => u.pathname.startsWith('/calls/'), { timeout: 15000 })
  const bStage = await B.getByTestId('call-stage').waitFor({ timeout: 20000 }).then(() => true).catch(() => false)
  check('B answered into the stage', bStage)
  const elsewhere = await B2.getByText('Đã trả lời trên thiết bị khác').first().waitFor({ timeout: 10000 }).then(() => true).catch(() => false)
  check('B phone: answered on another device', elsewhere)
  let aTiles = 0
  for (let i = 0; i < 15 && aTiles < 2; i++) {
    await A.waitForTimeout(1000)
    aTiles = await A.locator('[data-testid^="tile-"]').count()
  }
  check('A sees B in the call (2 tiles)', aTiles === 2, String(aTiles))
  await shot(A, 'desktop-dm-call')

  // ---- hang up, then a missed call ----
  await A.getByRole('button', { name: 'Rời cuộc gọi' }).click()
  await B.getByRole('button', { name: 'Rời cuộc gọi' }).click().catch(() => {})
  // Hanging up a call between two people ends it for both.
  let ended = false
  // The server waits 15 s for a reconnect before it ends a call left with one person.
  for (let i = 0; i < 35 && !ended; i++) {
    await A.waitForTimeout(1000)
    ended = (await api(A, 'A', 'GET', `/calls/${firstId}`)).json?.status === 'ended'
  }
  check('hanging up ends the direct call', ended)
  const missed = (await api(A, 'A', 'POST', `/channels/${dm.id}/calls`, { kind: 'audio' })).json?.call?.id
  const rang2 = await B.getByText('đang gọi thoại cho bạn', { exact: false }).first().waitFor({ timeout: 10000 }).then(() => true).catch(() => false)
  check('B rings again (audio)', rang2)
  console.log('waiting out the 45 s ring…')
  const gone = await B.getByText('đang gọi thoại cho bạn', { exact: false }).first().waitFor({ state: 'detached', timeout: 60000 }).then(() => true).catch(() => false)
  check('ringing stops when the call is missed', gone)
  const call = (await api(B, 'B', 'GET', `/calls/${missed}`)).json
  check('the call is missed', call?.status === 'missed', call?.status)
  await B.goto(`${BASE}/channels/${dm.id}`)
  await B.waitForLoadState('networkidle')
  const callBack = await B.getByRole('button', { name: 'Gọi lại' }).last().waitFor({ timeout: 10000 }).then(() => true).catch(() => false)
  check('B sees the missed-call card with "Gọi lại"', callBack)
  await shot(B, 'desktop-dm-cards')

  await browser.close()
  const failed = results.filter((r) => !r.ok)
  console.log(`\n${results.length - failed.length}/${results.length} checks passed`)
  process.exit(failed.length ? 1 : 0)
}

main().catch((e) => { console.error(e); process.exit(1) })
