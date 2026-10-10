import { launch, newPage, step, shot, signInUI, enterWorkspace, tokens, sleep, saveResults, BASE, run, sql, COMPOSER, compose } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
import { request, api } from './lib.mjs'
import { execFileSync } from 'node:child_process'
await launch()
const T = tokens()
const api2 = await request.newContext({ baseURL: BASE })
const WSA = WORLD.wsA, NAMEA = `QA Tenant A ${WORLD.run}`, ph = /Tin nhắn cho|Viết tin nhắn/
const F = 'flow13-resilience'
const dock = (...a) => execFileSync('docker', a).toString().trim()
const health = (c) => dock('inspect', '--format', '{{.State.Health.Status}}', c)
const { page: O } = await newPage()
const { page: M } = await newPage()
let sockets = 0
M.on('websocket', () => { sockets++ })
await step(F, 'owner and member sign in and open #general', async () => {
  await signInUI(O, WORLD.O.email, null); await enterWorkspace(O, NAMEA)
  await signInUI(M, WORLD.M3.email, null); await enterWorkspace(M, NAMEA)
  for (const p of [O, M]) { await p.getByText('general', { exact: true }).nth(0).click(); await p.locator(COMPOSER).first().waitFor({ timeout: 15000 }) }
  return 'both in #general'
}, M)
const before = `Trước sự cố ${run}`
await step(F, 'baseline: message is live before the outage', async () => { await compose(O, before); await M.getByText(before).first().waitFor({ timeout: 10000 }); return 'live' }, M)
const socketsBefore = sockets
await step(F, 'kill messaging (docker kill); outage begins', async () => {
  dock('kill', 'nexus-e2e-messaging-1')
  await sleep(3000)
  const st = dock('inspect', '--format', '{{.State.Status}} exit={{.State.ExitCode}}', 'nexus-e2e-messaging-1')
  if (!st.startsWith('exited')) throw new Error('container still ' + st)
  return st
}, M)
const during = { status: 0 }
await step(F, 'during the outage a post is refused by the edge (no silent success)', async () => {
  const ch = sql(`select id from channels where workspace_id = '${WSA}' and name = 'general' limit 1`)
  const r = await api(api2, { token: T.O }, 'POST', `/channels/${ch}/messages`, { content: 'lúc mất kết nối ' + run })
  during.status = r.status
  if (r.status < 500) throw new Error('post accepted during outage: ' + r.status)
  return 'refused with ' + r.status
}, M)
await step(F, 'member page is still open during the outage and raises no page error', async () => {
  await sleep(15000)
  await shot(M, '70-member-during-outage')
  if (M.errors.length) throw new Error('page errors: ' + M.errors.slice(0, 2).join(' | '))
  return 'no page errors'
}, M)
await step(F, 'restart messaging (docker start); healthy again', async () => {
  dock('start', 'nexus-e2e-messaging-1')
  const t0 = Date.now(); let h = ''
  for (let i = 0; i < 60; i++) { h = dock('inspect', '--format', '{{.State.Health.Status}}', 'nexus-e2e-messaging-1'); if (h === 'healthy') break; await sleep(2000) }
  if (h !== 'healthy') throw new Error('messaging not healthy after restart: ' + h)
  return `healthy after ${Math.round((Date.now() - t0) / 1000)}s`
}, M)
const after = `Sau khôi phục ${run}`
await step(F, 'owner posts after restart; member sees it live without reload (WS reconnect + resync)', async () => {
  // allow the reconnect backoff and the auth socket to come back
  let posted = false
  for (let i = 0; i < 10 && !posted; i++) { const ch = sql(`select id from channels where workspace_id = '${WSA}' and name = 'general' limit 1`); const r = await api(api2, { token: T.O }, 'POST', `/channels/${ch}/messages`, { content: after }); posted = r.status === 201; if (!posted) await sleep(2000) }
  if (!posted) throw new Error('owner could not post after restart')
  await M.getByText(after).first().waitFor({ timeout: 30000 })
  return `live (new websocket connections: ${sockets - socketsBefore})`
}, M)
await step(F, 'member socket re-established after the outage', async () => { if (sockets <= socketsBefore) throw new Error('no new websocket after restart'); return `sockets ${sockets}` }, M)
console.log('member page errors', M.errors.slice(0, 5))
saveResults('flow13-msg-ui')
process.exit(0)
