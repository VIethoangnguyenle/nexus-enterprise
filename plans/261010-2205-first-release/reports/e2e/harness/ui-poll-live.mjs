import { launch, newPage, step, shot, signInUI, enterWorkspace, tokens, sleep, saveResults, BASE, run, sql, COMPOSER, compose } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
import { request, api } from './lib.mjs'
await launch()
const T = tokens()
const api2 = await request.newContext({ baseURL: BASE })
const WSA = WORLD.wsA, NAMEA = `QA Tenant A ${WORLD.run}`
const F = 'flow5-poll-task-live'
const { page: M } = await newPage()
await step(F, 'member opens #general', async () => { await signInUI(M, WORLD.M3.email, null); await enterWorkspace(M, NAMEA); await M.getByText('general', { exact: true }).nth(0).click(); await M.locator(COMPOSER).first().waitFor({ timeout: 15000 }); return 'open' }, M)
const CH = sql(`select id from channels where workspace_id = '${WSA}' and name = 'general' limit 1`)
const q = `Họp thứ Sáu ${run}`
await step(F, 'poll created by owner via API appears in the member stream live', async () => {
  const r = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/polls`, { question: q, options: ['Có', 'Không'], multiple: false, anonymous: false })
  if (r.status !== 201) throw new Error('poll ' + r.status)
  await sleep(5000)
  const seen = await M.getByText(q).count()
  await shot(M, '80-poll-live-check')
  if (!seen) throw new Error('not rendered live after 5s')
  return 'live'
}, M)
await step(F, 'poll visible after reload (is it rendered at all?)', async () => {
  await M.reload({ waitUntil: 'networkidle' })
  await M.getByText('general', { exact: true }).nth(0).click().catch(() => {})
  await sleep(1500)
  const seen = await M.getByText(q).count()
  await shot(M, '81-poll-after-reload')
  if (!seen) throw new Error('poll absent even after reload')
  return 'present after reload'
}, M)
const t = `Soạn biên bản ${run}`
await step(F, 'task created by owner via API appears live in the member stream', async () => {
  const r = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/tasks`, { title: t, assignee_id: '' })
  if (r.status !== 201) throw new Error('task ' + r.status)
  await sleep(5000)
  const seen = await M.getByText(t).count()
  if (!seen) throw new Error('task not rendered live after 5s')
  return 'live'
}, M)
console.log('member errors', M.errors.slice(0, 4))
saveResults('flow5-poll-task-live')
process.exit(0)
