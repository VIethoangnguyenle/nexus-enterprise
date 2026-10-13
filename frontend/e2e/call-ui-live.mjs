/**
 * Call screens against the running dev stack (make dev, Vite :5173, LiveKit :7880).
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
    args: [
      '--use-fake-ui-for-media-stream', '--use-fake-device-for-media-stream',
      '--auto-select-desktop-capture-source=Entire screen', '--autoplay-policy=no-user-gesture-required',
    ],
  })
  const ctxA = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: 'light', permissions: ['camera', 'microphone'], storageState: stateOf('A') })
  const ctxB = await browser.newContext({
    viewport: { width: 390, height: 844 }, deviceScaleFactor: 2, isMobile: true, hasTouch: true, colorScheme: 'light',
    permissions: ['camera', 'microphone'], storageState: stateOf('B'),
  })
  const A = await ctxA.newPage()
  const B = await ctxB.newPage()

  console.log('sign in A, B')
  await signIn(A, EMAIL_A, 'A')
  await signIn(B, EMAIL_B, 'B')
  await ctxA.storageState({ path: stateFile('A') })
  await ctxB.storageState({ path: stateFile('B') })

  // A group both belong to
  const wsRes = await api(A, 'A', 'GET', '/me/workspaces')
  const wsId = wsRes.json?.workspaces?.[0]?.id
  const bMe = await api(B, 'B', 'GET', '/me')
  const bId = bMe.json?.user?.id ?? bMe.json?.id
  console.log('workspace', wsId, 'B id', bId)
  const chans = (await api(A, 'A', 'GET', `/workspaces/${wsId}/channels`)).json?.channels ?? []
  let bNode = null
  for (const c of chans) {
    const mem = (await api(A, 'A', 'GET', `/channels/${c.id}/members`)).json?.members ?? []
    const hit = mem.find((m) => m.user_id === bId)
    if (hit) { bNode = hit.ngac_node_id; break }
  }
  if (!bNode) throw new Error('no shared channel between A and B')
  // A scratch group of its own, so removing B at the end touches nothing real.
  const group = (await api(A, 'A', 'POST', `/workspaces/${wsId}/channels`, { name: `Đối soát cuộc gọi ${new Date().toISOString().slice(11, 16)}`, channel_type: 'workspace' })).json
  const added = await api(A, 'A', 'POST', `/channels/${group.id}/members`, { ngac_node_id: bNode })
  check('scratch group with both', added.status < 300, String(added.status))
  console.log('group channel:', group.name)

  const started = await api(A, 'A', 'POST', `/channels/${group.id}/calls`, { kind: 'video' })
  check('start call (201/200)', started.status === 201 || started.status === 200, String(started.status))
  const callId = started.json?.call?.id
  const callUrl = `/calls/${callId}?ws=${wsId}`

  // ---- A: pre-join (desktop) ----
  await A.goto(BASE + callUrl)
  await A.getByText('Sẵn sàng tham gia?').waitFor({ timeout: 15000 })
  await A.waitForTimeout(1500)
  await shot(A, 'desktop-prejoin')
  check('pre-join shows', true)
  await A.getByRole('button', { name: 'Tham gia ngay' }).click()
  await A.getByTestId('call-stage').waitFor({ timeout: 20000 })
  check('A joined the stage', true)

  // ---- B: phone, answer=video (direct) ----
  await B.goto(BASE + callUrl)
  await B.getByText('Sẵn sàng tham gia?').waitFor({ timeout: 15000 })
  await B.waitForTimeout(1200)
  await shot(B, 'phone-prejoin')
  console.log('B micro options:', await B.locator('#pj-mic option').allTextContents())
  await B.getByRole('button', { name: 'Tham gia ngay' }).click()
  await B.getByTestId('phone-call').waitFor({ timeout: 20000 })
  check('B joined on phone', true)
  await A.waitForTimeout(3000)

  const tiles = await A.locator('[data-testid^="tile-"]').count()
  check('A sees 2 tiles', tiles === 2, String(tiles))
  const videos = await A.locator('video').count()
  check('A has video elements playing', videos >= 1, String(videos))
  await shot(A, 'desktop-stage-2')
  await shot(B, 'phone-1to1')
  console.log('element at bottom-left of phone:', await B.evaluate(() => { const e = document.elementFromPoint(40, 800); return e ? (e.outerHTML.slice(0, 200) + ' | z=' + getComputedStyle(e.closest('[style*=z-index], .fixed') || e).zIndex) : null }))

  // ---- hand + reaction ----
  await reveal(B); await B.getByRole('button', { name: 'Thêm' }).click()
  await B.getByRole('button', { name: 'Giơ tay' }).click()
  await A.waitForTimeout(800)
  const hand = await A.locator('[aria-label^="Đang giơ tay"]').count()
  check('A sees B hand raised', hand === 1, String(hand))
  await reveal(B); await B.getByRole('button', { name: 'Thêm' }).click()
  await B.getByRole('button', { name: 'Vỗ tay' }).click()
  await A.waitForTimeout(600)
  const floaters = await A.locator('.cs-float').count()
  check('A sees B reaction', floaters >= 1, String(floaters))
  await shot(A, 'desktop-hand-reaction')

  // A reacts, B sees; A raises hand, B sees (phone group view is 1-1 so check sheet badge)
  await A.getByRole('button', { name: 'Thả cảm xúc' }).click()
  await A.getByRole('menuitem', { name: 'Thích' }).click()
  await B.waitForTimeout(600)
  check('B sees A reaction', (await B.locator('.cs-float').count()) >= 1)

  // ---- side panel ----
  await A.getByRole('button', { name: 'Người tham gia' }).click()
  await A.waitForTimeout(800)
  await shot(A, 'desktop-panel-people')
  check('people panel lists both', (await A.locator('.cs-prow').count()) >= 2)
  await A.getByRole('tab', { name: 'Chat' }).click()
  await A.waitForTimeout(1500)
  await shot(A, 'desktop-panel-chat')
  const chatText = `chat trong cuộc gọi ${Date.now()}`
  const editor = A.locator('.cs-panel [contenteditable="true"]').first()
  if (await editor.count()) {
    await editor.click()
    await A.keyboard.type(chatText)
    await A.keyboard.press('Enter')
    await A.waitForTimeout(1200)
    check('chat message sent from call panel', (await A.locator('.cs-panel').getByText(chatText).count()) >= 1)
  } else check('chat editor present', false)
  await A.getByRole('button', { name: 'Đóng bảng' }).click()

  // ---- presenting ----
  await A.getByRole('button', { name: 'Trình bày màn hình' }).click()
  await A.waitForTimeout(2500)
  const presenting = await A.locator('.cs-prebar').count()
  check('A presenting (prebar)', presenting === 1)
  if (presenting) {
    await shot(A, 'desktop-presenting')
    await B.waitForTimeout(1500)
    check('B (phone) sees the presentation', (await B.locator('video').count()) >= 1)
    await shot(B, 'phone-presentation')
    await A.getByText('Dừng trình bày', { exact: true }).first().click()
  }

  // ---- phone sheet ----
  await reveal(B); await B.getByRole('button', { name: 'Thêm' }).click()
  await B.waitForTimeout(500)
  await shot(B, 'phone-sheet')
  await B.keyboard.press('Escape')
  await B.locator('.cs-sheet-scrim').click({ force: true }).catch(() => {})

  // ---- minimise + navigate keeps the call ----
  await A.locator('a[href^="/channels"]').first().click()
  await A.waitForTimeout(1500)
  const mini = await A.getByRole('region', { name: 'Cuộc gọi đang thu nhỏ' }).count()
  check('A navigated away: mini tile shows', mini === 1, `url=${A.url()}`)
  await shot(A, 'desktop-mini')
  const bTiles = await B.locator('[data-testid^="tile-"]').count()
  check('B still sees both (A still in the call)', bTiles === 2, String(bTiles))
  await A.getByRole('button', { name: 'Mở lại cuộc gọi' }).click()
  await A.waitForTimeout(1500)
  console.log('A url after reopen:', A.url())
  await shot(A, 'debug-reopen')
  await A.getByTestId('call-stage').waitFor({ timeout: 10000 })
  check('A reopens the same call', true)

  // ---- phone minimise ----
  await reveal(B); await B.getByRole('button', { name: 'Thu nhỏ cuộc gọi' }).click()
  await B.waitForTimeout(1500)
  check('B minimised to chat with mini tile', (await B.getByRole('region', { name: 'Cuộc gọi đang thu nhỏ' }).count()) === 1)
  await shot(B, 'phone-mini')

  // ---- mockup counterparts ----
  const M = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  await M.goto(MOCKUP)
  await M.waitForTimeout(800)
  for (const [id, name] of [['prejoin', 'mockup-prejoin'], ['call', 'mockup-stage'], ['present', 'mockup-present'], ['phone', 'mockup-phone'], ['states', 'mockup-states']]) {
    await M.locator(`#${id}`).screenshot({ path: path.join(OUT, `${name}.png`) })
  }

  // ---- B loses access: the server cuts B off ----
  const removedAt = Date.now()
  const removed = await api(A, 'A', 'DELETE', `/channels/${group.id}/members/${bNode}`)
  check('A removes B from the group', removed.status < 300, String(removed.status))
  const banner = await B.getByText('Bạn không còn ở trong nhóm này').waitFor({ timeout: 20000 }).then(() => true).catch(() => false)
  const cutOffMs = Date.now() - removedAt
  check('B is cut off with the danger banner', banner)
  // The spec promises a removed member leaves the call within 5 seconds.
  check('B is cut off within 5 s', banner && cutOffMs <= 5000, `${cutOffMs} ms`)
  await shot(B, 'phone-removed')

  // ---- leave ----
  await A.getByRole('button', { name: 'Rời cuộc gọi' }).click()
  await A.getByText('Cuộc gọi đã kết thúc').waitFor({ timeout: 10000 })
  await shot(A, 'desktop-ended')
  check('A leave shows summary', true)

  await ctxA.storageState({ path: stateFile('A') })
  await ctxB.storageState({ path: stateFile('B') })
  fs.writeFileSync(path.join(OUT, 'results.json'), JSON.stringify(results, null, 2))
  await browser.close()
  const failed = results.filter((r) => !r.ok)
  console.log(`\n${results.length - failed.length}/${results.length} checks passed`)
  process.exit(failed.length ? 1 : 0)
}

main().catch((e) => { console.error(e); process.exit(2) })
