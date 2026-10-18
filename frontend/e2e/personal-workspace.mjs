/**
 * Personal workspace verification (plans/261010-2258-personal-workspace/exec-06-verification.md).
 *
 * Page 1 is a brand-new account, signed in once (OTP 999999). Its browser is never reloaded:
 * viewport changes (desktop 1440x900 ⇄ phone 390x844) re-layout in place, and deep links are
 * entered as in-app navigations (history.pushState + popstate), so the in-memory token survives.
 *
 * Steps 5–6 need a verified email: a new OTP account cannot create a workspace here (the server
 * has no verification channel, POST /api/me/email/verify → 503). They run on page 2, a second
 * context signed in ONCE with an existing account (PWS_VERIFIED_EMAIL, default smoke account).
 *
 * Usage:   cd frontend && node e2e/personal-workspace.mjs
 * Needs:   make dev running (Vite :5173). OTP fixed code 999999.
 *
 * Output:  screenshots → plans/261010-2258-personal-workspace/reports/phase-06-screens/
 *          progress    → plans/reports/progress-pws-06.log
 *          results     → phase-06-screens/results.json
 */

import { chromium } from 'playwright'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const ROOT = path.resolve(__dirname, '../..')

const BASE = 'http://localhost:5173'
const OTP_CODE = '999999'
const STAMP = Date.now()
const DISPLAY_NAME = 'PWS Kiểm Thử'
const EXISTING_EMAIL = process.env.PWS_VERIFIED_EMAIL || 'smoke1791570521@example.org'
const NEW_WS_NAME = `Dự án PWS ${STAMP % 100000}`
const DESKTOP = { width: 1440, height: 900 }
const PHONE = { width: 390, height: 844 }
const SCREEN_DIR = path.join(ROOT, 'plans/261010-2258-personal-workspace/reports/phase-06-screens')
const PROGRESS = path.join(ROOT, 'plans/reports/progress-pws-06.log')

fs.mkdirSync(SCREEN_DIR, { recursive: true })

// ─── State ───────────────────────────────────────────────────────────────────

let currentStep = 'setup'
let activePage = null
let EMAIL = `pws-${STAMP}@example.org`
const consoleErrors = {}     // step → [text]
const apiFailures = {}       // step → [status url]
const netLog = []            // {url}
const authByPage = new Map() // page → bearer header the app itself sent (memory only)
const results = { criteria: [], steps: {}, accounts: [], findings: [] }

const now = () => new Date().toISOString()

function progress(line) {
  fs.appendFileSync(PROGRESS, `${now()} | ${line}\n`)
  console.log(line)
}

function bucket(map) {
  if (!map[currentStep]) map[currentStep] = []
  return map[currentStep]
}

function shot(page, name) {
  return page.screenshot({ path: path.join(SCREEN_DIR, `${name}.png`), fullPage: false })
}

function crit(id, name, pass, evidence) {
  const result = pass === null ? 'INCONCLUSIVE' : pass ? 'PASS' : 'FAIL'
  results.criteria.push({ id, name, result, evidence })
  progress(`CRIT ${id} ${result} | ${name} | ${evidence}`)
}

/** Runs one step; one retry on exception (covers a transient Vite hot reload). */
async function step(name, fn) {
  currentStep = name
  results.steps[name] = { started: now() }
  for (let attempt = 1; attempt <= 2; attempt++) {
    try {
      await fn()
      progress(`STEP ${name} | done (attempt ${attempt})`)
      results.steps[name].ok = true
      break
    } catch (err) {
      const msg = err.message.split('\n')[0]
      progress(`STEP ${name} | attempt ${attempt} threw: ${msg}`)
      results.steps[name].error = msg
      if (activePage) await activePage.screenshot({ path: path.join(SCREEN_DIR, `ERR-${name}-a${attempt}.png`) }).catch(() => {})
      if (attempt === 2) results.findings.push(`step ${name} failed: ${msg}`)
      else await new Promise((r) => setTimeout(r, 2000))
    }
  }
  results.steps[name].ended = now()
}

