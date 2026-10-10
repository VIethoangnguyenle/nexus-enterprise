import { launch, newPage, step, shot, signInUI, verifyEmail, sleep, run, saveResults, sql, BASE } from './ui-lib.mjs'
import { api, rec } from './lib.mjs'
import { request } from './lib.mjs'
import { ctx as ctxApi } from './world.mjs'
await launch()
const F = 'flow2-invite-leave'
// the flow-1 owner and workspace (latest), found in the DB
const ownerEmail = sql(`select email from users where email like 'e2e-ui-owner-%' order by created_at desc limit 1`)
const wsName = sql(`select name from workspaces where name like 'UI Tenant %' order by created_at desc limit 1`)
const wsId = sql(`select id from workspaces where name = '${wsName}'`)
const invName = `e2e-ui-inv-${Date.now().toString(36)}@example.test`
const api2 = await request.newContext({ baseURL: BASE })
const { signInApi } = await import('./lib.mjs')
const O = await signInApi(api2, ownerEmail, 'Ui Owner')
await (await import('./lib.mjs')).api(api2, O, 'POST', '/auth/switch-tenant', { tenant_id: wsId }).then((r) => { O.token = r.body.access_token })
await step(F, 'owner invites a new person by email (API)', async () => {
  const r = await (await import('./lib.mjs')).api(api2, O, 'POST', `/workspaces/${wsId}/members`, { email: invName, role_id: '', department_id: '' })
  if (r.status >= 300) throw new Error(r.status + JSON.stringify(r.body))
  return r.status
})
const { ctx: bc, page } = await newPage()
await step(F, 'invitee signs in by OTP on UI and verifies email (SQL)', async () => { await signInUI(page, invName, 'Ivy Invitee'); verifyEmail(invName); return page.url() }, page)
await step(F, 'invitee sees the offer on workspace selection (Lời mời)', async () => {
  await page.goto(`${BASE}/workspace-select`, { waitUntil: 'networkidle' })
  await page.getByText('Lời mời').first().waitFor({ timeout: 10000 })
  await page.getByText(wsName).first().waitFor({ timeout: 10000 })
  await shot(page, '10-invitee-offer')
  return 'offer visible'
}, page)
await step(F, 'invitee accepts (Tham gia) and enters the workspace', async () => {
  await page.getByRole('button', { name: 'Tham gia' }).first().click()
  await page.waitForURL(/\/channels/, { timeout: 15000 })
  return page.url()
}, page)
await step(F, 'invitee leaves the workspace from Cài đặt (confirmed)', async () => {
  await page.goto(`${BASE}/settings?ws=${wsId}`, { waitUntil: 'networkidle' })
  await page.getByRole('tab', { name: 'Workspace' }).click().catch(() => page.getByText('Workspace', { exact: true }).first().click())
  await page.getByRole('button', { name: 'Rời workspace' }).first().click()
  await page.getByRole('button', { name: 'Rời workspace' }).last().click()
  await page.waitForURL(/workspace-select|\/channels/, { timeout: 15000 })
  return page.url()
}, page)
// owner: the last owner cannot leave
const { ctx: bo, page: po } = await newPage()
await step(F, 'owner signs in on UI', async () => { await signInUI(po, ownerEmail, null); return po.url() }, po)
await step(F, 'last owner: leaving is refused with an explanation', async () => {
  await po.goto(`${BASE}/settings?ws=${wsId}`, { waitUntil: 'networkidle' })
  await po.getByRole('tab', { name: 'Workspace' }).click().catch(() => po.getByText('Workspace', { exact: true }).first().click())
  await po.getByRole('button', { name: 'Rời workspace' }).first().click()
  await po.getByRole('button', { name: 'Rời workspace' }).last().click()
  await po.getByText('Bạn là chủ sở hữu cuối cùng').first().waitFor({ timeout: 10000 })
  await shot(po, '11-last-owner-refused')
  return 'refusal shown'
}, po)
console.log('invitee page errors:', page.errors.slice(0, 4))
console.log('owner page errors:', po.errors.slice(0, 4))
saveResults('flow2-ui')
process.exit(0)
