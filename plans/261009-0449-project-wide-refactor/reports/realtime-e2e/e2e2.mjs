import fs from 'node:fs'
import { chromium, signIn, api, switchTenant, run, BASE } from './setup.mjs'
const OUT = '/home/zane/Desktop/projects/nexus-enterprise/plans/261009-0449-project-wide-refactor/reports/realtime-e2e'
const log = []
const say = (m) => { console.log(m); log.push(m) }
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

function varint(b, i) { let n = 0, sh = 0; for (;;) { const c = b[i++]; n |= (c & 0x7f) << sh; if (!(c & 0x80)) break; sh += 7 } return [n, i] }
// -> {k:'domain', domain, kind, ws} | {k:...}
function decode(b) {
  const t = b[0]
  if (t === 0x92 && b[1] === 0x01) {
    let [len, i] = varint(b, 2)
    const end = i + len, out = { k: 'domain' }
    while (i < end) {
      const tag = b[i++]; const f = tag >> 3, w = tag & 7
      if (w === 2) { let l; [l, i] = varint(b, i); const s = b.subarray(i, i + l).toString(); i += l; if (f === 1) out.domain = s; if (f === 2) out.kind = s }
      else if (w === 0) { [, i] = varint(b, i) } else break
    }
    return out
  }
  if (t === 0x82 && b[1] === 0x01) return { k: 'approval' }
  if (t === 0x9a && b[1] === 0x01) return { k: 'subscribed' }
  if (t === 0x8a && b[1] === 0x01) return { k: 'presence' }
  if (t === 0x0a) return { k: 'auth' }
  return { k: 'other' }
}
async function watch(page, g) {
  const frames = [], live = []
  await page.routeWebSocket(/\/api\/ws/, (ws) => {
    if (g.blocked) { ws.close({ code: 1006 }); return }
    const server = ws.connectToServer(); live.push(ws)
    ws.onMessage((m) => server.send(m))
    server.onMessage((m) => { frames.push({ t: Date.now(), ...decode(Buffer.from(m)) }); ws.send(m) })
    server.onClose(() => ws.close()); ws.onClose(() => server.close())
  })
  return { frames, live }
}
async function waitText(page, text, ms = 8000) {
  const t0 = Date.now()
  try { await page.getByText(text).first().waitFor({ state: 'visible', timeout: ms }); return Date.now() - t0 } catch { return -1 }
}
const browser = await chromium.launch()
const mk = () => browser.newContext({ viewport: { width: 1360, height: 860 } })
async function seed(ctx, who) {
  await ctx.addInitScript((u) => localStorage.setItem('ngac-auth', JSON.stringify({ state: { user: u }, version: 0 })),
    { id: who.user.id, username: who.user.username, ngac_node_id: who.user.ngac_node_id })
}
const [ctxA, ctxB, ctxD] = [await mk(), await mk(), await mk()]
const A = await signIn(ctxA, `e2e2-a-${run}@example.test`, 'Alice')
const B = await signIn(ctxB, `e2e2-b-${run}@example.test`, 'Binh')
const D = await signIn(ctxD, `e2e2-d-${run}@example.test`, 'Dung')
for (const [c, w] of [[ctxA, A], [ctxB, B], [ctxD, D]]) await seed(c, w)
let r = await api(ctxA, A, 'POST', '/me/workspaces', { name: 'E2E Review ' + run })
const W = r.body.id
await switchTenant(ctxA, A, W)
await api(ctxA, A, 'POST', `/admin/tenants/${W}/provision`)
async function join(ctx, who, email) {
  await api(ctxA, A, 'POST', `/workspaces/${W}/members`, { email })
  r = await api(ctx, who, 'GET', '/invitations')
  await api(ctx, who, 'POST', `/invitations/${r.body.invitations[0].id}/accept`)
  await switchTenant(ctx, who, W)
}
await join(ctxB, B, `e2e2-b-${run}@example.test`)
r = await api(ctxA, A, 'POST', '/approval/templates', {
  name: `Duyệt ${run}`, entity_type: 'general', priority: 1, conditions: [],
  steps: [{ step_order: 1, name: 'Duyệt', approver_type: 'specific_user', approver_value: B.user.ngac_node_id, required_count: 1, timeout_hours: 24 }], form_fields: [],
})
const template = r.body.id
const pageB = await ctxB.newPage()
const gateB = { blocked: false }
const wB = await watch(pageB, gateB)
await pageB.goto(`${BASE}/drive?ws=${W}`)
await sleep(2500)