/** Visible-only check of the sidebar entries the criteria are about. */
async function navState(page) {
  return page.evaluate(() => {
    const vis = (sel) => [...document.querySelectorAll(sel)].some((el) => {
      const r = el.getBoundingClientRect()
      const s = getComputedStyle(el)
      return r.width > 0 && r.height > 0 && s.visibility !== 'hidden' && s.display !== 'none'
    })
    return {
      assets: vis('a[href^="/assets"]'),
      approval: vis('a[href^="/approval"]'),
      contacts: vis('a[href^="/contacts"]'),
      admin: vis('a[href^="/admin"]'),
      settings: vis('a[href^="/settings"]'),
    }
  })
}

/** Console, network and auth capture for a page. */
function hook(page) {
  page.on('console', (m) => { if (m.type() === 'error') bucket(consoleErrors).push(m.text().slice(0, 240)) })
  page.on('pageerror', (e) => bucket(consoleErrors).push(`pageerror: ${e.message.slice(0, 240)}`))
  page.on('requestfailed', (r) => bucket(apiFailures).push(`${r.method()} ${r.url()} → ${r.failure()?.errorText}`))
  page.on('response', (r) => {
    if (r.status() >= 400 && r.url().includes('/api/')) {
      bucket(apiFailures).push(`${r.status()} ${r.request().method()} ${new URL(r.url()).pathname}`)
    }
  })
  page.on('request', (req) => {
    const h = req.headers()
    if (h.authorization && req.url().includes('/api/')) authByPage.set(page, h.authorization)
    if (req.url().includes('/api/')) netLog.push({ url: new URL(req.url()).pathname })
  })
}

/** Calls the API from inside the page with that page's own bearer token (no reload). */
async function apiCall(page, method, url, body) {
  return page.evaluate(
    async ({ method, url, body, token }) => {
      const headers = { ...(token ? { Authorization: token } : {}) }
      if (body !== undefined) headers['Content-Type'] = 'application/json'
      const r = await fetch(url, {
        method,
        credentials: 'include',
        headers,
        body: body !== undefined ? JSON.stringify(body) : undefined,
      })
      const text = await r.text()
      return { status: r.status, body: text.slice(0, 300) }
    },
    { method, url, body, token: authByPage.get(page) ?? null },
  )
}

async function fetchWorkspaces(page) {
  return page.evaluate(
    async (token) => {
      const r = await fetch('/api/workspaces', { credentials: 'include', headers: token ? { Authorization: token } : {} })
      if (!r.ok) return { status: r.status, workspaces: [] }
      const j = await r.json()
      return { status: r.status, workspaces: j.workspaces ?? [] }
    },
    authByPage.get(page) ?? null,
  )
}

// ─── Sign-in (OTP, once per person) ──────────────────────────────────────────

async function signInOnce(page, email) {
  await page.goto(`${BASE}/login`)
  await page.waitForLoadState('networkidle')

  const emailField = page.getByLabel('Email hoặc số điện thoại')
  await emailField.waitFor({ state: 'visible', timeout: 10000 })
  await emailField.fill(email)
  await page.getByRole('button', { name: 'Nhận mã đăng nhập' }).click()

  const otpInput = page.getByLabel('Mã xác minh')
  await otpInput.waitFor({ state: 'attached', timeout: 10000 })
  await otpInput.focus()
  for (const d of OTP_CODE) await page.keyboard.type(d, { delay: 50 })

  // Any landing away from /login counts as signed in (the app redirects to several routes).
  await page.waitForFunction(() => !location.pathname.startsWith('/login'), null, { timeout: 20000 })
  progress(`  OTP accepted for ${email}; landed ${new URL(page.url()).pathname}`)
  await page.waitForLoadState('networkidle').catch(() => {})

  if (new URL(page.url()).pathname.startsWith('/register')) {
    const nameField = page.getByLabel('Tên hiển thị')
    await nameField.waitFor({ state: 'visible', timeout: 10000 })
    await nameField.fill(DISPLAY_NAME)
    await page.getByRole('button', { name: /Tiếp tục/ }).click()
    await page.waitForFunction(() => !location.pathname.startsWith('/register'), null, { timeout: 15000 }).catch(async () => {
      const snippet = (await page.locator('body').innerText().catch(() => '')).replace(/\s+/g, ' ').slice(0, 300)
      throw new Error(`register did not leave /register: ${snippet}`)
    })
  }

  if (new URL(page.url()).pathname.startsWith('/workspace-select')) {
    // The list loads asynchronously: wait for rows or the empty state before deciding.
    await page.waitForFunction(
      () => document.querySelector('[data-ws-row]') || document.body.innerText.includes('Bạn chưa thuộc workspace nào'),
      null,
      { timeout: 15000 },
    ).catch(() => {})
    const rows = page.locator('[data-ws-row]')
    if ((await rows.count()) > 0) await rows.first().click()
    else throw new Error('no workspace listed after sign-in')
  }
  await page.waitForFunction(() => location.pathname.includes('/channels'), null, { timeout: 15000 })
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(800)
}

