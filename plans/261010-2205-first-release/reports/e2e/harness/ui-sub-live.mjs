import { launch, newPage, step, shot, signInUI, enterWorkspace, tokens, sleep, saveResults, BASE, run, sql, COMPOSER, compose } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
import { request, api } from './lib.mjs'
await launch()
const T = tokens()
const api2 = await request.newContext({ baseURL: BASE })
const WSA = WORLD.wsA, NAMEA = `QA Tenant A ${WORLD.run}`
const F = 'flow5-subfeatures-live'
const { page: M } = await newPage()
await step(F, 'member opens #general', async () => { await signInUI(M, WORLD.M3.email, null); await enterWorkspace(M, NAMEA); await M.getByText('general', { exact: true }).nth(0).click(); await M.locator(COMPOSER).first().waitFor({ timeout: 15000 }); return 'open' }, M)
const CH = sql(`select id from channels where workspace_id = '${WSA}' and name = 'general' limit 1`)
const msgText = `Tin để thả cảm xúc ${run}`
const mid = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/messages`, { content: msgText })
await step(F, 'base message is live for the member', async () => { await M.getByText(msgText).first().waitFor({ timeout: 8000 }); return 'live' }, M)
await step(F, 'reaction added by owner (API) appears live on member page (5s)', async () => {
  const r = await api(api2, { token: T.O }, 'POST', `/api/messages/${mid.body.id}/reactions`, { emoji: '👍' }).catch(() => null)
  const r2 = await api(api2, { token: T.O }, 'POST', `/messages/${mid.body.id}/reactions`, { emoji: '👍' })
  if (r2.status >= 300) throw new Error('reaction ' + r2.status + JSON.stringify(r2.body).slice(0, 100))
  await sleep(5000)
  const live = await M.locator('text=👍').count()
  if (!live) throw new Error('reaction not rendered live (no reload)')
  return 'live'
}, M)
await step(F, 'pin by owner (API) appears live on member page (5s)', async () => {
  const r = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/pins`, { message_id: mid.body.id })
  if (r.status >= 300) throw new Error('pin ' + r.status + JSON.stringify(r.body).slice(0, 100))
  await sleep(5000)
  const live = await M.getByLabel('Đã ghim').count()
  if (!live) throw new Error('pin not rendered live (no reload)')
  return 'live'
}, M)
await step(F, 'poll vote by owner (API) updates the tally live (5s)', async () => {
  const p = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/polls`, { question: `Bình chọn ${run}`, options: ['A', 'B'], is_multi: false, is_anonymous: false })
  if (p.status !== 201) throw new Error('poll ' + p.status)
  const pid = p.body.id, opt = p.body.options?.[0]?.id
  await sleep(1500)
  const v = await api(api2, { token: T.O }, 'POST', `/polls/${pid}/vote`, { option_id: opt })
  await sleep(5000)
  const live = await M.getByText(/1 phiếu|1 lượt|1 người/).count()
  if (v.status >= 300) throw new Error('vote ' + v.status + JSON.stringify(v.body).slice(0, 100))
  if (!live) throw new Error('tally not updated live (no reload)')
  return 'live'
}, M)
await step(F, 'task status change (API) appears live on member page (5s)', async () => {
  await M.getByText('Công việc', { exact: true }).first().click()
  await sleep(1000)
  const t = await api(api2, { token: T.O }, 'POST', `/channels/${CH}/tasks`, { title: `Việc ${run}`, assignee_id: '' })
  await sleep(1500)
  const u = await api(api2, { token: T.O }, 'PATCH', `/tasks/${t.body.id}`, { status: 'done' })
  if (u.status >= 300) throw new Error('task patch ' + u.status + JSON.stringify(u.body).slice(0, 100))
  await sleep(5000)
  const live = await M.getByText(`Việc ${run}`).count()
  if (!live) throw new Error('task created (and then marked done) not in the member Công việc list without reload')
  return 'live'
}, M)
console.log('member errors', M.errors.slice(0, 4))
saveResults('flow5-subfeatures-live')
process.exit(0)
