import { launch, newPage, step, shot, signInUI, sleep, saveResults, BASE } from './ui-lib.mjs'
import { WORLD } from './ui-env.mjs'
await launch()
const F = 'flow4-drive-ui'
const filePath = `${process.env.SP_DIR}/files/hop-dong-ui.pdf`
const rowMenu = async (page, label) => {
  const nameEl = page.getByText(label, { exact: false }).first()
  await nameEl.waitFor({ timeout: 10000 })
  const box = await nameEl.boundingBox()
  await page.mouse.click(1313, box.y + box.height / 2)
}
const { page: po } = await newPage()
await step(F, 'owner signs in (UI)', async () => { await signInUI(po, WORLD.O.email, null); await po.goto(`${BASE}/drive?ws=${WORLD.wsA}`, { waitUntil: 'networkidle' }); return po.url() }, po)
await step(F, 'upload a file through the presigned URL (UI input)', async () => {
  await po.locator('input[type=file]').first().setInputFiles(filePath)
  await po.getByText('hop-dong-ui.pdf').first().waitFor({ timeout: 30000 })
  return 'listed'
}, po)
await step(F, 'share the file to M3 with "Có thể sửa"', async () => {
  await rowMenu(po, 'hop-dong-ui.pdf')
  await po.getByRole('menuitem', { name: 'Chia sẻ' }).click({ timeout: 5000 }).catch(() => po.getByText('Chia sẻ', { exact: true }).first().click())
  const dlg = po.getByRole('dialog')
  await dlg.waitFor({ timeout: 8000 })
  await dlg.getByPlaceholder('Tìm theo tên hoặc phòng ban').fill('Hoa')
  await sleep(1200)
  await shot(po, '21-share-picker')
  await dlg.getByText('Hoa Member', { exact: true }).click({ timeout: 6000 })
  await dlg.getByLabel('Quyền').selectOption('write')
  await sleep(300)
  await shot(po, '21b-permission-set')
  const q = await dlg.innerText(); if (!q.includes('Có thể sửa')) throw new Error('permission not set to Có thể sửa: ' + q.replace(/\n/g,' | '))
  await dlg.getByRole('button', { name: 'Chia sẻ', exact: true }).click()
  await dlg.getByText('Đang chia sẻ với').waitFor({ timeout: 8000 })
  await shot(po, '22-shared')
  return 'shared with write'
}, po)
console.log('owner errors', po.errors.slice(0, 4))
saveResults('flow4-drive-ui-partial')
process.exit(0)