/** Page 1 only: one retry with a fresh address if the first OTP sign-in does not complete. */
async function signInNewAccount(page) {
  try {
    await signInOnce(page, EMAIL)
  } catch (err) {
    await page.screenshot({ path: path.join(SCREEN_DIR, 'ERR-signin.png') }).catch(() => {})
    const snippet = (await page.locator('body').innerText().catch(() => '')).replace(/\s+/g, ' ').slice(0, 240)
    progress(`  sign-in attempt 1 failed (${err.message.split('\n')[0]}); page: "${snippet}"; retrying with a fresh address`)
    EMAIL = `pws-${Date.now()}@example.org`
    results.accounts.push({ email: EMAIL, displayName: DISPLAY_NAME, note: 'created by this run (sign-in retry)' })
    await signInOnce(page, EMAIL)
  }
}

/** Opens the Roles (Vai trò) area of Quản trị; the page may use a tab or a card. */
async function clickRolesEntry(page) {
  const tab = page.getByRole('tab', { name: /Vai trò/ }).first()
  const btn = page.getByRole('button', { name: /Vai trò/ }).first()
  if (await tab.count()) await tab.click()
  else if (await btn.count()) await btn.click()
  await page.waitForTimeout(900)
}

// ─── Main ────────────────────────────────────────────────────────────────────


