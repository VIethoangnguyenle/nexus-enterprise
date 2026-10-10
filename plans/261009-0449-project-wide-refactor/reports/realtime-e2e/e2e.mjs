import fs from 'node:fs'
import { chromium, signIn, api, switchTenant, run, BASE } from './setup.mjs'

const OUT = '/home/zane/Desktop/projects/nexus-enterprise/plans/261009-0449-project-wide-refactor/reports/realtime-e2e'
fs.mkdirSync(OUT, { recursive: true })
const log = []
const say = (m) => { console.log(m); log.push(m) }
const sleep = (ms) => new Promise((r) => setTimeout(r, ms))

// Classify a ServerEnvelope frame by its first tag bytes.
function kind(buf) {
  const t = buf[0]
  if (t === 0x0a) return 'auth'
  if (t === 0x92 && buf[1] === 0x01) return 'domain'
  if (t === 0x82 && buf[1] === 0x01) return 'approval'
  if (t === 0x9a && buf[1] === 0x01) return 'subscribed'
  if (t === 0x8a && buf[1] === 0x01) return 'presence'
  if (t === 0x7a && buf[1] === undefined) return 'error'
  if (t === 0x7a) return 'error'
  return 'other:' + t
}
const gate = { blocked: false }
async function watch(page, name, g = gate) {
  const frames = []
  const live = []
  await page.routeWebSocket(/\/api\/ws/, (ws) => {
    if (g.blocked) { ws.close({ code: 1006 }); return }
    const server = ws.connectToServer()
    live.push(ws)
    ws.onMessage((m) => server.send(m))
    server.onMessage((m) => {
      const b = typeof m === 'string' ? Buffer.from(m) : Buffer.from(m)
      frames.push({ t: Date.now(), k: kind(b), n: b.length })
      ws.send(m)
    })
    server.onClose(() => ws.close())
  })
  return { frames, name, live, count: (k) => frames.filter((f) => f.k === k).length }
}

async function waitText(page, text, ms = 8000) {
  const t0 = Date.now()
  try {
    await page.getByText(text, { exact: false }).first().waitFor({ state: 'visible', timeout: ms })
    return Date.now() - t0
  } catch { return -1 }
}

const browser = await chromium.launch()
async function seedSession(ctx, who) {
  await ctx.addInitScript((u) => {
    localStorage.setItem('ngac-auth', JSON.stringify({ state: { user: u }, version: 0 }))
  }, { id: who.user.id, username: who.user.username, ngac_node_id: who.user.ngac_node_id })
}
const mk = async (name) => {
  const ctx = await browser.newContext({ viewport: { width: 1360, height: 860 } })
  return ctx
}
const ctxA = await mk('A'), ctxB = await mk('B'), ctxC = await mk('C'), ctxD = await mk('D')
const A = await signIn(ctxA, `e2e-a-${run}@example.test`, 'Alice')
const B = await signIn(ctxB, `e2e-b-${run}@example.test`, 'Binh')
const C = await signIn(ctxC, `e2e-c-${run}@example.test`, 'Cuong')
const D = await signIn(ctxD, `e2e-d-${run}@example.test`, 'Dung')

let r = await api(ctxA, A, 'POST', '/me/workspaces', { name: 'E2E Alpha ' + run })
const W = r.body.id
r = await api(ctxC, C, 'POST', '/me/workspaces', { name: 'E2E Beta ' + run })
const W2 = r.body.id
for (const [c, w] of [[ctxA, A], [ctxB, B], [ctxC, C], [ctxD, D]]) await seedSession(c, w)
await switchTenant(ctxA, A, W)
await switchTenant(ctxC, C, W2)
say(`workspace W=${W} (A,B,D)  other tenant W2=${W2} (C)`)

async function join(ctx, who, email) {
  r = await api(ctxA, A, 'POST', `/workspaces/${W}/members`, { email })
  if (r.status !== 202) throw new Error('invite ' + r.status + JSON.stringify(r.body))
  r = await api(ctx, who, 'GET', '/invitations')
  const inv = r.body.invitations.find((i) => true)
  r = await api(ctx, who, 'POST', `/invitations/${inv.id}/accept`)
  if (r.status >= 300) throw new Error('accept ' + r.status + JSON.stringify(r.body))
  await switchTenant(ctx, who, W)
}
await join(ctxB, B, `e2e-b-${run}@example.test`)

r = await api(ctxA, A, 'POST', `/admin/tenants/${W}/provision`)
say('provision approval schema ' + r.status)
// fixtures A owns
r = await api(ctxA, A, 'POST', `/workspaces/${W}/drive/folders`, { name: `Hồ sơ ${run}` })
const folder = r.body.id ?? r.body.item?.id ?? r.body.folder?.id
say('folder create ' + r.status + ' ' + folder)
r = await api(ctxA, A, 'POST', '/approval/templates', {
  name: `Tạm ứng ${run}`, entity_type: 'general', priority: 1, conditions: [],
  steps: [{ step_order: 1, name: 'Duyệt', approver_type: 'specific_user', approver_value: B.user.ngac_node_id, required_count: 1, timeout_hours: 24 }],
  form_fields: [],
})
const template = r.body.id
say('template ' + r.status + ' ' + template)

