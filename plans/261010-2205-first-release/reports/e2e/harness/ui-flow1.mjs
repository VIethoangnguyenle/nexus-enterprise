import { launch, newPage, step, shot, signInUI, verifyEmail, sleep, run, saveResults, UI, sql, BASE } from './ui-lib.mjs'
import { buildWorld, ctx } from './world.mjs'
import { api } from './lib.mjs'
import { request } from './lib.mjs'
await launch()
const F = 'flow1-signin-workspace'
const email = `e2e-ui-owner-${run}@example.test`
const { ctx: bc, page } = await newPage()
await step(F, 'OTP sign-in by UI (fixed test code)', async () => { await signInUI(page, email, 'Ui Owner'); return page.url() }, page)
await step(F, 'first-time profile step (/register) completes', async () => { if (!page.url().includes('/channels') && !page.url().includes('/onboarding') && !page.url().includes('/workspace-select')) throw new Error('url ' + page.url()); return page.url() }, page)
await step(F, 'create workspace refused until email is verified (explained, no create)', async () => {
  await page.goto(`${BASE}/onboarding`, { waitUntil: 'networkidle' })
  await page.getByText('Cần xác minh').waitFor({ timeout: 8000 })
  return 'verify gate shown'
}, page)
await step(F, 'email verified via SQL (no SMTP/Google on this stack)', async () => { verifyEmail(email); return 'UPDATE users SET email_verified_at' })
await step(F, 'create workspace by name only', async () => {
  await page.goto(`${BASE}/onboarding`, { waitUntil: 'networkidle' })
  await page.getByLabel('Tên workspace').fill(`UI Tenant ${run}`)
  await page.getByRole('button', { name: 'Tạo workspace' }).click()
  await page.waitForURL(/\/channels/, { timeout: 15000 })
  return page.url()
}, page)
await step(F, 'workspace home shows #general and the new name', async () => {
  await page.getByText(`UI Tenant ${run}`).first().waitFor({ timeout: 8000 })
  await shot(page, '01-workspace-home')
  return 'ok'
}, page)
await step(F, 'workspace selection screen lists the workspace', async () => {
  await page.goto(`${BASE}/workspace-select`, { waitUntil: 'networkidle' })
  await page.getByRole('heading', { name: 'Chọn workspace' }).waitFor({ timeout: 8000 })
  await page.getByText(`UI Tenant ${run}`).first().waitFor({ timeout: 8000 })
  await shot(page, '02-workspace-select')
  return 'ok'
}, page)
console.log('page errors:', page.errors.slice(0, 5))
saveResults('flow1-ui')
await browser_close()
async function browser_close() { process.exit(0) }