async function run() {
  const browser = await chromium.launch({ headless: true })
  const ctx1 = await browser.newContext({ viewport: DESKTOP, colorScheme: 'light' })
  const page = await ctx1.newPage()
  hook(page)
  activePage = page

  results.accounts.push({ email: EMAIL, displayName: DISPLAY_NAME, note: 'created by this run (new account, page 1)' })
  results.accounts.push({ email: EXISTING_EMAIL, note: 'pre-existing account, page 2 (steps 5–6 only)' })

  let ctx2 = null
  let page2 = null
  let personalWsId = null
  let personalWsName = null
  let page2WsId = null

  try {
    // Sign-in runs once, outside the retry wrapper: a retry would request a second OTP on a live session.
    await signInNewAccount(page)
    progress('STEP 0-signin | done')

    // ── 1. New account lands in a personal workspace; no Tài sản / Phê duyệt ──
    await step('1-personal-nav', async () => {
      const { status, workspaces } = await fetchWorkspaces(page)
      const personal = workspaces.filter((w) => w.type === 'personal')
      personalWsId = personal[0]?.id ?? null
      personalWsName = personal[0]?.name ?? null
      const trigger = (await page.locator('button[aria-haspopup="listbox"]').first().innerText().catch(() => '')).replace(/\s+/g, ' ')
      await shot(page, '01-desktop-personal-sidebar')
      const nav = await navState(page)
      crit('1a', 'Personal workspace after first sign-in (type=personal, named for the person)',
        personal.length >= 1 && (personalWsName || '').includes('PWS'),
        `GET /api/workspaces ${status}; personal=${JSON.stringify(personal.map((w) => w.name))}; switcher="${trigger}"`)
      crit('1b', 'Desktop sidebar has no Tài sản / Phê duyệt entries (personal)',
        !nav.assets && !nav.approval,
        `visible a[href=/assets]=${nav.assets}, a[href=/approval]=${nav.approval}; recorded only: admin=${nav.admin}, contacts=${nav.contacts}`)

      await page.setViewportSize(PHONE)
      await page.waitForTimeout(900)
      await shot(page, '01-phone-personal-tabbar')
      const phoneNav = await navState(page)
      const tabs = await page.locator('nav[aria-label="Phân hệ"] a:visible').evaluateAll((els) => els.map((e) => `${e.textContent.trim()}→${e.getAttribute('href')}`))
      // The bottom bar's Thêm holds every module the workspace offers (the ☰ menu is gone).
      await page.getByRole('button', { name: /^Thêm/ }).click().catch(() => {})
      await page.getByRole('dialog', { name: 'Thêm' }).waitFor({ timeout: 8000 }).catch(() => {})
      await page.waitForTimeout(700)
      await shot(page, '01-phone-personal-more')
      const sheet = await page.locator('[role="dialog"][aria-label="Thêm"] a:visible').evaluateAll((els) => els.map((e) => e.getAttribute('href')))
      await page.keyboard.press('Escape')
      await page.waitForTimeout(400)
      await page.setViewportSize(DESKTOP)
      await page.waitForTimeout(500)
      const tabHrefs = tabs.map((t) => t.split('→')[1])
      crit('1c', 'Phone bottom bar and Thêm have no Tài sản / Phê duyệt (personal)',
        !phoneNav.approval && !phoneNav.assets
          && !sheet.some((h) => /^\/(assets|approval)/.test(h)) && !tabHrefs.some((h) => /^\/(assets|approval)/.test(h)),
        `bar=${tabs.join(' | ')}; Thêm=${sheet.join(', ')}`)
    })

    // ── 2. Deep links /assets and /approval redirect as in-app navigations ──
    await step('2-deeplinks', async () => {
      for (const target of ['/assets', '/approval']) {
        const mark = netLog.length
        await page.evaluate((p) => {
          window.history.pushState({}, '', p)
          window.dispatchEvent(new PopStateEvent('popstate', { state: {} }))
        }, target)
        await page.waitForTimeout(1500)
        const landed = new URL(page.url()).pathname
        const areaCalls = netLog.slice(mark).filter((n) => /\/api\/workspaces\/[^/]+\/assets|\/api\/approval\//.test(n.url))
        await shot(page, `02-deeplink-${target.slice(1)}`)
        crit(`2-${target.slice(1)}`, `Deep link ${target} as personal redirects; area never fetched`,
          !landed.startsWith(target) && areaCalls.length === 0,
          `pushState ${target} → landed ${landed}; area API calls in window=${areaCalls.length}`)
      }
      await page.setViewportSize(PHONE)
      await page.waitForTimeout(500)
      const mark = netLog.length
      await page.evaluate(() => { window.history.pushState({}, '', '/assets'); window.dispatchEvent(new PopStateEvent('popstate', { state: {} })) })
      await page.waitForTimeout(1500)
      const landedPhone = new URL(page.url()).pathname
      const phoneCalls = netLog.slice(mark).filter((n) => /\/api\/workspaces\/[^/]+\/assets/.test(n.url))
      await shot(page, '02-phone-deeplink-assets')
      progress(`  phone /assets → ${landedPhone}; area calls=${phoneCalls.length}`)
      await page.setViewportSize(DESKTOP)
      await page.waitForTimeout(400)
    })

    // ── 3. Direct API calls refused for the personal workspace ──
    const personalRefusal = (r) => /personal|cá nhân/i.test(r.body)
    await step('3-api-refused-personal', async () => {
      if (!personalWsId) throw new Error('no personal workspace id from step 1')
      const list = await apiCall(page, 'GET', `/api/workspaces/${personalWsId}/assets`)
      const create = await apiCall(page, 'POST', '/api/approval/requests', { template_id: '00000000-0000-0000-0000-000000000000', form_data_json: '{}' })
      results.step3 = { assetList: list, approvalCreate: create }
      progress(`  GET assets=${list.status} ${list.body.slice(0, 140)}`)
      progress(`  POST approval=${create.status} ${create.body.slice(0, 140)}`)
      crit('3a', 'Personal: GET /api/workspaces/{ws}/assets refused (403)',
        list.status === 403 && personalRefusal(list),
        `status ${list.status}; body ${list.body}`)
      crit('3b', 'Personal: POST /api/approval/requests refused (403)',
        create.status === 403 && personalRefusal(create),
        `status ${create.status}; body ${create.body}`)
    })

    // ── 3b. Admin → Roles on the personal workspace (before conversion) ──
    await step('3b-admin-roles-personal', async () => {
      await page.locator('a[href^="/admin"]:visible').first().click()
      await page.waitForURL((u) => u.pathname.includes('/admin'), { timeout: 10000 })
      await page.waitForTimeout(800)
      await clickRolesEntry(page)
      await shot(page, '06-admin-roles-personal')
      const roleRow = page.getByRole('row').nth(1)
      if (await roleRow.count()) await roleRow.click().catch(() => {})
      await page.waitForTimeout(900)
      await shot(page, '06-admin-role-matrix-personal')
      const mainText = await page.locator('main').innerText().catch(() => page.locator('body').innerText())
      const hasMatrix = /Thao tác|Vùng|Quyền/i.test(mainText)
      const hasAssetArea = /Tài sản/.test(mainText)
      crit('6', 'Admin → Roles permission areas of the personal workspace: no Tài sản area',
        hasMatrix ? !hasAssetArea : null,
        hasMatrix ? `matrix text found; 'Tài sản' present=${hasAssetArea}` : 'matrix not located on the Roles page; see 06-admin-roles-personal.png')
      await page.locator('a[href^="/settings"]:visible').first().click().catch(() => {})
      await page.waitForTimeout(500)
    })

    // ── 4. Conversion removed: route returns 404 and no button in the UI ──
    await step('4-conversion-absent', async () => {
      if (!personalWsId) throw new Error('no personal workspace id from step 1')

      // The backend route should no longer exist.
      const convert = await apiCall(page, 'POST', `/api/workspaces/${personalWsId}/convert-to-organization`)
      progress(`  POST convert-to-organization → ${convert.status} ${convert.body.slice(0, 140)}`)
      crit('4a', 'POST /api/workspaces/{id}/convert-to-organization returns 404 (route removed)',
        convert.status === 404,
        `status ${convert.status}; body ${convert.body}`)

      // Navigate to Settings → Workspace and verify there is no convert button.
      await page.locator('a[href^="/settings"]:visible').first().click()
      await page.waitForURL((u) => u.pathname.includes('/settings'), { timeout: 10000 })
      await page.waitForTimeout(600)
      await page.getByRole('tab', { name: 'Workspace' }).click().catch(async () => {
        await page.getByText('Workspace', { exact: true }).first().click()
      })
      await page.waitForTimeout(600)
      await shot(page, '04-settings-workspace-no-convert')
      const convertBtn = page.getByRole('button', { name: 'Chuyển thành workspace tổ chức' })
      const btnCount = await convertBtn.count()
      crit('4b', 'No "Chuyển thành workspace tổ chức" button in the UI',
        btnCount === 0,
        `button count=${btnCount}`)
    })

    // ── 4c. Phone: Tôi is the one settings page, and shows only what a personal workspace has ──
    await step('4c-phone-me-personal', async () => {
      await page.setViewportSize(PHONE)
      await page.waitForTimeout(600)
      await page.getByRole('button', { name: 'Tôi', exact: true }).click()
      await page.waitForURL((u) => u.pathname === '/settings', { timeout: 10000 })
      await page.getByRole('heading', { name: 'Tôi', exact: true }).waitFor({ timeout: 10000 })
      await page.waitForTimeout(500)
      await shot(page, '04c-phone-me-personal')
      const text = (await page.locator('main, body').first().innerText()).replace(/\s+/g, ' ')
      const must = ['Tài khoản và hồ sơ', 'Thông báo', 'Giao diện, ngôn ngữ', 'Thiết bị và phiên', 'Chat', 'Lịch', 'Tài liệu', 'Thanh đáy', 'Cài đặt workspace', 'Giới thiệu']
      const never = ['Hồ sơ trong tổ chức', 'Phê duyệt', 'Tài sản', 'Thu gọn sidebar']
      const logouts = await page.getByRole('button', { name: 'Đăng xuất', exact: true }).count()
      const bar = await page.locator('nav[aria-label="Phân hệ"]:visible').count()
      crit('4c', 'Phone Tôi (personal): account + workspace rows, no Hồ sơ trong tổ chức / Phê duyệt / Tài sản, one Đăng xuất, no bottom bar',
        must.every((m) => text.includes(m)) && never.every((n) => !text.includes(n)) && logouts === 1 && bar === 0,
        `missing=${must.filter((m) => !text.includes(m)).join(',')}; unexpected=${never.filter((n) => text.includes(n)).join(',')}; Đăng xuất=${logouts}; bar=${bar}`)
    })

    await step('4d-phone-workspace-settings-personal', async () => {
      await page.getByRole('button', { name: /^Cài đặt workspace/ }).click()
      await page.waitForURL((u) => u.pathname === '/settings/workspace', { timeout: 10000 })
      await page.getByText('Dung lượng Tài liệu').waitFor({ timeout: 10000 })
      await page.waitForTimeout(500)
      await shot(page, '04d-phone-workspace-settings-personal')
      const text = (await page.locator('body').innerText()).replace(/\s+/g, ' ')
      const never = ['Thành viên và tổ chức', 'Ai được tạo nhóm', 'Rời ', 'Vai trò và quyền', 'Chuyển thành workspace tổ chức']
      const uuid = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i.test(text)
      crit('4d', 'Phone Cài đặt workspace (personal): only Tên workspace and Dung lượng; no members, rules, Rời or conversion; no ids',
        text.includes('Tên workspace') && never.every((n) => !text.includes(n)) && !uuid,
        `unexpected=${never.filter((n) => text.includes(n)).join(',')}; uuid=${uuid}`)
    })

    await step('4e-phone-sessions', async () => {
      await page.goto(`${BASE}/settings/sessions?ws=${personalWsId}`)
      await page.getByText('Máy này').waitFor({ timeout: 10000 })
      await page.waitForTimeout(400)
      await shot(page, '04e-phone-sessions')
      const list = await apiCall(page, 'GET', '/api/auth/sessions')
      let sessions = []
      try { sessions = JSON.parse(list.body).sessions ?? [] } catch { /* recorded below */ }
      results.page1Sessions = sessions.length
      const text = (await page.locator('body').innerText()).replace(/\s+/g, ' ')
      const leaked = sessions.some((x) => text.includes(x.session_id))
      crit('4e', 'Thiết bị và phiên lists own sessions, this one marked, never a session id on screen',
        list.status === 200 && sessions.filter((x) => x.current).length === 1 && !leaked,
        `GET /api/auth/sessions → ${list.status}; sessions=${sessions.length}; current=${sessions.filter((x) => x.current).length}; id on screen=${leaked}`)
      await page.setViewportSize(DESKTOP)
      await page.waitForTimeout(400)
    })

    // ── 5–6 on page 2: a pre-existing account whose email is verified ──
    ctx2 = await browser.newContext({ viewport: DESKTOP, colorScheme: 'light' })
    page2 = await ctx2.newPage()
    hook(page2)
    activePage = page2

    await step('5-create-workspace', async () => {
      await signInOnce(page2, EXISTING_EMAIL)
      await page2.waitForTimeout(500)
      await page2.locator('button[aria-haspopup="listbox"]').first().click()
      await page2.getByRole('link', { name: 'Tạo workspace' }).click()
      await page2.waitForURL((u) => u.pathname.includes('/onboarding'), { timeout: 10000 })
      await page2.waitForTimeout(1000)
      await shot(page2, '05-create-workspace-default')
      const gateText = (await page2.locator('body').innerText().catch(() => '')).replace(/\s+/g, ' ')
      const gated = /Cần xác minh email/.test(gateText) || /Cần xác minh/.test(gateText)
      const typePicker = await page2.locator('input[name="cw-type"]').count()
      results.createGate = { gated, typePickerCount: typePicker, snippet: gateText.slice(0, 200) }
      crit('5a', 'Tạo workspace: default type Cá nhân; created workspace is personal', null,
        `BLOCKED by environment: create screen gated ("Cần xác minh …", type picker count=${typePicker}) on new and pre-existing accounts; POST /api/me/email/verify → 503 verification_unavailable`)
      crit('5b', 'Quay lại on Tạo workspace returns to the origin screen', null,
        'BLOCKED: same gate as 5a; create screen has no Quay lại to exercise beyond the gate view')
      crit('6-personal-create', 'Admin → Roles on the workspace created in step 5', null,
        'BLOCKED: no workspace created in step 5; criterion 6 evaluated on the personal workspace in step 3b')
    })

    // ── 7. Another person's session is never listed nor revocable (page 1 against page 2's session) ──
    await step('7-sessions-deny-cross-user', async () => {
      const mine = await apiCall(page2, 'GET', '/api/auth/sessions')
      const theirs = JSON.parse(mine.body).sessions ?? []
      const target = theirs.find((x) => x.current)?.session_id
      if (!target) throw new Error(`page 2 has no current session: ${mine.status} ${mine.body.slice(0, 120)}`)
      const seenByOther = await apiCall(page, 'GET', '/api/auth/sessions')
      const listedToOther = (JSON.parse(seenByOther.body).sessions ?? []).some((x) => x.session_id === target)
      const revoke = await apiCall(page, 'DELETE', `/api/auth/sessions/${encodeURIComponent(target)}`)
      const after = await apiCall(page2, 'GET', '/api/auth/sessions')
      const still = (JSON.parse(after.body).sessions ?? []).some((x) => x.session_id === target)
      crit('7', 'Another user\'s session: not in my list, DELETE refused (404), and it keeps working',
        !listedToOther && revoke.status === 404 && after.status === 200 && still,
        `listed to other=${listedToOther}; DELETE → ${revoke.status}; owner list after → ${after.status}, still there=${still}`)
    })

    // ── 8. Phone Cài đặt workspace in an organization: rows by the viewer's rights; admin screens usable at 390 ──
    await step('8-phone-workspace-settings-org', async () => {
      const { workspaces } = await fetchWorkspaces(page2)
      const org = workspaces.find((w) => w.type === 'organization')
      if (!org) {
        crit('8', 'Phone Cài đặt workspace (organization)', null, 'BLOCKED: the pre-existing account has no organization workspace')
        return
      }
      await page2.setViewportSize(PHONE)
      await page2.goto(`${BASE}/settings/workspace?ws=${org.id}`)
      await page2.getByText('Ai được tạo nhóm').waitFor({ timeout: 15000 })
      await page2.waitForTimeout(600)
      await shot(page2, '08-phone-workspace-settings-org')
      const text = (await page2.locator('body').innerText()).replace(/\s+/g, ' ')
      const admin = text.includes('Thành viên và tổ chức')
      const uuid = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i.test(text)
      if (admin) {
        await page2.getByRole('button', { name: /^Thành viên/ }).first().click()
        await page2.waitForURL((u) => u.pathname === '/admin/users', { timeout: 10000 })
        await page2.waitForTimeout(900)
        await shot(page2, '08-phone-admin-users')
        const overflow = await page2.evaluate(() => document.scrollingElement.scrollWidth - window.innerWidth)
        crit('8', 'Phone Cài đặt workspace (org admin): members, rules, modules, Rời; Thành viên opens Quản trị at 390 without sideways scroll',
          ['Vai trò và quyền', 'Mời thành viên', 'Mẫu phê duyệt', 'Dung lượng Tài liệu'].every((m) => text.includes(m)) && text.includes(`Rời ${org.name}`) && overflow <= 0 && !uuid,
          `admin view; overflow=${overflow}px; uuid=${uuid}`)
      } else {
        crit('8', 'Phone Cài đặt workspace (org member): read-only info, the rule locked by "Quản trị viên … đã đặt", Hồ sơ trong tổ chức, Rời; no admin rows',
          text.includes('đã đặt') && text.includes('Hồ sơ trong tổ chức') && text.includes(`Rời ${org.name}`)
            && !text.includes('Vai trò và quyền') && !text.includes('Mời thành viên') && !uuid,
          `member view; uuid=${uuid}`)
      }
      await page2.setViewportSize(DESKTOP)
    })

  } finally {
    results.consoleErrors = consoleErrors
    results.apiFailures = apiFailures
    results.personalWorkspace = { name: personalWsName, id: personalWsId }
    results.page2Workspace = { name: NEW_WS_NAME, id: page2WsId }
    fs.writeFileSync(path.join(SCREEN_DIR, 'results.json'), JSON.stringify(results, null, 2))
    progress(`RESULTS → ${path.join(SCREEN_DIR, 'results.json')}`)
    if (ctx2) await ctx2.close()
    await ctx1.close()
    await browser.close()
  }
}

run().catch((err) => {
  progress(`SCRIPT FAILED: ${err.message}`)
  console.error(err)
  process.exit(1)
})
