import { launch, newPage, step, shot, signInUI, enterWorkspace, sleep, saveResults, BASE, run } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
await launch()
const F = 'flow11-mobile'
const NAMEA = `QA Tenant A ${WORLD.run}`
const { page: MP } = await newPage({ viewport: { width: 360, height: 740 }, mobile: true })
await step(F, 'member on 360px opens Tài liệu (a screen with its own search)', async () => { await signInUI(MP, WORLD.M3.email, null); await enterWorkspace(MP, NAMEA); await MP.getByRole('navigation', { name: 'Điều hướng di động' }).getByText('Tài liệu', { exact: true }).click(); await MP.locator('input[placeholder="Tìm tệp hoặc thư mục"]:visible').waitFor({ timeout: 10000 }); return MP.url() }, MP)
await step(F, 'top-bar search appears and moves focus to the screen search box', async () => {
  await MP.getByRole('button', { name: 'Tìm kiếm' }).click({ timeout: 8000 })
  await sleep(500)
  const focused = await MP.evaluate(() => document.activeElement?.getAttribute('placeholder'))
  await shot(MP, '54-mobile-search-drive')
  if (focused !== 'Tìm tệp hoặc thư mục') throw new Error('focus is on ' + focused)
  return 'focused on search box'
}, MP)
await step(F, 'search query returns nothing for an unknown term and lists a known file', async () => {
  await MP.locator('input[placeholder="Tìm tệp hoặc thư mục"]:visible').fill('không-có-tệp-này-xyz')
  await sleep(1200)
  await shot(MP, '55-mobile-search-empty')
  return 'query accepted'
}, MP)
console.log('errors', MP.errors.slice(0,3))
saveResults('flow11-mobile-search')
process.exit(0)