// ---- (c) create a folder: B gets the drive event and no permission event
let mark = wB.frames.length
r = await api(ctxA, A, 'POST', `/workspaces/${W}/drive/folders`, { name: `Thư mục mới ${run}` })
const folder = r.body.id
let ms = await waitText(pageB, `Thư mục mới ${run}`)
await sleep(3500) // settle (750 ms) + retry window
let got = wB.frames.slice(mark)
say(`(c) folder created: B sees it in ${ms} ms; B frames = ${JSON.stringify(got.map((f) => f.k + (f.domain ? ':' + f.domain + '/' + f.kind : '')))}`)
const permToB = got.filter((f) => f.domain === 'permission').length
say(`(c) permission frames delivered to B (a bystander) for a folder creation: ${permToB}`)
await pageB.screenshot({ path: `${OUT}/09-review-c-folder-created.png` })
// narrow audience: D joins; only D is named by the permission event
const wD = await watch(await ctxD.newPage(), { blocked: false })
mark = wB.frames.length
await api(ctxA, A, 'POST', `/workspaces/${W}/members`, { email: `e2e2-d-${run}@example.test` })
r = await api(ctxD, D, 'GET', '/invitations')
await api(ctxD, D, 'POST', `/invitations/${r.body.invitations[0].id}/accept`)
await sleep(3500)
got = wB.frames.slice(mark)
say(`(c) D joins: frames to bystander B = ${JSON.stringify(got.map((f) => f.k + (f.domain ? ':' + f.domain + '/' + f.kind : '')))}`)
say(`(c) permission frames to B when someone else joins: ${got.filter((f) => f.domain === 'permission').length}`)

// ---- (b) token refresh in B's tab
await pageB.goto(`${BASE}/approval?ws=${W}`)
await sleep(2500)
const authsBefore = wB.frames.filter((f) => f.k === 'auth').length
let tripped = false
await pageB.route('**/api/approval/my-requests*', (route) => { if (!tripped) { tripped = true; say('(b) intercepted one request with 401'); return route.fulfill({ status: 401, body: '{"code":"session_required","message":"expired"}', contentType: 'application/json' }) } return route.continue() })
await pageB.getByText('Tôi đã gửi').first().click()
await sleep(1500)
await sleep(4000)
await pageB.getByText('Chờ tôi duyệt').first().click()
await sleep(1000)
const authsAfter = wB.frames.filter((f) => f.k === 'auth').length
say(`(b) forced 401 -> token refresh; socket re-authenticated: ${authsAfter > authsBefore} (auth frames ${authsBefore} -> ${authsAfter})`)
mark = wB.frames.length
r = await api(ctxA, A, 'POST', '/approval/requests', { template_id: template, entity_type: 'general', entity_id: '', form_data_json: '{}' })
ms = await waitText(pageB, `Duyệt ${run}`, 8000)
say(`(b) after the refresh B still receives events: approval visible in ${ms} ms; subscribed frames after = ${wB.frames.slice(mark).length > 0}`)
await pageB.screenshot({ path: `${OUT}/10-review-b-after-token-refresh.png` })
await pageB.goto(`${BASE}/drive?ws=${W}`)
await sleep(2500)
const newName = `Sau refresh ${run}`
r = await api(ctxA, A, 'PUT', `/drive/items/${folder}/rename`, { name: newName, new_name: newName })
ms = await waitText(pageB, newName)
say(`(b) workspace stream after refresh: folder rename visible in ${ms} ms`)

// ---- (a) remove B while the tab is open
r = await api(ctxA, A, 'GET', `/workspaces/${W}/members`)
mark = wB.frames.length
const t0 = Date.now()
r = await api(ctxA, A, 'DELETE', `/workspaces/${W}/members/${B.user.ngac_node_id}`)
say('(a) remove B: ' + r.status)
await sleep(1200)
const after = `Sau khi bị xoá ${run}`
r = await api(ctxA, A, 'PUT', `/drive/items/${folder}/rename`, { name: after, new_name: after })
r = await api(ctxA, A, 'POST', `/workspaces/${W}/drive/folders`, { name: `Không được thấy ${run}` })
await sleep(3000)
got = wB.frames.slice(mark)
const leaked = got.filter((f) => f.k === 'domain' && f.domain !== 'workspace' && f.domain !== 'permission')
say(`(a) frames to B after removal: ${JSON.stringify(got.map((f) => f.k + (f.domain ? ':' + f.domain + '/' + f.kind : '')))}`)
say(`(a) drive/doc/asset frames received after removal (must be 0): ${leaked.length}`)
await pageB.screenshot({ path: `${OUT}/11-review-a-after-removal.png` })
fs.appendFileSync(`${OUT}/e2e-log.txt`, '\n--- review fixes: checks a, b, c ---\n' + log.join('\n') + '\n')
await browser.close()
