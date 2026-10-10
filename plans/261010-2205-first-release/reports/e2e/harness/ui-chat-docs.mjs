import { launch, newPage, step, shot, signInUI, enterWorkspace, tokens, sleep, saveResults, BASE, run, sql, COMPOSER, compose } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
import { request } from './lib.mjs'
import { api } from './lib.mjs'
await launch()
const WS = WORLD.wsA, WSNAME = `QA Tenant A ${WORLD.run}`
const T = tokens()
const api2 = await request.newContext({ baseURL: BASE })
const F5 = 'flow5-chat-realtime', F8 = 'flow8-documents'
const { page: O, ctx: cO } = await newPage()
const { page: M } = await newPage()
const ph = /Tin nhắn cho|Viết tin nhắn/
await step(F5, 'owner signs in (UI) and enters the workspace', async () => { await signInUI(O, WORLD.O.email, null); return enterWorkspace(O, WSNAME) }, O)
await step(F5, 'member M3 signs in (UI) and enters the workspace', async () => { await signInUI(M, WORLD.M3.email, null); return enterWorkspace(M, WSNAME) }, M)
const CH = sql(`select id from channels where workspace_id = '${WS}' and name = 'general' limit 1`)
const openGeneral = async (p) => { await p.getByText('general', { exact: true }).nth(0).click(); await p.locator(COMPOSER).first().waitFor({ timeout: 15000 }); await sleep(500) }
await step(F5, 'both open #general from the channel list', async () => { await openGeneral(O); await openGeneral(M); return CH }, M)
const msg = `Xin chào từ chủ ${run}`
await step(F5, 'owner posts; member sees it live (no reload)', async () => {
  const t0 = Date.now()
  await compose(O, msg)
  await M.getByText(msg).first().waitFor({ timeout: 10000 })
  return `live in ${Date.now() - t0} ms`
}, M)
const reply = `Đã nhận ${run}`
await step(F5, 'member replies; owner sees it live', async () => {
  const t0 = Date.now()
  await compose(M, reply)
  await O.getByText(reply).first().waitFor({ timeout: 10000 })
  return `live in ${Date.now() - t0} ms`
}, O)
await step(F5, 'member pins owner message (Ghim); owner sees the pin live', async () => {
  await M.getByText(msg).first().hover()
  await M.getByRole('button', { name: /^Ghim/ }).first().click({ timeout: 6000 })
  await O.getByLabel('Đã ghim').first().waitFor({ timeout: 10000 })
  await shot(O, '30-pin-live')
  return 'pin visible to owner'
}, O)
await step(F5, 'poll created via API; member sees the question live', async () => {
  const r = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/polls`, { question: `Họp thứ Sáu? ${run}`, options: ['Có', 'Không'], multiple: false, anonymous: false })
  if (r.status >= 300) throw new Error('create poll ' + r.status + JSON.stringify(r.body).slice(0, 200))
  await M.getByText(`Họp thứ Sáu? ${run}`).first().waitFor({ timeout: 10000 })
  await shot(M, '32-poll-live')
  return `poll ${r.body.id || ''}`
}, M)
await step(F5, 'task created via API; member sees it live', async () => {
  const r = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/tasks`, { title: `Soạn biên bản ${run}`, assignee_id: '' })
  if (r.status >= 300) throw new Error('create task ' + r.status + JSON.stringify(r.body).slice(0, 200))
  await M.getByText(`Soạn biên bản ${run}`).first().waitFor({ timeout: 10000 })
  return 'task visible to member'
}, M)
// ---- documents
await step(F8, 'owner creates a text document ("Văn bản mới")', async () => {
  await O.goto(`${BASE}/documents?ws=${WS}`, { waitUntil: 'networkidle' })
  await O.getByRole('button', { name: 'Văn bản mới' }).first().click()
  await O.waitForURL(/documents\//, { timeout: 15000 })
  return O.url()
}, O)
const docUrl = O.url()
const docId = docUrl.split('/documents/')[1].split('?')[0]
await step(F8, 'type in the editor; autosave shows "Đã lưu"', async () => {
  await O.locator('[contenteditable="true"]').first().click()
  await O.keyboard.type('Bản nháp một ' + run)
  await O.getByRole('status').filter({ hasText: /Đã lưu/ }).first().waitFor({ timeout: 15000 })
  return 'autosaved'
}, O)
await step(F8, 'member (read on Documents) opens the document and sees the text', async () => {
  await M.goto(docUrl, { waitUntil: 'networkidle' })
  await M.getByText('Bản nháp một ' + run).first().waitFor({ timeout: 10000 })
  return 'visible'
}, M)
const T2 = await cO.newPage()
T2.errors = []; T2.on('pageerror', (e) => T2.errors.push(e.message)); T2.on('dialog', (d) => { T2.errors.push('DIALOG ' + d.message()); d.dismiss() })
await step(F8, 'second tab of the owner opens the same document', async () => { await T2.goto(docUrl, { waitUntil: 'networkidle' }); await T2.getByText('Bản nháp một ' + run).first().waitFor({ timeout: 10000 }); return 'open' }, T2)
await step(F8, 'tab 1 saves a newer version', async () => {
  await O.locator('[contenteditable="true"]').first().click(); await O.keyboard.press('End'); await O.keyboard.type(' — tab 1')
  await O.getByRole('status').filter({ hasText: /Đã lưu/ }).first().waitFor({ timeout: 15000 })
  return 'saved'
}, O)
await step(F8, 'tab 2 writes from the old version: conflict dialog, no silent overwrite', async () => {
  await T2.locator('[contenteditable="true"]').first().click(); await T2.keyboard.press('End'); await T2.keyboard.type(' — tab 2 cũ')
  await T2.getByRole('dialog').getByRole('button', { name: 'Tải lại bản mới nhất' }).waitFor({ timeout: 20000 })
  await shot(T2, '31-conflict-dialog')
  return 'conflict dialog shown'
}, T2)
await step(F8, 'conflict "Tải lại bản mới nhất" loads the server version', async () => {
  await T2.getByRole('dialog').getByRole('button', { name: 'Tải lại bản mới nhất' }).click()
  await T2.getByText(' — tab 1').first().waitFor({ timeout: 10000 })
  return 'server version shown'
}, T2)
// sanitisation: hostile markup stored through the API must not execute or render as live markup
const cur = await api(api2, { token: T.O }, 'GET', `/documents/texts/${docId}`)
const hostile = '<p>An toàn</p><img src="x" onerror="window.__pwned=1"><a href="javascript:window.__pwned=2">bấm đây</a><script>window.__pwned=3</script><iframe src="javascript:window.__pwned=4"></iframe>'
await step(F8, 'hostile markup stored via API (script, onerror, javascript: link, iframe)', async () => {
  const r = await api(api2, { token: T.O }, 'PATCH', `/documents/texts/${docId}`, { base_version: cur.body.version, content: hostile })
  if (r.status !== 200) throw new Error('patch ' + r.status + JSON.stringify(r.body).slice(0, 200))
  return 'stored'
}, O)
await step(F8, 'member opens it: no script runs, no event handler, no javascript: link, no iframe', async () => {
  const errsBefore = M.errors.length
  await M.goto(docUrl, { waitUntil: 'networkidle' })
  await M.getByText('An toàn').first().waitFor({ timeout: 10000 })
  await sleep(800)
  const pw = await M.evaluate(() => window.__pwned ?? null)
  const dom = await M.evaluate(() => ({
    onerrorImg: document.querySelectorAll('img[onerror]').length,
    jsLinks: [...document.querySelectorAll('a')].filter((a) => /^javascript:/i.test(a.getAttribute('href') || '')).length,
    iframes: document.querySelectorAll('iframe').length,
    scripts: [...document.querySelectorAll('script')].filter((s) => /__pwned/.test(s.textContent || '')).length,
  }))
  const dialogs = M.errors.slice(errsBefore).filter((e) => e.startsWith('DIALOG'))
  await shot(M, '33-sanitised-doc')
  if (pw !== null || dom.onerrorImg || dom.jsLinks || dom.iframes || dom.scripts || dialogs.length) throw new Error(`EXECUTED pwned=${pw} dom=${JSON.stringify(dom)} dialogs=${dialogs.length}`)
  return JSON.stringify(dom)
}, M)
console.log('owner errors', O.errors.slice(0, 5), 'member errors', M.errors.slice(0, 5), 'T2', T2.errors.slice(0, 3))
saveResults('flow5-8-ui')
process.exit(0)