// ---- observers
const pageB = await ctxB.newPage()
await pageB.addInitScript(() => {
  const Orig = window.WebSocket
  window.__sockets = []
  window.WebSocket = function (...a) { const s = new Orig(...a); window.__sockets.push(s); return s }
  window.WebSocket.prototype = Orig.prototype
  Object.assign(window.WebSocket, { OPEN: 1, CLOSED: 3, CLOSING: 2, CONNECTING: 0 })
})
const gateB = { blocked: false }
const wB = await watch(pageB, 'B', gateB)
const pageC = await ctxC.newPage()
const wC = await watch(pageC, 'C', { blocked: false })
await pageB.goto(`${BASE}/drive?ws=${W}`)
await pageC.goto(`${BASE}/drive?ws=${W2}`)
await waitText(pageB, `Hồ sơ ${run}`, 15000)
await sleep(1500)
say(`B frames after load: ${JSON.stringify(wB.frames.map((f) => f.k))}`)
say(`C frames after load: ${JSON.stringify(wC.frames.map((f) => f.k))}`)
await pageB.screenshot({ path: `${OUT}/01-B-drive-before.png` })

// ---- 1. rename folder
let t0 = Date.now()
const newName = `Hồ sơ ĐỔI TÊN ${run}`
r = await api(ctxA, A, 'PUT', `/drive/items/${folder}/rename`, { name: newName, new_name: newName })
say('rename status ' + r.status)
let ms = await waitText(pageB, newName)
say(`1. folder rename visible on B in ${ms} ms`)
await pageB.screenshot({ path: `${OUT}/02-B-drive-renamed.png` })

// ---- 2. text document
await pageB.goto(`${BASE}/drive?ws=${W}&view=texts`)
await sleep(1500)
t0 = Date.now()
r = await api(ctxA, A, 'POST', `/workspaces/${W}/documents/texts`, { title: `Báo cáo ${run}` })
say('doc create ' + r.status + JSON.stringify(r.body).slice(0, 120))
ms = await waitText(pageB, `Báo cáo ${run}`)
say(`2. text document visible on B in ${ms} ms`)
await pageB.screenshot({ path: `${OUT}/03-B-documents-new.png` })

// ---- 3. add member (D accepts) while B watches contacts
await pageB.goto(`${BASE}/contacts?ws=${W}`)
await sleep(1500)
r = await api(ctxA, A, 'POST', `/workspaces/${W}/members`, { email: `e2e-d-${run}@example.test` })
r = await api(ctxD, D, 'GET', '/invitations')
const inv = r.body.invitations[0]
t0 = Date.now()
r = await api(ctxD, D, 'POST', `/invitations/${inv.id}/accept`)
say('accept ' + r.status)
ms = await waitText(pageB, 'Dung')
say(`3. new member visible on B contacts in ${ms} ms`)
await pageB.screenshot({ path: `${OUT}/04-B-contacts-new-member.png` })

// ---- 4. approval
await pageB.goto(`${BASE}/approval?ws=${W}`)
await sleep(1500)
t0 = Date.now()
r = await api(ctxA, A, 'POST', '/approval/requests', {
  template_id: template, entity_type: 'general', entity_id: '', form_data_json: JSON.stringify({ title: `Yêu cầu ${run}` }),
})
say('approval create ' + r.status + JSON.stringify(r.body).slice(0, 160))
const reqId = r.body.id
ms = await waitText(pageB, `Tạm ứng ${run}`)
say(`4. approval visible on B in ${ms} ms`)
await pageB.screenshot({ path: `${OUT}/05-B-approval-new.png` })

// ---- tenant isolation
await sleep(1000)
await pageC.screenshot({ path: `${OUT}/06-C-other-tenant.png` })
const cKinds = wC.frames.map((f) => f.k)
say(`C (other tenant) frames: ${JSON.stringify(cKinds)}`)
const leaked = cKinds.filter((k) => k === 'domain' || k === 'approval').length
say(`isolation: other-tenant domain/approval frames = ${leaked}`)
say(`B frames kinds: domain=${wB.count('domain')} approval=${wB.count('approval')} subscribed=${wB.count('subscribed')}`)

// ---- 5. 30s disconnect on B
await pageB.goto(`${BASE}/drive?ws=${W}`)
await sleep(2000)
const before = wB.frames.length
gateB.blocked = true
for (const ws of wB.live) { try { ws.close({ code: 4000 }) } catch {} }
say('B offline at ' + new Date().toISOString())
await sleep(2000)
const offName = `Hồ sơ KHI MẤT MẠNG ${run}`
r = await api(ctxA, A, 'PUT', `/drive/items/${folder}/rename`, { name: offName, new_name: offName })
say('rename while B offline: ' + r.status)
r = await api(ctxA, A, 'POST', `/workspaces/${W}/documents/texts`, { title: `Văn bản lúc mất mạng ${run}` })
await pageB.screenshot({ path: `${OUT}/07-B-offline-stale.png` })
const staleShown = await pageB.getByText(offName).count()
say(`while offline B shows new name: ${staleShown > 0} (expected false)`)
await sleep(28000)
gateB.blocked = false
say('B back online at ' + new Date().toISOString())
t0 = Date.now()
ms = await waitText(pageB, offName, 40000)
say(`5. after 30s outage, B shows the folder name changed meanwhile after ${ms} ms from reconnect`)
await pageB.screenshot({ path: `${OUT}/08-B-after-reconnect.png` })
r = await api(ctxA, A, 'PUT', `/drive/items/${folder}/rename`, { name: `Sau nối lại ${run}`, new_name: `Sau nối lại ${run}` })
ms = await waitText(pageB, `Sau nối lại ${run}`)
say(`5c. live updates resume after reconnect: ${ms} ms`)
const docShown = await pageB.getByText(`Văn bản lúc mất mạng ${run}`).count()
say(`5b. (same page, no reload) the document created meanwhile is not on the drive screen: ${docShown === 0}`)
const kindsAfter = wB.frames.slice(before).map((f) => f.k)
say(`B frames after reconnect: ${JSON.stringify(kindsAfter)}`)

fs.writeFileSync(`${OUT}/e2e-log.txt`, log.join('\n') + '\n')
await browser.close()
