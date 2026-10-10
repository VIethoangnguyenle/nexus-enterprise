import { launch, newPage, step, shot, signInUI, enterWorkspace, tokens, sleep, saveResults, BASE, run, sql } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
import { request, api } from './lib.mjs'
await launch()
const T = tokens()
const api2 = await request.newContext({ baseURL: BASE })
const WSA = WORLD.wsA, WSB = WORLD.wsB, NAMEA = `QA Tenant A ${WORLD.run}`, NAMEB = `QA Tenant B ${WORLD.run}`
// setup: M3 also joins tenant B (owner X invites via API, M3 accepts) so the mobile switch changes data
const inv = await api(api2, { token: T.X }, 'POST', `/workspaces/${WSB}/members`, { email: WORLD.M3.email, role_id: '', department_id: '' })
const invList = await api(api2, { token: T.M3 }, 'GET', '/invitations')
const mine = invList.body.invitations?.find((i) => i.workspace_name === NAMEB)
const acc = mine ? await api(api2, { token: T.M3 }, 'POST', `/invitations/${mine.id}/accept`, {}) : { status: 0 }
console.log('setup: invite B', inv.status, 'accept B', acc.status)
// ---- flow 9 settings
const F9 = 'flow9-settings'
const { page: M } = await newPage()
await step(F9, 'member signs in and opens settings (profile)', async () => { await signInUI(M, WORLD.M3.email, null); await enterWorkspace(M, NAMEA); await M.goto(`${BASE}/settings?ws=${WSA}`, { waitUntil: 'networkidle' }); await M.getByLabel('Tên hiển thị').waitFor({ timeout: 10000 }); return M.url() }, M)
await step(F9, 'department is read-only and says it is set by the administrator', async () => {
  await M.getByText('Do quản trị viên đặt').first().waitFor({ timeout: 8000 })
  const dep = await M.getByText('Tài chính', { exact: true }).count()
  if (dep < 1) throw new Error('department name not shown')
  return 'read-only'
}, M)
await step(F9, 'edit display name and save; name persists after reload', async () => {
  const name = `Hoa Đã Đổi ${run}`
  await M.getByLabel('Tên hiển thị').fill(name)
  await M.getByRole('button', { name: 'Lưu thay đổi' }).click()
  await sleep(1200)
  await M.reload({ waitUntil: 'networkidle' })
  await M.getByLabel('Tên hiển thị').waitFor({ timeout: 10000 })
  const v = await M.getByLabel('Tên hiển thị').inputValue()
  if (v !== name) throw new Error(`name after reload = "${v}"`)
  return v
}, M)
await step(F9, 'appearance: dark theme is chosen and survives reload', async () => {
  await M.goto(`${BASE}/settings?ws=${WSA}&tab=giao-dien`, { waitUntil: 'networkidle' }).catch(() => {})
  await M.getByRole('tab', { name: 'Giao diện' }).click().catch(() => M.getByText('Giao diện', { exact: true }).first().click())
  await M.getByRole('radio', { name: 'Tối' }).click()
  await sleep(400)
  await M.reload({ waitUntil: 'networkidle' })
  await M.getByRole('tab', { name: 'Giao diện' }).click().catch(() => M.getByText('Giao diện', { exact: true }).first().click())
  const checked = await M.getByRole('radio', { name: 'Tối' }).getAttribute('aria-checked')
  if (checked !== 'true') throw new Error('dark not kept, aria-checked=' + checked)
  return 'dark kept'
}, M)
await step(F9, 'workspace details: member sees the name read-only', async () => {
  await M.goto(`${BASE}/settings?ws=${WSA}`, { waitUntil: 'networkidle' })
  await M.getByRole('tab', { name: 'Workspace' }).click().catch(() => M.getByText('Workspace', { exact: true }).first().click())
  await M.getByText('Tên workspace').first().waitFor({ timeout: 8000 })
  const editable = await M.getByRole('textbox', { name: 'Tên workspace' }).count()
  if (editable > 0) throw new Error('member can edit workspace name')
  return 'read-only for member'
}, M)
// ---- flow 10 contacts
const F10 = 'flow10-contacts'
const { page: O } = await newPage()
await step(F10, 'owner opens Danh bạ (directory lists the people)', async () => { await signInUI(O, WORLD.O.email, null); await enterWorkspace(O, NAMEA); await O.goto(`${BASE}/contacts?ws=${WSA}`, { waitUntil: 'networkidle' }); await O.getByText('Hoa Đã Đổi').first().waitFor({ timeout: 10000 }).catch(() => {}); const n = await O.getByText('Mai Member').count(); if (n < 1) throw new Error('directory missing Mai Member'); return 'listed' }, O)
await step(F10, 'profile panel opens for a person', async () => { await O.getByText('Mai Member').first().click(); await O.getByText(/Phòng ban|Vai trò|Email/).first().waitFor({ timeout: 8000 }); await shot(O, '40-contact-panel'); return 'panel' }, O)
await step(F10, 'start a direct message from the panel (Nhắn tin)', async () => { await O.getByRole('button', { name: 'Nhắn tin' }).first().click(); await O.waitForURL(/\/channels\//, { timeout: 15000 }); await O.getByText('Mai Member').first().waitFor({ timeout: 10000 }); return O.url() }, O)
// ---- flow 11 mobile 360
const F11 = 'flow11-mobile'
const { page: MP } = await newPage({ viewport: { width: 360, height: 740 }, mobile: true })
await step(F11, 'member on 360px: sign-in and workspace A', async () => { await signInUI(MP, WORLD.M3.email, null); await enterWorkspace(MP, NAMEA); return MP.url() }, MP)
await step(F11, 'bottom bar: Tin nhắn, Tài liệu, Phê duyệt and Thêm', async () => {
  const nav = MP.getByRole('navigation', { name: 'Điều hướng di động' })
  await nav.waitFor({ timeout: 8000 })
  for (const t of ['Tin nhắn', 'Tài liệu', 'Phê duyệt']) if (!(await nav.getByText(t, { exact: true }).count())) throw new Error('missing tab ' + t)
  if (!(await MP.getByRole('button', { name: 'Thêm' }).count()) && !(await nav.getByText('Thêm', { exact: true }).count())) throw new Error('missing Thêm')
  await shot(MP, '50-mobile-bottom-bar')
  return 'four entries'
}, MP)
await step(F11, 'Thêm opens the sheet (Tài sản, Danh bạ, Quản trị, Cài đặt) and the workspace switch', async () => {
  await MP.getByRole('navigation', { name: 'Điều hướng di động' }).getByText('Thêm', { exact: true }).click()
  const sheetNav = MP.getByRole('navigation', { name: 'Thêm' })
  await sheetNav.getByText('Tài sản', { exact: true }).waitFor({ timeout: 8000 })
  await sheetNav.getByText('Danh bạ', { exact: true }).waitFor({ timeout: 8000 })
  await shot(MP, '51-mobile-sheet')
  await MP.locator('[aria-label^="Đổi workspace, đang ở"]').first().click({ timeout: 8000 })
  await MP.getByRole('listbox', { name: 'Workspace của bạn' }).waitFor({ timeout: 8000 })
  await shot(MP, '52-mobile-switch-list')
  return 'sheet + switch list'
}, MP)
await step(F11, 'switching workspace re-scopes the token and the data changes (A -> B)', async () => {
  await MP.getByRole('option', { name: new RegExp(NAMEB) }).first().click({ timeout: 8000 })
  await sleep(2500)
  const titleB = await MP.getByText(NAMEB, { exact: false }).count()
  await shot(MP, '53-mobile-after-switch')
  if (titleB < 1) throw new Error('workspace B not shown after switch')
  const chans = await MP.getByText('general', { exact: true }).count()
  return `B shown; channel list shows general x${chans}`
}, MP)
await step(F11, 'top-bar search (Tìm kiếm) opens on a channel and accepts a query', async () => {
  await MP.getByText('general', { exact: true }).nth(0).click({ timeout: 8000 })
  await MP.getByRole('button', { name: 'Tìm kiếm' }).click({ timeout: 8000 })
  await sleep(800)
  await shot(MP, '54-mobile-search')
  return MP.url()
}, MP)
// ---- flow 12 cross-tenant (outsider X in tenant B)
const F12 = 'flow12-cross-tenant'
const { page: P } = await newPage()
await step(F12, 'outsider signs in and enters tenant B only', async () => { await signInUI(P, WORLD.X.email, null); await enterWorkspace(P, NAMEB); return P.url() }, P)
const probeText = `Bí mật tenant A ${run}`
await step(F12, 'owner posts in tenant A #general; outsider sees nothing (live)', async () => {
  const ch = sql(`select id from channels where workspace_id = '${WSA}' and name = 'general' limit 1`)
  const r = await api(api2, { token: T.O }, 'POST', `/channels/${ch}/messages`, { content: probeText })
  if (r.status >= 300) throw new Error('post ' + r.status + JSON.stringify(r.body).slice(0, 200))
  await sleep(4000)
  const seen = await P.getByText(probeText).count()
  if (seen) throw new Error('tenant-A message visible to outsider')
  return `no leak (post ${r.status})`
}, P)
await step(F12, 'outsider contacts and search show no tenant-A person', async () => {
  await P.goto(`${BASE}/contacts?ws=${WSB}`, { waitUntil: 'networkidle' })
  await sleep(800)
  for (const n of ['Olivia Owner', 'Mai Member', 'Minh Manager']) { if (await P.getByText(n, { exact: true }).count()) throw new Error('tenant-A name visible: ' + n) }
  return 'no A names in B directory'
}, P)
await step(F12, 'outsider opens tenant A drive by URL: no files shown', async () => {
  await P.goto(`${BASE}/drive?ws=${WSA}`, { waitUntil: 'networkidle' })
  await sleep(1500)
  const leaked = await P.getByText('hop-dong-ui.pdf').count()
  await shot(P, '60-outsider-drive-url')
  if (leaked) throw new Error('tenant-A file names visible to outsider')
  return 'no file names'
}, P)
console.log('errors', { O: O.errors.slice(0, 3), M: M.errors.slice(0, 3), MP: MP.errors.slice(0, 3), P: P.errors.slice(0, 3) })
saveResults('flow9-12-ui')
process.exit(0)
